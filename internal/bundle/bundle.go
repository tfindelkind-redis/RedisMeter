// Package bundle provides export/import functionality for sharing benchmark data.
package bundle

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// Bundle represents an exportable/importable data package.
type Bundle struct {
	// Metadata
	Manifest Manifest `json:"manifest"`

	// Data sections
	Benchmarks    []json.RawMessage `json:"benchmarks,omitempty"`
	Baselines     []json.RawMessage `json:"baselines,omitempty"`
	Workloads     []json.RawMessage `json:"workloads,omitempty"`
	RunProfiles   []json.RawMessage `json:"run_profiles,omitempty"`
	InfraProfiles []json.RawMessage `json:"infra_profiles,omitempty"`
	Logs          []json.RawMessage `json:"logs,omitempty"`

	// Raw files (HTML reports, etc.)
	Files map[string][]byte `json:"-"`
}

// Manifest contains metadata about the export bundle.
type Manifest struct {
	ID          string    `json:"id"`
	Version     string    `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	CreatedBy   string    `json:"created_by,omitempty"`
	Description string    `json:"description,omitempty"`

	// Source information
	SourceHost    string `json:"source_host,omitempty"`
	SourceVersion string `json:"source_version,omitempty"`

	// Contents summary
	Contents ContentsSummary `json:"contents"`

	// Compatibility
	MinVersion string `json:"min_version,omitempty"`
}

// ContentsSummary describes what's included in the bundle.
type ContentsSummary struct {
	BenchmarkCount    int      `json:"benchmark_count"`
	BaselineCount     int      `json:"baseline_count"`
	WorkloadCount     int      `json:"workload_count"`
	RunProfileCount   int      `json:"run_profile_count"`
	InfraProfileCount int      `json:"infra_profile_count"`
	LogCount          int      `json:"log_count"`
	FileCount         int      `json:"file_count"`
	TotalSizeBytes    int64    `json:"total_size_bytes"`
	BenchmarkIDs      []string `json:"benchmark_ids,omitempty"`
	BaselineIDs       []string `json:"baseline_ids,omitempty"`
}

// ExportOptions configures what to include in the export.
type ExportOptions struct {
	// What to include
	IncludeBenchmarks    bool `json:"include_benchmarks"`
	IncludeBaselines     bool `json:"include_baselines"`
	IncludeWorkloads     bool `json:"include_workloads"`
	IncludeRunProfiles   bool `json:"include_run_profiles"`
	IncludeInfraProfiles bool `json:"include_infra_profiles"`
	IncludeLogs          bool `json:"include_logs"`
	IncludeReports       bool `json:"include_reports"`

	// Filters
	BenchmarkIDs []string   `json:"benchmark_ids,omitempty"`
	BaselineIDs  []string   `json:"baseline_ids,omitempty"`
	WorkloadIDs  []string   `json:"workload_ids,omitempty"`
	Since        *time.Time `json:"since,omitempty"`
	Until        *time.Time `json:"until,omitempty"`

	// Metadata
	Description string `json:"description,omitempty"`
	CreatedBy   string `json:"created_by,omitempty"`
}

// DefaultExportOptions returns options that include everything.
func DefaultExportOptions() ExportOptions {
	return ExportOptions{
		IncludeBenchmarks:    true,
		IncludeBaselines:     true,
		IncludeWorkloads:     true,
		IncludeRunProfiles:   true,
		IncludeInfraProfiles: true,
		IncludeLogs:          true,
		IncludeReports:       true,
	}
}

// ImportOptions configures how to handle imports.
type ImportOptions struct {
	// Conflict resolution
	OverwriteExisting bool `json:"overwrite_existing"`
	SkipExisting      bool `json:"skip_existing"`

	// What to import
	ImportBenchmarks    bool `json:"import_benchmarks"`
	ImportBaselines     bool `json:"import_baselines"`
	ImportWorkloads     bool `json:"import_workloads"`
	ImportRunProfiles   bool `json:"import_run_profiles"`
	ImportInfraProfiles bool `json:"import_infra_profiles"`
	ImportLogs          bool `json:"import_logs"`
}

// DefaultImportOptions returns options that import everything without overwriting.
func DefaultImportOptions() ImportOptions {
	return ImportOptions{
		OverwriteExisting:   false,
		SkipExisting:        true,
		ImportBenchmarks:    true,
		ImportBaselines:     true,
		ImportWorkloads:     true,
		ImportRunProfiles:   true,
		ImportInfraProfiles: true,
		ImportLogs:          true,
	}
}

// ImportResult contains the results of an import operation.
type ImportResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`

	// Counts
	BenchmarksImported    int `json:"benchmarks_imported"`
	BenchmarksSkipped     int `json:"benchmarks_skipped"`
	BaselinesImported     int `json:"baselines_imported"`
	BaselinesSkipped      int `json:"baselines_skipped"`
	WorkloadsImported     int `json:"workloads_imported"`
	WorkloadsSkipped      int `json:"workloads_skipped"`
	RunProfilesImported   int `json:"run_profiles_imported"`
	RunProfilesSkipped    int `json:"run_profiles_skipped"`
	InfraProfilesImported int `json:"infra_profiles_imported"`
	InfraProfilesSkipped  int `json:"infra_profiles_skipped"`
	LogsImported          int `json:"logs_imported"`
	LogsSkipped           int `json:"logs_skipped"`

	// Warnings
	Warnings []string `json:"warnings,omitempty"`
}

