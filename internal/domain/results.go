// Package domain contains core business entities for RedisMeter.
package domain

// Results contains the benchmark results and metrics.
type Results struct {
	// Summary metrics
	Summary *SummaryMetrics `json:"summary"`
	
	// Detailed metrics per operation type
	ByOperation map[string]*OperationMetrics `json:"by_operation,omitempty"`
	
	// Time series data (if captured)
	TimeSeries []*TimeSeriesPoint `json:"time_series,omitempty"`
	
	// Histogram data
	LatencyHistogram *Histogram `json:"latency_histogram,omitempty"`
	
	// Raw output from memtier (for debugging)
	RawOutput string `json:"raw_output,omitempty"`
}

// SummaryMetrics contains aggregate metrics for the entire run.
type SummaryMetrics struct {
	// Throughput
	TotalRequests    int64   `json:"total_requests"`
	TotalOps         int64   `json:"total_ops"`
	OpsPerSecond     float64 `json:"ops_per_second"`
	BytesPerSecond   float64 `json:"bytes_per_second,omitempty"`
	
	// Latency (in milliseconds)
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
	MinLatencyMs     float64 `json:"min_latency_ms"`
	MaxLatencyMs     float64 `json:"max_latency_ms"`
	
	// Percentiles (in milliseconds)
	P50LatencyMs     float64 `json:"p50_latency_ms"`
	P90LatencyMs     float64 `json:"p90_latency_ms"`
	P95LatencyMs     float64 `json:"p95_latency_ms"`
	P99LatencyMs     float64 `json:"p99_latency_ms"`
	P999LatencyMs    float64 `json:"p999_latency_ms,omitempty"`
	
	// Errors
	Errors           int64   `json:"errors"`
	ErrorRate        float64 `json:"error_rate"`
	
	// Connection stats
	Connections      int     `json:"connections,omitempty"`
}

// OperationMetrics contains metrics for a specific operation type.
type OperationMetrics struct {
	Operation        string  `json:"operation"`
	Count            int64   `json:"count"`
	OpsPerSecond     float64 `json:"ops_per_second"`
	AvgLatencyMs     float64 `json:"avg_latency_ms"`
	P50LatencyMs     float64 `json:"p50_latency_ms"`
	P90LatencyMs     float64 `json:"p90_latency_ms"`
	P95LatencyMs     float64 `json:"p95_latency_ms"`
	P99LatencyMs     float64 `json:"p99_latency_ms"`
}

// TimeSeriesPoint represents metrics at a point in time.
type TimeSeriesPoint struct {
	Timestamp    string  `json:"timestamp"` // RFC3339 format
	OpsPerSecond float64 `json:"ops_per_second"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	P99LatencyMs float64 `json:"p99_latency_ms"`
	Errors       int64   `json:"errors,omitempty"`
}

// Histogram contains latency distribution data.
type Histogram struct {
	Buckets []HistogramBucket `json:"buckets"`
}

// HistogramBucket represents a single bucket in a histogram.
type HistogramBucket struct {
	UpperBoundMs float64 `json:"upper_bound_ms"`
	Count        int64   `json:"count"`
	Cumulative   int64   `json:"cumulative"`
}

// Metrics is used for streaming real-time metrics during execution.
type Metrics struct {
	Timestamp    string  `json:"timestamp"`
	OpsPerSecond float64 `json:"ops_per_second"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	P99LatencyMs float64 `json:"p99_latency_ms"`
	Errors       int64   `json:"errors"`
	Progress     float64 `json:"progress"` // 0.0 to 1.0
}
