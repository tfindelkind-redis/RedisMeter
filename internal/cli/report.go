package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tfindelkind-redis/redismeter/internal/analysis"
	"github.com/tfindelkind-redis/redismeter/internal/domain"
	"github.com/tfindelkind-redis/redismeter/internal/reporter"
)

// reportCmd generates benchmark reports.
var reportCmd = &cobra.Command{
	Use:   "report <run-id>",
	Short: "Generate a benchmark report",
	Long: `Generate a formatted report for a benchmark run.

Supported formats:
  - html:     Interactive HTML report with charts
  - markdown: Markdown report for documentation
  - json:     JSON report for programmatic access
  - text:     Plain text report for terminal
  - slack:    Slack message format

Reports can include:
  - Run summary and key metrics
  - Operations breakdown
  - Latency percentiles
  - Environment information
  - Comparison with baseline or other runs
  - Analysis results
`,
	Example: `  # Generate HTML report
  redismeter report run-abc123 --format html -o report.html

  # Generate Markdown report to stdout
  redismeter report run-abc123 --format markdown

  # Generate report with comparison
  redismeter report run-abc123 --compare run-xyz789

  # Generate report with analysis
  redismeter report run-abc123 --analyze

  # List available formats
  redismeter report --formats`,
	Args: cobra.MaximumNArgs(1),
	RunE: runReport,
}

func init() {
	rootCmd.AddCommand(reportCmd)

	reportCmd.Flags().StringP("format", "f", "text", "Report format (html, markdown, json, text, slack)")
	reportCmd.Flags().StringP("output", "o", "", "Output file (default: stdout)")
	reportCmd.Flags().String("title", "", "Report title")
	reportCmd.Flags().String("description", "", "Report description")
	reportCmd.Flags().String("compare", "", "Compare with another run ID")
	reportCmd.Flags().String("baseline", "", "Compare with baseline ID")
	reportCmd.Flags().Bool("analyze", false, "Include analysis in report")
	reportCmd.Flags().StringSlice("analyzers", []string{}, "Specific analyzers to include")
	reportCmd.Flags().Bool("formats", false, "List available formats")
}

func runReport(cmd *cobra.Command, args []string) error {
	// List formats
	if listFormats, _ := cmd.Flags().GetBool("formats"); listFormats {
		fmt.Println("Available report formats:")
		fmt.Println()
		for _, r := range reporter.DefaultRegistry.List() {
			fmt.Printf("  %-12s %s\n", r.Name(), r.Description())
		}
		return nil
	}

	// Require run ID
	if len(args) < 1 {
		return fmt.Errorf("run ID required")
	}

	runID := args[0]

	// Initialize storage using existing helper
	storageType, storagePath := getStorageConfig()
	storage, err := newRunStorage(storageType, storagePath)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	if closer, ok := storage.(interface{ Close() error }); ok {
		defer closer.Close()
	}

	ctx := cmd.Context()

	// Load the run
	run, err := storage.GetRun(ctx, runID)
	if err != nil {
		return fmt.Errorf("failed to load run: %w", err)
	}

	// Build report
	report := &reporter.Report{
		Run: run,
	}

	// Set title and description
	if title, _ := cmd.Flags().GetString("title"); title != "" {
		report.Title = title
	} else {
		report.Title = fmt.Sprintf("Benchmark Report: %s", run.Workload.Name)
	}

	if desc, _ := cmd.Flags().GetString("description"); desc != "" {
		report.Description = desc
	}

	// Add comparison if requested
	if compareID, _ := cmd.Flags().GetString("compare"); compareID != "" {
		run2, err := storage.GetRun(ctx, compareID)
		if err != nil {
			return fmt.Errorf("failed to load comparison run: %w", err)
		}

		comparator := analysis.NewComparator(nil) // storage not needed for direct comparison
		comparison := comparator.Compare(run, run2)
		report.Comparison = comparison
		report.Runs = []*domain.BenchmarkRun{run, run2}
		report.Title = fmt.Sprintf("Comparison Report: %s vs %s", run.ID[:8], run2.ID[:8])
	}

	// Add baseline comparison if requested
	if baselineID, _ := cmd.Flags().GetString("baseline"); baselineID != "" {
		var baseline domain.Baseline
		if err := storage.Load(ctx, "baseline", baselineID, &baseline); err != nil {
			return fmt.Errorf("failed to load baseline: %w", err)
		}

		comparator := analysis.NewComparator(nil)
		baselineComparison := comparator.CompareRunToBaselineMetrics(run, &baseline)
		report.BaselineComparison = baselineComparison
		report.Baseline = &baseline
		report.Title = fmt.Sprintf("Baseline Comparison: %s vs %s", run.ID[:8], baseline.Name)
	}

	// Add analysis if requested
	if analyze, _ := cmd.Flags().GetBool("analyze"); analyze {
		analyzerNames, _ := cmd.Flags().GetStringSlice("analyzers")

		var analyzers []analysis.Analyzer
		if len(analyzerNames) > 0 {
			for _, name := range analyzerNames {
				if a, ok := analysis.DefaultRegistry.Get(name); ok {
					analyzers = append(analyzers, a)
				}
			}
		} else {
			analyzers = analysis.DefaultRegistry.All()
		}

		var analysisReports []*analysis.AnalysisReport
		for _, a := range analyzers {
			analysisReport, err := a.Analyze(ctx, run)
			if err != nil {
				continue
			}
			analysisReports = append(analysisReports, analysisReport)
		}
		report.Analysis = analysisReports
	}

	// Get format
	formatName, _ := cmd.Flags().GetString("format")
	formatName = strings.ToLower(formatName)

	rep, ok := reporter.DefaultRegistry.Get(formatName)
	if !ok {
		return fmt.Errorf("unknown format: %s (use --formats to list available)", formatName)
	}

	// Determine output
	output := os.Stdout
	if outputPath, _ := cmd.Flags().GetString("output"); outputPath != "" {
		f, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer f.Close()
		output = f
	}

	// Generate report
	if err := rep.Generate(ctx, report, output); err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	// Print success message if writing to file
	if outputPath, _ := cmd.Flags().GetString("output"); outputPath != "" {
		fmt.Fprintf(os.Stderr, "Report written to %s\n", outputPath)
	}

	return nil
}


