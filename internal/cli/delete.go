// Package cli implements the command-line interface for RedisMeter.
package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tfindelkind-redis/redismeter/internal/engine"
	"github.com/tfindelkind-redis/redismeter/internal/storage"
)

var deleteCmd = &cobra.Command{
	Use:   "delete <run-id>",
	Short: "Delete a benchmark run",
	Long: `Delete a benchmark run from storage.

Examples:
  redismeter delete 20240101-120000-abc123`,
	Args: cobra.ExactArgs(1),
	RunE: deleteRun,
}

var forceDeleteFlag bool

func init() {
	rootCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().BoolVarP(&forceDeleteFlag, "force", "f", false, "Skip confirmation")
}

func deleteRun(cmd *cobra.Command, args []string) error {
	runID := args[0]

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

	// Verify run exists
	run, err := eng.GetRun(context.Background(), runID)
	if err != nil {
		return fmt.Errorf("run not found: %w", err)
	}

	// Confirm deletion
	if !forceDeleteFlag {
		fmt.Printf("Delete benchmark run %s", runID)
		if run.Workload != nil {
			fmt.Printf(" (workload: %s)", run.Workload.Name)
		}
		fmt.Println("?")
		fmt.Print("Type 'yes' to confirm: ")

		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	if err := eng.DeleteRun(context.Background(), runID); err != nil {
		return fmt.Errorf("failed to delete run: %w", err)
	}

	fmt.Printf("✅ Deleted run %s\n", runID)
	return nil
}
