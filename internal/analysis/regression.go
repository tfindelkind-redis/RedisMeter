// Package analysis provides performance analysis and comparison capabilities.
package analysis

import (
	"context"
	"fmt"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
)

// RegressionAnalyzer checks for performance regressions compared to baselines.
type RegressionAnalyzer struct {
	thresholds RegressionThresholds
}

// RegressionThresholds defines thresholds for regression detection.
type RegressionThresholds struct {
	ThroughputMinOps     float64 // Minimum acceptable ops/sec
	LatencyMaxAvg        float64 // Maximum acceptable avg latency (ms)
	LatencyMaxP99        float64 // Maximum acceptable P99 latency (ms)
	LatencyMaxP999       float64 // Maximum acceptable P999 latency (ms)
	ErrorRateMax         float64 // Maximum acceptable error rate (as fraction)
	ThroughputVariation  float64 // Warning threshold for throughput variation (CoV)
	LatencyVariation     float64 // Warning threshold for latency variation
}

// DefaultRegressionThresholds returns sensible default thresholds.
func DefaultRegressionThresholds() RegressionThresholds {
	return RegressionThresholds{
		ThroughputMinOps:     1000,    // At least 1000 ops/sec
		LatencyMaxAvg:        10,      // Max 10ms average latency
		LatencyMaxP99:        50,      // Max 50ms P99 latency
		LatencyMaxP999:       100,     // Max 100ms P999 latency
		ErrorRateMax:         0.001,   // Max 0.1% error rate
		ThroughputVariation:  10,      // Warning if CoV > 10%
		LatencyVariation:     20,      // Warning if latency CoV > 20%
	}
}

// NewRegressionAnalyzer creates a new regression analyzer with custom thresholds.
func NewRegressionAnalyzer(thresholds RegressionThresholds) *RegressionAnalyzer {
	return &RegressionAnalyzer{thresholds: thresholds}
}

// NewDefaultRegressionAnalyzer creates a regression analyzer with default thresholds.
func NewDefaultRegressionAnalyzer() *RegressionAnalyzer {
	return NewRegressionAnalyzer(DefaultRegressionThresholds())
}

// Name returns the analyzer name.
func (a *RegressionAnalyzer) Name() string {
	return "regression"
}

// Description returns the analyzer description.
func (a *RegressionAnalyzer) Description() string {
	return "Analyzes benchmark results for performance issues and potential regressions"
}

