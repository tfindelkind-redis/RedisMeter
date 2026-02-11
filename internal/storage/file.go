// Package storage provides persistence backends for RedisMeter.
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
	"github.com/tfindelkind-redis/redismeter/internal/plugin"
)

// FileStorage implements StoragePlugin using the local filesystem.
type FileStorage struct {
	baseDir string
	mu      sync.RWMutex
}

// NewFileStorage creates a new file-based storage plugin.
func NewFileStorage(baseDir string) (*FileStorage, error) {
	if baseDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		baseDir = filepath.Join(home, ".redismeter")
	}

	fs := &FileStorage{baseDir: baseDir}

	// Create directory structure
	dirs := []string{
		baseDir,
		filepath.Join(baseDir, "runs"),
		filepath.Join(baseDir, "baseline"),
		filepath.Join(baseDir, "workloads"),
		filepath.Join(baseDir, "config"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return fs, nil
}

// Metadata returns plugin metadata.
func (fs *FileStorage) Metadata() plugin.Metadata {
	return plugin.Metadata{
		Name:        "file",
		Version:     "1.0.0",
		Type:        plugin.TypeStorage,
		Description: "File-based storage using JSON files",
	}
}

// Initialize sets up the plugin.
func (fs *FileStorage) Initialize(ctx context.Context, config map[string]interface{}) error {
	if dir, ok := config["base_dir"].(string); ok && dir != "" {
		fs.baseDir = dir
	}
	return nil
}

// HealthCheck returns the plugin health status.
func (fs *FileStorage) HealthCheck(ctx context.Context) plugin.HealthStatus {
	// Check if base directory is writable
	testFile := filepath.Join(fs.baseDir, ".health_check")
	if err := os.WriteFile(testFile, []byte("ok"), 0644); err != nil {
		return plugin.HealthStatus{Healthy: false, Message: err.Error()}
	}
	os.Remove(testFile)
	return plugin.HealthStatus{Healthy: true, Message: "OK"}
}

// Shutdown gracefully stops the plugin.
func (fs *FileStorage) Shutdown(ctx context.Context) error {
	return nil
}

// entityDir returns the directory for a given entity type.
func (fs *FileStorage) entityDir(entityType string) string {
	return filepath.Join(fs.baseDir, entityType)
}

// entityPath returns the file path for a specific entity.
func (fs *FileStorage) entityPath(entityType, id string) string {
	return filepath.Join(fs.entityDir(entityType), id+".json")
}

// Save stores an entity and returns its ID.
func (fs *FileStorage) Save(ctx context.Context, entityType string, entity interface{}) (string, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Get ID from entity
	id := fs.getEntityID(entity)
	if id == "" {
		return "", fmt.Errorf("entity has no ID")
	}

	data, err := json.MarshalIndent(entity, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal entity: %w", err)
	}

	// Ensure directory exists
	dir := fs.entityDir(entityType)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	path := fs.entityPath(entityType, id)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return id, nil
}

// Load retrieves an entity by ID.
func (fs *FileStorage) Load(ctx context.Context, entityType string, id string, dest interface{}) error {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	path := fs.entityPath(entityType, id)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("entity not found: %s/%s", entityType, id)
		}
		return fmt.Errorf("failed to read file: %w", err)
	}

	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("failed to unmarshal entity: %w", err)
	}

	return nil
}

// Query finds entities matching the given filter.
func (fs *FileStorage) Query(ctx context.Context, entityType string, filter plugin.QueryFilter, dest interface{}) error {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	dir := fs.entityDir(entityType)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Empty result
		}
		return fmt.Errorf("failed to read directory: %w", err)
	}

	// Handle different entity types
	switch entityType {
	case "baseline":
		return fs.queryBaselines(entries, dir, filter, dest)
	default:
		return fs.queryRuns(entries, dir, filter, dest)
	}
}

