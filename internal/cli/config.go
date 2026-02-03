package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// configCmd represents the config command.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage RedisMeter configuration",
	Long: `View and modify RedisMeter configuration settings.

The config command allows you to view and update various configuration
options including storage backend settings, default workloads, and more.`,
}

// configShowCmd shows current configuration.
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  `Display all current RedisMeter configuration settings.`,
	Example: `  # Show all configuration
  redismeter config show

  # Show configuration with verbose output
  redismeter config show -v`,
	RunE: runConfigShow,
}

// configSetCmd sets a configuration value.
var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a configuration value.

Supported configuration keys:
  storage.type     - Storage backend type (file, sqlite)
  storage.path     - Storage path
  verbose          - Enable verbose output
  memtier.path     - Path to memtier_benchmark binary`,
	Example: `  # Set storage type to SQLite
  redismeter config set storage.type sqlite

  # Set storage path
  redismeter config set storage.path /data/redismeter

  # Set memtier path
  redismeter config set memtier.path /usr/local/bin/memtier_benchmark`,
	Args: cobra.ExactArgs(2),
	RunE: runConfigSet,
}

// configGetCmd gets a configuration value.
var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Long:  `Get a specific configuration value.`,
	Example: `  # Get storage type
  redismeter config get storage.type

  # Get storage path
  redismeter config get storage.path`,
	Args: cobra.ExactArgs(1),
	RunE: runConfigGet,
}

// configInitCmd initializes configuration file.
var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration file",
	Long: `Initialize a new RedisMeter configuration file.

This command creates a new configuration file with default values
if one doesn't already exist.`,
	Example: `  # Initialize default config file
  redismeter config init

  # Initialize config file at specific location
  redismeter config init --config /etc/redismeter/config.yaml`,
	RunE: runConfigInit,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configInitCmd)

	configInitCmd.Flags().Bool("force", false, "Overwrite existing configuration file")
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	// Get all configuration
	allSettings := viper.AllSettings()

	// Print config file location
	if configFile := viper.ConfigFileUsed(); configFile != "" {
		fmt.Printf("Config file: %s\n\n", configFile)
	} else {
		fmt.Println("Config file: (not found - using defaults)")
		fmt.Println()
	}

	// Print all settings as YAML
	yamlData, err := yaml.Marshal(allSettings)
	if err != nil {
		return fmt.Errorf("failed to format configuration: %w", err)
	}

	fmt.Println("Current configuration:")
	fmt.Println(string(yamlData))

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	// Validate the key
	validKeys := map[string]bool{
		"storage.type": true,
		"storage.path": true,
		"verbose":      true,
		"memtier.path": true,
	}

	if !validKeys[key] {
		return fmt.Errorf("invalid configuration key: %s\nUse 'redismeter config show' to see valid keys", key)
	}

	// Set the value in viper
	viper.Set(key, value)

	// Ensure config file exists
	configPath := getConfigPath()
	if err := ensureConfigFile(configPath); err != nil {
		return fmt.Errorf("failed to ensure config file: %w", err)
	}

	// Write config
	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to write configuration: %w", err)
	}

	fmt.Printf("Set %s = %s\n", key, value)
	return nil
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := viper.Get(key)

	if value == nil {
		fmt.Printf("%s: (not set)\n", key)
	} else {
		fmt.Printf("%s: %v\n", key, value)
	}

	return nil
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")

	configPath := getConfigPath()

	// Check if file already exists
	if _, err := os.Stat(configPath); err == nil && !force {
		return fmt.Errorf("configuration file already exists: %s\nUse --force to overwrite", configPath)
	}

	// Set default values
	setDefaultConfig()

	// Ensure directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write config file
	if err := viper.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}

	fmt.Printf("Configuration file created: %s\n", configPath)
	return nil
}

// getConfigPath returns the path to the config file.
func getConfigPath() string {
	if cfgFile != "" {
		return cfgFile
	}

	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".redismeter.yaml")
}

// ensureConfigFile ensures the config file exists.
func ensureConfigFile(path string) error {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create directory if needed
		configDir := filepath.Dir(path)
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return err
		}

		// Create empty config file
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		f.Close()
	}

	return nil
}

// setDefaultConfig sets default configuration values.
func setDefaultConfig() {
	home, _ := os.UserHomeDir()

	viper.SetDefault("storage.type", "file")
	viper.SetDefault("storage.path", filepath.Join(home, ".redismeter"))
	viper.SetDefault("verbose", false)
}
