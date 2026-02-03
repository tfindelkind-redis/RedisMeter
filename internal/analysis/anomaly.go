// Package analysis provides performance analysis and anomaly detection.
package analysis

import (
	"context"
	"math"
	"sort"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
)

// AnomalyDetector detects anomalies in benchmark results.
type AnomalyDetector struct {
	config AnomalyConfig
}

// AnomalyConfig configures anomaly detection parameters.
type AnomalyConfig struct {
	// ZScoreThreshold for statistical outlier detection (default: 3.0)
	ZScoreThreshold float64 `json:"z_score_threshold"`

	// IQRMultiplier for IQR-based outlier detection (default: 1.5)
	IQRMultiplier float64 `json:"iqr_multiplier"`

	// MovingAverageWindow for trend detection (default: 5)
	MovingAverageWindow int `json:"moving_average_window"`

	// DeviationThreshold percentage for moving average deviation (default: 20.0)
	DeviationThreshold float64 `json:"deviation_threshold"`

	// MinDataPoints required for detection (default: 3)
	MinDataPoints int `json:"min_data_points"`
}

// DefaultAnomalyConfig returns sensible defaults.
func DefaultAnomalyConfig() AnomalyConfig {
	return AnomalyConfig{
		ZScoreThreshold:     3.0,
		IQRMultiplier:       1.5,
		MovingAverageWindow: 5,
		DeviationThreshold:  20.0,
		MinDataPoints:       3,
	}
}

// NewAnomalyDetector creates a new anomaly detector with the given config.
func NewAnomalyDetector(config AnomalyConfig) *AnomalyDetector {
	if config.ZScoreThreshold == 0 {
		config.ZScoreThreshold = 3.0
	}
	if config.IQRMultiplier == 0 {
		config.IQRMultiplier = 1.5
	}
	if config.MovingAverageWindow == 0 {
		config.MovingAverageWindow = 5
	}
	if config.DeviationThreshold == 0 {
		config.DeviationThreshold = 20.0
	}
	if config.MinDataPoints == 0 {
		config.MinDataPoints = 3
	}
	return &AnomalyDetector{config: config}
}

// Anomaly represents a detected anomaly.
type Anomaly struct {
	// RunID of the anomalous run
	RunID string `json:"run_id"`

	// Metric that triggered the anomaly
	Metric string `json:"metric"`

	// Value that was detected as anomalous
	Value float64 `json:"value"`

	// Expected value (e.g., mean or moving average)
	Expected float64 `json:"expected"`

	// Deviation from expected (percentage)
	Deviation float64 `json:"deviation"`

	// Method used to detect the anomaly
	Method string `json:"method"`

	// Severity: "warning", "critical"
	Severity string `json:"severity"`

	// Description of the anomaly
	Description string `json:"description"`

	// Score indicating how anomalous (higher = more anomalous)
	Score float64 `json:"score"`
}

// AnomalyReport contains all detected anomalies for a set of runs.
type AnomalyReport struct {
	// Anomalies detected
	Anomalies []Anomaly `json:"anomalies"`

	// Statistics used for detection
	Statistics map[string]*MetricStatistics `json:"statistics"`

	// Summary
	TotalRuns      int `json:"total_runs"`
	AnomalousRuns  int `json:"anomalous_runs"`
	WarningCount   int `json:"warning_count"`
	CriticalCount  int `json:"critical_count"`
}

// MetricStatistics contains statistics for a single metric.
type MetricStatistics struct {
	Mean       float64   `json:"mean"`
	Median     float64   `json:"median"`
	StdDev     float64   `json:"std_dev"`
	Min        float64   `json:"min"`
	Max        float64   `json:"max"`
	Q1         float64   `json:"q1"`
	Q3         float64   `json:"q3"`
	IQR        float64   `json:"iqr"`
	LowerFence float64   `json:"lower_fence"`
	UpperFence float64   `json:"upper_fence"`
	Values     []float64 `json:"values,omitempty"`
}

