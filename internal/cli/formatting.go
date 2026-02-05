package cli

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/tfindelkind-redis/redismeter/internal/domain"
)

// Formatting utilities for enhanced CLI output

// Spinner provides an animated spinner with status updates.
type Spinner struct {
	mu         sync.Mutex
	frames     []string
	frameIdx   int
	message    string
	resource   string
	elapsed    string
	total      int
	completed  int
	active     bool
	stopCh     chan struct{}
	doneCh     chan struct{}
}

// NewSpinner creates a new spinner.
func NewSpinner() *Spinner {
	return &Spinner{
		frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	}
}

// Start begins the spinner animation.
func (s *Spinner) Start(message string) {
	s.mu.Lock()
	// Reset channels for new start
	s.stopCh = make(chan struct{})
	s.doneCh = make(chan struct{})
	s.message = message
	s.active = true
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		defer close(s.doneCh)

		for {
			select {
			case <-s.stopCh:
				return
			case <-ticker.C:
				s.mu.Lock()
				if !s.active {
					s.mu.Unlock()
					return
				}
				s.render()
				s.frameIdx = (s.frameIdx + 1) % len(s.frames)
				s.mu.Unlock()
			}
		}
	}()
}

// Update updates the spinner message and status.
func (s *Spinner) Update(message, resource, elapsed string, completed, total int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.message = message
	s.resource = resource
	s.elapsed = elapsed
	s.completed = completed
	s.total = total
}

// Stop stops the spinner.
func (s *Spinner) Stop() {
	s.mu.Lock()
	if !s.active {
		s.mu.Unlock()
		return
	}
	s.active = false
	stopCh := s.stopCh
	doneCh := s.doneCh
	s.mu.Unlock()

	close(stopCh)
	<-doneCh

	// Clear the line
	fmt.Print("\r\033[K")
}

// Success stops the spinner with a success message.
func (s *Spinner) Success(message string) {
	s.Stop()
	fmt.Printf("  %s %s\n", color.GreenString("✓"), message)
}

// Fail stops the spinner with a failure message.
func (s *Spinner) Fail(message string) {
	s.Stop()
	fmt.Printf("  %s %s\n", color.RedString("✗"), message)
}

func (s *Spinner) render() {
	frame := color.CyanString(s.frames[s.frameIdx])

	// Build status line
	var status strings.Builder
	status.WriteString(fmt.Sprintf("\r  %s %s", frame, s.message))

	if s.resource != "" {
		status.WriteString(fmt.Sprintf(" %s", color.YellowString(s.resource)))
	}

	if s.elapsed != "" {
		status.WriteString(fmt.Sprintf(" %s", color.HiBlackString("[%s]", s.elapsed)))
	}

	if s.total > 0 {
		progress := float64(s.completed) / float64(s.total)
		status.WriteString(fmt.Sprintf(" %s", color.HiBlackString("(%d/%d)", s.completed, s.total)))
		// Mini progress bar
		bar := miniProgressBar(progress, 10)
		status.WriteString(fmt.Sprintf(" %s", bar))
	}

	// Pad to clear previous line content
	status.WriteString(strings.Repeat(" ", 20))

	fmt.Print(status.String())
}

func miniProgressBar(progress float64, width int) string {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	filled := int(progress * float64(width))
	empty := width - filled
	return fmt.Sprintf("%s%s",
		color.GreenString(strings.Repeat("█", filled)),
		color.HiBlackString(strings.Repeat("░", empty)))
}

// InfraProgressDisplay provides a rich progress display for infrastructure operations.
type InfraProgressDisplay struct {
	spinner   *Spinner
	phase     string
	startTime time.Time
	resources map[string]string // resource -> status
	mu        sync.Mutex
}

// NewInfraProgressDisplay creates a new infrastructure progress display.
func NewInfraProgressDisplay() *InfraProgressDisplay {
	return &InfraProgressDisplay{
		spinner:   NewSpinner(),
		resources: make(map[string]string),
		startTime: time.Now(),
	}
}

