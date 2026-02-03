// Package cli implements the command-line interface for RedisMeter.
package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/tfindelkind-redis/redismeter/internal/engine"
	"github.com/tfindelkind-redis/redismeter/internal/storage"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List benchmark runs",
	Long: `List all benchmark runs stored locally.

Examples:
  # List recent runs
  redismeter list

  # List more runs
  redismeter list -n 50`,
	RunE: listRuns,
}

var (
	limitFlag int
)

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().IntVarP(&limitFlag, "limit", "n", 20, "Maximum number of results")
}

func listRuns(cmd *cobra.Command, args []string) error {
	storageType, storagePath := getStorageConfig()
	engCfg := engine.EngineConfig{
		StorageConfig: storage.StorageConfig{
			Type: storage.StorageType(storageType),
			Path: storagePath,
		},
	}

	eng, err := engine.NewEngineWithConfig(engCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize engine: %w", err)
	}

	runs, err := eng.ListRuns(context.Background(), limitFlag)
	if err != nil {
		return fmt.Errorf("failed to list runs: %w", err)
	}

	if len(runs) == 0 {
		fmt.Println()
		fmt.Println("📋 No benchmark runs found.")
		fmt.Println()
		fmt.Println("   Run your first benchmark with:")
		fmt.Println("   redismeter run cache --target redis://localhost:6379")
		fmt.Println()
		return nil
	}

	fmt.Println()
	fmt.Println("📋 Benchmark Runs")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tWORKLOAD\tTARGET\tOPS/SEC\tP99 (ms)\tDURATION\tCREATED")
	fmt.Fprintln(w, "──\t──────\t────────\t──────\t───────\t────────\t────────\t───────")

	for _, run := range runs {
		status := statusIcon(string(run.Status))
		workload := "-"
		if run.Workload != nil {
			workload = run.Workload.Name
		}
		target := "-"
		if run.Target != nil {
			target = fmt.Sprintf("%s:%d", run.Target.Host, run.Target.Port)
		}
		opsPerSec := "-"
		p99 := "-"
		if run.Results != nil && run.Results.Summary != nil {
			opsPerSec = fmt.Sprintf("%.0f", run.Results.Summary.OpsPerSecond)
			p99 = fmt.Sprintf("%.2f", run.Results.Summary.P99LatencyMs)
		}
		duration := run.Duration
		if duration == "" {
			duration = "-"
		}
		created := run.CreatedAt.Format("2006-01-02 15:04")

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			run.ID, status, workload, target, opsPerSec, p99, duration, created)
	}

	w.Flush()
	fmt.Println()

	return nil
}

func statusIcon(status string) string {
	switch status {
	case "completed":
		return "✅"
	case "running":
		return "🔄"
	case "failed":
		return "❌"
	case "cancelled":
		return "⚠️"
	default:
		return "⏳"
	}
}