// DetectAnomalies analyzes a set of runs and detects anomalies.
func (d *AnomalyDetector) DetectAnomalies(ctx context.Context, runs []*domain.BenchmarkRun) (*AnomalyReport, error) {
	report := &AnomalyReport{
		Anomalies:  []Anomaly{},
		Statistics: make(map[string]*MetricStatistics),
		TotalRuns:  len(runs),
	}

	if len(runs) < d.config.MinDataPoints {
		return report, nil
	}

	// Extract metrics from runs
	throughputs := make([]float64, 0, len(runs))
	avgLatencies := make([]float64, 0, len(runs))
	p99Latencies := make([]float64, 0, len(runs))
	runIDs := make([]string, 0, len(runs))

	for _, run := range runs {
		if run.Results == nil || run.Results.Summary == nil {
			continue
		}
		throughputs = append(throughputs, run.Results.Summary.OpsPerSecond)
		avgLatencies = append(avgLatencies, run.Results.Summary.AvgLatencyMs)
		p99Latencies = append(p99Latencies, run.Results.Summary.P99LatencyMs)
		runIDs = append(runIDs, run.ID)
	}

	// Calculate statistics
	report.Statistics["throughput"] = d.calculateStatistics(throughputs)
	report.Statistics["avg_latency"] = d.calculateStatistics(avgLatencies)
	report.Statistics["p99_latency"] = d.calculateStatistics(p99Latencies)

	// Detect anomalies using multiple methods
	anomalySet := make(map[string]bool) // Track unique anomalies

	// Z-Score detection
	d.detectZScoreAnomalies(runIDs, throughputs, "throughput", report, anomalySet, true)
	d.detectZScoreAnomalies(runIDs, avgLatencies, "avg_latency", report, anomalySet, false)
	d.detectZScoreAnomalies(runIDs, p99Latencies, "p99_latency", report, anomalySet, false)

	// IQR detection
	d.detectIQRAnomalies(runIDs, throughputs, "throughput", report, anomalySet, true)
	d.detectIQRAnomalies(runIDs, avgLatencies, "avg_latency", report, anomalySet, false)
	d.detectIQRAnomalies(runIDs, p99Latencies, "p99_latency", report, anomalySet, false)

	// Moving average deviation detection
	if len(runs) >= d.config.MovingAverageWindow {
		d.detectMovingAverageAnomalies(runIDs, throughputs, "throughput", report, anomalySet, true)
		d.detectMovingAverageAnomalies(runIDs, avgLatencies, "avg_latency", report, anomalySet, false)
		d.detectMovingAverageAnomalies(runIDs, p99Latencies, "p99_latency", report, anomalySet, false)
	}

	// Count anomalous runs and severity
	anomalousRunIDs := make(map[string]bool)
	for _, a := range report.Anomalies {
		anomalousRunIDs[a.RunID] = true
		if a.Severity == "critical" {
			report.CriticalCount++
		} else {
			report.WarningCount++
		}
	}
	report.AnomalousRuns = len(anomalousRunIDs)

	return report, nil
}

// DetectSingleRunAnomaly checks if a single run is anomalous compared to historical data.
func (d *AnomalyDetector) DetectSingleRunAnomaly(ctx context.Context, run *domain.BenchmarkRun, historicalRuns []*domain.BenchmarkRun) ([]Anomaly, error) {
	if run.Results == nil || run.Results.Summary == nil {
		return nil, nil
	}

	if len(historicalRuns) < d.config.MinDataPoints {
		return nil, nil
	}

	anomalies := []Anomaly{}

	// Extract historical metrics
	throughputs := make([]float64, 0, len(historicalRuns))
	avgLatencies := make([]float64, 0, len(historicalRuns))
	p99Latencies := make([]float64, 0, len(historicalRuns))

	for _, hr := range historicalRuns {
		if hr.Results == nil || hr.Results.Summary == nil {
			continue
		}
		throughputs = append(throughputs, hr.Results.Summary.OpsPerSecond)
		avgLatencies = append(avgLatencies, hr.Results.Summary.AvgLatencyMs)
		p99Latencies = append(p99Latencies, hr.Results.Summary.P99LatencyMs)
	}

	// Check throughput
	if a := d.checkValueAnomaly(run.ID, run.Results.Summary.OpsPerSecond, throughputs, "throughput", true); a != nil {
		anomalies = append(anomalies, *a)
	}

	// Check avg latency
	if a := d.checkValueAnomaly(run.ID, run.Results.Summary.AvgLatencyMs, avgLatencies, "avg_latency", false); a != nil {
		anomalies = append(anomalies, *a)
	}

	// Check p99 latency
	if a := d.checkValueAnomaly(run.ID, run.Results.Summary.P99LatencyMs, p99Latencies, "p99_latency", false); a != nil {
		anomalies = append(anomalies, *a)
	}

	return anomalies, nil
}

