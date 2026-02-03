package reporter

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"strings"
	"time"
)

// HTMLReporter generates HTML reports.
type HTMLReporter struct {
	options ReportOptions
}

// Name returns the reporter name.
func (r *HTMLReporter) Name() string {
	return "html"
}

// Description returns the reporter description.
func (r *HTMLReporter) Description() string {
	return "Generate interactive HTML reports with charts"
}

// ContentType returns the MIME type.
func (r *HTMLReporter) ContentType() string {
	return "text/html"
}

// FileExtension returns the file extension.
func (r *HTMLReporter) FileExtension() string {
	return ".html"
}

// Generate generates an HTML report.
func (r *HTMLReporter) Generate(ctx context.Context, report *Report, w io.Writer) error {
	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"formatNumber":   formatNumber,
		"formatDuration": formatDuration,
		"formatPercent":  formatPercent,
		"formatLatency":  formatLatency,
		"statusClass":    statusClass,
		"changeClass":    changeClass,
	}).Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	data := struct {
		*Report
		Generated string
	}{
		Report:    report,
		Generated: time.Now().Format("2006-01-02 15:04:05"),
	}

	return tmpl.Execute(w, data)
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}} - RedisMeter Report</title>
    <style>
        :root {
            --primary: #dc382d;
            --primary-dark: #b62d24;
            --success: #22c55e;
            --warning: #f59e0b;
            --danger: #ef4444;
            --gray-50: #f9fafb;
            --gray-100: #f3f4f6;
            --gray-200: #e5e7eb;
            --gray-300: #d1d5db;
            --gray-600: #4b5563;
            --gray-800: #1f2937;
            --gray-900: #111827;
        }
        
        * { box-sizing: border-box; margin: 0; padding: 0; }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            background: var(--gray-50);
            color: var(--gray-800);
            line-height: 1.6;
        }
        
        .container {
            max-width: 1200px;
            margin: 0 auto;
            padding: 2rem;
        }
        
        header {
            background: linear-gradient(135deg, var(--gray-900), var(--gray-800));
            color: white;
            padding: 2rem;
            margin-bottom: 2rem;
            border-radius: 12px;
        }
        
        header h1 {
            font-size: 2rem;
            margin-bottom: 0.5rem;
        }
        
        header .meta {
            color: var(--gray-300);
            font-size: 0.9rem;
        }
        
        .card {
            background: white;
            border-radius: 12px;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
            margin-bottom: 1.5rem;
            overflow: hidden;
        }
        
        .card-header {
            background: var(--gray-100);
            padding: 1rem 1.5rem;
            border-bottom: 1px solid var(--gray-200);
            font-weight: 600;
        }
        
        .card-body {
            padding: 1.5rem;
        }
        
        .metrics-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1.5rem;
        }
        
        .metric {
            text-align: center;
            padding: 1rem;
            background: var(--gray-50);
            border-radius: 8px;
        }
        
        .metric-value {
            font-size: 2rem;
            font-weight: 700;
            color: var(--primary);
        }
        
        .metric-label {
            font-size: 0.85rem;
            color: var(--gray-600);
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }
        
        table {
            width: 100%;
            border-collapse: collapse;
        }
        
        th, td {
            padding: 0.75rem 1rem;
            text-align: left;
            border-bottom: 1px solid var(--gray-200);
        }
        
        th {
            background: var(--gray-50);
            font-weight: 600;
            font-size: 0.85rem;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }
        
        .status-completed { color: var(--success); }
        .status-running { color: var(--primary); }
        .status-failed { color: var(--danger); }
        
        .change-positive { color: var(--success); }
        .change-negative { color: var(--danger); }
        .change-neutral { color: var(--gray-600); }
        
        .badge {
            display: inline-block;
            padding: 0.25rem 0.75rem;
            border-radius: 9999px;
            font-size: 0.75rem;
            font-weight: 600;
        }
        
        .badge-success { background: #dcfce7; color: #166534; }
        .badge-warning { background: #fef3c7; color: #92400e; }
        .badge-danger { background: #fee2e2; color: #991b1b; }
        
        footer {
            text-align: center;
            padding: 2rem;
            color: var(--gray-600);
            font-size: 0.85rem;
        }
        
        @media print {
            body { background: white; }
            .container { max-width: 100%; }
            .card { box-shadow: none; border: 1px solid var(--gray-200); }
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>{{if .Title}}{{.Title}}{{else}}Benchmark Report{{end}}</h1>
            <div class="meta">
                Generated: {{.Generated}} | RedisMeter
                {{if .Description}}<br>{{.Description}}{{end}}
            </div>
        </header>

        {{if .Run}}
        <!-- Single Run Report -->
        <div class="card">
            <div class="card-header">Run Summary</div>
            <div class="card-body">
                <div class="metrics-grid">
                    <div class="metric">
                        <div class="metric-value">{{formatNumber .Run.Results.Totals.OpsPerSec}}</div>
                        <div class="metric-label">Ops/sec</div>
                    </div>
                    <div class="metric">
                        <div class="metric-value">{{formatLatency .Run.Results.Totals.AvgLatency}}</div>
                        <div class="metric-label">Avg Latency</div>
                    </div>
                    <div class="metric">
                        <div class="metric-value">{{formatLatency .Run.Results.Totals.P99Latency}}</div>
                        <div class="metric-label">P99 Latency</div>
                    </div>
                    <div class="metric">
                        <div class="metric-value">{{formatDuration .Run.Results.TotalDuration}}</div>
                        <div class="metric-label">Duration</div>
                    </div>
                </div>
            </div>
        </div>

        <div class="card">
            <div class="card-header">Run Details</div>
            <div class="card-body">
                <table>
                    <tr><th>Run ID</th><td>{{.Run.ID}}</td></tr>
                    <tr><th>Workload</th><td>{{.Run.Workload.Name}}</td></tr>
                    <tr><th>Target</th><td>{{.Run.Target.Host}}:{{.Run.Target.Port}}</td></tr>
                    <tr><th>Status</th><td class="status-{{.Run.Status | statusClass}}">{{.Run.Status}}</td></tr>
                    <tr><th>Clients</th><td>{{.Run.Workload.Clients}}</td></tr>
                    <tr><th>Threads</th><td>{{.Run.Workload.Threads}}</td></tr>
                    <tr><th>Pipeline</th><td>{{.Run.Workload.Pipeline}}</td></tr>
                </table>
            </div>
        </div>

        {{if .Run.Results.Operations}}
        <div class="card">
            <div class="card-header">Operations Breakdown</div>
            <div class="card-body">
                <table>
                    <thead>
                        <tr>
                            <th>Operation</th>
                            <th>Ops/sec</th>
                            <th>Avg Latency</th>
                            <th>P99 Latency</th>
                            <th>Hits</th>
                            <th>Misses</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range $op, $stats := .Run.Results.Operations}}
                        <tr>
                            <td><strong>{{$op}}</strong></td>
                            <td>{{formatNumber $stats.OpsPerSec}}</td>
                            <td>{{formatLatency $stats.AvgLatency}}</td>
                            <td>{{formatLatency $stats.P99Latency}}</td>
                            <td>{{$stats.Hits}}</td>
                            <td>{{$stats.Misses}}</td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
        </div>
        {{end}}
        {{end}}

        {{if .Comparison}}
        <!-- Comparison Report -->
        <div class="card">
            <div class="card-header">Comparison Results</div>
            <div class="card-body">
                <table>
                    <thead>
                        <tr>
                            <th>Metric</th>
                            <th>Run 1</th>
                            <th>Run 2</th>
                            <th>Change</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr>
                            <td>Throughput (ops/sec)</td>
                            <td>{{formatNumber .Comparison.Run1.Results.Totals.OpsPerSec}}</td>
                            <td>{{formatNumber .Comparison.Run2.Results.Totals.OpsPerSec}}</td>
                            <td class="{{changeClass .Comparison.ThroughputChange true}}">{{formatPercent .Comparison.ThroughputChange}}</td>
                        </tr>
                        <tr>
                            <td>Avg Latency</td>
                            <td>{{formatLatency .Comparison.Run1.Results.Totals.AvgLatency}}</td>
                            <td>{{formatLatency .Comparison.Run2.Results.Totals.AvgLatency}}</td>
                            <td class="{{changeClass .Comparison.LatencyChange false}}">{{formatPercent .Comparison.LatencyChange}}</td>
                        </tr>
                        <tr>
                            <td>P99 Latency</td>
                            <td>{{formatLatency .Comparison.Run1.Results.Totals.P99Latency}}</td>
                            <td>{{formatLatency .Comparison.Run2.Results.Totals.P99Latency}}</td>
                            <td class="{{changeClass .Comparison.P99LatencyChange false}}">{{formatPercent .Comparison.P99LatencyChange}}</td>
                        </tr>
                    </tbody>
                </table>
            </div>
        </div>
        {{end}}

        {{if .Analysis}}
        <!-- Analysis Reports -->
        {{range .Analysis}}
        <div class="card">
            <div class="card-header">{{.Name}} Analysis</div>
            <div class="card-body">
                <p>{{.Summary}}</p>
                {{if .Findings}}
                <h4 style="margin-top: 1rem;">Findings</h4>
                <ul>
                    {{range .Findings}}
                    <li>
                        <span class="badge {{if eq .Severity "critical"}}badge-danger{{else if eq .Severity "warning"}}badge-warning{{else}}badge-success{{end}}">
                            {{.Severity}}
                        </span>
                        {{.Message}}
                    </li>
                    {{end}}
                </ul>
                {{end}}
                {{if .Recommendations}}
                <h4 style="margin-top: 1rem;">Recommendations</h4>
                <ul>
                    {{range .Recommendations}}
                    <li>{{.}}</li>
                    {{end}}
                </ul>
                {{end}}
            </div>
        </div>
        {{end}}
        {{end}}

        <footer>
            Generated by RedisMeter | {{.Generated}}
        </footer>
    </div>
</body>
</html>`

func formatNumber(n float64) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", n/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fK", n/1000)
	}
	return fmt.Sprintf("%.0f", n)
}

func formatDuration(seconds float64) string {
	if seconds < 60 {
		return fmt.Sprintf("%.1fs", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%.1fm", seconds/60)
	}
	return fmt.Sprintf("%.1fh", seconds/3600)
}

func formatPercent(p float64) string {
	return fmt.Sprintf("%+.2f%%", p)
}

func formatLatency(ms float64) string {
	if ms < 1 {
		return fmt.Sprintf("%.3f ms", ms)
	}
	return fmt.Sprintf("%.2f ms", ms)
}

func statusClass(status string) string {
	return strings.ToLower(status)
}

func changeClass(change float64, higherIsBetter bool) string {
	if higherIsBetter {
		if change > 5 {
			return "change-positive"
		}
		if change < -5 {
			return "change-negative"
		}
	} else {
		if change < -5 {
			return "change-positive"
		}
		if change > 5 {
			return "change-negative"
		}
	}
	return "change-neutral"
}