// Analyze performs regression analysis on a single run.
func (a *RegressionAnalyzer) Analyze(ctx context.Context, run *domain.BenchmarkRun) (*AnalysisReport, error) {
	if run.Results == nil || run.Results.Summary == nil {
		return nil, fmt.Errorf("run has no results")
	}

	report := &AnalysisReport{
		Analyzer: a.Name(),
		Score:    100,
		Status:   "good",
		Metrics:  make(map[string]interface{}),
	}

	summary := run.Results.Summary
	var findings []Finding
	var recommendations []Recommendation

	// Analyze throughput
	if summary.OpsPerSecond < a.thresholds.ThroughputMinOps {
		report.Score -= 20
		findings = append(findings, Finding{
			Category:    "throughput",
			Severity:    "error",
			Title:       "Low Throughput",
			Description: fmt.Sprintf("Throughput (%.0f ops/s) is below minimum threshold (%.0f ops/s)", summary.OpsPerSecond, a.thresholds.ThroughputMinOps),
			Metric:      "ops_per_second",
			Value:       summary.OpsPerSecond,
			Expected:    a.thresholds.ThroughputMinOps,
		})
		recommendations = append(recommendations, Recommendation{
			Priority:    "high",
			Title:       "Improve Throughput",
			Description: "Consider increasing the number of clients, threads, or pipeline depth",
			Impact:      "Could significantly improve operations per second",
		})
	}

	// Analyze average latency
	if summary.AvgLatencyMs > a.thresholds.LatencyMaxAvg {
		report.Score -= 15
		findings = append(findings, Finding{
			Category:    "latency",
			Severity:    "warning",
			Title:       "High Average Latency",
			Description: fmt.Sprintf("Average latency (%.3f ms) exceeds threshold (%.3f ms)", summary.AvgLatencyMs, a.thresholds.LatencyMaxAvg),
			Metric:      "avg_latency_ms",
			Value:       summary.AvgLatencyMs,
			Expected:    a.thresholds.LatencyMaxAvg,
		})
	}

	// Analyze P99 latency
	if summary.P99LatencyMs > a.thresholds.LatencyMaxP99 {
		report.Score -= 20
		findings = append(findings, Finding{
			Category:    "latency",
			Severity:    "error",
			Title:       "High P99 Latency",
			Description: fmt.Sprintf("P99 latency (%.3f ms) exceeds threshold (%.3f ms)", summary.P99LatencyMs, a.thresholds.LatencyMaxP99),
			Metric:      "p99_latency_ms",
			Value:       summary.P99LatencyMs,
			Expected:    a.thresholds.LatencyMaxP99,
		})
		recommendations = append(recommendations, Recommendation{
			Priority:    "high",
			Title:       "Reduce Tail Latency",
			Description: "Investigate slow operations, consider Redis configuration tuning, or check for network issues",
			Impact:      "Improves user experience for worst-case scenarios",
		})
	}

	// Analyze P999 latency
	if summary.P999LatencyMs > a.thresholds.LatencyMaxP999 {
		report.Score -= 10
		findings = append(findings, Finding{
			Category:    "latency",
			Severity:    "warning",
			Title:       "High P999 Latency",
			Description: fmt.Sprintf("P999 latency (%.3f ms) exceeds threshold (%.3f ms)", summary.P999LatencyMs, a.thresholds.LatencyMaxP999),
			Metric:      "p999_latency_ms",
			Value:       summary.P999LatencyMs,
			Expected:    a.thresholds.LatencyMaxP999,
		})
	}

	// Analyze error rate
	if summary.ErrorRate > a.thresholds.ErrorRateMax {
		report.Score -= 25
		findings = append(findings, Finding{
			Category:    "reliability",
			Severity:    "error",
			Title:       "High Error Rate",
			Description: fmt.Sprintf("Error rate (%.4f%%) exceeds threshold (%.4f%%)", summary.ErrorRate*100, a.thresholds.ErrorRateMax*100),
			Metric:      "error_rate",
			Value:       summary.ErrorRate,
			Expected:    a.thresholds.ErrorRateMax,
		})
		recommendations = append(recommendations, Recommendation{
			Priority:    "high",
			Title:       "Investigate Errors",
			Description: "Check Redis server logs, connection issues, and memory usage",
			Impact:      "Critical for reliability and data integrity",
		})
	}

	// Check latency distribution (P99/P50 ratio)
	if summary.P50LatencyMs > 0 {
		latencyRatio := summary.P99LatencyMs / summary.P50LatencyMs
		report.Metrics["latency_ratio_p99_p50"] = latencyRatio

		if latencyRatio > 5 {
			findings = append(findings, Finding{
				Category:    "latency",
				Severity:    "warning",
				Title:       "Wide Latency Distribution",
				Description: fmt.Sprintf("P99/P50 ratio (%.1fx) indicates inconsistent performance", latencyRatio),
				Metric:      "latency_ratio",
				Value:       latencyRatio,
			})
			recommendations = append(recommendations, Recommendation{
				Priority:    "medium",
				Title:       "Investigate Latency Spikes",
				Description: "Look for periodic operations (BGSAVE, AOF rewrite), slow queries, or resource contention",
				Impact:      "More predictable response times",
			})
		}
	}

	// If no issues found, add a positive finding
	if len(findings) == 0 {
		findings = append(findings, Finding{
			Category:    "overall",
			Severity:    "info",
			Title:       "Good Performance",
			Description: "All metrics are within acceptable thresholds",
		})
	}

	// Determine overall status
	if report.Score >= 80 {
		report.Status = "good"
		report.Summary = "Benchmark results are within acceptable ranges"
	} else if report.Score >= 50 {
		report.Status = "warning"
		report.Summary = "Some performance metrics need attention"
	} else {
		report.Status = "critical"
		report.Summary = "Significant performance issues detected"
	}

	report.Findings = findings
	report.Recommendations = recommendations
	report.Metrics["ops_per_second"] = summary.OpsPerSecond
	report.Metrics["avg_latency_ms"] = summary.AvgLatencyMs
	report.Metrics["p99_latency_ms"] = summary.P99LatencyMs

	return report, nil
}

