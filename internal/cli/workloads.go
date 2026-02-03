// Package cli implements the command-line interface for RedisMeter.
package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/tfindelkind-redis/redismeter/internal/workload"
)

var workloadsCmd = &cobra.Command{
	Use:   "workloads",
	Short: "List available workloads",
	Long:  `List all available built-in workload definitions.`,
	RunE:  listWorkloads,
}

func init() {
	rootCmd.AddCommand(workloadsCmd)
}

func listWorkloads(cmd *cobra.Command, args []string) error {
	workloads := workload.DefaultRegistry.ListAll()

	// Sort by name
	sort.Slice(workloads, func(i, j int) bool {
		return workloads[i].Name < workloads[j].Name
	})

	fmt.Println()
	fmt.Println("📦 Available Workloads")
	fmt.Println()

	for _, w := range workloads {
		fmt.Printf("  %s\n", w.Name)
		fmt.Printf("    %s\n", w.Description)
		fmt.Printf("    Type: %s | Threads: %d | Clients: %d | Duration: %s\n",
			w.Type, w.Threads, w.Clients, w.Duration)
		fmt.Println()
	}

	fmt.Println("Use a workload with: redismeter run <workload-name> --target <redis-url>")
	fmt.Println()

	return nil
}