// DataProvider is the interface for accessing data to export.
type DataProvider interface {
	// Benchmarks
	GetBenchmarks(ctx context.Context, ids []string) ([]json.RawMessage, error)
	GetAllBenchmarks(ctx context.Context, since, until *time.Time) ([]json.RawMessage, error)

	// Baselines
	GetBaselines(ctx context.Context, ids []string) ([]json.RawMessage, error)
	GetAllBaselines(ctx context.Context) ([]json.RawMessage, error)

	// Workloads
	GetWorkloads(ctx context.Context, ids []string) ([]json.RawMessage, error)
	GetAllWorkloads(ctx context.Context) ([]json.RawMessage, error)

	// Run Profiles
	GetRunProfiles(ctx context.Context, ids []string) ([]json.RawMessage, error)
	GetAllRunProfiles(ctx context.Context) ([]json.RawMessage, error)

	// Infrastructure Profiles
	GetInfraProfiles(ctx context.Context) ([]json.RawMessage, error)

	// Logs
	GetLogs(ctx context.Context, since, until *time.Time, benchmarkIDs []string) ([]json.RawMessage, error)

	// Reports
	GetReportFiles(ctx context.Context, benchmarkIDs []string) (map[string][]byte, error)
}

// DataImporter is the interface for importing data.
type DataImporter interface {
	ImportBenchmark(ctx context.Context, data json.RawMessage, overwrite bool) (bool, error)
	ImportBaseline(ctx context.Context, data json.RawMessage, overwrite bool) (bool, error)
	ImportWorkload(ctx context.Context, data json.RawMessage, overwrite bool) (bool, error)
	ImportRunProfile(ctx context.Context, data json.RawMessage, overwrite bool) (bool, error)
	ImportInfraProfile(ctx context.Context, data json.RawMessage, overwrite bool) (bool, error)
	ImportLog(ctx context.Context, data json.RawMessage) error
}

// Exporter handles bundle exports.
type Exporter struct {
	provider DataProvider
	version  string
	hostname string
}

// NewExporter creates a new exporter.
func NewExporter(provider DataProvider, version string) *Exporter {
	hostname, _ := os.Hostname()
	return &Exporter{
		provider: provider,
		version:  version,
		hostname: hostname,
	}
}

// Export creates a bundle from the data store.
func (e *Exporter) Export(ctx context.Context, opts ExportOptions) (*Bundle, error) {
	bundle := &Bundle{
		Manifest: Manifest{
			ID:            uuid.New().String(),
			Version:       "1.0",
			CreatedAt:     time.Now(),
			CreatedBy:     opts.CreatedBy,
			Description:   opts.Description,
			SourceHost:    e.hostname,
			SourceVersion: e.version,
			MinVersion:    "1.0.0",
		},
		Files: make(map[string][]byte),
	}

	var err error

	// Export benchmarks
	if opts.IncludeBenchmarks {
		if len(opts.BenchmarkIDs) > 0 {
			bundle.Benchmarks, err = e.provider.GetBenchmarks(ctx, opts.BenchmarkIDs)
		} else {
			bundle.Benchmarks, err = e.provider.GetAllBenchmarks(ctx, opts.Since, opts.Until)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to export benchmarks: %w", err)
		}
		bundle.Manifest.Contents.BenchmarkCount = len(bundle.Benchmarks)
		bundle.Manifest.Contents.BenchmarkIDs = extractIDs(bundle.Benchmarks)
	}

	// Export baselines
	if opts.IncludeBaselines {
		if len(opts.BaselineIDs) > 0 {
			bundle.Baselines, err = e.provider.GetBaselines(ctx, opts.BaselineIDs)
		} else {
			bundle.Baselines, err = e.provider.GetAllBaselines(ctx)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to export baselines: %w", err)
		}
		bundle.Manifest.Contents.BaselineCount = len(bundle.Baselines)
		bundle.Manifest.Contents.BaselineIDs = extractIDs(bundle.Baselines)
	}

	// Export workloads
	if opts.IncludeWorkloads {
		if len(opts.WorkloadIDs) > 0 {
			bundle.Workloads, err = e.provider.GetWorkloads(ctx, opts.WorkloadIDs)
		} else {
			bundle.Workloads, err = e.provider.GetAllWorkloads(ctx)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to export workloads: %w", err)
		}
		bundle.Manifest.Contents.WorkloadCount = len(bundle.Workloads)
	}

	// Export run profiles
	if opts.IncludeRunProfiles {
		bundle.RunProfiles, err = e.provider.GetAllRunProfiles(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to export run profiles: %w", err)
		}
		bundle.Manifest.Contents.RunProfileCount = len(bundle.RunProfiles)
	}

	// Export infrastructure profiles
	if opts.IncludeInfraProfiles {
		bundle.InfraProfiles, err = e.provider.GetInfraProfiles(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to export infra profiles: %w", err)
		}
		bundle.Manifest.Contents.InfraProfileCount = len(bundle.InfraProfiles)
	}

	// Export logs
	if opts.IncludeLogs {
		bundle.Logs, err = e.provider.GetLogs(ctx, opts.Since, opts.Until, opts.BenchmarkIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to export logs: %w", err)
		}
		bundle.Manifest.Contents.LogCount = len(bundle.Logs)
	}

	// Export report files
	if opts.IncludeReports {
		bundle.Files, err = e.provider.GetReportFiles(ctx, bundle.Manifest.Contents.BenchmarkIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to export reports: %w", err)
		}
		bundle.Manifest.Contents.FileCount = len(bundle.Files)
	}

	return bundle, nil
}

