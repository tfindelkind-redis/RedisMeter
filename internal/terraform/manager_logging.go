// Package terraform provides infrastructure management using Terraform.
// This file adds logging integration to the Terraform manager.
package terraform

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/logging"
)

// LoggingManager wraps Manager with logging capabilities.
type LoggingManager struct {
	*Manager
	logger logging.Logger
}

// NewLoggingManager creates a new Terraform manager with logging.
func NewLoggingManager(baseDir string, logger logging.Logger) (*LoggingManager, error) {
	manager, err := NewManager(baseDir)
	if err != nil {
		logger.Error("terraform:init", fmt.Sprintf("Failed to create Terraform manager: %v", err), map[string]interface{}{
			"base_dir": baseDir,
			"error":    err.Error(),
		})
		return nil, err
	}

	logger.Info("terraform:init", "Terraform manager initialized", map[string]interface{}{
		"base_dir":      baseDir,
		"terraform_bin": manager.terraformBin,
	})

	return &LoggingManager{
		Manager: manager,
		logger:  logger,
	}, nil
}

// ProvisionWithProgress creates new infrastructure with logging.
func (m *LoggingManager) ProvisionWithProgress(ctx context.Context, config InfraConfig, progress ProgressCallback) (*InfraState, error) {
	// Get or create infra-scoped logger
	logger := m.logger

	// Log the provisioning start
	configJSON, _ := json.MarshalIndent(config, "", "  ")
	logger.Info("terraform:provision", "Starting infrastructure provisioning", map[string]interface{}{
		"name":     config.Name,
		"provider": config.Provider,
		"region":   config.Region,
		"config":   string(configJSON),
	})

	startTime := time.Now()

	// Create a wrapped progress callback that logs events
	loggingProgress := func(event TerraformEvent) {
		// Log each phase/action
		level := logging.LevelInfo
		if event.Action == "failed" || event.Action == "error" {
			level = logging.LevelError
		}

		logger.Log(level, logging.SourceTerraform, fmt.Sprintf("terraform:%s", event.Phase), event.Message, map[string]interface{}{
			"phase":     event.Phase,
			"action":    event.Action,
			"resource":  event.Resource,
			"completed": event.Completed,
			"total":     event.Total,
			"elapsed":   event.ElapsedTime,
		})

		// Call original progress callback if provided
		if progress != nil {
			progress(event)
		}
	}

	// Run the actual provisioning
	state, err := m.Manager.ProvisionWithProgress(ctx, config, loggingProgress)

	duration := time.Since(startTime)

	// Log result
	if err != nil {
		logger.Error("terraform:provision", fmt.Sprintf("Infrastructure provisioning failed: %v", err), map[string]interface{}{
			"name":     config.Name,
			"provider": config.Provider,
			"region":   config.Region,
			"duration": duration.String(),
			"error":    err.Error(),
		})
	} else {
		outputSummary := map[string]interface{}{}
		if state.Outputs != nil {
			outputSummary = map[string]interface{}{
				"redis_hostname": state.Outputs.RedisHostname,
				"redis_port":     state.Outputs.RedisPort,
				"runner_count":   len(state.Outputs.RunnerIPs),
			}
		}

		logger.Info("terraform:provision", "Infrastructure provisioning completed", map[string]interface{}{
			"id":        state.ID,
			"name":      state.Name,
			"status":    state.Status,
			"provider":  state.Provider,
			"region":    state.Region,
			"duration":  duration.String(),
			"outputs":   outputSummary,
			"workspace": state.WorkspacePath,
		})
	}

	// Store infra ID in logger for subsequent operations
	if state != nil {
		logger = logger.WithInfra(state.ID)
	}

	return state, err
}

// Provision creates new infrastructure with logging (wrapper).
func (m *LoggingManager) Provision(ctx context.Context, config InfraConfig) (*InfraState, error) {
	return m.ProvisionWithProgress(ctx, config, nil)
}

