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

	"github.com/tfindelkind-redis/redismeter/internal/domain"
)

// Config holds memtier_benchmark configuration options.
type Config struct {
	// Connection - Basic
	Host       string
	Port       int
	UnixSocket string // UNIX Domain socket path
	Password   string
	Username   string
	URI        string // redis://user:password@host:port/dbnum

	// Connection - TLS
	TLS           bool
	TLSCert       string // Client certificate file
	TLSKey        string // Private key file
	TLSCACert     string // CA certs bundle
	TLSSkipVerify bool   // Skip server cert verification
	TLSProtocols  string // TLS version: TLSv1,TLSv1.1,TLSv1.2,TLSv1.3
	TLSSNI        string // SNI header

	// Connection - Network
	ForceIPv4 bool // Force IPv4 resolution
	ForceIPv6 bool // Force IPv6 resolution
	Cluster   bool // Redis cluster mode

	// Workload settings (WHAT to test)
	Ratio        string  // e.g., "1:1" for 50% GET, 50% SET
	KeyPattern   string  // R=random, S=sequential, G=gaussian, Z=zipf, P=parallel
	KeyMinimum   int64
	KeyMaximum   int64
	KeyPrefix    string
	KeyStddev    float64 // For Gaussian distribution
	KeyMedian    int64   // For Gaussian distribution
	ZipfExponent float64 // For Zipf distribution (0-5)

	// Workload - Data size
	DataSize     int
	DataSizeMin  int
	DataSizeMax  int
	DataSizeList string // Weighted list: "size1:weight1,size2:weight2"
	DataPattern  string // R=random, S=sequential
	RandomData   bool   // Randomize data content
	DataOffset   int    // Use SETRANGE/GETRANGE with offset
	ExpiryMin    int
	ExpiryMax    int

	// Data import options
	DataImport   string // File to import data from
	DataVerify   bool   // Verify imported data after test
	VerifyOnly   bool   // Only verify, no other test
	GenerateKeys bool   // Generate keys for imported objects
	NoExpiry     bool   // Ignore expiry in imported data

	// Custom commands
	CustomCommands []CustomCommand

	// Run profile settings (HOW to run)
	Threads            int
	Clients            int
	Requests           int64
	Duration           time.Duration
	Pipeline           int
	RateLimit          int
	RunCount           int    // Number of test iterations
	ReconnectInterval  int    // Reconnect after N requests
	Protocol           string // redis, resp2, resp3
	SelectDB           int    // Redis DB number
	DistinctClientSeed bool
	RandomizeSeed      bool
	MultiKeyGet        int // Multi-key GET up to N keys

	// WAIT options (for replication)
	WaitRatio      string // Set:Wait ratio (default no WAIT)
	NumSlavesMin   int    // WAIT for min slaves
	NumSlavesMax   int    // WAIT for max slaves
	WaitTimeoutMin int    // WAIT timeout min (ms)
	WaitTimeoutMax int    // WAIT timeout max (ms)

	// Output options
	JSONOutput       bool
	JSONOutFile      string   // JSON output file path
	OutFile          string   // Output file path
	HdrFilePrefix    string   // HDR histogram file prefix
	ClientStats      string   // Per-client stats file
	HideHistogram    bool
	PrintPercentiles []float64
	PrintAllRuns     bool   // Print results for all iterations
	ShowConfig       bool   // Print detailed config before running
	Debug            bool   // Print debug output
}

// CustomCommand for arbitrary memtier commands.
type CustomCommand struct {
	Command    string
	Ratio      int
	KeyPattern string
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Host:       "localhost",
		Port:       6379,
		Ratio:      "1:1",
		KeyPattern: "R:R", // random for both SET and GET
		KeyMinimum: 1,     // memtier requires > 0
		KeyMaximum: 10000000,
		DataSize:   32,
		Threads:    4,
		Clients:    50,
		Duration:   30 * time.Second,
		Pipeline:   1,
		RunCount:   1,
		Protocol:   "redis",
		JSONOutput: true,
	}
}

