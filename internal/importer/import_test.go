package importer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
	"github.com/tfindelkind-redis/redismeter/internal/plugin"
)

// mockRunStorage implements storage.RunStorage for testing.
type mockRunStorage struct {
	runs map[string]*domain.BenchmarkRun
}

func newMockRunStorage() *mockRunStorage {
	return &mockRunStorage{
		runs: make(map[string]*domain.BenchmarkRun),
	}
}

func (m *mockRunStorage) Metadata() plugin.Metadata {
	return plugin.Metadata{Name: "mock", Version: "1.0.0"}
}

func (m *mockRunStorage) Initialize(ctx context.Context, config map[string]interface{}) error {
	return nil
}

func (m *mockRunStorage) HealthCheck(ctx context.Context) plugin.HealthStatus {
	return plugin.HealthStatus{Healthy: true}
}

func (m *mockRunStorage) Shutdown(ctx context.Context) error {
	return nil
}

func (m *mockRunStorage) Save(ctx context.Context, entityType string, entity interface{}) (string, error) {
	if entityType == "run" {
		run, ok := entity.(*domain.BenchmarkRun)
		if ok {
			m.runs[run.ID] = run
			return run.ID, nil
		}
	}
	return "", nil
}

func (m *mockRunStorage) Load(ctx context.Context, entityType string, id string, dest interface{}) error {
	if entityType == "run" {
		run, exists := m.runs[id]
		if !exists {
			return fmt.Errorf("not found: %s", id)
		}
		if runPtr, ok := dest.(*domain.BenchmarkRun); ok {
			*runPtr = *run
		}
	}
	return nil
}

func (m *mockRunStorage) Query(ctx context.Context, entityType string, filter plugin.QueryFilter, dest interface{}) error {
	if entityType == "run" {
		if runsPtr, ok := dest.(*[]*domain.BenchmarkRun); ok {
			for _, run := range m.runs {
				*runsPtr = append(*runsPtr, run)
			}
		}
	}
	return nil
}

func (m *mockRunStorage) Delete(ctx context.Context, entityType string, id string) error {
	delete(m.runs, id)
	return nil
}

func (m *mockRunStorage) SaveRun(ctx context.Context, run *domain.BenchmarkRun) error {
	m.runs[run.ID] = run
	return nil
}

func (m *mockRunStorage) ListRuns(ctx context.Context, limit int) ([]*domain.BenchmarkRun, error) {
	var runs []*domain.BenchmarkRun
	for _, run := range m.runs {
		runs = append(runs, run)
		if limit > 0 && len(runs) >= limit {
			break
		}
	}
	return runs, nil
}

func (m *mockRunStorage) GetRun(ctx context.Context, id string) (*domain.BenchmarkRun, error) {
	run, exists := m.runs[id]
	if !exists {
		return nil, fmt.Errorf("not found: %s", id)
	}
	return run, nil
}

func (m *mockRunStorage) DeleteRun(ctx context.Context, id string) error {
	delete(m.runs, id)
	return nil
}