// AnalyzeMultiple performs trend analysis across multiple runs.
func (a *RegressionAnalyzer) AnalyzeMultiple(ctx context.Context, runs []*domain.BenchmarkRun) (*AnalysisReport, error) {
	if len(runs) < 2 {
		return nil, fmt.Errorf("need at least 2 runs for trend analysis")
	}

	report := &AnalysisReport{
		Analyzer: a.Name() + "-trend",
		Score:    100,
		Status:   "good",
		Summary:  fmt.Sprintf("Trend analysis of %d runs", len(runs)),
		Metrics:  make(map[string]interface{}),
	}

	// Collect metrics
	var throughputs, latencies, p99s []float64
	for _, run := range runs {
		if run.Results != nil && run.Results.Summary != nil {
			throughputs = append(throughputs, run.Results.Summary.OpsPerSecond)
			latencies = append(latencies, run.Results.Summary.AvgLatencyMs)
			p99s = append(p99s, run.Results.Summary.P99LatencyMs)
		}
	}

	var findings []Finding

	// Analyze throughput trend
	if len(throughputs) >= 2 {
		stats := CalculateStatistics(throughputs)
		report.Metrics["throughput_mean"] = stats.Mean
		report.Metrics["throughput_std_dev"] = stats.StdDev
		report.Metrics["throughput_coeff_var"] = stats.CoeffVar

		if stats.CoeffVar > a.thresholds.ThroughputVariation {
			report.Score -= 15
			findings = append(findings, Finding{
				Category:    "stability",
				Severity:    "warning",
				Title:       "Unstable Throughput",
				Description: fmt.Sprintf("Throughput coefficient of variation (%.1f%%) indicates inconsistent performance", stats.CoeffVar),
				Metric:      "throughput_cov",
				Value:       stats.CoeffVar,
				Expected:    a.thresholds.ThroughputVariation,
			})
		}

		// Check for regression trend (compare first half to second half)
		if len(throughputs) >= 4 {
			mid := len(throughputs) / 2
			firstHalf := throughputs[:mid]
			secondHalf := throughputs[mid:]
			ttest := TwoSampleTTest(firstHalf, secondHalf)

			if ttest.Significant && ttest.MeanDiff < 0 {
				// Performance degraded significantly
				report.Score -= 20
				findings = append(findings, Finding{
					Category:    "regression",
					Severity:    "error",
					Title:       "Throughput Regression Detected",
					Description: fmt.Sprintf("Recent runs show %.1f%% lower throughput (statistically significant)", -ttest.MeanDiff/mean(firstHalf)*100),
					Metric:      "throughput_trend",
					Value:       ttest.MeanDiff,
				})
			}
		}
	}

	// Analyze latency trend
	if len(latencies) >= 2 {
		stats := CalculateStatistics(latencies)
		report.Metrics["latency_mean"] = stats.Mean
		report.Metrics["latency_std_dev"] = stats.StdDev
		report.Metrics["latency_coeff_var"] = stats.CoeffVar

		if stats.CoeffVar > a.thresholds.LatencyVariation {
			findings = append(findings, Finding{
				Category:    "stability",
				Severity:    "warning",
				Title:       "Unstable Latency",
				Description: fmt.Sprintf("Latency coefficient of variation (%.1f%%) indicates inconsistent performance", stats.CoeffVar),
				Metric:      "latency_cov",
				Value:       stats.CoeffVar,
			})
		}
	}

	// Determine status
	if report.Score >= 80 {
		report.Status = "good"
	} else if report.Score >= 50 {
		report.Status = "warning"
	} else {
		report.Status = "critical"
	}

	if len(findings) == 0 {
		findings = append(findings, Finding{
			Category:    "overall",
			Severity:    "info",
			Title:       "Stable Performance",
			Description: "No significant performance trends or regressions detected",
		})
	}

	report.Findings = findings
	return report, nil
}

