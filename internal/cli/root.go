// Package cli implements the command-line interface for RedisMeter.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tfindelkind-redis/redismeter/internal/storage"
)

var (
	cfgFile     string
	verbose     bool
	storageType string
	storagePath string
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "redismeter",
	Short: "Redis performance benchmarking and baselining tool",
	Long: `RedisMeter is a Redis-native performance benchmarking and baselining framework
that enables engineers to reliably measure, compare, and understand Redis
performance at scale.

Built on memtier_benchmark, it provides orchestration, result aggregation,
baseline management, and higher-level workflows for repeatable benchmarks.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.redismeter.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVar(&storageType, "storage", "", "storage backend (file, sqlite)")
	rootCmd.PersistentFlags().StringVar(&storagePath, "storage-path", "", "storage path (directory for file, db path for sqlite)")

	// Bind flags to viper
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("storage.type", rootCmd.PersistentFlags().Lookup("storage"))
	viper.BindPFlag("storage.path", rootCmd.PersistentFlags().Lookup("storage-path"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search for config in home directory
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName(".redismeter")
	}

	// Read environment variables with REDISMETER_ prefix
	viper.SetEnvPrefix("REDISMETER")
	viper.AutomaticEnv()

	// Read config file if it exists
	if err := viper.ReadInConfig(); err == nil {
		if verbose {
			fmt.Println("Using config file:", viper.ConfigFileUsed())
		}
	}
}

// getStorageConfig returns the storage configuration from viper.
func getStorageConfig() (storageType string, storagePath string) {
	return viper.GetString("storage.type"), viper.GetString("storage.path")
}

// newRunStorage creates a new storage instance based on the configuration.
func newRunStorage(storageType, storagePath string) (storage.RunStorage, error) {
	config := storage.StorageConfig{
		Type: storage.StorageType(storageType),
		Path: storagePath,
	}
	return storage.NewStorage(config)
}
