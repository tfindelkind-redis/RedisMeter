// Package analysis provides performance analysis and comparison capabilities.
package analysis

import (
	"context"
	"fmt"
	"math"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
	"github.com/tfindelkind-redis/redismeter/internal/storage"
)

// Comparator compares benchmark runs and baselines.
type Comparator struct {
	storage storage.RunStorage
}

// NewComparator creates a new comparator with the given storage.
func NewComparator(store storage.RunStorage) *Comparator {
	return &Comparator{storage: store}
}

// CompareRuns compares two benchmark runs and returns detailed comparison.
func (c *Comparator) CompareRuns(ctx context.Context, runID1, runID2 string) (*RunComparison, error) {
	run1, err := c.storage.GetRun(ctx, runID1)
	if err != nil {
		return nil, fmt.Errorf("failed to load run %s: %w", runID1, err)
	}

	run2, err := c.storage.GetRun(ctx, runID2)
	if err != nil {
		return nil, fmt.Errorf("failed to load run %s: %w", runID2, err)
	}

	return c.compareRunData(run1, run2), nil
}

// CompareToBaseline compares a run against a baseline.
func (c *Comparator) CompareToBaseline(ctx context.Context, runID, baselineID string) (*domain.ComparisonResult, error) {
	run, err := c.storage.GetRun(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to load run %s: %w", runID, err)
	}

	// Load baseline
	var baseline domain.Baseline
	if err := c.storage.Load(ctx, "baseline", baselineID, &baseline); err != nil {
		return nil, fmt.Errorf("failed to load baseline %s: %w", baselineID, err)
	}

	return c.compareRunToBaseline(run, &baseline), nil
}

// Compare directly compares two benchmark runs without storage lookup.
func (c *Comparator) Compare(run1, run2 *domain.BenchmarkRun) *RunComparison {
	return c.compareRunData(run1, run2)
}

// CompareRunToBaselineMetrics compares a run directly against a baseline without storage lookup.
func (c *Comparator) CompareRunToBaselineMetrics(run *domain.BenchmarkRun, baseline *domain.Baseline) *domain.ComparisonResult {
	return c.compareRunToBaseline(run, baseline)
}

// RunComparison contains detailed comparison between two runs.
type RunComparison struct {
	Run1ID string `json:"run1_id"`
	Run2ID string `json:"run2_id"`

	// Which run is better overall
	BetterRun string `json:"better_run"` // "run1", "run2", "equal"

	// Metrics comparison (run2 relative to run1)
	Metrics *MetricsDiff `json:"metrics"`

	// Environment comparison
	EnvironmentMatch bool     `json:"environment_match"`
	EnvironmentDiffs []string `json:"environment_diffs,omitempty"`

	// Workload comparison
	WorkloadMatch bool   `json:"workload_match"`
	WorkloadDiff  string `json:"workload_diff,omitempty"`

	// Detailed statistics
	Statistics *StatisticalComparison `json:"statistics,omitempty"`
}

// MetricsDiff shows the difference between two sets of metrics.
type MetricsDiff struct {
	// Throughput
	OpsPerSecond1  float64 `json:"ops_per_second_1"`
	OpsPerSecond2  float64 `json:"ops_per_second_2"`
	ThroughputDiff float64 `json:"throughput_diff"` // percentage change

	// Latency (average)
	AvgLatency1  float64 `json:"avg_latency_1"`
	AvgLatency2  float64 `json:"avg_latency_2"`
	AvgLatencyDiff float64 `json:"avg_latency_diff"`

	// P50 Latency
	P50Latency1  float64 `json:"p50_latency_1"`
	P50Latency2  float64 `json:"p50_latency_2"`
	P50LatencyDiff float64 `json:"p50_latency_diff"`

	// P99 Latency
	P99Latency1  float64 `json:"p99_latency_1"`
	P99Latency2  float64 `json:"p99_latency_2"`
	P99LatencyDiff float64 `json:"p99_latency_diff"`

	// P999 Latency
	P999Latency1  float64 `json:"p999_latency_1"`
	P999Latency2  float64 `json:"p999_latency_2"`
	P999LatencyDiff float64 `json:"p999_latency_diff"`

	// Error rate
	ErrorRate1  float64 `json:"error_rate_1"`
	ErrorRate2  float64 `json:"error_rate_2"`
	ErrorRateDiff float64 `json:"error_rate_diff"`
}

