// Package storage provides persistence backends for RedisMeter.
package storage

import (
	"context"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
	"github.com/tfindelkind-redis/redismeter/internal/plugin"
)

// RunStorage provides run-specific storage operations.
// This interface extends the base StoragePlugin with convenience methods
// for working with benchmark runs.
type RunStorage interface {
	plugin.StoragePlugin

	// SaveRun persists a benchmark run.
	SaveRun(ctx context.Context, run *domain.BenchmarkRun) error

	// ListRuns returns recent benchmark runs.
	ListRuns(ctx context.Context, limit int) ([]*domain.BenchmarkRun, error)

	// GetRun retrieves a single benchmark run by ID.
	GetRun(ctx context.Context, id string) (*domain.BenchmarkRun, error)

	// DeleteRun removes a benchmark run by ID.
	DeleteRun(ctx context.Context, id string) error
}

// Ensure both storage implementations satisfy RunStorage.
var _ RunStorage = (*FileStorage)(nil)
var _ RunStorage = (*SQLiteStorage)(nil)