// WriteToZip writes the bundle to a ZIP file.
func (b *Bundle) WriteToZip(w io.Writer) error {
	zw := zip.NewWriter(w)
	defer zw.Close()

	// Write manifest
	if err := writeJSONToZip(zw, "manifest.json", b.Manifest); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	// Write data sections
	if len(b.Benchmarks) > 0 {
		if err := writeJSONToZip(zw, "data/benchmarks.json", b.Benchmarks); err != nil {
			return fmt.Errorf("failed to write benchmarks: %w", err)
		}
	}

	if len(b.Baselines) > 0 {
		if err := writeJSONToZip(zw, "data/baselines.json", b.Baselines); err != nil {
			return fmt.Errorf("failed to write baselines: %w", err)
		}
	}

	if len(b.Workloads) > 0 {
		if err := writeJSONToZip(zw, "data/workloads.json", b.Workloads); err != nil {
			return fmt.Errorf("failed to write workloads: %w", err)
		}
	}

	if len(b.RunProfiles) > 0 {
		if err := writeJSONToZip(zw, "data/run_profiles.json", b.RunProfiles); err != nil {
			return fmt.Errorf("failed to write run profiles: %w", err)
		}
	}

	if len(b.InfraProfiles) > 0 {
		if err := writeJSONToZip(zw, "data/infra_profiles.json", b.InfraProfiles); err != nil {
			return fmt.Errorf("failed to write infra profiles: %w", err)
		}
	}

	if len(b.Logs) > 0 {
		if err := writeJSONToZip(zw, "data/logs.json", b.Logs); err != nil {
			return fmt.Errorf("failed to write logs: %w", err)
		}
	}

	// Write files
	for name, content := range b.Files {
		fw, err := zw.Create(filepath.Join("files", name))
		if err != nil {
			return fmt.Errorf("failed to create file entry %s: %w", name, err)
		}
		if _, err := fw.Write(content); err != nil {
			return fmt.Errorf("failed to write file %s: %w", name, err)
		}
	}

	return nil
}

// SaveToFile saves the bundle to a ZIP file.
func (b *Bundle) SaveToFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	return b.WriteToZip(f)
}

