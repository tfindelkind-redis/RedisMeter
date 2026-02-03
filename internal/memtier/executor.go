// Package memtier provides integration with memtier_benchmark.
package memtier

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config holds memtier_benchmark configuration options.
type Config struct {
	// Connection
	Host     string
	Port     int
	Password string
	Username string
	TLS      bool
	Cluster  bool

	// Workload
	Ratio       string // e.g., "1:1" for 50% GET, 50% SET
	KeyPattern  string // random, sequential, gaussian
	KeyMinimum  int64
	KeyMaximum  int64
	KeyPrefix   string
	DataSize    int
	DataSizeMin int
	DataSizeMax int
	Expiry      int

	// Execution
	Threads     int
	Clients     int
	Requests    int64
	Duration    time.Duration
	Pipeline    int
	RateLimit   int
	Randomize   bool

	// Output
	JSONOutput bool
	HideHistogram bool
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Host:       "localhost",
		Port:       6379,
		Ratio:      "1:1",
		KeyPattern: "R", // random
		KeyMinimum: 1,
		KeyMaximum: 10000000,
		DataSize:   32,
		Threads:    4,
		Clients:    50,
		Duration:   30 * time.Second,
		Pipeline:   1,
		JSONOutput: true,
	}
}

// CommandBuilder builds memtier_benchmark command arguments.
type CommandBuilder struct {
	config *Config
}

// NewCommandBuilder creates a new command builder.
func NewCommandBuilder(config *Config) *CommandBuilder {
	return &CommandBuilder{config: config}
}

// Build returns the command arguments for memtier_benchmark.
func (b *CommandBuilder) Build() []string {
	args := []string{}

	// Connection
	args = append(args, "-s", b.config.Host)
	args = append(args, "-p", strconv.Itoa(b.config.Port))

	if b.config.Password != "" {
		args = append(args, "-a", b.config.Password)
	}
	if b.config.Username != "" {
		args = append(args, "--user", b.config.Username)
	}
	if b.config.TLS {
		args = append(args, "--tls")
		args = append(args, "--tls-skip-verify")
	}
	if b.config.Cluster {
		args = append(args, "--cluster-mode")
	}

	// Workload
	args = append(args, "--ratio", b.config.Ratio)
	args = append(args, "--key-pattern", b.config.KeyPattern)
	args = append(args, "--key-minimum", strconv.FormatInt(b.config.KeyMinimum, 10))
	args = append(args, "--key-maximum", strconv.FormatInt(b.config.KeyMaximum, 10))

	if b.config.KeyPrefix != "" {
		args = append(args, "--key-prefix", b.config.KeyPrefix)
	}

	if b.config.DataSizeMin > 0 && b.config.DataSizeMax > 0 {
		args = append(args, "--data-size-range", fmt.Sprintf("%d-%d", b.config.DataSizeMin, b.config.DataSizeMax))
	} else if b.config.DataSize > 0 {
		args = append(args, "-d", strconv.Itoa(b.config.DataSize))
	}

	if b.config.Expiry > 0 {
		args = append(args, "--expiry-range", fmt.Sprintf("%d-%d", b.config.Expiry, b.config.Expiry))
	}

	// Execution
	args = append(args, "-t", strconv.Itoa(b.config.Threads))
	args = append(args, "-c", strconv.Itoa(b.config.Clients))

	if b.config.Requests > 0 {
		args = append(args, "-n", strconv.FormatInt(b.config.Requests, 10))
	}
	if b.config.Duration > 0 {
		args = append(args, "--test-time", strconv.Itoa(int(b.config.Duration.Seconds())))
	}

	args = append(args, "--pipeline", strconv.Itoa(b.config.Pipeline))

	if b.config.RateLimit > 0 {
		args = append(args, "--rate-limiting", strconv.Itoa(b.config.RateLimit))
	}
	if b.config.Randomize {
		args = append(args, "--randomize")
	}

	// Output
	if b.config.JSONOutput {
		args = append(args, "--json-out-file", "/dev/stdout")
	}
	if b.config.HideHistogram {
		args = append(args, "--hide-histogram")
	}

	return args
}

// Executor runs memtier_benchmark and captures results.
type Executor struct {
	binaryPath string
	mu         sync.Mutex
	cmd        *exec.Cmd
	cancel     context.CancelFunc
}

// NewExecutor creates a new memtier executor.
func NewExecutor() *Executor {
	return &Executor{
		binaryPath: "memtier_benchmark",
	}
}

// SetBinaryPath sets a custom path to memtier_benchmark binary.
func (e *Executor) SetBinaryPath(path string) {
	e.binaryPath = path
}

// CheckAvailable verifies memtier_benchmark is installed and accessible.
func (e *Executor) CheckAvailable() error {
	cmd := exec.Command(e.binaryPath, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("memtier_benchmark not found or not executable: %w (output: %s)", err, string(output))
	}
	return nil
}

// GetVersion returns the memtier_benchmark version.
func (e *Executor) GetVersion() (string, error) {
	cmd := exec.Command(e.binaryPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// ExecutionResult holds the output from a memtier run.
type ExecutionResult struct {
	RawJSON    []byte
	RawText    string
	ExitCode   int
	Duration   time.Duration
	Error      error
}

// Run executes memtier_benchmark with the given configuration.
func (e *Executor) Run(ctx context.Context, config *Config, progressFn func(line string)) (*ExecutionResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	defer func() { e.cancel = nil }()

	// Create temp file for JSON output
	tmpDir := os.TempDir()
	jsonFile := filepath.Join(tmpDir, fmt.Sprintf("redismeter-%d.json", time.Now().UnixNano()))
	defer os.Remove(jsonFile)

	// Override JSON output to temp file
	config.JSONOutput = false // We'll add it manually
	builder := NewCommandBuilder(config)
	args := builder.Build()
	args = append(args, "--json-out-file", jsonFile)

	e.cmd = exec.CommandContext(ctx, e.binaryPath, args...)

	// Capture stderr for progress
	stderr, err := e.cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Capture stdout (text output)
	stdout, err := e.cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	startTime := time.Now()

	if err := e.cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start memtier_benchmark: %w", err)
	}

	// Read output
	var textOutput strings.Builder

	// Read stderr for progress
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			textOutput.WriteString(line + "\n")
			if progressFn != nil {
				progressFn(line)
			}
		}
	}()

	// Read stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			textOutput.WriteString(line + "\n")
			if progressFn != nil {
				progressFn(line)
			}
		}
	}()

	err = e.cmd.Wait()
	duration := time.Since(startTime)

	result := &ExecutionResult{
		RawText:  textOutput.String(),
		Duration: duration,
	}

	// Read JSON from file
	jsonOutput, readErr := os.ReadFile(jsonFile)
	if readErr == nil {
		result.RawJSON = jsonOutput
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
		result.Error = err
	}

	return result, nil
}

// Stop terminates a running execution.
func (e *Executor) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cancel != nil {
		e.cancel()
	}
	return nil
}