func TestImporter_ImportJSON(t *testing.T) {
	store := newMockRunStorage()
	options := DefaultOptions()
	options.Format = FormatJSON

	imp := NewImporter(options, store)

	// Create test data
	runs := []*domain.BenchmarkRun{
		{
			ID:        "test-run-1",
			Status:    domain.RunStatusCompleted,
			CreatedAt: time.Now(),
			Name:      "Test Run 1",
		},
		{
			ID:        "test-run-2",
			Status:    domain.RunStatusCompleted,
			CreatedAt: time.Now(),
			Name:      "Test Run 2",
		},
	}

	data, _ := json.Marshal(runs)
	result, err := imp.Import(context.Background(), bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.Imported != 2 {
		t.Errorf("Expected 2 imported, got %d", result.Imported)
	}

	// Verify runs were saved
	savedRuns, _ := store.ListRuns(context.Background(), 0)
	if len(savedRuns) != 2 {
		t.Errorf("Expected 2 saved runs, got %d", len(savedRuns))
	}
}

func TestImporter_ImportJSONL(t *testing.T) {
	store := newMockRunStorage()
	options := DefaultOptions()
	options.Format = FormatJSONL

	imp := NewImporter(options, store)

	// Create test data (one JSON object per line)
	run1, _ := json.Marshal(&domain.BenchmarkRun{
		ID:        "jsonl-run-1",
		Status:    domain.RunStatusCompleted,
		CreatedAt: time.Now(),
	})
	run2, _ := json.Marshal(&domain.BenchmarkRun{
		ID:        "jsonl-run-2",
		Status:    domain.RunStatusCompleted,
		CreatedAt: time.Now(),
	})

	data := string(run1) + "\n" + string(run2) + "\n"

	result, err := imp.Import(context.Background(), strings.NewReader(data))
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.Imported != 2 {
		t.Errorf("Expected 2 imported, got %d", result.Imported)
	}
}

func TestImporter_ImportCSV(t *testing.T) {
	store := newMockRunStorage()
	options := DefaultOptions()
	options.Format = FormatCSV

	imp := NewImporter(options, store)

	// Create test CSV data
	csv := `id,status,created_at,name
csv-run-1,completed,2024-01-15T10:30:00Z,Test Run 1
csv-run-2,completed,2024-01-15T11:00:00Z,Test Run 2`

	result, err := imp.Import(context.Background(), strings.NewReader(csv))
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.Imported != 2 {
		t.Errorf("Expected 2 imported, got %d", result.Imported)
	}

	// Verify runs were saved
	run1, err := store.GetRun(context.Background(), "csv-run-1")
	if err != nil {
		t.Errorf("Failed to get run: %v", err)
	}
	if run1.Name != "Test Run 1" {
		t.Errorf("Expected name 'Test Run 1', got '%s'", run1.Name)
	}
}

func TestImporter_ConflictSkip(t *testing.T) {
	store := newMockRunStorage()

	// Pre-populate with existing run
	existingRun := &domain.BenchmarkRun{
		ID:        "existing-run",
		Status:    domain.RunStatusCompleted,
		CreatedAt: time.Now(),
		Name:      "Original Name",
	}
	store.SaveRun(context.Background(), existingRun)

	options := DefaultOptions()
	options.Format = FormatJSON
	options.ConflictResolution = ConflictSkip

	imp := NewImporter(options, store)

	// Try to import run with same ID
	runs := []*domain.BenchmarkRun{
		{
			ID:        "existing-run",
			Status:    domain.RunStatusCompleted,
			CreatedAt: time.Now(),
			Name:      "New Name",
		},
	}

	data, _ := json.Marshal(runs)
	result, err := imp.Import(context.Background(), bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.Skipped != 1 {
		t.Errorf("Expected 1 skipped, got %d", result.Skipped)
	}

	// Verify original run is unchanged
	run, _ := store.GetRun(context.Background(), "existing-run")
	if run.Name != "Original Name" {
		t.Errorf("Expected original name 'Original Name', got '%s'", run.Name)
	}
}

func TestImporter_ConflictReplace(t *testing.T) {
	store := newMockRunStorage()

	// Pre-populate with existing run
	existingRun := &domain.BenchmarkRun{
		ID:        "existing-run",
		Status:    domain.RunStatusCompleted,
		CreatedAt: time.Now(),
		Name:      "Original Name",
	}
	store.SaveRun(context.Background(), existingRun)

	options := DefaultOptions()
	options.Format = FormatJSON
	options.ConflictResolution = ConflictReplace

	imp := NewImporter(options, store)

	// Try to import run with same ID
	runs := []*domain.BenchmarkRun{
		{
			ID:        "existing-run",
			Status:    domain.RunStatusCompleted,
			CreatedAt: time.Now(),
			Name:      "Replaced Name",
		},
	}

	data, _ := json.Marshal(runs)
	result, err := imp.Import(context.Background(), bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.Replaced != 1 {
		t.Errorf("Expected 1 replaced, got %d", result.Replaced)
	}

	// Verify run was replaced
	run, _ := store.GetRun(context.Background(), "existing-run")
	if run.Name != "Replaced Name" {
		t.Errorf("Expected replaced name 'Replaced Name', got '%s'", run.Name)
	}
}

func TestImporter_ConflictRename(t *testing.T) {
	store := newMockRunStorage()

	// Pre-populate with existing run
	existingRun := &domain.BenchmarkRun{
		ID:        "existing-run-12345678",
		Status:    domain.RunStatusCompleted,
		CreatedAt: time.Now(),
		Name:      "Original Run",
	}
	store.SaveRun(context.Background(), existingRun)

	options := DefaultOptions()
	options.Format = FormatJSON
	options.ConflictResolution = ConflictRename

	imp := NewImporter(options, store)

	// Try to import run with same ID
	runs := []*domain.BenchmarkRun{
		{
			ID:        "existing-run-12345678",
			Status:    domain.RunStatusCompleted,
			CreatedAt: time.Now(),
			Name:      "New Run",
		},
	}

	data, _ := json.Marshal(runs)
	result, err := imp.Import(context.Background(), bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.Imported != 1 {
		t.Errorf("Expected 1 imported, got %d", result.Imported)
	}

	// Verify both runs exist
	savedRuns, _ := store.ListRuns(context.Background(), 0)
	if len(savedRuns) != 2 {
		t.Errorf("Expected 2 runs, got %d", len(savedRuns))
	}
}

func TestImporter_DryRun(t *testing.T) {
	store := newMockRunStorage()

	options := DefaultOptions()
	options.Format = FormatJSON
	options.DryRun = true

	imp := NewImporter(options, store)

	runs := []*domain.BenchmarkRun{
		{
			ID:        "dry-run-1",
			Status:    domain.RunStatusCompleted,
			CreatedAt: time.Now(),
		},
	}

	data, _ := json.Marshal(runs)
	result, err := imp.Import(context.Background(), bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.Imported != 1 {
		t.Errorf("Expected 1 imported, got %d", result.Imported)
	}

	// Verify no runs were actually saved
	savedRuns, _ := store.ListRuns(context.Background(), 0)
	if len(savedRuns) != 0 {
		t.Errorf("Expected 0 saved runs in dry run, got %d", len(savedRuns))
	}
}

func TestImporter_ValidationFailure(t *testing.T) {
	store := newMockRunStorage()

	options := DefaultOptions()
	options.Format = FormatJSON
	options.Validate = true

	imp := NewImporter(options, store)

	// Create invalid run (missing ID)
	runs := []*domain.BenchmarkRun{
		{
			ID:        "",
			Status:    domain.RunStatusCompleted,
			CreatedAt: time.Now(),
		},
	}

	data, _ := json.Marshal(runs)
	result, err := imp.Import(context.Background(), bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.Failed != 1 {
		t.Errorf("Expected 1 failed, got %d", result.Failed)
	}

	if len(result.Errors) == 0 {
		t.Error("Expected error messages")
	}
}

func TestImporter_AutoDetectJSON(t *testing.T) {
	store := newMockRunStorage()

	options := DefaultOptions()
	options.Format = FormatAuto

	imp := NewImporter(options, store)

	runs := []*domain.BenchmarkRun{
		{
			ID:        "auto-json-1",
			Status:    domain.RunStatusCompleted,
			CreatedAt: time.Now(),
		},
	}

	data, _ := json.Marshal(runs)
	result, err := imp.Import(context.Background(), bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.Imported != 1 {
		t.Errorf("Expected 1 imported, got %d", result.Imported)
	}
}

func TestImporter_AutoDetectJSONL(t *testing.T) {
	store := newMockRunStorage()

	options := DefaultOptions()
	options.Format = FormatAuto

	imp := NewImporter(options, store)

	run, _ := json.Marshal(&domain.BenchmarkRun{
		ID:        "auto-jsonl-1",
		Status:    domain.RunStatusCompleted,
		CreatedAt: time.Now(),
	})

	result, err := imp.Import(context.Background(), strings.NewReader(string(run)+"\n"))
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.Imported != 1 {
		t.Errorf("Expected 1 imported, got %d", result.Imported)
	}
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		sample   string
		expected Format
	}{
		{"[{\"id\":\"1\"}]", FormatJSON},
		{"{\"id\":\"1\"}", FormatJSONL},
		{"  [{\"id\":\"1\"}]", FormatJSON},
		{"  {\"id\":\"1\"}", FormatJSONL},
		{"id,status\n1,completed", FormatCSV},
	}

	for _, test := range tests {
		result := detectFormat(test.sample)
		if result != test.expected {
			t.Errorf("detectFormat(%q) = %s, expected %s", test.sample, result, test.expected)
		}
	}
}

func TestValidateRun(t *testing.T) {
	tests := []struct {
		name    string
		run     *domain.BenchmarkRun
		wantErr bool
	}{
		{
			name: "valid run",
			run: &domain.BenchmarkRun{
				ID:        "test",
				Status:    domain.RunStatusCompleted,
				CreatedAt: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "missing id",
			run: &domain.BenchmarkRun{
				ID:        "",
				Status:    domain.RunStatusCompleted,
				CreatedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "missing status",
			run: &domain.BenchmarkRun{
				ID:        "test",
				Status:    "",
				CreatedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "missing created_at",
			run: &domain.BenchmarkRun{
				ID:     "test",
				Status: domain.RunStatusCompleted,
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateRun(test.run)
			if (err != nil) != test.wantErr {
				t.Errorf("validateRun() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}
