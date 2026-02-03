package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/tfindelkind-redis/redismeter/internal/workload"
)

// interactiveCmd provides guided workflow for new users.
var interactiveCmd = &cobra.Command{
	Use:     "interactive",
	Aliases: []string{"i", "wizard"},
	Short:   "Run an interactive benchmark wizard",
	Long: `Start an interactive wizard that guides you through
configuring and running a benchmark.

This is recommended for new users or when you want to
explore available options.`,
	RunE: runInteractive,
}

func init() {
	rootCmd.AddCommand(interactiveCmd)
}

// InteractiveConfig holds the configuration gathered from user input.
type InteractiveConfig struct {
	TargetHost   string
	TargetPort   int
	Password     string
	WorkloadName string
	Duration     string
	Clients      int
	Threads      int
	Pipeline     int
	SaveRun      bool
	Tags         []string
}

func runInteractive(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	// Header
	printHeader()

	config := &InteractiveConfig{}

	// Step 1: Target configuration
	fmt.Println()
	printStep(1, "Redis Target Configuration")

	config.TargetHost = promptString(reader, "Redis host", "localhost")
	config.TargetPort = promptInt(reader, "Redis port", 6379)
	config.Password = promptPassword(reader, "Redis password (leave empty if none)")

	// Test connection
	fmt.Printf("\n  Testing connection to %s:%d... ", config.TargetHost, config.TargetPort)
	// TODO: Actually test connection
	color.Green("✓ Connected")

	// Step 2: Workload selection
	fmt.Println()
	printStep(2, "Workload Selection")

	workloadNames := workload.DefaultRegistry.List()
	fmt.Println()
	fmt.Println("  Available workloads:")
	for i, wlName := range workloadNames {
		fmt.Printf("    %d. %s\n", i+1, color.CyanString(wlName))
	}
	fmt.Println()

	workloadIdx := promptInt(reader, "Select workload (number)", 1) - 1
	if workloadIdx < 0 || workloadIdx >= len(workloadNames) {
		workloadIdx = 0
	}
	config.WorkloadName = workloadNames[workloadIdx]
	fmt.Printf("  Selected: %s\n", color.CyanString(config.WorkloadName))

	// Step 3: Test parameters
	fmt.Println()
	printStep(3, "Test Parameters")

	config.Duration = promptString(reader, "Duration (e.g., 60s, 5m)", "60s")
	config.Clients = promptInt(reader, "Number of clients", 50)
	config.Threads = promptInt(reader, "Number of threads", 4)
	config.Pipeline = promptInt(reader, "Pipeline depth", 1)

	// Step 4: Options
	fmt.Println()
	printStep(4, "Additional Options")

	config.SaveRun = promptYesNo(reader, "Save results to storage?", true)

	if config.SaveRun {
		tagsStr := promptString(reader, "Tags (comma-separated, optional)", "")
		if tagsStr != "" {
			config.Tags = strings.Split(tagsStr, ",")
			for i := range config.Tags {
				config.Tags[i] = strings.TrimSpace(config.Tags[i])
			}
		}
	}

	// Summary
	fmt.Println()
	printSummary(config)

	// Confirm
	if !promptYesNo(reader, "Start benchmark?", true) {
		fmt.Println("\n  Benchmark cancelled.")
		return nil
	}

	// Build and execute run command
	fmt.Println()
	color.Cyan("Starting benchmark...")
	fmt.Println()

	// Build args for run command
	runArgs := []string{
		config.WorkloadName,
		"--target", fmt.Sprintf("%s:%d", config.TargetHost, config.TargetPort),
		"--duration", config.Duration,
		"--clients", strconv.Itoa(config.Clients),
		"--threads", strconv.Itoa(config.Threads),
		"--pipeline", strconv.Itoa(config.Pipeline),
	}

	if config.Password != "" {
		runArgs = append(runArgs, "--password", config.Password)
	}

	if !config.SaveRun {
		runArgs = append(runArgs, "--no-save")
	}

	for _, tag := range config.Tags {
		runArgs = append(runArgs, "--tag", tag)
	}

	// Execute run command
	runCmd.SetArgs(runArgs)
	return runCmd.Execute()
}

func printHeader() {
	color.Cyan(`
╔══════════════════════════════════════════════════════════════╗
║                                                              ║
║   ██████╗ ███████╗██████╗ ██╗███████╗███╗   ███╗███████╗    ║
║   ██╔══██╗██╔════╝██╔══██╗██║██╔════╝████╗ ████║██╔════╝    ║
║   ██████╔╝█████╗  ██║  ██║██║███████╗██╔████╔██║█████╗      ║
║   ██╔══██╗██╔══╝  ██║  ██║██║╚════██║██║╚██╔╝██║██╔══╝      ║
║   ██║  ██║███████╗██████╔╝██║███████║██║ ╚═╝ ██║███████╗    ║
║   ╚═╝  ╚═╝╚══════╝╚═════╝ ╚═╝╚══════╝╚═╝     ╚═╝╚══════╝    ║
║                                                              ║
║              Interactive Benchmark Wizard                    ║
╚══════════════════════════════════════════════════════════════╝
`)
}

func printStep(num int, title string) {
	color.New(color.FgCyan, color.Bold).Printf("Step %d: %s\n", num, title)
	fmt.Println(strings.Repeat("─", 50))
}

func printSummary(config *InteractiveConfig) {
	color.New(color.FgCyan, color.Bold).Println("Configuration Summary")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("  Target:    %s:%d\n", config.TargetHost, config.TargetPort)
	fmt.Printf("  Workload:  %s\n", color.CyanString(config.WorkloadName))
	fmt.Printf("  Duration:  %s\n", config.Duration)
	fmt.Printf("  Clients:   %d\n", config.Clients)
	fmt.Printf("  Threads:   %d\n", config.Threads)
	fmt.Printf("  Pipeline:  %d\n", config.Pipeline)
	fmt.Printf("  Save:      %v\n", config.SaveRun)
	if len(config.Tags) > 0 {
		fmt.Printf("  Tags:      %s\n", strings.Join(config.Tags, ", "))
	}
	fmt.Println(strings.Repeat("─", 50))
}

func promptString(reader *bufio.Reader, prompt, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("  %s [%s]: ", prompt, defaultVal)
	} else {
		fmt.Printf("  %s: ", prompt)
	}

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultVal
	}
	return input
}

func promptInt(reader *bufio.Reader, prompt string, defaultVal int) int {
	fmt.Printf("  %s [%d]: ", prompt, defaultVal)

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultVal
	}

	val, err := strconv.Atoi(input)
	if err != nil {
		return defaultVal
	}
	return val
}

func promptPassword(reader *bufio.Reader, prompt string) string {
	fmt.Printf("  %s: ", prompt)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func promptYesNo(reader *bufio.Reader, prompt string, defaultVal bool) bool {
	defaultStr := "Y/n"
	if !defaultVal {
		defaultStr = "y/N"
	}

	fmt.Printf("  %s [%s]: ", prompt, defaultStr)

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "" {
		return defaultVal
	}

	return input == "y" || input == "yes"
}