// DestroyWithProgress tears down infrastructure with logging.
func (m *LoggingManager) DestroyWithProgress(ctx context.Context, id string, progress ProgressCallback) error {
	logger := m.logger.WithInfra(id)

	// Get current state for logging
	state, _ := m.Manager.GetState(id)

	logger.Info("terraform:destroy", "Starting infrastructure destruction", map[string]interface{}{
		"infra_id": id,
		"name":     state.Name,
		"provider": state.Provider,
		"region":   state.Region,
		"status":   state.Status,
	})

	startTime := time.Now()

	// Create wrapped progress callback
	loggingProgress := func(event TerraformEvent) {
		logger.Log(logging.LevelInfo, logging.SourceTerraform, fmt.Sprintf("terraform:%s", event.Phase), event.Message, map[string]interface{}{
			"phase":     event.Phase,
			"action":    event.Action,
			"resource":  event.Resource,
			"completed": event.Completed,
			"total":     event.Total,
		})

		if progress != nil {
			progress(event)
		}
	}

	// Run destruction
	err := m.Manager.DestroyWithProgress(ctx, id, loggingProgress)

	duration := time.Since(startTime)

	if err != nil {
		logger.Error("terraform:destroy", fmt.Sprintf("Infrastructure destruction failed: %v", err), map[string]interface{}{
			"infra_id": id,
			"duration": duration.String(),
			"error":    err.Error(),
		})
	} else {
		logger.Info("terraform:destroy", "Infrastructure destroyed successfully", map[string]interface{}{
			"infra_id": id,
			"duration": duration.String(),
		})
	}

	return err
}

// Destroy tears down infrastructure with logging (wrapper).
func (m *LoggingManager) Destroy(ctx context.Context, id string) error {
	return m.DestroyWithProgress(ctx, id, nil)
}

// RefreshState updates state from Terraform with logging.
func (m *LoggingManager) RefreshState(ctx context.Context, id string) (*InfraState, error) {
	logger := m.logger.WithInfra(id)

	logger.Debug("terraform:refresh", "Refreshing infrastructure state", map[string]interface{}{
		"infra_id": id,
	})

	state, err := m.Manager.RefreshState(ctx, id)

	if err != nil {
		logger.Warn("terraform:refresh", fmt.Sprintf("Failed to refresh state: %v", err), map[string]interface{}{
			"infra_id": id,
			"error":    err.Error(),
		})
	} else {
		logger.Debug("terraform:refresh", "State refreshed", map[string]interface{}{
			"infra_id": id,
			"status":   state.Status,
		})
	}

	return state, err
}

// GetState returns state with logging.
func (m *LoggingManager) GetState(id string) (*InfraState, error) {
	state, err := m.Manager.GetState(id)
	if err != nil {
		m.logger.Debug("terraform:state", fmt.Sprintf("Infrastructure not found: %s", id), map[string]interface{}{
			"infra_id": id,
			"error":    err.Error(),
		})
	}
	return state, err
}

// ListInfrastructure returns all tracked infrastructure with logging.
func (m *LoggingManager) ListInfrastructure() ([]*InfraState, error) {
	states, err := m.Manager.ListInfrastructure()
	if err != nil {
		m.logger.Warn("terraform:list", fmt.Sprintf("Failed to list infrastructure: %v", err), map[string]interface{}{
			"error": err.Error(),
		})
		return nil, err
	}

	m.logger.Debug("terraform:list", fmt.Sprintf("Found %d infrastructure(s)", len(states)), map[string]interface{}{
		"count": len(states),
	})

	return states, nil
}

// CleanupExpired removes expired infrastructure with logging.
func (m *LoggingManager) CleanupExpired(ctx context.Context) (int, error) {
	m.logger.Info("terraform:cleanup", "Starting expired infrastructure cleanup", nil)

	states, err := m.ListInfrastructure()
	if err != nil {
		return 0, err
	}

	cleaned := 0
	now := time.Now()

	for _, state := range states {
		if !state.ExpiresAt.IsZero() && state.ExpiresAt.Before(now) && state.Status == "ready" {
			m.logger.Info("terraform:cleanup", fmt.Sprintf("Destroying expired infrastructure: %s", state.ID), map[string]interface{}{
				"infra_id":   state.ID,
				"name":       state.Name,
				"expired_at": state.ExpiresAt.Format(time.RFC3339),
			})

			if err := m.Destroy(ctx, state.ID); err != nil {
				m.logger.Error("terraform:cleanup", fmt.Sprintf("Failed to destroy expired infrastructure: %v", err), map[string]interface{}{
					"infra_id": state.ID,
					"error":    err.Error(),
				})
				continue
			}
			cleaned++
		}
	}

	m.logger.Info("terraform:cleanup", fmt.Sprintf("Cleanup completed: %d infrastructure(s) destroyed", cleaned), map[string]interface{}{
		"cleaned": cleaned,
	})

	return cleaned, nil
}
