package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tfindelkind-redis/redismeter/internal/storage"
)

// migrateCmd represents the migrate command.
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate data between storage backends",
	Long: `Migrate benchmark data from one storage backend to another.

This command allows you to migrate all benchmark runs from one storage
backend to another, for example from file-based storage to SQLite.

The migration preserves all data including runs, baselines, and tags.`,
	Example: `  # Migrate from file storage to SQLite
  redismeter migrate --from file --from-path ~/.redismeter --to sqlite --to-path ./data.db

  # Migrate from SQLite to file storage
  redismeter migrate --from sqlite --from-path ./data.db --to file --to-path ~/.redismeter-new

  # Dry run to see what would be migrated
  redismeter migrate --from file --to sqlite --dry-run`,
	RunE: runMigrate,
}

func init() {
	rootCmd.AddCommand(migrateCmd)

	migrateCmd.Flags().String("from", "", "Source storage type (file, sqlite)")
	migrateCmd.Flags().String("from-path", "", "Source storage path")
	migrateCmd.Flags().String("to", "", "Destination storage type (file, sqlite)")
	migrateCmd.Flags().String("to-path", "", "Destination storage path")
	migrateCmd.Flags().Bool("dry-run", false, "Show what would be migrated without actually migrating")

	migrateCmd.MarkFlagRequired("from")
	migrateCmd.MarkFlagRequired("to")
}

func runMigrate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	fromType, _ := cmd.Flags().GetString("from")
	fromPath, _ := cmd.Flags().GetString("from-path")
	toType, _ := cmd.Flags().GetString("to")
	toPath, _ := cmd.Flags().GetString("to-path")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// Validate types
	if fromType != "file" && fromType != "sqlite" {
		return fmt.Errorf("invalid source type: %s (must be 'file' or 'sqlite')", fromType)
	}
	if toType != "file" && toType != "sqlite" {
		return fmt.Errorf("invalid destination type: %s (must be 'file' or 'sqlite')", toType)
	}

	// Use default paths if not specified
	if fromPath == "" {
		home, _ := os.UserHomeDir()
		fromPath = home + "/.redismeter"
	}
	if toPath == "" {
		home, _ := os.UserHomeDir()
		toPath = home + "/.redismeter-migrated"
	}

	// Open source storage
	srcConfig := storage.StorageConfig{
		Type: storage.StorageType(fromType),
		Path: fromPath,
	}
	srcStorage, err := storage.NewStorage(srcConfig)
	if err != nil {
		return fmt.Errorf("failed to open source storage: %w", err)
	}
	defer srcStorage.Shutdown(ctx)

	// Open destination storage
	dstConfig := storage.StorageConfig{
		Type: storage.StorageType(toType),
		Path: toPath,
	}
	dstStorage, err := storage.NewStorage(dstConfig)
	if err != nil {
		return fmt.Errorf("failed to open destination storage: %w", err)
	}
	defer dstStorage.Shutdown(ctx)

	fmt.Printf("Migrating from %s (%s) to %s (%s)\n", fromType, fromPath, toType, toPath)
	if dryRun {
		fmt.Println("(dry run - no changes will be made)")
	}
	fmt.Println()

	// Get all runs from source
	runs, err := srcStorage.ListRuns(ctx, 0) // 0 = no limit
	if err != nil {
		return fmt.Errorf("failed to list source runs: %w", err)
	}

	fmt.Printf("Found %d benchmark runs to migrate\n", len(runs))

	if len(runs) == 0 {
		fmt.Println("No data to migrate.")
		return nil
	}

	// Migrate runs
	migrated := 0
	failed := 0
	var errors []string

	for _, run := range runs {
		if dryRun {
			fmt.Printf("  Would migrate: %s (%s)\n", run.ID, run.Name)
			migrated++
			continue
		}

		// Get the full run data
		fullRun, err := srcStorage.GetRun(ctx, run.ID)
		if err != nil {
			failed++
			errors = append(errors, fmt.Sprintf("%s: %v", run.ID, err))
			continue
		}

		// Save to destination
		if err := dstStorage.SaveRun(ctx, fullRun); err != nil {
			failed++
			errors = append(errors, fmt.Sprintf("%s: %v", run.ID, err))
			continue
		}

		migrated++
		if verbose {
			fmt.Printf("  Migrated: %s\n", run.ID)
		}
	}

	fmt.Println()
	fmt.Printf("Migration complete:\n")
	fmt.Printf("  Migrated: %d\n", migrated)
	fmt.Printf("  Failed:   %d\n", failed)

	if len(errors) > 0 {
		fmt.Printf("\nErrors:\n")
		for _, err := range errors {
			fmt.Printf("  - %s\n", err)
		}
	}

	if !dryRun && migrated > 0 {
		fmt.Printf("\nTo switch to the new storage backend, run:\n")
		fmt.Printf("  redismeter config set storage.type %s\n", toType)
		fmt.Printf("  redismeter config set storage.path %s\n", toPath)
	}

	return nil
}
