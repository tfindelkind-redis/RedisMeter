package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/tfindelkind-redis/redismeter/internal/domain"
	"github.com/tfindelkind-redis/redismeter/internal/plugin"
)

// baselineCmd represents the baseline command group.
var baselineCmd = &cobra.Command{
	Use:   "baseline",
	Short: "Manage performance baselines",
	Long: `Manage performance baselines for benchmark comparison.

Baselines capture performance metrics from a benchmark run that can be
used as a reference point for future comparisons. They help identify
performance regressions or improvements over time.`,
}

// baselineCreateCmd creates a baseline from a run.
var baselineCreateCmd = &cobra.Command{
	Use:   "create <run-id>",
	Short: "Create a baseline from a benchmark run",
	Long: `Create a new baseline from an existing benchmark run.

The baseline captures the performance metrics from the specified run
and can be used for future comparisons.`,
	Example: `  # Create a baseline from a run
  redismeter baseline create abc12345

  # Create a named baseline
  redismeter baseline create abc12345 --name "production-v1.0"

  # Create a baseline with description
  redismeter baseline create abc12345 --name "release-v2" --description "Baseline for v2 release"

  # Create and set as active baseline
  redismeter baseline create abc12345 --name "current" --active`,
	Args: cobra.ExactArgs(1),
	RunE: runBaselineCreate,
}

// baselineListCmd lists all baselines.
var baselineListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all baselines",
	Long:  `List all saved performance baselines.`,
	Example: `  # List all baselines
  redismeter baseline list

  # List with JSON output
  redismeter baseline list --json`,
	RunE: runBaselineList,
}

// baselineShowCmd shows baseline details.
var baselineShowCmd = &cobra.Command{
	Use:   "show <baseline-id>",
	Short: "Show baseline details",
	Long:  `Display detailed information about a specific baseline.`,
	Example: `  # Show baseline details
  redismeter baseline show base-abc123

  # Show with JSON output
  redismeter baseline show base-abc123 --json`,
	Args: cobra.ExactArgs(1),
	RunE: runBaselineShow,
}

// baselineDeleteCmd deletes a baseline.
var baselineDeleteCmd = &cobra.Command{
	Use:   "delete <baseline-id>",
	Short: "Delete a baseline",
	Long:  `Delete a performance baseline.`,
	Example: `  # Delete a baseline
  redismeter baseline delete base-abc123

  # Delete without confirmation
  redismeter baseline delete base-abc123 --force`,
	Args: cobra.ExactArgs(1),
	RunE: runBaselineDelete,
}

// baselineSetActiveCmd sets a baseline as active.
var baselineSetActiveCmd = &cobra.Command{
	Use:   "set-active <baseline-id>",
	Short: "Set a baseline as the active baseline",
	Long: `Set a baseline as the active/default baseline for comparisons.

Only one baseline can be active at a time. The active baseline
is used by default when running comparison commands.`,
	Example: `  # Set active baseline
  redismeter baseline set-active base-abc123`,
	Args: cobra.ExactArgs(1),
	RunE: runBaselineSetActive,
}

func init() {
	rootCmd.AddCommand(baselineCmd)
	baselineCmd.AddCommand(baselineCreateCmd)
	baselineCmd.AddCommand(baselineListCmd)
	baselineCmd.AddCommand(baselineShowCmd)
	baselineCmd.AddCommand(baselineDeleteCmd)
	baselineCmd.AddCommand(baselineSetActiveCmd)

	// Create flags
	baselineCreateCmd.Flags().StringP("name", "n", "", "Name for the baseline")
	baselineCreateCmd.Flags().StringP("description", "d", "", "Description for the baseline")
	baselineCreateCmd.Flags().Bool("active", false, "Set as active baseline")
	baselineCreateCmd.Flags().StringSlice("tags", nil, "Tags for the baseline")
	baselineCreateCmd.Flags().Float64("max-throughput-regression", 0.05, "Maximum allowed throughput regression (as fraction, e.g., 0.05 = 5%)")
	baselineCreateCmd.Flags().Float64("max-latency-regression", 0.10, "Maximum allowed latency regression")
	baselineCreateCmd.Flags().Float64("max-p99-regression", 0.15, "Maximum allowed P99 latency regression")

	// List flags
	baselineListCmd.Flags().Bool("json", false, "Output as JSON")
	baselineListCmd.Flags().Bool("active-only", false, "Show only active baseline")

	// Show flags
	baselineShowCmd.Flags().Bool("json", false, "Output as JSON")

	// Delete flags
	baselineDeleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation")
}