// Start begins the progress display.
func (p *InfraProgressDisplay) Start(phase string) {
	p.mu.Lock()
	p.phase = phase
	p.startTime = time.Now()
	p.mu.Unlock()

	var msg string
	switch phase {
	case "init":
		msg = "Initializing Terraform..."
	case "apply":
		msg = "Creating resources..."
	case "destroy":
		msg = "Destroying resources..."
	default:
		msg = "Working..."
	}
	p.spinner.Start(msg)
}

// Update processes a terraform event.
func (p *InfraProgressDisplay) Update(action, resource, elapsed string, completed, total int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Track resource status
	if resource != "" {
		p.resources[resource] = action
	}

	var msg string
	switch action {
	case "creating":
		msg = "Creating"
	case "created":
		msg = "Created"
	case "destroying":
		msg = "Destroying"
	case "destroyed":
		msg = "Destroyed"
	case "waiting":
		msg = "Waiting for"
	case "refreshing":
		msg = "Refreshing"
	case "complete":
		msg = "Complete"
	default:
		msg = p.phase
	}

	p.spinner.Update(msg, resource, elapsed, completed, total)
}

// Success marks the operation as successful.
func (p *InfraProgressDisplay) Success(message string) {
	p.spinner.Success(message)
}

// Fail marks the operation as failed.
func (p *InfraProgressDisplay) Fail(message string) {
	p.spinner.Fail(message)
}

// Stop stops the display.
func (p *InfraProgressDisplay) Stop() {
	p.spinner.Stop()
}

// TableWriter helps create formatted table output.
type TableWriter struct {
	headers []string
	rows    [][]string
	widths  []int
}

// NewTableWriter creates a new table writer with the given headers.
func NewTableWriter(headers ...string) *TableWriter {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	return &TableWriter{
		headers: headers,
		widths:  widths,
	}
}

// AddRow adds a row to the table.
func (t *TableWriter) AddRow(cells ...string) {
	// Ensure we have the right number of cells
	row := make([]string, len(t.headers))
	for i := range row {
		if i < len(cells) {
			row[i] = cells[i]
		}
		if len(row[i]) > t.widths[i] {
			t.widths[i] = len(row[i])
		}
	}
	t.rows = append(t.rows, row)
}

// Render outputs the table.
func (t *TableWriter) Render() {
	// Header
	t.printRow(t.headers, true)
	t.printSeparator()

	// Rows
	for _, row := range t.rows {
		t.printRow(row, false)
	}
}

func (t *TableWriter) printRow(cells []string, isHeader bool) {
	for i, cell := range cells {
		if isHeader {
			color.New(color.Bold).Printf("%-*s", t.widths[i]+2, cell)
		} else {
			fmt.Printf("%-*s", t.widths[i]+2, cell)
		}
	}
	fmt.Println()
}

func (t *TableWriter) printSeparator() {
	for _, w := range t.widths {
		fmt.Print(strings.Repeat("─", w+2))
	}
	fmt.Println()
}

// Sparkline generates a sparkline string from a slice of values.
func Sparkline(values []float64) string {
	if len(values) == 0 {
		return ""
	}

	// Find min and max
	min, max := values[0], values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	// Sparkline characters (from low to high)
	chars := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

	var sb strings.Builder
	for _, v := range values {
		// Normalize value to 0-7 range
		idx := 0
		if max > min {
			normalized := (v - min) / (max - min)
			idx = int(normalized * 7)
			if idx > 7 {
				idx = 7
			}
		}
		sb.WriteRune(chars[idx])
	}

	return sb.String()
}

