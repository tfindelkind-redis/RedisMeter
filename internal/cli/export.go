// Package cli implements the command-line interface for RedisMeter.
package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tfindelkind-redis/redismeter/internal/domain"
	"github.com/tfindelkind-redis/redismeter/internal/engine"
	"github.com/tfindelkind-redis/redismeter/internal/export"
	"github.com/tfindelkind-redis/redismeter/internal/plugin"
	"github.com/tfindelkind-redis/redismeter/internal/storage"
)

var exportCmd = &cobra.Command{
	Use:   "export [run-id...]",
	Short: "Export benchmark data",
	Long: `Export benchmark data to various formats.

Supported formats:
  json         Pretty-printed JSON (default)
  json-compact Compact JSON (no whitespace)
  jsonl        JSON Lines (one object per line)
  csv          Comma-separated values

Examples:
  # Export all runs to JSON
  redismeter export > runs.json

  # Export specific runs
  redismeter export run-123 run-456 -o export.json

  # Export to CSV
  redismeter export --format csv > runs.csv

  # Export last 10 runs as JSON Lines
  redismeter export -n 10 --format jsonl > runs.jsonl

  # Export only completed runs
  redismeter export --status completed`,
	RunE: runExport,
}

var (
	exportFormatFlag       string
	exportOutputFlag       string
	exportLimitFlag        int
	exportStatusFlag       string
	exportWorkloadFlag     string
	exportTagFlag          []string
	exportIncludeRawFlag   bool
	exportNoEnvironmentFlag bool
)

func init() {
	rootCmd.AddCommand(exportCmd)

	exportCmd.Flags().StringVarP(&exportFormatFlag, "format", "f", "json", "Export format (json, json-compact, jsonl, csv)")
	exportCmd.Flags().StringVarP(&exportOutputFlag, "output", "o", "", "Output file (default: stdout)")
	exportCmd.Flags().IntVarP(&exportLimitFlag, "limit", "n", 0, "Limit number of runs to export (0 = all)")
	exportCmd.Flags().StringVar(&exportStatusFlag, "status", "", "Filter by status (completed, failed, running)")
	exportCmd.Flags().StringVar(&exportWorkloadFlag, "workload", "", "Filter by workload name")
	exportCmd.Flags().StringSliceVar(&exportTagFlag, "tag", nil, "Filter by tags")
	exportCmd.Flags().BoolVar(&exportIncludeRawFlag, "include-raw", false, "Include raw memtier output")
	exportCmd.Flags().BoolVar(&exportNoEnvironmentFlag, "no-environment", false, "Exclude environment details")
}

func runExport(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get storage configuration
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

	// Collect runs to export
	var runs []*domain.BenchmarkRun

	if len(args) > 0 {
		// Export specific runs by ID
		for _, id := range args {
			run, err := eng.GetRun(ctx, id)
			if err != nil {
				return fmt.Errorf("run not found: %s", id)
			}
			runs = append(runs, run)
		}
	} else {
		// Query runs with filters
		filter := plugin.QueryFilter{}

		if exportLimitFlag > 0 {
			filter.Limit = exportLimitFlag
		}

		if exportStatusFlag != "" || exportWorkloadFlag != "" {
			filter.Conditions = make(map[string]interface{})
			if exportStatusFlag != "" {
				filter.Conditions["status"] = exportStatusFlag
			}
			if exportWorkloadFlag != "" {
				filter.Conditions["workload"] = exportWorkloadFlag
			}
		}

		if len(exportTagFlag) > 0 {
			filter.Tags = exportTagFlag
		}

		runs, err = eng.QueryRuns(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to query runs: %w", err)
		}
	}

	if len(runs) == 0 {
		fmt.Fprintln(os.Stderr, "No runs found matching the criteria")
		return nil
	}

	// Parse format
	format := export.Format(strings.ToLower(exportFormatFlag))

	// Create exporter
	opts := export.Options{
		Format:             format,
		IncludeEnvironment: !exportNoEnvironmentFlag,
		IncludeRawOutput:   exportIncludeRawFlag,
	}
	exporter := export.NewExporter(opts)

	// Determine output destination
	var output *os.File
	if exportOutputFlag != "" {
		f, err := os.Create(exportOutputFlag)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer f.Close()
		output = f
	} else {
		output = os.Stdout
	}

	// Export
	if err := exporter.ExportRuns(ctx, runs, output); err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	if exportOutputFlag != "" {
		fmt.Fprintf(os.Stderr, "✅ Exported %d run(s) to %s\n", len(runs), exportOutputFlag)
	}

	return nil
}

// Unused but needed to import domain package
var _ = domain.BenchmarkRun{}
