// Package observability provides metrics export and observability integration.
package observability

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
)

// MetricType represents the type of metric.
type MetricType string

const (
	MetricTypeCounter   MetricType = "counter"
	MetricTypeGauge     MetricType = "gauge"
	MetricTypeHistogram MetricType = "histogram"
	MetricTypeSummary   MetricType = "summary"
)

// Metric represents a single metric.
type Metric struct {
	Name        string            `json:"name"`
	Help        string            `json:"help"`
	Type        MetricType        `json:"type"`
	Value       float64           `json:"value"`
	Labels      map[string]string `json:"labels"`
	Timestamp   time.Time         `json:"timestamp"`
	Buckets     map[float64]int64 `json:"buckets,omitempty"`     // For histograms
	Quantiles   map[float64]float64 `json:"quantiles,omitempty"` // For summaries
	Count       int64             `json:"count,omitempty"`
	Sum         float64           `json:"sum,omitempty"`
}

// MetricsCollector collects metrics from benchmark runs.
type MetricsCollector struct {
	mu       sync.RWMutex
	metrics  map[string]*Metric
	registry *MetricsRegistry
}

// MetricsRegistry holds metric definitions.
type MetricsRegistry struct {
	mu          sync.RWMutex
	definitions map[string]MetricDefinition
}

// MetricDefinition defines a metric.
type MetricDefinition struct {
	Name   string
	Help   string
	Type   MetricType
	Labels []string
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector() *MetricsCollector {
	mc := &MetricsCollector{
		metrics:  make(map[string]*Metric),
		registry: NewMetricsRegistry(),
	}
	mc.registerDefaultMetrics()
	return mc
}

// NewMetricsRegistry creates a new metrics registry.
func NewMetricsRegistry() *MetricsRegistry {
	return &MetricsRegistry{
		definitions: make(map[string]MetricDefinition),
	}
}

func (c *MetricsCollector) registerDefaultMetrics() {
	// Benchmark metrics
	c.registry.Register(MetricDefinition{
		Name:   "redismeter_benchmark_runs_total",
		Help:   "Total number of benchmark runs",
		Type:   MetricTypeCounter,
		Labels: []string{"workload", "status"},
	})

	c.registry.Register(MetricDefinition{
		Name:   "redismeter_benchmark_duration_seconds",
		Help:   "Benchmark duration in seconds",
		Type:   MetricTypeGauge,
		Labels: []string{"workload", "run_id"},
	})

	c.registry.Register(MetricDefinition{
		Name:   "redismeter_throughput_ops_per_second",
		Help:   "Benchmark throughput in operations per second",
		Type:   MetricTypeGauge,
		Labels: []string{"workload", "run_id"},
	})

	// Latency metrics
	c.registry.Register(MetricDefinition{
		Name:   "redismeter_latency_avg_ms",
		Help:   "Average latency in milliseconds",
		Type:   MetricTypeGauge,
		Labels: []string{"workload", "run_id"},
	})

	c.registry.Register(MetricDefinition{
		Name:   "redismeter_latency_p50_ms",
		Help:   "50th percentile latency in milliseconds",
		Type:   MetricTypeGauge,
		Labels: []string{"workload", "run_id"},
	})

	c.registry.Register(MetricDefinition{
		Name:   "redismeter_latency_p90_ms",
		Help:   "90th percentile latency in milliseconds",
		Type:   MetricTypeGauge,
		Labels: []string{"workload", "run_id"},
	})

	c.registry.Register(MetricDefinition{
		Name:   "redismeter_latency_p95_ms",
		Help:   "95th percentile latency in milliseconds",
		Type:   MetricTypeGauge,
		Labels: []string{"workload", "run_id"},
	})

	c.registry.Register(MetricDefinition{
		Name:   "redismeter_latency_p99_ms",
		Help:   "99th percentile latency in milliseconds",
		Type:   MetricTypeGauge,
		Labels: []string{"workload", "run_id"},
	})

	c.registry.Register(MetricDefinition{
		Name:   "redismeter_latency_min_ms",
		Help:   "Minimum latency in milliseconds",
		Type:   MetricTypeGauge,
		Labels: []string{"workload", "run_id"},
	})

	c.registry.Register(MetricDefinition{
		Name:   "redismeter_latency_max_ms",
		Help:   "Maximum latency in milliseconds",
		Type:   MetricTypeGauge,
		Labels: []string{"workload", "run_id"},
	})

	// Error metrics
	c.registry.Register(MetricDefinition{
		Name:   "redismeter_errors_total",
		Help:   "Total number of errors",
		Type:   MetricTypeCounter,
		Labels: []string{"workload", "run_id"},
	})

	c.registry.Register(MetricDefinition{
		Name:   "redismeter_error_rate",
		Help:   "Error rate (0-1)",
		Type:   MetricTypeGauge,
		Labels: []string{"workload", "run_id"},
	})

	// Connection metrics
	c.registry.Register(MetricDefinition{
		Name:   "redismeter_connections",
		Help:   "Number of connections used",
		Type:   MetricTypeGauge,
		Labels: []string{"workload", "run_id"},
	})

	// Anomaly metrics
	c.registry.Register(MetricDefinition{
		Name:   "redismeter_anomalies_detected_total",
		Help:   "Total number of anomalies detected",
		Type:   MetricTypeCounter,
		Labels: []string{"workload", "severity"},
	})
}

// Register registers a metric definition.
func (r *MetricsRegistry) Register(def MetricDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.definitions[def.Name] = def
}

// Get retrieves a metric definition.
func (r *MetricsRegistry) Get(name string) (MetricDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.definitions[name]
	return def, ok
}

// CollectFromRun collects metrics from a benchmark run.
func (c *MetricsCollector) CollectFromRun(run *domain.BenchmarkRun) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if run == nil || run.Results == nil || run.Results.Summary == nil {
		return
	}

	workload := ""
	if run.Workload != nil {
		workload = run.Workload.Name
	}
	runID := run.ID
	summary := run.Results.Summary
	now := time.Now()

	// Throughput
	c.setMetric("redismeter_throughput_ops_per_second", summary.OpsPerSecond, map[string]string{
		"workload": workload,
		"run_id":   runID,
	}, now)

	// Latencies
	c.setMetric("redismeter_latency_avg_ms", summary.AvgLatencyMs, map[string]string{
		"workload": workload,
		"run_id":   runID,
	}, now)

	c.setMetric("redismeter_latency_p50_ms", summary.P50LatencyMs, map[string]string{
		"workload": workload,
		"run_id":   runID,
	}, now)

	c.setMetric("redismeter_latency_p90_ms", summary.P90LatencyMs, map[string]string{
		"workload": workload,
		"run_id":   runID,
	}, now)

	c.setMetric("redismeter_latency_p95_ms", summary.P95LatencyMs, map[string]string{
		"workload": workload,
		"run_id":   runID,
	}, now)

	c.setMetric("redismeter_latency_p99_ms", summary.P99LatencyMs, map[string]string{
		"workload": workload,
		"run_id":   runID,
	}, now)

	c.setMetric("redismeter_latency_min_ms", summary.MinLatencyMs, map[string]string{
		"workload": workload,
		"run_id":   runID,
	}, now)

	c.setMetric("redismeter_latency_max_ms", summary.MaxLatencyMs, map[string]string{
		"workload": workload,
		"run_id":   runID,
	}, now)

	// Errors
	c.setMetric("redismeter_errors_total", float64(summary.Errors), map[string]string{
		"workload": workload,
		"run_id":   runID,
	}, now)

	c.setMetric("redismeter_error_rate", summary.ErrorRate, map[string]string{
		"workload": workload,
		"run_id":   runID,
	}, now)

	// Connections - use Workload.Clients if available
	connections := 0
	if run.Workload != nil {
		connections = run.Workload.Clients
	}
	c.setMetric("redismeter_connections", float64(connections), map[string]string{
		"workload": workload,
		"run_id":   runID,
	}, now)

	// Duration
	duration := run.EndTime.Sub(run.StartTime).Seconds()
	c.setMetric("redismeter_benchmark_duration_seconds", duration, map[string]string{
		"workload": workload,
		"run_id":   runID,
	}, now)

	// Increment runs counter
	c.incrementMetric("redismeter_benchmark_runs_total", map[string]string{
		"workload": workload,
		"status":   string(run.Status),
	})
}

