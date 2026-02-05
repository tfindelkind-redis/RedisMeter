package bundle

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// MockDataProvider for testing
type MockDataProvider struct {
	benchmarks    []json.RawMessage
	baselines     []json.RawMessage
	workloads     []json.RawMessage
	runProfiles   []json.RawMessage
	infraProfiles []json.RawMessage
	logs          []json.RawMessage
	reportFiles   map[string][]byte
}

func NewMockDataProvider() *MockDataProvider {
	return &MockDataProvider{
		reportFiles: make(map[string][]byte),
	}
}

func (m *MockDataProvider) GetBenchmarks(ctx context.Context, ids []string) ([]json.RawMessage, error) {
	if ids == nil {
		return m.benchmarks, nil
	}
	// Filter by IDs
	result := make([]json.RawMessage, 0)
	for _, b := range m.benchmarks {
		var item map[string]interface{}
		json.Unmarshal(b, &item)
		id, _ := item["id"].(string)
		for _, wantID := range ids {
			if id == wantID {
				result = append(result, b)
				break
			}
		}
	}
	return result, nil
}

func (m *MockDataProvider) GetAllBenchmarks(ctx context.Context, since, until *time.Time) ([]json.RawMessage, error) {
	return m.benchmarks, nil
}

func (m *MockDataProvider) GetBaselines(ctx context.Context, ids []string) ([]json.RawMessage, error) {
	return m.baselines, nil
}

func (m *MockDataProvider) GetAllBaselines(ctx context.Context) ([]json.RawMessage, error) {
	return m.baselines, nil
}

func (m *MockDataProvider) GetWorkloads(ctx context.Context, ids []string) ([]json.RawMessage, error) {
	return m.workloads, nil
}

func (m *MockDataProvider) GetAllWorkloads(ctx context.Context) ([]json.RawMessage, error) {
	return m.workloads, nil
}

func (m *MockDataProvider) GetRunProfiles(ctx context.Context, ids []string) ([]json.RawMessage, error) {
	return m.runProfiles, nil
}

func (m *MockDataProvider) GetAllRunProfiles(ctx context.Context) ([]json.RawMessage, error) {
	return m.runProfiles, nil
}

func (m *MockDataProvider) GetInfraProfiles(ctx context.Context) ([]json.RawMessage, error) {
	return m.infraProfiles, nil
}

func (m *MockDataProvider) GetLogs(ctx context.Context, since, until *time.Time, benchmarkIDs []string) ([]json.RawMessage, error) {
	return m.logs, nil
}

func (m *MockDataProvider) GetReportFiles(ctx context.Context, benchmarkIDs []string) (map[string][]byte, error) {
	return m.reportFiles, nil
}

// MockDataImporter for testing
type MockDataImporter struct {
	importedBenchmarks    []json.RawMessage
	importedBaselines     []json.RawMessage
	importedWorkloads     []json.RawMessage
	importedRunProfiles   []json.RawMessage
	importedInfraProfiles []json.RawMessage
	importedLogs          []json.RawMessage
}

func NewMockDataImporter() *MockDataImporter {
	return &MockDataImporter{}
}

func (m *MockDataImporter) ImportBenchmark(ctx context.Context, data json.RawMessage, overwrite bool) (bool, error) {
	m.importedBenchmarks = append(m.importedBenchmarks, data)
	return true, nil
}

func (m *MockDataImporter) ImportBaseline(ctx context.Context, data json.RawMessage, overwrite bool) (bool, error) {
	m.importedBaselines = append(m.importedBaselines, data)
	return true, nil
}

func (m *MockDataImporter) ImportWorkload(ctx context.Context, data json.RawMessage, overwrite bool) (bool, error) {
	m.importedWorkloads = append(m.importedWorkloads, data)
	return true, nil
}

func (m *MockDataImporter) ImportRunProfile(ctx context.Context, data json.RawMessage, overwrite bool) (bool, error) {
	m.importedRunProfiles = append(m.importedRunProfiles, data)
	return true, nil
}

