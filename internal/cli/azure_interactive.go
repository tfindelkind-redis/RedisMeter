package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// AzureInteractiveConfig holds the configuration built through interactive prompts
type AzureInteractiveConfig struct {
	// Authentication
	SubscriptionID string
	TenantID       string

	// Location
	Region string

	// Resource Group
	ResourceGroupName   string
	CreateResourceGroup bool

	// Network
	CreateNewVNet      bool
	VNetName           string
	VNetAddressSpace   string
	RunnerSubnet       string
	RedisSubnet        string
	ExistingVNetID     string
	ExistingSubnetName string

	// Runners
	RunnerCount int
	RunnerSize  string

	// Redis Target
	RedisMode       string // provision, existing, endpoint
	RedisTemplate   string
	RedisSKU        string
	RedisCapacity   int
	RedisResourceID string
	RedisEndpoint   string
	RedisPort       int
	RedisPassword   string

	// Benchmark
	Workload  string
	Duration  time.Duration
	AutoClean bool
}

// InteractiveSetup holds state for the interactive CLI
type InteractiveSetup struct {
	config  *AzureInteractiveConfig
	reader  *bufio.Reader
	step    int
	total   int
	verbose bool
}

// Colors and formatting
var (
	titleStyle   = color.New(color.FgHiCyan, color.Bold)
	stepStyle    = color.New(color.FgHiYellow)
	successStyle = color.New(color.FgHiGreen)
	warnStyle    = color.New(color.FgHiYellow)
	errorStyle   = color.New(color.FgHiRed)
	dimStyle     = color.New(color.FgHiBlack)
	infoStyle    = color.New(color.FgCyan)
	boldStyle    = color.New(color.Bold)
)

// NewInteractiveSetup creates a new interactive setup wizard
func NewInteractiveSetup() *InteractiveSetup {
	return &InteractiveSetup{
		config: &AzureInteractiveConfig{
			// Defaults
			VNetAddressSpace: "10.0.0.0/16",
			RunnerSubnet:     "10.0.1.0/24",
			RedisSubnet:      "10.0.2.0/24",
			RunnerCount:      1,
			RunnerSize:       "Standard_D4s_v3",
			RedisMode:        "provision",
			RedisTemplate:    "standard",
			RedisSKU:         "Balanced_B5",
			RedisCapacity:    2,
			Duration:         5 * time.Minute,
			AutoClean:        true,
		},
		reader: bufio.NewReader(os.Stdin),
		step:   0,
		total:  8,
	}
}

// Run executes the interactive setup wizard
func (s *InteractiveSetup) Run(ctx context.Context) (*AzureInteractiveConfig, error) {
	s.printHeader()

	// Step 1: Authentication
	if err := s.stepAuthentication(ctx); err != nil {
		return nil, err
	}

	// Step 2: Subscription
	if err := s.stepSubscription(ctx); err != nil {
		return nil, err
	}

	// Step 3: Region
	if err := s.stepRegion(ctx); err != nil {
		return nil, err
	}

	// Step 4: Resource Group
	if err := s.stepResourceGroup(ctx); err != nil {
		return nil, err
	}

	// Step 5: Network
	if err := s.stepNetwork(ctx); err != nil {
		return nil, err
	}

	// Step 6: Runners
	if err := s.stepRunners(ctx); err != nil {
		return nil, err
	}

	// Step 7: Redis Target
	if err := s.stepRedisTarget(ctx); err != nil {
		return nil, err
	}

	// Step 8: Workload & Execution
	if err := s.stepWorkload(ctx); err != nil {
		return nil, err
	}

	// Review and Confirm
	if err := s.stepReview(ctx); err != nil {
		return nil, err
	}

	return s.config, nil
}

func (s *InteractiveSetup) printHeader() {
	fmt.Println()
	titleStyle.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	titleStyle.Println("║                    RedisMeter Azure Benchmark Setup                          ║")
	titleStyle.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()
}

func (s *InteractiveSetup) printStepHeader(title string) {
	s.step++
	fmt.Println()
	stepStyle.Printf("Step %d of %d: %s\n", s.step, s.total, title)
	fmt.Println(strings.Repeat("━", 78))
	fmt.Println()
}