func (c *MetricsCollector) setMetric(name string, value float64, labels map[string]string, timestamp time.Time) {
	key := c.metricKey(name, labels)
	def, ok := c.registry.Get(name)
	if !ok {
		return
	}

	c.metrics[key] = &Metric{
		Name:      name,
		Help:      def.Help,
		Type:      def.Type,
		Value:     value,
		Labels:    labels,
		Timestamp: timestamp,
	}
}

func (c *MetricsCollector) incrementMetric(name string, labels map[string]string) {
	key := c.metricKey(name, labels)
	def, ok := c.registry.Get(name)
	if !ok {
		return
	}

	if m, exists := c.metrics[key]; exists {
		m.Value++
		m.Timestamp = time.Now()
	} else {
		c.metrics[key] = &Metric{
			Name:      name,
			Help:      def.Help,
			Type:      def.Type,
			Value:     1,
			Labels:    labels,
			Timestamp: time.Now(),
		}
	}
}

func (c *MetricsCollector) metricKey(name string, labels map[string]string) string {
	var sb strings.Builder
	sb.WriteString(name)

	// Sort labels for consistent keys
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		sb.WriteString("|")
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(labels[k])
	}

	return sb.String()
}

// GetAllMetrics returns all collected metrics.
func (c *MetricsCollector) GetAllMetrics() []*Metric {
	c.mu.RLock()
	defer c.mu.RUnlock()

	metrics := make([]*Metric, 0, len(c.metrics))
	for _, m := range c.metrics {
		metrics = append(metrics, m)
	}
	return metrics
}