// LatencyAnalyzer focuses on latency distribution analysis.
type LatencyAnalyzer struct{}

// NewLatencyAnalyzer creates a new latency analyzer.
func NewLatencyAnalyzer() *LatencyAnalyzer {
	return &LatencyAnalyzer{}
}

// Name returns the analyzer name.
func (a *LatencyAnalyzer) Name() string {
	return "latency"
}

// Description returns the analyzer description.
func (a *LatencyAnalyzer) Description() string {
	return "Analyzes latency distribution and identifies outliers"
}

// Analyze performs latency analysis on a single run.
func (a *LatencyAnalyzer) Analyze(ctx context.Context, run *domain.BenchmarkRun) (*AnalysisReport, error) {
	if run.Results == nil || run.Results.Summary == nil {
		return nil, fmt.Errorf("run has no results")
	}

	report := &AnalysisReport{
		Analyzer: a.Name(),
		Score:    100,
		Status:   "good",
		Metrics:  make(map[string]interface{}),
	}

	summary := run.Results.Summary
	var findings []Finding

	// Calculate key metrics
	report.Metrics["avg_latency"] = summary.AvgLatencyMs
	report.Metrics["p50_latency"] = summary.P50LatencyMs
	report.Metrics["p99_latency"] = summary.P99LatencyMs
	report.Metrics["p999_latency"] = summary.P999LatencyMs

	// P99 to P50 ratio analysis
	if summary.P50LatencyMs > 0 {
		ratio := summary.P99LatencyMs / summary.P50LatencyMs
		report.Metrics["p99_p50_ratio"] = ratio

		if ratio < 2 {
			findings = append(findings, Finding{
				Category:    "distribution",
				Severity:    "info",
				Title:       "Tight Latency Distribution",
				Description: fmt.Sprintf("P99/P50 ratio of %.2fx indicates very consistent latency", ratio),
			})
		} else if ratio < 5 {
			findings = append(findings, Finding{
				Category:    "distribution",
				Severity:    "info",
				Title:       "Normal Latency Distribution",
				Description: fmt.Sprintf("P99/P50 ratio of %.2fx is within normal range", ratio),
			})
		} else if ratio < 10 {
			report.Score -= 10
			findings = append(findings, Finding{
				Category:    "distribution",
				Severity:    "warning",
				Title:       "Wide Latency Distribution",
				Description: fmt.Sprintf("P99/P50 ratio of %.2fx suggests occasional slow operations", ratio),
			})
		} else {
			report.Score -= 20
			findings = append(findings, Finding{
				Category:    "distribution",
				Severity:    "error",
				Title:       "Very Wide Latency Distribution",
				Description: fmt.Sprintf("P99/P50 ratio of %.2fx indicates significant outliers", ratio),
			})
		}
	}

	// P999 to P99 ratio (tail analysis)
	if summary.P99LatencyMs > 0 {
		tailRatio := summary.P999LatencyMs / summary.P99LatencyMs
		report.Metrics["p999_p99_ratio"] = tailRatio

		if tailRatio > 3 {
			report.Score -= 10
			findings = append(findings, Finding{
				Category:    "tail",
				Severity:    "warning",
				Title:       "Long Tail Latency",
				Description: fmt.Sprintf("P999/P99 ratio of %.2fx indicates extreme outliers", tailRatio),
			})
		}
	}

	// Overall assessment
	if report.Score >= 80 {
		report.Status = "good"
		report.Summary = "Latency distribution looks healthy"
	} else if report.Score >= 50 {
		report.Status = "warning"
		report.Summary = "Some latency distribution concerns"
	} else {
		report.Status = "critical"
		report.Summary = "Significant latency distribution issues"
	}

	report.Findings = findings
	return report, nil
}

