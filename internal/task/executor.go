package task

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// StepHandler defines how a specific step is executed.
type StepHandler interface {
	// Execute runs the step. It should:
	// - Check for existing checkpoint and resume if possible
	// - Update progress periodically via the callback
	// - Return an error if the step fails
	Execute(ctx context.Context, task *Task, step *Step, progress ProgressFunc) error

	// CanResume returns true if this step supports resuming from checkpoint.
	CanResume(step *Step) bool

	// Validate checks if the step can be executed (prerequisites met, etc.)
	Validate(ctx context.Context, task *Task) error
}

// ProgressFunc is called to report step progress.
type ProgressFunc func(percent int, message string)

// Executor runs tasks through their step workflow.
type Executor struct {
	store           *Store
	handlers        map[StepType]StepHandler
	mu              sync.RWMutex
	running         map[string]context.CancelFunc // Active task cancellation functions
	checkpointEvery time.Duration                 // How often to save checkpoints
}

// NewExecutor creates a new task executor.
func NewExecutor(store *Store) *Executor {
	return &Executor{
		store:           store,
		handlers:        make(map[StepType]StepHandler),
		running:         make(map[string]context.CancelFunc),
		checkpointEvery: 30 * time.Second,
	}
}

// RegisterHandler registers a handler for a step type.
func (e *Executor) RegisterHandler(stepType StepType, handler StepHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.handlers[stepType] = handler
}

// CreateTask creates a new task and persists it.
func (e *Executor) CreateTask(ctx context.Context, name string, tool ToolType, config string) (*Task, error) {
	id := uuid.New().String()
	task := NewTask(id, name, tool)
	task.ToolConfig = config

	// Add standard steps for this tool
	steps := GetStandardSteps(tool)
	task.AddSteps(steps)

	if err := e.store.Save(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to save task: %w", err)
	}

	return task, nil
}

// Start begins executing a task from its current position.
func (e *Executor) Start(ctx context.Context, taskID string) error {
	task, err := e.store.Get(ctx, taskID)
	if err != nil {
		return err
	}

	if task.IsTerminal() {
		return fmt.Errorf("task %s is in terminal state: %s", taskID, task.Status)
	}

	// Check if already running
	e.mu.RLock()
	_, running := e.running[taskID]
	e.mu.RUnlock()
	if running {
		return fmt.Errorf("task %s is already running", taskID)
	}

	// Create cancellable context
	runCtx, cancel := context.WithCancel(ctx)

	e.mu.Lock()
	e.running[taskID] = cancel
	e.mu.Unlock()

	// Run in background
	go func() {
		defer func() {
			e.mu.Lock()
			delete(e.running, taskID)
			e.mu.Unlock()
		}()

		if err := e.run(runCtx, task); err != nil {
			// Task already marked as failed in run()
			fmt.Printf("Task %s failed: %v\n", taskID, err)
		}
	}()

	return nil
}

// Resume resumes a paused or failed task.
func (e *Executor) Resume(ctx context.Context, taskID string) error {
	task, err := e.store.Get(ctx, taskID)
	if err != nil {
		return err
	}

	if !task.CanResume() {
		return fmt.Errorf("task %s cannot be resumed (status: %s)", taskID, task.Status)
	}

	task.Resume()
	if err := e.store.Save(ctx, task); err != nil {
		return err
	}

	return e.Start(ctx, taskID)
}

// Pause pauses a running task.
func (e *Executor) Pause(ctx context.Context, taskID string) error {
	e.mu.Lock()
	cancel, ok := e.running[taskID]
	e.mu.Unlock()

	if !ok {
		return fmt.Errorf("task %s is not running", taskID)
	}

	cancel() // Cancel the context, causing the task to pause

	task, err := e.store.Get(ctx, taskID)
	if err != nil {
		return err
	}

	task.Pause()
	return e.store.Save(ctx, task)
}

// Cancel cancels a task permanently.
func (e *Executor) Cancel(ctx context.Context, taskID string) error {
	// Cancel if running
	e.mu.Lock()
	cancel, ok := e.running[taskID]
	if ok {
		cancel()
		delete(e.running, taskID)
	}
	e.mu.Unlock()

	task, err := e.store.Get(ctx, taskID)
	if err != nil {
		return err
	}

	task.Cancel()
	return e.store.Save(ctx, task)
}

