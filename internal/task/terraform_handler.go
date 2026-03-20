package task

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/infraprofile"
	"github.com/tfindelkind-redis/redismeter/internal/terraform"
)

// DeployPhase represents a phase in the deployment process.
type DeployPhase string

const (
	PhaseNotStarted   DeployPhase = "not_started"
	PhaseInitializing DeployPhase = "initializing"
	PhaseApplying     DeployPhase = "applying"
	PhaseWaitingReady DeployPhase = "waiting_ready"
	PhaseReady        DeployPhase = "ready"
	PhaseDestroying   DeployPhase = "destroying"
	PhaseDestroyed    DeployPhase = "destroyed"
	PhaseFailed       DeployPhase = "failed"
)

// DeploymentCheckpoint stores the state needed to resume a deployment.
type DeploymentCheckpoint struct {
	InfraID       string                 `json:"infra_id"`        // Terraform infrastructure ID
	Phase         DeployPhase            `json:"phase"`           // Current deployment phase
	StartedAt     time.Time              `json:"started_at"`      // When deployment started
	LastUpdatedAt time.Time              `json:"last_updated_at"` // Last checkpoint update
	TerraformDir  string                 `json:"terraform_dir"`   // Terraform workspace directory
	Outputs       map[string]interface{} `json:"outputs,omitempty"`
	Error         string                 `json:"error,omitempty"`
}

// TerraformDeployHandler deploys infrastructure using Terraform.
// This handler properly integrates with the existing terraform.Manager
// and supports checkpointing for resume after interruption.
type TerraformDeployHandler struct {
	BaseHandler
	manager      *terraform.Manager
	profileStore infraprofile.Store
}

// NewTerraformDeployHandler creates a handler with the Terraform manager.
func NewTerraformDeployHandler(manager *terraform.Manager, profileStore infraprofile.Store) *TerraformDeployHandler {
	return &TerraformDeployHandler{
		BaseHandler:  BaseHandler{name: "terraform_deploy"},
		manager:      manager,
		profileStore: profileStore,
	}
}

// NewTerraformDeployHandlerDefault creates a handler with default Terraform manager.
func NewTerraformDeployHandlerDefault() (*TerraformDeployHandler, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	tfDir := filepath.Join(homeDir, ".redismeter", "terraform")
	manager, err := terraform.NewManager(tfDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create terraform manager: %w", err)
	}

	profileDir := filepath.Join(homeDir, ".redismeter", "profiles")
	profileStore, err := infraprofile.NewFileStore(profileDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create profile store: %w", err)
	}

	return &TerraformDeployHandler{
		BaseHandler:  BaseHandler{name: "terraform_deploy"},
		manager:      manager,
		profileStore: profileStore,
	}, nil
}

func (h *TerraformDeployHandler) Validate(ctx context.Context, task *Task) error {
	if !task.DeployInfra {
		return fmt.Errorf("deploy_infra is not enabled for this task")
	}
	if task.CloudProvider == "" {
		return fmt.Errorf("cloud_provider is required for infrastructure deployment")
	}
	if task.InfraProfileID == "" {
		return fmt.Errorf("infra_profile_id is required for infrastructure deployment")
	}
	if h.manager == nil {
		return fmt.Errorf("terraform manager not initialized")
	}
	return nil
}

func (h *TerraformDeployHandler) Execute(ctx context.Context, task *Task, step *Step, progress ProgressFunc) error {
	// Check for existing checkpoint (resume from previous deployment)
	checkpoint := h.loadCheckpoint(step)

	if checkpoint != nil && checkpoint.InfraID != "" {
		return h.resumeDeployment(ctx, task, step, checkpoint, progress)
	}

	return h.startNewDeployment(ctx, task, step, progress)
}

