// Package task provides persistent state management for long-running benchmark tasks.
// It implements a state machine with checkpointing to ensure tasks can survive
// connection drops, server restarts, and resume from the last successful step.
package task

import (
	"encoding/json"
	"fmt"
	"time"
)

// TaskStatus represents the overall status of a task.
type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"   // Task created but not started
	StatusRunning   TaskStatus = "running"   // Task is actively executing
	StatusPaused    TaskStatus = "paused"    // Task paused (can be resumed)
	StatusCompleted TaskStatus = "completed" // Task finished successfully
	StatusFailed    TaskStatus = "failed"    // Task failed (may be retryable)
	StatusCancelled TaskStatus = "cancelled" // Task was cancelled by user
	StatusTimedOut  TaskStatus = "timed_out" // Task exceeded timeout
)

// StepStatus represents the status of an individual step.
type StepStatus string

const (
	StepPending   StepStatus = "pending"   // Step not yet started
	StepRunning   StepStatus = "running"   // Step currently executing
	StepCompleted StepStatus = "completed" // Step finished successfully
	StepFailed    StepStatus = "failed"    // Step failed
	StepSkipped   StepStatus = "skipped"   // Step was skipped
	StepRetrying  StepStatus = "retrying"  // Step is being retried
)

// StepType identifies the type of step in a benchmark workflow.
type StepType string

const (
	// Infrastructure deployment steps (cloud/managed)
	StepDeployInfra      StepType = "deploy_infra"       // Deploy cloud infrastructure (Azure AMR, VMs, etc.)
	StepWaitInfraReady   StepType = "wait_infra_ready"   // Wait for infrastructure to be fully ready
	StepValidateInfra    StepType = "validate_infra"     // Check infrastructure is reachable
	StepPrepareRunner    StepType = "prepare_runner"     // Install dependencies on runner (memtier, Docker, etc.)

	// Database steps
	StepPrepareDatabase StepType = "prepare_database" // Clear/configure database
	StepUploadDataset   StepType = "upload_dataset"   // Load data (vector datasets can be huge)
	StepBuildIndex      StepType = "build_index"      // Create indexes (can take hours)

	// Benchmark steps
	StepWarmup       StepType = "warmup"        // Optional warmup phase
	StepRunBenchmark StepType = "run_benchmark" // Execute the actual benchmark

	// Collection steps
	StepCollectResults StepType = "collect_results" // Gather metrics and results
	StepCollectLogs    StepType = "collect_logs"    // Gather logs from runner

	// Cleanup steps
	StepCleanup        StepType = "cleanup"         // Optional cleanup (data, indexes)
	StepDestroyInfra   StepType = "destroy_infra"   // Destroy cloud infrastructure (optional)
)

// ToolType identifies the benchmark tool being used.
type ToolType string

const (
	ToolMemtier  ToolType = "memtier_benchmark"
	ToolVectorDB ToolType = "vector_db_benchmark"
	ToolFTSB     ToolType = "ftsb"
)