// FromWorkloadAndRunProfile creates a Config from domain models.
func FromWorkloadAndRunProfile(workload *domain.Workload, runProfile *domain.RunProfile) *Config {
	cfg := DefaultConfig()

	// Apply workload settings (WHAT to test)
	if workload != nil {
		// Calculate ratio from operations
		var getRatio, setRatio float64
		for _, op := range workload.Operations {
			switch strings.ToUpper(op.Command) {
			case "GET":
				getRatio += op.Ratio
			case "SET":
				setRatio += op.Ratio
			}
		}
		if getRatio > 0 || setRatio > 0 {
			// Convert to memtier ratio format (SET:GET)
			if getRatio == 0 {
				cfg.Ratio = "1:0"
			} else if setRatio == 0 {
				cfg.Ratio = "0:1"
			} else {
				// Normalize to simple ratio
				total := getRatio + setRatio
				setNorm := int(setRatio / total * 10)
				getNorm := int(getRatio / total * 10)
				cfg.Ratio = fmt.Sprintf("%d:%d", setNorm, getNorm)
			}
		}

		// Key pattern
		if workload.KeyPattern != nil {
			kp := workload.KeyPattern
			cfg.KeyPrefix = kp.Prefix
			switch strings.ToLower(kp.Pattern) {
			case "random":
				cfg.KeyPattern = "R:R"
			case "sequential":
				cfg.KeyPattern = "S:S"
			case "gaussian":
				cfg.KeyPattern = "G:G"
			case "zipf":
				cfg.KeyPattern = "Z:Z"
			case "parallel":
				cfg.KeyPattern = "P:P"
			default:
				cfg.KeyPattern = kp.Pattern
			}
			if kp.KeyMin > 0 {
				cfg.KeyMinimum = kp.KeyMin
			}
			if kp.KeyMax > 0 {
				cfg.KeyMaximum = kp.KeyMax
			} else if kp.KeyRange > 0 {
				cfg.KeyMaximum = kp.KeyRange
			}
			cfg.KeyStddev = kp.KeyStddev
			cfg.KeyMedian = kp.KeyMedian
			cfg.ZipfExponent = kp.ZipfExponent
		}

		// Data size
		if workload.DataSize != nil {
			ds := workload.DataSize
			if ds.Fixed > 0 {
				cfg.DataSize = ds.Fixed
			} else {
				cfg.DataSizeMin = ds.Min
				cfg.DataSizeMax = ds.Max
			}
			cfg.DataSizeList = ds.SizeList
			cfg.DataPattern = ds.SizePattern
		}

		cfg.RandomData = workload.RandomData
		cfg.DataOffset = workload.DataOffset
		cfg.ExpiryMin = workload.ExpiryMin
		cfg.ExpiryMax = workload.ExpiryMax

		// Custom commands
		for _, cc := range workload.CustomCommands {
			cfg.CustomCommands = append(cfg.CustomCommands, CustomCommand{
				Command:    cc.Command,
				Ratio:      cc.Ratio,
				KeyPattern: cc.KeyPattern,
			})
		}

		// Backwards compatibility: use workload execution settings if no run profile
		if runProfile == nil {
			if workload.Threads > 0 {
				cfg.Threads = workload.Threads
			}
			if workload.Clients > 0 {
				cfg.Clients = workload.Clients
			}
			if workload.Duration != "" {
				if d, err := time.ParseDuration(workload.Duration); err == nil {
					cfg.Duration = d
				}
			}
			if workload.Requests > 0 {
				cfg.Requests = workload.Requests
			}
			if workload.Pipeline > 0 {
				cfg.Pipeline = workload.Pipeline
			}
			if workload.RateLimiting != nil && workload.RateLimiting.RequestsPerSecond > 0 {
				cfg.RateLimit = workload.RateLimiting.RequestsPerSecond
			}
		}
	}

	// Apply run profile settings (HOW to run) - these override workload settings
	if runProfile != nil {
		if runProfile.Threads > 0 {
			cfg.Threads = runProfile.Threads
		}
		if runProfile.Clients > 0 {
			cfg.Clients = runProfile.Clients
		}
		if runProfile.Duration != "" {
			if d, err := time.ParseDuration(runProfile.Duration); err == nil {
				cfg.Duration = d
			}
		}
		if runProfile.Requests > 0 {
			cfg.Requests = runProfile.Requests
			cfg.Duration = 0 // Requests override duration
		}
		if runProfile.Pipeline > 0 {
			cfg.Pipeline = runProfile.Pipeline
		}
		cfg.RateLimit = runProfile.RateLimit
		cfg.RunCount = runProfile.RunCount
		cfg.ReconnectInterval = runProfile.ReconnectInterval
		if runProfile.Protocol != "" {
			cfg.Protocol = runProfile.Protocol
		}
		cfg.SelectDB = runProfile.SelectDB
		cfg.DistinctClientSeed = runProfile.DistinctClientSeed
		cfg.RandomizeSeed = runProfile.RandomizeSeed
		cfg.MultiKeyGet = runProfile.MultiKeyGet
		cfg.HideHistogram = runProfile.HideHistogram
		cfg.PrintPercentiles = runProfile.PrintPercentiles
	}

	return cfg
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

	// Connection - Basic
	if b.config.UnixSocket != "" {
		args = append(args, "-S", b.config.UnixSocket)
	} else if b.config.URI != "" {
		args = append(args, "-u", b.config.URI)
	} else {
		args = append(args, "-s", b.config.Host)
		args = append(args, "-p", strconv.Itoa(b.config.Port))
	}

	if b.config.Password != "" {
		args = append(args, "-a", b.config.Password)
	}
	if b.config.Username != "" {
		// For Redis 6+ ACL: user:password format in -a
		// Or use separate user flag if available
		args = append(args, "--user", b.config.Username)
	}

	// Connection - Network
	if b.config.ForceIPv4 {
		args = append(args, "-4")
	}
	if b.config.ForceIPv6 {
		args = append(args, "-6")
	}

	// Connection - TLS
	if b.config.TLS {
		args = append(args, "--tls")
		if b.config.TLSCert != "" {
			args = append(args, "--cert", b.config.TLSCert)
		}
		if b.config.TLSKey != "" {
			args = append(args, "--key", b.config.TLSKey)
		}
		if b.config.TLSCACert != "" {
			args = append(args, "--cacert", b.config.TLSCACert)
		}
		if b.config.TLSSkipVerify {
			args = append(args, "--tls-skip-verify")
		}
		if b.config.TLSProtocols != "" {
			args = append(args, "--tls-protocols", b.config.TLSProtocols)
		}
		if b.config.TLSSNI != "" {
			args = append(args, "--sni", b.config.TLSSNI)
		}
	}

	// Cluster mode
	if b.config.Cluster {
		args = append(args, "--cluster-mode")
	}

	// Protocol
	if b.config.Protocol != "" && b.config.Protocol != "redis" {
		args = append(args, "-P", b.config.Protocol)
	}

	// DB selection
	if b.config.SelectDB > 0 {
		args = append(args, "--select-db", strconv.Itoa(b.config.SelectDB))
	}

	// Check if we're using custom commands (they conflict with --ratio and --key-pattern)
	hasCustomCommands := len(b.config.CustomCommands) > 0

	// Workload - Key settings (only if NOT using custom commands)
	if !hasCustomCommands {
		args = append(args, "--ratio", b.config.Ratio)
		
		// Ensure key-pattern is in correct format (X:X)
		keyPattern := b.config.KeyPattern
		if keyPattern != "" && !strings.Contains(keyPattern, ":") {
			keyPattern = keyPattern + ":" + keyPattern
		}
		args = append(args, "--key-pattern", keyPattern)
	}
	
	// Key range (always needed) - ensure key-minimum > 0
	keyMin := b.config.KeyMinimum
	if keyMin <= 0 {
		keyMin = 1
	}
	args = append(args, "--key-minimum", strconv.FormatInt(keyMin, 10))
	args = append(args, "--key-maximum", strconv.FormatInt(b.config.KeyMaximum, 10))

	if b.config.KeyPrefix != "" {
		args = append(args, "--key-prefix", b.config.KeyPrefix)
	}
	if b.config.KeyStddev > 0 {
		args = append(args, "--key-stddev", strconv.FormatFloat(b.config.KeyStddev, 'f', -1, 64))
	}
	if b.config.KeyMedian > 0 {
		args = append(args, "--key-median", strconv.FormatInt(b.config.KeyMedian, 10))
	}
	if b.config.ZipfExponent > 0 {
		args = append(args, "--key-zipf-exp", strconv.FormatFloat(b.config.ZipfExponent, 'f', -1, 64))
	}

	// Workload - Data settings
	if b.config.DataSizeList != "" {
		args = append(args, "--data-size-list", b.config.DataSizeList)
	} else if b.config.DataSizeMin > 0 && b.config.DataSizeMax > 0 {
		args = append(args, "--data-size-range", fmt.Sprintf("%d-%d", b.config.DataSizeMin, b.config.DataSizeMax))
		if b.config.DataPattern != "" {
			args = append(args, "--data-size-pattern", b.config.DataPattern)
		}
	} else if b.config.DataSize > 0 {
		args = append(args, "-d", strconv.Itoa(b.config.DataSize))
	}

	if b.config.RandomData {
		args = append(args, "-R")
	}
	if b.config.DataOffset > 0 {
		args = append(args, "--data-offset", strconv.Itoa(b.config.DataOffset))
	}

	// Workload - Expiry
	if b.config.ExpiryMin > 0 || b.config.ExpiryMax > 0 {
		min := b.config.ExpiryMin
		max := b.config.ExpiryMax
		if max == 0 {
			max = min
		}
		if min == 0 {
			min = max
		}
		args = append(args, "--expiry-range", fmt.Sprintf("%d-%d", min, max))
	}

	// Data import options
	if b.config.DataImport != "" {
		args = append(args, "--data-import", b.config.DataImport)
		if b.config.DataVerify {
			args = append(args, "--data-verify")
		}
		if b.config.VerifyOnly {
			args = append(args, "--verify-only")
		}
		if b.config.GenerateKeys {
			args = append(args, "--generate-keys")
		}
		if b.config.NoExpiry {
			args = append(args, "--no-expiry")
		}
	}

	// Custom commands (when using --command, must use --command-key-pattern instead of --key-pattern)
	for _, cmd := range b.config.CustomCommands {
		args = append(args, "--command", cmd.Command)
		if cmd.Ratio > 0 {
			args = append(args, "--command-ratio", strconv.Itoa(cmd.Ratio))
		}
		// --command-key-pattern is required when using custom commands
		keyPattern := cmd.KeyPattern
		if keyPattern == "" {
			keyPattern = "R" // Default to Random
		}
		args = append(args, "--command-key-pattern", keyPattern)
	}

	// Execution - Parallelism
	args = append(args, "-t", strconv.Itoa(b.config.Threads))
	args = append(args, "-c", strconv.Itoa(b.config.Clients))

	// Execution - Duration/Requests
	if b.config.Requests > 0 {
		args = append(args, "-n", strconv.FormatInt(b.config.Requests, 10))
	}
	if b.config.Duration > 0 && b.config.Requests == 0 {
		args = append(args, "--test-time", strconv.Itoa(int(b.config.Duration.Seconds())))
	}

	// Execution - Pipeline
	args = append(args, "--pipeline", strconv.Itoa(b.config.Pipeline))

	// Execution - Rate limiting
	if b.config.RateLimit > 0 {
		args = append(args, "--rate-limiting", strconv.Itoa(b.config.RateLimit))
	}

	// Execution - Iterations
	if b.config.RunCount > 1 {
		args = append(args, "-x", strconv.Itoa(b.config.RunCount))
	}

	// Execution - Reconnect
	if b.config.ReconnectInterval > 0 {
		args = append(args, "--reconnect-interval", strconv.Itoa(b.config.ReconnectInterval))
	}

	// Execution - Multi-key operations
	if b.config.MultiKeyGet > 0 {
		args = append(args, "--multi-key-get", strconv.Itoa(b.config.MultiKeyGet))
	}

	// Execution - Randomization
	if b.config.DistinctClientSeed {
		args = append(args, "--distinct-client-seed")
	}
	if b.config.RandomizeSeed {
		args = append(args, "--randomize")
	}

	// WAIT options (for replication scenarios)
	if b.config.WaitRatio != "" {
		args = append(args, "--wait-ratio", b.config.WaitRatio)
	}
	if b.config.NumSlavesMin > 0 || b.config.NumSlavesMax > 0 {
		min := b.config.NumSlavesMin
		max := b.config.NumSlavesMax
		if max == 0 {
			max = min
		}
		if min == 0 {
			min = max
		}
		args = append(args, "--num-slaves", fmt.Sprintf("%d-%d", min, max))
	}
	if b.config.WaitTimeoutMin > 0 || b.config.WaitTimeoutMax > 0 {
		min := b.config.WaitTimeoutMin
		max := b.config.WaitTimeoutMax
		if max == 0 {
			max = min
		}
		if min == 0 {
			min = max
		}
		args = append(args, "--wait-timeout", fmt.Sprintf("%d-%d", min, max))
	}

	// Output options
	if b.config.Debug {
		args = append(args, "-D")
	}
	if b.config.ShowConfig {
		args = append(args, "--show-config")
	}
	if b.config.OutFile != "" {
		args = append(args, "-o", b.config.OutFile)
	}
	if b.config.JSONOutFile != "" {
		args = append(args, "--json-out-file", b.config.JSONOutFile)
	} else if b.config.JSONOutput {
		args = append(args, "--json-out-file", "/dev/stdout")
	}
	if b.config.HdrFilePrefix != "" {
		args = append(args, "--hdr-file-prefix", b.config.HdrFilePrefix)
	}
	if b.config.ClientStats != "" {
		args = append(args, "--client-stats", b.config.ClientStats)
	}
	if b.config.HideHistogram {
		args = append(args, "--hide-histogram")
	}
	if len(b.config.PrintPercentiles) > 0 {
		pcts := make([]string, len(b.config.PrintPercentiles))
		for i, p := range b.config.PrintPercentiles {
			pcts[i] = strconv.FormatFloat(p, 'f', -1, 64)
		}
		args = append(args, "--print-percentiles", strings.Join(pcts, ","))
	}
	if b.config.PrintAllRuns {
		args = append(args, "--print-all-runs")
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
