package cicd

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
)

func TestDetectEnvironment(t *testing.T) {
	// Save and restore environment
	origGitHub := os.Getenv("GITHUB_ACTIONS")
	origGitLab := os.Getenv("GITLAB_CI")
	origCI := os.Getenv("CI")
	defer func() {
		os.Setenv("GITHUB_ACTIONS", origGitHub)
		os.Setenv("GITLAB_CI", origGitLab)
		os.Setenv("CI", origCI)
	}()

	// Clear all CI vars
	os.Unsetenv("GITHUB_ACTIONS")
	os.Unsetenv("GITLAB_CI")
	os.Unsetenv("JENKINS_URL")
	os.Unsetenv("CIRCLECI")
	os.Unsetenv("TF_BUILD")
	os.Unsetenv("CI")

	// Test local detection
	env := DetectEnvironment()
	if env != EnvironmentLocal {
		t.Errorf("Expected local environment, got %s", env)
	}

	// Test GitHub detection
	os.Setenv("GITHUB_ACTIONS", "true")
	env = DetectEnvironment()
	if env != EnvironmentGitHub {
		t.Errorf("Expected github environment, got %s", env)
	}
	os.Unsetenv("GITHUB_ACTIONS")

	// Test GitLab detection
	os.Setenv("GITLAB_CI", "true")
	env = DetectEnvironment()
	if env != EnvironmentGitLab {
		t.Errorf("Expected gitlab environment, got %s", env)
	}
	os.Unsetenv("GITLAB_CI")

	// Test unknown CI detection
	os.Setenv("CI", "true")
	env = DetectEnvironment()
	if env != EnvironmentUnknown {
		t.Errorf("Expected unknown environment, got %s", env)
	}
}

func TestGateCheck_Operators(t *testing.T) {
	evaluator := NewGateEvaluator()

	tests := []struct {
		name     string
		operator string
		value    float64
		actual   float64
		expected bool
	}{
		{"gt_true", "gt", 100, 150, true},
		{"gt_false", "gt", 100, 50, false},
		{"lt_true", "lt", 100, 50, true},
		{"lt_false", "lt", 100, 150, false},
		{"gte_true_equal", "gte", 100, 100, true},
		{"gte_true_greater", "gte", 100, 150, true},
		{"lte_true_equal", "lte", 100, 100, true},
		{"lte_false", "lte", 100, 150, false},
		{"eq_true", "eq", 100, 100, true},
		{"eq_false", "eq", 100, 101, false},
		{"neq_true", "neq", 100, 101, true},
		{"neq_false", "neq", 100, 100, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluator.compareValue(tt.actual, tt.operator, tt.value)
			if result != tt.expected {
				t.Errorf("Expected %v for %f %s %f, got %v",
					tt.expected, tt.actual, tt.operator, tt.value, result)
			}
		})
	}
}

func TestGateEvaluator_Evaluate(t *testing.T) {
	evaluator := NewGateEvaluator()

	gate := PerformanceGate{
		Name:        "test-gate",
		Description: "Test gate",
		Checks: []GateCheck{
			{
				Metric:      "throughput",
				Operator:    "gte",
				Value:       10000,
				Required:    true,
				Description: "Minimum throughput",
			},
			{
				Metric:      "error_rate",
				Operator:    "lte",
				Value:       0.01,
				Required:    true,
				Description: "Maximum error rate",
			},
		},
	}
	evaluator.AddGate(gate)

	// Test passing run
	run := &domain.BenchmarkRun{
		ID: "test-1",
		Results: &domain.Results{
			Summary: &domain.SummaryMetrics{
				OpsPerSecond: 50000,
				ErrorRate:    0.001,
			},
		},
	}

	results, err := evaluator.Evaluate(context.Background(), run)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if !results[0].Passed {
		t.Error("Expected gate to pass")
	}

	// Test failing run
	run.Results.Summary.OpsPerSecond = 5000 // Below threshold
	results, err = evaluator.Evaluate(context.Background(), run)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if results[0].Passed {
		t.Error("Expected gate to fail")
	}
}

func TestGateEvaluator_AllMetricTypes(t *testing.T) {
	run := &domain.BenchmarkRun{
		ID: "test-1",
		Results: &domain.Results{
			Summary: &domain.SummaryMetrics{
				OpsPerSecond:  50000,
				AvgLatencyMs:  1.5,
				P50LatencyMs:  1.0,
				P90LatencyMs:  3.0,
				P95LatencyMs:  5.0,
				P99LatencyMs:  10.0,
				ErrorRate:     0.001,
				Errors:        100,
			},
		},
	}

	// Test each metric type via gates
	tests := []struct {
		metric   string
		operator string
		value    float64
		pass     bool
	}{
		{"throughput", "gte", 40000, true},
		{"ops_per_second", "lte", 60000, true},
		{"avg_latency", "lte", 2.0, true},
		{"avg_latency_ms", "gte", 1.0, true},
		{"p50_latency", "lte", 2.0, true},
		{"p90_latency", "lte", 5.0, true},
		{"p95_latency", "lte", 10.0, true},
		{"p99_latency", "lte", 15.0, true},
		{"error_rate", "lte", 0.01, true},
		{"errors", "lte", 200, true},
	}

	for _, tt := range tests {
		t.Run(tt.metric, func(t *testing.T) {
			e := NewGateEvaluator()
			e.AddGate(PerformanceGate{
				Name: "test",
				Checks: []GateCheck{{
					Metric:   tt.metric,
					Operator: tt.operator,
					Value:    tt.value,
					Required: true,
				}},
			})
			results, _ := e.Evaluate(context.Background(), run)
			if results[0].Passed != tt.pass {
				t.Errorf("Expected pass=%v for %s %s %f", tt.pass, tt.metric, tt.operator, tt.value)
			}
		})
	}
}

