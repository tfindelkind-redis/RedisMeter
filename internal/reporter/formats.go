package reporter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
)

// JSONReporter generates JSON reports.
type JSONReporter struct {
	options ReportOptions
	pretty  bool
}

// Name returns the reporter name.
func (r *JSONReporter) Name() string {
	return "json"
}

// Description returns the reporter description.
func (r *JSONReporter) Description() string {
	return "Generate JSON reports for programmatic access"
}

// ContentType returns the MIME type.
func (r *JSONReporter) ContentType() string {
	return "application/json"
}

// FileExtension returns the file extension.
func (r *JSONReporter) FileExtension() string {
	return ".json"
}

// Generate generates a JSON report.
func (r *JSONReporter) Generate(ctx context.Context, report *Report, w io.Writer) error {
	output := map[string]interface{}{
		"title":       report.Title,
		"description": report.Description,
		"generated":   time.Now().UTC().Format(time.RFC3339),
		"generator":   "redismeter",
	}

	if report.Run != nil {
		output["run"] = report.Run
	}

	if len(report.Runs) > 0 {
		output["runs"] = report.Runs
	}

	if report.Comparison != nil {
		output["comparison"] = report.Comparison
	}

	if len(report.Analysis) > 0 {
		output["analysis"] = report.Analysis
	}

	if report.Baseline != nil {
		output["baseline"] = report.Baseline
	}

	if len(report.Metadata) > 0 {
		output["metadata"] = report.Metadata
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// TextReporter generates plain text reports.
type TextReporter struct {
	options ReportOptions
}

// Name returns the reporter name.
func (r *TextReporter) Name() string {
	return "text"
}

// Description returns the reporter description.
func (r *TextReporter) Description() string {
	return "Generate plain text reports for terminal output"
}

// ContentType returns the MIME type.
func (r *TextReporter) ContentType() string {
	return "text/plain"
}

// FileExtension returns the file extension.
func (r *TextReporter) FileExtension() string {
	return ".txt"
}

// Generate generates a plain text report.
func (r *TextReporter) Generate(ctx context.Context, report *Report, w io.Writer) error {
	var sb strings.Builder

	title := report.Title
	if title == "" {
		title = "Benchmark Report"
	}

	// Header
	sb.WriteString(strings.Repeat("=", 70) + "\n")
	sb.WriteString(fmt.Sprintf("  %s\n", strings.ToUpper(title)))
	sb.WriteString(strings.Repeat("=", 70) + "\n")
	sb.WriteString(fmt.Sprintf("  Generated: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	if report.Description != "" {
		sb.WriteString(fmt.Sprintf("  %s\n", report.Description))
	}
	sb.WriteString(strings.Repeat("-", 70) + "\n\n")

	// Single run report
	if report.Run != nil {
		run := report.Run
		summary := getRunSummary(run)

		sb.WriteString("SUMMARY\n")
		sb.WriteString(strings.Repeat("-", 40) + "\n")
		sb.WriteString(fmt.Sprintf("  %-20s %s\n", "Run ID:", run.ID))
		sb.WriteString(fmt.Sprintf("  %-20s %s\n", "Workload:", run.Workload.Name))
		sb.WriteString(fmt.Sprintf("  %-20s %s:%d\n", "Target:", run.Target.Host, run.Target.Port))
		sb.WriteString(fmt.Sprintf("  %-20s %s\n", "Status:", run.Status))
		sb.WriteString("\n")

		sb.WriteString("KEY METRICS\n")
		sb.WriteString(strings.Repeat("-", 40) + "\n")
		sb.WriteString(fmt.Sprintf("  %-20s %.0f ops/sec\n", "Throughput:", summary.OpsPerSecond))
		sb.WriteString(fmt.Sprintf("  %-20s %.3f ms\n", "Avg Latency:", summary.AvgLatencyMs))
		sb.WriteString(fmt.Sprintf("  %-20s %.3f ms\n", "P99 Latency:", summary.P99LatencyMs))
		sb.WriteString(fmt.Sprintf("  %-20s %d\n", "Total Operations:", summary.TotalOps))
		sb.WriteString("\n")

		// Operations breakdown
		if run.Results != nil && len(run.Results.ByOperation) > 0 {
			sb.WriteString("OPERATIONS\n")
			sb.WriteString(strings.Repeat("-", 70) + "\n")
			sb.WriteString(fmt.Sprintf("  %-12s %12s %12s %12s %10s\n",
				"Operation", "Ops/sec", "Avg (ms)", "P99 (ms)", "Count"))
			sb.WriteString(strings.Repeat("-", 70) + "\n")

			for op, stats := range run.Results.ByOperation {
				sb.WriteString(fmt.Sprintf("  %-12s %12.0f %12.3f %12.3f %10d\n",
					op, stats.OpsPerSecond, stats.AvgLatencyMs, stats.P99LatencyMs, stats.Count))
			}
			sb.WriteString("\n")
		}

		// Latency percentiles
		sb.WriteString("LATENCY PERCENTILES\n")
		sb.WriteString(strings.Repeat("-", 40) + "\n")
		sb.WriteString(fmt.Sprintf("  %-20s %.3f ms\n", "P50:", summary.P50LatencyMs))
		sb.WriteString(fmt.Sprintf("  %-20s %.3f ms\n", "P95:", summary.P95LatencyMs))
		sb.WriteString(fmt.Sprintf("  %-20s %.3f ms\n", "P99:", summary.P99LatencyMs))
		sb.WriteString(fmt.Sprintf("  %-20s %.3f ms\n", "P99.9:", summary.P999LatencyMs))
		sb.WriteString("\n")

		// Configuration
		sb.WriteString("CONFIGURATION\n")
		sb.WriteString(strings.Repeat("-", 40) + "\n")
		sb.WriteString(fmt.Sprintf("  %-20s %d\n", "Clients:", run.Workload.Clients))
		sb.WriteString(fmt.Sprintf("  %-20s %d\n", "Threads:", run.Workload.Threads))
		sb.WriteString(fmt.Sprintf("  %-20s %d\n", "Pipeline:", run.Workload.Pipeline))
		if run.Workload.Duration != "" {
			sb.WriteString(fmt.Sprintf("  %-20s %s\n", "Duration:", run.Workload.Duration))
		}
		if run.Workload.Requests > 0 {
			sb.WriteString(fmt.Sprintf("  %-20s %d\n", "Requests:", run.Workload.Requests))
		}
		sb.WriteString("\n")
	}

	// Comparison report
	if report.Comparison != nil {
		comp := report.Comparison

		sb.WriteString("COMPARISON\n")
		sb.WriteString(strings.Repeat("-", 70) + "\n")
		sb.WriteString(fmt.Sprintf("  %-20s %-15s %-15s %-15s\n", "Metric", "Run 1", "Run 2", "Change"))
		sb.WriteString(strings.Repeat("-", 70) + "\n")
		
		if comp.Metrics != nil {
			sb.WriteString(fmt.Sprintf("  %-20s %-15.0f %-15.0f %+.2f%%\n",
				"Throughput",
				comp.Metrics.OpsPerSecond1,
				comp.Metrics.OpsPerSecond2,
				comp.Metrics.ThroughputDiff))
			sb.WriteString(fmt.Sprintf("  %-20s %-15.3f %-15.3f %+.2f%%\n",
				"Avg Latency (ms)",
				comp.Metrics.AvgLatency1,
				comp.Metrics.AvgLatency2,
				comp.Metrics.AvgLatencyDiff))
			sb.WriteString(fmt.Sprintf("  %-20s %-15.3f %-15.3f %+.2f%%\n",
				"P99 Latency (ms)",
				comp.Metrics.P99Latency1,
				comp.Metrics.P99Latency2,
				comp.Metrics.P99LatencyDiff))
		}
		sb.WriteString("\n")

		if !comp.EnvironmentMatch {
			sb.WriteString("  WARNING: Environments differ between runs!\n\n")
		}
	}

	// Analysis reports
	if len(report.Analysis) > 0 {
		for _, analysisReport := range report.Analysis {
			sb.WriteString(fmt.Sprintf("%s ANALYSIS\n", strings.ToUpper(analysisReport.Analyzer)))
			sb.WriteString(strings.Repeat("-", 40) + "\n")
			sb.WriteString(fmt.Sprintf("  %s\n\n", analysisReport.Summary))

			if len(analysisReport.Findings) > 0 {
				sb.WriteString("  Findings:\n")
				for _, finding := range analysisReport.Findings {
					sb.WriteString(fmt.Sprintf("    [%s] %s: %s\n", strings.ToUpper(finding.Severity), finding.Title, finding.Description))
				}
				sb.WriteString("\n")
			}

			if len(analysisReport.Recommendations) > 0 {
				sb.WriteString("  Recommendations:\n")
				for _, rec := range analysisReport.Recommendations {
					sb.WriteString(fmt.Sprintf("    - [%s] %s: %s\n", rec.Priority, rec.Title, rec.Description))
				}
				sb.WriteString("\n")
			}
		}
	}

	sb.WriteString(strings.Repeat("=", 70) + "\n")
	sb.WriteString("  Generated by RedisMeter\n")
	sb.WriteString(strings.Repeat("=", 70) + "\n")

	_, err := w.Write([]byte(sb.String()))
	return err
}

// SlackReporter generates Slack message format.
type SlackReporter struct {
	options ReportOptions
}

// Name returns the reporter name.
func (r *SlackReporter) Name() string {
	return "slack"
}

// Description returns the reporter description.
func (r *SlackReporter) Description() string {
	return "Generate Slack message blocks"
}

// ContentType returns the MIME type.
func (r *SlackReporter) ContentType() string {
	return "application/json"
}

// FileExtension returns the file extension.
func (r *SlackReporter) FileExtension() string {
	return ".json"
}

// Generate generates a Slack message format.
func (r *SlackReporter) Generate(ctx context.Context, report *Report, w io.Writer) error {
	blocks := []map[string]interface{}{}

	// Header
	title := report.Title
	if title == "" {
		title = "Benchmark Report"
	}

	blocks = append(blocks, map[string]interface{}{
		"type": "header",
		"text": map[string]interface{}{
			"type":  "plain_text",
			"text":  title,
			"emoji": true,
		},
	})

	// Single run report
	if report.Run != nil {
		run := report.Run

		// Summary section
		opsPerSec := float64(0)
		avgLatency := float64(0)
		p99Latency := float64(0)
		duration := float64(0)

		if run.Results != nil && run.Results.Summary != nil {
			opsPerSec = run.Results.Summary.OpsPerSecond
			avgLatency = run.Results.Summary.AvgLatencyMs
			p99Latency = run.Results.Summary.P99LatencyMs
		}

		blocks = append(blocks, map[string]interface{}{
			"type": "section",
			"fields": []map[string]string{
				{"type": "mrkdwn", "text": fmt.Sprintf("*Workload:*\n%s", run.Workload.Name)},
				{"type": "mrkdwn", "text": fmt.Sprintf("*Target:*\n%s:%d", run.Target.Host, run.Target.Port)},
				{"type": "mrkdwn", "text": fmt.Sprintf("*Throughput:*\n%.0f ops/sec", opsPerSec)},
				{"type": "mrkdwn", "text": fmt.Sprintf("*Avg Latency:*\n%.3f ms", avgLatency)},
			},
		})

		// Latency details
		blocks = append(blocks, map[string]interface{}{
			"type": "section",
			"fields": []map[string]string{
				{"type": "mrkdwn", "text": fmt.Sprintf("*P99 Latency:*\n%.3f ms", p99Latency)},
				{"type": "mrkdwn", "text": fmt.Sprintf("*Duration:*\n%.1f seconds", duration)},
			},
		})

		// Status
		statusEmoji := "✅"
		if run.Status == "failed" {
			statusEmoji = "❌"
		}
		blocks = append(blocks, map[string]interface{}{
			"type": "context",
			"elements": []map[string]string{
				{"type": "mrkdwn", "text": fmt.Sprintf("%s *Status:* %s | *Run ID:* `%s`", statusEmoji, run.Status, run.ID)},
			},
		})
	}

	// Comparison
	if report.Comparison != nil {
		comp := report.Comparison

		throughputEmoji := "➡️"
		if comp.Metrics != nil && comp.Metrics.ThroughputDiff > 5 {
			throughputEmoji = "📈"
		} else if comp.Metrics != nil && comp.Metrics.ThroughputDiff < -5 {
			throughputEmoji = "📉"
		}

		latencyChange := float64(0)
		throughputChange := float64(0)
		if comp.Metrics != nil {
			latencyChange = comp.Metrics.AvgLatencyDiff
			throughputChange = comp.Metrics.ThroughputDiff
		}

		blocks = append(blocks, map[string]interface{}{
			"type": "divider",
		})

		blocks = append(blocks, map[string]interface{}{
			"type": "section",
			"text": map[string]string{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*Comparison Results*\n%s Throughput: %+.2f%% | Latency: %+.2f%%",
					throughputEmoji, throughputChange, latencyChange),
			},
		})
	}

	// Footer
	blocks = append(blocks, map[string]interface{}{
		"type": "context",
		"elements": []map[string]string{
			{"type": "mrkdwn", "text": fmt.Sprintf("Generated by RedisMeter at %s", time.Now().Format("2006-01-02 15:04:05"))},
		},
	})

	output := map[string]interface{}{
		"blocks": blocks,
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func init() {
	// Register Slack reporter
	DefaultRegistry.Register(&SlackReporter{})
}

// getRunSummary safely extracts summary metrics from a run.
func getRunSummary(run *domain.BenchmarkRun) *domain.SummaryMetrics {
	if run != nil && run.Results != nil && run.Results.Summary != nil {
		return run.Results.Summary
	}
	return &domain.SummaryMetrics{}
}