// startNewDeployment initiates a new infrastructure deployment.
func (h *TerraformDeployHandler) startNewDeployment(ctx context.Context, task *Task, step *Step, progress ProgressFunc) error {
	progress(5, "Loading infrastructure profile...")

	// Load the infrastructure profile
	profile, err := h.profileStore.Get(ctx, task.InfraProfileID)
	if err != nil {
		return fmt.Errorf("failed to load infra profile %s: %w", task.InfraProfileID, err)
	}

	// Convert profile to Terraform config
	tfConfig, err := h.profileToTerraformConfig(profile, task)
	if err != nil {
		return fmt.Errorf("failed to convert profile to terraform config: %w", err)
	}

	progress(10, "Initializing Terraform deployment...")

	// Create checkpoint before starting
	checkpoint := &DeploymentCheckpoint{
		Phase:         PhaseInitializing,
		StartedAt:     time.Now().UTC(),
		LastUpdatedAt: time.Now().UTC(),
	}
	h.saveCheckpoint(step, checkpoint)

	// Start provisioning with progress callback
	var lastProgress int
	state, err := h.manager.ProvisionWithProgress(ctx, *tfConfig, func(event terraform.TerraformEvent) {
		// Map Terraform events to task progress
		pct, msg := h.mapTerraformProgress(event, &lastProgress)
		if pct > 0 {
			progress(pct, msg)
		}

		// Update checkpoint with current state
		checkpoint.Phase = h.mapPhase(event.Phase)
		checkpoint.LastUpdatedAt = time.Now().UTC()
		h.saveCheckpoint(step, checkpoint)
	})

	if err != nil {
		// Check if it's a context cancellation (pause/cancel)
		if ctx.Err() != nil {
			// Save checkpoint for resume
			if state != nil {
				checkpoint.InfraID = state.ID
				checkpoint.TerraformDir = state.WorkspacePath
			}
			checkpoint.Phase = PhaseApplying // Will resume apply
			h.saveCheckpoint(step, checkpoint)
			return ctx.Err()
		}

		checkpoint.Phase = PhaseFailed
		checkpoint.Error = err.Error()
		h.saveCheckpoint(step, checkpoint)
		return fmt.Errorf("terraform provisioning failed: %w", err)
	}

	// Deployment completed successfully
	checkpoint.InfraID = state.ID
	checkpoint.TerraformDir = state.WorkspacePath
	checkpoint.Phase = PhaseReady
	checkpoint.LastUpdatedAt = time.Now().UTC()

	if state.Outputs != nil {
		checkpoint.Outputs = map[string]interface{}{
			"redis_hostname":     state.Outputs.RedisHostname,
			"redis_port":         state.Outputs.RedisPort,
			"resource_group":     state.Outputs.ResourceGroupName,
			"runner_ips":         state.Outputs.RunnerIPs,
			"runner_private_ips": state.Outputs.RunnerPrivateIPs,
		}
	}
	h.saveCheckpoint(step, checkpoint)

	// Update task with deployment info
	task.DeploymentID = state.ID
	if state.Outputs != nil && state.Outputs.RedisHostname != "" {
		port := state.Outputs.RedisPort
		if port == 0 {
			port = 6380 // Default Azure Redis SSL port
		}
		task.TargetURL = fmt.Sprintf("rediss://%s:%d", state.Outputs.RedisHostname, port)
	}

	progress(100, fmt.Sprintf("Infrastructure deployed: %s", state.ID))
	return nil
}

