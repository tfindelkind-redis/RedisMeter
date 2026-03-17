package task

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/memtier"
)

// MemtierHandler executes memtier_benchmark as part of a task.
// This integrates the existing memtier executor with the task system.
type MemtierHandler struct {
	BaseHandler
	executor *memtier.Executor
}

// NewMemtierHandler creates a new memtier benchmark handler.
func NewMemtierHandler() *MemtierHandler {
	return &MemtierHandler{
		BaseHandler: BaseHandler{name: "memtier_benchmark"},
		executor:    memtier.NewExecutor(),
	}
}

// SetBinaryPath configures the path to memtier_benchmark binary.
func (h *MemtierHandler) SetBinaryPath(path string) {
	h.executor.SetBinaryPath(path)
}

func (h *MemtierHandler) Validate(ctx context.Context, task *Task) error {
	if task.Tool != ToolMemtier {
		return fmt.Errorf("task tool must be memtier, got %s", task.Tool)
	}
	if task.ToolConfig == "" {
		return fmt.Errorf("memtier config is required")
	}

	// Verify memtier is available
	if err := h.executor.CheckAvailable(); err != nil {
		return fmt.Errorf("memtier_benchmark not available: %w", err)
	}

	return nil
}

func (h *MemtierHandler) Execute(ctx context.Context, task *Task, step *Step, progress ProgressFunc) error {
	// Parse config from task
	var config memtier.Config
	if err := json.Unmarshal([]byte(task.ToolConfig), &config); err != nil {
		return fmt.Errorf("failed to parse memtier config: %w", err)
	}

	// If target URL is set on task, override config
	if task.TargetURL != "" {
		if err := applyTargetURL(&config, task.TargetURL); err != nil {
			return fmt.Errorf("failed to apply target URL: %w", err)
		}
	}

	progress(5, "Starting memtier_benchmark...")

	// Track progress based on output
	startTime := time.Now()
	progressParser := newProgressParser(&config)

	result, err := h.executor.Run(ctx, &config, func(line string) {
		// Parse progress from memtier output
		pct, msg := progressParser.parseLine(line, startTime)
		if pct > 0 {
			// Scale from 5-95% (reserve 0-5 for startup, 95-100 for completion)
			scaledPct := 5 + (pct * 90 / 100)
			progress(scaledPct, msg)
		}
	})

	if err != nil {
		// Check if it's a context cancellation
		if ctx.Err() != nil {
			// Save checkpoint with runtime info
			step.CheckpointData = map[string]interface{}{
				"runtime_seconds": time.Since(startTime).Seconds(),
				"status":          "interrupted",
				"partial_output":  truncateOutput(result.RawText, 10000),
			}
			return ctx.Err()
		}
		return fmt.Errorf("memtier_benchmark failed: %w", err)
	}

	// Store results
	if result.RawJSON != nil {
		step.Output = string(result.RawJSON)
	} else {
		step.Output = result.RawText
	}

	progress(100, fmt.Sprintf("Benchmark completed in %v", result.Duration.Truncate(time.Second)))
	return nil
}

// applyTargetURL parses a Redis URL and applies it to the config.
func applyTargetURL(config *memtier.Config, url string) error {
	// Support formats:
	// - redis://host:port
	// - rediss://host:port (TLS)
	// - redis://user:password@host:port
	// - host:port
	// - host

	url = strings.TrimSpace(url)

	if strings.HasPrefix(url, "rediss://") {
		config.TLS = true
		url = strings.TrimPrefix(url, "rediss://")
	} else if strings.HasPrefix(url, "redis://") {
		url = strings.TrimPrefix(url, "redis://")
	}

	// Handle user:password@
	if idx := strings.LastIndex(url, "@"); idx != -1 {
		auth := url[:idx]
		url = url[idx+1:]

		if colonIdx := strings.Index(auth, ":"); colonIdx != -1 {
			config.Username = auth[:colonIdx]
			config.Password = auth[colonIdx+1:]
		} else {
			config.Password = auth
		}
	}

	// Handle host:port
	if colonIdx := strings.LastIndex(url, ":"); colonIdx != -1 {
		config.Host = url[:colonIdx]
		portStr := url[colonIdx+1:]
		// Remove any trailing path
		if slashIdx := strings.Index(portStr, "/"); slashIdx != -1 {
			portStr = portStr[:slashIdx]
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return fmt.Errorf("invalid port: %s", portStr)
		}
		config.Port = port
	} else {
		config.Host = url
		if config.Port == 0 {
			config.Port = 6379
		}
	}

	return nil
}

// progressParser extracts progress information from memtier output.
type progressParser struct {
	totalRequests int64
	duration      time.Duration
	hasRequests   bool
	hasDuration   bool
}

func newProgressParser(config *memtier.Config) *progressParser {
	return &progressParser{
		totalRequests: config.Requests,
		duration:      config.Duration,
		hasRequests:   config.Requests > 0,
		hasDuration:   config.Duration > 0,
	}
}

