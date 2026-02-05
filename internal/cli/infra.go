package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tfindelkind-redis/redismeter/internal/terraform"
)

// infraCmd is the parent command for infrastructure management.
var infraCmd = &cobra.Command{
	Use:   "infra",
	Short: "Manage cloud infrastructure with Terraform",
	Long: `Manage cloud infrastructure for benchmarking using Terraform.

Infrastructure is persisted with state tracking, so you always know what's
deployed even days later. Supports two modes:

  Ephemeral Mode (default):
    redismeter infra up --name test --delete-after
    # Run benchmarks, infrastructure auto-deleted after

  Persistent Mode:
    redismeter infra up --name test
    # Run benchmarks multiple times
    redismeter infra down test

Use 'redismeter infra list' to see all tracked infrastructure.`,
}

// infraUpCmd provisions new infrastructure.
var infraUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Provision new infrastructure",
	Long: `Provision new cloud infrastructure for benchmarking.

Examples:
  # Provision AMR and runner VMs in Azure
  redismeter infra up --name mytest --provider azure --region westus3 \
    --amr --amr-sku Balanced_B0 \
    --runners 2 --runner-type Standard_D4s_v3

  # Provision only AMR (no runner VMs)
  redismeter infra up --name redis-only --provider azure --region eastus \
    --amr --amr-sku Balanced_B1 --amr-ha

  # Provision with TTL for auto-cleanup
  redismeter infra up --name temp-test --provider azure --region westus3 \
    --amr --runners 1 --ttl 2h`,
	RunE: runInfraUp,
}

// infraDownCmd destroys infrastructure.
var infraDownCmd = &cobra.Command{
	Use:   "down <infra-id>",
	Short: "Destroy infrastructure",
	Long: `Destroy infrastructure and clean up all cloud resources.

Examples:
  # Destroy by ID
  redismeter infra down rm-mytest-20260204-123456

  # Destroy all expired infrastructure
  redismeter infra down --expired`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInfraDown,
}

// infraListCmd lists all infrastructure.
var infraListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tracked infrastructure",
	Long: `List all infrastructure managed by RedisMeter.

Shows status, provider, region, and connection details for each deployment.`,
	RunE: runInfraList,
}

// infraStatusCmd shows detailed status.
var infraStatusCmd = &cobra.Command{
	Use:   "status <infra-id>",
	Short: "Show detailed infrastructure status",
	Long: `Show detailed status including connection information, outputs,
and Terraform state for the specified infrastructure.`,
	Args: cobra.ExactArgs(1),
	RunE: runInfraStatus,
}

