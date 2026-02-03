// Package storage provides persistence backends for RedisMeter.
package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
	"github.com/tfindelkind-redis/redismeter/internal/plugin"
)

func TestNewSQLiteStorage(t *testing.T) {
	// Create temp directory for test database
	tmpDir, err := os.MkdirTemp("", "redismeter-sqlite-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")

	storage, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create SQLite storage: %v", err)
	}
	defer storage.Shutdown(context.Background())

	// Verify database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}

	// Check metadata
	meta := storage.Metadata()
	if meta.Name != "sqlite" {
		t.Errorf("Expected metadata name 'sqlite', got %s", meta.Name)
	}
	if meta.Type != plugin.TypeStorage {
		t.Errorf("Expected plugin type storage, got %v", meta.Type)
	}
}

func TestSQLiteStorage_HealthCheck(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "redismeter-sqlite-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := NewSQLiteStorage(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Shutdown(context.Background())

	ctx := context.Background()
	health := storage.HealthCheck(ctx)

	if !health.Healthy {
		t.Errorf("Expected healthy status, got: %s", health.Message)
	}
}

func TestSQLiteStorage_SaveAndLoadRun(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "redismeter-sqlite-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := NewSQLiteStorage(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Shutdown(context.Background())

	ctx := context.Background()

	// Create a test run
	run := &domain.BenchmarkRun{
		ID:        "test-run-123",
		CreatedAt: time.Now(),
		Status:    domain.RunStatusCompleted,
		Name:      "Test Run",
		Workload: &domain.Workload{
			Name:        "cache-test",
			Type:        "builtin",
			Description: "Cache test workload",
		},
		Target: &domain.Target{
			Host: "localhost",
			Port: 6379,
		},
		Results: &domain.Results{
			Summary: &domain.SummaryMetrics{
				TotalOps:     100000,
				OpsPerSecond: 9523.81,
				AvgLatencyMs: 1.05,
				MinLatencyMs: 0.5,
				MaxLatencyMs: 5.2,
			},
		},
		Tags: []string{"test", "ci"},
	}

	// Save the run
	id, err := storage.Save(ctx, "runs", run)
	if err != nil {
		t.Fatalf("Failed to save run: %v", err)
	}
	if id != run.ID {
		t.Errorf("Expected ID %s, got %s", run.ID, id)
	}

	// Load the run back
	var loaded domain.BenchmarkRun
	err = storage.Load(ctx, "runs", run.ID, &loaded)
	if err != nil {
		t.Fatalf("Failed to load run: %v", err)
	}

	// Verify loaded data
	if loaded.ID != run.ID {
		t.Errorf("Loaded ID mismatch: expected %s, got %s", run.ID, loaded.ID)
	}
	if loaded.Status != run.Status {
		t.Errorf("Loaded status mismatch: expected %s, got %s", run.Status, loaded.Status)
	}
	if loaded.Name != run.Name {
		t.Errorf("Loaded name mismatch: expected %s, got %s", run.Name, loaded.Name)
	}
	if loaded.Workload == nil || loaded.Workload.Name != run.Workload.Name {
		t.Error("Workload not loaded correctly")
	}
	if loaded.Target == nil || loaded.Target.Host != run.Target.Host {
		t.Error("Target not loaded correctly")
	}
	if loaded.Results == nil || loaded.Results.Summary == nil {
		t.Error("Results not loaded correctly")
	} else if loaded.Results.Summary.TotalOps != run.Results.Summary.TotalOps {
		t.Errorf("TotalOps mismatch: expected %d, got %d", run.Results.Summary.TotalOps, loaded.Results.Summary.TotalOps)
	}
}

