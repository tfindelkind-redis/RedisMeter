// Package export provides data export functionality for RedisMeter.
package export

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
)

// Format represents an export format.
type Format string

const (
	// FormatJSON exports as pretty-printed JSON.
	FormatJSON Format = "json"
	// FormatJSONCompact exports as compact JSON (no indentation).
	FormatJSONCompact Format = "json-compact"
	// FormatJSONL exports as JSON Lines (one object per line).
	FormatJSONL Format = "jsonl"
	// FormatCSV exports as CSV.
	FormatCSV Format = "csv"
)

// Options configures the export operation.
type Options struct {
	// Format specifies the output format.
	Format Format

	// IncludeEnvironment includes environment details in the export.
	IncludeEnvironment bool

	// IncludeRawOutput includes raw memtier output in the export.
	IncludeRawOutput bool
}

// DefaultOptions returns the default export options.
func DefaultOptions() Options {
	return Options{
		Format:             FormatJSON,
		IncludeEnvironment: true,
		IncludeRawOutput:   false,
	}
}

// Exporter exports benchmark data to various formats.
type Exporter struct {
	options Options
}

// NewExporter creates a new exporter with the given options.
func NewExporter(options Options) *Exporter {
	return &Exporter{options: options}
}

// ExportRuns exports multiple benchmark runs to the writer.
func (e *Exporter) ExportRuns(ctx context.Context, runs []*domain.BenchmarkRun, w io.Writer) error {
	// Prepare data for export
	exportData := make([]map[string]interface{}, 0, len(runs))
	for _, run := range runs {
		data := e.prepareRunData(run)
		exportData = append(exportData, data)
	}

	switch e.options.Format {
	case FormatJSON:
		return e.exportJSON(exportData, w, true)
	case FormatJSONCompact:
		return e.exportJSON(exportData, w, false)
	case FormatJSONL:
		return e.exportJSONL(exportData, w)
	case FormatCSV:
		return e.exportCSV(runs, w)
	default:
		return fmt.Errorf("unsupported format: %s", e.options.Format)
	}
}

// ExportRun exports a single benchmark run to the writer.
func (e *Exporter) ExportRun(ctx context.Context, run *domain.BenchmarkRun, w io.Writer) error {
	return e.ExportRuns(ctx, []*domain.BenchmarkRun{run}, w)
}

// prepareRunData converts a BenchmarkRun to a map for export.
func (e *Exporter) prepareRunData(run *domain.BenchmarkRun) map[string]interface{} {
	data := map[string]interface{}{
		"id":          run.ID,
		"created_at":  run.CreatedAt.Format(time.RFC3339),
		"updated_at":  run.UpdatedAt.Format(time.RFC3339),
		"status":      string(run.Status),
		"name":        run.Name,
		"description": run.Description,
		"tags":        run.Tags,
		"labels":      run.Labels,
	}

	if !run.StartTime.IsZero() {
		data["start_time"] = run.StartTime.Format(time.RFC3339)
	}
	if !run.EndTime.IsZero() {
		data["end_time"] = run.EndTime.Format(time.RFC3339)
	}
	if run.Duration != "" {
		data["duration"] = run.Duration
	}
	if run.Error != "" {
		data["error"] = run.Error
	}

	// Workload
	if run.Workload != nil {
		data["workload"] = map[string]interface{}{
			"name":        run.Workload.Name,
			"type":        run.Workload.Type,
			"description": run.Workload.Description,
		}
	}

	// Target
	if run.Target != nil {
		data["target"] = map[string]interface{}{
			"host": run.Target.Host,
			"port": run.Target.Port,
			"url":  run.Target.URL,
		}
	}

	// Results
	if run.Results != nil && run.Results.Summary != nil {
		s := run.Results.Summary
		data["results"] = map[string]interface{}{
			"ops_per_second":  s.OpsPerSecond,
			"avg_latency_ms":  s.AvgLatencyMs,
			"min_latency_ms":  s.MinLatencyMs,
			"max_latency_ms":  s.MaxLatencyMs,
			"p50_latency_ms":  s.P50LatencyMs,
			"p90_latency_ms":  s.P90LatencyMs,
			"p95_latency_ms":  s.P95LatencyMs,
			"p99_latency_ms":  s.P99LatencyMs,
			"p999_latency_ms": s.P999LatencyMs,
			"errors":          s.Errors,
			"error_rate":      s.ErrorRate,
			"total_ops":       s.TotalOps,
			"total_requests":  s.TotalRequests,
		}

		// Include by-operation breakdown
		if len(run.Results.ByOperation) > 0 {
			byOp := make(map[string]interface{})
			for op, metrics := range run.Results.ByOperation {
				byOp[op] = map[string]interface{}{
					"count":          metrics.Count,
					"ops_per_second": metrics.OpsPerSecond,
					"avg_latency_ms": metrics.AvgLatencyMs,
					"p50_latency_ms": metrics.P50LatencyMs,
					"p99_latency_ms": metrics.P99LatencyMs,
				}
			}
			data["results"].(map[string]interface{})["by_operation"] = byOp
		}

		// Include raw output if requested
		if e.options.IncludeRawOutput && run.Results.RawOutput != "" {
			data["results"].(map[string]interface{})["raw_output"] = run.Results.RawOutput
		}
	}

	// Environment
	if e.options.IncludeEnvironment && run.Environment != nil {
		env := run.Environment
		envData := map[string]interface{}{
			"fingerprint":        env.Fingerprint,
			"redismeter_version": env.RedisMeterVersion,
			"memtier_version":    env.MemtierVersion,
		}

		// Host information
		if env.Host != nil && (env.Host.Hostname != "" || env.Host.OS != "") {
			envData["host"] = map[string]interface{}{
				"hostname":  env.Host.Hostname,
				"os":        env.Host.OS,
				"arch":      env.Host.Arch,
				"cpu_model": env.Host.CPUModel,
				"cpus":      env.Host.CPUs,
				"memory_gb": env.Host.MemoryGB,
			}
		}

		// Redis information
		if env.Redis != nil && env.Redis.Version != "" {
			redisData := map[string]interface{}{
				"version":           env.Redis.Version,
				"mode":              env.Redis.Mode,
				"memory_used":       env.Redis.MemoryUsed,
				"memory_max":        env.Redis.MemoryMax,
				"connected_clients": env.Redis.ConnectedClients,
				"evicted_keys":      env.Redis.EvictedKeys,
				"total_keys":        env.Redis.TotalKeys,
				"modules":           env.Redis.Modules,
			}
			if env.Redis.Config != nil {
				redisData["config"] = env.Redis.Config
			}
			envData["redis"] = redisData
		}

		// Cloud information
		if env.Cloud != nil {
			envData["cloud"] = map[string]interface{}{
				"provider":      env.Cloud.Provider,
				"region":        env.Cloud.Region,
				"instance_type": env.Cloud.InstanceType,
			}
		}

		data["environment"] = envData
	}

	return data
}

