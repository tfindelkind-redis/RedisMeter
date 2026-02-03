// Package plugin provides the plugin framework for RedisMeter.
package plugin

import (
	"context"

	"github.com/tfindelkind-redis/redismeter/internal/domain"
)

// ExecutorPlugin defines the interface for benchmark execution backends.
type ExecutorPlugin interface {
	Plugin

	// Execute starts a benchmark workload against the target.
	// Returns an execution handle for tracking progress.
	Execute(ctx context.Context, workload *domain.Workload, target *domain.Target) (*ExecutionHandle, error)

	// Status returns the current status of an execution.
	Status(ctx context.Context, handle *ExecutionHandle) (*ExecutionStatus, error)

	// Stop terminates a running execution.
	Stop(ctx context.Context, handle *ExecutionHandle) error

	// StreamMetrics returns a channel of real-time metrics.
	StreamMetrics(ctx context.Context, handle *ExecutionHandle) (<-chan *domain.Metrics, error)
}

// ExecutionHandle identifies a running benchmark execution.
type ExecutionHandle struct {
	ID        string `json:"id"`
	ExecutorName string `json:"executor_name"`
}

// ExecutionStatus represents the state of a benchmark execution.
type ExecutionStatus struct {
	Handle    *ExecutionHandle       `json:"handle"`
	State     ExecutionState         `json:"state"`
	Progress  float64                `json:"progress"` // 0.0 to 1.0
	StartTime string                 `json:"start_time,omitempty"`
	EndTime   string                 `json:"end_time,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Metrics   *domain.Metrics        `json:"metrics,omitempty"`
}

// ExecutionState represents the lifecycle state of an execution.
type ExecutionState string

const (
	StateQueued    ExecutionState = "queued"
	StateRunning   ExecutionState = "running"
	StateCompleted ExecutionState = "completed"
	StateFailed    ExecutionState = "failed"
	StateCancelled ExecutionState = "cancelled"
)