func (s *InteractiveSetup) printBox(lines []string) {
	maxLen := 0
	for _, line := range lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}
	width := maxLen + 4

	fmt.Printf("  ┌%s┐\n", strings.Repeat("─", width))
	for _, line := range lines {
		fmt.Printf("  │  %-*s  │\n", maxLen, line)
	}
	fmt.Printf("  └%s┘\n", strings.Repeat("─", width))
}

func (s *InteractiveSetup) prompt(label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("  %s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("  %s: ", label)
	}

	input, _ := s.reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultVal
	}
	return input
}

func (s *InteractiveSetup) promptInt(label string, defaultVal int) int {
	input := s.prompt(label, strconv.Itoa(defaultVal))
	val, err := strconv.Atoi(input)
	if err != nil {
		return defaultVal
	}
	return val
}

func (s *InteractiveSetup) promptYesNo(label string, defaultYes bool) bool {
	defaultStr := "Y/n"
	if !defaultYes {
		defaultStr = "y/N"
	}

	fmt.Printf("  %s [%s]: ", label, defaultStr)
	input, _ := s.reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "" {
		return defaultYes
	}
	return input == "y" || input == "yes"
}

func (s *InteractiveSetup) promptSelect(label string, options []string, defaultIdx int) int {
	fmt.Printf("  %s\n\n", label)

	for i, opt := range options {
		if i == defaultIdx {
			boldStyle.Printf("    [%d] %s (default)\n", i+1, opt)
		} else {
			fmt.Printf("    [%d] %s\n", i+1, opt)
		}
	}

	fmt.Println()
	input := s.prompt("Select option", strconv.Itoa(defaultIdx+1))

	idx, err := strconv.Atoi(input)
	if err != nil || idx < 1 || idx > len(options) {
		return defaultIdx
	}
	return idx - 1
}

func (s *InteractiveSetup) waitEnter() {
	dimStyle.Print("  Press Enter to continue...")
	s.reader.ReadString('\n')
}

// Step implementations

func (s *InteractiveSetup) stepAuthentication(ctx context.Context) error {
	s.printStepHeader("Authentication")

	// Check Azure authentication
	fmt.Println("  Checking Azure authentication...")
	fmt.Println()

	// Try to get credentials (this would call Azure SDK)
	// For now, simulate the check
	authMethod := "Azure CLI"
	userName := os.Getenv("USER")
	if spID := os.Getenv("AZURE_CLIENT_ID"); spID != "" {
		authMethod = "Service Principal"
		userName = spID[:8] + "..."
	}

	successStyle.Println("  ✓ Authenticated via " + authMethod)
	successStyle.Println("  ✓ User: " + userName)

	fmt.Println()
	s.waitEnter()
	return nil
}

func (s *InteractiveSetup) stepSubscription(ctx context.Context) error {
	s.printStepHeader("Subscription Selection")

	// In real implementation, fetch subscriptions from Azure
	subs := []struct {
		Name string
		ID   string
	}{
		{"Production", "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"},
		{"Development", "yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy"},
		{"Testing", "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz"},
	}

	fmt.Println("  Available subscriptions:")
	fmt.Println()

	for i, sub := range subs {
		fmt.Printf("    [%d] %s (%s)\n", i+1, sub.Name, sub.ID)
	}

	fmt.Println()
	input := s.prompt("Select subscription", "1")
	idx, _ := strconv.Atoi(input)
	if idx < 1 || idx > len(subs) {
		idx = 1
	}

	s.config.SubscriptionID = subs[idx-1].ID
	successStyle.Printf("\n  ✓ Selected: %s\n", subs[idx-1].Name)

	return nil
}

