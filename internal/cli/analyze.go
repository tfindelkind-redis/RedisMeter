package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tfindelkind-redis/redismeter/internal/analysis"
)

// analyzeCmd represents the analyze command.
var analyzeCmd = &cobra.Command{
	Use:   "analyze <run-id>",
	Short: "Analyze a benchmark run",
	Long: `Perform detailed analysis on a benchmark run.

The analyze command runs various analyzers to identify performance
issues, potential regressions, and provide recommendations for
improvement.

Available analyzers:
  regression  - Checks against performance thresholds
  latency     - Analyzes latency distribution
  throughput  - Analyzes throughput characteristics`,
	Example: `  # Analyze a single run with all analyzers
  redismeter analyze abc12345

  # Analyze with specific analyzer
  redismeter analyze abc12345 --analyzer regression

  # Analyze with JSON output
  redismeter analyze abc12345 --json

  # Analyze with custom thresholds
  redismeter analyze abc12345 --min-throughput 5000 --max-latency 5`,
	Args: cobra.ExactArgs(1),
	RunE: runAnalyze,
}

func init() {
	rootCmd.AddCommand(analyzeCmd)

	analyzeCmd.Flags().StringP("analyzer", "a", "all", "Analyzer to use: regression, latency, throughput, all")
	analyzeCmd.Flags().Bool("json", false, "Output as JSON")

	// Custom thresholds
	analyzeCmd.Flags().Float64("min-throughput", 1000, "Minimum acceptable throughput (ops/s)")
	analyzeCmd.Flags().Float64("max-latency", 10, "Maximum acceptable average latency (ms)")
	analyzeCmd.Flags().Float64("max-p99", 50, "Maximum acceptable P99 latency (ms)")
	analyzeCmd.Flags().Float64("max-error-rate", 0.001, "Maximum acceptable error rate (fraction)")
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	runID := args[0]

	analyzerName, _ := cmd.Flags().GetString("analyzer")
	jsonOutput, _ := cmd.Flags().GetBool("json")
	minThroughput, _ := cmd.Flags().GetFloat64("min-throughput")
	maxLatency, _ := cmd.Flags().GetFloat64("max-latency")
	maxP99, _ := cmd.Flags().GetFloat64("max-p99")
	maxErrorRate, _ := cmd.Flags().GetFloat64("max-error-rate")

	// Get storage
	storageType, storagePath := getStorageConfig()
	store, err := newRunStorage(storageType, storagePath)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Shutdown(ctx)

	// Load run
	run, err := store.GetRun(ctx, runID)
	if err != nil {
		return fmt.Errorf("failed to load run: %w", err)
	}

	if run.Results == nil || run.Results.Summary == nil {
		return fmt.Errorf("run %s has no results to analyze", runID)
	}

	// Build analyzer registry with custom thresholds
	registry := analysis.NewAnalyzerRegistry()

	thresholds := analysis.RegressionThresholds{
		ThroughputMinOps:    minThroughput,
		LatencyMaxAvg:       maxLatency,
		LatencyMaxP99:       maxP99,
		LatencyMaxP999:      maxP99 * 2,
		ErrorRateMax:        maxErrorRate,
		ThroughputVariation: 10,
		LatencyVariation:    20,
	}
	registry.Register(analysis.NewRegressionAnalyzer(thresholds))
	registry.Register(analysis.NewLatencyAnalyzer())
	registry.Register(analysis.NewThroughputAnalyzer())

	// Run analysis
	var reports []*analysis.AnalysisReport

	if analyzerName == "all" {
		reports, err = registry.AnalyzeAll(ctx, run)
		if err != nil {
			return fmt.Errorf("analysis failed: %w", err)
		}
	} else {
		analyzer, ok := registry.Get(analyzerName)
		if !ok {
			return fmt.Errorf("unknown analyzer: %s", analyzerName)
		}
		report, err := analyzer.Analyze(ctx, run)
		if err != nil {
			return fmt.Errorf("analysis failed: %w", err)
		}
		reports = append(reports, report)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(reports)
	}

	// Pretty print reports
	fmt.Printf("Analysis of run: %s\n", runID)
	if run.Name != "" {
		fmt.Printf("Name: %s\n", run.Name)
	}
	fmt.Println()

	for _, report := range reports {
		printAnalysisReport(report)
	}

	return nil
}

func printAnalysisReport(report *analysis.AnalysisReport) {
	// Status icon
	statusIcon := "✓"
	switch report.Status {
	case "warning":
		statusIcon = "⚠"
	case "critical":
		statusIcon = "✗"
	}

	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("%s %s Analysis (Score: %d/100)\n", statusIcon, report.Analyzer, report.Score)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("Summary: %s\n\n", report.Summary)

	// Findings
	if len(report.Findings) > 0 {
		fmt.Println("Findings:")
		for _, finding := range report.Findings {
			icon := "ℹ"
			switch finding.Severity {
			case "warning":
				icon = "⚠"
			case "error":
				icon = "✗"
			}
			fmt.Printf("  %s [%s] %s\n", icon, finding.Category, finding.Title)
			fmt.Printf("    %s\n", finding.Description)
			if finding.Value != nil && finding.Expected != nil {
				fmt.Printf("    Value: %v, Expected: %v\n", finding.Value, finding.Expected)
			}
		}
		fmt.Println()
	}

	// Recommendations
	if len(report.Recommendations) > 0 {
		fmt.Println("Recommendations:")
		for _, rec := range report.Recommendations {
			priorityIcon := "→"
			switch rec.Priority {
			case "high":
				priorityIcon = "!"
			case "medium":
				priorityIcon = "·"
			}
			fmt.Printf("  %s [%s] %s\n", priorityIcon, rec.Priority, rec.Title)
			fmt.Printf("    %s\n", rec.Description)
			if rec.Impact != "" {
				fmt.Printf("    Impact: %s\n", rec.Impact)
			}
		}
		fmt.Println()
	}

	// Key metrics
	if len(report.Metrics) > 0 {
		fmt.Println("Key Metrics:")
		for name, value := range report.Metrics {
			fmt.Printf("  %s: %v\n", name, value)
		}
		fmt.Println()
	}
}