func (m *MockDataImporter) ImportInfraProfile(ctx context.Context, data json.RawMessage, overwrite bool) (bool, error) {
	m.importedInfraProfiles = append(m.importedInfraProfiles, data)
	return true, nil
}

func (m *MockDataImporter) ImportLog(ctx context.Context, data json.RawMessage) error {
	m.importedLogs = append(m.importedLogs, data)
	return nil
}

func TestExportOptions(t *testing.T) {
	opts := DefaultExportOptions()
	if !opts.IncludeBenchmarks {
		t.Error("Default should include benchmarks")
	}
	if !opts.IncludeBaselines {
		t.Error("Default should include baselines")
	}
	if !opts.IncludeWorkloads {
		t.Error("Default should include workloads")
	}
	if !opts.IncludeLogs {
		t.Error("Default should include logs")
	}
}

func TestImportOptions(t *testing.T) {
	opts := DefaultImportOptions()
	if opts.OverwriteExisting {
		t.Error("Default should not overwrite")
	}
	if !opts.SkipExisting {
		t.Error("Default should skip existing")
	}
	if !opts.ImportBenchmarks {
		t.Error("Default should import benchmarks")
	}
}

func TestExporter(t *testing.T) {
	provider := NewMockDataProvider()
	provider.benchmarks = []json.RawMessage{
		json.RawMessage(`{"id": "bench-1", "name": "Test Benchmark"}`),
	}
	provider.baselines = []json.RawMessage{
		json.RawMessage(`{"id": "base-1", "name": "Test Baseline"}`),
	}

	exporter := NewExporter(provider, "1.0.0")
	ctx := context.Background()

	opts := ExportOptions{
		IncludeBenchmarks: true,
		IncludeBaselines:  true,
		Description:       "Test export",
	}

	bundle, err := exporter.Export(ctx, opts)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if bundle.Manifest.Version != "1.0" {
		t.Errorf("Expected version '1.0', got '%s'", bundle.Manifest.Version)
	}
	if bundle.Manifest.Description != "Test export" {
		t.Error("Description not set")
	}
	if len(bundle.Benchmarks) != 1 {
		t.Errorf("Expected 1 benchmark, got %d", len(bundle.Benchmarks))
	}
	if len(bundle.Baselines) != 1 {
		t.Errorf("Expected 1 baseline, got %d", len(bundle.Baselines))
	}
	if bundle.Manifest.Contents.BenchmarkCount != 1 {
		t.Error("Benchmark count not set in manifest")
	}
}

func TestExporterWithFilters(t *testing.T) {
	provider := NewMockDataProvider()
	provider.benchmarks = []json.RawMessage{
		json.RawMessage(`{"id": "bench-1", "name": "Benchmark 1"}`),
		json.RawMessage(`{"id": "bench-2", "name": "Benchmark 2"}`),
	}

	exporter := NewExporter(provider, "1.0.0")
	ctx := context.Background()

	opts := ExportOptions{
		IncludeBenchmarks: true,
		BenchmarkIDs:      []string{"bench-1"},
	}

	bundle, err := exporter.Export(ctx, opts)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	if len(bundle.Benchmarks) != 1 {
		t.Errorf("Expected 1 filtered benchmark, got %d", len(bundle.Benchmarks))
	}
}

func TestBundleWriteToZip(t *testing.T) {
	provider := NewMockDataProvider()
	provider.benchmarks = []json.RawMessage{
		json.RawMessage(`{"id": "bench-1", "name": "Test"}`),
	}

	exporter := NewExporter(provider, "1.0.0")
	ctx := context.Background()

	bundle, _ := exporter.Export(ctx, ExportOptions{IncludeBenchmarks: true})

	var buf bytes.Buffer
	if err := bundle.WriteToZip(&buf); err != nil {
		t.Fatalf("WriteToZip failed: %v", err)
	}

	// Verify it's a valid ZIP
	reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("Invalid ZIP: %v", err)
	}

	// Check for manifest.json
	foundManifest := false
	foundData := false
	for _, f := range reader.File {
		if f.Name == "manifest.json" {
			foundManifest = true
		}
		if f.Name == "data/benchmarks.json" {
			foundData = true
		}
	}

	if !foundManifest {
		t.Error("manifest.json not found in ZIP")
	}
	if !foundData {
		t.Error("data/benchmarks.json not found in ZIP")
	}
}

