package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tfindelkind-redis/redismeter/internal/importer"
)

// importCmd represents the import command.
var importCmd = &cobra.Command{
	Use:   "import [file]",
	Short: "Import benchmark data from external formats",
	Long: `Import benchmark runs from JSON, JSON Lines, or CSV files.

The import command supports multiple input formats and conflict resolution strategies.

Supported formats:
  json   - JSON array of benchmark runs
  jsonl  - JSON Lines (one JSON object per line)
  csv    - CSV with header row
  auto   - Auto-detect format (default)

Conflict resolution:
  skip    - Skip records with existing IDs (default)
  replace - Replace existing records with imported data
  rename  - Generate new IDs for conflicting records`,
	Example: `  # Import from JSON file
  redismeter import benchmarks.json

  # Import from CSV with replace strategy
  redismeter import --format csv --conflict replace data.csv

  # Dry run to validate without importing
  redismeter import --dry-run benchmarks.jsonl

  # Import from stdin
  cat data.json | redismeter import -

  # Import with rename strategy for conflicts
  redismeter import --conflict rename old-data.json`,
	Args: cobra.ExactArgs(1),
	RunE: runImport,
}

func init() {
	rootCmd.AddCommand(importCmd)

	importCmd.Flags().StringP("format", "f", "auto", "Input format: json, jsonl, csv, auto")
	importCmd.Flags().StringP("conflict", "c", "skip", "Conflict resolution: skip, replace, rename")
	importCmd.Flags().Bool("dry-run", false, "Validate without importing")
	importCmd.Flags().Bool("no-validate", false, "Skip validation")
}

func runImport(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Parse options
	formatStr, _ := cmd.Flags().GetString("format")
	conflictStr, _ := cmd.Flags().GetString("conflict")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	noValidate, _ := cmd.Flags().GetBool("no-validate")

	// Build import options
	options := importer.DefaultOptions()
	options.DryRun = dryRun
	options.Validate = !noValidate

	// Parse format
	switch formatStr {
	case "json":
		options.Format = importer.FormatJSON
	case "jsonl":
		options.Format = importer.FormatJSONL
	case "csv":
		options.Format = importer.FormatCSV
	case "auto":
		options.Format = importer.FormatAuto
	default:
		return fmt.Errorf("invalid format: %s", formatStr)
	}

	// Parse conflict resolution
	switch conflictStr {
	case "skip":
		options.ConflictResolution = importer.ConflictSkip
	case "replace":
		options.ConflictResolution = importer.ConflictReplace
	case "rename":
		options.ConflictResolution = importer.ConflictRename
	default:
		return fmt.Errorf("invalid conflict resolution: %s", conflictStr)
	}

	// Get storage
	storageType, storagePath := getStorageConfig()
	store, err := newRunStorage(storageType, storagePath)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Shutdown(ctx)

	// Open input file
	inputFile := args[0]
	var input *os.File

	if inputFile == "-" {
		input = os.Stdin
	} else {
		f, err := os.Open(inputFile)
		if err != nil {
			return fmt.Errorf("failed to open input file: %w", err)
		}
		defer f.Close()
		input = f
	}

	// Create importer
	imp := importer.NewImporter(options, store)

	// Run import
	result, err := imp.Import(ctx, input)
	if err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	// Print results
	if dryRun {
		fmt.Println("Dry run - no changes made")
	}
	fmt.Printf("Import complete:\n")
	fmt.Printf("  Imported: %d\n", result.Imported)
	fmt.Printf("  Skipped:  %d\n", result.Skipped)
	fmt.Printf("  Replaced: %d\n", result.Replaced)
	fmt.Printf("  Failed:   %d\n", result.Failed)

	if len(result.Errors) > 0 {
		fmt.Printf("\nErrors:\n")
		for _, err := range result.Errors {
			fmt.Printf("  - %s\n", err)
		}
	}

	return nil
}
