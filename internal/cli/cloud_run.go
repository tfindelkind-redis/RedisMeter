package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	osExec "os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tfindelkind-redis/redismeter/internal/cloud"
	"github.com/tfindelkind-redis/redismeter/internal/domain"
	"github.com/tfindelkind-redis/redismeter/internal/memtier"
	"github.com/tfindelkind-redis/redismeter/internal/terraform"
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

	// Azure Managed Redis (AMR) provisioning flags
	cloudRunCmd.Flags().Bool("provision-amr", false, "Provision Azure Managed Redis (AMR) as target")
	cloudRunCmd.Flags().String("amr-sku", "Balanced_B0", "AMR SKU (Balanced_B0, Balanced_B1, Balanced_B3, Balanced_B5, etc.)")
	cloudRunCmd.Flags().String("amr-clustering", "OSSCluster", "AMR clustering policy (OSSCluster or EnterpriseCluster)")
	cloudRunCmd.Flags().Bool("amr-ha", false, "Enable high availability for AMR")
	cloudRunCmd.Flags().String("resource-group", "", "Azure resource group (created if not exists)")

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

	// AMR provisioning flags
	provisionAMR, _ := cmd.Flags().GetBool("provision-amr")
	amrSKU, _ := cmd.Flags().GetString("amr-sku")
	amrClustering, _ := cmd.Flags().GetString("amr-clustering")
	amrHA, _ := cmd.Flags().GetBool("amr-ha")
	resourceGroup, _ := cmd.Flags().GetString("resource-group")

	// Check if using existing Terraform-managed infrastructure
	if existingInfra != "" {
		return runWithTerraformInfra(ctx, cmd, workloadName, existingInfra)
	}

	// Validate flags
	if !provisionAMR && targetStr == "" && existingInfra == "" {
		return fmt.Errorf("either --target, --provision-amr, or --infra is required")
	}

	if provisionAMR && provider != "azure" {
		return fmt.Errorf("--provision-amr requires --provider azure")
	}

	// Parse target (will be overwritten if provisioning AMR)
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
	var amrDeployer *cloud.AMRBicepDeployer
	var amrResult *cloud.AMRBicepDeploymentResult

	// Provision Azure Managed Redis if requested
	if provisionAMR {
		if region == "" {
			region = "westus3"
		}

		// Get subscription ID
		subscriptionID, err := getAzureSubscriptionID()
		if err != nil {
			return fmt.Errorf("failed to get Azure subscription: %w", err)
		}

		// Create or use resource group
		if resourceGroup == "" {
			resourceGroup = fmt.Sprintf("rg-redismeter-%s", time.Now().Format("20060102-150405"))
		}

		if !jsonOutput {
			fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
			fmt.Println("║          RedisMeter Azure Cloud Benchmark                        ║")
			fmt.Println("╠══════════════════════════════════════════════════════════════════╣")
			fmt.Printf("║  Region: %-55s║\n", region)
			fmt.Printf("║  Resource Group: %-47s║\n", resourceGroup)
			fmt.Printf("║  AMR SKU: %-54s║\n", amrSKU)
			fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
			fmt.Println()
		}

		// Create resource group first
		if !jsonOutput {
			fmt.Println("📦 Step 1: Creating resource group...")
		}
		if err := createAzureResourceGroup(subscriptionID, resourceGroup, region); err != nil {
			return fmt.Errorf("failed to create resource group: %w", err)
		}
		if !jsonOutput {
			fmt.Printf("   ✅ Resource group '%s' ready\n\n", resourceGroup)
		}

		// Deploy AMR using Bicep
		if !jsonOutput {
			fmt.Println("🚀 Step 2: Deploying Azure Managed Redis (this takes 10-20 minutes)...")
		}

		amrDeployer, err = cloud.NewAMRBicepDeployer(subscriptionID)
		if err != nil {
			return fmt.Errorf("failed to create AMR deployer: %w", err)
		}

		redisName := fmt.Sprintf("rm-%d", time.Now().Unix())
		amrParams := &cloud.AMRBicepDeploymentParams{
			RedisName:        redisName,
			Location:         region,
			SKUName:          amrSKU,
			ClusteringPolicy: amrClustering,
			HighAvailability: amrHA,
			EvictionPolicy:   "VolatileLRU",
			Tags: map[string]string{
				"environment": "benchmark",
				"managed-by":  "redismeter",
				"workload":    workloadName,
			},
		}

		if !jsonOutput {
			fmt.Printf("   📋 Redis Name: %s\n", redisName)
			fmt.Printf("   📋 SKU: %s\n", amrSKU)
			fmt.Printf("   📋 Clustering: %s\n", amrClustering)
			fmt.Printf("   ⏳ Starting deployment...\n\n")
		}

		startTime := time.Now()
		amrResult, err = amrDeployer.DeployAMR(ctx, resourceGroup, amrParams)
		if err != nil {
			return fmt.Errorf("failed to deploy AMR: %w", err)
		}
		deployDuration := time.Since(startTime)

		if !jsonOutput {
			fmt.Println()
			fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
			fmt.Println("║           ✅ AMR Deployment Successful!                          ║")
			fmt.Println("╠══════════════════════════════════════════════════════════════════╣")
			fmt.Printf("║  Hostname: %-53s║\n", amrResult.RedisHostName)
			fmt.Printf("║  Port: %-57d║\n", amrResult.RedisPort)
			fmt.Printf("║  Duration: %-53s║\n", deployDuration.Round(time.Second))
			fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
			fmt.Println()
		}

		// Update target with AMR endpoint
		target.Host = amrResult.RedisHostName
		target.Port = amrResult.RedisPort
		target.Password = amrResult.RedisPrimaryKey
		target.TLS = &domain.TLSConfig{Enabled: true} // AMR always requires TLS
		target.Cluster = (amrClustering == "OSSCluster")
	}

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

		// Default instance type for Azure
		if provider == "azure" && instanceType == "c5.xlarge" {
			instanceType = "Standard_D4s_v3"
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
				ExistingEndpoint: fmt.Sprintf("%s:%d", target.Host, target.Port),
			},
			TTL: 2 * time.Hour, // Auto-cleanup after 2 hours
		}

		if !jsonOutput {
			fmt.Printf("🖥️  Step 3: Provisioning %d %s runner VM(s) in %s %s...\n", count, instanceType, provider, region)
		}

		infra, err = p.Provision(ctx, spec)
		if err != nil {
			return fmt.Errorf("failed to provision infrastructure: %w", err)
		}

		if !jsonOutput {
			fmt.Printf("   ✅ Infrastructure ready: %s\n\n", infra.ID)
		}
	}

	// Ensure cleanup unless keeping
	if !keepInfra && existingInfra == "" {
		defer func() {
			if !jsonOutput {
				fmt.Printf("\n🧹 Cleaning up resources...\n")
			}

			// Teardown VMs
			if p != nil && infra != nil {
				if err := p.Teardown(ctx, infra); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to teardown infrastructure: %v\n", err)
					fmt.Fprintf(os.Stderr, "Run 'redismeter cloud teardown %s' to clean up manually.\n", infra.ID)
				} else if !jsonOutput {
					fmt.Printf("   ✅ VMs torn down\n")
				}
			}

			// Delete AMR resource group (includes AMR instance)
			if provisionAMR && resourceGroup != "" {
				if !jsonOutput {
					fmt.Printf("   Deleting resource group '%s' (includes AMR)...\n", resourceGroup)
				}
				cmd := osExec.Command("az", "group", "delete",
					"--name", resourceGroup,
					"--yes",
					"--no-wait")
				if err := cmd.Run(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to delete resource group: %v\n", err)
					fmt.Fprintf(os.Stderr, "Run 'az group delete --name %s --yes' to clean up manually.\n", resourceGroup)
				} else if !jsonOutput {
					fmt.Printf("   ✅ Resource group deletion initiated (runs in background)\n")
				}
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
		InfraID:       infra.ID,
		Provider:      provider,
		Region:        region,
		InstanceType:  instanceType,
		InstanceCount: len(hosts),
		Workload:      workloadName,
		Target:        targetStr,
		StartTime:     time.Now().Add(-time.Minute), // Approximate
		EndTime:       time.Now(),
		HostResults:   results,
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

// getAzureSubscriptionID gets the Azure subscription ID from env or az CLI.
func getAzureSubscriptionID() (string, error) {
	// Check environment variable first
	if subID := os.Getenv("AZURE_SUBSCRIPTION_ID"); subID != "" {
		return subID, nil
	}

	// Try to get from az CLI
	cmd := execCommand("az", "account", "show", "--query", "id", "-o", "tsv")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("AZURE_SUBSCRIPTION_ID not set and failed to get from az CLI: %w", err)
	}

	subID := strings.TrimSpace(string(out))
	if subID == "" {
		return "", fmt.Errorf("no Azure subscription found")
	}

	return subID, nil
}

// createAzureResourceGroup creates an Azure resource group using az CLI.
func createAzureResourceGroup(subscriptionID, name, location string) error {
	cmd := execCommand("az", "group", "create",
		"--name", name,
		"--location", location,
		"--subscription", subscriptionID,
		"-o", "none")

	return cmd.Run()
}

// execCommand is a wrapper for exec.Command that can be mocked in tests.
var execCommand = func(name string, args ...string) *execCmd {
	cmd := &execCmd{name: name, args: args}
	return cmd
}

type execCmd struct {
	name string
	args []string
}

func (c *execCmd) Output() ([]byte, error) {
	return realExec(c.name, c.args...).Output()
}

func (c *execCmd) Run() error {
	return realExec(c.name, c.args...).Run()
}

func realExec(name string, args ...string) *osExecCmd {
	return osExec.Command(name, args...)
}

type osExecCmd = osExec.Cmd

// runWithTerraformInfra runs benchmark using existing Terraform-managed infrastructure.
func runWithTerraformInfra(ctx context.Context, cmd *cobra.Command, workloadName, infraPattern string) error {
	// Get config directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	baseDir := filepath.Join(homeDir, ".redismeter", "terraform")

	// Initialize Terraform manager
	tfManager, err := terraform.NewManager(baseDir)
	if err != nil {
		return fmt.Errorf("failed to initialize Terraform manager: %w", err)
	}

	// Find matching infrastructure
	infraStates, err := tfManager.ListInfrastructure()
	if err != nil {
		return fmt.Errorf("failed to list infrastructure: %w", err)
	}

	var matchedState *terraform.InfraState
	for _, state := range infraStates {
		// Match by ID or by name pattern
		if state.ID == infraPattern || strings.Contains(state.ID, infraPattern) ||
			strings.Contains(state.Name, infraPattern) {
			if state.Status == "ready" {
				matchedState = state
				break
			}
		}
	}

	if matchedState == nil {
		return fmt.Errorf("no ready infrastructure found matching '%s'. Use 'redismeter infra list' to see available infrastructure", infraPattern)
	}

	fmt.Printf("Using infrastructure: %s (created %s)\n", matchedState.ID, matchedState.CreatedAt.Format(time.RFC3339))

	// Verify outputs
	if matchedState.Outputs == nil {
		return fmt.Errorf("infrastructure %s has no outputs. Try 'redismeter infra refresh %s'", matchedState.ID, matchedState.ID)
	}

	// Get benchmark parameters from flags
	clients, _ := cmd.Flags().GetInt("clients")
	threads, _ := cmd.Flags().GetInt("threads")
	requests, _ := cmd.Flags().GetInt("requests")
	dataSize, _ := cmd.Flags().GetInt("data-size")
	ratio, _ := cmd.Flags().GetString("ratio")
	pipeline, _ := cmd.Flags().GetInt("pipeline")
	jsonOutput, _ := cmd.Flags().GetBool("json")
	outputFile, _ := cmd.Flags().GetString("output")

	// Build target from infrastructure outputs
	target := &domain.Target{
		Host:     matchedState.Outputs.RedisHostname,
		Port:     matchedState.Outputs.RedisPort,
		Password: matchedState.Outputs.RedisPrimaryKey,
		TLS:      &domain.TLSConfig{Enabled: true, InsecureSkipVerify: true}, // AMR always uses TLS
	}

	if target.Host == "" {
		return fmt.Errorf("infrastructure %s has no Redis hostname. Verify Redis was provisioned", matchedState.ID)
	}

	fmt.Printf("Target: %s:%d\n", target.Host, target.Port)

	// Load workload
	wl, err := loadWorkload(workloadName)
	if err != nil {
		return fmt.Errorf("failed to load workload: %w", err)
	}

	// Override workload settings
	if clients > 0 {
		wl.Clients = clients
	}
	if threads > 0 {
		wl.Threads = threads
	}
	if requests > 0 {
		wl.Requests = int64(requests)
	}
	if dataSize > 0 {
		wl.DataSize = &domain.DataSize{Fixed: dataSize}
	}
	// Handle ratio flag - compute from operations if provided
	_ = ratio // ratio is handled via workload operations
	if pipeline > 0 {
		wl.Pipeline = pipeline
	}

	// Check if we have runner VMs
	if len(matchedState.Outputs.RunnerIPs) > 0 {
		return runDistributedBenchmarkWithInfra(ctx, cmd, wl, target, matchedState, jsonOutput, outputFile)
	}

	// No runners - run locally
	fmt.Println("No runner VMs found in infrastructure. Running benchmark locally...")
	return runLocalBenchmark(ctx, wl, target, jsonOutput, outputFile)
}

// runDistributedBenchmarkWithInfra runs a benchmark using Terraform-managed runner VMs.
func runDistributedBenchmarkWithInfra(ctx context.Context, cmd *cobra.Command, wl *domain.Workload, target *domain.Target, state *terraform.InfraState, jsonOutput bool, outputFile string) error {
	runnerIPs := state.Outputs.RunnerIPs
	benchmarkStartTime := time.Now()

	fmt.Printf("Running distributed benchmark on %d runner(s): %v\n", len(runnerIPs), runnerIPs)

	// Get SSH config
	sshUser := "azureuser"
	if state.Config.Runners != nil && state.Config.Runners.SSHUser != "" {
		sshUser = state.Config.Runners.SSHUser
	}

	// Build memtier command with JSON output
	jsonOutFile := "/tmp/memtier_results.json"
	memtierArgs := buildMemtierCommand(wl, target, jsonOutFile)
	memtierCmd := "memtier_benchmark " + strings.Join(memtierArgs, " ")

	// Verify time sync and connectivity on all runners first
	fmt.Printf("\nVerifying runner connectivity and time sync...\n")
	var runnerTimes []time.Time
	for _, ip := range runnerIPs {
		// Check connectivity and get current time from runner
		output, err := runSSHCommand(ctx, ip, sshUser, "date -u +%s.%N")
		if err != nil {
			return fmt.Errorf("failed to connect to runner %s: %w", ip, err)
		}
		var unixTime float64
		fmt.Sscanf(strings.TrimSpace(output), "%f", &unixTime)
		runnerTime := time.Unix(int64(unixTime), int64((unixTime-float64(int64(unixTime)))*1e9))
		runnerTimes = append(runnerTimes, runnerTime)
		fmt.Printf("  ✓ Runner %s: time=%s\n", ip, runnerTime.Format("15:04:05.000"))
	}

	// Check time skew between runners
	if len(runnerTimes) > 1 {
		var maxSkew time.Duration
		for i := 1; i < len(runnerTimes); i++ {
			skew := runnerTimes[i].Sub(runnerTimes[0])
			if skew < 0 {
				skew = -skew
			}
			if skew > maxSkew {
				maxSkew = skew
			}
		}
		fmt.Printf("  Time skew between runners: %v\n", maxSkew)
		if maxSkew > 5*time.Second {
			fmt.Printf("  ⚠️  Warning: Time skew >5s detected. Results may not be perfectly synchronized.\n")
		}
	}

	// Schedule synchronized start: 5 seconds from now
	// This gives time to dispatch commands to all runners
	startTime := time.Now().Add(5 * time.Second)
	startTimeUnix := startTime.Unix()

	// Build the synchronized command that waits until the target time, runs benchmark, then outputs JSON
	syncCmd := fmt.Sprintf(`
		TARGET=%d
		NOW=$(date +%%s)
		WAIT=$((TARGET - NOW))
		if [ $WAIT -gt 0 ] && [ $WAIT -lt 30 ]; then
			echo "Waiting ${WAIT}s for synchronized start at $(date -d @$TARGET '+%%H:%%M:%%S')"
			sleep $WAIT
		fi
		echo "Starting benchmark at $(date '+%%H:%%M:%%S.%%3N')"
		%s
		echo "===JSON_OUTPUT_START==="
		cat %s 2>/dev/null || echo "{}"
		echo "===JSON_OUTPUT_END==="
	`, startTimeUnix, memtierCmd, jsonOutFile)

	fmt.Printf("\nScheduled start time: %s (in 5 seconds)\n", startTime.Format("15:04:05"))
	fmt.Printf("Launching benchmarks on all %d runners in parallel...\n", len(runnerIPs))

	// Run on all runners in PARALLEL
	type runnerResult struct {
		ip       string
		output   string
		jsonData []byte
		err      error
		start    time.Time
		end      time.Time
	}

	resultsCh := make(chan runnerResult, len(runnerIPs))
	var wg sync.WaitGroup

	for _, ip := range runnerIPs {
		wg.Add(1)
		go func(runnerIP string) {
			defer wg.Done()
			start := time.Now()
			output, err := runSSHCommand(ctx, runnerIP, sshUser, syncCmd)
			end := time.Now()

			// Extract JSON data from output
			var jsonData []byte
			if err == nil {
				jsonData = extractJSONFromOutput(output)
			}

			resultsCh <- runnerResult{
				ip:       runnerIP,
				output:   output,
				jsonData: jsonData,
				err:      err,
				start:    start,
				end:      end,
			}
		}(ip)
	}

	// Wait for all runners to complete
	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	// Collect results
	var allResults []runnerResult
	var allJSONData [][]byte
	var successCount int
	for result := range resultsCh {
		if result.err != nil {
			fmt.Printf("  ✗ Runner %s failed: %v\n", result.ip, result.err)
			continue
		}
		successCount++
		allResults = append(allResults, result)
		if len(result.jsonData) > 0 {
			allJSONData = append(allJSONData, result.jsonData)
		}
		fmt.Printf("  ✓ Runner %s completed (wall time: %v)\n", result.ip, result.end.Sub(result.start).Round(time.Second))
	}

	if len(allResults) == 0 {
		return fmt.Errorf("all runners failed")
	}

	benchmarkEndTime := time.Now()

	// Parse and aggregate results
	aggregatedResults, parseErr := parseAndAggregateResults(allJSONData)
	if parseErr != nil {
		fmt.Printf("  ⚠️  Warning: Could not parse JSON results: %v\n", parseErr)
		fmt.Printf("  Raw results will be displayed but not saved to database.\n")
	}

	// Combine and display results
	fmt.Printf("\n=== Benchmark Results (%d/%d runners) ===\n", successCount, len(runnerIPs))

	if aggregatedResults != nil && aggregatedResults.Summary != nil {
		s := aggregatedResults.Summary
		fmt.Println()
		fmt.Println("📊 Aggregated Results (combined from all runners)")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("  Throughput:     %.2f ops/sec\n", s.OpsPerSecond)
		fmt.Printf("  Avg Latency:    %.3f ms\n", s.AvgLatencyMs)
		fmt.Printf("  P50 Latency:    %.3f ms\n", s.P50LatencyMs)
		fmt.Printf("  P99 Latency:    %.3f ms\n", s.P99LatencyMs)
		fmt.Printf("  P99.9 Latency:  %.3f ms\n", s.P999LatencyMs)

		if len(aggregatedResults.ByOperation) > 0 {
			fmt.Println()
			fmt.Println("  By Operation:")
			for op, metrics := range aggregatedResults.ByOperation {
				fmt.Printf("    %s: %.2f ops/sec, %.3f ms avg\n", op, metrics.OpsPerSecond, metrics.AvgLatencyMs)
			}
		}
	}

	// Save to storage if we have parsed results
	var savedRunID string
	if aggregatedResults != nil {
		run, err := saveCloudBenchmarkRun(ctx, wl, target, state, aggregatedResults, benchmarkStartTime, benchmarkEndTime, len(runnerIPs))
		if err != nil {
			fmt.Printf("  ⚠️  Warning: Could not save results to database: %v\n", err)
		} else {
			savedRunID = run.ID
			fmt.Println()
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Printf("✅ Run ID: %s\n", run.ID)
			fmt.Printf("   Duration: %s\n", run.Duration)
			fmt.Println()
			fmt.Println("View details with: redismeter show", run.ID)
		}
	}

	if jsonOutput {
		combined := map[string]interface{}{
			"run_id":            savedRunID,
			"infrastructure_id": state.ID,
			"runner_count":      len(runnerIPs),
			"successful_runs":   successCount,
			"target":            fmt.Sprintf("%s:%d", target.Host, target.Port),
			"workload":          wl.Name,
			"duration":          benchmarkEndTime.Sub(benchmarkStartTime).String(),
		}
		if aggregatedResults != nil && aggregatedResults.Summary != nil {
			combined["results"] = aggregatedResults
		}
		data, _ := json.MarshalIndent(combined, "", "  ")
		fmt.Println(string(data))
		if outputFile != "" {
			os.WriteFile(outputFile, data, 0644)
		}
	} else if aggregatedResults == nil {
		// Fall back to showing raw output if JSON parsing failed
		for i, result := range allResults {
			fmt.Printf("\n--- Runner %d (%s) ---\n%s\n", i+1, result.ip, result.output)
		}
	}

	return nil
}

// extractJSONFromOutput extracts JSON data from runner output that's wrapped in markers.
func extractJSONFromOutput(output string) []byte {
	startMarker := "===JSON_OUTPUT_START==="
	endMarker := "===JSON_OUTPUT_END==="

	startIdx := strings.Index(output, startMarker)
	endIdx := strings.Index(output, endMarker)

	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		return nil
	}

	jsonStr := strings.TrimSpace(output[startIdx+len(startMarker) : endIdx])
	return []byte(jsonStr)
}