func runBaselineCreate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	runID := args[0]

	// Get storage
	storageType, storagePath := getStorageConfig()
	store, err := newRunStorage(storageType, storagePath)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Shutdown(ctx)

	// Load the run
	run, err := store.GetRun(ctx, runID)
	if err != nil {
		return fmt.Errorf("failed to load run %s: %w", runID, err)
	}

	// Verify run has results
	if run.Results == nil || run.Results.Summary == nil {
		return fmt.Errorf("run %s has no results - cannot create baseline", runID)
	}

	// Get flags
	name, _ := cmd.Flags().GetString("name")
	description, _ := cmd.Flags().GetString("description")
	active, _ := cmd.Flags().GetBool("active")
	tags, _ := cmd.Flags().GetStringSlice("tags")
	maxThroughputReg, _ := cmd.Flags().GetFloat64("max-throughput-regression")
	maxLatencyReg, _ := cmd.Flags().GetFloat64("max-latency-regression")
	maxP99Reg, _ := cmd.Flags().GetFloat64("max-p99-regression")

	// Generate name if not provided
	if name == "" {
		name = fmt.Sprintf("baseline-%s", time.Now().Format("20060102-150405"))
	}

	// Create baseline
	baseline := &domain.Baseline{
		ID:          fmt.Sprintf("base-%s", uuid.New().String()[:8]),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		RunID:       runID,
		Name:        name,
		Description: description,
		Tags:        tags,
		Active:      active,
		Metrics:     run.Results.Summary,
		Workload:    run.Workload,
		Thresholds: &domain.BaselineThresholds{
			MaxThroughputRegression: maxThroughputReg,
			MaxLatencyRegression:    maxLatencyReg,
			MaxP99Regression:        maxP99Reg,
		},
	}

	// If setting as active, deactivate other baselines
	if active {
		if err := deactivateAllBaselines(ctx, store); err != nil {
			return fmt.Errorf("failed to deactivate existing baselines: %w", err)
		}
	}

	// Save baseline
	if _, err := store.Save(ctx, "baseline", baseline); err != nil {
		return fmt.Errorf("failed to save baseline: %w", err)
	}

	fmt.Printf("Created baseline: %s\n", baseline.ID)
	fmt.Printf("  Name: %s\n", baseline.Name)
	fmt.Printf("  Run:  %s\n", baseline.RunID)
	if active {
		fmt.Printf("  Status: ACTIVE\n")
	}
	fmt.Printf("\nMetrics captured:\n")
	fmt.Printf("  Ops/sec:     %.2f\n", baseline.Metrics.OpsPerSecond)
	fmt.Printf("  Avg latency: %.3f ms\n", baseline.Metrics.AvgLatencyMs)
	fmt.Printf("  P99 latency: %.3f ms\n", baseline.Metrics.P99LatencyMs)

	return nil
}

func runBaselineList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	jsonOutput, _ := cmd.Flags().GetBool("json")
	activeOnly, _ := cmd.Flags().GetBool("active-only")

	// Get storage
	storageType, storagePath := getStorageConfig()
	store, err := newRunStorage(storageType, storagePath)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Shutdown(ctx)

	// Query baselines
	var baselines []*domain.Baseline
	filter := plugin.QueryFilter{
		Limit: 100,
	}
	if activeOnly {
		filter.Conditions = map[string]interface{}{"active": true}
	}
	if err := store.Query(ctx, "baseline", filter, &baselines); err != nil {
		return fmt.Errorf("failed to list baselines: %w", err)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(baselines)
	}

	if len(baselines) == 0 {
		fmt.Println("No baselines found.")
		return nil
	}

	// Table output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tRUN\tACTIVE\tOPS/SEC\tP99 LATENCY\tCREATED")
	fmt.Fprintln(w, "--\t----\t---\t------\t-------\t-----------\t-------")

	for _, b := range baselines {
		active := ""
		if b.Active {
			active = "✓"
		}
		opsPerSec := ""
		p99 := ""
		if b.Metrics != nil {
			opsPerSec = fmt.Sprintf("%.0f", b.Metrics.OpsPerSecond)
			p99 = fmt.Sprintf("%.3f ms", b.Metrics.P99LatencyMs)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			b.ID,
			truncate(b.Name, 20),
			truncate(b.RunID, 12),
			active,
			opsPerSec,
			p99,
			b.CreatedAt.Format("2006-01-02 15:04"),
		)
	}
	w.Flush()

	return nil
}

