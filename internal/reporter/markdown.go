package reporter

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
)

// MarkdownReporter generates Markdown reports.
type MarkdownReporter struct {
	options ReportOptions
}

// Name returns the reporter name.
func (r *MarkdownReporter) Name() string {
	return "markdown"
}

// Description returns the reporter description.
func (r *MarkdownReporter) Description() string {
	return "Generate Markdown reports for documentation"
}

// ContentType returns the MIME type.
func (r *MarkdownReporter) ContentType() string {
	return "text/markdown"
}

// FileExtension returns the file extension.
func (r *MarkdownReporter) FileExtension() string {
	return ".md"
}

// Generate generates a Markdown report.
func (r *MarkdownReporter) Generate(ctx context.Context, report *Report, w io.Writer) error {
	var sb strings.Builder

	// Title
	title := report.Title
	if title == "" {
		title = "Benchmark Report"
	}
	sb.WriteString(fmt.Sprintf("# %s\n\n", title))

	if report.Description != "" {
		sb.WriteString(fmt.Sprintf("> %s\n\n", report.Description))
	}

	sb.WriteString(fmt.Sprintf("*Generated: %s*\n\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString("---\n\n")

	// Single run report
	if report.Run != nil {
		run := report.Run
		summary := getSummaryMetrics(run)

		sb.WriteString("## Summary\n\n")

		// Key metrics table
		sb.WriteString("| Metric | Value |\n")
		sb.WriteString("|--------|-------|\n")
		sb.WriteString(fmt.Sprintf("| **Throughput** | %.0f ops/sec |\n", summary.OpsPerSecond))
		sb.WriteString(fmt.Sprintf("| **Avg Latency** | %.3f ms |\n", summary.AvgLatencyMs))
		sb.WriteString(fmt.Sprintf("| **P99 Latency** | %.3f ms |\n", summary.P99LatencyMs))
		sb.WriteString(fmt.Sprintf("| **Total Ops** | %d |\n", summary.TotalOps))
		sb.WriteString("\n")

		// Run details
		sb.WriteString("## Run Details\n\n")
		sb.WriteString("| Property | Value |\n")
		sb.WriteString("|----------|-------|\n")
		sb.WriteString(fmt.Sprintf("| Run ID | `%s` |\n", run.ID))
		sb.WriteString(fmt.Sprintf("| Workload | %s |\n", run.Workload.Name))
		sb.WriteString(fmt.Sprintf("| Target | %s:%d |\n", run.Target.Host, run.Target.Port))
		sb.WriteString(fmt.Sprintf("| Status | %s |\n", statusEmoji(string(run.Status))))
		sb.WriteString(fmt.Sprintf("| Clients | %d |\n", run.Workload.Clients))
		sb.WriteString(fmt.Sprintf("| Threads | %d |\n", run.Workload.Threads))
		sb.WriteString(fmt.Sprintf("| Pipeline | %d |\n", run.Workload.Pipeline))
		sb.WriteString("\n")

		// Operations breakdown
		if run.Results != nil && len(run.Results.ByOperation) > 0 {
			sb.WriteString("## Operations Breakdown\n\n")
			sb.WriteString("| Operation | Ops/sec | Avg Latency | P99 Latency | Count |\n")
			sb.WriteString("|-----------|---------|-------------|-------------|-------|\n")

			for op, stats := range run.Results.ByOperation {
				sb.WriteString(fmt.Sprintf("| **%s** | %.0f | %.3f ms | %.3f ms | %d |\n",
					op,
					stats.OpsPerSecond,
					stats.AvgLatencyMs,
					stats.P99LatencyMs,
					stats.Count))
			}
			sb.WriteString("\n")
		}

		// Latency percentiles
		sb.WriteString("## Latency Percentiles\n\n")
		sb.WriteString("| Percentile | Latency (ms) |\n")
		sb.WriteString("|------------|-------------|\n")
		sb.WriteString(fmt.Sprintf("| P50 | %.3f |\n", summary.P50LatencyMs))
		sb.WriteString(fmt.Sprintf("| P95 | %.3f |\n", summary.P95LatencyMs))
		sb.WriteString(fmt.Sprintf("| P99 | %.3f |\n", summary.P99LatencyMs))
		sb.WriteString(fmt.Sprintf("| P99.9 | %.3f |\n", summary.P999LatencyMs))
		sb.WriteString("\n")

		// Environment info if available
		if run.Environment != nil {
			sb.WriteString("## Environment\n\n")

			if run.Environment.Host != nil {
				sb.WriteString("### Host\n\n")
				sb.WriteString(fmt.Sprintf("- **Hostname**: %s\n", run.Environment.Host.Hostname))
				sb.WriteString(fmt.Sprintf("- **OS**: %s %s\n", run.Environment.Host.OS, run.Environment.Host.Arch))
				sb.WriteString(fmt.Sprintf("- **CPUs**: %d\n", run.Environment.Host.CPUs))
				sb.WriteString(fmt.Sprintf("- **Memory**: %.1f GB\n", run.Environment.Host.MemoryGB))
				sb.WriteString("\n")
			}

			if run.Environment.Redis != nil && run.Environment.Redis.Version != "" {
				sb.WriteString("### Redis\n\n")
				sb.WriteString(fmt.Sprintf("- **Version**: %s\n", run.Environment.Redis.Version))
				if run.Environment.Redis.Mode != "" {
					sb.WriteString(fmt.Sprintf("- **Mode**: %s\n", run.Environment.Redis.Mode))
				}
				if run.Environment.Redis.MemoryUsed > 0 {
					sb.WriteString(fmt.Sprintf("- **Memory Used**: %.2f MB\n", float64(run.Environment.Redis.MemoryUsed)/(1024*1024)))
				}
				if run.Environment.Redis.ConnectedClients > 0 {
					sb.WriteString(fmt.Sprintf("- **Connected Clients**: %d\n", run.Environment.Redis.ConnectedClients))
				}
				sb.WriteString("\n")
			}
		}
	}

	// Comparison report
	if report.Comparison != nil {
		comp := report.Comparison

		sb.WriteString("## Comparison Results\n\n")

		sb.WriteString("| Metric | Run 1 | Run 2 | Change |\n")
		sb.WriteString("|--------|-------|-------|--------|\n")
		
		if comp.Metrics != nil {
			sb.WriteString(fmt.Sprintf("| Throughput | %.0f ops/sec | %.0f ops/sec | %s |\n",
				comp.Metrics.OpsPerSecond1,
				comp.Metrics.OpsPerSecond2,
				formatChangeWithEmoji(comp.Metrics.ThroughputDiff, true)))
			sb.WriteString(fmt.Sprintf("| Avg Latency | %.3f ms | %.3f ms | %s |\n",
				comp.Metrics.AvgLatency1,
				comp.Metrics.AvgLatency2,
				formatChangeWithEmoji(comp.Metrics.AvgLatencyDiff, false)))
			sb.WriteString(fmt.Sprintf("| P99 Latency | %.3f ms | %.3f ms | %s |\n",
				comp.Metrics.P99Latency1,
				comp.Metrics.P99Latency2,
				formatChangeWithEmoji(comp.Metrics.P99LatencyDiff, false)))
		}
		sb.WriteString("\n")

		if !comp.EnvironmentMatch {
			sb.WriteString("⚠️ **Note**: Environments differ between runs. Comparison may not be meaningful.\n\n")
		}
	}

	// Analysis reports
	if len(report.Analysis) > 0 {
		sb.WriteString("## Analysis\n\n")

		for _, analysisReport := range report.Analysis {
			sb.WriteString(fmt.Sprintf("### %s\n\n", analysisReport.Analyzer))
			sb.WriteString(fmt.Sprintf("%s\n\n", analysisReport.Summary))

			if len(analysisReport.Findings) > 0 {
				sb.WriteString("#### Findings\n\n")
				for _, finding := range analysisReport.Findings {
					emoji := severityEmoji(finding.Severity)
					sb.WriteString(fmt.Sprintf("- %s **%s**: %s - %s\n", emoji, finding.Severity, finding.Title, finding.Description))
				}
				sb.WriteString("\n")
			}

			if len(analysisReport.Recommendations) > 0 {
				sb.WriteString("#### Recommendations\n\n")
				for _, rec := range analysisReport.Recommendations {
					sb.WriteString(fmt.Sprintf("- [%s] **%s**: %s\n", rec.Priority, rec.Title, rec.Description))
				}
				sb.WriteString("\n")
			}
		}
	}

	sb.WriteString("---\n\n")
	sb.WriteString("*Generated by [RedisMeter](https://github.com/tfindelkind-redis/redismeter)*\n")

	_, err := w.Write([]byte(sb.String()))
	return err
}

func statusEmoji(status string) string {
	switch strings.ToLower(status) {
	case "completed":
		return "✅ Completed"
	case "running":
		return "🔄 Running"
	case "failed":
		return "❌ Failed"
	default:
		return status
	}
}

func severityEmoji(severity string) string {
	switch strings.ToLower(severity) {
	case "critical":
		return "🔴"
	case "warning":
		return "🟡"
	case "info":
		return "🔵"
	default:
		return "⚪"
	}
}

func formatChangeWithEmoji(change float64, higherIsBetter bool) string {
	emoji := "➡️"
	if higherIsBetter {
		if change > 5 {
			emoji = "📈"
		} else if change < -5 {
			emoji = "📉"
		}
	} else {
		if change < -5 {
			emoji = "📈"
		} else if change > 5 {
			emoji = "📉"
		}
	}
	return fmt.Sprintf("%s %+.2f%%", emoji, change)
}

// getSummaryMetrics safely extracts summary metrics from a run.
func getSummaryMetrics(run *domain.BenchmarkRun) *domain.SummaryMetrics {
	if run != nil && run.Results != nil && run.Results.Summary != nil {
		return run.Results.Summary
	}
	return &domain.SummaryMetrics{}
}
