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

func TestFileStorage(t *testing.T) {
	// Create temp directory for tests
	tmpDir := t.TempDir()
	ctx := context.Background()
	
	storage, err := NewFileStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStorage() error = %v", err)
	}

	// Test Save
	t.Run("Save", func(t *testing.T) {
		run := createTestRun("test-001")
		id, err := storage.Save(ctx, "runs", run)
		if err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		if id != run.ID {
			t.Errorf("Save() returned id = %q, want %q", id, run.ID)
		}

		// Verify file exists
		runFile := filepath.Join(tmpDir, "runs", run.ID+".json")
		if _, err := os.Stat(runFile); os.IsNotExist(err) {
			t.Errorf("Run file not created at %s", runFile)
		}
	})

	// Test Load
	t.Run("Load", func(t *testing.T) {
		// Save first
		run := createTestRun("test-002")
		if _, err := storage.Save(ctx, "runs", run); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		// Load it back
		var loaded domain.BenchmarkRun
		err := storage.Load(ctx, "runs", run.ID, &loaded)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if loaded.ID != run.ID {
			t.Errorf("ID = %q, want %q", loaded.ID, run.ID)
		}
		if loaded.Status != run.Status {
			t.Errorf("Status = %q, want %q", loaded.Status, run.Status)
		}
		if loaded.Workload.Name != run.Workload.Name {
			t.Errorf("Workload.Name = %q, want %q", loaded.Workload.Name, run.Workload.Name)
		}
	})

	// Test Load nonexistent
	t.Run("LoadNonexistent", func(t *testing.T) {
		var loaded domain.BenchmarkRun
		err := storage.Load(ctx, "runs", "nonexistent-id", &loaded)
		if err == nil {
			t.Error("Expected error loading nonexistent run")
		}
	})

	// Test Query
	t.Run("Query", func(t *testing.T) {
		// Save a few runs
		for i := 0; i < 3; i++ {
			run := createTestRun("query-test-" + string(rune('a'+i)))
			if _, err := storage.Save(ctx, "runs", run); err != nil {
				t.Fatalf("Save() error = %v", err)
			}
		}

		var results []*domain.BenchmarkRun
		err := storage.Query(ctx, "runs", plugin.QueryFilter{}, &results)
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}

		if len(results) < 3 {
			t.Errorf("Query() returned %d runs, want at least 3", len(results))
		}
	})

	// Test Delete
	t.Run("Delete", func(t *testing.T) {
		run := createTestRun("delete-test")
		if _, err := storage.Save(ctx, "runs", run); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		// Verify it exists
		var loaded domain.BenchmarkRun
		if err := storage.Load(ctx, "runs", run.ID, &loaded); err != nil {
			t.Fatalf("Run should exist before delete: %v", err)
		}

		// Delete it
		if err := storage.Delete(ctx, "runs", run.ID); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		// Verify it's gone
		if err := storage.Load(ctx, "runs", run.ID, &loaded); err == nil {
			t.Error("Run should not exist after delete")
		}
	})

	// Test Delete nonexistent
	t.Run("DeleteNonexistent", func(t *testing.T) {
		err := storage.Delete(ctx, "runs", "nonexistent-id")
		if err == nil {
			t.Error("Expected error deleting nonexistent run")
		}
	})
}

func TestFileStorageQueryWithFilters(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()
	storage, _ := NewFileStorage(tmpDir)

	// Create runs with different statuses and workloads
	runs := []struct {
		id       string
		status   domain.RunStatus
		workload string
	}{
		{"query-001", domain.RunStatusCompleted, "cache"},
		{"query-002", domain.RunStatusCompleted, "write-heavy"},
		{"query-003", domain.RunStatusFailed, "cache"},
		{"query-004", domain.RunStatusCompleted, "cache"},
	}

	for _, r := range runs {
		run := &domain.BenchmarkRun{
			ID:        r.id,
			CreatedAt: time.Now(),
			Status:    r.status,
			Workload: &domain.Workload{
				Name: r.workload,
			},
			Target: &domain.Target{
				Host: "localhost",
				Port: 6379,
			},
			StartTime: time.Now(),
		}
		if _, err := storage.Save(ctx, "runs", run); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	}

	// Test Query returns all runs with empty filter
	t.Run("QueryAll", func(t *testing.T) {
		var results []*domain.BenchmarkRun
		err := storage.Query(ctx, "runs", plugin.QueryFilter{}, &results)
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}

		// Should find all 4 runs
		if len(results) != 4 {
			t.Errorf("Query() returned %d runs, want 4", len(results))
		}
	})

	// Test Query with limit
	t.Run("QueryWithLimit", func(t *testing.T) {
		var results []*domain.BenchmarkRun
		err := storage.Query(ctx, "runs", plugin.QueryFilter{
			Limit: 2,
		}, &results)
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}

		// Should only return 2 runs
		if len(results) != 2 {
			t.Errorf("Query(limit=2) returned %d runs, want 2", len(results))
		}
	})
}

func TestFileStorageMetadata(t *testing.T) {
	storage := &FileStorage{}
	meta := storage.Metadata()

	if meta.Name != "file" {
		t.Errorf("Name = %q, want %q", meta.Name, "file")
	}
	if meta.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", meta.Version, "1.0.0")
	}
}

// Helper to create test runs
func createTestRun(id string) *domain.BenchmarkRun {
	return &domain.BenchmarkRun{
		ID:        id,
		CreatedAt: time.Now(),
		Status:    domain.RunStatusCompleted,
		Workload: &domain.Workload{
			Name:        "test-workload",
			Description: "Test workload for unit tests",
			Threads:     4,
			Clients:     50,
			Requests:    10000,
		},
		Target: &domain.Target{
			Host: "localhost",
			Port: 6379,
		},
		Environment: &domain.Environment{
			OS:       "darwin",
			Arch:     "arm64",
			Hostname: "test-host",
		},
		Results: &domain.Results{
			Summary: &domain.SummaryMetrics{
				TotalOps:     100000,
				AvgLatencyMs: 1.5,
				P50LatencyMs: 1.2,
				P99LatencyMs: 3.5,
				P999LatencyMs: 5.0,
			},
		},
		StartTime: time.Now().Add(-10 * time.Second),
		EndTime:   time.Now(),
	}
}