func TestBundleReadFromZip(t *testing.T) {
	// Create a bundle
	provider := NewMockDataProvider()
	provider.benchmarks = []json.RawMessage{
		json.RawMessage(`{"id": "bench-1", "name": "Test"}`),
	}
	provider.baselines = []json.RawMessage{
		json.RawMessage(`{"id": "base-1", "name": "Baseline"}`),
	}

	exporter := NewExporter(provider, "1.0.0")
	ctx := context.Background()

	original, _ := exporter.Export(ctx, ExportOptions{
		IncludeBenchmarks: true,
		IncludeBaselines:  true,
	})

	// Write to ZIP
	var buf bytes.Buffer
	original.WriteToZip(&buf)

	// Read back
	restored, err := LoadFromZip(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("LoadFromZip failed: %v", err)
	}

	if len(restored.Benchmarks) != 1 {
		t.Errorf("Expected 1 benchmark, got %d", len(restored.Benchmarks))
	}
	if len(restored.Baselines) != 1 {
		t.Errorf("Expected 1 baseline, got %d", len(restored.Baselines))
	}
	if restored.Manifest.ID != original.Manifest.ID {
		t.Error("Manifest ID doesn't match")
	}
}

func TestBundleWithFiles(t *testing.T) {
	provider := NewMockDataProvider()
	provider.reportFiles = map[string][]byte{
		"reports/test.html": []byte("<html><body>Test Report</body></html>"),
	}

	exporter := NewExporter(provider, "1.0.0")
	ctx := context.Background()

	bundle, _ := exporter.Export(ctx, ExportOptions{
		IncludeReports: true,
	})

	// Add the files manually for this test
	bundle.Files = provider.reportFiles
	bundle.Manifest.Contents.FileCount = len(bundle.Files)

	// Write to ZIP
	var buf bytes.Buffer
	bundle.WriteToZip(&buf)

	// Read and verify files
	reader, _ := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))

	foundFile := false
	for _, f := range reader.File {
		if f.Name == "files/reports/test.html" {
			foundFile = true
			rc, _ := f.Open()
			data, _ := io.ReadAll(rc)
			rc.Close()
			if string(data) != "<html><body>Test Report</body></html>" {
				t.Error("File content mismatch")
			}
		}
	}
	if !foundFile {
		t.Error("Report file not found in ZIP")
	}
}

func TestImporter(t *testing.T) {
	// Create a bundle
	provider := NewMockDataProvider()
	provider.benchmarks = []json.RawMessage{
		json.RawMessage(`{"id": "bench-1", "name": "Test"}`),
	}

	exporter := NewExporter(provider, "1.0.0")
	ctx := context.Background()

	bundle, _ := exporter.Export(ctx, ExportOptions{IncludeBenchmarks: true})

	// Write to ZIP
	var buf bytes.Buffer
	bundle.WriteToZip(&buf)

	// Import
	importer := NewMockDataImporter()
	imp := NewImporter(importer)

	// First load the bundle from ZIP
	loadedBundle, err := LoadFromZip(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("LoadFromZip failed: %v", err)
	}

	result, err := imp.Import(ctx, loadedBundle, DefaultImportOptions())
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Import not successful: %s", result.Error)
	}
	if result.BenchmarksImported != 1 {
		t.Errorf("Expected 1 benchmark imported, got %d", result.BenchmarksImported)
	}
	if len(importer.importedBenchmarks) != 1 {
		t.Error("Benchmark not actually imported")
	}
}

