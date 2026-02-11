package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/tfindelkind-redis/redismeter/internal/infraprofile"
)

var profilesCmd = &cobra.Command{
	Use:     "profiles",
	Aliases: []string{"profile", "infra-profiles"},
	Short:   "Manage infrastructure profiles",
	Long: `Manage infrastructure profiles for benchmark environments.

Profiles define the compute, memory, network, and storage characteristics
of the infrastructure used for benchmarking. They can be automatically
captured from cloud providers or manually defined.`,
}

var profilesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all infrastructure profiles",
	RunE:  runProfilesList,
}

var profilesShowCmd = &cobra.Command{
	Use:   "show [profile-id]",
	Short: "Show detailed profile information",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfilesShow,
}

var profilesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new infrastructure profile",
	Long:  `Create a new infrastructure profile from a JSON file or interactively.`,
	RunE:  runProfilesCreate,
}

var profilesDeleteCmd = &cobra.Command{
	Use:   "delete [profile-id]",
	Short: "Delete an infrastructure profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfilesDelete,
}

var profilesExportCmd = &cobra.Command{
	Use:   "export [output-file]",
	Short: "Export profiles to JSON",
	RunE:  runProfilesExport,
}

var profilesImportCmd = &cobra.Command{
	Use:   "import [input-file]",
	Short: "Import profiles from JSON",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfilesImport,
}

var profilesStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show profile statistics",
	RunE:  runProfilesStats,
}

// Flags
var (
	profilesProvider string
	profilesTag      string
	profilesFile     string
	profilesForce    bool
	profilesFormat   string
	profilesVerbose  bool
)

func init() {
	rootCmd.AddCommand(profilesCmd)
	profilesCmd.AddCommand(profilesListCmd)
	profilesCmd.AddCommand(profilesShowCmd)
	profilesCmd.AddCommand(profilesCreateCmd)
	profilesCmd.AddCommand(profilesDeleteCmd)
	profilesCmd.AddCommand(profilesExportCmd)
	profilesCmd.AddCommand(profilesImportCmd)
	profilesCmd.AddCommand(profilesStatsCmd)

	// List flags
	profilesListCmd.Flags().StringVarP(&profilesProvider, "provider", "p", "", "Filter by provider (aws, gcp, azure, local)")
	profilesListCmd.Flags().StringVarP(&profilesTag, "tag", "t", "", "Filter by tag")
	profilesListCmd.Flags().BoolVarP(&profilesVerbose, "verbose", "v", false, "Show detailed output")

	// Create flags
	profilesCreateCmd.Flags().StringVarP(&profilesFile, "file", "f", "", "JSON file with profile definition")

	// Delete flags
	profilesDeleteCmd.Flags().BoolVar(&profilesForce, "force", false, "Skip confirmation")

	// Export flags
	profilesExportCmd.Flags().StringVarP(&profilesFormat, "format", "f", "json", "Export format (json)")
	profilesExportCmd.Flags().StringVarP(&profilesProvider, "provider", "p", "", "Filter by provider")
}

func getProfileStore() (*infraprofile.FileStore, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	storePath := filepath.Join(homeDir, ".redismeter", "infra-profiles")

	// Ensure directory exists
	if err := os.MkdirAll(storePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create profile directory: %w", err)
	}

	return infraprofile.NewFileStore(storePath)
}