// ToBytes returns the bundle as a ZIP byte slice.
func (b *Bundle) ToBytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := b.WriteToZip(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Importer handles bundle imports.
type Importer struct {
	importer DataImporter
}

// NewImporter creates a new importer.
func NewImporter(importer DataImporter) *Importer {
	return &Importer{importer: importer}
}

// Import imports a bundle into the data store.
func (i *Importer) Import(ctx context.Context, bundle *Bundle, opts ImportOptions) (*ImportResult, error) {
	result := &ImportResult{Success: true}

	// Import benchmarks
	if opts.ImportBenchmarks && len(bundle.Benchmarks) > 0 {
		for _, data := range bundle.Benchmarks {
			imported, err := i.importer.ImportBenchmark(ctx, data, opts.OverwriteExisting)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("benchmark import warning: %v", err))
				continue
			}
			if imported {
				result.BenchmarksImported++
			} else {
				result.BenchmarksSkipped++
			}
		}
	}

	// Import baselines
	if opts.ImportBaselines && len(bundle.Baselines) > 0 {
		for _, data := range bundle.Baselines {
			imported, err := i.importer.ImportBaseline(ctx, data, opts.OverwriteExisting)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("baseline import warning: %v", err))
				continue
			}
			if imported {
				result.BaselinesImported++
			} else {
				result.BaselinesSkipped++
			}
		}
	}

	// Import workloads
	if opts.ImportWorkloads && len(bundle.Workloads) > 0 {
		for _, data := range bundle.Workloads {
			imported, err := i.importer.ImportWorkload(ctx, data, opts.OverwriteExisting)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("workload import warning: %v", err))
				continue
			}
			if imported {
				result.WorkloadsImported++
			} else {
				result.WorkloadsSkipped++
			}
		}
	}

	// Import run profiles
	if opts.ImportRunProfiles && len(bundle.RunProfiles) > 0 {
		for _, data := range bundle.RunProfiles {
			imported, err := i.importer.ImportRunProfile(ctx, data, opts.OverwriteExisting)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("run profile import warning: %v", err))
				continue
			}
			if imported {
				result.RunProfilesImported++
			} else {
				result.RunProfilesSkipped++
			}
		}
	}

	// Import infrastructure profiles
	if opts.ImportInfraProfiles && len(bundle.InfraProfiles) > 0 {
		for _, data := range bundle.InfraProfiles {
			imported, err := i.importer.ImportInfraProfile(ctx, data, opts.OverwriteExisting)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("infra profile import warning: %v", err))
				continue
			}
			if imported {
				result.InfraProfilesImported++
			} else {
				result.InfraProfilesSkipped++
			}
		}
	}

	// Import logs
	if opts.ImportLogs && len(bundle.Logs) > 0 {
		for _, data := range bundle.Logs {
			if err := i.importer.ImportLog(ctx, data); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("log import warning: %v", err))
				result.LogsSkipped++
				continue
			}
			result.LogsImported++
		}
	}

	return result, nil
}

// LoadFromFile loads a bundle from a ZIP file.
func LoadFromFile(path string) (*Bundle, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	return LoadFromZip(f, stat.Size())
}

// LoadFromBytes loads a bundle from a ZIP byte slice.
func LoadFromBytes(data []byte) (*Bundle, error) {
	r := bytes.NewReader(data)
	return LoadFromZip(r, int64(len(data)))
}

// LoadFromZip loads a bundle from a ZIP reader.
func LoadFromZip(r io.ReaderAt, size int64) (*Bundle, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip: %w", err)
	}

	bundle := &Bundle{
		Files: make(map[string][]byte),
	}

	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open %s: %w", f.Name, err)
		}

		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", f.Name, err)
		}

		switch f.Name {
		case "manifest.json":
			if err := json.Unmarshal(content, &bundle.Manifest); err != nil {
				return nil, fmt.Errorf("failed to parse manifest: %w", err)
			}
		case "data/benchmarks.json":
			if err := json.Unmarshal(content, &bundle.Benchmarks); err != nil {
				return nil, fmt.Errorf("failed to parse benchmarks: %w", err)
			}
		case "data/baselines.json":
			if err := json.Unmarshal(content, &bundle.Baselines); err != nil {
				return nil, fmt.Errorf("failed to parse baselines: %w", err)
			}
		case "data/workloads.json":
			if err := json.Unmarshal(content, &bundle.Workloads); err != nil {
				return nil, fmt.Errorf("failed to parse workloads: %w", err)
			}
		case "data/run_profiles.json":
			if err := json.Unmarshal(content, &bundle.RunProfiles); err != nil {
				return nil, fmt.Errorf("failed to parse run profiles: %w", err)
			}
		case "data/infra_profiles.json":
			if err := json.Unmarshal(content, &bundle.InfraProfiles); err != nil {
				return nil, fmt.Errorf("failed to parse infra profiles: %w", err)
			}
		case "data/logs.json":
			if err := json.Unmarshal(content, &bundle.Logs); err != nil {
				return nil, fmt.Errorf("failed to parse logs: %w", err)
			}
		default:
			// Store as file
			if len(f.Name) > 6 && f.Name[:6] == "files/" {
				bundle.Files[f.Name[6:]] = content
			}
		}
	}

	return bundle, nil
}

func writeJSONToZip(zw *zip.Writer, name string, data interface{}) error {
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	fw, err := zw.Create(name)
	if err != nil {
		return err
	}

	_, err = fw.Write(content)
	return err
}

func extractIDs(items []json.RawMessage) []string {
	var ids []string
	for _, item := range items {
		var obj struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(item, &obj); err == nil && obj.ID != "" {
			ids = append(ids, obj.ID)
		}
	}
	return ids
}