func (d *AnomalyDetector) checkValueAnomaly(runID string, value float64, historical []float64, metric string, higherIsBetter bool) *Anomaly {
	stats := d.calculateStatistics(historical)

	// Z-score check
	if stats.StdDev > 0 {
		zScore := (value - stats.Mean) / stats.StdDev
		
		isAnomaly := false
		severity := "warning"
		
		if higherIsBetter {
			// For throughput, low values are bad
			if zScore < -d.config.ZScoreThreshold {
				isAnomaly = true
				if zScore < -d.config.ZScoreThreshold*1.5 {
					severity = "critical"
				}
			}
		} else {
			// For latency, high values are bad
			if zScore > d.config.ZScoreThreshold {
				isAnomaly = true
				if zScore > d.config.ZScoreThreshold*1.5 {
					severity = "critical"
				}
			}
		}

		if isAnomaly {
			deviation := ((value - stats.Mean) / stats.Mean) * 100
			return &Anomaly{
				RunID:       runID,
				Metric:      metric,
				Value:       value,
				Expected:    stats.Mean,
				Deviation:   deviation,
				Method:      "z_score",
				Severity:    severity,
				Score:       math.Abs(zScore),
				Description: formatAnomalyDescription(metric, value, stats.Mean, deviation, higherIsBetter),
			}
		}
	}

	// IQR check
	if value < stats.LowerFence || value > stats.UpperFence {
		deviation := ((value - stats.Mean) / stats.Mean) * 100
		severity := "warning"
		
		// More extreme outliers are critical
		extremeLowerFence := stats.Q1 - 3*stats.IQR
		extremeUpperFence := stats.Q3 + 3*stats.IQR
		if value < extremeLowerFence || value > extremeUpperFence {
			severity = "critical"
		}

		score := 0.0
		if value < stats.LowerFence {
			score = (stats.LowerFence - value) / stats.IQR
		} else {
			score = (value - stats.UpperFence) / stats.IQR
		}

		return &Anomaly{
			RunID:       runID,
			Metric:      metric,
			Value:       value,
			Expected:    stats.Median,
			Deviation:   deviation,
			Method:      "iqr",
			Severity:    severity,
			Score:       score,
			Description: formatAnomalyDescription(metric, value, stats.Median, deviation, higherIsBetter),
		}
	}

	return nil
}

func (d *AnomalyDetector) calculateStatistics(values []float64) *MetricStatistics {
	if len(values) == 0 {
		return &MetricStatistics{}
	}

	stats := &MetricStatistics{
		Values: values,
	}

	// Sort for percentile calculations
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	// Basic statistics
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	stats.Mean = sum / float64(len(values))

	// Standard deviation
	sumSq := 0.0
	for _, v := range values {
		sumSq += (v - stats.Mean) * (v - stats.Mean)
	}
	stats.StdDev = math.Sqrt(sumSq / float64(len(values)))

	// Min/Max
	stats.Min = sorted[0]
	stats.Max = sorted[len(sorted)-1]

	// Median
	n := len(sorted)
	if n%2 == 0 {
		stats.Median = (sorted[n/2-1] + sorted[n/2]) / 2
	} else {
		stats.Median = sorted[n/2]
	}

	// Quartiles
	stats.Q1 = percentile(sorted, 25)
	stats.Q3 = percentile(sorted, 75)
	stats.IQR = stats.Q3 - stats.Q1

	// Fences for IQR-based outlier detection
	stats.LowerFence = stats.Q1 - d.config.IQRMultiplier*stats.IQR
	stats.UpperFence = stats.Q3 + d.config.IQRMultiplier*stats.IQR

	return stats
}

func (d *AnomalyDetector) detectZScoreAnomalies(runIDs []string, values []float64, metric string, report *AnomalyReport, seen map[string]bool, higherIsBetter bool) {
	stats := report.Statistics[metric]
	if stats.StdDev == 0 {
		return
	}

	for i, v := range values {
		key := runIDs[i] + ":" + metric
		if seen[key] {
			continue
		}

		zScore := (v - stats.Mean) / stats.StdDev
		isAnomaly := false
		severity := "warning"

		if higherIsBetter {
			if zScore < -d.config.ZScoreThreshold {
				isAnomaly = true
				if zScore < -d.config.ZScoreThreshold*1.5 {
					severity = "critical"
				}
			}
		} else {
			if zScore > d.config.ZScoreThreshold {
				isAnomaly = true
				if zScore > d.config.ZScoreThreshold*1.5 {
					severity = "critical"
				}
			}
		}

		if isAnomaly {
			deviation := ((v - stats.Mean) / stats.Mean) * 100
			report.Anomalies = append(report.Anomalies, Anomaly{
				RunID:       runIDs[i],
				Metric:      metric,
				Value:       v,
				Expected:    stats.Mean,
				Deviation:   deviation,
				Method:      "z_score",
				Severity:    severity,
				Score:       math.Abs(zScore),
				Description: formatAnomalyDescription(metric, v, stats.Mean, deviation, higherIsBetter),
			})
			seen[key] = true
		}
	}
}

