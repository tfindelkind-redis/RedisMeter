package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tfindelkind-redis/redismeter/internal/cloud"
	"github.com/tfindelkind-redis/redismeter/internal/domain"
	"github.com/tfindelkind-redis/redismeter/internal/workload"
)

// cloudRunCmd runs a benchmark on cloud infrastructure.
var cloudRunCmd = &cobra.Command{
	Use:   "run <workload>",
	Short: "Run a benchmark on cloud infrastructure",
	Long: `Run a benchmark on cloud infrastructure.

This command provisions cloud infrastructure, runs the benchmark,
collects results, and optionally tears down the infrastructure.

The workload can be a built-in workload name or a path to a 
workload YAML file.`,
	Example: `  # Run cache workload on 2 AWS instances
  redismeter cloud run cache --provider aws --count 2 --target redis.example.com:6379

  # Run custom workload with spot instances
  redismeter cloud run my-workload.yaml --provider aws --count 4 --spot

  # Run and keep infrastructure for further testing
  redismeter cloud run cache --provider aws --count 2 --target redis:6379 --keep

  # Run on existing infrastructure
  redismeter cloud run cache --infra infra-abc123`,
	Args: cobra.ExactArgs(1),
	RunE: runCloudRun,
}

func init() {
	cloudCmd.AddCommand(cloudRunCmd)

	// Infrastructure flags
	cloudRunCmd.Flags().String("provider", "aws", "Cloud provider (aws, gcp, azure)")
	cloudRunCmd.Flags().String("region", "", "Cloud region")
	cloudRunCmd.Flags().String("instance-type", "c5.xlarge", "Instance type for load generators")
	cloudRunCmd.Flags().Int("count", 1, "Number of load generator instances")
	cloudRunCmd.Flags().Bool("spot", false, "Use spot/preemptible instances")
	cloudRunCmd.Flags().String("key-name", "", "SSH key name")
	cloudRunCmd.Flags().String("infra", "", "Use existing infrastructure ID")
	cloudRunCmd.Flags().Bool("keep", false, "Keep infrastructure after benchmark")

	// Target flags
	cloudRunCmd.Flags().StringP("target", "t", "", "Redis target (host:port)")
	cloudRunCmd.Flags().StringP("password", "a", "", "Redis password")

	// Benchmark flags
	cloudRunCmd.Flags().IntP("clients", "c", 50, "Number of clients per instance")
	cloudRunCmd.Flags().IntP("threads", "T", 4, "Number of threads per instance")
	cloudRunCmd.Flags().Int("requests", 0, "Total requests (0 = use duration)")
	cloudRunCmd.Flags().String("duration", "60", "Test duration in seconds")
	cloudRunCmd.Flags().Int("pipeline", 1, "Pipeline depth")

	// Output flags
	cloudRunCmd.Flags().Bool("json", false, "Output as JSON")
	cloudRunCmd.Flags().StringP("output", "o", "", "Output file for results")
}