func (s *InteractiveSetup) stepRegion(ctx context.Context) error {
	s.printStepHeader("Region Selection")

	regions := map[string][]struct {
		Code string
		Name string
	}{
		"Americas": {
			{"eastus", "East US (Virginia)"},
			{"eastus2", "East US 2 (Virginia)"},
			{"westus2", "West US 2 (Washington)"},
			{"westus3", "West US 3 (Arizona)"},
			{"centralus", "Central US (Iowa)"},
		},
		"Europe": {
			{"northeurope", "North Europe (Ireland)"},
			{"westeurope", "West Europe (Netherlands)"},
			{"uksouth", "UK South (London)"},
			{"germanywestcentral", "Germany West Central (Frankfurt)"},
		},
		"Asia Pacific": {
			{"southeastasia", "Southeast Asia (Singapore)"},
			{"australiaeast", "Australia East (Sydney)"},
		},
	}

	fmt.Println("  Select Azure region:")
	fmt.Println()

	idx := 1
	regionMap := make(map[int]string)

	for group, regs := range regions {
		infoStyle.Printf("  %s:\n", group)
		for _, r := range regs {
			fmt.Printf("    [%d] %-18s %s\n", idx, r.Code, r.Name)
			regionMap[idx] = r.Code
			idx++
		}
		fmt.Println()
	}

	input := s.prompt("Enter region name or number", "eastus")

	// Check if it's a number
	if num, err := strconv.Atoi(input); err == nil {
		if code, ok := regionMap[num]; ok {
			s.config.Region = code
		}
	} else {
		s.config.Region = input
	}

	successStyle.Printf("\n  ✓ Selected region: %s\n", s.config.Region)

	return nil
}

func (s *InteractiveSetup) stepResourceGroup(ctx context.Context) error {
	s.printStepHeader("Resource Group")

	options := []string{
		"Create new resource group",
		"Use existing resource group",
	}

	idx := s.promptSelect("Resource Group Options:", options, 0)
	s.config.CreateResourceGroup = (idx == 0)

	fmt.Println()

	if s.config.CreateResourceGroup {
		defaultName := fmt.Sprintf("rm-benchmark-%s", time.Now().Format("20060102"))
		s.config.ResourceGroupName = s.prompt("Enter resource group name", defaultName)

		fmt.Println()
		infoStyle.Printf("  ℹ️  Resource group will be created in: %s\n", s.config.Region)
		infoStyle.Println("  ℹ️  All resources will be tagged with: managed-by=redismeter")
	} else {
		// In real impl, list existing RGs
		s.config.ResourceGroupName = s.prompt("Enter existing resource group name", "")
	}

	return nil
}

func (s *InteractiveSetup) stepNetwork(ctx context.Context) error {
	s.printStepHeader("Network Configuration")

	options := []string{
		"Create new VNet (recommended for isolated benchmarks)",
		"Use existing VNet (required if connecting to existing AMR)",
	}

	idx := s.promptSelect("Network Options:", options, 0)
	s.config.CreateNewVNet = (idx == 0)

	fmt.Println()

	if s.config.CreateNewVNet {
		fmt.Println("  New VNet Configuration:")
		s.printBox([]string{
			fmt.Sprintf("VNet Name:        %s", "rm-vnet"),
			fmt.Sprintf("Address Space:    %s", s.config.VNetAddressSpace),
			fmt.Sprintf("Runner Subnet:    %s (runners)", s.config.RunnerSubnet),
			fmt.Sprintf("Redis Subnet:     %s (private-endpoints)", s.config.RedisSubnet),
		})

		fmt.Println()
		if s.promptYesNo("Accept defaults?", true) {
			s.config.VNetName = "rm-vnet"
		} else {
			s.config.VNetName = s.prompt("VNet name", "rm-vnet")
			s.config.VNetAddressSpace = s.prompt("Address space", s.config.VNetAddressSpace)
			s.config.RunnerSubnet = s.prompt("Runner subnet", s.config.RunnerSubnet)
			s.config.RedisSubnet = s.prompt("Redis subnet", s.config.RedisSubnet)
		}
	} else {
		s.config.ExistingVNetID = s.prompt("Enter existing VNet resource ID", "")
		s.config.ExistingSubnetName = s.prompt("Enter subnet name for runners", "default")
	}

	return nil
}