// resumeDeployment continues a previously started deployment.
func (h *TerraformDeployHandler) resumeDeployment(ctx context.Context, task *Task, step *Step, checkpoint *DeploymentCheckpoint, progress ProgressFunc) error {
	progress(10, fmt.Sprintf("Resuming deployment: %s (phase: %s)", checkpoint.InfraID, checkpoint.Phase))

	// Get current state from Terraform
	state, err := h.manager.GetState(checkpoint.InfraID)
	if err != nil {
		// State not found - deployment may have been interrupted before state was saved
		return fmt.Errorf("failed to get terraform state for %s: %w", checkpoint.InfraID, err)
	}

	switch state.Status {
	case "ready":
		// Already complete
		progress(100, "Infrastructure already ready")
		task.DeploymentID = state.ID
		if state.Outputs != nil && state.Outputs.RedisHostname != "" {
			port := state.Outputs.RedisPort
			if port == 0 {
				port = 6380
			}
			task.TargetURL = fmt.Sprintf("rediss://%s:%d", state.Outputs.RedisHostname, port)
		}
		return nil

	case "failed":
		return fmt.Errorf("previous deployment failed: %s", state.Error)

	case "provisioning", "pending":
		// Continue the apply
		progress(20, "Continuing Terraform apply...")

		// Terraform apply is idempotent - running it again will continue from where it left off
		var lastProgress int
		state, err = h.manager.ProvisionWithProgress(ctx, state.Config, func(event terraform.TerraformEvent) {
			pct, msg := h.mapTerraformProgress(event, &lastProgress)
			if pct > 0 {
				progress(pct, msg)
			}
		})

		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("terraform apply failed: %w", err)
		}

		// Update checkpoint and task
		checkpoint.Phase = PhaseReady
		checkpoint.LastUpdatedAt = time.Now().UTC()
		h.saveCheckpoint(step, checkpoint)

		task.DeploymentID = state.ID
		if state.Outputs != nil && state.Outputs.RedisHostname != "" {
			port := state.Outputs.RedisPort
			if port == 0 {
				port = 6380
			}
			task.TargetURL = fmt.Sprintf("rediss://%s:%d", state.Outputs.RedisHostname, port)
		}

		progress(100, "Infrastructure deployment resumed and completed")
		return nil

	default:
		return fmt.Errorf("unexpected terraform state status: %s", state.Status)
	}
}

// profileToTerraformConfig converts an infraprofile.Profile to terraform.InfraConfig.
func (h *TerraformDeployHandler) profileToTerraformConfig(profile *infraprofile.Profile, task *Task) (*terraform.InfraConfig, error) {
	config := &terraform.InfraConfig{
		Name:     fmt.Sprintf("%s-%s", task.Name, task.ID),
		Provider: string(profile.Provider),
		Tags: map[string]string{
			"redismeter_task": task.ID,
			"profile":         profile.ID,
			"created_by":      "redismeter",
		},
	}

	switch profile.Provider {
	case infraprofile.ProviderAzure:
		if profile.Config.Azure == nil {
			return nil, fmt.Errorf("azure profile missing azure config")
		}
		azConfig := profile.Config.Azure
		config.Region = azConfig.Location

		// Map Azure Managed Redis config
		config.AMR = &terraform.AMRConfig{
			SKU:              azConfig.SKU,
			Modules:          azConfig.Modules,
			HighAvailability: azConfig.HighAvailability,
			ClusteringPolicy: azConfig.ClusteringPolicy,
			EvictionPolicy:   azConfig.EvictionPolicy,
		}

		// Map runner VM config if present
		if azConfig.VMCount > 0 {
			config.Runners = &terraform.RunnerConfig{
				Count:        azConfig.VMCount,
				InstanceType: azConfig.VMSize,
				SSHPublicKey: azConfig.SSHKeyPath,
				SSHUser:      "azureuser",
			}
		}

	case infraprofile.ProviderAWS:
		if profile.Config.AWS == nil {
			return nil, fmt.Errorf("aws profile missing aws config")
		}
		awsConfig := profile.Config.AWS
		config.Region = awsConfig.Region
		// TODO: Map AWS ElastiCache config

	default:
		return nil, fmt.Errorf("unsupported provider for terraform deployment: %s", profile.Provider)
	}

	return config, nil
}

// mapTerraformProgress converts Terraform events to progress percentages.
func (h *TerraformDeployHandler) mapTerraformProgress(event terraform.TerraformEvent, lastProgress *int) (int, string) {
	var pct int
	var msg string

	switch event.Phase {
	case "init":
		pct = 15
		msg = fmt.Sprintf("Terraform init: %s", event.Message)
	case "plan":
		pct = 25
		msg = fmt.Sprintf("Terraform plan: %s", event.Message)
	case "apply":
		// Scale apply from 30-95%
		if event.Total > 0 {
			applyPct := (event.Completed * 65) / event.Total
			pct = 30 + applyPct
		} else {
			pct = 50
		}
		if event.Resource != "" {
			msg = fmt.Sprintf("Creating %s", event.Resource)
		} else {
			msg = event.Message
		}
	default:
		return 0, ""
	}

	// Only report if progress increased
	if pct > *lastProgress {
		*lastProgress = pct
		return pct, msg
	}
	return 0, ""
}

