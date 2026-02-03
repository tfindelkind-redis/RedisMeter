// Package export provides data export functionality for RedisMeter.
package export

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
)

func createTestRun() *domain.BenchmarkRun {
	return &domain.BenchmarkRun{
		ID:        "test-run-123",
		CreatedAt: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		UpdatedAt: time.Date(2024, 1, 15, 10, 35, 0, 0, time.UTC),
		Status:    domain.RunStatusCompleted,
		Name:      "Test Run",
		Tags:      []string{"test", "ci"},
		Workload: &domain.Workload{
			Name:        "cache-test",
			Type:        "builtin",
			Description: "Cache test workload",
		},
		Target: &domain.Target{
			Host: "localhost",
			Port: 6379,
			URL:  "redis://localhost:6379",
		},
		Results: &domain.Results{
			Summary: &domain.SummaryMetrics{
				TotalOps:      100000,
				OpsPerSecond:  50000.5,
				AvgLatencyMs:  1.2,
				MinLatencyMs:  0.5,
				MaxLatencyMs:  15.3,
				P50LatencyMs:  1.0,
				P90LatencyMs:  2.5,
				P95LatencyMs:  3.2,
				P99LatencyMs:  5.8,
				P999LatencyMs: 12.1,
				Errors:        5,
				ErrorRate:     0.00005,
			},
		},
		Environment: &domain.Environment{
			Fingerprint:       "abc123",
			Hostname:          "test-host",
			OS:                "linux",
			Arch:              "amd64",
			RedisMeterVersion: "0.1.0",
			MemtierVersion:    "2.0.0",
			RedisVersion:      "7.0.0",
		},
		Duration:  "5m30s",
		StartTime: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		EndTime:   time.Date(2024, 1, 15, 10, 35, 30, 0, time.UTC),
	}
}

func TestExporter_JSON(t *testing.T) {
	run := createTestRun()
	exporter := NewExporter(Options{Format: FormatJSON, IncludeEnvironment: true})

	var buf bytes.Buffer
	err := exporter.ExportRuns(context.Background(), []*domain.BenchmarkRun{run}, &buf)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Verify it's valid JSON
	var result []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Invalid JSON output: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("Expected 1 run, got %d", len(result))
	}

	// Check key fields
	if result[0]["id"] != "test-run-123" {
		t.Errorf("Expected id 'test-run-123', got %v", result[0]["id"])
	}
	if result[0]["status"] != "completed" {
		t.Errorf("Expected status 'completed', got %v", result[0]["status"])
	}

	// Check nested results
	results, ok := result[0]["results"].(map[string]interface{})
	if !ok {
		t.Fatal("Results not found in output")
	}
	if results["ops_per_second"].(float64) != 50000.5 {
		t.Errorf("Expected ops_per_second 50000.5, got %v", results["ops_per_second"])
	}

	// Check environment included
	env, ok := result[0]["environment"].(map[string]interface{})
	if !ok {
		t.Fatal("Environment not found in output")
	}
	if env["fingerprint"] != "abc123" {
		t.Errorf("Expected fingerprint 'abc123', got %v", env["fingerprint"])
	}
}

func TestExporter_JSONCompact(t *testing.T) {
	run := createTestRun()
	exporter := NewExporter(Options{Format: FormatJSONCompact})

	var buf bytes.Buffer
	err := exporter.ExportRuns(context.Background(), []*domain.BenchmarkRun{run}, &buf)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Verify it's valid JSON without newlines in the middle (compact)
	output := buf.String()
	if strings.Count(output, "\n") != 1 { // Should only have trailing newline
		t.Error("Expected compact JSON (single line)")
	}
}

func TestExporter_JSONL(t *testing.T) {
	runs := []*domain.BenchmarkRun{createTestRun(), createTestRun()}
	runs[1].ID = "test-run-456"

	exporter := NewExporter(Options{Format: FormatJSONL})

	var buf bytes.Buffer
	err := exporter.ExportRuns(context.Background(), runs, &buf)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Verify each line is valid JSON
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("Expected 2 lines, got %d", len(lines))
	}

	for i, line := range lines {
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Errorf("Line %d is not valid JSON: %v", i+1, err)
		}
	}
}

func TestExporter_CSV(t *testing.T) {
	run := createTestRun()
	exporter := NewExporter(Options{Format: FormatCSV})

	var buf bytes.Buffer
	err := exporter.ExportRuns(context.Background(), []*domain.BenchmarkRun{run}, &buf)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Parse CSV
	reader := csv.NewReader(&buf)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("Invalid CSV output: %v", err)
	}

	// Should have header + 1 data row
	if len(records) != 2 {
		t.Fatalf("Expected 2 rows (header + data), got %d", len(records))
	}

	// Check header
	header := records[0]
	if header[0] != "id" {
		t.Errorf("Expected first header 'id', got %s", header[0])
	}

	// Check data row
	data := records[1]
	if data[0] != "test-run-123" {
		t.Errorf("Expected id 'test-run-123', got %s", data[0])
	}
	if data[2] != "completed" {
		t.Errorf("Expected status 'completed', got %s", data[2])
	}
	if data[3] != "cache-test" {
		t.Errorf("Expected workload 'cache-test', got %s", data[3])
	}
}

func TestExporter_NoEnvironment(t *testing.T) {
	run := createTestRun()
	exporter := NewExporter(Options{Format: FormatJSON, IncludeEnvironment: false})

	var buf bytes.Buffer
	err := exporter.ExportRuns(context.Background(), []*domain.BenchmarkRun{run}, &buf)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	var result []map[string]interface{}
	json.Unmarshal(buf.Bytes(), &result)

	if _, ok := result[0]["environment"]; ok {
		t.Error("Environment should not be included")
	}
}

func TestExporter_IncludeRawOutput(t *testing.T) {
	run := createTestRun()
	run.Results.RawOutput = `{"test": "raw output"}`

	exporter := NewExporter(Options{
		Format:           FormatJSON,
		IncludeRawOutput: true,
	})

	var buf bytes.Buffer
	err := exporter.ExportRuns(context.Background(), []*domain.BenchmarkRun{run}, &buf)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	var result []map[string]interface{}
	json.Unmarshal(buf.Bytes(), &result)

	results := result[0]["results"].(map[string]interface{})
	if results["raw_output"] != `{"test": "raw output"}` {
		t.Error("Raw output should be included")
	}
}

func TestExporter_EmptyRuns(t *testing.T) {
	exporter := NewExporter(Options{Format: FormatJSON})

	var buf bytes.Buffer
	err := exporter.ExportRuns(context.Background(), []*domain.BenchmarkRun{}, &buf)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Should produce empty array
	var result []map[string]interface{}
	json.Unmarshal(buf.Bytes(), &result)

	if len(result) != 0 {
		t.Errorf("Expected empty array, got %d items", len(result))
	}
}

func TestExporter_UnsupportedFormat(t *testing.T) {
	exporter := NewExporter(Options{Format: "invalid"})

	var buf bytes.Buffer
	err := exporter.ExportRuns(context.Background(), []*domain.BenchmarkRun{createTestRun()}, &buf)
	if err == nil {
		t.Error("Expected error for unsupported format")
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.Format != FormatJSON {
		t.Errorf("Expected default format JSON, got %s", opts.Format)
	}
	if !opts.IncludeEnvironment {
		t.Error("Expected IncludeEnvironment to be true by default")
	}
	if opts.IncludeRawOutput {
		t.Error("Expected IncludeRawOutput to be false by default")
	}
}