func runProfilesList(cmd *cobra.Command, args []string) error {
	store, err := getProfileStore()
	if err != nil {
		return err
	}
	defer store.Close()

	ctx := context.Background()
	var profiles []*infraprofile.Profile

	if profilesProvider != "" {
		profiles, err = store.ListByProvider(ctx, infraprofile.Provider(profilesProvider))
	} else if profilesTag != "" {
		profiles, err = store.ListByTag(ctx, profilesTag)
	} else {
		profiles, err = store.List(ctx)
	}

	if err != nil {
		return fmt.Errorf("failed to list profiles: %w", err)
	}

	if len(profiles) == 0 {
		fmt.Println("No profiles found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if profilesVerbose {
		fmt.Fprintln(w, "ID\tNAME\tPROVIDER\tTAGS\tUSED\tCREATED")
		fmt.Fprintln(w, strings.Repeat("-", 80))
		for _, p := range profiles {
			tags := strings.Join(p.Tags, ", ")
			if len(tags) > 20 {
				tags = tags[:17] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n",
				truncateID(p.ID, 8),
				truncate(p.Name, 20),
				p.Provider,
				tags,
				p.UseCount,
				p.CreatedAt.Format("2006-01-02"),
			)
		}
	} else {
		fmt.Fprintln(w, "ID\tNAME\tPROVIDER\tTAGS")
		fmt.Fprintln(w, strings.Repeat("-", 60))
		for _, p := range profiles {
			tags := strings.Join(p.Tags, ", ")
			if len(tags) > 20 {
				tags = tags[:17] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				truncateID(p.ID, 8),
				truncate(p.Name, 20),
				p.Provider,
				tags,
			)
		}
	}

	w.Flush()
	fmt.Printf("\nTotal: %d profiles\n", len(profiles))

	return nil
}

func runProfilesShow(cmd *cobra.Command, args []string) error {
	store, err := getProfileStore()
	if err != nil {
		return err
	}
	defer store.Close()

	ctx := context.Background()
	profile, err := store.Get(ctx, args[0])
	if err != nil {
		return fmt.Errorf("failed to get profile: %w", err)
	}
	if profile == nil {
		// Try by name as fallback
		profile, err = store.GetByName(ctx, args[0])
		if err != nil {
			return fmt.Errorf("failed to get profile by name: %w", err)
		}
		if profile == nil {
			return fmt.Errorf("profile not found: %s", args[0])
		}
	}

	fmt.Printf("Profile: %s\n", profile.Name)
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("ID:          %s\n", profile.ID)
	fmt.Printf("Provider:    %s\n", profile.Provider)
	fmt.Printf("Created:     %s\n", profile.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Updated:     %s\n", profile.UpdatedAt.Format(time.RFC3339))
	fmt.Printf("Use Count:   %d\n", profile.UseCount)

	if profile.Description != "" {
		fmt.Printf("Description: %s\n", profile.Description)
	}

	if len(profile.Tags) > 0 {
		fmt.Printf("Tags:        %s\n", strings.Join(profile.Tags, ", "))
	}

	// Print provider-specific configuration
	fmt.Println("\nConfiguration:")
	switch profile.Provider {
	case infraprofile.ProviderAzure:
		if profile.Config.Azure != nil {
			cfg := profile.Config.Azure
			fmt.Printf("  Location:        %s\n", cfg.Location)
			fmt.Printf("  SKU:             %s\n", cfg.SKU)
			fmt.Printf("  High Availability: %t\n", cfg.HighAvailability)
			if cfg.PersistenceType != "" {
				fmt.Printf("  Persistence:     %s\n", cfg.PersistenceType)
			}
			if len(cfg.Modules) > 0 {
				fmt.Printf("  Modules:         %s\n", strings.Join(cfg.Modules, ", "))
			}
			if cfg.VMSize != "" {
				fmt.Printf("  VM Size:         %s\n", cfg.VMSize)
			}
			if cfg.VMCount > 0 {
				fmt.Printf("  VM Count:        %d\n", cfg.VMCount)
			}
		}
	case infraprofile.ProviderAWS:
		if profile.Config.AWS != nil {
			cfg := profile.Config.AWS
			fmt.Printf("  Region:        %s\n", cfg.Region)
			fmt.Printf("  Node Type:     %s\n", cfg.NodeType)
			fmt.Printf("  Nodes:         %d\n", cfg.NumCacheNodes)
		}
	case infraprofile.ProviderGCP:
		if profile.Config.GCP != nil {
			cfg := profile.Config.GCP
			fmt.Printf("  Region:        %s\n", cfg.Region)
			fmt.Printf("  Tier:          %s\n", cfg.Tier)
			fmt.Printf("  Memory (GB):   %d\n", cfg.MemorySizeGB)
		}
	case infraprofile.ProviderLocal:
		if profile.Config.Local != nil {
			cfg := profile.Config.Local
			fmt.Printf("  Host:          %s\n", cfg.Host)
			fmt.Printf("  Port:          %d\n", cfg.Port)
		}
	}

	if profile.LastUsedAt != nil {
		fmt.Printf("\nLast Used: %s\n", profile.LastUsedAt.Format(time.RFC3339))
	}

	return nil
}

func runProfilesCreate(cmd *cobra.Command, args []string) error {
	store, err := getProfileStore()
	if err != nil {
		return err
	}
	defer store.Close()

	var profile infraprofile.Profile

	if profilesFile != "" {
		data, err := os.ReadFile(profilesFile)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		if err := json.Unmarshal(data, &profile); err != nil {
			return fmt.Errorf("failed to parse JSON: %w", err)
		}
	} else {
		return fmt.Errorf("please provide a profile file with --file")
	}

	// Validate and set defaults
	if profile.Name == "" {
		return fmt.Errorf("profile name is required")
	}
	if profile.Provider == "" {
		return fmt.Errorf("provider is required")
	}

	ctx := context.Background()
	if err := store.Create(ctx, &profile); err != nil {
		return fmt.Errorf("failed to create profile: %w", err)
	}

	fmt.Printf("Created profile: %s (ID: %s)\n", profile.Name, profile.ID)
	return nil
}

func runProfilesDelete(cmd *cobra.Command, args []string) error {
	store, err := getProfileStore()
	if err != nil {
		return err
	}
	defer store.Close()

	profileID := args[0]
	ctx := context.Background()

	// Get profile first to show name
	profile, err := store.Get(ctx, profileID)
	if err != nil {
		return fmt.Errorf("profile not found: %w", err)
	}

	if !profilesForce {
		fmt.Printf("Delete profile '%s' (%s)? [y/N]: ", profile.Name, profile.ID)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	if err := store.Delete(ctx, profileID); err != nil {
		return fmt.Errorf("failed to delete profile: %w", err)
	}

	fmt.Printf("Deleted profile: %s\n", profileID)
	return nil
}

func runProfilesExport(cmd *cobra.Command, args []string) error {
	store, err := getProfileStore()
	if err != nil {
		return err
	}
	defer store.Close()

	ctx := context.Background()

	var profiles []*infraprofile.Profile
	if profilesProvider != "" {
		profiles, err = store.ListByProvider(ctx, infraprofile.Provider(profilesProvider))
	} else {
		profiles, err = store.List(ctx)
	}

	if err != nil {
		return fmt.Errorf("failed to list profiles: %w", err)
	}

	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal profiles: %w", err)
	}

	// Determine output
	if len(args) > 0 {
		if err := os.WriteFile(args[0], data, 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
		fmt.Printf("Exported %d profiles to %s\n", len(profiles), args[0])
	} else {
		fmt.Println(string(data))
	}

	return nil
}

func runProfilesImport(cmd *cobra.Command, args []string) error {
	store, err := getProfileStore()
	if err != nil {
		return err
	}
	defer store.Close()

	data, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var profiles []*infraprofile.Profile
	if err := json.Unmarshal(data, &profiles); err != nil {
		// Try single profile
		var profile infraprofile.Profile
		if err := json.Unmarshal(data, &profile); err != nil {
			return fmt.Errorf("failed to parse JSON: %w", err)
		}
		profiles = []*infraprofile.Profile{&profile}
	}

	ctx := context.Background()
	imported := 0
	for _, p := range profiles {
		if err := store.Create(ctx, p); err != nil {
			fmt.Printf("Warning: failed to import profile %s: %v\n", p.Name, err)
			continue
		}
		imported++
	}

	fmt.Printf("Imported %d profiles\n", imported)
	return nil
}

func runProfilesStats(cmd *cobra.Command, args []string) error {
	store, err := getProfileStore()
	if err != nil {
		return err
	}
	defer store.Close()

	ctx := context.Background()
	stats, err := store.GetStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get stats: %w", err)
	}

	fmt.Println("Profile Statistics")
	fmt.Println(strings.Repeat("=", 40))
	fmt.Printf("Total Profiles: %d\n", stats.TotalProfiles)

	if len(stats.ByProvider) > 0 {
		fmt.Println("\nBy Provider:")
		for provider, count := range stats.ByProvider {
			fmt.Printf("  %-10s %d\n", provider, count)
		}
	}

	if stats.MostUsed != nil {
		fmt.Printf("\nMost Used Profile:     %s\n", stats.MostUsed.Name)
	}
	if stats.RecentlyUsed != nil {
		fmt.Printf("Recently Used Profile: %s\n", stats.RecentlyUsed.Name)
	}

	return nil
}

// Helper function
func truncateID(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