// infraRefreshCmd refreshes infrastructure state.
var infraRefreshCmd = &cobra.Command{
	Use:   "refresh <infra-id>",
	Short: "Refresh infrastructure state from cloud provider",
	Long:  `Update the local state by querying the cloud provider for current status.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runInfraRefresh,
}

func init() {
	rootCmd.AddCommand(infraCmd)
	infraCmd.AddCommand(infraUpCmd)
	infraCmd.AddCommand(infraDownCmd)
	infraCmd.AddCommand(infraListCmd)
	infraCmd.AddCommand(infraStatusCmd)
	infraCmd.AddCommand(infraRefreshCmd)

	// infraUpCmd flags
	infraUpCmd.Flags().String("name", "", "Name for the infrastructure (required)")
	infraUpCmd.Flags().String("provider", "azure", "Cloud provider (azure, aws)")
	infraUpCmd.Flags().String("region", "westus3", "Cloud region")
	infraUpCmd.Flags().Duration("ttl", 0, "Time-to-live for auto-cleanup (e.g., 2h)")
	infraUpCmd.Flags().StringToString("tags", nil, "Tags to apply to resources")

	// AMR flags
	infraUpCmd.Flags().Bool("amr", false, "Provision Azure Managed Redis")
	infraUpCmd.Flags().String("amr-sku", "Balanced_B0", "AMR SKU (Balanced_B0, Balanced_B1, etc.)")
	infraUpCmd.Flags().StringSlice("amr-modules", nil, "Redis modules (RedisJSON, RediSearch, etc.)")
	infraUpCmd.Flags().Bool("amr-ha", false, "Enable high availability for AMR")
	infraUpCmd.Flags().String("amr-clustering", "OSSCluster", "Clustering policy (OSSCluster, EnterpriseCluster)")
	infraUpCmd.Flags().String("amr-eviction", "VolatileLRU", "Eviction policy")

	// Runner flags
	infraUpCmd.Flags().Int("runners", 0, "Number of runner VMs to provision")
	infraUpCmd.Flags().String("runner-type", "Standard_D4s_v3", "VM instance type for runners")
	infraUpCmd.Flags().Bool("runners-spot", false, "Use spot instances for runners")
	infraUpCmd.Flags().String("ssh-key", "", "SSH public key (or path to .pub file)")
	infraUpCmd.Flags().String("ssh-user", "azureuser", "SSH username for VMs")

	infraUpCmd.MarkFlagRequired("name")

	// infraDownCmd flags
	infraDownCmd.Flags().Bool("expired", false, "Destroy all expired infrastructure")
	infraDownCmd.Flags().Bool("all", false, "Destroy all infrastructure")
	infraDownCmd.Flags().Bool("force", false, "Force destruction without confirmation")
}

func getInfraManager() (*terraform.Manager, error) {
	// Get base directory for Terraform workspaces
	baseDir := viper.GetString("infra.terraform_dir")
	if baseDir == "" {
		homeDir, _ := os.UserHomeDir()
		baseDir = filepath.Join(homeDir, ".redismeter", "terraform")
	}

	return terraform.NewManager(baseDir)
}

func runInfraUp(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	name, _ := cmd.Flags().GetString("name")
	provider, _ := cmd.Flags().GetString("provider")
	region, _ := cmd.Flags().GetString("region")
	ttl, _ := cmd.Flags().GetDuration("ttl")
	tags, _ := cmd.Flags().GetStringToString("tags")

	// AMR config
	provisionAMR, _ := cmd.Flags().GetBool("amr")
	amrSKU, _ := cmd.Flags().GetString("amr-sku")
	amrModules, _ := cmd.Flags().GetStringSlice("amr-modules")
	amrHA, _ := cmd.Flags().GetBool("amr-ha")
	amrClustering, _ := cmd.Flags().GetString("amr-clustering")
	amrEviction, _ := cmd.Flags().GetString("amr-eviction")

	// Runner config
	runnerCount, _ := cmd.Flags().GetInt("runners")
	runnerType, _ := cmd.Flags().GetString("runner-type")
	sshKeyArg, _ := cmd.Flags().GetString("ssh-key")
	sshUser, _ := cmd.Flags().GetString("ssh-user")

	// Build config
	config := terraform.InfraConfig{
		Name:     name,
		Provider: provider,
		Region:   region,
		TTL:      ttl,
		Tags:     tags,
	}

	if provisionAMR {
		config.AMR = &terraform.AMRConfig{
			SKU:              amrSKU,
			Modules:          amrModules,
			HighAvailability: amrHA,
			ClusteringPolicy: amrClustering,
			EvictionPolicy:   amrEviction,
		}
	}

	if runnerCount > 0 {
		// Get SSH public key
		sshPubKey := sshKeyArg
		if sshPubKey == "" {
			// Try reading from default location
			homeDir, _ := os.UserHomeDir()
			defaultPubKeyPath := filepath.Join(homeDir, ".ssh", "id_rsa.pub")
			if data, err := os.ReadFile(defaultPubKeyPath); err == nil {
				sshPubKey = strings.TrimSpace(string(data))
			}
		} else if _, err := os.Stat(sshPubKey); err == nil {
			// It's a file path, read it
			if data, err := os.ReadFile(sshPubKey); err == nil {
				sshPubKey = strings.TrimSpace(string(data))
			}
		}

		if sshPubKey == "" {
			return fmt.Errorf("SSH public key required for runner VMs. Provide --ssh-key or create ~/.ssh/id_rsa.pub")
		}

		config.Runners = &terraform.RunnerConfig{
			Count:        runnerCount,
			InstanceType: runnerType,
			SSHPublicKey: sshPubKey,
			SSHUser:      sshUser,
		}
	}

	// Get manager
	manager, err := getInfraManager()
	if err != nil {
		return fmt.Errorf("failed to initialize Terraform manager: %w", err)
	}

	// Print banner
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║          RedisMeter Infrastructure Provisioning                  ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Name: %-55s  ║\n", name)
	fmt.Printf("║  Provider: %-51s  ║\n", provider)
	fmt.Printf("║  Region: %-53s  ║\n", region)
	if config.AMR != nil {
		fmt.Printf("║  AMR SKU: %-52s  ║\n", amrSKU)
	}
	if config.Runners != nil {
		fmt.Printf("║  Runners: %d × %-47s  ║\n", runnerCount, runnerType)
	}
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	fmt.Println("🚀 Starting Terraform provisioning...")
	fmt.Println("   This may take 10-20 minutes for AMR deployment.")
	fmt.Println()

	// Create progress display
	progress := NewInfraProgressDisplay()
	var lastPhase string

	// Progress callback for Terraform operations
	progressCallback := func(event terraform.TerraformEvent) {
		// Start spinner for new phase
		if event.Phase != lastPhase {
			if lastPhase != "" {
				progress.Success(fmt.Sprintf("%s complete", lastPhase))
			}
			lastPhase = event.Phase
			progress.Start(event.Phase)
		}

		// Update progress
		progress.Update(event.Action, event.Resource, event.ElapsedTime, event.Completed, event.Total)
	}

	startTime := time.Now()
	state, err := manager.ProvisionWithProgress(ctx, config, progressCallback)

	// Stop progress display
	if err != nil {
		progress.Fail(fmt.Sprintf("Provisioning failed: %v", err))
		if state != nil {
			fmt.Printf("   Workspace: %s\n", state.WorkspacePath)
			fmt.Println("   Check the workspace for Terraform logs and state.")
		}
		return err
	}

	progress.Success(fmt.Sprintf("%s complete", lastPhase))

	duration := time.Since(startTime)

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║           ✅ Infrastructure Ready!                               ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  ID: %-57s  ║\n", state.ID)
	fmt.Printf("║  Duration: %-51s  ║\n", duration.Round(time.Second))
	if state.Outputs != nil {
		if state.Outputs.RedisHostname != "" {
			fmt.Printf("║  Redis: %-54s  ║\n", state.Outputs.RedisHostname+":10000")
		}
		if len(state.Outputs.RunnerIPs) > 0 {
			fmt.Printf("║  Runners: %-52s  ║\n", strings.Join(state.Outputs.RunnerIPs, ", "))
		}
	}
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	fmt.Println("📋 Next steps:")
	fmt.Printf("   • Run benchmark: redismeter cloud run cache --infra %s\n", state.ID)
	fmt.Printf("   • View status:   redismeter infra status %s\n", state.ID)
	fmt.Printf("   • Destroy:       redismeter infra down %s\n", state.ID)
	fmt.Println()

	if state.Outputs != nil && state.Outputs.RedisPrimaryKey != "" {
		fmt.Println("🔑 Connection details:")
		fmt.Printf("   Host: %s\n", state.Outputs.RedisHostname)
		fmt.Printf("   Port: %d\n", state.Outputs.RedisPort)
		fmt.Println("   Password: (stored in Terraform state)")
		fmt.Println()
	}

	return nil
}

func runInfraDown(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	manager, err := getInfraManager()
	if err != nil {
		return err
	}

	expiredOnly, _ := cmd.Flags().GetBool("expired")
	all, _ := cmd.Flags().GetBool("all")
	force, _ := cmd.Flags().GetBool("force")

	if expiredOnly || all {
		states, err := manager.ListInfrastructure()
		if err != nil {
			return err
		}

		var toDestroy []*terraform.InfraState
		for _, state := range states {
			if state.Status == "destroyed" {
				continue
			}
			if all || (expiredOnly && !state.ExpiresAt.IsZero() && time.Now().After(state.ExpiresAt)) {
				toDestroy = append(toDestroy, state)
			}
		}

		if len(toDestroy) == 0 {
			fmt.Println("No infrastructure to destroy.")
			return nil
		}

		fmt.Printf("Will destroy %d infrastructure(s):\n", len(toDestroy))
		for _, state := range toDestroy {
			fmt.Printf("  • %s (%s)\n", state.ID, state.Status)
		}

		if !force {
			fmt.Print("\nContinue? [y/N]: ")
			var response string
			fmt.Scanln(&response)
			if strings.ToLower(response) != "y" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		for _, state := range toDestroy {
			fmt.Printf("\n🗑️  Destroying %s...\n", state.ID)

			// Create progress display for this infrastructure
			progress := NewInfraProgressDisplay()
			var lastPhase string

			progressCallback := func(event terraform.TerraformEvent) {
				if event.Phase != lastPhase {
					if lastPhase != "" {
						progress.Success(fmt.Sprintf("%s complete", lastPhase))
					}
					lastPhase = event.Phase
					progress.Start(event.Phase)
				}
				progress.Update(event.Action, event.Resource, event.ElapsedTime, event.Completed, event.Total)
			}

			if err := manager.DestroyWithProgress(ctx, state.ID, progressCallback); err != nil {
				progress.Fail(fmt.Sprintf("Failed: %v", err))
			} else {
				progress.Success(fmt.Sprintf("%s complete", lastPhase))
				fmt.Printf("   %s Destroyed\n", color.GreenString("✓"))
			}
		}

		return nil
	}

	// Single infrastructure
	if len(args) == 0 {
		return fmt.Errorf("infrastructure ID required (or use --expired/--all)")
	}

	id := args[0]
	state, err := manager.GetState(id)
	if err != nil {
		return fmt.Errorf("infrastructure not found: %s", id)
	}

	if !force {
		fmt.Printf("Will destroy infrastructure: %s\n", state.ID)
		fmt.Printf("  Provider: %s\n", state.Provider)
		fmt.Printf("  Region: %s\n", state.Region)
		if state.Outputs != nil && state.Outputs.RedisHostname != "" {
			fmt.Printf("  Redis: %s\n", state.Outputs.RedisHostname)
		}
		fmt.Print("\nContinue? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	fmt.Printf("\n🗑️  Destroying %s...\n", id)

	// Create progress display
	progress := NewInfraProgressDisplay()
	var lastPhase string

	progressCallback := func(event terraform.TerraformEvent) {
		if event.Phase != lastPhase {
			if lastPhase != "" {
				progress.Success(fmt.Sprintf("%s complete", lastPhase))
			}
			lastPhase = event.Phase
			progress.Start(event.Phase)
		}
		progress.Update(event.Action, event.Resource, event.ElapsedTime, event.Completed, event.Total)
	}

	startTime := time.Now()
	if err := manager.DestroyWithProgress(ctx, id, progressCallback); err != nil {
		progress.Fail(fmt.Sprintf("Destroy failed: %v", err))
		return err
	}

	progress.Success(fmt.Sprintf("%s complete", lastPhase))
	duration := time.Since(startTime)

	fmt.Printf("\n%s Infrastructure destroyed successfully in %s.\n",
		color.GreenString("✅"), duration.Round(time.Second))
	return nil
}

func runInfraList(cmd *cobra.Command, args []string) error {
	manager, err := getInfraManager()
	if err != nil {
		return err
	}

	states, err := manager.ListInfrastructure()
	if err != nil {
		return err
	}

	if len(states) == 0 {
		fmt.Println("No infrastructure found.")
		fmt.Println("Use 'redismeter infra up' to provision new infrastructure.")
		return nil
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                         RedisMeter Infrastructure                                ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  %-35s  %-10s  %-10s  %-12s  ║\n", "ID", "STATUS", "PROVIDER", "CREATED")
	fmt.Println("╠══════════════════════════════════════════════════════════════════════════════════╣")

	for _, state := range states {
		statusIcon := getStatusIcon(state.Status)
		created := state.CreatedAt.Format("2006-01-02")
		fmt.Printf("║  %-35s  %s %-8s  %-10s  %-12s  ║\n",
			truncateStr(state.ID, 35),
			statusIcon,
			state.Status,
			state.Provider,
			created)
	}
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	return nil
}

func runInfraStatus(cmd *cobra.Command, args []string) error {
	manager, err := getInfraManager()
	if err != nil {
		return err
	}

	state, err := manager.GetState(args[0])
	if err != nil {
		return fmt.Errorf("infrastructure not found: %s", args[0])
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                   Infrastructure Details                         ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  ID:        %-52s  ║\n", state.ID)
	fmt.Printf("║  Name:      %-52s  ║\n", state.Name)
	fmt.Printf("║  Status:    %s %-50s  ║\n", getStatusIcon(state.Status), state.Status)
	fmt.Printf("║  Provider:  %-52s  ║\n", state.Provider)
	fmt.Printf("║  Region:    %-52s  ║\n", state.Region)
	fmt.Printf("║  Created:   %-52s  ║\n", state.CreatedAt.Format(time.RFC3339))
	fmt.Printf("║  Updated:   %-52s  ║\n", state.UpdatedAt.Format(time.RFC3339))
	if !state.ExpiresAt.IsZero() {
		fmt.Printf("║  Expires:   %-52s  ║\n", state.ExpiresAt.Format(time.RFC3339))
	}
	fmt.Println("╠══════════════════════════════════════════════════════════════════╣")

	if state.Outputs != nil {
		fmt.Println("║  Outputs:                                                        ║")
		if state.Outputs.RedisHostname != "" {
			fmt.Printf("║    Redis Host: %-49s  ║\n", state.Outputs.RedisHostname)
			fmt.Printf("║    Redis Port: %-49d  ║\n", state.Outputs.RedisPort)
		}
		if state.Outputs.ResourceGroupName != "" {
			fmt.Printf("║    Resource Group: %-45s  ║\n", state.Outputs.ResourceGroupName)
		}
		if len(state.Outputs.RunnerIPs) > 0 {
			fmt.Printf("║    Runner IPs: %-49s  ║\n", strings.Join(state.Outputs.RunnerIPs, ", "))
		}
	}

	if state.Error != "" {
		fmt.Println("╠══════════════════════════════════════════════════════════════════╣")
		fmt.Printf("║  Error: %-56s  ║\n", truncateStr(state.Error, 56))
	}

	fmt.Println("╠══════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Workspace: %-52s  ║\n", truncateStr(state.WorkspacePath, 52))
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	return nil
}

func runInfraRefresh(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	manager, err := getInfraManager()
	if err != nil {
		return err
	}

	fmt.Printf("🔄 Refreshing state for %s...\n", args[0])

	state, err := manager.RefreshState(ctx, args[0])
	if err != nil {
		return err
	}

	fmt.Println("✅ State refreshed.")
	if state.Outputs != nil && state.Outputs.RedisHostname != "" {
		fmt.Printf("   Redis: %s:%d\n", state.Outputs.RedisHostname, state.Outputs.RedisPort)
	}

	return nil
}

func getStatusIcon(status string) string {
	switch status {
	case "ready":
		return "✅"
	case "provisioning", "destroying":
		return "⏳"
	case "failed":
		return "❌"
	case "destroyed":
		return "🗑️"
	default:
		return "⚪"
	}
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
