package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
)

func TestMetricsCollector_RegisterDefaultMetrics(t *testing.T) {
	collector := NewMetricsCollector()

	// Check that default metrics are registered
	expectedMetrics := []string{
		"redismeter_benchmark_runs_total",
		"redismeter_throughput_ops_per_second",
		"redismeter_latency_avg_ms",
		"redismeter_latency_p99_ms",
		"redismeter_errors_total",
	}

	for _, name := range expectedMetrics {
		def, ok := collector.registry.Get(name)
		if !ok {
			t.Errorf("Expected metric %s to be registered", name)
			continue
		}
		if def.Name != name {
			t.Errorf("Expected metric name %s, got %s", name, def.Name)
		}
	}
}

func TestMetricsCollector_CollectFromRun(t *testing.T) {
	collector := NewMetricsCollector()

	run := &domain.BenchmarkRun{
		ID: "test-run-1",
		Workload: &domain.Workload{
			Name:    "test-workload",
			Clients: 10,
		},
		StartTime: time.Now().Add(-30 * time.Second),
		EndTime:   time.Now(),
		Results: &domain.Results{
			Summary: &domain.SummaryMetrics{
				OpsPerSecond:  50000,
				AvgLatencyMs:  1.5,
				P50LatencyMs:  1.0,
				P90LatencyMs:  3.0,
				P95LatencyMs:  5.0,
				P99LatencyMs:  10.0,
				MinLatencyMs:  0.5,
				MaxLatencyMs:  20.0,
				Errors:        5,
				ErrorRate:     0.0001,
			},
		},
	}

	collector.CollectFromRun(run)

	metrics := collector.GetAllMetrics()
	if len(metrics) == 0 {
		t.Error("Expected metrics to be collected")
	}

	// Check specific metrics
	foundThroughput := false
	for _, m := range metrics {
		if m.Name == "redismeter_throughput_ops_per_second" {
			foundThroughput = true
			if m.Value != 50000 {
				t.Errorf("Expected throughput 50000, got %f", m.Value)
			}
			if m.Labels["workload"] != "test-workload" {
				t.Errorf("Expected workload label 'test-workload', got %s", m.Labels["workload"])
			}
		}
	}

	if !foundThroughput {
		t.Error("Expected throughput metric to be collected")
	}
}

func TestMetricsCollector_NilResults(t *testing.T) {
	collector := NewMetricsCollector()

	// Should not panic
	collector.CollectFromRun(nil)

	run := &domain.BenchmarkRun{
		ID: "test-run-1",
	}
	collector.CollectFromRun(run)

	metrics := collector.GetAllMetrics()
	if len(metrics) != 0 {
		t.Error("Expected no metrics for nil results")
	}
}

func TestPrometheusExporter_Handler(t *testing.T) {
	collector := NewMetricsCollector()

	run := &domain.BenchmarkRun{
		ID: "test-run-1",
		Workload: &domain.Workload{
			Name: "test-workload",
		},
		StartTime: time.Now().Add(-30 * time.Second),
		EndTime:   time.Now(),
		Results: &domain.Results{
			Summary: &domain.SummaryMetrics{
				OpsPerSecond:  50000,
				AvgLatencyMs:  1.5,
				P99LatencyMs:  10.0,
				ErrorRate:     0.0001,
			},
		},
	}

	collector.CollectFromRun(run)

	exporter := NewPrometheusExporter(collector)
	handler := exporter.Handler()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("Expected text/plain content type, got %s", contentType)
	}

	body := w.Body.String()

	// Check for expected metric lines
	if !strings.Contains(body, "# HELP redismeter_throughput_ops_per_second") {
		t.Error("Expected HELP line for throughput metric")
	}
	if !strings.Contains(body, "# TYPE redismeter_throughput_ops_per_second gauge") {
		t.Error("Expected TYPE line for throughput metric")
	}
	if !strings.Contains(body, "redismeter_throughput_ops_per_second{") {
		t.Error("Expected throughput metric line")
	}
}

func TestPrometheusExporter_FormatMetricLine(t *testing.T) {
	collector := NewMetricsCollector()
	exporter := NewPrometheusExporter(collector)

	tests := []struct {
		name     string
		metric   *Metric
		expected string
	}{
		{
			name: "no labels",
			metric: &Metric{
				Name:  "test_metric",
				Value: 42.5,
			},
			expected: "test_metric 42.5",
		},
		{
			name: "with labels",
			metric: &Metric{
				Name:  "test_metric",
				Value: 100,
				Labels: map[string]string{
					"foo": "bar",
					"baz": "qux",
				},
			},
			expected: `test_metric{baz="qux",foo="bar"} 100`,
		},
		{
			name: "label with special characters",
			metric: &Metric{
				Name:  "test_metric",
				Value: 1,
				Labels: map[string]string{
					"path": "/api/v1",
				},
			},
			expected: `test_metric{path="/api/v1"} 1`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exporter.formatMetricLine(tt.metric)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestMetricsServer_Config(t *testing.T) {
	config := DefaultMetricsServerConfig()

	if config.Address != ":9090" {
		t.Errorf("Expected default address :9090, got %s", config.Address)
	}
	if config.Path != "/metrics" {
		t.Errorf("Expected default path /metrics, got %s", config.Path)
	}
}

func TestMetricsServer_Collector(t *testing.T) {
	config := DefaultMetricsServerConfig()
	server := NewMetricsServer(config)

	collector := server.Collector()
	if collector == nil {
		t.Error("Expected non-nil collector")
	}
}

func TestMetricsServer_Shutdown(t *testing.T) {
	config := MetricsServerConfig{
		Address: ":0", // Use random port
		Path:    "/metrics",
	}
	server := NewMetricsServer(config)

	// Start server in background
	go func() {
		server.Start()
	}()

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Shutdown should work
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := server.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

func TestMetricType(t *testing.T) {
	if MetricTypeCounter != "counter" {
		t.Errorf("Expected counter, got %s", MetricTypeCounter)
	}
	if MetricTypeGauge != "gauge" {
		t.Errorf("Expected gauge, got %s", MetricTypeGauge)
	}
	if MetricTypeHistogram != "histogram" {
		t.Errorf("Expected histogram, got %s", MetricTypeHistogram)
	}
	if MetricTypeSummary != "summary" {
		t.Errorf("Expected summary, got %s", MetricTypeSummary)
	}
}

func TestMetricsRegistry(t *testing.T) {
	registry := NewMetricsRegistry()

	def := MetricDefinition{
		Name:   "test_metric",
		Help:   "A test metric",
		Type:   MetricTypeGauge,
		Labels: []string{"label1", "label2"},
	}

	registry.Register(def)

	// Get should work
	retrieved, ok := registry.Get("test_metric")
	if !ok {
		t.Error("Expected to find registered metric")
	}
	if retrieved.Name != "test_metric" {
		t.Errorf("Expected name 'test_metric', got %s", retrieved.Name)
	}
	if retrieved.Help != "A test metric" {
		t.Errorf("Expected help 'A test metric', got %s", retrieved.Help)
	}

	// Get non-existent should fail
	_, ok = registry.Get("nonexistent")
	if ok {
		t.Error("Expected not to find nonexistent metric")
	}
}