func (d *AnomalyDetector) detectIQRAnomalies(runIDs []string, values []float64, metric string, report *AnomalyReport, seen map[string]bool, higherIsBetter bool) {
	stats := report.Statistics[metric]

	for i, v := range values {
		key := runIDs[i] + ":" + metric
		if seen[key] {
			continue
		}

		if v < stats.LowerFence || v > stats.UpperFence {
			deviation := ((v - stats.Mean) / stats.Mean) * 100
			severity := "warning"

			extremeLowerFence := stats.Q1 - 3*stats.IQR
			extremeUpperFence := stats.Q3 + 3*stats.IQR
			if v < extremeLowerFence || v > extremeUpperFence {
				severity = "critical"
			}

			score := 0.0
			if v < stats.LowerFence {
				score = (stats.LowerFence - v) / stats.IQR
			} else {
				score = (v - stats.UpperFence) / stats.IQR
			}

			report.Anomalies = append(report.Anomalies, Anomaly{
				RunID:       runIDs[i],
				Metric:      metric,
				Value:       v,
				Expected:    stats.Median,
				Deviation:   deviation,
				Method:      "iqr",
				Severity:    severity,
				Score:       score,
				Description: formatAnomalyDescription(metric, v, stats.Median, deviation, higherIsBetter),
			})
			seen[key] = true
		}
	}
}

func (d *AnomalyDetector) detectMovingAverageAnomalies(runIDs []string, values []float64, metric string, report *AnomalyReport, seen map[string]bool, higherIsBetter bool) {
	window := d.config.MovingAverageWindow

	for i := window; i < len(values); i++ {
		key := runIDs[i] + ":" + metric
		if seen[key] {
			continue
		}

		// Calculate moving average of previous values
		sum := 0.0
		for j := i - window; j < i; j++ {
			sum += values[j]
		}
		ma := sum / float64(window)

		// Check deviation
		deviation := ((values[i] - ma) / ma) * 100
		isAnomaly := false
		severity := "warning"

		if higherIsBetter {
			if deviation < -d.config.DeviationThreshold {
				isAnomaly = true
				if deviation < -d.config.DeviationThreshold*2 {
					severity = "critical"
				}
			}
		} else {
			if deviation > d.config.DeviationThreshold {
				isAnomaly = true
				if deviation > d.config.DeviationThreshold*2 {
					severity = "critical"
				}
			}
		}

		if isAnomaly {
			report.Anomalies = append(report.Anomalies, Anomaly{
				RunID:       runIDs[i],
				Metric:      metric,
				Value:       values[i],
				Expected:    ma,
				Deviation:   deviation,
				Method:      "moving_average",
				Severity:    severity,
				Score:       math.Abs(deviation) / d.config.DeviationThreshold,
				Description: formatAnomalyDescription(metric, values[i], ma, deviation, higherIsBetter),
			})
			seen[key] = true
		}
	}
}

func formatAnomalyDescription(metric string, value, expected, deviation float64, higherIsBetter bool) string {
	direction := "higher"
	quality := "worse"
	
	if deviation < 0 {
		direction = "lower"
	}
	
	if (higherIsBetter && deviation > 0) || (!higherIsBetter && deviation < 0) {
		quality = "better"
	}
	
	metricName := metric
	switch metric {
	case "throughput":
		metricName = "Throughput"
	case "avg_latency":
		metricName = "Average latency"
	case "p99_latency":
		metricName = "P99 latency"
	}

	return formatDescription(metricName, value, expected, direction, math.Abs(deviation), quality)
}

func formatDescription(metric string, value, expected float64, direction string, deviation float64, quality string) string {
	return metric + " is " + direction + " than expected (" +
		formatFloat(value) + " vs " + formatFloat(expected) + 
		", " + formatFloat(deviation) + "% " + direction + ", " + quality + ")"
}

func formatFloat(v float64) string {
	if v >= 1000 {
		return formatWithCommas(v)
	}
	return formatPrecise(v)
}

func formatWithCommas(v float64) string {
	s := ""
	n := int64(v)
	for n > 0 {
		if s != "" {
			s = "," + s
		}
		part := n % 1000
		n = n / 1000
		if n > 0 {
			s = padLeft(part) + s
		} else {
			s = intToString(part) + s
		}
	}
	if s == "" {
		s = "0"
	}
	return s
}

func padLeft(n int64) string {
	s := intToString(n)
	for len(s) < 3 {
		s = "0" + s
	}
	return s
}

func intToString(n int64) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n = n / 10
	}
	return s
}

func formatPrecise(v float64) string {
	// Simple float formatting
	if v == float64(int64(v)) {
		return intToString(int64(v))
	}
	// Round to 3 decimal places
	rounded := math.Round(v*1000) / 1000
	intPart := int64(rounded)
	fracPart := int64(math.Round((rounded - float64(intPart)) * 1000))
	if fracPart == 0 {
		return intToString(intPart)
	}
	// Remove trailing zeros
	for fracPart%10 == 0 && fracPart > 0 {
		fracPart = fracPart / 10
	}
	return intToString(intPart) + "." + intToString(fracPart)
}
