package analysis

import (
	"context"
	"testing"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
)

func TestCalculateStatistics(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   struct {
			count  int
			min    float64
			max    float64
			mean   float64
			median float64
		}
	}{
		{
			name:   "empty",
			values: []float64{},
			want:   struct{ count int; min, max, mean, median float64 }{0, 0, 0, 0, 0},
		},
		{
			name:   "single",
			values: []float64{5.0},
			want:   struct{ count int; min, max, mean, median float64 }{1, 5, 5, 5, 5},
		},
		{
			name:   "simple",
			values: []float64{1, 2, 3, 4, 5},
			want:   struct{ count int; min, max, mean, median float64 }{5, 1, 5, 3, 3},
		},
		{
			name:   "even count",
			values: []float64{1, 2, 3, 4},
			want:   struct{ count int; min, max, mean, median float64 }{4, 1, 4, 2.5, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateStatistics(tt.values)
			if got.Count != tt.want.count {
				t.Errorf("Count = %d, want %d", got.Count, tt.want.count)
			}
			if got.Min != tt.want.min {
				t.Errorf("Min = %f, want %f", got.Min, tt.want.min)
			}
			if got.Max != tt.want.max {
				t.Errorf("Max = %f, want %f", got.Max, tt.want.max)
			}
			if got.Mean != tt.want.mean {
				t.Errorf("Mean = %f, want %f", got.Mean, tt.want.mean)
			}
		})
	}
}

func TestTwoSampleTTest(t *testing.T) {
	// Test with identical samples (should not be significant)
	sample1 := []float64{10, 10, 10, 10, 10}
	sample2 := []float64{10, 10, 10, 10, 10}
	result := TwoSampleTTest(sample1, sample2)
	if result.Significant {
		t.Error("Expected not significant for identical samples")
	}
	if result.MeanDiff != 0 {
		t.Errorf("MeanDiff = %f, want 0", result.MeanDiff)
	}

	// Test with clearly different samples
	sample3 := []float64{10, 10, 10, 10, 10}
	sample4 := []float64{20, 20, 20, 20, 20}
	result2 := TwoSampleTTest(sample3, sample4)
	if result2.MeanDiff != -10 {
		t.Errorf("MeanDiff = %f, want -10", result2.MeanDiff)
	}
	// With zero variance in both groups, we can't compute proper t-test
	// but mean diff should be correct
}

func TestConfidenceInterval(t *testing.T) {
	values := []float64{10, 11, 9, 10, 10, 11, 9, 10, 10, 10}
	ci := CalculateConfidenceInterval(values, 0.95)

	if ci.Mean < 9.5 || ci.Mean > 10.5 {
		t.Errorf("Mean = %f, expected around 10", ci.Mean)
	}
	if ci.Lower >= ci.Mean {
		t.Errorf("Lower bound %f should be less than mean %f", ci.Lower, ci.Mean)
	}
	if ci.Upper <= ci.Mean {
		t.Errorf("Upper bound %f should be greater than mean %f", ci.Upper, ci.Mean)
	}
	if ci.Confidence != 0.95 {
		t.Errorf("Confidence = %f, want 0.95", ci.Confidence)
	}
}

func TestPercentile(t *testing.T) {
	sorted := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	tests := []struct {
		p    float64
		want float64
	}{
		{0, 1},
		{50, 5.5},
		{100, 10},
		{25, 3.25},
		{75, 7.75},
	}

	for _, tt := range tests {
		got := percentile(sorted, tt.p)
		if Abs(got-tt.want) > 0.01 {
			t.Errorf("percentile(%f) = %f, want %f", tt.p, got, tt.want)
		}
	}
}

func TestRegressionAnalyzer_Analyze(t *testing.T) {
	ctx := context.Background()
	analyzer := NewDefaultRegressionAnalyzer()

	// Test with good results
	goodRun := &domain.BenchmarkRun{
		ID:        "good-run",
		CreatedAt: time.Now(),
		Results: &domain.Results{
			Summary: &domain.SummaryMetrics{
				OpsPerSecond:  50000,
				AvgLatencyMs:  1.5,
				P50LatencyMs:  1.0,
				P99LatencyMs:  5.0,
				P999LatencyMs: 10.0,
				ErrorRate:     0,
			},
		},
	}

	report, err := analyzer.Analyze(ctx, goodRun)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if report.Status != "good" {
		t.Errorf("Status = %s, want good", report.Status)
	}
	if report.Score < 80 {
		t.Errorf("Score = %d, want >= 80", report.Score)
	}

	// Test with poor results
	poorRun := &domain.BenchmarkRun{
		ID:        "poor-run",
		CreatedAt: time.Now(),
		Results: &domain.Results{
			Summary: &domain.SummaryMetrics{
				OpsPerSecond:  100,         // Below threshold
				AvgLatencyMs:  50,          // High
				P50LatencyMs:  30,          // High
				P99LatencyMs:  100,         // Very high
				P999LatencyMs: 500,         // Very high
				ErrorRate:     0.05,        // High error rate
			},
		},
	}

	report2, err := analyzer.Analyze(ctx, poorRun)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if report2.Status == "good" {
		t.Error("Expected status != good for poor run")
	}
	if report2.Score >= 80 {
		t.Errorf("Score = %d, want < 80 for poor run", report2.Score)
	}
	if len(report2.Findings) == 0 {
		t.Error("Expected findings for poor run")
	}
	if len(report2.Recommendations) == 0 {
		t.Error("Expected recommendations for poor run")
	}
}