// Histogram generates an ASCII histogram.
func Histogram(values []float64, bins int, width int) string {
	if len(values) == 0 || bins <= 0 {
		return ""
	}

	// Find min and max
	min, max := values[0], values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	if max == min {
		max = min + 1
	}

	// Create bins
	binWidth := (max - min) / float64(bins)
	counts := make([]int, bins)
	maxCount := 0

	for _, v := range values {
		idx := int((v - min) / binWidth)
		if idx >= bins {
			idx = bins - 1
		}
		counts[idx]++
		if counts[idx] > maxCount {
			maxCount = counts[idx]
		}
	}

	// Render histogram
	var sb strings.Builder
	for i, count := range counts {
		rangeStart := min + float64(i)*binWidth
		rangeEnd := rangeStart + binWidth

		// Calculate bar width
		barWidth := 0
		if maxCount > 0 {
			barWidth = int(float64(count) / float64(maxCount) * float64(width))
		}

		sb.WriteString(fmt.Sprintf("%8.2f - %8.2f │", rangeStart, rangeEnd))
		sb.WriteString(strings.Repeat("█", barWidth))
		sb.WriteString(fmt.Sprintf(" %d\n", count))
	}

	return sb.String()
}

// BarChart generates a horizontal bar chart.
func BarChart(labels []string, values []float64, width int) string {
	if len(labels) == 0 || len(labels) != len(values) {
		return ""
	}

	// Find max value and max label length
	maxVal := values[0]
	maxLabelLen := len(labels[0])
	for i, v := range values {
		if v > maxVal {
			maxVal = v
		}
		if len(labels[i]) > maxLabelLen {
			maxLabelLen = len(labels[i])
		}
	}

	if maxVal == 0 {
		maxVal = 1
	}

	var sb strings.Builder
	for i, label := range labels {
		barWidth := int(values[i] / maxVal * float64(width))
		sb.WriteString(fmt.Sprintf("%*s │", maxLabelLen, label))
		sb.WriteString(strings.Repeat("█", barWidth))
		sb.WriteString(fmt.Sprintf(" %.2f\n", values[i]))
	}

	return sb.String()
}

// ColoredMetric returns a colored string based on thresholds.
func ColoredMetric(value float64, format string, goodThreshold, badThreshold float64, higherIsBetter bool) string {
	formatted := fmt.Sprintf(format, value)

	if higherIsBetter {
		if value >= goodThreshold {
			return color.GreenString(formatted)
		} else if value <= badThreshold {
			return color.RedString(formatted)
		}
	} else {
		if value <= goodThreshold {
			return color.GreenString(formatted)
		} else if value >= badThreshold {
			return color.RedString(formatted)
		}
	}

	return color.YellowString(formatted)
}

// PercentChange formats a percentage change with color.
func PercentChange(change float64) string {
	formatted := fmt.Sprintf("%+.2f%%", change)
	if change > 5 {
		return color.GreenString(formatted)
	} else if change < -5 {
		return color.RedString(formatted)
	}
	return formatted
}

// ProgressBar generates a progress bar string.
func ProgressBar(progress float64, width int) string {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}

	filled := int(progress * float64(width))
	empty := width - filled

	return fmt.Sprintf("[%s%s] %.0f%%",
		strings.Repeat("█", filled),
		strings.Repeat("░", empty),
		progress*100)
}

// FormatDuration formats a duration in seconds to a human-readable string.
func FormatDuration(seconds float64) string {
	if seconds < 60 {
		return fmt.Sprintf("%.1fs", seconds)
	} else if seconds < 3600 {
		return fmt.Sprintf("%.1fm", seconds/60)
	}
	return fmt.Sprintf("%.1fh", seconds/3600)
}

// FormatBytes formats bytes to a human-readable string.
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatNumber formats large numbers with suffixes (K, M, B).
func FormatNumber(n float64) string {
	if math.Abs(n) < 1000 {
		return fmt.Sprintf("%.0f", n)
	} else if math.Abs(n) < 1000000 {
		return fmt.Sprintf("%.1fK", n/1000)
	} else if math.Abs(n) < 1000000000 {
		return fmt.Sprintf("%.1fM", n/1000000)
	}
	return fmt.Sprintf("%.1fB", n/1000000000)
}

