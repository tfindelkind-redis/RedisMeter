// Package cli implements the command-line interface for RedisMeter.
package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tfindelkind-redis/redismeter/internal/engine"
	"github.com/tfindelkind-redis/redismeter/internal/storage"
)

var showCmd = &cobra.Command{
	Use:   "show <run-id>",
	Short: "Show details of a benchmark run",
	Long: `Display detailed information about a specific benchmark run.

Examples:
  redismeter show 20240101-120000-abc123
  redismeter show 20240101-120000-abc123 --json`,
	Args: cobra.ExactArgs(1),
	RunE: showRun,
}

var jsonOutputFlag bool

func init() {
	rootCmd.AddCommand(showCmd)
	showCmd.Flags().BoolVar(&jsonOutputFlag, "json", false, "Output in JSON format")
}

func showRun(cmd *cobra.Command, args []string) error {
	runID := args[0]

	storageType, storagePath := getStorageConfig()
	engCfg := engine.EngineConfig{
		StorageConfig: storage.StorageConfig{
			Type: storage.StorageType(storageType),
			Path: storagePath,
		},
	}

	eng, err := engine.NewEngineWithConfig(engCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize engine: %w", err)
	}

	run, err := eng.GetRun(context.Background(), runID)
	if err != nil {
		return fmt.Errorf("run not found: %w", err)
	}

	if jsonOutputFlag {
		data, err := json.MarshalIndent(run, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	// Pretty print
	fmt.Println()
	fmt.Println("📊 Benchmark Run Details")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// Basic info
	fmt.Printf("  ID:          %s\n", run.ID)
	fmt.Printf("  Status:      %s %s\n", statusIcon(string(run.Status)), run.Status)
	fmt.Printf("  Created:     %s\n", run.CreatedAt.Format("2006-01-02 15:04:05"))
	if run.Name != "" {
		fmt.Printf("  Name:        %s\n", run.Name)
	}
	if len(run.Tags) > 0 {
		fmt.Printf("  Tags:        %v\n", run.Tags)
	}

	// Target
	fmt.Println()
	fmt.Println("  Target")
	fmt.Println("  ──────")
	if run.Target != nil {
		fmt.Printf("    Host:      %s:%d\n", run.Target.Host, run.Target.Port)
		if run.Target.Cluster {
			fmt.Printf("    Mode:      Cluster\n")
		}
		if run.Target.TLS != nil && run.Target.TLS.Enabled {
			fmt.Printf("    TLS:       Enabled\n")
		}
	}

	// Workload
	fmt.Println()
	fmt.Println("  Workload")
	fmt.Println("  ────────")
	if run.Workload != nil {
		fmt.Printf("    Name:      %s\n", run.Workload.Name)
		fmt.Printf("    Type:      %s\n", run.Workload.Type)
		fmt.Printf("    Duration:  %s\n", run.Workload.Duration)
		fmt.Printf("    Threads:   %d\n", run.Workload.Threads)
		fmt.Printf("    Clients:   %d\n", run.Workload.Clients)
		fmt.Printf("    Pipeline:  %d\n", run.Workload.Pipeline)
	}

	// Environment
	fmt.Println()
	fmt.Println("  Environment")
	fmt.Println("  ───────────")
	if run.Environment != nil {
		fmt.Printf("    Fingerprint:  %s\n", run.Environment.Fingerprint)
		if run.Environment.Host != nil {
			fmt.Printf("    OS:           %s/%s\n", run.Environment.Host.OS, run.Environment.Host.Arch)
			fmt.Printf("    CPU:          %s (%d cores)\n", run.Environment.Host.CPUModel, run.Environment.Host.CPUs)
			fmt.Printf("    Memory:       %.1f GB\n", run.Environment.Host.MemoryGB)
		}
		if run.Environment.Redis != nil && run.Environment.Redis.Version != "" {
			fmt.Printf("    Redis:        %s\n", run.Environment.Redis.Version)
			if run.Environment.Redis.Mode != "" {
				fmt.Printf("    Redis Mode:   %s\n", run.Environment.Redis.Mode)
			}
			if run.Environment.Redis.MemoryUsed > 0 {
				fmt.Printf("    Redis Memory: %.2f MB used", float64(run.Environment.Redis.MemoryUsed)/(1024*1024))
				if run.Environment.Redis.MemoryMax > 0 {
					fmt.Printf(" / %.2f MB max", float64(run.Environment.Redis.MemoryMax)/(1024*1024))
				}
				fmt.Println()
			}
			if run.Environment.Redis.ConnectedClients > 0 {
				fmt.Printf("    Redis Clients: %d connected\n", run.Environment.Redis.ConnectedClients)
			}
			if run.Environment.Redis.TotalKeys > 0 {
				fmt.Printf("    Redis Keys:   %d total\n", run.Environment.Redis.TotalKeys)
			}
			if run.Environment.Redis.EvictedKeys > 0 {
				fmt.Printf("    Evicted Keys: %d\n", run.Environment.Redis.EvictedKeys)
			}
		}
		if run.Environment.NetworkLatencyMs > 0 {
			fmt.Printf("    Net Latency:  %.2f ms\n", run.Environment.NetworkLatencyMs)
		}
	}

	// Results
	if run.Results != nil && run.Results.Summary != nil {
		fmt.Println()
		fmt.Println("  Results")
		fmt.Println("  ───────")
		s := run.Results.Summary
		fmt.Printf("    Throughput:      %.2f ops/sec\n", s.OpsPerSecond)
		if s.BytesPerSecond > 0 {
			fmt.Printf("    Bandwidth:       %.2f MB/sec\n", s.BytesPerSecond/1024/1024)
		}
		fmt.Println()
		fmt.Println("    Latency:")
		fmt.Printf("      Average:       %.3f ms\n", s.AvgLatencyMs)
		fmt.Printf("      P50:           %.3f ms\n", s.P50LatencyMs)
		fmt.Printf("      P90:           %.3f ms\n", s.P90LatencyMs)
		fmt.Printf("      P95:           %.3f ms\n", s.P95LatencyMs)
		fmt.Printf("      P99:           %.3f ms\n", s.P99LatencyMs)
		fmt.Printf("      P99.9:         %.3f ms\n", s.P999LatencyMs)

		if len(run.Results.ByOperation) > 0 {
			fmt.Println()
			fmt.Println("    By Operation:")
			for op, m := range run.Results.ByOperation {
				fmt.Printf("      %s:\n", op)
				fmt.Printf("        Ops/sec:   %.2f\n", m.OpsPerSecond)
				fmt.Printf("        Avg:       %.3f ms\n", m.AvgLatencyMs)
				fmt.Printf("        P99:       %.3f ms\n", m.P99LatencyMs)
			}
		}
	}

	// Error
	if run.Error != "" {
		fmt.Println()
		fmt.Println("  Error")
		fmt.Println("  ─────")
		fmt.Printf("    %s\n", run.Error)
	}

	// Timing
	fmt.Println()
	fmt.Println("  Timing")
	fmt.Println("  ──────")
	fmt.Printf("    Started:   %s\n", run.StartTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("    Ended:     %s\n", run.EndTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("    Duration:  %s\n", run.Duration)

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	return nil
}
