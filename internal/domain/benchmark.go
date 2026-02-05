// Package domain contains core business entities for RedisMeter.
package domain

import (
	"time"
)

// BenchmarkRun represents a single benchmark execution with its results.
type BenchmarkRun struct {
	// Identity
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Configuration
	Workload    *Workload    `json:"workload"`
	RunProfile  *RunProfile  `json:"run_profile,omitempty"`
	Target      *Target      `json:"target"`
	Environment *Environment `json:"environment"`

	// Execution
	Status    RunStatus `json:"status"`
	StartTime time.Time `json:"start_time,omitempty"`
	EndTime   time.Time `json:"end_time,omitempty"`
	Duration  string    `json:"duration,omitempty"`

	// Results
	Results *Results `json:"results,omitempty"`
	Error   string   `json:"error,omitempty"`

	// Metadata
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// RunStatus represents the lifecycle state of a benchmark run.
type RunStatus string

const (
	RunStatusPending   RunStatus = "pending"
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCancelled RunStatus = "cancelled"
)

// RunProfile defines HOW to execute a benchmark (execution settings).
// This is separate from Workload which defines WHAT to test.
type RunProfile struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IsBuiltin   bool   `json:"is_builtin,omitempty"`

	// Parallelism
	Threads int `json:"threads"` // Number of threads (default: 4)
	Clients int `json:"clients"` // Clients per thread (default: 50)

	// Duration/Requests - one of these should be set
	Duration string `json:"duration,omitempty"` // Test duration (e.g., "30s", "5m")
	Requests int64  `json:"requests,omitempty"` // Total requests per client (overrides duration)

	// Pipelining
	Pipeline int `json:"pipeline"` // Concurrent pipelined requests (default: 1)

	// Rate Limiting
	RateLimit int `json:"rate_limit,omitempty"` // Max requests/sec per connection (0=unlimited)

	// Iterations
	RunCount int `json:"run_count,omitempty"` // Number of test iterations (default: 1)

	// Connection
	ReconnectInterval int    `json:"reconnect_interval,omitempty"` // Reconnect after N requests (0=never)
	Protocol          string `json:"protocol,omitempty"`           // redis, resp2, resp3 (default: redis)
	SelectDB          int    `json:"select_db,omitempty"`          // Redis DB number (default: 0)

	// Advanced
	DistinctClientSeed bool `json:"distinct_client_seed,omitempty"` // Different seed per client
	RandomizeSeed      bool `json:"randomize_seed,omitempty"`       // Timestamp-based random seed
	MultiKeyGet        int  `json:"multi_key_get,omitempty"`        // Multi-key GET up to N keys (0=disabled)

	// Output
	PrintPercentiles []float64 `json:"print_percentiles,omitempty"` // Percentiles to print (default: 50,99,99.9)
	HideHistogram    bool      `json:"hide_histogram,omitempty"`    // Hide latency histogram
}

// Workload defines WHAT to test (workload pattern).
// This is separate from RunProfile which defines HOW to execute.
type Workload struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`       // e.g., "cache", "session", "custom"
	IsBuiltin   bool   `json:"is_builtin,omitempty"`

	// Operations - what commands to run
	Operations []Operation `json:"operations"`

	// Key configuration
	KeyPattern *KeyPattern `json:"key_pattern,omitempty"`

	// Data configuration
	DataSize    *DataSize `json:"data_size,omitempty"`
	RandomData  bool      `json:"random_data,omitempty"`  // Randomize value content
	DataOffset  int       `json:"data_offset,omitempty"`  // Use SETRANGE/GETRANGE with offset

	// Expiry
	ExpiryMin int `json:"expiry_min,omitempty"` // Min expiry in seconds
	ExpiryMax int `json:"expiry_max,omitempty"` // Max expiry in seconds

	// Custom commands (for arbitrary command workloads)
	CustomCommands []CustomCommand `json:"custom_commands,omitempty"`

	// DEPRECATED: These are moving to RunProfile, kept for backwards compatibility
	Threads      int           `json:"threads,omitempty"`
	Clients      int           `json:"clients,omitempty"`
	Duration     string        `json:"duration,omitempty"`
	Requests     int64         `json:"requests,omitempty"`
	Pipeline     int           `json:"pipeline,omitempty"`
	RateLimiting *RateLimiting `json:"rate_limiting,omitempty"`

	// Custom parameters passed to executor
	Custom map[string]interface{} `json:"custom,omitempty"`
}

// CustomCommand defines an arbitrary memtier command.
type CustomCommand struct {
	Command    string `json:"command"`               // e.g., "SET __key__ __data__"
	Ratio      int    `json:"ratio,omitempty"`       // How many times to send in sequence
	KeyPattern string `json:"key_pattern,omitempty"` // G=Gaussian, R=Random, S=Sequential, P=Parallel, Z=Zipf
}

// Operation defines a single Redis operation in a workload.
type Operation struct {
	Command string   `json:"command"` // e.g., "GET", "SET", "HGET"
	Ratio   float64  `json:"ratio"`   // Proportion of this operation (0.0 to 1.0)
	Args    []string `json:"args,omitempty"`
}

// KeyPattern configures how keys are generated.
type KeyPattern struct {
	Prefix   string `json:"prefix,omitempty"`
	Pattern  string `json:"pattern"` // random (R), sequential (S), gaussian (G), zipf (Z), parallel (P)
	KeyRange int64  `json:"key_range,omitempty"`
	KeyMin   int64  `json:"key_min,omitempty"` // Key ID minimum (default: 0)
	KeyMax   int64  `json:"key_max,omitempty"` // Key ID maximum (default: 10000000)

	// Gaussian distribution settings
	KeyStddev float64 `json:"key_stddev,omitempty"` // Standard deviation (default: key_range/6)
	KeyMedian int64   `json:"key_median,omitempty"` // Median point (default: center of range)

	// Zipf distribution settings
	ZipfExponent float64 `json:"zipf_exponent,omitempty"` // Exponent 0-5 (default: 1)
}

// DataSize configures the size of values.
type DataSize struct {
	Min   int `json:"min,omitempty"`
	Max   int `json:"max,omitempty"`
	Fixed int `json:"fixed,omitempty"`

	// Weighted size list: "size1:weight1,size2:weight2,..."
	SizeList string `json:"size_list,omitempty"`

	// Pattern: R=random sizes from range, S=evenly distributed across key range
	SizePattern string `json:"size_pattern,omitempty"`
}

// RateLimiting configures request rate limits.
type RateLimiting struct {
	RequestsPerSecond int `json:"requests_per_second,omitempty"`
	BurstSize         int `json:"burst_size,omitempty"`
}

// DefaultRunProfile returns a sensible default run profile.
func DefaultRunProfile() *RunProfile {
	return &RunProfile{
		Name:        "default",
		Description: "Default execution profile",
		Threads:     4,
		Clients:     50,
		Duration:    "30s",
		Pipeline:    1,
		RunCount:    1,
		Protocol:    "redis",
	}
}