func TestGateEvaluator_NoResults(t *testing.T) {
	evaluator := NewGateEvaluator()

	gate := PerformanceGate{
		Name: "test-gate",
		Checks: []GateCheck{
			{
				Metric:   "throughput",
				Operator: "gte",
				Value:    10000,
				Required: true,
			},
		},
	}
	evaluator.AddGate(gate)

	// Run with no results
	run := &domain.BenchmarkRun{
		ID: "test-1",
	}

	results, err := evaluator.Evaluate(context.Background(), run)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if results[0].Passed {
		t.Error("Expected gate to fail with no results")
	}
}

func TestAllPassed(t *testing.T) {
	gate := &PerformanceGate{Name: "test"}

	// All pass
	results := []GateResult{
		{Gate: gate, Passed: true},
		{Gate: gate, Passed: true},
	}
	if !AllPassed(results) {
		t.Error("Expected all passed")
	}

	// One fails
	results = []GateResult{
		{Gate: gate, Passed: true},
		{Gate: gate, Passed: false},
	}
	if AllPassed(results) {
		t.Error("Expected not all passed")
	}

	// Empty should pass
	results = []GateResult{}
	if !AllPassed(results) {
		t.Error("Expected empty to pass")
	}
}

func TestWritePRComment(t *testing.T) {
	gate := &PerformanceGate{
		Name:        "test-gate",
		Description: "Test gate",
	}

	results := []GateResult{
		{
			Gate:   gate,
			Passed: true,
			CheckResults: []CheckResult{
				{
					Check: GateCheck{
						Description: "Throughput check",
					},
					Passed:      true,
					ActualValue: 50000,
					Message:     "throughput: 50000 >= 10000 ✓",
				},
			},
			Summary: "All checks passed (1/1)",
		},
	}

	run := &domain.BenchmarkRun{
		ID: "test-1",
		Results: &domain.Results{
			Summary: &domain.SummaryMetrics{
				OpsPerSecond: 50000,
				AvgLatencyMs: 1.5,
				P99LatencyMs: 10.0,
				ErrorRate:    0.001,
			},
		},
	}

	comment := WritePRComment(results, run)

	// Check expected content
	if comment == "" {
		t.Error("Expected non-empty comment")
	}
	if len(comment) < 100 {
		t.Error("Expected longer comment")
	}
}

func TestGetCIInfo_Local(t *testing.T) {
	// Save and restore environment
	origGitHub := os.Getenv("GITHUB_ACTIONS")
	defer os.Setenv("GITHUB_ACTIONS", origGitHub)
	os.Unsetenv("GITHUB_ACTIONS")
	os.Unsetenv("GITLAB_CI")
	os.Unsetenv("JENKINS_URL")
	os.Unsetenv("CIRCLECI")
	os.Unsetenv("TF_BUILD")
	os.Unsetenv("CI")

	info := GetCIInfo()
	if info.Environment != EnvironmentLocal {
		t.Errorf("Expected local environment, got %s", info.Environment)
	}
}

func TestGateEvaluator_LoadGates(t *testing.T) {
	evaluator := NewGateEvaluator()

	gates := []PerformanceGate{
		{Name: "gate1", Description: "Gate 1"},
		{Name: "gate2", Description: "Gate 2"},
	}

	evaluator.LoadGates(gates)

	results, err := evaluator.Evaluate(context.Background(), &domain.BenchmarkRun{
		ID: "test",
		Results: &domain.Results{
			Summary: &domain.SummaryMetrics{},
		},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

func TestGateEvaluator_EvaluateWithBaseline(t *testing.T) {
	evaluator := NewGateEvaluator()

	gate := PerformanceGate{
		Name: "baseline-gate",
		Checks: []GateCheck{
			{
				Metric:   "throughput",
				Operator: "gte",
				Value:    10000,
				Required: true,
			},
		},
		Baseline: &BaselineReference{
			Type:         "previous",
			MaxDeviation: 10,
		},
	}
	evaluator.AddGate(gate)

	current := &domain.BenchmarkRun{
		ID:        "current",
		StartTime: time.Now().Add(-30 * time.Second),
		EndTime:   time.Now(),
		Workload:  &domain.Workload{Name: "test"},
		Results: &domain.Results{
			Summary: &domain.SummaryMetrics{
				OpsPerSecond: 50000,
				AvgLatencyMs: 1.5,
				P99LatencyMs: 10.0,
			},
		},
	}

	baseline := &domain.BenchmarkRun{
		ID:        "baseline",
		StartTime: time.Now().Add(-60 * time.Second),
		EndTime:   time.Now().Add(-30 * time.Second),
		Workload:  &domain.Workload{Name: "test"},
		Results: &domain.Results{
			Summary: &domain.SummaryMetrics{
				OpsPerSecond: 48000,
				AvgLatencyMs: 1.6,
				P99LatencyMs: 11.0,
			},
		},
	}

	results, err := evaluator.EvaluateWithBaseline(context.Background(), current, baseline)
	if err != nil {
		t.Fatalf("EvaluateWithBaseline failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	// Current is better than baseline, should pass
	if !results[0].Passed {
		t.Error("Expected gate to pass")
	}
}