// StatisticalComparison contains statistical analysis of the comparison.
type StatisticalComparison struct {
	// Confidence level (0-1)
	Confidence float64 `json:"confidence"`

	// Is the difference statistically significant?
	SignificantDifference bool `json:"significant_difference"`

	// Summary statistics
	ThroughputStdDev1 float64 `json:"throughput_std_dev_1,omitempty"`
	ThroughputStdDev2 float64 `json:"throughput_std_dev_2,omitempty"`
	LatencyStdDev1    float64 `json:"latency_std_dev_1,omitempty"`
	LatencyStdDev2    float64 `json:"latency_std_dev_2,omitempty"`
}

// compareRunData performs the actual comparison of two runs.
func (c *Comparator) compareRunData(run1, run2 *domain.BenchmarkRun) *RunComparison {
	comparison := &RunComparison{
		Run1ID: run1.ID,
		Run2ID: run2.ID,
	}

	// Compare metrics
	if run1.Results != nil && run1.Results.Summary != nil &&
		run2.Results != nil && run2.Results.Summary != nil {
		comparison.Metrics = c.compareMetrics(run1.Results.Summary, run2.Results.Summary)
		comparison.BetterRun = determineBetterRun(comparison.Metrics)
	} else {
		comparison.BetterRun = "unknown"
	}

	// Compare environments
	comparison.EnvironmentMatch, comparison.EnvironmentDiffs = c.compareEnvironments(run1.Environment, run2.Environment)

	// Compare workloads
	comparison.WorkloadMatch, comparison.WorkloadDiff = c.compareWorkloads(run1.Workload, run2.Workload)

	return comparison
}

// compareMetrics compares two summary metrics and returns the differences.
func (c *Comparator) compareMetrics(m1, m2 *domain.SummaryMetrics) *MetricsDiff {
	diff := &MetricsDiff{
		OpsPerSecond1: m1.OpsPerSecond,
		OpsPerSecond2: m2.OpsPerSecond,
		AvgLatency1:   m1.AvgLatencyMs,
		AvgLatency2:   m2.AvgLatencyMs,
		P50Latency1:   m1.P50LatencyMs,
		P50Latency2:   m2.P50LatencyMs,
		P99Latency1:   m1.P99LatencyMs,
		P99Latency2:   m2.P99LatencyMs,
		P999Latency1:  m1.P999LatencyMs,
		P999Latency2:  m2.P999LatencyMs,
		ErrorRate1:    m1.ErrorRate,
		ErrorRate2:    m2.ErrorRate,
	}

	// Calculate percentage changes (positive = improvement for throughput, regression for latency)
	diff.ThroughputDiff = percentChange(m1.OpsPerSecond, m2.OpsPerSecond)
	diff.AvgLatencyDiff = percentChange(m1.AvgLatencyMs, m2.AvgLatencyMs)
	diff.P50LatencyDiff = percentChange(m1.P50LatencyMs, m2.P50LatencyMs)
	diff.P99LatencyDiff = percentChange(m1.P99LatencyMs, m2.P99LatencyMs)
	diff.P999LatencyDiff = percentChange(m1.P999LatencyMs, m2.P999LatencyMs)
	diff.ErrorRateDiff = percentChange(m1.ErrorRate, m2.ErrorRate)

	return diff
}

