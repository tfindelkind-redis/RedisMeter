// Package engine provides the core benchmark orchestration.
package engine

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tfindelkind-redis/redismeter/internal/domain"
	"github.com/tfindelkind-redis/redismeter/internal/environment"
	"github.com/tfindelkind-redis/redismeter/internal/memtier"
	"github.com/tfindelkind-redis/redismeter/internal/plugin"
	"github.com/tfindelkind-redis/redismeter/internal/storage"
	"github.com/tfindelkind-redis/redismeter/internal/workload"
)

// Engine orchestrates benchmark execution.
type Engine struct {
	storage    storage.RunStorage
	memtier    *memtier.Executor
	envCapture *environment.Capturer
}

// EngineConfig holds configuration for the engine.
type EngineConfig struct {
	// StorageConfig configures the storage backend.
	StorageConfig storage.StorageConfig
}

// NewEngine creates a new benchmark engine with default configuration.
func NewEngine() (*Engine, error) {
	return NewEngineWithConfig(EngineConfig{})
}

// NewEngineWithConfig creates a new benchmark engine with custom configuration.
func NewEngineWithConfig(cfg EngineConfig) (*Engine, error) {
	store, err := storage.NewStorage(cfg.StorageConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Cast to RunStorage
	runStore, ok := store.(storage.RunStorage)
	if !ok {
		return nil, fmt.Errorf("storage backend does not support run operations")
	}

	return &Engine{
		storage:    runStore,
		memtier:    memtier.NewExecutor(),
		envCapture: environment.NewCapturer(),
	}, nil
}

// RunConfig holds configuration for a benchmark run.
type RunConfig struct {
	WorkloadName   string
	WorkloadFile   string
	RunProfileName string // Name of run profile to use
	TargetURL      string
	// Legacy overrides - these override run profile settings
	Duration  string
	Threads   int
	Clients   int
	Pipeline  int
	Requests  int64
	RateLimit int
	Name      string
	Tags      []string
}

// ProgressCallback is called with progress updates during execution.
type ProgressCallback func(message string)

// Run executes a benchmark and returns the result.
func (e *Engine) Run(ctx context.Context, cfg *RunConfig, progress ProgressCallback) (*domain.BenchmarkRun, error) {
	// Generate unique run ID
	runID := generateRunID()

	if progress != nil {
		progress(fmt.Sprintf("Starting benchmark run %s", runID))
	}

	// Parse target URL
	target, err := parseTargetURL(cfg.TargetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid target URL: %w", err)
	}

	if progress != nil {
		progress(fmt.Sprintf("Target: %s:%d", target.Host, target.Port))
	}

	// Load workload
	var wl *domain.Workload
	if cfg.WorkloadFile != "" {
		wl, err = workload.LoadFromFile(cfg.WorkloadFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load workload file: %w", err)
		}
	} else {
		wl, err = workload.DefaultRegistry.Get(cfg.WorkloadName)
		if err != nil {
			return nil, fmt.Errorf("workload not found: %w", err)
		}
	}

	// Build run profile from configuration
	runProfile := buildRunProfile(cfg)

	// Apply legacy overrides from config to workload (for backwards compatibility)
	if cfg.Duration != "" && runProfile.Duration == "" {
		wl.Duration = cfg.Duration
	}
	if cfg.Threads > 0 && runProfile.Threads == 0 {
		wl.Threads = cfg.Threads
	}
	if cfg.Clients > 0 && runProfile.Clients == 0 {
		wl.Clients = cfg.Clients
	}
	if cfg.Pipeline > 0 && runProfile.Pipeline == 0 {
		wl.Pipeline = cfg.Pipeline
	}

	if progress != nil {
		progress(fmt.Sprintf("Workload: %s (%s)", wl.Name, wl.Description))
	}

	// Capture environment
	if progress != nil {
		progress("Capturing environment fingerprint...")
	}
	env, err := e.envCapture.Capture(ctx, target)
	if err != nil {
		// Non-fatal - continue without full environment
		env = &domain.Environment{
			Fingerprint:       "unknown",
			RedisMeterVersion: "0.1.0-dev",
		}
	}

	// Create benchmark run record
	run := &domain.BenchmarkRun{
		ID:          runID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Workload:    wl,
		RunProfile:  runProfile,
		Target:      target,
		Environment: env,
		Status:      domain.RunStatusRunning,
		StartTime:   time.Now(),
		Name:        cfg.Name,
		Tags:        cfg.Tags,
	}

	// Check memtier availability
	if err := e.memtier.CheckAvailable(); err != nil {
		run.Status = domain.RunStatusFailed
		run.Error = err.Error()
		run.EndTime = time.Now()
		e.storage.SaveRun(ctx, run)
		return run, fmt.Errorf("memtier_benchmark not available: %w", err)
	}

	// Build memtier config from workload + run profile
	memtierCfg := memtier.FromWorkloadAndRunProfile(wl, runProfile)
	memtierCfg.Host = target.Host
	memtierCfg.Port = target.Port
	memtierCfg.Password = target.Password
	memtierCfg.TLS = target.TLS != nil && target.TLS.Enabled
	memtierCfg.Cluster = target.Cluster

	// Determine duration for progress message
	durationStr := runProfile.Duration
	if runProfile.Requests > 0 {
		durationStr = fmt.Sprintf("%d requests", runProfile.Requests)
	} else if durationStr == "" {
		durationStr = wl.Duration
	}

	if progress != nil {
		progress(fmt.Sprintf("Running benchmark (duration: %s)...", durationStr))
	}

	// Execute benchmark
	result, err := e.memtier.Run(ctx, memtierCfg, func(line string) {
		if progress != nil && strings.Contains(line, "Ops/sec") {
			progress(line)
		}
	})

	run.EndTime = time.Now()
	run.Duration = run.EndTime.Sub(run.StartTime).String()

	if err != nil || result.Error != nil {
		run.Status = domain.RunStatusFailed
		if err != nil {
			run.Error = err.Error()
		} else {
			run.Error = result.Error.Error()
		}
		e.storage.SaveRun(ctx, run)
		return run, fmt.Errorf("benchmark execution failed: %w", err)
	}

	// Parse results
	if progress != nil {
		progress("Parsing results...")
	}

	parser := memtier.NewParser()
	results, err := parser.Parse(result.RawJSON)
	if err != nil {
		run.Status = domain.RunStatusFailed
		run.Error = fmt.Sprintf("failed to parse results: %v", err)
		e.storage.SaveRun(ctx, run)
		return run, err
	}

	// Always preserve raw memtier output for debugging and analysis
	results.RawOutput = string(result.RawJSON)

	run.Results = results
	run.Status = domain.RunStatusCompleted
	run.UpdatedAt = time.Now()

	// Save run
	if err := e.storage.SaveRun(ctx, run); err != nil {
		return run, fmt.Errorf("failed to save run: %w", err)
	}

	if progress != nil {
		progress(fmt.Sprintf("Benchmark complete. Run ID: %s", runID))
	}

	return run, nil
}

// ListRuns returns recent benchmark runs.
func (e *Engine) ListRuns(ctx context.Context, limit int) ([]*domain.BenchmarkRun, error) {
	return e.storage.ListRuns(ctx, limit)
}

// GetRun retrieves a benchmark run by ID.
func (e *Engine) GetRun(ctx context.Context, id string) (*domain.BenchmarkRun, error) {
	return e.storage.GetRun(ctx, id)
}

// DeleteRun removes a benchmark run.
func (e *Engine) DeleteRun(ctx context.Context, id string) error {
	return e.storage.DeleteRun(ctx, id)
}

// QueryRuns queries benchmark runs with the given filter.
func (e *Engine) QueryRuns(ctx context.Context, filter plugin.QueryFilter) ([]*domain.BenchmarkRun, error) {
	var runs []*domain.BenchmarkRun
	if err := e.storage.Query(ctx, "runs", filter, &runs); err != nil {
		return nil, err
	}
	return runs, nil
}

// generateRunID creates a unique run identifier.
func generateRunID() string {
	// Format: YYYYMMDD-HHMMSS-XXXX where XXXX is random
	now := time.Now()
	id := uuid.New().String()[:8]
	return fmt.Sprintf("%s-%s", now.Format("20060102-150405"), id)
}

// parseTargetURL parses a Redis URL into a Target.
func parseTargetURL(rawURL string) (*domain.Target, error) {
	// Handle simple host:port format
	if !strings.Contains(rawURL, "://") {
		rawURL = "redis://" + rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	target := &domain.Target{
		URL:  rawURL,
		Host: u.Hostname(),
		Port: 6379, // default
	}

	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil {
			return nil, fmt.Errorf("invalid port: %s", u.Port())
		}
		target.Port = port
	}

	// Extract password from URL
	if u.User != nil {
		if pwd, set := u.User.Password(); set {
			target.Password = pwd
			target.Username = u.User.Username()
		} else {
			// Password only (no username)
			target.Password = u.User.Username()
		}
	}

	// Check for cluster mode
	if u.Scheme == "redis+cluster" || strings.Contains(rawURL, "cluster=true") {
		target.Cluster = true
	}

	// Check for TLS
	if u.Scheme == "rediss" || strings.Contains(rawURL, "tls=true") {
		target.TLS = &domain.TLSConfig{Enabled: true}
	}

	return target, nil
}

// workloadToMemtierConfig converts a workload to memtier config.
func workloadToMemtierConfig(wl *domain.Workload, target *domain.Target) *memtier.Config {
	cfg := memtier.DefaultConfig()

	// Connection
	cfg.Host = target.Host
	cfg.Port = target.Port
	cfg.Password = target.Password
	cfg.Username = target.Username
	cfg.Cluster = target.Cluster
	if target.TLS != nil {
		cfg.TLS = target.TLS.Enabled
	}

	// Workload ratio
	var getRatio, setRatio float64
	for _, op := range wl.Operations {
		switch strings.ToUpper(op.Command) {
		case "GET":
			getRatio = op.Ratio
		case "SET":
			setRatio = op.Ratio
		}
	}
	if getRatio > 0 || setRatio > 0 {
		// Normalize and format as memtier ratio (SET:GET)
		total := getRatio + setRatio
		if total > 0 {
			setN := int(setRatio / total * 10)
			getN := int(getRatio / total * 10)
			if setN == 0 && setRatio > 0 {
				setN = 1
			}
			if getN == 0 && getRatio > 0 {
				getN = 1
			}
			cfg.Ratio = fmt.Sprintf("%d:%d", setN, getN)
		}
	}

	// Key pattern
	if wl.KeyPattern != nil {
		cfg.KeyPrefix = wl.KeyPattern.Prefix
		cfg.KeyMaximum = wl.KeyPattern.KeyRange
		switch wl.KeyPattern.Pattern {
		case "random":
			cfg.KeyPattern = "R:R"
		case "sequential":
			cfg.KeyPattern = "S:S"
		case "gaussian":
			cfg.KeyPattern = "G:G"
		default:
			cfg.KeyPattern = "R:R"
		}
	}

	// Data size
	if wl.DataSize != nil {
		if wl.DataSize.Fixed > 0 {
			cfg.DataSize = wl.DataSize.Fixed
		} else if wl.DataSize.Min > 0 && wl.DataSize.Max > 0 {
			cfg.DataSizeMin = wl.DataSize.Min
			cfg.DataSizeMax = wl.DataSize.Max
		}
	}

	// Execution params
	cfg.Threads = wl.Threads
	cfg.Clients = wl.Clients
	cfg.Pipeline = wl.Pipeline

	// Duration
	if wl.Duration != "" {
		if d, err := time.ParseDuration(wl.Duration); err == nil {
			cfg.Duration = d
		}
	}

	// Requests (if specified instead of duration)
	if wl.Requests > 0 {
		cfg.Requests = wl.Requests
		cfg.Duration = 0
	}

	return cfg
}

// buildRunProfile creates a run profile from config, merging with built-in if specified.
func buildRunProfile(cfg *RunConfig) *domain.RunProfile {
	var profile *domain.RunProfile

	// Start with built-in profile if specified
	if cfg.RunProfileName != "" {
		profile = getBuiltinRunProfile(cfg.RunProfileName)
	}

	// If no profile found, use default
	if profile == nil {
		profile = domain.DefaultRunProfile()
	}

	// Apply CLI overrides
	if cfg.Duration != "" {
		profile.Duration = cfg.Duration
	}
	if cfg.Threads > 0 {
		profile.Threads = cfg.Threads
	}
	if cfg.Clients > 0 {
		profile.Clients = cfg.Clients
	}
	if cfg.Pipeline > 0 {
		profile.Pipeline = cfg.Pipeline
	}
	if cfg.Requests > 0 {
		profile.Requests = cfg.Requests
		profile.Duration = "" // Requests override duration
	}
	if cfg.RateLimit > 0 {
		profile.RateLimit = cfg.RateLimit
	}

	return profile
}

// getBuiltinRunProfile returns a built-in run profile by name.
func getBuiltinRunProfile(name string) *domain.RunProfile {
	profiles := map[string]*domain.RunProfile{
		"default": {
			Name:        "default",
			Description: "Default execution profile - balanced settings",
			IsBuiltin:   true,
			Threads:     4,
			Clients:     50,
			Duration:    "30s",
			Pipeline:    1,
			RunCount:    1,
			Protocol:    "redis",
		},
		"quick-test": {
			Name:        "quick-test",
			Description: "Quick test - short duration for validation",
			IsBuiltin:   true,
			Threads:     2,
			Clients:     10,
			Duration:    "10s",
			Pipeline:    1,
			RunCount:    1,
			Protocol:    "redis",
		},
		"high-load": {
			Name:        "high-load",
			Description: "High load test - maximum parallelism",
			IsBuiltin:   true,
			Threads:     8,
			Clients:     100,
			Duration:    "60s",
			Pipeline:    10,
			RunCount:    1,
			Protocol:    "redis",
		},
		"low-latency": {
			Name:        "low-latency",
			Description: "Low latency measurement - minimal pipelining",
			IsBuiltin:   true,
			Threads:     2,
			Clients:     10,
			Duration:    "30s",
			Pipeline:    1,
			RunCount:    3,
			Protocol:    "redis",
		},
		"throughput": {
			Name:        "throughput",
			Description: "Throughput focused - aggressive pipelining",
			IsBuiltin:   true,
			Threads:     4,
			Clients:     100,
			Duration:    "60s",
			Pipeline:    20,
			RunCount:    1,
			Protocol:    "redis",
		},
		"stress": {
			Name:        "stress",
			Description: "Stress test - extended duration with high load",
			IsBuiltin:   true,
			Threads:     8,
			Clients:     200,
			Duration:    "300s",
			Pipeline:    10,
			RunCount:    1,
			Protocol:    "redis",
		},
		"rate-limited": {
			Name:        "rate-limited",
			Description: "Rate limited - controlled request rate",
			IsBuiltin:   true,
			Threads:     4,
			Clients:     50,
			Duration:    "30s",
			Pipeline:    1,
			RunCount:    1,
			RateLimit:   10000,
			Protocol:    "redis",
		},
		"request-based": {
			Name:        "request-based",
			Description: "Request based - fixed number of requests",
			IsBuiltin:   true,
			Threads:     4,
			Clients:     50,
			Requests:    100000,
			Pipeline:    1,
			RunCount:    1,
			Protocol:    "redis",
		},
	}

	if p, ok := profiles[name]; ok {
		// Return a copy
		copy := *p
		return &copy
	}
	return nil
}
