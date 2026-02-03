// Package cli implements the command-line interface for RedisMeter.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of RedisMeter",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("RedisMeter v0.1.0-dev")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
