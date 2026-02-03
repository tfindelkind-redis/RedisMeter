package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/tfindelkind-redis/redismeter/internal/analysis"
	"github.com/tfindelkind-redis/redismeter/internal/domain"
	"github.com/tfindelkind-redis/redismeter/internal/plugin"
	"github.com/tfindelkind-redis/redismeter/internal/storage"
)

// compareCmd represents the compare command.
var compareCmd = &cobra.Command{
	Use:   "compare <run1> <run2>",
	Short: "Compare two benchmark runs",
	Long: `Compare two benchmark runs and show the differences.

The comparison shows throughput, latency, and error rate differences
between the two runs, along with environment and workload comparisons.

The second run is compared relative to the first (i.e., "how does run2
compare to run1?").`,
	Example: `  # Compare two runs
  redismeter compare abc12345 def67890

  # Compare with JSON output
  redismeter compare abc12345 def67890 --json

  # Compare a run against the active baseline
  redismeter compare abc12345 --baseline`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runCompare,
}

// compareBaselineCmd compares a run to a baseline.
var compareBaselineCmd = &cobra.Command{
	Use:   "baseline <run-id> [baseline-id]",
	Short: "Compare a run against a baseline",
	Long: `Compare a benchmark run against a baseline.

If no baseline ID is provided, the active baseline will be used.
The comparison checks if the run meets the baseline's threshold
requirements.`,
	Example: `  # Compare against active baseline
  redismeter compare baseline abc12345

  # Compare against specific baseline
  redismeter compare baseline abc12345 base-xyz789`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runCompareBaseline,
}

func init() {
	rootCmd.AddCommand(compareCmd)
	compareCmd.AddCommand(compareBaselineCmd)

	// Compare flags
	compareCmd.Flags().Bool("json", false, "Output as JSON")
	compareCmd.Flags().BoolP("baseline", "b", false, "Compare first run against active baseline")

	// Compare baseline flags
	compareBaselineCmd.Flags().Bool("json", false, "Output as JSON")
	compareBaselineCmd.Flags().Bool("fail-on-regression", false, "Exit with error code if comparison fails")
}

func runCompare(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	jsonOutput, _ := cmd.Flags().GetBool("json")
	useBaseline, _ := cmd.Flags().GetBool("baseline")

	// Get storage
	storageType, storagePath := getStorageConfig()
	store, err := newRunStorage(storageType, storagePath)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Shutdown(ctx)

	// If comparing to baseline
	if useBaseline {
		if len(args) < 1 {
			return fmt.Errorf("run ID required")
		}
		return compareToActiveBaseline(ctx, store, args[0], jsonOutput)
	}

	// Need two runs for comparison
	if len(args) < 2 {
		return fmt.Errorf("two run IDs required for comparison")
	}

	// Create comparator
	comparator := analysis.NewComparator(store)

	// Compare runs
	result, err := comparator.CompareRuns(ctx, args[0], args[1])
	if err != nil {
		return fmt.Errorf("comparison failed: %w", err)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	// Pretty print comparison
	printRunComparison(result)

	return nil
}

func runCompareBaseline(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	jsonOutput, _ := cmd.Flags().GetBool("json")
	failOnRegression, _ := cmd.Flags().GetBool("fail-on-regression")

	runID := args[0]
	var baselineID string
	if len(args) > 1 {
		baselineID = args[1]
	}

	// Get storage
	storageType, storagePath := getStorageConfig()
	store, err := newRunStorage(storageType, storagePath)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Shutdown(ctx)

	// Find baseline ID if not provided
	if baselineID == "" {
		activeBaseline, err := findActiveBaseline(ctx, store)
		if err != nil {
			return err
		}
		baselineID = activeBaseline
	}

	// Create comparator
	comparator := analysis.NewComparator(store)

	// Compare to baseline
	result, err := comparator.CompareToBaseline(ctx, runID, baselineID)
	if err != nil {
		return fmt.Errorf("comparison failed: %w", err)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	// Pretty print comparison
	printBaselineComparison(result)

	// Exit with error if regression detected and flag set
	if failOnRegression && !result.Pass {
		os.Exit(1)
	}

	return nil
}

func compareToActiveBaseline(ctx context.Context, store storage.RunStorage, runID string, jsonOutput bool) error {
	// Find active baseline
	baselineID, err := findActiveBaseline(ctx, store)
	if err != nil {
		return err
	}

	// Load run and baseline
	run, err := store.GetRun(ctx, runID)
	if err != nil {
		return fmt.Errorf("failed to load run: %w", err)
	}

	var baseline domain.Baseline
	if err := store.Load(ctx, "baseline", baselineID, &baseline); err != nil {
		return fmt.Errorf("failed to load baseline: %w", err)
	}

	// Create comparison using the comparator
	comparator := analysis.NewComparator(store)
	result, err := comparator.CompareToBaseline(ctx, runID, baselineID)
	if err != nil {
		return fmt.Errorf("comparison failed: %w", err)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			Run      *domain.BenchmarkRun     `json:"run"`
			Baseline *domain.Baseline         `json:"baseline"`
			Result   *domain.ComparisonResult `json:"result"`
		}{run, &baseline, result})
	}

	printBaselineComparison(result)
	return nil
}