func TestImporterSelective(t *testing.T) {
	// Create a bundle with multiple types
	provider := NewMockDataProvider()
	provider.benchmarks = []json.RawMessage{
		json.RawMessage(`{"id": "bench-1"}`),
	}
	provider.baselines = []json.RawMessage{
		json.RawMessage(`{"id": "base-1"}`),
	}
	provider.workloads = []json.RawMessage{
		json.RawMessage(`{"id": "workload-1"}`),
	}

	exporter := NewExporter(provider, "1.0.0")
	ctx := context.Background()

	bundle, _ := exporter.Export(ctx, ExportOptions{
		IncludeBenchmarks: true,
		IncludeBaselines:  true,
		IncludeWorkloads:  true,
	})

	var buf bytes.Buffer
	bundle.WriteToZip(&buf)

	// Import only benchmarks
	importer := NewMockDataImporter()
	imp := NewImporter(importer)

	opts := ImportOptions{
		ImportBenchmarks: true,
		ImportBaselines:  false,
		ImportWorkloads:  false,
	}

	// First load the bundle from ZIP
	loadedBundle, _ := LoadFromZip(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	result, _ := imp.Import(ctx, loadedBundle, opts)

	if result.BenchmarksImported != 1 {
		t.Error("Benchmarks should be imported")
	}
	if len(importer.importedBaselines) != 0 {
		t.Error("Baselines should not be imported")
	}
	if len(importer.importedWorkloads) != 0 {
		t.Error("Workloads should not be imported")
	}
}

func TestBundleToFile(t *testing.T) {
	provider := NewMockDataProvider()
	provider.benchmarks = []json.RawMessage{
		json.RawMessage(`{"id": "bench-1"}`),
	}

	exporter := NewExporter(provider, "1.0.0")
	ctx := context.Background()

	bundle, _ := exporter.Export(ctx, ExportOptions{IncludeBenchmarks: true})

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test-bundle.zip")

	// Write to file
	f, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	bundle.WriteToZip(f)
	f.Close()

	// Read from file
	f, _ = os.Open(filePath)
	defer f.Close()
	stat, _ := f.Stat()

	restored, err := LoadFromZip(f, stat.Size())
	if err != nil {
		t.Fatalf("LoadFromZip failed: %v", err)
	}

	if len(restored.Benchmarks) != 1 {
		t.Error("Benchmark not restored from file")
	}
}

func TestManifestSerialization(t *testing.T) {
	manifest := Manifest{
		ID:          "test-id",
		Version:     "1.0",
		CreatedAt:   time.Now(),
		Description: "Test manifest",
		Contents: ContentsSummary{
			BenchmarkCount: 5,
			BaselineCount:  3,
		},
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var restored Manifest
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if restored.ID != "test-id" {
		t.Error("ID not preserved")
	}
	if restored.Contents.BenchmarkCount != 5 {
		t.Error("BenchmarkCount not preserved")
	}
}

func TestImportResultTracking(t *testing.T) {
	result := ImportResult{Success: true}

	// Simulate imports
	result.BenchmarksImported = 3
	result.BenchmarksSkipped = 1
	result.BaselinesImported = 2

	total := result.BenchmarksImported + result.BaselinesImported
	if total != 5 {
		t.Errorf("Expected 5 total imports, got %d", total)
	}
}

func TestExtractIDs(t *testing.T) {
	data := []json.RawMessage{
		json.RawMessage(`{"id": "id-1", "name": "One"}`),
		json.RawMessage(`{"id": "id-2", "name": "Two"}`),
		json.RawMessage(`{"name": "No ID"}`),
	}

	ids := extractIDs(data)

	if len(ids) != 2 {
		t.Errorf("Expected 2 IDs, got %d", len(ids))
	}
	if ids[0] != "id-1" || ids[1] != "id-2" {
		t.Error("IDs not correctly extracted")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