// compareEnvironments compares two environments and returns match status and differences.
func (c *Comparator) compareEnvironments(env1, env2 *domain.Environment) (bool, []string) {
	if env1 == nil || env2 == nil {
		if env1 == nil && env2 == nil {
			return true, nil
		}
		return false, []string{"one environment is missing"}
	}

	var diffs []string

	// Compare host info
	if env1.Hostname != env2.Hostname {
		diffs = append(diffs, fmt.Sprintf("hostname: %s vs %s", env1.Hostname, env2.Hostname))
	}
	if env1.OS != env2.OS {
		diffs = append(diffs, fmt.Sprintf("os: %s vs %s", env1.OS, env2.OS))
	}
	if env1.CPUCores != env2.CPUCores {
		diffs = append(diffs, fmt.Sprintf("cpu_cores: %d vs %d", env1.CPUCores, env2.CPUCores))
	}
	if env1.MemoryGB != env2.MemoryGB {
		diffs = append(diffs, fmt.Sprintf("memory_gb: %.1f vs %.1f", env1.MemoryGB, env2.MemoryGB))
	}

	// Compare Redis info
	if env1.RedisVersion != env2.RedisVersion {
		diffs = append(diffs, fmt.Sprintf("redis_version: %s vs %s", env1.RedisVersion, env2.RedisVersion))
	}

	return len(diffs) == 0, diffs
}

// compareWorkloads compares two workloads and returns match status and description.
func (c *Comparator) compareWorkloads(w1, w2 *domain.Workload) (bool, string) {
	if w1 == nil || w2 == nil {
		if w1 == nil && w2 == nil {
			return true, ""
		}
		return false, "one workload is missing"
	}

	if w1.Name != w2.Name {
		return false, fmt.Sprintf("different workloads: %s vs %s", w1.Name, w2.Name)
	}

	return true, ""
}

// compareRunToBaseline compares a run against a baseline.
func (c *Comparator) compareRunToBaseline(run *domain.BenchmarkRun, baseline *domain.Baseline) *domain.ComparisonResult {
	result := &domain.ComparisonResult{
		RunID:      run.ID,
		BaselineID: baseline.ID,
	}

	// Compare metrics
	if run.Results != nil && run.Results.Summary != nil && baseline.Metrics != nil {
		result.Metrics = c.calculateMetricsComparison(baseline.Metrics, run.Results.Summary)
	}

	// Check thresholds
	if baseline.Thresholds != nil && result.Metrics != nil {
		result.Violations = c.checkThresholds(result.Metrics, baseline.Thresholds)
	}

	// Determine verdict
	result.Pass = len(result.Violations) == 0
	if result.Pass {
		result.Verdict = "pass"
	} else {
		// Check if any violations are errors (not just warnings)
		hasError := false
		for _, v := range result.Violations {
			if v.Severity == "error" {
				hasError = true
				break
			}
		}
		if hasError {
			result.Verdict = "fail"
		} else {
			result.Verdict = "warning"
		}
	}

	// Compare environments
	if run.Environment != nil && baseline.Environment != nil {
		envMatch, envDiffs := c.checkEnvironmentConstraints(run.Environment, baseline.Environment)
		result.EnvironmentMatch = envMatch
		result.EnvironmentDiffs = envDiffs
	} else {
		result.EnvironmentMatch = true
	}

	return result
}

// calculateMetricsComparison calculates the comparison metrics between baseline and run.
func (c *Comparator) calculateMetricsComparison(baseline, run *domain.SummaryMetrics) *domain.MetricsComparison {
	return &domain.MetricsComparison{
		ThroughputChange: percentChange(baseline.OpsPerSecond, run.OpsPerSecond),
		AvgLatencyChange: percentChange(baseline.AvgLatencyMs, run.AvgLatencyMs),
		P50LatencyChange: percentChange(baseline.P50LatencyMs, run.P50LatencyMs),
		P99LatencyChange: percentChange(baseline.P99LatencyMs, run.P99LatencyMs),
		ErrorRateChange:  percentChange(baseline.ErrorRate, run.ErrorRate),
	}
}