func TestSQLiteStorage_QueryRuns(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "redismeter-sqlite-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := NewSQLiteStorage(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Shutdown(context.Background())

	ctx := context.Background()

	// Create test runs
	runs := []*domain.BenchmarkRun{
		{
			ID:        "run-1",
			CreatedAt: time.Now().Add(-2 * time.Hour),
			Status:    domain.RunStatusCompleted,
			Workload:  &domain.Workload{Name: "cache-test"},
			Target:    &domain.Target{Host: "localhost", Port: 6379},
			Tags:      []string{"production", "cache"},
		},
		{
			ID:        "run-2",
			CreatedAt: time.Now().Add(-1 * time.Hour),
			Status:    domain.RunStatusCompleted,
			Workload:  &domain.Workload{Name: "write-heavy"},
			Target:    &domain.Target{Host: "localhost", Port: 6379},
			Tags:      []string{"staging"},
		},
		{
			ID:        "run-3",
			CreatedAt: time.Now(),
			Status:    domain.RunStatusFailed,
			Workload:  &domain.Workload{Name: "cache-test"},
			Target:    &domain.Target{Host: "redis.example.com", Port: 6380},
			Tags:      []string{"production"},
		},
	}

	for _, run := range runs {
		_, err := storage.Save(ctx, "runs", run)
		if err != nil {
			t.Fatalf("Failed to save run %s: %v", run.ID, err)
		}
	}

	// Test query with no filter (should return all)
	var allRuns []*domain.BenchmarkRun
	err = storage.Query(ctx, "runs", plugin.QueryFilter{}, &allRuns)
	if err != nil {
		t.Fatalf("Failed to query runs: %v", err)
	}
	if len(allRuns) != 3 {
		t.Errorf("Expected 3 runs, got %d", len(allRuns))
	}

	// Test query with status filter
	var completedRuns []*domain.BenchmarkRun
	err = storage.Query(ctx, "runs", plugin.QueryFilter{
		Conditions: map[string]interface{}{"status": "completed"},
	}, &completedRuns)
	if err != nil {
		t.Fatalf("Failed to query completed runs: %v", err)
	}
	if len(completedRuns) != 2 {
		t.Errorf("Expected 2 completed runs, got %d", len(completedRuns))
	}

	// Test query with workload filter
	var cacheRuns []*domain.BenchmarkRun
	err = storage.Query(ctx, "runs", plugin.QueryFilter{
		Conditions: map[string]interface{}{"workload": "cache-test"},
	}, &cacheRuns)
	if err != nil {
		t.Fatalf("Failed to query cache-test runs: %v", err)
	}
	if len(cacheRuns) != 2 {
		t.Errorf("Expected 2 cache-test runs, got %d", len(cacheRuns))
	}

	// Test query with tag filter
	var prodRuns []*domain.BenchmarkRun
	err = storage.Query(ctx, "runs", plugin.QueryFilter{
		Tags: []string{"production"},
	}, &prodRuns)
	if err != nil {
		t.Fatalf("Failed to query production runs: %v", err)
	}
	if len(prodRuns) != 2 {
		t.Errorf("Expected 2 production runs, got %d", len(prodRuns))
	}

	// Test query with limit
	var limitedRuns []*domain.BenchmarkRun
	err = storage.Query(ctx, "runs", plugin.QueryFilter{Limit: 1}, &limitedRuns)
	if err != nil {
		t.Fatalf("Failed to query with limit: %v", err)
	}
	if len(limitedRuns) != 1 {
		t.Errorf("Expected 1 run with limit, got %d", len(limitedRuns))
	}
}

func TestSQLiteStorage_DeleteRun(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "redismeter-sqlite-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := NewSQLiteStorage(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Shutdown(context.Background())

	ctx := context.Background()

	// Save a run
	run := &domain.BenchmarkRun{
		ID:        "delete-test-run",
		CreatedAt: time.Now(),
		Status:    domain.RunStatusCompleted,
	}
	storage.Save(ctx, "runs", run)

	// Delete it
	err = storage.Delete(ctx, "runs", run.ID)
	if err != nil {
		t.Fatalf("Failed to delete run: %v", err)
	}

	// Verify it's gone
	var loaded domain.BenchmarkRun
	err = storage.Load(ctx, "runs", run.ID, &loaded)
	if err == nil {
		t.Error("Expected error loading deleted run")
	}
}

func TestSQLiteStorage_SaveAndLoadBaseline(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "redismeter-sqlite-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := NewSQLiteStorage(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Shutdown(context.Background())

	ctx := context.Background()

	// Create a baseline
	baseline := &domain.Baseline{
		ID:          "baseline-123",
		CreatedAt:   time.Now(),
		Name:        "Production Baseline",
		Description: "Baseline for production cache workload",
		Active:      true,
		Metrics: &domain.SummaryMetrics{
			TotalOps:     1000000,
			OpsPerSecond: 50000,
		},
		Thresholds: &domain.BaselineThresholds{
			MaxThroughputRegression: 0.10,
			MaxLatencyRegression:    0.15,
		},
	}

	// Save the baseline
	id, err := storage.Save(ctx, "baselines", baseline)
	if err != nil {
		t.Fatalf("Failed to save baseline: %v", err)
	}
	if id != baseline.ID {
		t.Errorf("Expected ID %s, got %s", baseline.ID, id)
	}

	// Load the baseline back
	var loaded domain.Baseline
	err = storage.Load(ctx, "baselines", baseline.ID, &loaded)
	if err != nil {
		t.Fatalf("Failed to load baseline: %v", err)
	}

	// Verify loaded data
	if loaded.ID != baseline.ID {
		t.Errorf("Loaded ID mismatch: expected %s, got %s", baseline.ID, loaded.ID)
	}
	if loaded.Name != baseline.Name {
		t.Errorf("Loaded name mismatch: expected %s, got %s", baseline.Name, loaded.Name)
	}
	if !loaded.Active {
		t.Error("Expected active=true")
	}
	if loaded.Metrics == nil || loaded.Metrics.OpsPerSecond != baseline.Metrics.OpsPerSecond {
		t.Error("Metrics not loaded correctly")
	}
	if loaded.Thresholds == nil || loaded.Thresholds.MaxThroughputRegression != baseline.Thresholds.MaxThroughputRegression {
		t.Error("Thresholds not loaded correctly")
	}
}