// AnalyzeMultiple is not implemented for latency analyzer.
func (a *LatencyAnalyzer) AnalyzeMultiple(ctx context.Context, runs []*domain.BenchmarkRun) (*AnalysisReport, error) {
	return nil, fmt.Errorf("multi-run analysis not supported for latency analyzer")
}

// ThroughputAnalyzer focuses on throughput stability analysis.
type ThroughputAnalyzer struct{}

// NewThroughputAnalyzer creates a new throughput analyzer.
func NewThroughputAnalyzer() *ThroughputAnalyzer {
	return &ThroughputAnalyzer{}
}

// Name returns the analyzer name.
func (a *ThroughputAnalyzer) Name() string {
	return "throughput"
}

// Description returns the analyzer description.
func (a *ThroughputAnalyzer) Description() string {
	return "Analyzes throughput stability and identifies bottlenecks"
}

// Analyze performs throughput analysis.
func (a *ThroughputAnalyzer) Analyze(ctx context.Context, run *domain.BenchmarkRun) (*AnalysisReport, error) {
	if run.Results == nil || run.Results.Summary == nil {
		return nil, fmt.Errorf("run has no results")
	}

	report := &AnalysisReport{
		Analyzer: a.Name(),
		Score:    100,
		Status:   "good",
		Summary:  "Throughput analysis complete",
		Metrics:  make(map[string]interface{}),
	}

	summary := run.Results.Summary
	var findings []Finding

	report.Metrics["ops_per_second"] = summary.OpsPerSecond
	report.Metrics["total_ops"] = summary.TotalOps

	// Categorize throughput level
	if summary.OpsPerSecond > 100000 {
		findings = append(findings, Finding{
			Category:    "performance",
			Severity:    "info",
			Title:       "Excellent Throughput",
			Description: fmt.Sprintf("%.0f ops/s is excellent performance", summary.OpsPerSecond),
		})
	} else if summary.OpsPerSecond > 10000 {
		findings = append(findings, Finding{
			Category:    "performance",
			Severity:    "info",
			Title:       "Good Throughput",
			Description: fmt.Sprintf("%.0f ops/s is good performance", summary.OpsPerSecond),
		})
	} else if summary.OpsPerSecond > 1000 {
		findings = append(findings, Finding{
			Category:    "performance",
			Severity:    "warning",
			Title:       "Moderate Throughput",
			Description: fmt.Sprintf("%.0f ops/s may be acceptable depending on requirements", summary.OpsPerSecond),
		})
		report.Score -= 10
	} else {
		findings = append(findings, Finding{
			Category:    "performance",
			Severity:    "error",
			Title:       "Low Throughput",
			Description: fmt.Sprintf("%.0f ops/s is relatively low", summary.OpsPerSecond),
		})
		report.Score -= 25
	}

	report.Findings = findings

	if report.Score >= 80 {
		report.Status = "good"
	} else if report.Score >= 50 {
		report.Status = "warning"
	} else {
		report.Status = "critical"
	}

	return report, nil
}

// AnalyzeMultiple is not implemented for throughput analyzer.
func (a *ThroughputAnalyzer) AnalyzeMultiple(ctx context.Context, runs []*domain.BenchmarkRun) (*AnalysisReport, error) {
	return nil, fmt.Errorf("multi-run analysis not supported - use regression analyzer")
}
