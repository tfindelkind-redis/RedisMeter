package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	osExec "os/exec"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tfindelkind-redis/redismeter/internal/cloud"
)

// cloudCmd represents the cloud command group.
var cloudCmd = &cobra.Command{
	Use:   "cloud",
	Short: "Manage cloud infrastructure for distributed benchmarks",
	Long: `Manage cloud infrastructure for running distributed benchmarks.

This command group allows you to provision, manage, and tear down
cloud infrastructure for running benchmark workloads across multiple
nodes. Supported providers include AWS, GCP, and Azure.`,
}

// cloudProvisionCmd provisions cloud infrastructure.
var cloudProvisionCmd = &cobra.Command{
	Use:   "provision",
	Short: "Provision cloud infrastructure",
	Long: `Provision cloud infrastructure for benchmark execution.

This creates load generator instances that can run memtier_benchmark
against your Redis target. The infrastructure will be automatically
tagged for easy identification and cleanup.`,
	Example: `  # Provision 2 load generators in AWS
  redismeter cloud provision --provider aws --region us-east-1 --count 2

  # Provision with specific instance type
  redismeter cloud provision --provider aws --instance-type c5.xlarge --count 4

  # Provision with spot instances for cost savings
  redismeter cloud provision --provider aws --count 2 --spot

  # Provision with TTL for automatic cleanup
  redismeter cloud provision --provider aws --count 2 --ttl 1h`,
	RunE: runCloudProvision,
}

// cloudListCmd lists active infrastructure.
var cloudListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active cloud infrastructure",
	Long:  `List all active RedisMeter-managed cloud infrastructure.`,
	Example: `  # List all infrastructure
  redismeter cloud list

  # List with JSON output
  redismeter cloud list --json`,
	RunE: runCloudList,
}