func findActiveBaseline(ctx context.Context, store interface {
	Query(ctx context.Context, entityType string, filter plugin.QueryFilter, dest interface{}) error
}) (string, error) {
	var baselines []*domain.Baseline
	filter := plugin.QueryFilter{
		Conditions: map[string]interface{}{"active": true},
		Limit:      1,
	}
	if err := store.Query(ctx, "baseline", filter, &baselines); err != nil {
		return "", fmt.Errorf("failed to query baselines: %w", err)
	}

	if len(baselines) == 0 {
		return "", fmt.Errorf("no active baseline found - use 'redismeter baseline set-active <id>' to set one")
	}

	return baselines[0].ID, nil
}

func printRunComparison(result *analysis.RunComparison) {
	fmt.Printf("Comparison: %s vs %s\n\n", result.Run1ID, result.Run2ID)

	// Overall verdict
	switch result.BetterRun {
	case "run1":
		fmt.Printf("Result: Run 1 performed better\n\n")
	case "run2":
		fmt.Printf("Result: Run 2 performed better\n\n")
	case "equal":
		fmt.Printf("Result: Runs performed equally\n\n")
	default:
		fmt.Printf("Result: Unable to determine\n\n")
	}

	if result.Metrics != nil {
		fmt.Println("Metrics Comparison:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "METRIC\tRUN 1\tRUN 2\tCHANGE")
		fmt.Fprintln(w, "------\t-----\t-----\t------")

		fmt.Fprintf(w, "Throughput (ops/s)\t%.2f\t%.2f\t%s\n",
			result.Metrics.OpsPerSecond1,
			result.Metrics.OpsPerSecond2,
			formatChange(result.Metrics.ThroughputDiff, true),
		)
		fmt.Fprintf(w, "Avg Latency (ms)\t%.3f\t%.3f\t%s\n",
			result.Metrics.AvgLatency1,
			result.Metrics.AvgLatency2,
			formatChange(result.Metrics.AvgLatencyDiff, false),
		)
		fmt.Fprintf(w, "P50 Latency (ms)\t%.3f\t%.3f\t%s\n",
			result.Metrics.P50Latency1,
			result.Metrics.P50Latency2,
			formatChange(result.Metrics.P50LatencyDiff, false),
		)
		fmt.Fprintf(w, "P99 Latency (ms)\t%.3f\t%.3f\t%s\n",
			result.Metrics.P99Latency1,
			result.Metrics.P99Latency2,
			formatChange(result.Metrics.P99LatencyDiff, false),
		)
		fmt.Fprintf(w, "P999 Latency (ms)\t%.3f\t%.3f\t%s\n",
			result.Metrics.P999Latency1,
			result.Metrics.P999Latency2,
			formatChange(result.Metrics.P999LatencyDiff, false),
		)
		fmt.Fprintf(w, "Error Rate\t%.4f\t%.4f\t%s\n",
			result.Metrics.ErrorRate1,
			result.Metrics.ErrorRate2,
			formatChange(result.Metrics.ErrorRateDiff, false),
		)
		w.Flush()
	}

	fmt.Println()

	// Environment comparison
	if result.EnvironmentMatch {
		fmt.Println("Environment: ✓ Matching")
	} else {
		fmt.Println("Environment: ✗ Different")
		for _, diff := range result.EnvironmentDiffs {
			fmt.Printf("  - %s\n", diff)
		}
	}

	// Workload comparison
	if result.WorkloadMatch {
		fmt.Println("Workload: ✓ Matching")
	} else {
		fmt.Printf("Workload: ✗ %s\n", result.WorkloadDiff)
	}
}

func printBaselineComparison(result *domain.ComparisonResult) {
	fmt.Printf("Baseline Comparison\n")
	fmt.Printf("  Run:      %s\n", result.RunID)
	fmt.Printf("  Baseline: %s\n\n", result.BaselineID)

	// Verdict
	switch result.Verdict {
	case "pass":
		fmt.Println("Result: ✓ PASS - Run meets baseline requirements")
	case "warning":
		fmt.Println("Result: ⚠ WARNING - Run has minor regressions")
	case "fail":
		fmt.Println("Result: ✗ FAIL - Run has significant regressions")
	}
	fmt.Println()

	if result.Metrics != nil {
		fmt.Println("Performance Changes (relative to baseline):")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "METRIC\tCHANGE")
		fmt.Fprintln(w, "------\t------")

		fmt.Fprintf(w, "Throughput\t%s\n", formatChange(result.Metrics.ThroughputChange, true))
		fmt.Fprintf(w, "Avg Latency\t%s\n", formatChange(result.Metrics.AvgLatencyChange, false))
		fmt.Fprintf(w, "P50 Latency\t%s\n", formatChange(result.Metrics.P50LatencyChange, false))
		fmt.Fprintf(w, "P99 Latency\t%s\n", formatChange(result.Metrics.P99LatencyChange, false))
		fmt.Fprintf(w, "Error Rate\t%s\n", formatChange(result.Metrics.ErrorRateChange, false))
		w.Flush()
	}

	// Threshold violations
	if len(result.Violations) > 0 {
		fmt.Println("\nThreshold Violations:")
		for _, v := range result.Violations {
			symbol := "⚠"
			if v.Severity == "error" {
				symbol = "✗"
			}
			fmt.Printf("  %s %s: %.2f%% (threshold: %.2f%%)\n",
				symbol, v.Metric, v.Actual, v.Threshold)
		}
	}

	// Environment
	fmt.Println()
	if result.EnvironmentMatch {
		fmt.Println("Environment: ✓ Compatible")
	} else {
		fmt.Println("Environment: ⚠ Differences detected")
		for _, diff := range result.EnvironmentDiffs {
			fmt.Printf("  - %s\n", diff)
		}
	}
}

// formatChange formats a percentage change with color indicators.
func formatChange(change float64, higherIsBetter bool) string {
	var indicator string
	if change > 0.01 {
		if higherIsBetter {
			indicator = "↑" // improvement
		} else {
			indicator = "↓" // regression (higher latency/errors is bad)
		}
	} else if change < -0.01 {
		if higherIsBetter {
			indicator = "↓" // regression (lower throughput is bad)
		} else {
			indicator = "↑" // improvement (lower latency is good)
		}
	} else {
		indicator = "≈" // roughly equal
	}

	if change >= 0 {
		return fmt.Sprintf("%s +%.2f%%", indicator, change)
	}
	return fmt.Sprintf("%s %.2f%%", indicator, change)
}