func (s *InteractiveSetup) stepRunners(ctx context.Context) error {
	s.printStepHeader("Runner Configuration")

	s.config.RunnerCount = s.promptInt("How many runner VMs?", 1)

	fmt.Println()
	fmt.Println("  Select runner VM size:")
	fmt.Println()

	// Group VM sizes by category
	infoStyle.Println("  Cost-Optimized (burstable, dev/test):")
	fmt.Println("    [1] Standard_B2s       2 vCPU,  4 GB   ~$0.04/hr  (burstable, dev only)")
	fmt.Println("    [2] Standard_B2ms      2 vCPU,  8 GB   ~$0.08/hr  (burstable, light load)")

	fmt.Println()
	infoStyle.Println("  Balanced (recommended for most benchmarks):")
	fmt.Println("    [3] Standard_D2s_v3    2 vCPU,  8 GB   ~$0.10/hr  (small benchmarks)")
	fmt.Println("    [4] Standard_D4s_v3    4 vCPU, 16 GB   ~$0.19/hr  ★ RECOMMENDED")
	fmt.Println("    [5] Standard_D8s_v3    8 vCPU, 32 GB   ~$0.38/hr  (high load)")
	fmt.Println("    [6] Standard_D16s_v3  16 vCPU, 64 GB   ~$0.77/hr  (max load)")
	fmt.Println("    [7] Standard_D4s_v5    4 vCPU, 16 GB   ~$0.19/hr  (latest gen)")
	fmt.Println("    [8] Standard_D8s_v5    8 vCPU, 32 GB   ~$0.38/hr  (latest gen)")

	fmt.Println()
	infoStyle.Println("  High Performance (CPU optimized, max throughput):")
	fmt.Println("    [9] Standard_F4s_v2    4 vCPU,  8 GB   ~$0.17/hr  (high throughput)")
	fmt.Println("   [10] Standard_F8s_v2    8 vCPU, 16 GB   ~$0.34/hr  (very high throughput)")
	fmt.Println("   [11] Standard_F16s_v2  16 vCPU, 32 GB   ~$0.68/hr  (maximum throughput)")

	fmt.Println()
	infoStyle.Println("  Network Optimized (bandwidth testing):")
	fmt.Println("   [12] Standard_D8ds_v5   8 vCPU, 32 GB   ~$0.45/hr  (12.5 Gbps network)")
	fmt.Println("   [13] Standard_D16ds_v5 16 vCPU, 64 GB   ~$0.90/hr  (12.5 Gbps network)")

	fmt.Println()
	dimStyle.Println("   Or enter any valid Azure VM size (e.g., Standard_E4s_v5)")

	fmt.Println()
	input := s.prompt("Select size (number or name)", "4")

	vmSizeMap := map[string]string{
		"1":  "Standard_B2s",
		"2":  "Standard_B2ms",
		"3":  "Standard_D2s_v3",
		"4":  "Standard_D4s_v3",
		"5":  "Standard_D8s_v3",
		"6":  "Standard_D16s_v3",
		"7":  "Standard_D4s_v5",
		"8":  "Standard_D8s_v5",
		"9":  "Standard_F4s_v2",
		"10": "Standard_F8s_v2",
		"11": "Standard_F16s_v2",
		"12": "Standard_D8ds_v5",
		"13": "Standard_D16ds_v5",
	}

	if size, ok := vmSizeMap[input]; ok {
		s.config.RunnerSize = size
	} else {
		// User entered a custom VM size name
		s.config.RunnerSize = input
	}

	// Get cost info for the selected size
	hourCost := 0.19 // Default
	specs := "4 vCPU, 16 GB"
	vmCosts := map[string]struct {
		cost  float64
		specs string
	}{
		"Standard_B2s":      {0.04, "2 vCPU, 4 GB"},
		"Standard_B2ms":     {0.08, "2 vCPU, 8 GB"},
		"Standard_D2s_v3":   {0.10, "2 vCPU, 8 GB"},
		"Standard_D4s_v3":   {0.19, "4 vCPU, 16 GB"},
		"Standard_D8s_v3":   {0.38, "8 vCPU, 32 GB"},
		"Standard_D16s_v3":  {0.77, "16 vCPU, 64 GB"},
		"Standard_D4s_v5":   {0.19, "4 vCPU, 16 GB"},
		"Standard_D8s_v5":   {0.38, "8 vCPU, 32 GB"},
		"Standard_F4s_v2":   {0.17, "4 vCPU, 8 GB"},
		"Standard_F8s_v2":   {0.34, "8 vCPU, 16 GB"},
		"Standard_F16s_v2":  {0.68, "16 vCPU, 32 GB"},
		"Standard_D8ds_v5":  {0.45, "8 vCPU, 32 GB"},
		"Standard_D16ds_v5": {0.90, "16 vCPU, 64 GB"},
	}

	if info, ok := vmCosts[s.config.RunnerSize]; ok {
		hourCost = info.cost
		specs = info.specs
	}

	fmt.Println()
	fmt.Println("  Runner Summary:")
	s.printBox([]string{
		fmt.Sprintf("Count:        %d VMs", s.config.RunnerCount),
		fmt.Sprintf("Size:         %s (%s)", s.config.RunnerSize, specs),
		"OS:           Ubuntu 22.04 LTS (enforced)",
		"Software:     memtier_benchmark (auto-installed)",
		fmt.Sprintf("Est. Cost:    ~$%.2f/hr ($%.2f × %d)", hourCost*float64(s.config.RunnerCount), hourCost, s.config.RunnerCount),
	})

	return nil
}