func TestSQLiteStorage_GetStats(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "redismeter-sqlite-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := NewSQLiteStorage(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Shutdown(context.Background())

	ctx := context.Background()

	// Save some test data
	storage.Save(ctx, "runs", &domain.BenchmarkRun{
		ID:        "run-1",
		CreatedAt: time.Now(),
		Status:    domain.RunStatusCompleted,
	})
	storage.Save(ctx, "runs", &domain.BenchmarkRun{
		ID:        "run-2",
		CreatedAt: time.Now(),
		Status:    domain.RunStatusFailed,
	})
	storage.Save(ctx, "baselines", &domain.Baseline{
		ID:        "baseline-1",
		CreatedAt: time.Now(),
		Name:      "Test Baseline",
	})

	// Get stats
	stats, err := storage.GetStats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	// Verify stats
	totalRuns, ok := stats["total_runs"].(int)
	if !ok || totalRuns != 2 {
		t.Errorf("Expected total_runs=2, got %v", stats["total_runs"])
	}

	totalBaselines, ok := stats["total_baselines"].(int)
	if !ok || totalBaselines != 1 {
		t.Errorf("Expected total_baselines=1, got %v", stats["total_baselines"])
	}

	dbSize, ok := stats["db_size_bytes"].(int64)
	if !ok || dbSize == 0 {
		t.Error("Expected non-zero db_size_bytes")
	}
}

func TestSQLiteStorage_ConvenienceMethods(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "redismeter-sqlite-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := NewSQLiteStorage(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Shutdown(context.Background())

	ctx := context.Background()

	// Save a run
	run := &domain.BenchmarkRun{
		ID:        "convenience-test",
		CreatedAt: time.Now(),
		Status:    domain.RunStatusCompleted,
		Name:      "Convenience Test",
	}
	storage.Save(ctx, "runs", run)

	// Test ListRuns
	runs, err := storage.ListRuns(ctx, 10)
	if err != nil {
		t.Fatalf("ListRuns failed: %v", err)
	}
	if len(runs) != 1 {
		t.Errorf("Expected 1 run, got %d", len(runs))
	}

	// Test GetRun
	loaded, err := storage.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}
	if loaded.Name != run.Name {
		t.Errorf("GetRun name mismatch: expected %s, got %s", run.Name, loaded.Name)
	}

	// Test DeleteRun
	err = storage.DeleteRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("DeleteRun failed: %v", err)
	}

	// Verify deletion
	_, err = storage.GetRun(ctx, run.ID)
	if err == nil {
		t.Error("Expected error after DeleteRun")
	}
}

func TestSQLiteStorage_CascadeDelete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "redismeter-sqlite-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := NewSQLiteStorage(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Shutdown(context.Background())

	ctx := context.Background()

	// Save a run with tags
	run := &domain.BenchmarkRun{
		ID:        "cascade-test",
		CreatedAt: time.Now(),
		Status:    domain.RunStatusCompleted,
		Tags:      []string{"tag1", "tag2", "tag3"},
	}
	storage.Save(ctx, "runs", run)

	// Delete the run (should cascade delete tags)
	err = storage.Delete(ctx, "runs", run.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify tags were deleted (query by tag should return empty)
	var results []*domain.BenchmarkRun
	storage.Query(ctx, "runs", plugin.QueryFilter{Tags: []string{"tag1"}}, &results)
	if len(results) != 0 {
		t.Error("Expected tags to be cascade deleted")
	}
}

func TestSQLiteStorage_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "redismeter-sqlite-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := NewSQLiteStorage(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Shutdown(context.Background())

	ctx := context.Background()

	// Try to load non-existent run
	var run domain.BenchmarkRun
	err = storage.Load(ctx, "runs", "non-existent", &run)
	if err == nil {
		t.Error("Expected error for non-existent run")
	}

	// Try to delete non-existent run
	err = storage.Delete(ctx, "runs", "non-existent")
	if err == nil {
		t.Error("Expected error for deleting non-existent run")
	}
}

func TestSQLiteStorage_UnsupportedEntityType(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "redismeter-sqlite-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage, err := NewSQLiteStorage(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Shutdown(context.Background())

	ctx := context.Background()

	// Try with unsupported entity type
	_, err = storage.Save(ctx, "unknown", struct{}{})
	if err == nil {
		t.Error("Expected error for unsupported entity type")
	}

	var dest interface{}
	err = storage.Load(ctx, "unknown", "id", &dest)
	if err == nil {
		t.Error("Expected error for unsupported entity type")
	}

	err = storage.Query(ctx, "unknown", plugin.QueryFilter{}, &dest)
	if err == nil {
		t.Error("Expected error for unsupported entity type")
	}

	err = storage.Delete(ctx, "unknown", "id")
	if err == nil {
		t.Error("Expected error for unsupported entity type")
	}
}