// cloudShowCmd shows infrastructure details.
var cloudShowCmd = &cobra.Command{
	Use:   "show <infra-id>",
	Short: "Show infrastructure details",
	Long:  `Display detailed information about specific infrastructure.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runCloudShow,
}

// cloudTeardownCmd tears down infrastructure.
var cloudTeardownCmd = &cobra.Command{
	Use:   "teardown <infra-id>",
	Short: "Tear down cloud infrastructure",
	Long:  `Terminate and clean up cloud infrastructure.`,
	Example: `  # Tear down specific infrastructure
  redismeter cloud teardown infra-abc123

  # Tear down without confirmation
  redismeter cloud teardown infra-abc123 --force`,
	Args: cobra.ExactArgs(1),
	RunE: runCloudTeardown,
}

// cloudRegionsCmd lists available regions.
var cloudRegionsCmd = &cobra.Command{
	Use:   "regions",
	Short: "List available cloud regions",
	Long:  `List available regions for the specified cloud provider.`,
	Example: `  # List AWS regions
  redismeter cloud regions --provider aws`,
	RunE: runCloudRegions,
}

// cloudInstanceTypesCmd lists available instance types.
var cloudInstanceTypesCmd = &cobra.Command{
	Use:   "instance-types",
	Short: "List available instance types",
	Long:  `List available instance types for the specified cloud provider.`,
	Example: `  # List instance types in us-east-1
  redismeter cloud instance-types --provider aws --region us-east-1`,
	RunE: runCloudInstanceTypes,
}

// cloudEstimateCmd estimates infrastructure costs.
var cloudEstimateCmd = &cobra.Command{
	Use:   "estimate",
	Short: "Estimate infrastructure costs",
	Long:  `Estimate the cost of running infrastructure for a given duration.`,
	Example: `  # Estimate cost for 2 hours
  redismeter cloud estimate --provider aws --count 2 --instance-type c5.xlarge --duration 2h`,
	RunE: runCloudEstimate,
}

// cloudSSHCmd connects to an instance via SSH.
var cloudSSHCmd = &cobra.Command{
	Use:   "ssh <infra-id> [instance-index]",
	Short: "SSH to a load generator instance",
	Long: `Connect to a load generator instance via SSH.

If instance-index is not specified, connects to the first instance.`,
	Example: `  # SSH to first instance
  redismeter cloud ssh infra-abc123

  # SSH to specific instance
  redismeter cloud ssh infra-abc123 1`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runCloudSSH,
}

func init() {
	rootCmd.AddCommand(cloudCmd)
	cloudCmd.AddCommand(cloudProvisionCmd)
	cloudCmd.AddCommand(cloudListCmd)
	cloudCmd.AddCommand(cloudShowCmd)
	cloudCmd.AddCommand(cloudTeardownCmd)
	cloudCmd.AddCommand(cloudRegionsCmd)
	cloudCmd.AddCommand(cloudInstanceTypesCmd)
	cloudCmd.AddCommand(cloudEstimateCmd)
	cloudCmd.AddCommand(cloudSSHCmd)

	// Provision flags
	cloudProvisionCmd.Flags().String("provider", "aws", "Cloud provider (aws, gcp, azure)")
	cloudProvisionCmd.Flags().String("region", "", "Cloud region")
	cloudProvisionCmd.Flags().String("instance-type", "", "Instance type for load generators")
	cloudProvisionCmd.Flags().Int("count", 1, "Number of load generator instances")
	cloudProvisionCmd.Flags().Bool("spot", false, "Use spot/preemptible instances")
	cloudProvisionCmd.Flags().Float64("max-spot-price", 0, "Maximum spot price (per hour)")
	cloudProvisionCmd.Flags().String("key-name", "", "SSH key name (must exist in provider)")
	cloudProvisionCmd.Flags().String("name", "", "Infrastructure name")
	cloudProvisionCmd.Flags().Duration("ttl", 0, "Time-to-live for automatic cleanup")
	cloudProvisionCmd.Flags().String("redis-endpoint", "", "Existing Redis endpoint to benchmark")
	cloudProvisionCmd.Flags().StringToString("tags", nil, "Tags to apply to resources")
	cloudProvisionCmd.Flags().String("vpc-id", "", "Existing VPC ID to use")
	cloudProvisionCmd.Flags().String("subnet-id", "", "Existing subnet ID to use")
	cloudProvisionCmd.Flags().Bool("json", false, "Output as JSON")

	// List flags
	cloudListCmd.Flags().Bool("json", false, "Output as JSON")

	// Show flags
	cloudShowCmd.Flags().Bool("json", false, "Output as JSON")

	// Teardown flags
	cloudTeardownCmd.Flags().BoolP("force", "f", false, "Skip confirmation")

	// Regions flags
	cloudRegionsCmd.Flags().String("provider", "aws", "Cloud provider")
	cloudRegionsCmd.Flags().Bool("json", false, "Output as JSON")

	// Instance types flags
	cloudInstanceTypesCmd.Flags().String("provider", "aws", "Cloud provider")
	cloudInstanceTypesCmd.Flags().String("region", "", "Cloud region")
	cloudInstanceTypesCmd.Flags().Bool("json", false, "Output as JSON")

	// Estimate flags
	cloudEstimateCmd.Flags().String("provider", "aws", "Cloud provider")
	cloudEstimateCmd.Flags().String("region", "", "Cloud region")
	cloudEstimateCmd.Flags().String("instance-type", "t3.medium", "Instance type")
	cloudEstimateCmd.Flags().Int("count", 1, "Number of instances")
	cloudEstimateCmd.Flags().Bool("spot", false, "Use spot instances")
	cloudEstimateCmd.Flags().Duration("duration", 1*time.Hour, "Duration to estimate")
	cloudEstimateCmd.Flags().Bool("json", false, "Output as JSON")

	// SSH flags
	cloudSSHCmd.Flags().String("user", "ubuntu", "SSH user")
	cloudSSHCmd.Flags().String("key", "", "Path to SSH private key")
}

// Provider registry for managing providers
var cloudProviders = make(map[string]cloud.ProviderPlugin)

func getCloudProvider(ctx context.Context, provider string) (cloud.ProviderPlugin, error) {
	if p, ok := cloudProviders[provider]; ok {
		return p, nil
	}

	var p cloud.ProviderPlugin
	var err error
	switch provider {
	case "aws":
		p = cloud.NewAWSProvider()
	case "azure":
		// Get subscription ID
		subID := viper.GetString("azure.subscription_id")
		if subID == "" {
			// Try from environment
			subID = os.Getenv("AZURE_SUBSCRIPTION_ID")
		}
		if subID == "" {
			// Try from az CLI
			cmd := osExec.Command("az", "account", "show", "--query", "id", "-o", "tsv")
			out, cmdErr := cmd.Output()
			if cmdErr == nil {
				subID = strings.TrimSpace(string(out))
			}
		}
		if subID == "" {
			return nil, fmt.Errorf("Azure subscription ID not found. Set AZURE_SUBSCRIPTION_ID or run 'az login'")
		}

		// Get SSH public key
		sshPubKey := viper.GetString("azure.ssh_public_key")
		if sshPubKey == "" {
			// Try reading from default location
			homeDir, _ := os.UserHomeDir()
			defaultPubKeyPath := homeDir + "/.ssh/id_rsa.pub"
			if pubKeyData, readErr := os.ReadFile(defaultPubKeyPath); readErr == nil {
				sshPubKey = strings.TrimSpace(string(pubKeyData))
			}
		}
		if sshPubKey == "" {
			return nil, fmt.Errorf("SSH public key not found. Set azure.ssh_public_key in config or create ~/.ssh/id_rsa.pub")
		}

		// Get SSH private key path
		sshPrivKey := viper.GetString("ssh.private_key_path")
		if sshPrivKey == "" {
			homeDir, _ := os.UserHomeDir()
			sshPrivKey = homeDir + "/.ssh/id_rsa"
		}

		// Get SSH user with default
		sshUser := viper.GetString("ssh.user")
		if sshUser == "" {
			sshUser = "azureuser"
		}

		azConfig := cloud.AzureConfig{
			SubscriptionID: subID,
			SSHPublicKey:   sshPubKey,
			SSHPrivateKey:  sshPrivKey,
			SSHUser:        sshUser,
		}
		p, err = cloud.NewAzureProvider(azConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create Azure provider: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported cloud provider: %s", provider)
	}

	// Get provider-specific config from viper
	config := make(map[string]interface{})

	// AWS config
	if provider == "aws" {
		if region := viper.GetString("aws.region"); region != "" {
			config["region"] = region
		}
		if profile := viper.GetString("aws.profile"); profile != "" {
			config["profile"] = profile
		}
		if keyName := viper.GetString("aws.default_key_name"); keyName != "" {
			config["default_key_name"] = keyName
		}
		if ami := viper.GetString("aws.default_ami"); ami != "" {
			config["default_ami"] = ami
		}
		if vpcID := viper.GetString("aws.default_vpc_id"); vpcID != "" {
			config["default_vpc_id"] = vpcID
		}
		if subnetID := viper.GetString("aws.default_subnet_id"); subnetID != "" {
			config["default_subnet_id"] = subnetID
		}
	}

	// Azure config
	if provider == "azure" {
		if subID := viper.GetString("azure.subscription_id"); subID != "" {
			config["subscription_id"] = subID
		}
		if sshKey := viper.GetString("azure.ssh_public_key"); sshKey != "" {
			config["ssh_public_key"] = sshKey
		}
		if sshKeyPath := viper.GetString("ssh.private_key_path"); sshKeyPath != "" {
			config["ssh_private_key"] = sshKeyPath
		}
	}

	if err := p.Initialize(ctx, config); err != nil {
		return nil, fmt.Errorf("failed to initialize %s provider: %w", provider, err)
	}

	cloudProviders[provider] = p
	return p, nil
}

func runCloudProvision(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	provider, _ := cmd.Flags().GetString("provider")
	region, _ := cmd.Flags().GetString("region")
	instanceType, _ := cmd.Flags().GetString("instance-type")
	count, _ := cmd.Flags().GetInt("count")
	spot, _ := cmd.Flags().GetBool("spot")
	maxSpotPrice, _ := cmd.Flags().GetFloat64("max-spot-price")
	keyName, _ := cmd.Flags().GetString("key-name")
	name, _ := cmd.Flags().GetString("name")
	ttl, _ := cmd.Flags().GetDuration("ttl")
	redisEndpoint, _ := cmd.Flags().GetString("redis-endpoint")
	tags, _ := cmd.Flags().GetStringToString("tags")
	vpcID, _ := cmd.Flags().GetString("vpc-id")
	subnetID, _ := cmd.Flags().GetString("subnet-id")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	// Default region
	if region == "" {
		region = viper.GetString(fmt.Sprintf("%s.region", provider))
		if region == "" {
			region = "us-east-1"
		}
	}

	// Default name
	if name == "" {
		name = fmt.Sprintf("redismeter-%s", time.Now().Format("20060102-150405"))
	}

	// Get provider
	p, err := getCloudProvider(ctx, provider)
	if err != nil {
		return err
	}

	// Build spec
	spec := &cloud.InfraSpec{
		Name:     name,
		Provider: provider,
		Region:   region,
		LoadGenerator: &cloud.NodeGroupSpec{
			InstanceType:  instanceType,
			Count:         count,
			SpotInstances: spot,
			MaxSpotPrice:  maxSpotPrice,
			SSHKeyName:    keyName,
		},
		Tags: tags,
		TTL:  ttl,
	}

	if vpcID != "" || subnetID != "" {
		spec.Network = &cloud.NetworkSpec{
			VPCID:    vpcID,
			SubnetID: subnetID,
		}
	}

	if redisEndpoint != "" {
		spec.RedisTarget = &cloud.RedisTargetSpec{
			Type:             cloud.RedisTargetExisting,
			ExistingEndpoint: redisEndpoint,
		}
	}

	if !jsonOutput {
		fmt.Printf("Provisioning infrastructure...\n")
		fmt.Printf("  Provider:      %s\n", provider)
		fmt.Printf("  Region:        %s\n", region)
		fmt.Printf("  Instance type: %s\n", instanceType)
		fmt.Printf("  Count:         %d\n", count)
		if spot {
			fmt.Printf("  Spot:          yes\n")
		}
		fmt.Println()
	}

	// Provision
	infra, err := p.Provision(ctx, spec)
	if err != nil {
		return fmt.Errorf("provisioning failed: %w", err)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(infra)
	}

	fmt.Printf("✓ Infrastructure provisioned: %s\n\n", infra.ID)
	printInfrastructureDetails(infra)

	return nil
}

func runCloudList(cmd *cobra.Command, args []string) error {
	jsonOutput, _ := cmd.Flags().GetBool("json")

	// List all tracked infrastructure
	var infras []*cloud.Infrastructure
	for _, p := range cloudProviders {
		// Get all infras from provider
		// Note: This is a simplified implementation
		// In production, we'd track infras separately
		_ = p
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(infras)
	}

	if len(infras) == 0 {
		fmt.Println("No active infrastructure found.")
		fmt.Println("\nNote: Infrastructure tracking is session-based. Use 'cloud show <id>' with provider-specific tools to check infrastructure status.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tPROVIDER\tREGION\tSTATE\tINSTANCES\tCREATED")
	fmt.Fprintln(w, "--\t----\t--------\t------\t-----\t---------\t-------")

	for _, infra := range infras {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\t%s\n",
			infra.ID,
			truncate(infra.Name, 20),
			infra.Provider,
			infra.Region,
			infra.State,
			len(infra.LoadGenerators),
			infra.CreatedAt.Format("2006-01-02 15:04"),
		)
	}
	w.Flush()

	return nil
}

func runCloudShow(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	infraID := args[0]
	jsonOutput, _ := cmd.Flags().GetBool("json")

	// Try to find infrastructure in any provider
	var infra *cloud.Infrastructure
	for _, p := range cloudProviders {
		if i, err := p.GetInfrastructure(ctx, infraID); err == nil {
			infra = i
			break
		}
	}

	if infra == nil {
		return fmt.Errorf("infrastructure not found: %s", infraID)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(infra)
	}

	printInfrastructureDetails(infra)
	return nil
}

func runCloudTeardown(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	infraID := args[0]
	force, _ := cmd.Flags().GetBool("force")

	// Find infrastructure
	var infra *cloud.Infrastructure
	var provider cloud.ProviderPlugin
	for _, p := range cloudProviders {
		if i, err := p.GetInfrastructure(ctx, infraID); err == nil {
			infra = i
			provider = p
			break
		}
	}

	if infra == nil {
		return fmt.Errorf("infrastructure not found: %s", infraID)
	}

	// Confirm
	if !force {
		fmt.Printf("Tear down infrastructure '%s' (%s)?\n", infra.Name, infraID)
		fmt.Printf("  Provider:  %s\n", infra.Provider)
		fmt.Printf("  Region:    %s\n", infra.Region)
		fmt.Printf("  Instances: %d\n", len(infra.LoadGenerators))
		fmt.Print("\nConfirm [y/N]: ")

		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	fmt.Printf("Tearing down infrastructure...\n")

	if err := provider.Teardown(ctx, infra); err != nil {
		return fmt.Errorf("teardown failed: %w", err)
	}

	fmt.Printf("✓ Infrastructure torn down: %s\n", infraID)
	return nil
}

func runCloudRegions(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	provider, _ := cmd.Flags().GetString("provider")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	p, err := getCloudProvider(ctx, provider)
	if err != nil {
		return err
	}

	regions, err := p.ListRegions(ctx)
	if err != nil {
		return fmt.Errorf("failed to list regions: %w", err)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(regions)
	}

	fmt.Printf("Available regions for %s:\n\n", provider)
	for _, r := range regions {
		fmt.Printf("  %s\n", r.ID)
	}

	return nil
}

func runCloudInstanceTypes(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	provider, _ := cmd.Flags().GetString("provider")
	region, _ := cmd.Flags().GetString("region")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	p, err := getCloudProvider(ctx, provider)
	if err != nil {
		return err
	}

	types, err := p.ListInstanceTypes(ctx, region)
	if err != nil {
		return fmt.Errorf("failed to list instance types: %w", err)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(types)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TYPE\tVCPUs\tMEMORY\tCATEGORY")
	fmt.Fprintln(w, "----\t-----\t------\t--------")

	for _, t := range types {
		fmt.Fprintf(w, "%s\t%d\t%.1f GB\t%s\n",
			t.ID,
			t.VCPUs,
			t.MemoryGB,
			t.Category,
		)
	}
	w.Flush()

	return nil
}

func runCloudEstimate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	provider, _ := cmd.Flags().GetString("provider")
	region, _ := cmd.Flags().GetString("region")
	instanceType, _ := cmd.Flags().GetString("instance-type")
	count, _ := cmd.Flags().GetInt("count")
	spot, _ := cmd.Flags().GetBool("spot")
	duration, _ := cmd.Flags().GetDuration("duration")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	p, err := getCloudProvider(ctx, provider)
	if err != nil {
		return err
	}

	spec := &cloud.InfraSpec{
		Provider: provider,
		Region:   region,
		LoadGenerator: &cloud.NodeGroupSpec{
			InstanceType:  instanceType,
			Count:         count,
			SpotInstances: spot,
		},
	}

	estimate, err := p.EstimateCost(ctx, spec, duration)
	if err != nil {
		return fmt.Errorf("failed to estimate cost: %w", err)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(estimate)
	}

	fmt.Printf("Cost Estimate\n\n")
	fmt.Printf("  Provider:      %s\n", provider)
	fmt.Printf("  Instance type: %s\n", instanceType)
	fmt.Printf("  Count:         %d\n", count)
	fmt.Printf("  Duration:      %s\n", duration)
	if spot {
		fmt.Printf("  Spot:          yes (estimated 70%% savings)\n")
	}
	fmt.Println()
	fmt.Printf("  Hourly cost:   $%.4f\n", estimate.HourlyCost)
	fmt.Printf("  Total cost:    $%.2f %s\n", estimate.TotalCost, estimate.Currency)

	if len(estimate.Breakdown) > 0 {
		fmt.Println("\nBreakdown:")
		for _, item := range estimate.Breakdown {
			fmt.Printf("  %s: $%.2f\n", item.Description, item.TotalCost)
		}
	}

	return nil
}

func runCloudSSH(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	infraID := args[0]
	instanceIndex := 0
	if len(args) > 1 {
		fmt.Sscanf(args[1], "%d", &instanceIndex)
	}

	user, _ := cmd.Flags().GetString("user")
	keyPath, _ := cmd.Flags().GetString("key")

	// Find infrastructure
	var infra *cloud.Infrastructure
	for _, p := range cloudProviders {
		if i, err := p.GetInfrastructure(ctx, infraID); err == nil {
			infra = i
			break
		}
	}

	if infra == nil {
		return fmt.Errorf("infrastructure not found: %s", infraID)
	}

	if len(infra.LoadGenerators) == 0 {
		return fmt.Errorf("no instances found")
	}

	if instanceIndex >= len(infra.LoadGenerators) {
		return fmt.Errorf("instance index %d out of range (0-%d)", instanceIndex, len(infra.LoadGenerators)-1)
	}

	instance := infra.LoadGenerators[instanceIndex]
	if instance.PublicIP == "" {
		return fmt.Errorf("instance has no public IP")
	}

	// Print SSH command for user to run
	sshCmd := fmt.Sprintf("ssh %s@%s", user, instance.PublicIP)
	if keyPath != "" {
		sshCmd = fmt.Sprintf("ssh -i %s %s@%s", keyPath, user, instance.PublicIP)
	}

	fmt.Printf("Connect using:\n  %s\n", sshCmd)
	fmt.Println("\nOr use the 'cloud run' command to execute benchmarks directly.")

	return nil
}

func printInfrastructureDetails(infra *cloud.Infrastructure) {
	fmt.Printf("Infrastructure: %s\n", infra.ID)
	fmt.Printf("  Name:     %s\n", infra.Name)
	fmt.Printf("  Provider: %s\n", infra.Provider)
	fmt.Printf("  Region:   %s\n", infra.Region)
	fmt.Printf("  State:    %s\n", infra.State)
	fmt.Printf("  Created:  %s\n", infra.CreatedAt.Format(time.RFC3339))

	if infra.ExpiresAt != nil {
		fmt.Printf("  Expires:  %s\n", infra.ExpiresAt.Format(time.RFC3339))
	}

	if infra.RedisEndpoint != "" {
		fmt.Printf("\nRedis Target:\n")
		fmt.Printf("  Endpoint: %s:%d\n", infra.RedisEndpoint, infra.RedisPort)
	}

	if len(infra.LoadGenerators) > 0 {
		fmt.Printf("\nLoad Generators (%d):\n", len(infra.LoadGenerators))
		for i, inst := range infra.LoadGenerators {
			ip := inst.PublicIP
			if ip == "" {
				ip = inst.PrivateIP
			}
			fmt.Printf("  [%d] %s (%s) - %s - %s\n",
				i, inst.ID, inst.Type, inst.State, ip)
		}
	}

	if len(infra.Tags) > 0 {
		fmt.Printf("\nTags:\n")
		for k, v := range infra.Tags {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}
}