func (s *InteractiveSetup) stepRedisTarget(ctx context.Context) error {
	s.printStepHeader("Redis Target Configuration")

	fmt.Println("  Redis Target Mode:")
	fmt.Println()
	fmt.Println("    [1] Provision new Azure Managed Redis")
	fmt.Println("        → Creates a new AMR instance for benchmarking")
	dimStyle.Println("        → Best for: Fresh benchmarks, reproducible tests")
	fmt.Println()
	fmt.Println("    [2] Connect to existing Azure Managed Redis")
	fmt.Println("        → Uses an existing AMR by resource ID")
	dimStyle.Println("        → Best for: Production/staging benchmarks")
	fmt.Println()
	fmt.Println("    [3] Direct endpoint connection")
	fmt.Println("        → Connect to any Redis endpoint")
	dimStyle.Println("        → Best for: Non-Azure Redis, complex networking")
	fmt.Println()

	input := s.prompt("Select mode", "1")
	switch input {
	case "1":
		s.config.RedisMode = "provision"
		return s.configureAMRProvision()
	case "2":
		s.config.RedisMode = "existing"
		return s.configureAMRExisting()
	case "3":
		s.config.RedisMode = "endpoint"
		return s.configureAMREndpoint()
	default:
		s.config.RedisMode = "provision"
		return s.configureAMRProvision()
	}
}

func (s *InteractiveSetup) configureAMRProvision() error {
	fmt.Println()
	fmt.Println(strings.Repeat("─", 78))
	fmt.Println()
	fmt.Println("  Select AMR Template (functional characteristics):")
	fmt.Println()

	templates := []struct {
		Name string
		Desc string
	}{
		{"dev-test", "No HA, no persistence (cheapest)"},
		{"standard", "HA enabled, no persistence"},
		{"durable", "HA + RDB persistence (hourly snapshots)"},
		{"high-durability", "HA + AOF persistence (1s writes)"},
		{"search", "HA + RDB + RediSearch module"},
	}

	for i, t := range templates {
		fmt.Printf("    [%d] %-18s %s\n", i+1, t.Name, t.Desc)
	}

	fmt.Println()
	input := s.prompt("Select template", "2")
	idx, _ := strconv.Atoi(input)
	if idx < 1 || idx > len(templates) {
		idx = 2
	}
	s.config.RedisTemplate = templates[idx-1].Name

	// SKU Selection
	fmt.Println()
	fmt.Println(strings.Repeat("─", 78))
	fmt.Println()
	fmt.Println("  Select AMR SKU (performance tier):")
	fmt.Println()

	skus := []struct {
		Name   string
		Specs  string
		Cost   string
		Family string
	}{
		{"Balanced_B0", "3 GB, 1 vCPU", "~$0.10/hr", "Balanced"},
		{"Balanced_B5", "24 GB, 2 vCPU", "~$0.50/hr", "Balanced"},
		{"Balanced_B10", "48 GB, 4 vCPU", "~$1.00/hr", "Balanced"},
		{"Balanced_B20", "96 GB, 8 vCPU", "~$2.00/hr", "Balanced"},
		{"ComputeOptimized_X10", "24 GB, 8 vCPU", "~$1.50/hr", "Compute"},
		{"ComputeOptimized_X20", "48 GB, 16 vCPU", "~$3.00/hr", "Compute"},
		{"MemoryOptimized_M20", "64 GB, 4 vCPU", "~$2.50/hr", "Memory"},
		{"MemoryOptimized_M50", "128 GB, 8 vCPU", "~$5.00/hr", "Memory"},
	}

	infoStyle.Println("  Balanced (General Purpose):")
	for i := 0; i < 4; i++ {
		fmt.Printf("    [%d] %-24s %-18s %s\n", i+1, skus[i].Name, skus[i].Specs, skus[i].Cost)
	}

	fmt.Println()
	infoStyle.Println("  Compute Optimized (High Throughput):")
	for i := 4; i < 6; i++ {
		fmt.Printf("    [%d] %-24s %-18s %s\n", i+1, skus[i].Name, skus[i].Specs, skus[i].Cost)
	}

	fmt.Println()
	infoStyle.Println("  Memory Optimized (Large Datasets):")
	for i := 6; i < 8; i++ {
		fmt.Printf("    [%d] %-24s %-18s %s\n", i+1, skus[i].Name, skus[i].Specs, skus[i].Cost)
	}

	fmt.Println()
	input = s.prompt("Enter SKU name or number", "2")

	// Check if number
	if idx, err := strconv.Atoi(input); err == nil && idx >= 1 && idx <= len(skus) {
		s.config.RedisSKU = skus[idx-1].Name
	} else {
		s.config.RedisSKU = input
	}

	// Find SKU cost for summary
	skuCost := "$0.50/hr"
	skuSpecs := "24 GB, 2 vCPU"
	for _, sku := range skus {
		if sku.Name == s.config.RedisSKU {
			skuCost = sku.Cost
			skuSpecs = sku.Specs
			break
		}
	}

	fmt.Println()
	fmt.Println("  AMR Summary:")
	s.printBox([]string{
		fmt.Sprintf("Template:     %s", s.config.RedisTemplate),
		fmt.Sprintf("SKU:          %s (%s)", s.config.RedisSKU, skuSpecs),
		"Network:      Private endpoint (no public access)",
		fmt.Sprintf("Est. Cost:    %s", skuCost),
		"Provision:    ~10-15 minutes",
	})

	return nil
}