// parseLine extracts progress from a memtier output line.
func (p *progressParser) parseLine(line string, startTime time.Time) (percent int, message string) {
	// Memtier progress formats vary by output mode
	// Look for patterns like:
	// [RUN #1 100%, 500 secs]
	// [WARMUP 50%...]
	// Ops/sec: 12345

	// Handle [RUN #N XX%, YY secs] pattern
	runPattern := regexp.MustCompile(`\[RUN #\d+ (\d+)%`)
	if matches := runPattern.FindStringSubmatch(line); len(matches) > 1 {
		pct, _ := strconv.Atoi(matches[1])
		return pct, fmt.Sprintf("Running benchmark: %d%%", pct)
	}

	// Handle time-based progress
	if p.hasDuration && !p.hasRequests {
		elapsed := time.Since(startTime)
		pct := int(elapsed.Seconds() * 100 / p.duration.Seconds())
		if pct > 100 {
			pct = 100
		}
		if pct > 0 {
			return pct, fmt.Sprintf("Running: %v / %v", elapsed.Truncate(time.Second), p.duration)
		}
	}

	// Handle ops/sec output (indicates progress)
	if strings.Contains(line, "Ops/sec:") || strings.Contains(line, "ops/sec") {
		// Extract ops/sec for status message
		opsPattern := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(?:Ops/sec|ops/sec)`)
		if matches := opsPattern.FindStringSubmatch(line); len(matches) > 1 {
			return 0, fmt.Sprintf("Running at %s ops/sec", matches[1])
		}
	}

	return 0, ""
}

// truncateOutput truncates output to maxLen bytes.
func truncateOutput(output string, maxLen int) string {
	if len(output) <= maxLen {
		return output
	}
	return output[:maxLen] + "\n... (truncated)"
}

// MemtierConfig is a helper to build memtier configuration for tasks.
type MemtierConfig struct {
	// Connection
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Password string `json:"password,omitempty"`
	TLS      bool   `json:"tls,omitempty"`
	Cluster  bool   `json:"cluster,omitempty"`

	// Workload
	Ratio      string `json:"ratio,omitempty"`   // e.g., "1:1"
	KeyPattern string `json:"key_pattern,omitempty"`
	KeyMinimum int64  `json:"key_minimum,omitempty"`
	KeyMaximum int64  `json:"key_maximum,omitempty"`
	DataSize   int    `json:"data_size,omitempty"`

	// Run settings
	Threads  int           `json:"threads,omitempty"`
	Clients  int           `json:"clients,omitempty"`
	Requests int64         `json:"requests,omitempty"`
	Duration time.Duration `json:"duration,omitempty"`
	Pipeline int           `json:"pipeline,omitempty"`
}

// ToJSON converts the config to JSON for task.ToolConfig.
func (c *MemtierConfig) ToJSON() (string, error) {
	// Convert to full memtier.Config
	full := memtier.Config{
		Host:       c.Host,
		Port:       c.Port,
		Password:   c.Password,
		TLS:        c.TLS,
		Cluster:    c.Cluster,
		Ratio:      c.Ratio,
		KeyPattern: c.KeyPattern,
		KeyMinimum: c.KeyMinimum,
		KeyMaximum: c.KeyMaximum,
		DataSize:   c.DataSize,
		Threads:    c.Threads,
		Clients:    c.Clients,
		Requests:   c.Requests,
		Duration:   c.Duration,
		Pipeline:   c.Pipeline,
	}

	data, err := json.Marshal(full)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// CreateMemtierTask creates a new benchmark task for memtier_benchmark.
func CreateMemtierTask(name string, config *MemtierConfig, options ...TaskOption) (*Task, error) {
	configJSON, err := config.ToJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize config: %w", err)
	}

	steps := StandardMemtierSteps()

	task := &Task{
		ID:          generateID(),
		Name:        name,
		Tool:        ToolMemtier,
		ToolConfig:  configJSON,
		Status:      StatusPending,
		Steps:       make([]Step, 0, len(steps)),
		CreatedAt:   time.Now().UTC(),
		LastUpdatedAt: time.Now().UTC(),
		MaxRetries:  3,
	}

	// Initialize steps
	for _, stepType := range steps {
		task.Steps = append(task.Steps, Step{
			Type:   stepType,
			Status: StepPending,
		})
	}

	// Apply options
	for _, opt := range options {
		opt(task)
	}

	// If deploying infrastructure, prepend infra steps
	if task.DeployInfra {
		task.Steps = nil
		infraSteps := WithInfraDeployment(steps, task.DestroyInfra)
		for _, stepType := range infraSteps {
			task.Steps = append(task.Steps, Step{
				Type:   stepType,
				Status: StepPending,
			})
		}
	}

	return task, nil
}

// TaskOption is a functional option for configuring tasks.
type TaskOption func(*Task)

// WithDescription sets the task description.
func WithDescription(desc string) TaskOption {
	return func(t *Task) {
		t.Description = desc
	}
}

// WithTags adds tags to the task.
func WithTags(tags ...string) TaskOption {
	return func(t *Task) {
		t.Tags = append(t.Tags, tags...)
	}
}

// WithInfraProfile sets the infrastructure profile for cloud deployment.
func WithInfraProfile(profileID string) TaskOption {
	return func(t *Task) {
		t.InfraProfileID = profileID
	}
}

// WithCloudDeployment enables infrastructure deployment.
func WithCloudDeployment(provider string, destroyAfter bool) TaskOption {
	return func(t *Task) {
		t.DeployInfra = true
		t.DestroyInfra = destroyAfter
		t.CloudProvider = provider
	}
}

// WithTargetURL sets a specific target URL (skips infrastructure deployment).
func WithTargetURL(url string) TaskOption {
	return func(t *Task) {
		t.TargetURL = url
		t.DeployInfra = false
	}
}

// WithTimeout sets the task timeout.
func WithTimeout(seconds int) TaskOption {
	return func(t *Task) {
		t.TimeoutSeconds = seconds
	}
}

// generateID creates a unique task ID.
func generateID() string {
	return fmt.Sprintf("task-%d", time.Now().UnixNano())
}