// queryBaselines handles querying baseline entities.
func (fs *FileStorage) queryBaselines(entries []os.DirEntry, dir string, filter plugin.QueryFilter, dest interface{}) error {
	var results []*domain.Baseline

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var baseline domain.Baseline
		if err := json.Unmarshal(data, &baseline); err != nil {
			continue
		}

		results = append(results, &baseline)
	}

	// Sort by created_at descending (newest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	// Apply limit
	if filter.Limit > 0 && len(results) > filter.Limit {
		results = results[:filter.Limit]
	}

	// Copy to destination
	if baselinesPtr, ok := dest.(*[]*domain.Baseline); ok {
		*baselinesPtr = results
	}

	return nil
}

// queryRuns handles querying benchmark run entities.
func (fs *FileStorage) queryRuns(entries []os.DirEntry, dir string, filter plugin.QueryFilter, dest interface{}) error {
	var results []*domain.BenchmarkRun

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var run domain.BenchmarkRun
		if err := json.Unmarshal(data, &run); err != nil {
			continue
		}

		// Apply filters
		if fs.matchesFilter(&run, filter) {
			results = append(results, &run)
		}
	}

	// Sort by created_at descending (newest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	// Apply limit
	if filter.Limit > 0 && len(results) > filter.Limit {
		results = results[:filter.Limit]
	}

	// Copy to destination
	if runsPtr, ok := dest.(*[]*domain.BenchmarkRun); ok {
		*runsPtr = results
	}

	return nil
}

// Delete removes an entity by ID.
func (fs *FileStorage) Delete(ctx context.Context, entityType string, id string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	path := fs.entityPath(entityType, id)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("entity not found: %s/%s", entityType, id)
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// matchesFilter checks if a run matches the query filter.
func (fs *FileStorage) matchesFilter(run *domain.BenchmarkRun, filter plugin.QueryFilter) bool {
	// Tag filter
	if len(filter.Tags) > 0 {
		found := false
		for _, filterTag := range filter.Tags {
			for _, runTag := range run.Tags {
				if runTag == filterTag {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	// Time range filter
	if filter.TimeRange != nil {
		if filter.TimeRange.Start != "" {
			startTime, err := time.Parse(time.RFC3339, filter.TimeRange.Start)
			if err == nil && run.CreatedAt.Before(startTime) {
				return false
			}
		}
		if filter.TimeRange.End != "" {
			endTime, err := time.Parse(time.RFC3339, filter.TimeRange.End)
			if err == nil && run.CreatedAt.After(endTime) {
				return false
			}
		}
	}

	return true
}

// getEntityID extracts the ID from various entity types.
func (fs *FileStorage) getEntityID(entity interface{}) string {
	switch e := entity.(type) {
	case *domain.BenchmarkRun:
		return e.ID
	case *domain.Baseline:
		return e.ID
	default:
		return ""
	}
}

// ListRuns returns all benchmark runs.
func (fs *FileStorage) ListRuns(ctx context.Context, limit int) ([]*domain.BenchmarkRun, error) {
	var runs []*domain.BenchmarkRun
	filter := plugin.QueryFilter{Limit: limit}
	if err := fs.Query(ctx, "runs", filter, &runs); err != nil {
		return nil, err
	}
	return runs, nil
}

// GetRun retrieves a single benchmark run by ID.
func (fs *FileStorage) GetRun(ctx context.Context, id string) (*domain.BenchmarkRun, error) {
	var run domain.BenchmarkRun
	if err := fs.Load(ctx, "runs", id, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

// SaveRun stores a benchmark run.
func (fs *FileStorage) SaveRun(ctx context.Context, run *domain.BenchmarkRun) error {
	_, err := fs.Save(ctx, "runs", run)
	return err
}

// DeleteRun removes a benchmark run.
func (fs *FileStorage) DeleteRun(ctx context.Context, id string) error {
	return fs.Delete(ctx, "runs", id)
}
