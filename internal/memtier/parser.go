// Package memtier provides integration with memtier_benchmark.
package memtier

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
)

// MemtierOutput represents the JSON output from memtier_benchmark.
type MemtierOutput struct {
	Configuration  map[string]interface{} `json:"configuration"`
	RunInformation map[string]interface{} `json:"run information"`
	AllStats       AllStats               `json:"ALL STATS"`
}

// AllStats contains aggregate statistics.
type AllStats struct {
	Runtime RuntimeStats   `json:"Runtime"`
	Sets    OperationStats `json:"Sets,omitempty"`
	Gets    OperationStats `json:"Gets,omitempty"`
	Waits   OperationStats `json:"Waits,omitempty"`
	Totals  OperationStats `json:"Totals,omitempty"`
}

// RuntimeStats contains runtime information.
type RuntimeStats struct {
	StartTime     int64  `json:"Start time"`
	FinishTime    int64  `json:"Finish time"`
	TotalDuration int64  `json:"Total duration"`
	TimeUnit      string `json:"Time unit"`
	Interrupted   string `json:"Interrupted"`
}

// OperationStats contains per-operation statistics.
type OperationStats struct {
	Count              int64              `json:"Count"`
	OpsPerSec          float64            `json:"Ops/sec"`
	HitsPerSec         float64            `json:"Hits/sec"`
	MissesPerSec       float64            `json:"Misses/sec"`
	Latency            float64            `json:"Latency"`
	AvgLatency         float64            `json:"Average Latency"`
	AccumulatedLatency float64            `json:"Accumulated Latency"`
	MinLatency         float64            `json:"Min Latency"`
	MaxLatency         float64            `json:"Max Latency"`
	KBPerSec           float64            `json:"KB/sec"`
	Percentiles        PercentileStats    `json:"Percentile Latencies"`
	TimeSeries         map[string]interface{} `json:"Time-Serie,omitempty"`
}

// PercentileStats contains latency percentiles.
type PercentileStats struct {
	P50  float64 `json:"p50.00"`
	P90  float64 `json:"p90.00"`
	P95  float64 `json:"p95.00"`
	P99  float64 `json:"p99.00"`
	P999 float64 `json:"p99.90"`
}

// Parser converts memtier JSON output to domain Results.
type Parser struct{}

// NewParser creates a new memtier output parser.
func NewParser() *Parser {
	return &Parser{}
}

// Parse converts raw JSON bytes to domain Results.
func (p *Parser) Parse(data []byte) (*domain.Results, error) {
	// Fix memtier's non-standard JSON (trailing commas)
	fixedJSON := fixMemtierJSON(string(data))

	var output MemtierOutput
	if err := json.Unmarshal([]byte(fixedJSON), &output); err != nil {
		return nil, fmt.Errorf("failed to parse memtier JSON: %w", err)
	}

	return p.convertToResults(&output), nil
}

// fixMemtierJSON fixes memtier's non-standard JSON with trailing commas.
func fixMemtierJSON(input string) string {
	// Remove trailing commas before closing braces/brackets
	// Pattern: ,\s*} or ,\s*]
	re := regexp.MustCompile(`,(\s*[}\]])`)
	return re.ReplaceAllString(input, "$1")
}

// convertToResults transforms memtier output to domain Results.
func (p *Parser) convertToResults(output *MemtierOutput) *domain.Results {
	results := &domain.Results{
		Summary:     &domain.SummaryMetrics{},
		ByOperation: make(map[string]*domain.OperationMetrics),
	}

	// Calculate totals from Sets and Gets
	var totalOps float64
	var totalLatency float64
	var opCount int

	// Process Sets
	if output.AllStats.Sets.OpsPerSec > 0 {
		sets := output.AllStats.Sets
		results.ByOperation["SET"] = &domain.OperationMetrics{
			Operation:    "SET",
			Count:        sets.Count,
			OpsPerSecond: sets.OpsPerSec,
			AvgLatencyMs: sets.Latency,
			P50LatencyMs: p.getPercentileFromTimeSeries(sets.TimeSeries, "p50.00"),
			P90LatencyMs: p.getPercentileFromTimeSeries(sets.TimeSeries, "p90.00"),
			P95LatencyMs: p.getPercentileFromTimeSeries(sets.TimeSeries, "p95.00"),
			P99LatencyMs: p.getPercentileFromTimeSeries(sets.TimeSeries, "p99.00"),
		}
		totalOps += sets.OpsPerSec
		totalLatency += sets.Latency
		opCount++
	}

	// Process Gets
	if output.AllStats.Gets.OpsPerSec > 0 {
		gets := output.AllStats.Gets
		results.ByOperation["GET"] = &domain.OperationMetrics{
			Operation:    "GET",
			Count:        gets.Count,
			OpsPerSecond: gets.OpsPerSec,
			AvgLatencyMs: gets.Latency,
			P50LatencyMs: p.getPercentileFromTimeSeries(gets.TimeSeries, "p50.00"),
			P90LatencyMs: p.getPercentileFromTimeSeries(gets.TimeSeries, "p90.00"),
			P95LatencyMs: p.getPercentileFromTimeSeries(gets.TimeSeries, "p95.00"),
			P99LatencyMs: p.getPercentileFromTimeSeries(gets.TimeSeries, "p99.00"),
		}
		totalOps += gets.OpsPerSec
		totalLatency += gets.Latency
		opCount++
	}

	// Use Totals if available, otherwise calculate
	if output.AllStats.Totals.OpsPerSec > 0 {
		totals := output.AllStats.Totals
		results.Summary.OpsPerSecond = totals.OpsPerSec
		results.Summary.AvgLatencyMs = totals.Latency
		results.Summary.BytesPerSecond = totals.KBPerSec * 1024
		results.Summary.P50LatencyMs = totals.Percentiles.P50
		results.Summary.P90LatencyMs = totals.Percentiles.P90
		results.Summary.P95LatencyMs = totals.Percentiles.P95
		results.Summary.P99LatencyMs = totals.Percentiles.P99
		results.Summary.P999LatencyMs = totals.Percentiles.P999
	} else {
		// Calculate from individual operations
		results.Summary.OpsPerSecond = totalOps
		if opCount > 0 {
			results.Summary.AvgLatencyMs = totalLatency / float64(opCount)
		}

		// Get percentiles from first available operation
		for _, op := range results.ByOperation {
			results.Summary.P50LatencyMs = op.P50LatencyMs
			results.Summary.P90LatencyMs = op.P90LatencyMs
			results.Summary.P95LatencyMs = op.P95LatencyMs
			results.Summary.P99LatencyMs = op.P99LatencyMs
			break
		}
	}

	return results
}

// getPercentileFromTimeSeries extracts percentile from time series data.
func (p *Parser) getPercentileFromTimeSeries(timeSeries map[string]interface{}, key string) float64 {
	if timeSeries == nil {
		return 0
	}

	// Try to get from first time bucket
	for _, v := range timeSeries {
		if bucket, ok := v.(map[string]interface{}); ok {
			if val, ok := bucket[key]; ok {
				if f, ok := val.(float64); ok {
					return f
				}
			}
		}
		break // Only check first bucket
	}
	return 0
}

// ParseVersion extracts version info from memtier output.
func (p *Parser) ParseVersion(data []byte) (string, error) {
	fixedJSON := fixMemtierJSON(string(data))
	var output MemtierOutput
	if err := json.Unmarshal([]byte(fixedJSON), &output); err != nil {
		return "", err
	}

	if v, ok := output.Configuration["version"].(string); ok {
		return v, nil
	}
	return "unknown", nil
}
