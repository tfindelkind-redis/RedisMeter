package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tfindelkind-redis/redismeter/internal/bundle"
	"github.com/tfindelkind-redis/redismeter/internal/domain"
	"github.com/tfindelkind-redis/redismeter/internal/infraprofile"
	"github.com/tfindelkind-redis/redismeter/internal/logging"
	"github.com/tfindelkind-redis/redismeter/internal/storage"
)

var bundleCmd = &cobra.Command{
	Use:   "bundle",
	Short: "Export and import RedisMeter data bundles",
	Long: `Create and restore complete data bundles for RedisMeter.

Bundles include:
- Benchmark results and their raw outputs
- Baselines
- Infrastructure profiles
- Logs (optional)

This is useful for:
- Backing up all RedisMeter data
- Migrating between systems
- Sharing benchmark data with team members
- Archiving project results`,
}

var bundleExportCmd = &cobra.Command{
	Use:   "export [output-file]",
	Short: "Export data to a bundle file",
	Long: `Export RedisMeter data to a bundle file (.zip).

Examples:
  # Export all data
  redismeter bundle export backup.zip

  # Export specific benchmark
  redismeter bundle export --benchmark abc123 result.zip

  # Export without logs
  redismeter bundle export --no-logs backup.zip`,
	RunE: runBundleExport,
}

var bundleImportCmd = &cobra.Command{
	Use:   "import [input-file]",
	Short: "Import data from a bundle file",
	Long: `Import RedisMeter data from a bundle file.

Examples:
  # Import with skip existing (default)
  redismeter bundle import backup.zip

  # Import and overwrite existing data
  redismeter bundle import --overwrite backup.zip

  # Dry run to see what would be imported
  redismeter bundle import --dry-run backup.zip`,
	Args: cobra.ExactArgs(1),
	RunE: runBundleImport,
}

var bundleInfoCmd = &cobra.Command{
	Use:   "info [bundle-file]",
	Short: "Show information about a bundle file",
	Args:  cobra.ExactArgs(1),
	RunE:  runBundleInfo,
}

// Flags
var (
	bundleBenchmarkIDs []string
	bundleBaselineIDs  []string
	bundleNoLogs       bool
	bundleNoProfiles   bool
	bundleOverwrite    bool
	bundleDryRun       bool
	bundleForce        bool
	bundleDescription  string
)

func init() {
	rootCmd.AddCommand(bundleCmd)
	bundleCmd.AddCommand(bundleExportCmd)
	bundleCmd.AddCommand(bundleImportCmd)
	bundleCmd.AddCommand(bundleInfoCmd)

	// Export flags
	bundleExportCmd.Flags().StringSliceVarP(&bundleBenchmarkIDs, "benchmark", "b", nil, "Export specific benchmark(s) only")
	bundleExportCmd.Flags().StringSliceVar(&bundleBaselineIDs, "baseline", nil, "Export specific baseline(s) only")
	bundleExportCmd.Flags().BoolVar(&bundleNoLogs, "no-logs", false, "Exclude logs from export")
	bundleExportCmd.Flags().BoolVar(&bundleNoProfiles, "no-profiles", false, "Exclude infrastructure profiles")
	bundleExportCmd.Flags().StringVarP(&bundleDescription, "description", "d", "", "Bundle description")

	// Import flags
	bundleImportCmd.Flags().BoolVar(&bundleOverwrite, "overwrite", false, "Overwrite existing data")
	bundleImportCmd.Flags().BoolVar(&bundleDryRun, "dry-run", false, "Show what would be imported without making changes")
	bundleImportCmd.Flags().BoolVarP(&bundleForce, "force", "f", false, "Skip confirmation")
}

// cliDataProvider implements bundle.DataProvider using CLI stores
type cliDataProvider struct {
	store        *storage.FileStorage
	logStore     *logging.JSONStore
	profileStore *infraprofile.FileStore
}

