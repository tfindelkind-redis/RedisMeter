// Package memtier provides integration with memtier_benchmark.
// This file adds logging integration to the executor.
package memtier

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/logging"
)

// LoggingExecutor wraps Executor with logging capabilities.
type LoggingExecutor struct {
	*Executor
	logger logging.Logger
}

// NewLoggingExecutor creates a new executor with logging.
func NewLoggingExecutor(logger logging.Logger) *LoggingExecutor {
	return &LoggingExecutor{
		Executor: NewExecutor(),
		logger:   logger,
	}
}

// Run executes memtier_benchmark with the given configuration and logs all activity.
func (e *LoggingExecutor) Run(ctx context.Context, config *Config, benchmarkID string, progressFn func(line string)) (*ExecutionResult, error) {
	// Use logger with benchmark context if available
	logger := e.logger
	if benchmarkID != "" {
		logger = e.logger.WithBenchmark(benchmarkID)
	}

	// Build command for logging
	builder := NewCommandBuilder(config)
	args := builder.Build()
	cmdLine := fmt.Sprintf("%s %s", e.binaryPath, strings.Join(args, " "))

	// Create command log entry
	cmdLog := &logging.CommandLog{
		Command:   e.binaryPath,
		Args:      args,
		StartTime: time.Now(),
	}

	// Log command start
	logger.Info("memtier:run", "Starting memtier_benchmark", map[string]interface{}{
		"command":     cmdLine,
		"host":        config.Host,
		"port":        config.Port,
		"threads":     config.Threads,
		"clients":     config.Clients,
		"duration":    config.Duration.String(),
		"requests":    config.Requests,
		"ratio":       config.Ratio,
		"key_pattern": config.KeyPattern,
		"pipeline":    config.Pipeline,
		"tls":         config.TLS,
		"cluster":     config.Cluster,
		"custom_cmds": len(config.CustomCommands),
	})

	// Log debug config details
	if configJSON, err := json.MarshalIndent(config, "", "  "); err == nil {
		logger.Debug("memtier:config", "Full configuration", map[string]interface{}{
			"config": string(configJSON),
		})
	}

	// Wrapper for progress that also logs
	wrappedProgress := func(line string) {
		// Only log significant lines (not every progress tick)
		if strings.Contains(line, "Ops/sec") ||
			strings.Contains(line, "Error") ||
			strings.Contains(line, "error") ||
			strings.Contains(line, "CLUSTER") ||
			strings.Contains(line, "connected") {
			logger.Debug("memtier:output", line, nil)
		}
		if progressFn != nil {
			progressFn(line)
		}
	}

	// Run the actual benchmark
	result, err := e.Executor.Run(ctx, config, wrappedProgress)

	// Complete command log
	cmdLog.EndTime = time.Now()
	cmdLog.Duration = cmdLog.EndTime.Sub(cmdLog.StartTime)

	if result != nil {
		cmdLog.ExitCode = result.ExitCode
		cmdLog.Output = result.RawText
		if result.Error != nil {
			cmdLog.Error = result.Error.Error()
		}
	}

	// Log completion
	if err != nil {
		logger.Error("memtier:run", fmt.Sprintf("memtier_benchmark failed: %v", err), map[string]interface{}{
			"command":   cmdLine,
			"duration":  cmdLog.Duration.String(),
			"exit_code": cmdLog.ExitCode,
			"error":     cmdLog.Error,
		})
	} else if result.ExitCode != 0 {
		logger.Warn("memtier:run", fmt.Sprintf("memtier_benchmark exited with code %d", result.ExitCode), map[string]interface{}{
			"command":   cmdLine,
			"duration":  cmdLog.Duration.String(),
			"exit_code": result.ExitCode,
			"output":    truncate(result.RawText, 1000),
		})
	} else {
		// Parse results for logging summary
		var summary map[string]interface{}
		if len(result.RawJSON) > 0 {
			json.Unmarshal(result.RawJSON, &summary)
		}

		logger.Info("memtier:run", "memtier_benchmark completed successfully", map[string]interface{}{
			"command":   cmdLine,
			"duration":  cmdLog.Duration.String(),
			"json_size": len(result.RawJSON),
		})
	}

	return result, err
}

// CheckAvailable verifies memtier_benchmark is installed and logs the result.
func (e *LoggingExecutor) CheckAvailable() error {
	err := e.Executor.CheckAvailable()
	if err != nil {
		e.logger.Error("memtier:check", "memtier_benchmark not available", map[string]interface{}{
			"error": err.Error(),
		})
	} else {
		version, _ := e.Executor.GetVersion()
		e.logger.Info("memtier:check", "memtier_benchmark is available", map[string]interface{}{
			"version": version,
			"path":    e.binaryPath,
		})
	}
	return err
}

// SetBinaryPath sets a custom path and logs it.
func (e *LoggingExecutor) SetBinaryPath(path string) {
	e.Executor.SetBinaryPath(path)
	e.logger.Debug("memtier:config", "Binary path set", map[string]interface{}{
		"path": path,
	})
}

// Helper to truncate long strings for logging.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ConfigDiff compares two configs and returns the differences.
func ConfigDiff(a, b *Config) map[string]interface{} {
	diff := make(map[string]interface{})

	if a.Host != b.Host {
		diff["host"] = []string{a.Host, b.Host}
	}
	if a.Port != b.Port {
		diff["port"] = []int{a.Port, b.Port}
	}
	if a.Threads != b.Threads {
		diff["threads"] = []int{a.Threads, b.Threads}
	}
	if a.Clients != b.Clients {
		diff["clients"] = []int{a.Clients, b.Clients}
	}
	if a.Duration != b.Duration {
		diff["duration"] = []string{a.Duration.String(), b.Duration.String()}
	}
	if a.Requests != b.Requests {
		diff["requests"] = []int64{a.Requests, b.Requests}
	}
	if a.Ratio != b.Ratio {
		diff["ratio"] = []string{a.Ratio, b.Ratio}
	}
	if a.KeyPattern != b.KeyPattern {
		diff["key_pattern"] = []string{a.KeyPattern, b.KeyPattern}
	}
	if a.Pipeline != b.Pipeline {
		diff["pipeline"] = []int{a.Pipeline, b.Pipeline}
	}
	if a.TLS != b.TLS {
		diff["tls"] = []bool{a.TLS, b.TLS}
	}
	if a.Cluster != b.Cluster {
		diff["cluster"] = []bool{a.Cluster, b.Cluster}
	}

	return diff
}

// LogConfigChange logs differences between two configs.
func LogConfigChange(logger logging.Logger, oldConfig, newConfig *Config, reason string) {
	diff := ConfigDiff(oldConfig, newConfig)
	if len(diff) > 0 {
		logger.Info("memtier:config", reason, map[string]interface{}{
			"changes": diff,
		})
	}
}