// run executes the task through its steps.
func (e *Executor) run(ctx context.Context, task *Task) error {
	// Find starting point (resume from last incomplete step)
	startIdx := 0
	for i, step := range task.Steps {
		if step.Status == StepCompleted || step.Status == StepSkipped {
			startIdx = i + 1
		} else {
			break
		}
	}

	// Mark task as running
	now := time.Now()
	task.Status = StatusRunning
	if task.StartedAt == nil {
		task.StartedAt = &now
	}
	if err := e.store.Save(ctx, task); err != nil {
		return err
	}

	// Execute remaining steps
	for i := startIdx; i < len(task.Steps); i++ {
		step := &task.Steps[i]

		// Check for cancellation
		select {
		case <-ctx.Done():
			task.Pause()
			e.store.Save(context.Background(), task)
			return ctx.Err()
		default:
		}

		// Execute step
		if err := e.executeStep(ctx, task, step); err != nil {
			// Step failed
			task.FailStep(step.Type, err)
			task.Fail(err)
			e.store.Save(context.Background(), task)
			return err
		}
	}

	// All steps completed
	task.Complete(task.ResultID)
	return e.store.Save(ctx, task)
}

// executeStep runs a single step with progress tracking and checkpointing.
func (e *Executor) executeStep(ctx context.Context, task *Task, step *Step) error {
	e.mu.RLock()
	handler, ok := e.handlers[step.Type]
	e.mu.RUnlock()

	if !ok {
		// No handler registered - skip step
		step.Status = StepSkipped
		step.ProgressMsg = "no handler registered"
		return e.store.Save(ctx, task)
	}

	// Validate step can be executed
	if err := handler.Validate(ctx, task); err != nil {
		return fmt.Errorf("validation failed for step %s: %w", step.Type, err)
	}

	// Start the step
	if err := task.StartStep(step.Type); err != nil {
		return err
	}
	if err := e.store.Save(ctx, task); err != nil {
		return err
	}

	// Create progress callback with automatic checkpoint saving
	lastCheckpoint := time.Now()
	progressFn := func(percent int, message string) {
		task.UpdateStepProgress(step.Type, percent, message)

		// Save checkpoint periodically
		if time.Since(lastCheckpoint) >= e.checkpointEvery {
			e.store.Save(context.Background(), task)
			lastCheckpoint = time.Now()
		}
	}

	// Execute the step
	if err := handler.Execute(ctx, task, step, progressFn); err != nil {
		return err
	}

	// Complete the step
	if err := task.CompleteStep(step.Type, step.Output); err != nil {
		return err
	}

	return e.store.Save(ctx, task)
}

// GetTask returns the current state of a task.
func (e *Executor) GetTask(ctx context.Context, taskID string) (*Task, error) {
	return e.store.Get(ctx, taskID)
}

// ListTasks returns tasks matching the filter.
func (e *Executor) ListTasks(ctx context.Context, filter *TaskFilter) ([]*Task, error) {
	return e.store.List(ctx, filter)
}

// IsRunning returns true if the task is currently being executed.
func (e *Executor) IsRunning(taskID string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, ok := e.running[taskID]
	return ok
}

// GetRunningTasks returns IDs of all currently running tasks.
func (e *Executor) GetRunningTasks() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ids := make([]string, 0, len(e.running))
	for id := range e.running {
		ids = append(ids, id)
	}
	return ids
}

// AutoResumeStaleRunning finds tasks that were running when the server stopped
// and attempts to resume them.
func (e *Executor) AutoResumeStaleRunning(ctx context.Context, staleDuration time.Duration) ([]string, error) {
	tasks, err := e.store.ListActive(ctx)
	if err != nil {
		return nil, err
	}

	var resumed []string
	now := time.Now()

	for _, task := range tasks {
		// Skip if already running
		if e.IsRunning(task.ID) {
			continue
		}

		// Check if task appears stale (was running but not updated recently)
		if task.Status == StatusRunning && now.Sub(task.LastUpdatedAt) > staleDuration {
			// Mark as paused first to indicate interrupted state
			task.Status = StatusPaused
			e.store.Save(ctx, task)
		}

		// Try to resume
		if task.CanResume() {
			if err := e.Resume(ctx, task.ID); err != nil {
				fmt.Printf("Warning: Could not auto-resume task %s: %v\n", task.ID, err)
				continue
			}
			resumed = append(resumed, task.ID)
		}
	}

	return resumed, nil
}

// WaitForTask blocks until the task completes or the context is cancelled.
func (e *Executor) WaitForTask(ctx context.Context, taskID string) (*Task, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			task, err := e.store.Get(ctx, taskID)
			if err != nil {
				return nil, err
			}

			if task.IsTerminal() || task.Status == StatusPaused {
				return task, nil
			}
		}
	}
}