// mapPhase converts Terraform phase strings to DeployPhase.
func (h *TerraformDeployHandler) mapPhase(phase string) DeployPhase {
	switch phase {
	case "init":
		return PhaseInitializing
	case "plan", "apply":
		return PhaseApplying
	case "destroy":
		return PhaseDestroying
	default:
		return PhaseApplying
	}
}

// loadCheckpoint loads the deployment checkpoint from the step.
func (h *TerraformDeployHandler) loadCheckpoint(step *Step) *DeploymentCheckpoint {
	if step.CheckpointData == nil {
		return nil
	}

	// Convert map to struct
	data, err := json.Marshal(step.CheckpointData)
	if err != nil {
		return nil
	}

	var checkpoint DeploymentCheckpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil
	}

	return &checkpoint
}

// saveCheckpoint saves the deployment checkpoint to the step.
func (h *TerraformDeployHandler) saveCheckpoint(step *Step, checkpoint *DeploymentCheckpoint) {
	// Convert struct to map
	data, err := json.Marshal(checkpoint)
	if err != nil {
		return
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return
	}

	step.CheckpointData = m
	step.Checkpoint = checkpoint.InfraID // Simple string checkpoint for quick resume check
}

// TerraformWaitHandler waits for infrastructure to be ready.
// For most Terraform deployments, the infrastructure is ready immediately after apply.
// However, some resources (like Azure Managed Redis) may need additional time.
type TerraformWaitHandler struct {
	BaseHandler
	manager      *terraform.Manager
	pollInterval time.Duration
	maxWait      time.Duration
}

// NewTerraformWaitHandler creates a handler that waits for infrastructure readiness.
func NewTerraformWaitHandler(manager *terraform.Manager) *TerraformWaitHandler {
	return &TerraformWaitHandler{
		BaseHandler:  BaseHandler{name: "terraform_wait"},
		manager:      manager,
		pollInterval: 30 * time.Second,
		maxWait:      45 * time.Minute, // Azure Managed Redis can take 30+ minutes
	}
}

func (h *TerraformWaitHandler) Validate(ctx context.Context, task *Task) error {
	if task.DeploymentID == "" {
		return fmt.Errorf("deployment_id is required - infrastructure must be deployed first")
	}
	return nil
}