// PrometheusExporter exports metrics in Prometheus format.
type PrometheusExporter struct {
	collector *MetricsCollector
}

// NewPrometheusExporter creates a new Prometheus exporter.
func NewPrometheusExporter(collector *MetricsCollector) *PrometheusExporter {
	return &PrometheusExporter{
		collector: collector,
	}
}

// Handler returns an HTTP handler for /metrics endpoint.
func (e *PrometheusExporter) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metrics := e.collector.GetAllMetrics()
		output := e.formatMetrics(metrics)

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, output)
	})
}

func (e *PrometheusExporter) formatMetrics(metrics []*Metric) string {
	var sb strings.Builder

	// Group by metric name
	grouped := make(map[string][]*Metric)
	for _, m := range metrics {
		grouped[m.Name] = append(grouped[m.Name], m)
	}

	// Sort metric names
	names := make([]string, 0, len(grouped))
	for name := range grouped {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		metricGroup := grouped[name]
		if len(metricGroup) == 0 {
			continue
		}

		first := metricGroup[0]

		// HELP line
		sb.WriteString(fmt.Sprintf("# HELP %s %s\n", name, first.Help))

		// TYPE line
		sb.WriteString(fmt.Sprintf("# TYPE %s %s\n", name, first.Type))

		// Metric lines
		for _, m := range metricGroup {
			sb.WriteString(e.formatMetricLine(m))
			sb.WriteString("\n")
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

func (e *PrometheusExporter) formatMetricLine(m *Metric) string {
	if len(m.Labels) == 0 {
		return fmt.Sprintf("%s %g", m.Name, m.Value)
	}

	// Format labels
	var labels []string
	keys := make([]string, 0, len(m.Labels))
	for k := range m.Labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := m.Labels[k]
		// Escape label values
		v = strings.ReplaceAll(v, "\\", "\\\\")
		v = strings.ReplaceAll(v, "\"", "\\\"")
		v = strings.ReplaceAll(v, "\n", "\\n")
		labels = append(labels, fmt.Sprintf("%s=\"%s\"", k, v))
	}

	return fmt.Sprintf("%s{%s} %g", m.Name, strings.Join(labels, ","), m.Value)
}

// MetricsServer serves metrics over HTTP.
type MetricsServer struct {
	collector *MetricsCollector
	exporter  *PrometheusExporter
	server    *http.Server
}

// MetricsServerConfig configures the metrics server.
type MetricsServerConfig struct {
	Address string `json:"address"`
	Path    string `json:"path"`
}

// DefaultMetricsServerConfig returns sensible defaults.
func DefaultMetricsServerConfig() MetricsServerConfig {
	return MetricsServerConfig{
		Address: ":9090",
		Path:    "/metrics",
	}
}

// NewMetricsServer creates a new metrics server.
func NewMetricsServer(config MetricsServerConfig) *MetricsServer {
	collector := NewMetricsCollector()
	exporter := NewPrometheusExporter(collector)

	mux := http.NewServeMux()
	mux.Handle(config.Path, exporter.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	return &MetricsServer{
		collector: collector,
		exporter:  exporter,
		server: &http.Server{
			Addr:         config.Address,
			Handler:      mux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
	}
}

// Collector returns the metrics collector.
func (s *MetricsServer) Collector() *MetricsCollector {
	return s.collector
}

// Start starts the metrics server.
func (s *MetricsServer) Start() error {
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *MetricsServer) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// PushGatewayClient pushes metrics to a Prometheus Pushgateway.
type PushGatewayClient struct {
	url       string
	job       string
	collector *MetricsCollector
	client    *http.Client
}

// PushGatewayConfig configures the push gateway client.
type PushGatewayConfig struct {
	URL string `json:"url"`
	Job string `json:"job"`
}

// NewPushGatewayClient creates a new push gateway client.
func NewPushGatewayClient(config PushGatewayConfig, collector *MetricsCollector) *PushGatewayClient {
	return &PushGatewayClient{
		url:       config.URL,
		job:       config.Job,
		collector: collector,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Push pushes metrics to the gateway.
func (c *PushGatewayClient) Push(ctx context.Context) error {
	exporter := NewPrometheusExporter(c.collector)
	metrics := c.collector.GetAllMetrics()
	body := exporter.formatMetrics(metrics)

	url := fmt.Sprintf("%s/metrics/job/%s", strings.TrimSuffix(c.url, "/"), c.job)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain; version=0.0.4")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("push metrics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("pushgateway returned %d", resp.StatusCode)
	}

	return nil
}

// Delete deletes metrics from the gateway.
func (c *PushGatewayClient) Delete(ctx context.Context) error {
	url := fmt.Sprintf("%s/metrics/job/%s", strings.TrimSuffix(c.url, "/"), c.job)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("delete metrics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("pushgateway returned %d", resp.StatusCode)
	}

	return nil
}