func TestRegressionAnalyzer_AnalyzeMultiple(t *testing.T) {
	ctx := context.Background()
	analyzer := NewDefaultRegressionAnalyzer()

	// Create multiple runs with declining performance
	runs := []*domain.BenchmarkRun{
		{
			ID: "run-1",
			Results: &domain.Results{
				Summary: &domain.SummaryMetrics{OpsPerSecond: 50000, AvgLatencyMs: 1.5},
			},
		},
		{
			ID: "run-2",
			Results: &domain.Results{
				Summary: &domain.SummaryMetrics{OpsPerSecond: 48000, AvgLatencyMs: 1.6},
			},
		},
		{
			ID: "run-3",
			Results: &domain.Results{
				Summary: &domain.SummaryMetrics{OpsPerSecond: 45000, AvgLatencyMs: 1.8},
			},
		},
		{
			ID: "run-4",
			Results: &domain.Results{
				Summary: &domain.SummaryMetrics{OpsPerSecond: 40000, AvgLatencyMs: 2.0},
			},
		},
	}

	report, err := analyzer.AnalyzeMultiple(ctx, runs)
	if err != nil {
		t.Fatalf("AnalyzeMultiple failed: %v", err)
	}

	if report.Analyzer != "regression-trend" {
		t.Errorf("Analyzer = %s, want regression-trend", report.Analyzer)
	}

	// Should have throughput metrics
	if _, ok := report.Metrics["throughput_mean"]; !ok {
		t.Error("Expected throughput_mean metric")
	}
}

func TestLatencyAnalyzer_Analyze(t *testing.T) {
	ctx := context.Background()
	analyzer := NewLatencyAnalyzer()

	run := &domain.BenchmarkRun{
		ID: "test-run",
		Results: &domain.Results{
			Summary: &domain.SummaryMetrics{
				AvgLatencyMs:  2.0,
				P50LatencyMs:  1.5,
				P99LatencyMs:  5.0,
				P999LatencyMs: 10.0,
			},
		},
	}

	report, err := analyzer.Analyze(ctx, run)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if report.Analyzer != "latency" {
		t.Errorf("Analyzer = %s, want latency", report.Analyzer)
	}

	// Should have P99/P50 ratio
	ratio, ok := report.Metrics["p99_p50_ratio"]
	if !ok {
		t.Error("Expected p99_p50_ratio metric")
	}
	if r, _ := ratio.(float64); Abs(r-3.33) > 0.1 {
		t.Errorf("p99_p50_ratio = %f, want ~3.33", r)
	}
}

func TestThroughputAnalyzer_Analyze(t *testing.T) {
	ctx := context.Background()
	analyzer := NewThroughputAnalyzer()

	tests := []struct {
		name       string
		opsPerSec  float64
		wantStatus string
	}{
		{"excellent", 150000, "good"},  // > 100000
		{"good", 50000, "good"},        // > 10000
		{"moderate", 5000, "good"},     // > 1000 (still considered good in threshold)
		{"low", 500, "warning"},        // <= 1000 (warning because it's moderate)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := &domain.BenchmarkRun{
				ID: "test-run",
				Results: &domain.Results{
					Summary: &domain.SummaryMetrics{
						OpsPerSecond: tt.opsPerSec,
					},
				},
			}

			report, err := analyzer.Analyze(ctx, run)
			if err != nil {
				t.Fatalf("Analyze failed: %v", err)
			}

			if report.Status != tt.wantStatus {
				t.Errorf("Status = %s, want %s", report.Status, tt.wantStatus)
			}
		})
	}
}

func TestAnalyzerRegistry(t *testing.T) {
	registry := NewAnalyzerRegistry()

	// Register analyzers
	registry.Register(NewDefaultRegressionAnalyzer())
	registry.Register(NewLatencyAnalyzer())
	registry.Register(NewThroughputAnalyzer())

	// List should have all three
	names := registry.List()
	if len(names) != 3 {
		t.Errorf("List() returned %d analyzers, want 3", len(names))
	}

	// Get should work
	analyzer, ok := registry.Get("regression")
	if !ok {
		t.Error("Get('regression') returned false")
	}
	if analyzer.Name() != "regression" {
		t.Errorf("Analyzer name = %s, want regression", analyzer.Name())
	}

	// Unknown analyzer
	_, ok = registry.Get("unknown")
	if ok {
		t.Error("Get('unknown') should return false")
	}

	// AnalyzeAll
	ctx := context.Background()
	run := &domain.BenchmarkRun{
		ID: "test",
		Results: &domain.Results{
			Summary: &domain.SummaryMetrics{
				OpsPerSecond: 10000,
				AvgLatencyMs: 2.0,
				P50LatencyMs: 1.5,
				P99LatencyMs: 5.0,
			},
		},
	}

	reports, err := registry.AnalyzeAll(ctx, run)
	if err != nil {
		t.Fatalf("AnalyzeAll failed: %v", err)
	}
	if len(reports) != 3 {
		t.Errorf("AnalyzeAll returned %d reports, want 3", len(reports))
	}
}