// PrintRunSummary prints a formatted summary of a benchmark run.
func PrintRunSummary(run *domain.BenchmarkRun) {
	// Header
	color.New(color.FgCyan, color.Bold).Printf("\n══════════════════════════════════════════════════════════════\n")
	color.New(color.FgCyan, color.Bold).Printf("                    BENCHMARK RESULTS\n")
	color.New(color.FgCyan, color.Bold).Printf("══════════════════════════════════════════════════════════════\n\n")

	// Basic Info
	fmt.Printf("  Run ID:    %s\n", run.ID)
	fmt.Printf("  Workload:  %s\n", color.CyanString(run.Workload.Name))
	fmt.Printf("  Target:    %s:%d\n", run.Target.Host, run.Target.Port)
	fmt.Printf("  Status:    %s\n", statusColor(string(run.Status)))
	fmt.Println()

	// Get summary metrics safely
	summary := getRunSummary(run)

	// Key Metrics
	color.New(color.Bold).Println("  Key Metrics:")
	fmt.Println("  " + strings.Repeat("─", 56))

	// Throughput
	throughput := summary.OpsPerSecond
	fmt.Printf("  Throughput:    %s ops/sec\n",
		ColoredMetric(throughput, "%.0f", 50000, 10000, true))

	// Latency
	avgLatency := summary.AvgLatencyMs
	p99Latency := summary.P99LatencyMs
	fmt.Printf("  Avg Latency:   %s ms\n",
		ColoredMetric(avgLatency, "%.3f", 1.0, 10.0, false))
	fmt.Printf("  P99 Latency:   %s ms\n",
		ColoredMetric(p99Latency, "%.3f", 5.0, 50.0, false))

	// Data transfer
	if summary.BytesPerSecond > 0 {
		fmt.Printf("  Throughput:    %s/sec\n",
			FormatBytes(int64(summary.BytesPerSecond)))
	}

	fmt.Println()

	// Operation breakdown if available
	if run.Results != nil && len(run.Results.ByOperation) > 0 {
		color.New(color.Bold).Println("  Operations:")
		fmt.Println("  " + strings.Repeat("─", 56))

		// Sort operations by name
		ops := make([]string, 0, len(run.Results.ByOperation))
		for op := range run.Results.ByOperation {
			ops = append(ops, op)
		}
		sort.Strings(ops)

		for _, op := range ops {
			stats := run.Results.ByOperation[op]
			fmt.Printf("  %-12s %8.0f ops/sec  avg: %6.3f ms  p99: %6.3f ms\n",
				op+":",
				stats.OpsPerSecond,
				stats.AvgLatencyMs,
				stats.P99LatencyMs)
		}
		fmt.Println()
	}

	color.New(color.FgCyan, color.Bold).Printf("══════════════════════════════════════════════════════════════\n\n")
}

func statusColor(status string) string {
	switch strings.ToLower(status) {
	case "completed":
		return color.GreenString(status)
	case "running":
		return color.CyanString(status)
	case "failed":
		return color.RedString(status)
	default:
		return status
	}
}

// completionCmd generates shell completions.
var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for your shell.

To load completions:

Bash:
  $ source <(redismeter completion bash)
  # To load completions for each session, execute once:
  # Linux:
  $ redismeter completion bash > /etc/bash_completion.d/redismeter
  # macOS:
  $ redismeter completion bash > $(brew --prefix)/etc/bash_completion.d/redismeter

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc
  
  # To load completions for each session, execute once:
  $ redismeter completion zsh > "${fpath[1]}/_redismeter"
  
  # You will need to start a new shell for this setup to take effect.

Fish:
  $ redismeter completion fish | source
  # To load completions for each session, execute once:
  $ redismeter completion fish > ~/.config/fish/completions/redismeter.fish

PowerShell:
  PS> redismeter completion powershell | Out-String | Invoke-Expression
  # To load completions for every new session, run:
  PS> redismeter completion powershell > redismeter.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}

// getRunSummary safely extracts summary metrics from a run.
func getRunSummary(run *domain.BenchmarkRun) *domain.SummaryMetrics {
	if run != nil && run.Results != nil && run.Results.Summary != nil {
		return run.Results.Summary
	}
	return &domain.SummaryMetrics{}
}