// Task represents a benchmark task with its current state.
type Task struct {
	// Identity
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	// Tool configuration
	Tool       ToolType `json:"tool"`
	ToolConfig string   `json:"tool_config"` // JSON blob of tool-specific config

	// References
	WorkloadID     string `json:"workload_id,omitempty"`
	RunProfileID   string `json:"run_profile_id,omitempty"`
	InfraProfileID string `json:"infra_profile_id,omitempty"`
	TargetURL      string `json:"target_url,omitempty"`

	// Infrastructure deployment (for cloud runs)
	DeployInfra   bool   `json:"deploy_infra,omitempty"`    // Whether to deploy infrastructure
	DestroyInfra  bool   `json:"destroy_infra,omitempty"`   // Whether to destroy after completion
	DeploymentID  string `json:"deployment_id,omitempty"`   // Cloud deployment ID (for tracking/resume)
	CloudProvider string `json:"cloud_provider,omitempty"`  // e.g., "azure", "aws"

	// Status tracking
	Status      TaskStatus `json:"status"`
	CurrentStep StepType   `json:"current_step,omitempty"`
	Progress    int        `json:"progress"` // 0-100 overall progress

	// Steps
	Steps []Step `json:"steps"`

	// Timing
	CreatedAt     time.Time  `json:"created_at"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	LastUpdatedAt time.Time  `json:"last_updated_at"`

	// Timeout configuration
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`

	// Error handling
	Error      string `json:"error,omitempty"`
	RetryCount int    `json:"retry_count"`
	MaxRetries int    `json:"max_retries"`

	// Results
	ResultID string `json:"result_id,omitempty"` // Reference to BenchmarkRun

	// Metadata
	Tags   []string          `json:"tags,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

// Step represents a single step in the task workflow.
type Step struct {
	Type   StepType   `json:"type"`
	Status StepStatus `json:"status"`

	// Timing
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Duration    string     `json:"duration,omitempty"`

	// Progress within step (for long-running steps like upload_dataset)
	Progress    int    `json:"progress"` // 0-100
	ProgressMsg string `json:"progress_msg,omitempty"`

	// Error handling
	Error      string `json:"error,omitempty"`
	RetryCount int    `json:"retry_count"`

	// Checkpoint data - step-specific data needed for recovery
	Checkpoint     string                 `json:"checkpoint,omitempty"`      // Simple checkpoint string (e.g., deployment ID)
	CheckpointData map[string]interface{} `json:"checkpoint_data,omitempty"` // Structured checkpoint data

	// Output data from this step
	Output string `json:"output,omitempty"` // JSON blob
}

// Checkpoint holds recovery data for a specific step.
type Checkpoint struct {
	// Common fields
	Position      int64  `json:"position,omitempty"`       // Progress position (bytes, records, etc.)
	Total         int64  `json:"total,omitempty"`          // Total items
	LastProcessed string `json:"last_processed,omitempty"` // Last item ID/key

	// Step-specific data
	Data map[string]interface{} `json:"data,omitempty"`

	// Timestamp
	SavedAt time.Time `json:"saved_at"`
}

// NewTask creates a new task with the given configuration.
func NewTask(id, name string, tool ToolType) *Task {
	now := time.Now()
	return &Task{
		ID:            id,
		Name:          name,
		Tool:          tool,
		Status:        StatusPending,
		Progress:      0,
		Steps:         []Step{},
		CreatedAt:     now,
		LastUpdatedAt: now,
		RetryCount:    0,
		MaxRetries:    3,
		Labels:        make(map[string]string),
	}
}

// AddStep adds a step to the task workflow.
func (t *Task) AddStep(stepType StepType) {
	t.Steps = append(t.Steps, Step{
		Type:     stepType,
		Status:   StepPending,
		Progress: 0,
	})
}

// AddSteps adds multiple steps to the task workflow.
func (t *Task) AddSteps(steps []StepType) {
	for _, s := range steps {
		t.AddStep(s)
	}
}

// GetStep returns a step by type, or nil if not found.
func (t *Task) GetStep(stepType StepType) *Step {
	for i := range t.Steps {
		if t.Steps[i].Type == stepType {
			return &t.Steps[i]
		}
	}
	return nil
}

// GetCurrentStepIndex returns the index of the current step, or -1.
func (t *Task) GetCurrentStepIndex() int {
	for i, step := range t.Steps {
		if step.Status == StepRunning || step.Status == StepPending {
			return i
		}
	}
	return -1
}

// GetLastCompletedStepIndex returns the index of the last completed step.
func (t *Task) GetLastCompletedStepIndex() int {
	last := -1
	for i, step := range t.Steps {
		if step.Status == StepCompleted {
			last = i
		}
	}
	return last
}

// CanResume returns true if the task can be resumed.
func (t *Task) CanResume() bool {
	switch t.Status {
	case StatusRunning, StatusPaused, StatusFailed:
		return true
	default:
		return false
	}
}

// IsTerminal returns true if the task is in a terminal state.
func (t *Task) IsTerminal() bool {
	switch t.Status {
	case StatusCompleted, StatusCancelled, StatusTimedOut:
		return true
	case StatusFailed:
		return t.RetryCount >= t.MaxRetries
	default:
		return false
	}
}

// CalculateProgress calculates overall progress based on completed steps.
func (t *Task) CalculateProgress() int {
	if len(t.Steps) == 0 {
		return 0
	}

	totalWeight := 0
	completedWeight := 0

	// Weight each step - some steps are heavier than others
	// Infrastructure deployment can take 10-30+ minutes
	weights := map[StepType]int{
		StepDeployInfra:      20, // Can take 10-30+ minutes for Azure AMR
		StepWaitInfraReady:   10, // Waiting for provisioning
		StepValidateInfra:    3,
		StepPrepareRunner:    5,
		StepPrepareDatabase:  5,
		StepUploadDataset:    15, // Large vector datasets
		StepBuildIndex:       15, // Can take hours for large indexes
		StepWarmup:           5,
		StepRunBenchmark:     15,
		StepCollectResults:   3,
		StepCollectLogs:      2,
		StepCleanup:          1,
		StepDestroyInfra:     1,
	}

	for _, step := range t.Steps {
		w := weights[step.Type]
		if w == 0 {
			w = 10 // Default weight
		}
		totalWeight += w

		if step.Status == StepCompleted {
			completedWeight += w
		} else if step.Status == StepRunning {
			// Add partial progress for running step
			completedWeight += (w * step.Progress) / 100
		}
	}

	if totalWeight == 0 {
		return 0
	}

	return (completedWeight * 100) / totalWeight
}

// SetStepCheckpoint saves checkpoint data for a step.
func (t *Task) SetStepCheckpoint(stepType StepType, checkpoint *Checkpoint) error {
	step := t.GetStep(stepType)
	if step == nil {
		return fmt.Errorf("step %s not found", stepType)
	}

	checkpoint.SavedAt = time.Now()
	data, err := json.Marshal(checkpoint)
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint: %w", err)
	}

	step.Checkpoint = string(data)
	t.LastUpdatedAt = time.Now()
	return nil
}

// GetStepCheckpoint retrieves checkpoint data for a step.
func (t *Task) GetStepCheckpoint(stepType StepType) (*Checkpoint, error) {
	step := t.GetStep(stepType)
	if step == nil {
		return nil, fmt.Errorf("step %s not found", stepType)
	}

	if step.Checkpoint == "" {
		return nil, nil
	}

	var checkpoint Checkpoint
	if err := json.Unmarshal([]byte(step.Checkpoint), &checkpoint); err != nil {
		return nil, fmt.Errorf("failed to unmarshal checkpoint: %w", err)
	}

	return &checkpoint, nil
}

// StartStep marks a step as running.
func (t *Task) StartStep(stepType StepType) error {
	step := t.GetStep(stepType)
	if step == nil {
		return fmt.Errorf("step %s not found", stepType)
	}

	now := time.Now()
	step.Status = StepRunning
	step.StartedAt = &now
	step.Progress = 0
	t.CurrentStep = stepType
	t.LastUpdatedAt = now

	if t.Status == StatusPending {
		t.Status = StatusRunning
		t.StartedAt = &now
	}

	return nil
}

// CompleteStep marks a step as completed.
func (t *Task) CompleteStep(stepType StepType, output string) error {
	step := t.GetStep(stepType)
	if step == nil {
		return fmt.Errorf("step %s not found", stepType)
	}

	now := time.Now()
	step.Status = StepCompleted
	step.CompletedAt = &now
	step.Progress = 100
	step.Output = output

	if step.StartedAt != nil {
		step.Duration = now.Sub(*step.StartedAt).String()
	}

	t.Progress = t.CalculateProgress()
	t.LastUpdatedAt = now

	return nil
}

// FailStep marks a step as failed.
func (t *Task) FailStep(stepType StepType, err error) error {
	step := t.GetStep(stepType)
	if step == nil {
		return fmt.Errorf("step %s not found", stepType)
	}

	now := time.Now()
	step.Status = StepFailed
	step.CompletedAt = &now
	step.Error = err.Error()
	step.RetryCount++

	if step.StartedAt != nil {
		step.Duration = now.Sub(*step.StartedAt).String()
	}

	t.LastUpdatedAt = now

	return nil
}

// UpdateStepProgress updates the progress of a running step.
func (t *Task) UpdateStepProgress(stepType StepType, progress int, message string) error {
	step := t.GetStep(stepType)
	if step == nil {
		return fmt.Errorf("step %s not found", stepType)
	}

	step.Progress = progress
	step.ProgressMsg = message
	t.Progress = t.CalculateProgress()
	t.LastUpdatedAt = time.Now()

	return nil
}

// Complete marks the task as completed.
func (t *Task) Complete(resultID string) {
	now := time.Now()
	t.Status = StatusCompleted
	t.CompletedAt = &now
	t.Progress = 100
	t.ResultID = resultID
	t.LastUpdatedAt = now
}

// Fail marks the task as failed.
func (t *Task) Fail(err error) {
	now := time.Now()
	t.Status = StatusFailed
	t.CompletedAt = &now
	t.Error = err.Error()
	t.RetryCount++
	t.LastUpdatedAt = now
}

// Cancel marks the task as cancelled.
func (t *Task) Cancel() {
	now := time.Now()
	t.Status = StatusCancelled
	t.CompletedAt = &now
	t.LastUpdatedAt = now
}

// Pause marks the task as paused.
func (t *Task) Pause() {
	t.Status = StatusPaused
	t.LastUpdatedAt = time.Now()
}

// Resume marks the task as running again.
func (t *Task) Resume() {
	t.Status = StatusRunning
	t.LastUpdatedAt = time.Now()
}

// StandardMemtierSteps returns the standard steps for memtier_benchmark.
// Use WithInfraDeployment() for cloud deployments.
func StandardMemtierSteps() []StepType {
	return []StepType{
		StepValidateInfra,
		StepPrepareRunner,
		StepPrepareDatabase,
		StepRunBenchmark,
		StepCollectResults,
		StepCollectLogs,
	}
}

// StandardVectorDBSteps returns the standard steps for vector_db_benchmark.
func StandardVectorDBSteps() []StepType {
	return []StepType{
		StepValidateInfra,
		StepPrepareRunner,
		StepPrepareDatabase,
		StepUploadDataset,
		StepBuildIndex,
		StepRunBenchmark,
		StepCollectResults,
		StepCollectLogs,
		StepCleanup,
	}
}

// StandardFTSBSteps returns the standard steps for ftsb.
func StandardFTSBSteps() []StepType {
	return []StepType{
		StepValidateInfra,
		StepPrepareRunner,
		StepPrepareDatabase,
		StepUploadDataset,
		StepBuildIndex,
		StepWarmup,
		StepRunBenchmark,
		StepCollectResults,
		StepCollectLogs,
	}
}

// WithInfraDeployment prepends infrastructure deployment steps to any step list.
// Use this when running benchmarks on cloud infrastructure that needs to be provisioned.
func WithInfraDeployment(steps []StepType, destroyAfter bool) []StepType {
	deploySteps := []StepType{
		StepDeployInfra,
		StepWaitInfraReady,
	}
	result := append(deploySteps, steps...)
	if destroyAfter {
		result = append(result, StepDestroyInfra)
	}
	return result
}

// GetStandardSteps returns the standard steps for the given tool.
// For cloud deployments, wrap with WithInfraDeployment().
func GetStandardSteps(tool ToolType) []StepType {
	switch tool {
	case ToolMemtier:
		return StandardMemtierSteps()
	case ToolVectorDB:
		return StandardVectorDBSteps()
	case ToolFTSB:
		return StandardFTSBSteps()
	default:
		return StandardMemtierSteps()
	}
}
