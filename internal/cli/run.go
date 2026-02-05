// Package cli implements the command-line interface for RedisMeter.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tfindelkind-redis/redismeter/internal/engine"
	"github.com/tfindelkind-redis/redismeter/internal/storage"
)

var runCmd = &cobra.Command{
	Use:   "run [workload]",
	Short: "Execute a benchmark workload",
	Long: `Execute a benchmark workload against a Redis target.

Available built-in workloads:
  cache          Balanced GET/SET for caching (80% GET, 20% SET)
  write-heavy    Write-heavy workload (20% GET, 80% SET)
  read-only      Read-only workload (100% GET)
  mixed          Mixed operations with variable data sizes
  high-throughput Pipelined workload for maximum ops/sec
  low-latency    Optimized for minimal response time
  session        Session store simulation
  large-values   Testing with larger value sizes
  huge-read      100% GET with 100KB values

Available built-in run profiles:
  default        Balanced settings (4 threads, 50 clients, 30s)
  quick-test     Short validation test (2 threads, 10 clients, 10s)
  high-load      Maximum parallelism (8 threads, 100 clients, 60s)
  low-latency    Minimal pipelining for latency measurement
  throughput     Aggressive pipelining for max throughput
  stress         Extended stress test (300s duration)
  rate-limited   Controlled request rate (10k req/s)
  request-based  Fixed request count instead of duration

Examples:
  # Run the cache workload against local Redis
  redismeter run cache --target redis://localhost:6379

  # Run with custom duration and threads
  redismeter run cache --target redis://localhost:6379 --duration 60s --threads 4

  # Run with a specific run profile
  redismeter run cache --target redis://localhost:6379 --run-profile high-load

  # Run using a workload definition file
  redismeter run --workload-file ./my-workload.yaml --target redis://localhost:6379

  # Run with authentication
  redismeter run cache --target redis://user:password@localhost:6379`,
	Args: cobra.MaximumNArgs(1),
	RunE: runBenchmark,
}

var (
	targetFlag       string
	durationFlag     string
	threadsFlag      int
	clientsFlag      int
	pipelineFlag     int
	requestsFlag     int64
	rateLimitFlag    int
	runProfileFlag   string
	workloadFileFlag string
	runNameFlag      string
	runTagsFlag      []string
)

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVarP(&targetFlag, "target", "t", "", "Redis target URL (e.g., redis://localhost:6379)")
	runCmd.Flags().StringVarP(&durationFlag, "duration", "d", "", "Benchmark duration (e.g., 30s, 5m)")
	runCmd.Flags().IntVar(&threadsFlag, "threads", 0, "Number of threads")
	runCmd.Flags().IntVar(&clientsFlag, "clients", 0, "Number of clients per thread")
	runCmd.Flags().IntVar(&pipelineFlag, "pipeline", 0, "Pipeline depth")
	runCmd.Flags().Int64VarP(&requestsFlag, "requests", "n", 0, "Number of requests (overrides duration)")
	runCmd.Flags().IntVar(&rateLimitFlag, "rate-limit", 0, "Max requests per second per connection (0=unlimited)")
	runCmd.Flags().StringVar(&runProfileFlag, "run-profile", "", "Run profile to use (default, quick-test, high-load, etc.)")
	runCmd.Flags().StringVarP(&workloadFileFlag, "workload-file", "f", "", "Path to workload definition file")
	runCmd.Flags().StringVar(&runNameFlag, "name", "", "Name for this benchmark run")
	runCmd.Flags().StringSliceVar(&runTagsFlag, "tag", nil, "Tags for this run (can be specified multiple times)")

	runCmd.MarkFlagRequired("target")
}

func runBenchmark(cmd *cobra.Command, args []string) error {
	workloadName := "cache" // default workload
	if len(args) > 0 {
		workloadName = args[0]
	}

	// Get storage configuration
	storageType, storagePath := getStorageConfig()
	engCfg := engine.EngineConfig{
		StorageConfig: storage.StorageConfig{
			Type: storage.StorageType(storageType),
			Path: storagePath,
		},
	}

	// Create engine
	eng, err := engine.NewEngineWithConfig(engCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize engine: %w", err)
	}

	// Add verbose storage info
	if viper.GetBool("verbose") && storageType != "" {
		fmt.Printf("Storage backend: %s\n", storageType)
		if storagePath != "" {
			fmt.Printf("Storage path: %s\n", storagePath)
		}
	}

	// Setup config
	cfg := &engine.RunConfig{
		WorkloadName:   workloadName,
		WorkloadFile:   workloadFileFlag,
		RunProfileName: runProfileFlag,
		TargetURL:      targetFlag,
		Duration:       durationFlag,
		Threads:        threadsFlag,
		Clients:        clientsFlag,
		Pipeline:       pipelineFlag,
		Requests:       requestsFlag,
		RateLimit:      rateLimitFlag,
		Name:           runNameFlag,
		Tags:           runTagsFlag,
	}

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n⚠️  Interrupted, stopping benchmark...")
		cancel()
	}()

	// Progress callback
	progress := func(msg string) {
		fmt.Printf("  %s\n", msg)
	}

	fmt.Println()
	fmt.Println("🚀 RedisMeter Benchmark")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Run benchmark
	run, err := eng.Run(ctx, cfg, progress)
	if err != nil {
		fmt.Println()
		fmt.Printf("❌ Benchmark failed: %v\n", err)
		return err
	}

	// Print results
	fmt.Println()
	fmt.Println("📊 Results")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	if run.Results != nil && run.Results.Summary != nil {
		s := run.Results.Summary
		fmt.Printf("  Throughput:     %.2f ops/sec\n", s.OpsPerSecond)
		fmt.Printf("  Avg Latency:    %.3f ms\n", s.AvgLatencyMs)
		fmt.Printf("  P50 Latency:    %.3f ms\n", s.P50LatencyMs)
		fmt.Printf("  P99 Latency:    %.3f ms\n", s.P99LatencyMs)
		fmt.Printf("  P99.9 Latency:  %.3f ms\n", s.P999LatencyMs)
		
		if len(run.Results.ByOperation) > 0 {
			fmt.Println()
			fmt.Println("  By Operation:")
			for op, metrics := range run.Results.ByOperation {
				fmt.Printf("    %s: %.2f ops/sec, %.3f ms avg\n", op, metrics.OpsPerSecond, metrics.AvgLatencyMs)
			}
		}
	}

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("✅ Run ID: %s\n", run.ID)
	fmt.Printf("   Duration: %s\n", run.Duration)
	fmt.Println()
	fmt.Println("View details with: redismeter show", run.ID)

	return nil
}
