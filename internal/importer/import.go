// Package importer provides data import functionality for RedisMeter.
package importer

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
	"github.com/tfindelkind-redis/redismeter/internal/storage"
)

// Format represents an import format.
type Format string

const (
	// FormatJSON imports from JSON (array of objects).
	FormatJSON Format = "json"
	// FormatJSONL imports from JSON Lines (one object per line).
	FormatJSONL Format = "jsonl"
	// FormatCSV imports from CSV.
	FormatCSV Format = "csv"
	// FormatAuto auto-detects the format.
	FormatAuto Format = "auto"
)

// ConflictResolution defines how to handle ID conflicts.
type ConflictResolution string

const (
	// ConflictSkip skips conflicting records.
	ConflictSkip ConflictResolution = "skip"
	// ConflictReplace replaces existing records.
	ConflictReplace ConflictResolution = "replace"
	// ConflictRename generates new IDs for conflicts.
	ConflictRename ConflictResolution = "rename"
)

// Options configures the import operation.
type Options struct {
	// Format specifies the input format.
	Format Format

	// ConflictResolution determines how to handle ID conflicts.
	ConflictResolution ConflictResolution

	// DryRun validates without actually importing.
	DryRun bool

	// Validate performs data validation.
	Validate bool
}

// DefaultOptions returns the default import options.
func DefaultOptions() Options {
	return Options{
		Format:             FormatAuto,
		ConflictResolution: ConflictSkip,
		DryRun:             false,
		Validate:           true,
	}
}

// ImportResult contains the results of an import operation.
type ImportResult struct {
	Imported int      // Number of successfully imported records
	Skipped  int      // Number of skipped records (conflicts)
	Replaced int      // Number of replaced records
	Failed   int      // Number of failed records
	Errors   []string // Error messages for failed records
}

// Importer imports benchmark data from various formats.
type Importer struct {
	options Options
	storage storage.RunStorage
}

// NewImporter creates a new importer with the given options and storage.
func NewImporter(options Options, store storage.RunStorage) *Importer {
	return &Importer{
		options: options,
		storage: store,
	}
}

// Import reads data from the reader and imports it to storage.
func (i *Importer) Import(ctx context.Context, r io.Reader) (*ImportResult, error) {
	// Auto-detect format if needed
	format := i.options.Format
	if format == FormatAuto {
		// Read the first part of the data to detect format
		var buf strings.Builder
		tee := io.TeeReader(r, &buf)
		
		// Read first 512 bytes to detect format
		sample := make([]byte, 512)
		n, _ := tee.Read(sample)
		sample = sample[:n]
		
		format = detectFormat(string(sample))
		
		// Create a new reader that includes the buffered data
		r = io.MultiReader(strings.NewReader(buf.String()), r)
	}

	switch format {
	case FormatJSON:
		return i.importJSON(ctx, r)
	case FormatJSONL:
		return i.importJSONL(ctx, r)
	case FormatCSV:
		return i.importCSV(ctx, r)
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// detectFormat attempts to detect the format from the content.
func detectFormat(sample string) Format {
	trimmed := strings.TrimSpace(sample)
	
	// Check for JSON array
	if strings.HasPrefix(trimmed, "[") {
		return FormatJSON
	}
	
	// Check for JSON object (JSON Lines)
	if strings.HasPrefix(trimmed, "{") {
		return FormatJSONL
	}
	
	// Assume CSV for anything else
	return FormatCSV
}

// importJSON imports from a JSON array.
func (i *Importer) importJSON(ctx context.Context, r io.Reader) (*ImportResult, error) {
	var runs []*domain.BenchmarkRun
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&runs); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return i.importRuns(ctx, runs)
}

// importJSONL imports from JSON Lines format.
func (i *Importer) importJSONL(ctx context.Context, r io.Reader) (*ImportResult, error) {
	var runs []*domain.BenchmarkRun
	scanner := bufio.NewScanner(r)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var run domain.BenchmarkRun
		if err := json.Unmarshal([]byte(line), &run); err != nil {
			return nil, fmt.Errorf("line %d: failed to parse JSON: %w", lineNum, err)
		}
		runs = append(runs, &run)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading input: %w", err)
	}

	return i.importRuns(ctx, runs)
}

// importCSV imports from CSV format.
func (i *Importer) importCSV(ctx context.Context, r io.Reader) (*ImportResult, error) {
	reader := csv.NewReader(r)
	
	// Read header
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Build column index map
	colMap := make(map[string]int)
	for idx, col := range header {
		colMap[strings.ToLower(strings.TrimSpace(col))] = idx
	}

	var runs []*domain.BenchmarkRun
	lineNum := 1

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("line %d: failed to read CSV: %w", lineNum+1, err)
		}
		lineNum++

		run, err := i.csvRecordToRun(colMap, record)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}
		runs = append(runs, run)
	}

	return i.importRuns(ctx, runs)
}