// parseAndAggregateResults parses multiple memtier JSON outputs and aggregates them.
func parseAndAggregateResults(jsonDataList [][]byte) (*domain.Results, error) {
	if len(jsonDataList) == 0 {
		return nil, fmt.Errorf("no JSON data to parse")
	}

	parser := memtier.NewParser()
	var allResults []*domain.Results

	for i, data := range jsonDataList {
		if len(data) == 0 {
			continue
		}
		result, err := parser.Parse(data)
		if err != nil {
			fmt.Printf("  Warning: Failed to parse runner %d JSON: %v\n", i+1, err)
			continue
		}
		allResults = append(allResults, result)
	}

	if len(allResults) == 0 {
		return nil, fmt.Errorf("no results could be parsed")
	}

	// Aggregate results: sum ops/sec, average latencies
	aggregated := &domain.Results{
		Summary:     &domain.SummaryMetrics{},
		ByOperation: make(map[string]*domain.OperationMetrics),
	}

	// Sum throughput from all runners
	var totalOps float64
	var totalLatency float64
	var count int

	for _, r := range allResults {
		if r.Summary != nil {
			totalOps += r.Summary.OpsPerSecond
			totalLatency += r.Summary.AvgLatencyMs
			count++

			// Take max of percentile latencies (worst case)
			if r.Summary.P50LatencyMs > aggregated.Summary.P50LatencyMs {
				aggregated.Summary.P50LatencyMs = r.Summary.P50LatencyMs
			}
			if r.Summary.P90LatencyMs > aggregated.Summary.P90LatencyMs {
				aggregated.Summary.P90LatencyMs = r.Summary.P90LatencyMs
			}
			if r.Summary.P95LatencyMs > aggregated.Summary.P95LatencyMs {
				aggregated.Summary.P95LatencyMs = r.Summary.P95LatencyMs
			}
			if r.Summary.P99LatencyMs > aggregated.Summary.P99LatencyMs {
				aggregated.Summary.P99LatencyMs = r.Summary.P99LatencyMs
			}
			if r.Summary.P999LatencyMs > aggregated.Summary.P999LatencyMs {
				aggregated.Summary.P999LatencyMs = r.Summary.P999LatencyMs
			}
		}

		// Aggregate by operation
		for opName, opMetrics := range r.ByOperation {
			if existing, ok := aggregated.ByOperation[opName]; ok {
				existing.OpsPerSecond += opMetrics.OpsPerSecond
				existing.Count += opMetrics.Count
				// Average latencies
				existing.AvgLatencyMs = (existing.AvgLatencyMs + opMetrics.AvgLatencyMs) / 2
			} else {
				aggregated.ByOperation[opName] = &domain.OperationMetrics{
					Operation:    opMetrics.Operation,
					Count:        opMetrics.Count,
					OpsPerSecond: opMetrics.OpsPerSecond,
					AvgLatencyMs: opMetrics.AvgLatencyMs,
					P50LatencyMs: opMetrics.P50LatencyMs,
					P90LatencyMs: opMetrics.P90LatencyMs,
					P95LatencyMs: opMetrics.P95LatencyMs,
					P99LatencyMs: opMetrics.P99LatencyMs,
				}
			}
		}
	}

	aggregated.Summary.OpsPerSecond = totalOps
	if count > 0 {
		aggregated.Summary.AvgLatencyMs = totalLatency / float64(count)
	}

	return aggregated, nil
}