func runBaselineShow(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	baselineID := args[0]

	jsonOutput, _ := cmd.Flags().GetBool("json")

	// Get storage
	storageType, storagePath := getStorageConfig()
	store, err := newRunStorage(storageType, storagePath)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Shutdown(ctx)

	// Load baseline
	var baseline domain.Baseline
	if err := store.Load(ctx, "baseline", baselineID, &baseline); err != nil {
		return fmt.Errorf("baseline not found: %s", baselineID)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(baseline)
	}

	// Pretty print
	fmt.Printf("Baseline: %s\n", baseline.ID)
	fmt.Printf("  Name:        %s\n", baseline.Name)
	if baseline.Description != "" {
		fmt.Printf("  Description: %s\n", baseline.Description)
	}
	fmt.Printf("  Run ID:      %s\n", baseline.RunID)
	fmt.Printf("  Created:     %s\n", baseline.CreatedAt.Format(time.RFC3339))
	fmt.Printf("  Active:      %v\n", baseline.Active)

	if len(baseline.Tags) > 0 {
		fmt.Printf("  Tags:        %v\n", baseline.Tags)
	}

	if baseline.Workload != nil {
		fmt.Printf("\nWorkload:\n")
		fmt.Printf("  Name: %s\n", baseline.Workload.Name)
	}

	if baseline.Metrics != nil {
		fmt.Printf("\nMetrics:\n")
		fmt.Printf("  Ops/sec:       %.2f\n", baseline.Metrics.OpsPerSecond)
		fmt.Printf("  Avg latency:   %.3f ms\n", baseline.Metrics.AvgLatencyMs)
		fmt.Printf("  P50 latency:   %.3f ms\n", baseline.Metrics.P50LatencyMs)
		fmt.Printf("  P99 latency:   %.3f ms\n", baseline.Metrics.P99LatencyMs)
		fmt.Printf("  P999 latency:  %.3f ms\n", baseline.Metrics.P999LatencyMs)
		fmt.Printf("  Error rate:    %.2f%%\n", baseline.Metrics.ErrorRate*100)
	}

	if baseline.Thresholds != nil {
		fmt.Printf("\nThresholds:\n")
		fmt.Printf("  Max throughput regression: %.1f%%\n", baseline.Thresholds.MaxThroughputRegression*100)
		fmt.Printf("  Max latency regression:    %.1f%%\n", baseline.Thresholds.MaxLatencyRegression*100)
		fmt.Printf("  Max P99 regression:        %.1f%%\n", baseline.Thresholds.MaxP99Regression*100)
	}

	return nil
}

func runBaselineDelete(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	baselineID := args[0]

	force, _ := cmd.Flags().GetBool("force")

	// Get storage
	storageType, storagePath := getStorageConfig()
	store, err := newRunStorage(storageType, storagePath)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Shutdown(ctx)

	// Check if baseline exists
	var baseline domain.Baseline
	if err := store.Load(ctx, "baseline", baselineID, &baseline); err != nil {
		return fmt.Errorf("baseline not found: %s", baselineID)
	}

	// Confirm deletion
	if !force {
		fmt.Printf("Delete baseline '%s' (%s)? [y/N]: ", baseline.Name, baselineID)
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Delete baseline
	if err := store.Delete(ctx, "baseline", baselineID); err != nil {
		return fmt.Errorf("failed to delete baseline: %w", err)
	}

	fmt.Printf("Deleted baseline: %s\n", baselineID)
	return nil
}

func runBaselineSetActive(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	baselineID := args[0]

	// Get storage
	storageType, storagePath := getStorageConfig()
	store, err := newRunStorage(storageType, storagePath)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Shutdown(ctx)

	// Load the baseline
	var baseline domain.Baseline
	if err := store.Load(ctx, "baseline", baselineID, &baseline); err != nil {
		return fmt.Errorf("baseline not found: %s", baselineID)
	}

	// Deactivate all baselines
	if err := deactivateAllBaselines(ctx, store); err != nil {
		return fmt.Errorf("failed to deactivate baselines: %w", err)
	}

	// Activate this baseline
	baseline.Active = true
	baseline.UpdatedAt = time.Now()

	if _, err := store.Save(ctx, "baseline", &baseline); err != nil {
		return fmt.Errorf("failed to save baseline: %w", err)
	}

	fmt.Printf("Set active baseline: %s (%s)\n", baseline.Name, baselineID)
	return nil
}

// deactivateAllBaselines deactivates all baselines.
func deactivateAllBaselines(ctx context.Context, store interface {
	Query(ctx context.Context, entityType string, filter plugin.QueryFilter, dest interface{}) error
	Save(ctx context.Context, entityType string, entity interface{}) (string, error)
}) error {
	var baselines []*domain.Baseline
	filter := plugin.QueryFilter{
		Conditions: map[string]interface{}{"active": true},
		Limit:      100,
	}
	if err := store.Query(ctx, "baseline", filter, &baselines); err != nil {
		return err
	}

	for _, baseline := range baselines {
		if baseline.Active {
			baseline.Active = false
			baseline.UpdatedAt = time.Now()
			if _, err := store.Save(ctx, "baseline", baseline); err != nil {
				return err
			}
		}
	}

	return nil
}

// truncate truncates a string to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
