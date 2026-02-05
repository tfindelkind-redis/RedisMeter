package logging

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewLogger(t *testing.T) {
	logger := NewLogger(Config{MinLevel: LevelDebug})
	if logger == nil {
		t.Fatal("NewLogger returned nil")
	}

	// Test that logger doesn't panic (uses nil store, console off)
	logger.Debug("test", "test message", nil)
	logger.Info("test", "test message", nil)
	logger.Warn("test", "test message", nil)
	logger.Error("test", "test message", nil)
}

func TestLogLevelFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	store, err := NewJSONStore(logDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Set level to WARN - should filter out DEBUG and INFO
	logger := NewLogger(Config{
		Store:    store,
		MinLevel: LevelWarn,
	})

	logger.Debug("test", "debug message", nil)
	logger.Info("test", "info message", nil)
	logger.Warn("test", "warn message", nil)
	logger.Error("test", "error message", nil)

	// Wait for background writer
	time.Sleep(2 * time.Second)

	// Query the store to see what was logged
	ctx := context.Background()
	entries, _ := store.Query(ctx, &QueryFilter{})

	// Should only have WARN and ERROR
	if len(entries) != 2 {
		t.Errorf("Expected 2 entries (WARN, ERROR), got %d", len(entries))
	}

	var hasWarn, hasError bool
	for _, e := range entries {
		if e.Level == LevelWarn {
			hasWarn = true
		}
		if e.Level == LevelError {
			hasError = true
		}
	}
	if !hasWarn {
		t.Error("WARN message should be logged")
	}
	if !hasError {
		t.Error("ERROR message should be logged")
	}
}

func TestLoggerWithBenchmark(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	store, _ := NewJSONStore(logDir)
	defer store.Close()

	logger := NewLogger(Config{Store: store, MinLevel: LevelDebug})
	benchLogger := logger.WithBenchmark("bench-123")
	benchLogger.Info("test", "benchmark message", nil)

	// Wait for background writer
	time.Sleep(2 * time.Second)

	ctx := context.Background()
	entries, _ := store.Query(ctx, &QueryFilter{BenchmarkID: "bench-123"})

	if len(entries) != 1 {
		t.Errorf("Expected 1 entry with benchmark ID, got %d", len(entries))
	}
}

func TestLoggerWithInfra(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	store, _ := NewJSONStore(logDir)
	defer store.Close()

	logger := NewLogger(Config{Store: store, MinLevel: LevelDebug})
	infraLogger := logger.WithInfra("infra-456")
	infraLogger.Info("test", "infra message", nil)

	// Wait for background writer
	time.Sleep(2 * time.Second)

	ctx := context.Background()
	entries, _ := store.Query(ctx, &QueryFilter{InfraID: "infra-456"})

	if len(entries) != 1 {
		t.Errorf("Expected 1 entry with infra ID, got %d", len(entries))
	}
}

func TestLoggerChaining(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	store, _ := NewJSONStore(logDir)
	defer store.Close()

	logger := NewLogger(Config{Store: store, MinLevel: LevelDebug})

	// Chain multiple WithX calls
	chainedLogger := logger.
		WithBenchmark("bench-1").
		WithInfra("infra-1")

	chainedLogger.Info("test", "chained message", nil)

	// Wait for background writer
	time.Sleep(2 * time.Second)

	ctx := context.Background()

	// Should find by benchmark
	entries, _ := store.Query(ctx, &QueryFilter{BenchmarkID: "bench-1"})
	if len(entries) != 1 {
		t.Error("Benchmark ID should be present")
	}

	// Should find by infra
	entries, _ = store.Query(ctx, &QueryFilter{InfraID: "infra-1"})
	if len(entries) != 1 {
		t.Error("Infra ID should be present")
	}
}

func TestJSONStore(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	store, err := NewJSONStore(logDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Test Save
	entry := &Entry{
		ID:          "test-1",
		Timestamp:   time.Now(),
		Level:       LevelInfo,
		Source:      SourceMemtier,
		Operation:   "test-op",
		Message:     "Test log entry",
		BenchmarkID: "bench-test",
	}

	if err := store.Save(ctx, entry); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Wait for background writer
	time.Sleep(2 * time.Second)

	// Test Query
	entries, err := store.Query(ctx, &QueryFilter{})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}

	// Test Count
	count, err := store.Count(ctx, &QueryFilter{})
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected count 1, got %d", count)
	}
}

func TestJSONStoreFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	store, err := NewJSONStore(logDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create entries with different levels and sources
	entries := []*Entry{
		{ID: "1", Timestamp: time.Now(), Level: LevelDebug, Source: SourceMemtier, Operation: "op1", Message: "debug memtier"},
		{ID: "2", Timestamp: time.Now(), Level: LevelInfo, Source: SourceMemtier, Operation: "op2", Message: "info memtier"},
		{ID: "3", Timestamp: time.Now(), Level: LevelWarn, Source: SourceTerraform, Operation: "op3", Message: "warn terraform"},
		{ID: "4", Timestamp: time.Now(), Level: LevelError, Source: SourceAPI, Operation: "op4", Message: "error api", BenchmarkID: "bench-1"},
	}

	for _, e := range entries {
		store.Save(ctx, e)
	}

	// Wait for background writer
	time.Sleep(2 * time.Second)

	// Filter by level
	filter := &QueryFilter{Level: LevelError}
	results, _ := store.Query(ctx, filter)
	if len(results) != 1 {
		t.Errorf("Expected 1 error entry, got %d", len(results))
	}

	// Filter by source
	filter = &QueryFilter{Source: SourceMemtier}
	results, _ = store.Query(ctx, filter)
	if len(results) != 2 {
		t.Errorf("Expected 2 memtier entries, got %d", len(results))
	}

	// Filter by benchmark ID
	filter = &QueryFilter{BenchmarkID: "bench-1"}
	results, _ = store.Query(ctx, filter)
	if len(results) != 1 {
		t.Errorf("Expected 1 entry with benchmark ID, got %d", len(results))
	}

	// Filter with limit
	filter = &QueryFilter{Limit: 2}
	results, _ = store.Query(ctx, filter)
	if len(results) != 2 {
		t.Errorf("Expected 2 entries with limit, got %d", len(results))
	}
}

func TestJSONStoreDelete(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	store, err := NewJSONStore(logDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Add entries
	oldTime := time.Now().Add(-48 * time.Hour)
	recentTime := time.Now()

	store.Save(ctx, &Entry{ID: "old-1", Timestamp: oldTime, Level: LevelInfo, Source: SourceMemtier, Operation: "op", Message: "old"})
	store.Save(ctx, &Entry{ID: "new-1", Timestamp: recentTime, Level: LevelInfo, Source: SourceMemtier, Operation: "op", Message: "recent"})

	// Wait for background writer
	time.Sleep(2 * time.Second)

	// Delete old entries (before 24 hours ago)
	cutoff := time.Now().Add(-24 * time.Hour)
	deleted, err := store.Delete(ctx, cutoff)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if deleted != 1 {
		t.Errorf("Expected 1 deleted, got %d", deleted)
	}

	// Verify
	remaining, _ := store.Count(ctx, &QueryFilter{})
	if remaining != 1 {
		t.Errorf("Expected 1 remaining, got %d", remaining)
	}
}

func TestJSONStoreStats(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	store, err := NewJSONStore(logDir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Add entries with different levels
	store.Save(ctx, &Entry{ID: "1", Timestamp: time.Now(), Level: LevelDebug, Source: SourceMemtier, Operation: "op", Message: "debug"})
	store.Save(ctx, &Entry{ID: "2", Timestamp: time.Now(), Level: LevelInfo, Source: SourceMemtier, Operation: "op", Message: "info"})
	store.Save(ctx, &Entry{ID: "3", Timestamp: time.Now(), Level: LevelWarn, Source: SourceAPI, Operation: "op", Message: "warn"})
	store.Save(ctx, &Entry{ID: "4", Timestamp: time.Now(), Level: LevelError, Source: SourceAPI, Operation: "op", Message: "error1"})
	store.Save(ctx, &Entry{ID: "5", Timestamp: time.Now(), Level: LevelError, Source: SourceTerraform, Operation: "op", Message: "error2"})

	// Wait for background writer
	time.Sleep(2 * time.Second)

	stats, err := store.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats.TotalEntries != 5 {
		t.Errorf("Expected 5 total entries, got %d", stats.TotalEntries)
	}
	if stats.LevelCounts[LevelError] != 2 {
		t.Errorf("Expected 2 errors, got %d", stats.LevelCounts[LevelError])
	}
}

func TestCommandLog(t *testing.T) {
	cmdLog := &CommandLog{
		Command:   "memtier_benchmark",
		Args:      []string{"-s", "localhost"},
		StartTime: time.Now(),
	}

	if cmdLog.Command != "memtier_benchmark" {
		t.Error("Command not set")
	}
	if cmdLog.StartTime.IsZero() {
		t.Error("StartTime should be set")
	}

	// Simulate command completion
	time.Sleep(10 * time.Millisecond)
	cmdLog.EndTime = time.Now()
	cmdLog.Duration = cmdLog.EndTime.Sub(cmdLog.StartTime)
	cmdLog.ExitCode = 0
	cmdLog.Stdout = "output data"

	if cmdLog.ExitCode != 0 {
		t.Error("ExitCode not set correctly")
	}
	if cmdLog.EndTime.IsZero() {
		t.Error("EndTime should be set")
	}
	if cmdLog.Duration <= 0 {
		t.Error("Duration should be positive")
	}
	if cmdLog.Stdout != "output data" {
		t.Error("Stdout not captured")
	}
}

func TestLoggerWithContext(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "logs")

	store, _ := NewJSONStore(logDir)
	defer store.Close()

	logger := NewLogger(Config{Store: store, MinLevel: LevelDebug})

	// Log with context
	ctx := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	}
	logger.Info("test-op", "message with context", ctx)

	// Wait for background writer
	time.Sleep(2 * time.Second)

	bgCtx := context.Background()
	entries, _ := store.Query(bgCtx, &QueryFilter{})

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	if entries[0].Context["key1"] != "value1" {
		t.Error("Context key1 not preserved")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