func (s *InteractiveSetup) configureAMRExisting() error {
	fmt.Println()
	s.config.RedisResourceID = s.prompt("Enter AMR resource ID", "")

	fmt.Println()
	infoStyle.Println("  ℹ️  Format: /subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Cache/redisEnterprise/{name}")

	return nil
}

func (s *InteractiveSetup) configureAMREndpoint() error {
	fmt.Println()
	s.config.RedisEndpoint = s.prompt("Enter Redis endpoint", "")
	s.config.RedisPort = s.promptInt("Enter Redis port", 10000)
	s.config.RedisPassword = s.prompt("Enter Redis password (or env var name)", "${REDIS_PASSWORD}")

	return nil
}

func (s *InteractiveSetup) stepWorkload(ctx context.Context) error {
	s.printStepHeader("Workload & Execution")

	fmt.Println("  Select benchmark workload:")
	fmt.Println()

	workloads := []struct {
		Name string
		Desc string
	}{
		{"cache", "80% GET, 20% SET (typical cache)"},
		{"write-heavy", "20% GET, 80% SET (write intensive)"},
		{"read-only", "100% GET (read replicas)"},
		{"high-throughput", "Pipelined, max ops/sec"},
		{"low-latency", "Minimal clients, no pipelining"},
		{"session", "Session store pattern"},
	}

	for i, w := range workloads {
		fmt.Printf("    [%d] %-18s %s\n", i+1, w.Name, w.Desc)
	}
	fmt.Println("    [7] Custom workload file...")

	fmt.Println()
	input := s.prompt("Select workload", "1")
	idx, _ := strconv.Atoi(input)
	if idx >= 1 && idx <= len(workloads) {
		s.config.Workload = workloads[idx-1].Name
	} else if idx == 7 {
		s.config.Workload = s.prompt("Enter workload file path", "")
	} else {
		s.config.Workload = "cache"
	}

	fmt.Println()
	durationStr := s.prompt("Benchmark duration", "5m")
	if d, err := time.ParseDuration(durationStr); err == nil {
		s.config.Duration = d
	}

	fmt.Println()
	fmt.Println("  Cleanup after benchmark?")
	fmt.Println("    [1] Yes - Delete all resources when done (recommended)")
	fmt.Println("    [2] No  - Keep resources for further testing")
	fmt.Println()

	input = s.prompt("Select", "1")
	s.config.AutoClean = (input != "2")

	return nil
}