func runCloudRun(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	workloadName := args[0]

	// Get flags
	provider, _ := cmd.Flags().GetString("provider")
	region, _ := cmd.Flags().GetString("region")
	instanceType, _ := cmd.Flags().GetString("instance-type")
	count, _ := cmd.Flags().GetInt("count")
	spot, _ := cmd.Flags().GetBool("spot")
	keyName, _ := cmd.Flags().GetString("key-name")
	existingInfra, _ := cmd.Flags().GetString("infra")
	keepInfra, _ := cmd.Flags().GetBool("keep")
	targetStr, _ := cmd.Flags().GetString("target")
	password, _ := cmd.Flags().GetString("password")
	clients, _ := cmd.Flags().GetInt("clients")
	threads, _ := cmd.Flags().GetInt("threads")
	requests, _ := cmd.Flags().GetInt("requests")
	duration, _ := cmd.Flags().GetString("duration")
	pipeline, _ := cmd.Flags().GetInt("pipeline")
	jsonOutput, _ := cmd.Flags().GetBool("json")
	outputFile, _ := cmd.Flags().GetString("output")

	if targetStr == "" && existingInfra == "" {
		return fmt.Errorf("target is required (use --target host:port)")
	}

	// Parse target
	target := &domain.Target{
		Host: "localhost",
		Port: 6379,
	}
	if targetStr != "" {
		var host string
		var port int
		if n, _ := fmt.Sscanf(targetStr, "%[^:]:%d", &host, &port); n >= 1 {
			target.Host = host
			if n == 2 {
				target.Port = port
			}
		}
		target.Password = password
	}

	// Load workload
	workload, err := loadWorkload(workloadName)
	if err != nil {
		return fmt.Errorf("failed to load workload: %w", err)
	}

	// Override workload settings
	if clients > 0 {
		workload.Clients = clients
	}
	if threads > 0 {
		workload.Threads = threads
	}
	if requests > 0 {
		workload.Requests = int64(requests)
		workload.Duration = ""
	} else if duration != "" {
		workload.Duration = duration
		workload.Requests = 0
	}
	if pipeline > 0 {
		workload.Pipeline = pipeline
	}

	// Get or provision infrastructure
	var infra *cloud.Infrastructure
	var p cloud.ProviderPlugin

	if existingInfra != "" {
		// Use existing infrastructure
		for _, provider := range cloudProviders {
			if i, err := provider.GetInfrastructure(ctx, existingInfra); err == nil {
				infra = i
				p = provider
				break
			}
		}
		if infra == nil {
			return fmt.Errorf("infrastructure not found: %s", existingInfra)
		}
		if !jsonOutput {
			fmt.Printf("Using existing infrastructure: %s\n", infra.ID)
		}
	} else {
		// Provision new infrastructure
		var err error
		p, err = getCloudProvider(ctx, provider)
		if err != nil {
			return err
		}

		if region == "" {
			region = "us-east-1"
		}

		spec := &cloud.InfraSpec{
			Name:     fmt.Sprintf("redismeter-run-%s", time.Now().Format("20060102-150405")),
			Provider: provider,
			Region:   region,
			LoadGenerator: &cloud.NodeGroupSpec{
				InstanceType:  instanceType,
				Count:         count,
				SpotInstances: spot,
				SSHKeyName:    keyName,
			},
			RedisTarget: &cloud.RedisTargetSpec{
				Type:             cloud.RedisTargetExisting,
				ExistingEndpoint: targetStr,
			},
			TTL: 2 * time.Hour, // Auto-cleanup after 2 hours
		}

		if !jsonOutput {
			fmt.Printf("Provisioning %d %s instance(s) in %s %s...\n", count, instanceType, provider, region)
		}

		infra, err = p.Provision(ctx, spec)
		if err != nil {
			return fmt.Errorf("failed to provision infrastructure: %w", err)
		}

		if !jsonOutput {
			fmt.Printf("✓ Infrastructure ready: %s\n\n", infra.ID)
		}
	}

	// Ensure cleanup unless keeping
	if !keepInfra && existingInfra == "" {
		defer func() {
			if !jsonOutput {
				fmt.Printf("\nTearing down infrastructure...\n")
			}
			if err := p.Teardown(ctx, infra); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to teardown infrastructure: %v\n", err)
				fmt.Fprintf(os.Stderr, "Run 'redismeter cloud teardown %s' to clean up manually.\n", infra.ID)
			} else if !jsonOutput {
				fmt.Printf("✓ Infrastructure torn down\n")
			}
		}()
	}

	// Create SSH executor
	sshExecutor := cloud.NewSSHExecutor()

	// Get SSH config from flags/config
	sshConfig := map[string]interface{}{
		"user": "ubuntu",
		"port": 22,
	}
	if keyPath := viper.GetString("ssh.private_key_path"); keyPath != "" {
		sshConfig["private_key_path"] = keyPath
	}

	if err := sshExecutor.Initialize(ctx, sshConfig); err != nil {
		return fmt.Errorf("failed to initialize SSH executor: %w", err)
	}

	// Wait for SSH to be available on all instances
	if !jsonOutput {
		fmt.Printf("Waiting for instances to be ready...\n")
	}

	for _, inst := range infra.LoadGenerators {
		ip := inst.PublicIP
		if ip == "" {
			ip = inst.PrivateIP
		}
		if ip == "" {
			continue
		}

		// Try to connect with retries
		for i := 0; i < 30; i++ {
			if err := sshExecutor.Connect(ctx, ip); err == nil {
				break
			}
			time.Sleep(10 * time.Second)
		}
	}

	// Verify memtier is available on instances
	if !jsonOutput {
		fmt.Printf("Checking memtier_benchmark availability...\n")
	}

	for _, inst := range infra.LoadGenerators {
		ip := inst.PublicIP
		if ip == "" {
			ip = inst.PrivateIP
		}
		if ip == "" {
			continue
		}

		output, err := sshExecutor.RunCommand(ctx, ip, "which memtier_benchmark || echo 'not found'")
		if err != nil || output == "not found\n" {
			if !jsonOutput {
				fmt.Printf("Installing memtier_benchmark on %s...\n", ip)
			}
			// Install memtier
			installCmd := `sudo apt-get update && sudo apt-get install -y build-essential autoconf automake libpcre3-dev libevent-dev pkg-config zlib1g-dev libssl-dev git && 
				git clone https://github.com/RedisLabs/memtier_benchmark.git /tmp/memtier && 
				cd /tmp/memtier && autoreconf -ivf && ./configure && make -j$(nproc) && sudo make install`
			if _, err := sshExecutor.RunCommand(ctx, ip, installCmd); err != nil {
				return fmt.Errorf("failed to install memtier on %s: %w", ip, err)
			}
		}
	}

	// Run benchmark on all instances
	if !jsonOutput {
		fmt.Printf("\nRunning benchmark on %d instance(s)...\n", len(infra.LoadGenerators))
	}

	// Collect host IPs
	var hosts []string
	for _, inst := range infra.LoadGenerators {
		ip := inst.PublicIP
		if ip == "" {
			ip = inst.PrivateIP
		}
		if ip != "" {
			hosts = append(hosts, ip)
		}
	}

	if len(hosts) == 0 {
		return fmt.Errorf("no instances available to run benchmark")
	}

	// Create multi-node executor
	multiNode := cloud.NewMultiNodeExecutor(sshExecutor)

	// Start execution on all hosts
	execID, err := multiNode.ExecuteOnHosts(ctx, hosts, workload, target)
	if err != nil {
		return fmt.Errorf("failed to start benchmark: %w", err)
	}

	// Wait for completion
	if !jsonOutput {
		fmt.Printf("Benchmark running (execution ID: %s)...\n", execID)
	}

	if err := multiNode.WaitForCompletion(ctx, execID); err != nil {
		return fmt.Errorf("benchmark execution failed: %w", err)
	}

	// Collect results
	results, err := multiNode.GetAggregatedResults(execID)
	if err != nil {
		return fmt.Errorf("failed to get results: %w", err)
	}

	// Create result structure
	runResult := &CloudRunResult{
		InfraID:     infra.ID,
		Provider:    provider,
		Region:      region,
		InstanceType: instanceType,
		InstanceCount: len(hosts),
		Workload:    workloadName,
		Target:      targetStr,
		StartTime:   time.Now().Add(-time.Minute), // Approximate
		EndTime:     time.Now(),
		HostResults: results,
	}

	// Output results
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(runResult)
	}

	// Pretty print results
	fmt.Printf("\n✓ Benchmark complete!\n\n")
	fmt.Printf("Infrastructure: %s\n", infra.ID)
	fmt.Printf("Instances:      %d x %s\n", len(hosts), instanceType)
	fmt.Printf("Duration:       %s\n", runResult.EndTime.Sub(runResult.StartTime).Round(time.Second))

	fmt.Printf("\nResults from each host:\n")
	for host, output := range results {
		fmt.Printf("\n--- %s ---\n", host)
		// Print last 20 lines of output (summary)
		lines := splitLines(output)
		start := len(lines) - 20
		if start < 0 {
			start = 0
		}
		for _, line := range lines[start:] {
			fmt.Println(line)
		}
	}

	// Save to file if requested
	if outputFile != "" {
		f, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer f.Close()

		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		if err := enc.Encode(runResult); err != nil {
			return fmt.Errorf("failed to write results: %w", err)
		}
		fmt.Printf("\nResults saved to: %s\n", outputFile)
	}

	if keepInfra {
		fmt.Printf("\nInfrastructure kept: %s\n", infra.ID)
		fmt.Printf("Remember to tear down when done:\n  redismeter cloud teardown %s\n", infra.ID)
	}

	return nil
}

// CloudRunResult contains the results of a cloud benchmark run.
type CloudRunResult struct {
	InfraID       string            `json:"infra_id"`
	Provider      string            `json:"provider"`
	Region        string            `json:"region"`
	InstanceType  string            `json:"instance_type"`
	InstanceCount int               `json:"instance_count"`
	Workload      string            `json:"workload"`
	Target        string            `json:"target"`
	StartTime     time.Time         `json:"start_time"`
	EndTime       time.Time         `json:"end_time"`
	HostResults   map[string]string `json:"host_results"`
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// loadWorkload loads a workload by name or from file.
func loadWorkload(nameOrPath string) (*domain.Workload, error) {
	// Check if it's a file path
	if strings.HasSuffix(nameOrPath, ".yaml") || strings.HasSuffix(nameOrPath, ".yml") {
		if _, err := os.Stat(nameOrPath); err == nil {
			return workload.LoadFromFile(nameOrPath)
		}
	}

	// Try to load from registry
	return workload.DefaultRegistry.Get(nameOrPath)
}