// exportJSON exports data as JSON.
func (e *Exporter) exportJSON(data []map[string]interface{}, w io.Writer, pretty bool) error {
	encoder := json.NewEncoder(w)
	if pretty {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(data)
}

// exportJSONL exports data as JSON Lines.
func (e *Exporter) exportJSONL(data []map[string]interface{}, w io.Writer) error {
	encoder := json.NewEncoder(w)
	for _, item := range data {
		if err := encoder.Encode(item); err != nil {
			return err
		}
	}
	return nil
}

// csvHeaders are the headers for CSV export.
var csvHeaders = []string{
	"id",
	"created_at",
	"status",
	"workload",
	"target_host",
	"target_port",
	"ops_per_second",
	"avg_latency_ms",
	"p50_latency_ms",
	"p99_latency_ms",
	"p999_latency_ms",
	"errors",
	"error_rate",
	"duration",
	"name",
	"tags",
}

// exportCSV exports data as CSV.
func (e *Exporter) exportCSV(runs []*domain.BenchmarkRun, w io.Writer) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write headers
	if err := writer.Write(csvHeaders); err != nil {
		return err
	}

	// Write data rows
	for _, run := range runs {
		row := make([]string, len(csvHeaders))

		row[0] = run.ID
		row[1] = run.CreatedAt.Format(time.RFC3339)
		row[2] = string(run.Status)

		if run.Workload != nil {
			row[3] = run.Workload.Name
		}

		if run.Target != nil {
			row[4] = run.Target.Host
			row[5] = fmt.Sprintf("%d", run.Target.Port)
		}

		if run.Results != nil && run.Results.Summary != nil {
			s := run.Results.Summary
			row[6] = fmt.Sprintf("%.2f", s.OpsPerSecond)
			row[7] = fmt.Sprintf("%.3f", s.AvgLatencyMs)
			row[8] = fmt.Sprintf("%.3f", s.P50LatencyMs)
			row[9] = fmt.Sprintf("%.3f", s.P99LatencyMs)
			row[10] = fmt.Sprintf("%.3f", s.P999LatencyMs)
			row[11] = fmt.Sprintf("%d", s.Errors)
			row[12] = fmt.Sprintf("%.4f", s.ErrorRate)
		}

		row[13] = run.Duration
		row[14] = run.Name

		if len(run.Tags) > 0 {
			row[15] = fmt.Sprintf("%v", run.Tags)
		}

		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}