// csvRecordToRun converts a CSV record to a BenchmarkRun.
func (i *Importer) csvRecordToRun(colMap map[string]int, record []string) (*domain.BenchmarkRun, error) {
	getCol := func(name string) string {
		if idx, ok := colMap[name]; ok && idx < len(record) {
			return record[idx]
		}
		return ""
	}

	run := &domain.BenchmarkRun{
		ID:     getCol("id"),
		Status: domain.RunStatus(getCol("status")),
		Name:   getCol("name"),
	}

	// Parse created_at
	if createdAt := getCol("created_at"); createdAt != "" {
		t, err := time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("invalid created_at: %w", err)
		}
		run.CreatedAt = t
	}

	// Parse workload
	if workload := getCol("workload"); workload != "" {
		run.Workload = &domain.Workload{Name: workload}
	}

	// Parse target
	if host := getCol("target_host"); host != "" {
		run.Target = &domain.Target{Host: host}
		if port := getCol("target_port"); port != "" {
			p, _ := strconv.Atoi(port)
			run.Target.Port = p
		}
	}

	// Parse results
	if opsPerSec := getCol("ops_per_second"); opsPerSec != "" {
		ops, _ := strconv.ParseFloat(opsPerSec, 64)
		run.Results = &domain.Results{
			Summary: &domain.SummaryMetrics{
				OpsPerSecond: ops,
			},
		}

		if avgLat := getCol("avg_latency_ms"); avgLat != "" {
			run.Results.Summary.AvgLatencyMs, _ = strconv.ParseFloat(avgLat, 64)
		}
		if p50 := getCol("p50_latency_ms"); p50 != "" {
			run.Results.Summary.P50LatencyMs, _ = strconv.ParseFloat(p50, 64)
		}
		if p99 := getCol("p99_latency_ms"); p99 != "" {
			run.Results.Summary.P99LatencyMs, _ = strconv.ParseFloat(p99, 64)
		}
		if p999 := getCol("p999_latency_ms"); p999 != "" {
			run.Results.Summary.P999LatencyMs, _ = strconv.ParseFloat(p999, 64)
		}
		if errors := getCol("errors"); errors != "" {
			e, _ := strconv.ParseInt(errors, 10, 64)
			run.Results.Summary.Errors = e
		}
		if errorRate := getCol("error_rate"); errorRate != "" {
			run.Results.Summary.ErrorRate, _ = strconv.ParseFloat(errorRate, 64)
		}
	}

	// Parse duration
	run.Duration = getCol("duration")

	// Parse tags
	if tags := getCol("tags"); tags != "" {
		// Try to parse as JSON array
		var tagList []string
		if err := json.Unmarshal([]byte(tags), &tagList); err == nil {
			run.Tags = tagList
		} else {
			// Try comma-separated
			for _, tag := range strings.Split(tags, ",") {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					run.Tags = append(run.Tags, tag)
				}
			}
		}
	}

	return run, nil
}

// importRuns imports the parsed runs to storage.
func (i *Importer) importRuns(ctx context.Context, runs []*domain.BenchmarkRun) (*ImportResult, error) {
	result := &ImportResult{}

	for _, run := range runs {
		// Validate if enabled
		if i.options.Validate {
			if err := validateRun(run); err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", run.ID, err))
				continue
			}
		}

		// Check for existing record
		existing, err := i.storage.GetRun(ctx, run.ID)
		if err == nil && existing != nil {
			// Handle conflict
			switch i.options.ConflictResolution {
			case ConflictSkip:
				result.Skipped++
				continue
			case ConflictReplace:
				if !i.options.DryRun {
					if err := i.storage.SaveRun(ctx, run); err != nil {
						result.Failed++
						result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", run.ID, err))
						continue
					}
				}
				result.Replaced++
			case ConflictRename:
				// Generate new ID
				run.ID = fmt.Sprintf("%s-import-%d", run.ID[:8], time.Now().UnixNano())
				if !i.options.DryRun {
					if err := i.storage.SaveRun(ctx, run); err != nil {
						result.Failed++
						result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", run.ID, err))
						continue
					}
				}
				result.Imported++
			}
		} else {
			// No conflict - import normally
			if !i.options.DryRun {
				if err := i.storage.SaveRun(ctx, run); err != nil {
					result.Failed++
					result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", run.ID, err))
					continue
				}
			}
			result.Imported++
		}
	}

	return result, nil
}

// validateRun performs basic validation on a benchmark run.
func validateRun(run *domain.BenchmarkRun) error {
	if run.ID == "" {
		return fmt.Errorf("missing required field: id")
	}
	if run.Status == "" {
		return fmt.Errorf("missing required field: status")
	}
	if run.CreatedAt.IsZero() {
		return fmt.Errorf("missing required field: created_at")
	}
	return nil
}