// saveCloudBenchmarkRun creates and saves a BenchmarkRun to storage.
func saveCloudBenchmarkRun(ctx context.Context, wl *domain.Workload, target *domain.Target, state *terraform.InfraState, results *domain.Results, startTime, endTime time.Time, runnerCount int) (*domain.BenchmarkRun, error) {
	// Get storage configuration
	storageType, storagePath := getStorageConfig()
	store, err := newRunStorage(storageType, storagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	// Create benchmark run
	run := &domain.BenchmarkRun{
		ID:        uuid.New().String(),
		Workload:  wl,
		Target:    target,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  endTime.Sub(startTime).String(),
		Status:    domain.RunStatusCompleted,
		Results:   results,
		Tags:      []string{"cloud", state.Provider, state.Region},
		Labels: map[string]string{
			"cloud":          "true",
			"provider":       state.Provider,
			"region":         state.Region,
			"infrastructure": state.ID,
			"runner_count":   fmt.Sprintf("%d", runnerCount),
		},
	}

	// Add AMR-specific labels if applicable
	if state.Config.AMR != nil {
		run.Labels["amr_sku"] = state.Config.AMR.SKU
		if state.Config.AMR.ClusteringPolicy != "" {
			run.Labels["amr_clustering"] = state.Config.AMR.ClusteringPolicy
		}
	}

	// Add runner instance type if available
	if state.Config.Runners != nil {
		run.Labels["runner_instance_type"] = state.Config.Runners.InstanceType
	}

	// Save to storage
	if err := store.SaveRun(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to save run: %w", err)
	}

	return run, nil
}

// runLocalBenchmark runs a benchmark locally (no remote runners).
func runLocalBenchmark(ctx context.Context, wl *domain.Workload, target *domain.Target, jsonOutput bool, outputFile string) error {
	memtierArgs := buildMemtierCommand(wl, target, "")

	fmt.Printf("Running: memtier_benchmark %s\n\n", strings.Join(memtierArgs, " "))

	execCmd := osExec.CommandContext(ctx, "memtier_benchmark", memtierArgs...)
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	return execCmd.Run()
}

// buildMemtierCommand constructs the memtier_benchmark command line.
// If jsonOutFile is provided, adds --json-out-file flag for structured output.
func buildMemtierCommand(wl *domain.Workload, target *domain.Target, jsonOutFile string) []string {
	// Get data size
	dataSizeVal := 256 // default
	if wl.DataSize != nil {
		if wl.DataSize.Fixed > 0 {
			dataSizeVal = wl.DataSize.Fixed
		} else if wl.DataSize.Min > 0 {
			dataSizeVal = wl.DataSize.Min
		}
	}

	args := []string{
		"-s", target.Host,
		"-p", fmt.Sprintf("%d", target.Port),
		"-c", fmt.Sprintf("%d", wl.Clients),
		"-t", fmt.Sprintf("%d", wl.Threads),
		"-n", fmt.Sprintf("%d", wl.Requests),
		"-d", fmt.Sprintf("%d", dataSizeVal),
	}

	// Calculate ratio from operations
	if len(wl.Operations) > 0 {
		var getRatio, setRatio float64
		for _, op := range wl.Operations {
			switch op.Command {
			case "GET":
				getRatio = op.Ratio
			case "SET":
				setRatio = op.Ratio
			}
		}
		if getRatio > 0 || setRatio > 0 {
			// Convert to integer ratio format (e.g., "1:1")
			getInt := int(getRatio * 10)
			setInt := int(setRatio * 10)
			if getInt > 0 || setInt > 0 {
				args = append(args, "--ratio", fmt.Sprintf("%d:%d", setInt, getInt))
			}
		}
	}

	if wl.Pipeline > 0 {
		args = append(args, "--pipeline", fmt.Sprintf("%d", wl.Pipeline))
	}

	if target.Password != "" {
		args = append(args, "-a", target.Password)
	}

	if target.TLS != nil && target.TLS.Enabled {
		args = append(args, "--tls")
		if target.TLS.InsecureSkipVerify {
			args = append(args, "--tls-skip-verify")
		}
	}

	// Add JSON output file for structured results
	if jsonOutFile != "" {
		args = append(args, "--json-out-file", jsonOutFile)
	}

	return args
}

// runSSHCommand executes a command on a remote host via SSH.
func runSSHCommand(ctx context.Context, host, user, command string) (string, error) {
	sshArgs := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=30",
		fmt.Sprintf("%s@%s", user, host),
		command,
	}

	cmd := osExec.CommandContext(ctx, "ssh", sshArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("SSH command failed: %w, output: %s", err, string(output))
	}

	return string(output), nil
}