func (h *TerraformWaitHandler) Execute(ctx context.Context, task *Task, step *Step, progress ProgressFunc) error {
	// Check if already ready
	state, err := h.manager.GetState(task.DeploymentID)
	if err != nil {
		return fmt.Errorf("failed to get deployment state: %w", err)
	}

	if state.Status == "ready" {
		progress(100, "Infrastructure is ready")
		return nil
	}

	if state.Status == "failed" {
		return fmt.Errorf("infrastructure deployment failed: %s", state.Error)
	}

	// Load checkpoint for poll count
	pollCount := 0
	if step.CheckpointData != nil {
		if count, ok := step.CheckpointData["poll_count"].(float64); ok {
			pollCount = int(count)
		}
	}

	progress(10, fmt.Sprintf("Waiting for infrastructure to be ready (deployment: %s)...", task.DeploymentID))

	startTime := time.Now()
	maxPolls := int(h.maxWait / h.pollInterval)

	for pollCount < maxPolls {
		select {
		case <-ctx.Done():
			// Save checkpoint before returning
			step.CheckpointData = map[string]interface{}{
				"poll_count":    pollCount,
				"deployment_id": task.DeploymentID,
				"last_check":    time.Now().UTC(),
			}
			return ctx.Err()
		case <-time.After(h.pollInterval):
		}

		pollCount++

		// Calculate progress: reserve 10-95% for waiting
		waitProgress := 10 + (85 * pollCount / maxPolls)
		if waitProgress > 95 {
			waitProgress = 95
		}

		// Refresh state from Terraform
		state, err = h.manager.RefreshState(ctx, task.DeploymentID)
		if err != nil {
			progress(waitProgress, fmt.Sprintf("Warning: failed to refresh state: %v", err))
			continue
		}

		// Save checkpoint
		step.CheckpointData = map[string]interface{}{
			"poll_count":    pollCount,
			"deployment_id": task.DeploymentID,
			"last_check":    time.Now().UTC(),
			"status":        state.Status,
		}

		progress(waitProgress, fmt.Sprintf("Checking infrastructure status (attempt %d/%d): %s", pollCount, maxPolls, state.Status))

		if state.Status == "ready" {
			// Update task with outputs
			if state.Outputs != nil && state.Outputs.RedisHostname != "" {
				port := state.Outputs.RedisPort
				if port == 0 {
					port = 6380
				}
				task.TargetURL = fmt.Sprintf("rediss://%s:%d", state.Outputs.RedisHostname, port)
			}
			break
		}

		if state.Status == "failed" {
			return fmt.Errorf("infrastructure deployment failed: %s", state.Error)
		}
	}

	elapsed := time.Since(startTime)
	if pollCount >= maxPolls && state.Status != "ready" {
		return fmt.Errorf("infrastructure deployment timed out after %v", elapsed)
	}

	progress(100, fmt.Sprintf("Infrastructure ready after %v", elapsed.Truncate(time.Second)))
	return nil
}

// TerraformDestroyHandler destroys infrastructure using Terraform.
type TerraformDestroyHandler struct {
	BaseHandler
	manager *terraform.Manager
}

// NewTerraformDestroyHandler creates a handler that destroys infrastructure.
func NewTerraformDestroyHandler(manager *terraform.Manager) *TerraformDestroyHandler {
	return &TerraformDestroyHandler{
		BaseHandler: BaseHandler{name: "terraform_destroy"},
		manager:     manager,
	}
}

func (h *TerraformDestroyHandler) Validate(ctx context.Context, task *Task) error {
	// No deployment to destroy is valid - skip silently
	return nil
}

func (h *TerraformDestroyHandler) Execute(ctx context.Context, task *Task, step *Step, progress ProgressFunc) error {
	if task.DeploymentID == "" {
		progress(100, "No infrastructure to destroy")
		return nil
	}

	if !task.DestroyInfra {
		progress(100, "Infrastructure destruction skipped (destroy_infra=false)")
		return nil
	}

	// Check current state
	state, err := h.manager.GetState(task.DeploymentID)
	if err != nil {
		// State not found - might already be destroyed
		progress(100, "Infrastructure state not found - may already be destroyed")
		return nil
	}

	if state.Status == "destroyed" {
		progress(100, "Infrastructure already destroyed")
		return nil
	}

	progress(10, fmt.Sprintf("Destroying infrastructure: %s", task.DeploymentID))

	// Destroy with progress callback
	var lastProgress int
	err = h.manager.DestroyWithProgress(ctx, task.DeploymentID, func(event terraform.TerraformEvent) {
		// Scale destroy from 10-95%
		var pct int
		if event.Total > 0 {
			destroyPct := (event.Completed * 85) / event.Total
			pct = 10 + destroyPct
		} else {
			pct = 50
		}

		if pct > lastProgress {
			lastProgress = pct
			msg := event.Message
			if event.Resource != "" {
				msg = fmt.Sprintf("Destroying %s", event.Resource)
			}
			progress(pct, msg)
		}
	})

	if err != nil {
		if ctx.Err() != nil {
			// Save checkpoint - destroy can be resumed
			step.CheckpointData = map[string]interface{}{
				"deployment_id": task.DeploymentID,
				"phase":         "destroying",
			}
			return ctx.Err()
		}
		return fmt.Errorf("terraform destroy failed: %w", err)
	}

	progress(100, "Infrastructure destroyed")
	return nil
}
