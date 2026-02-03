// Package domain contains core business entities for RedisMeter.
package domain

import (
	"time"
)

// Baseline represents a saved performance baseline for comparison.
type Baseline struct {
	// Identity
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	
	// Reference run
	RunID     string    `json:"run_id"`
	
	// Metadata
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	
	// Validity
	Active      bool      `json:"active"`
	ValidFrom   time.Time `json:"valid_from,omitempty"`
	ValidUntil  time.Time `json:"valid_until,omitempty"`
	
	// Captured metrics (snapshot from run)
	Metrics     *SummaryMetrics `json:"metrics"`
	
	// Environment requirements for valid comparison
	Environment *EnvironmentConstraints `json:"environment,omitempty"`
	
	// Workload reference
	Workload    *Workload `json:"workload"`
	
	// Thresholds for comparison
	Thresholds  *BaselineThresholds `json:"thresholds,omitempty"`
}

// EnvironmentConstraints defines what environment factors must match
// for a run to be validly compared against this baseline.
type EnvironmentConstraints struct {
	RequireMatchingRedisVersion bool `json:"require_matching_redis_version,omitempty"`
	RequireMatchingEnvironment  bool `json:"require_matching_environment,omitempty"`
	AllowedEnvironmentDrift     float64 `json:"allowed_environment_drift,omitempty"` // percentage
}

// BaselineThresholds defines acceptable deviation from baseline.
type BaselineThresholds struct {
	// Maximum allowed regression (as percentage)
	MaxThroughputRegression float64 `json:"max_throughput_regression,omitempty"` // e.g., 0.05 = 5%
	MaxLatencyRegression    float64 `json:"max_latency_regression,omitempty"`
	MaxP99Regression        float64 `json:"max_p99_regression,omitempty"`
	
	// Maximum allowed error rate increase
	MaxErrorRateIncrease    float64 `json:"max_error_rate_increase,omitempty"`
}

// ComparisonResult represents the result of comparing a run against a baseline.
type ComparisonResult struct {
	// References
	RunID      string `json:"run_id"`
	BaselineID string `json:"baseline_id"`
	
	// Overall verdict
	Pass       bool   `json:"pass"`
	Verdict    string `json:"verdict"` // "pass", "fail", "warning"
	
	// Detailed metrics comparison
	Metrics    *MetricsComparison `json:"metrics"`
	
	// Environment comparison
	EnvironmentMatch bool     `json:"environment_match"`
	EnvironmentDiffs []string `json:"environment_diffs,omitempty"`
	
	// Threshold violations
	Violations []ThresholdViolation `json:"violations,omitempty"`
}

// MetricsComparison shows how run metrics compare to baseline.
type MetricsComparison struct {
	ThroughputChange float64 `json:"throughput_change"` // percentage, positive = improvement
	AvgLatencyChange float64 `json:"avg_latency_change"`
	P50LatencyChange float64 `json:"p50_latency_change"`
	P90LatencyChange float64 `json:"p90_latency_change"`
	P95LatencyChange float64 `json:"p95_latency_change"`
	P99LatencyChange float64 `json:"p99_latency_change"`
	ErrorRateChange  float64 `json:"error_rate_change"`
}

// ThresholdViolation describes a specific threshold that was exceeded.
type ThresholdViolation struct {
	Metric     string  `json:"metric"`
	Threshold  float64 `json:"threshold"`
	Actual     float64 `json:"actual"`
	Severity   string  `json:"severity"` // "warning", "error"
}