func newCLIDataProvider() (*cliDataProvider, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	basePath := filepath.Join(homeDir, ".redismeter")

	// Create main storage
	store, err := storage.NewFileStorage(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	// Create log store
	logStore, err := logging.NewJSONStore(filepath.Join(basePath, "logs"))
	if err != nil {
		return nil, fmt.Errorf("failed to create log store: %w", err)
	}

	// Create profile store
	profileStore, err := infraprofile.NewFileStore(filepath.Join(basePath, "infra-profiles"))
	if err != nil {
		logStore.Close()
		return nil, fmt.Errorf("failed to create profile store: %w", err)
	}

	return &cliDataProvider{
		store:        store,
		logStore:     logStore,
		profileStore: profileStore,
	}, nil
}

func (p *cliDataProvider) Close() error {
	var errs []error
	if p.logStore != nil {
		if err := p.logStore.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if p.profileStore != nil {
		if err := p.profileStore.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// Implement bundle.DataProvider interface
func (p *cliDataProvider) GetBenchmarks(ctx context.Context, ids []string) ([]json.RawMessage, error) {
	var result []json.RawMessage
	for _, id := range ids {
		run, err := p.store.GetRun(ctx, id)
		if err != nil {
			continue
		}
		data, err := json.Marshal(run)
		if err != nil {
			continue
		}
		result = append(result, data)
	}
	return result, nil
}

func (p *cliDataProvider) GetAllBenchmarks(ctx context.Context, since, until *time.Time) ([]json.RawMessage, error) {
	runs, err := p.store.ListRuns(ctx, 1000)
	if err != nil {
		return nil, err
	}

	var result []json.RawMessage
	for _, run := range runs {
		// Apply time filter
		if since != nil && run.CreatedAt.Before(*since) {
			continue
		}
		if until != nil && run.CreatedAt.After(*until) {
			continue
		}
		data, err := json.Marshal(run)
		if err != nil {
			continue
		}
		result = append(result, data)
	}
	return result, nil
}

func (p *cliDataProvider) GetBaselines(ctx context.Context, ids []string) ([]json.RawMessage, error) {
	var result []json.RawMessage
	for _, id := range ids {
		var baseline domain.Baseline
		err := p.store.Load(ctx, "baselines", id, &baseline)
		if err != nil {
			continue
		}
		data, err := json.Marshal(baseline)
		if err != nil {
			continue
		}
		result = append(result, data)
	}
	return result, nil
}

func (p *cliDataProvider) GetAllBaselines(ctx context.Context) ([]json.RawMessage, error) {
	var baselines []domain.Baseline
	// Read baseline files directly from the baselines directory
	baselineDir := filepath.Join(os.Getenv("HOME"), ".redismeter", "baselines")
	entries, err := os.ReadDir(baselineDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(baselineDir, entry.Name()))
		if err != nil {
			continue
		}
		var b domain.Baseline
		if err := json.Unmarshal(data, &b); err != nil {
			continue
		}
		baselines = append(baselines, b)
	}

	var result []json.RawMessage
	for _, b := range baselines {
		data, err := json.Marshal(b)
		if err != nil {
			continue
		}
		result = append(result, data)
	}
	return result, nil
}

func (p *cliDataProvider) GetWorkloads(ctx context.Context, ids []string) ([]json.RawMessage, error) {
	return nil, nil // Workloads are typically not stored
}

func (p *cliDataProvider) GetAllWorkloads(ctx context.Context) ([]json.RawMessage, error) {
	return nil, nil
}

func (p *cliDataProvider) GetRunProfiles(ctx context.Context, ids []string) ([]json.RawMessage, error) {
	return nil, nil
}

func (p *cliDataProvider) GetAllRunProfiles(ctx context.Context) ([]json.RawMessage, error) {
	return nil, nil
}

func (p *cliDataProvider) GetInfraProfiles(ctx context.Context) ([]json.RawMessage, error) {
	if p.profileStore == nil {
		return nil, nil
	}

	profiles, err := p.profileStore.List(ctx)
	if err != nil {
		return nil, err
	}

	var result []json.RawMessage
	for _, prof := range profiles {
		data, err := json.Marshal(prof)
		if err != nil {
			continue
		}
		result = append(result, data)
	}
	return result, nil
}

func (p *cliDataProvider) GetLogs(ctx context.Context, since, until *time.Time, benchmarkIDs []string) ([]json.RawMessage, error) {
	if p.logStore == nil {
		return nil, nil
	}

	filter := &logging.QueryFilter{
		Since: since,
		Until: until,
	}
	if len(benchmarkIDs) > 0 {
		filter.BenchmarkID = benchmarkIDs[0]
	}

	entries, err := p.logStore.Query(ctx, filter)
	if err != nil {
		return nil, err
	}

	var result []json.RawMessage
	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		result = append(result, data)
	}
	return result, nil
}

func (p *cliDataProvider) GetReportFiles(ctx context.Context, benchmarkIDs []string) (map[string][]byte, error) {
	return nil, nil // Reports are generated on demand
}

// cliDataImporter implements bundle.DataImporter
type cliDataImporter struct {
	store        *storage.FileStorage
	logStore     *logging.JSONStore
	profileStore *infraprofile.FileStore
}

func (i *cliDataImporter) ImportBenchmark(ctx context.Context, data json.RawMessage, overwrite bool) (bool, error) {
	var run domain.BenchmarkRun
	if err := json.Unmarshal(data, &run); err != nil {
		return false, err
	}

	// Check if exists
	existing, err := i.store.GetRun(ctx, run.ID)
	if err == nil && existing != nil && !overwrite {
		return false, nil // Skip existing
	}

	if err := i.store.SaveRun(ctx, &run); err != nil {
		return false, err
	}
	return true, nil
}

func (i *cliDataImporter) ImportBaseline(ctx context.Context, data json.RawMessage, overwrite bool) (bool, error) {
	var baseline domain.Baseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		return false, err
	}

	// Check if exists
	var existing domain.Baseline
	err := i.store.Load(ctx, "baselines", baseline.ID, &existing)
	if err == nil && !overwrite {
		return false, nil // Skip existing
	}

	_, err = i.store.Save(ctx, "baselines", &baseline)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (i *cliDataImporter) ImportWorkload(ctx context.Context, data json.RawMessage, overwrite bool) (bool, error) {
	return false, nil // Not implemented
}

func (i *cliDataImporter) ImportRunProfile(ctx context.Context, data json.RawMessage, overwrite bool) (bool, error) {
	return false, nil // Not implemented
}

func (i *cliDataImporter) ImportInfraProfile(ctx context.Context, data json.RawMessage, overwrite bool) (bool, error) {
	if i.profileStore == nil {
		return false, nil
	}

	var profile infraprofile.Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return false, err
	}

	// Check if exists
	existing, _ := i.profileStore.Get(ctx, profile.ID)
	if existing != nil && !overwrite {
		return false, nil // Skip existing
	}

	if existing != nil && overwrite {
		if err := i.profileStore.Update(ctx, &profile); err != nil {
			return false, err
		}
	} else {
		if err := i.profileStore.Create(ctx, &profile); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (i *cliDataImporter) ImportLog(ctx context.Context, data json.RawMessage) error {
	if i.logStore == nil {
		return nil
	}

	var entry logging.Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		return err
	}

	return i.logStore.Save(ctx, &entry)
}

func runBundleExport(cmd *cobra.Command, args []string) error {
	// Determine output file
	var outputFile string
	if len(args) > 0 {
		outputFile = args[0]
	} else {
		outputFile = fmt.Sprintf("redismeter-bundle-%s.zip", time.Now().Format("2006-01-02-150405"))
	}

	provider, err := newCLIDataProvider()
	if err != nil {
		return err
	}
	defer provider.Close()

	exporter := bundle.NewExporter(provider, "1.0.0")

	opts := bundle.ExportOptions{
		IncludeBenchmarks:    true,
		IncludeBaselines:     true,
		IncludeWorkloads:     true,
		IncludeRunProfiles:   true,
		IncludeInfraProfiles: !bundleNoProfiles,
		IncludeLogs:          !bundleNoLogs,
		IncludeReports:       true,
		BenchmarkIDs:         bundleBenchmarkIDs,
		BaselineIDs:          bundleBaselineIDs,
		Description:          bundleDescription,
	}

	ctx := context.Background()
	b, err := exporter.Export(ctx, opts)
	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	if err := b.SaveToFile(outputFile); err != nil {
		return fmt.Errorf("failed to save bundle: %w", err)
	}

	fmt.Printf("Bundle exported to: %s\n", outputFile)
	fmt.Println("\nContents:")
	fmt.Printf("  Benchmarks: %d\n", b.Manifest.Contents.BenchmarkCount)
	fmt.Printf("  Baselines:  %d\n", b.Manifest.Contents.BaselineCount)
	fmt.Printf("  Profiles:   %d\n", b.Manifest.Contents.InfraProfileCount)
	fmt.Printf("  Logs:       %d\n", b.Manifest.Contents.LogCount)

	return nil
}

func runBundleImport(cmd *cobra.Command, args []string) error {
	inputFile := args[0]

	// First, load the bundle to show what will be imported
	b, err := bundle.LoadFromFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read bundle: %w", err)
	}

	fmt.Printf("Bundle: %s\n", inputFile)
	fmt.Printf("Version: %s\n", b.Manifest.Version)
	fmt.Printf("Created: %s\n", b.Manifest.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Source:  %s\n", b.Manifest.SourceHost)
	fmt.Println("\nContents:")
	fmt.Printf("  Benchmarks: %d\n", b.Manifest.Contents.BenchmarkCount)
	fmt.Printf("  Baselines:  %d\n", b.Manifest.Contents.BaselineCount)
	fmt.Printf("  Profiles:   %d\n", b.Manifest.Contents.InfraProfileCount)
	fmt.Printf("  Logs:       %d\n", b.Manifest.Contents.LogCount)

	if bundleDryRun {
		fmt.Println("\n[Dry run - no changes made]")
		return nil
	}

	if !bundleForce {
		fmt.Print("\nImport this bundle? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	provider, err := newCLIDataProvider()
	if err != nil {
		return err
	}
	defer provider.Close()

	dataImporter := &cliDataImporter{
		store:        provider.store,
		logStore:     provider.logStore,
		profileStore: provider.profileStore,
	}

	importer := bundle.NewImporter(dataImporter)

	opts := bundle.ImportOptions{
		OverwriteExisting:   bundleOverwrite,
		SkipExisting:        !bundleOverwrite,
		ImportBenchmarks:    true,
		ImportBaselines:     true,
		ImportWorkloads:     true,
		ImportRunProfiles:   true,
		ImportInfraProfiles: true,
		ImportLogs:          true,
	}

	ctx := context.Background()
	result, err := importer.Import(ctx, b, opts)
	if err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	fmt.Println("\nImport complete:")
	fmt.Printf("  Benchmarks: %d imported, %d skipped\n", result.BenchmarksImported, result.BenchmarksSkipped)
	fmt.Printf("  Baselines:  %d imported, %d skipped\n", result.BaselinesImported, result.BaselinesSkipped)
	fmt.Printf("  Profiles:   %d imported, %d skipped\n", result.InfraProfilesImported, result.InfraProfilesSkipped)
	fmt.Printf("  Logs:       %d imported, %d skipped\n", result.LogsImported, result.LogsSkipped)

	if len(result.Warnings) > 0 {
		fmt.Println("\nWarnings:")
		for _, w := range result.Warnings {
			fmt.Printf("  - %s\n", w)
		}
	}

	return nil
}

func runBundleInfo(cmd *cobra.Command, args []string) error {
	inputFile := args[0]

	b, err := bundle.LoadFromFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read bundle: %w", err)
	}

	fmt.Printf("Bundle Information\n")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("File:        %s\n", inputFile)
	fmt.Printf("ID:          %s\n", b.Manifest.ID)
	fmt.Printf("Version:     %s\n", b.Manifest.Version)
	fmt.Printf("Created:     %s\n", b.Manifest.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Source Host: %s\n", b.Manifest.SourceHost)

	if b.Manifest.Description != "" {
		fmt.Printf("Description: %s\n", b.Manifest.Description)
	}

	fmt.Println("\nContents:")
	fmt.Printf("  Benchmarks:    %d\n", b.Manifest.Contents.BenchmarkCount)
	fmt.Printf("  Baselines:     %d\n", b.Manifest.Contents.BaselineCount)
	fmt.Printf("  Profiles:      %d\n", b.Manifest.Contents.InfraProfileCount)
	fmt.Printf("  Log Entries:   %d\n", b.Manifest.Contents.LogCount)
	fmt.Printf("  Files:         %d\n", b.Manifest.Contents.FileCount)

	if len(b.Manifest.Contents.BenchmarkIDs) > 0 {
		fmt.Println("\nBenchmark IDs:")
		for _, id := range b.Manifest.Contents.BenchmarkIDs {
			fmt.Printf("  - %s\n", id)
		}
	}

	if len(b.Manifest.Contents.BaselineIDs) > 0 {
		fmt.Println("\nBaseline IDs:")
		for _, id := range b.Manifest.Contents.BaselineIDs {
			fmt.Printf("  - %s\n", id)
		}
	}

	return nil
}