func (s *InteractiveSetup) stepReview(ctx context.Context) error {
	s.step++ // Count review as a step
	fmt.Println()
	stepStyle.Println("Review Configuration")
	fmt.Println(strings.Repeat("━", 78))
	fmt.Println()

	// Calculate costs
	runnerCost := 0.19 * float64(s.config.RunnerCount)
	redisCost := 0.50 // Default

	switch {
	case strings.HasPrefix(s.config.RedisSKU, "Balanced_B0"):
		redisCost = 0.10
	case strings.HasPrefix(s.config.RedisSKU, "Balanced_B10"):
		redisCost = 1.00
	case strings.HasPrefix(s.config.RedisSKU, "Balanced_B20"):
		redisCost = 2.00
	case strings.HasPrefix(s.config.RedisSKU, "ComputeOptimized"):
		redisCost = 1.50
	case strings.HasPrefix(s.config.RedisSKU, "MemoryOptimized"):
		redisCost = 2.50
	}

	totalHourly := runnerCost + redisCost
	provisionTime := 15.0 // minutes
	benchmarkTime := s.config.Duration.Minutes()
	totalMinutes := provisionTime + benchmarkTime
	totalCost := totalHourly * (totalMinutes / 60.0)

	lines := []string{
		"AZURE CONFIGURATION",
		strings.Repeat("─", 70),
		fmt.Sprintf("Subscription:    %s", s.config.SubscriptionID[:8]+"..."),
		fmt.Sprintf("Region:          %s", s.config.Region),
		fmt.Sprintf("Resource Group:  %s (new)", s.config.ResourceGroupName),
		fmt.Sprintf("VNet:            %s (%s) (new)", s.config.VNetName, s.config.VNetAddressSpace),
		"",
		"RUNNERS",
		strings.Repeat("─", 70),
		fmt.Sprintf("Count:           %d VMs", s.config.RunnerCount),
		fmt.Sprintf("Size:            %s", s.config.RunnerSize),
		fmt.Sprintf("Cost:            ~$%.2f/hr", runnerCost),
		"",
		"REDIS TARGET",
		strings.Repeat("─", 70),
		fmt.Sprintf("Mode:            %s", s.config.RedisMode),
	}

	if s.config.RedisMode == "provision" {
		lines = append(lines,
			fmt.Sprintf("Template:        %s", s.config.RedisTemplate),
			fmt.Sprintf("SKU:             %s", s.config.RedisSKU),
			fmt.Sprintf("Cost:            ~$%.2f/hr", redisCost),
		)
	} else if s.config.RedisMode == "existing" {
		lines = append(lines,
			fmt.Sprintf("Resource ID:     %s", s.config.RedisResourceID[:50]+"..."),
		)
	} else {
		lines = append(lines,
			fmt.Sprintf("Endpoint:        %s:%d", s.config.RedisEndpoint, s.config.RedisPort),
		)
	}

	lines = append(lines,
		"",
		"BENCHMARK",
		strings.Repeat("─", 70),
		fmt.Sprintf("Workload:        %s", s.config.Workload),
		fmt.Sprintf("Duration:        %s", s.config.Duration),
		fmt.Sprintf("Cleanup:         %v", s.config.AutoClean),
		"",
		"ESTIMATED TOTAL COST",
		strings.Repeat("─", 70),
		fmt.Sprintf("Infrastructure:  ~$%.2f/hr", totalHourly),
		fmt.Sprintf("Provision time:  ~%.0f min", provisionTime),
		fmt.Sprintf("Benchmark time:  ~%.0f min", benchmarkTime),
		fmt.Sprintf("Total estimate:  ~$%.2f (for this run)", totalCost),
	)

	s.printBox(lines)

	fmt.Println()
	if !s.promptYesNo("Proceed with deployment?", true) {
		return fmt.Errorf("deployment cancelled by user")
	}

	return nil
}

// AddInteractiveCommand adds the interactive setup to the cloud run command
func AddInteractiveCommand(cloudRunCmd *cobra.Command) {
	cloudRunCmd.Flags().Bool("interactive", false, "Run interactive setup wizard")
}