// checkThresholds checks if any thresholds are violated.
func (c *Comparator) checkThresholds(metrics *domain.MetricsComparison, thresholds *domain.BaselineThresholds) []domain.ThresholdViolation {
	var violations []domain.ThresholdViolation

	// Check throughput regression (negative change is bad)
	if thresholds.MaxThroughputRegression > 0 && metrics.ThroughputChange < -thresholds.MaxThroughputRegression*100 {
		violations = append(violations, domain.ThresholdViolation{
			Metric:    "throughput",
			Threshold: -thresholds.MaxThroughputRegression * 100,
			Actual:    metrics.ThroughputChange,
			Severity:  "error",
		})
	}

	// Check latency regression (positive change is bad for latency)
	if thresholds.MaxLatencyRegression > 0 && metrics.AvgLatencyChange > thresholds.MaxLatencyRegression*100 {
		violations = append(violations, domain.ThresholdViolation{
			Metric:    "avg_latency",
			Threshold: thresholds.MaxLatencyRegression * 100,
			Actual:    metrics.AvgLatencyChange,
			Severity:  "error",
		})
	}

	// Check P99 regression
	if thresholds.MaxP99Regression > 0 && metrics.P99LatencyChange > thresholds.MaxP99Regression*100 {
		violations = append(violations, domain.ThresholdViolation{
			Metric:    "p99_latency",
			Threshold: thresholds.MaxP99Regression * 100,
			Actual:    metrics.P99LatencyChange,
			Severity:  "error",
		})
	}

	// Check error rate increase
	if thresholds.MaxErrorRateIncrease > 0 && metrics.ErrorRateChange > thresholds.MaxErrorRateIncrease*100 {
		violations = append(violations, domain.ThresholdViolation{
			Metric:    "error_rate",
			Threshold: thresholds.MaxErrorRateIncrease * 100,
			Actual:    metrics.ErrorRateChange,
			Severity:  "error",
		})
	}

	return violations
}

// checkEnvironmentConstraints checks if a run's environment matches baseline constraints.
func (c *Comparator) checkEnvironmentConstraints(env *domain.Environment, constraints *domain.EnvironmentConstraints) (bool, []string) {
	// For now, just return true - environment constraints matching
	// would require storing the baseline's environment info
	return true, nil
}

// percentChange calculates the percentage change from old to new value.
// Returns positive for increase, negative for decrease.
func percentChange(old, new float64) float64 {
	if old == 0 {
		if new == 0 {
			return 0
		}
		return 100 // infinite increase
	}
	return ((new - old) / old) * 100
}

// determineBetterRun decides which run performed better based on metrics.
func determineBetterRun(metrics *MetricsDiff) string {
	if metrics == nil {
		return "unknown"
	}

	// Score based on key metrics
	// Higher throughput is better, lower latency is better
	score1 := 0
	score2 := 0

	// Throughput (higher is better)
	if metrics.OpsPerSecond1 > metrics.OpsPerSecond2*1.01 { // 1% threshold
		score1++
	} else if metrics.OpsPerSecond2 > metrics.OpsPerSecond1*1.01 {
		score2++
	}

	// P99 Latency (lower is better)
	if metrics.P99Latency1 < metrics.P99Latency2*0.99 {
		score1++
	} else if metrics.P99Latency2 < metrics.P99Latency1*0.99 {
		score2++
	}

	// Average Latency (lower is better)
	if metrics.AvgLatency1 < metrics.AvgLatency2*0.99 {
		score1++
	} else if metrics.AvgLatency2 < metrics.AvgLatency1*0.99 {
		score2++
	}

	// Error rate (lower is better)
	if metrics.ErrorRate1 < metrics.ErrorRate2 {
		score1++
	} else if metrics.ErrorRate2 < metrics.ErrorRate1 {
		score2++
	}

	if score1 > score2 {
		return "run1"
	} else if score2 > score1 {
		return "run2"
	}
	return "equal"
}

// Abs returns the absolute value of a float64.
func Abs(x float64) float64 {
	return math.Abs(x)
}
