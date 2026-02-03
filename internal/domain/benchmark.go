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

// Workload defines the benchmark workload configuration.
type Workload struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Type        string                 `json:"type"` // e.g., "cache", "session", "custom"
	
	// Workload parameters
	Operations  []Operation            `json:"operations"`
	KeyPattern  *KeyPattern            `json:"key_pattern,omitempty"`
	DataSize    *DataSize              `json:"data_size,omitempty"`
	
	// Execution parameters
	Threads     int                    `json:"threads,omitempty"`
	Clients     int                    `json:"clients,omitempty"`
	Duration    string                 `json:"duration,omitempty"`
	Requests    int64                  `json:"requests,omitempty"`
	
	// Advanced settings
	Pipeline    int                    `json:"pipeline,omitempty"`
	RateLimiting *RateLimiting         `json:"rate_limiting,omitempty"`
	
	// Custom parameters passed to executor
	Custom      map[string]interface{} `json:"custom,omitempty"`
}

// Operation defines a single Redis operation in a workload.
type Operation struct {
	Command string  `json:"command"` // e.g., "GET", "SET", "HGET"
	Ratio   float64 `json:"ratio"`   // Proportion of this operation (0.0 to 1.0)
	Args    []string `json:"args,omitempty"`
}

// KeyPattern configures how keys are generated.
type KeyPattern struct {
	Prefix      string `json:"prefix,omitempty"`
	Pattern     string `json:"pattern"` // e.g., "random", "sequential", "gaussian"
	KeyRange    int64  `json:"key_range,omitempty"`
	HotspotFrac float64 `json:"hotspot_fraction,omitempty"`
}

// DataSize configures the size of values.
type DataSize struct {
	Min     int    `json:"min,omitempty"`
	Max     int    `json:"max,omitempty"`
	Fixed   int    `json:"fixed,omitempty"`
	Pattern string `json:"pattern,omitempty"` // e.g., "random", "compressible"
}

// RateLimiting configures request rate limits.
type RateLimiting struct {
	RequestsPerSecond int `json:"requests_per_second,omitempty"`
	BurstSize         int `json:"burst_size,omitempty"`
}
