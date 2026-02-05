// Package cloud provides cloud infrastructure management for distributed benchmarking.
package cloud

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/google/uuid"
	"github.com/tfindelkind-redis/redismeter/internal/domain"
	"github.com/tfindelkind-redis/redismeter/internal/plugin"
)

// =============================================================================
// Azure Provider Configuration
// =============================================================================

// AzureConfig holds Azure-specific configuration.
type AzureConfig struct {
	SubscriptionID string
	SSHPublicKey   string // SSH public key for VM access
	SSHPrivateKey  string // Path to private key for SSH connections
	SSHUser        string // Username for SSH (default: azureuser)

	// Execution policies
	Policy ExecutionPolicy
}

// ExecutionPolicy defines retry and tolerance settings for robust execution.
type ExecutionPolicy struct {
	// Provisioning
	ProvisionTimeout  time.Duration // Max time to wait for VM provisioning (default: 10m)
	ProvisionRetries  int           // Number of retry attempts per VM (default: 2)
	ParallelProvision bool          // Provision VMs in parallel (default: true)

	// SSH & Configuration
	SSHConnectTimeout time.Duration // Timeout for SSH connection (default: 30s)
	SSHRetries        int           // SSH connection retry attempts (default: 5)
	SSHRetryDelay     time.Duration // Delay between SSH retries (default: 10s)
	ConfigureTimeout  time.Duration // Timeout for memtier installation (default: 5m)

	// Execution
	ExecutionTimeout    time.Duration // Max benchmark duration (default: 1h)
	HealthCheckInterval time.Duration // Health check frequency (default: 5s)
	HeartbeatTimeout    time.Duration // Max time without heartbeat (default: 30s)

	// Failure tolerance
	FailureTolerance  int  // Max VMs that can fail (default: 0 = none)
	ContinueOnPartial bool // Continue if some VMs fail to provision

	// Synchronization
	SyncWindow    time.Duration // Time window for synchronized start (default: 2s)
	PreStartDelay time.Duration // Delay before starting after sync (default: 5s)
}

// DefaultExecutionPolicy returns sensible defaults for Azure deployments.
func DefaultExecutionPolicy() ExecutionPolicy {
	return ExecutionPolicy{
		ProvisionTimeout:    10 * time.Minute,
		ProvisionRetries:    2,
		ParallelProvision:   true,
		SSHConnectTimeout:   30 * time.Second,
		SSHRetries:          5,
		SSHRetryDelay:       10 * time.Second,
		ConfigureTimeout:    5 * time.Minute,
		ExecutionTimeout:    1 * time.Hour,
		HealthCheckInterval: 5 * time.Second,
		HeartbeatTimeout:    30 * time.Second,
		FailureTolerance:    0,
		ContinueOnPartial:   false,
		SyncWindow:          2 * time.Second,
		PreStartDelay:       5 * time.Second,
	}
}

// =============================================================================
// Azure VM Size Catalog (for Runner VMs)
// =============================================================================

// AzureVMSize represents an Azure VM size with specifications.
type AzureVMSize struct {
	Name        string  `json:"name"`     // e.g., Standard_D4s_v3
	Family      string  `json:"family"`   // e.g., Dsv3
	Category    string  `json:"category"` // cost-optimized, balanced, high-performance
	VCPUs       int     `json:"vcpus"`
	MemoryGB    int     `json:"memory_gb"`
	NetworkGbps float64 `json:"network_gbps"` // Expected network bandwidth
	HourlyCost  float64 `json:"hourly_cost"`  // Approximate USD/hr (varies by region)
	Description string  `json:"description"`
}

// VMSizeCategory groups VM sizes by use case.
const (
	VMCategoryCostOptimized    = "cost-optimized"
	VMCategoryBalanced         = "balanced"
	VMCategoryHighPerformance  = "high-performance"
	VMCategoryNetworkOptimized = "network-optimized"
)

// GetAzureVMSizes returns available Azure VM sizes for runner VMs.
// Prices are approximate and vary by region.
func GetAzureVMSizes() []AzureVMSize {
	return []AzureVMSize{
		// Cost-Optimized (B-series burstable)
		{
			Name:        "Standard_B2s",
			Family:      "Bs",
			Category:    VMCategoryCostOptimized,
			VCPUs:       2,
			MemoryGB:    4,
			NetworkGbps: 1,
			HourlyCost:  0.042,
			Description: "Burstable, dev/test only",
		},
		{
			Name:        "Standard_B2ms",
			Family:      "Bs",
			Category:    VMCategoryCostOptimized,
			VCPUs:       2,
			MemoryGB:    8,
			NetworkGbps: 1,
			HourlyCost:  0.083,
			Description: "Burstable, light workloads",
		},

		// Balanced (D-series v3/v5)
		{
			Name:        "Standard_D2s_v3",
			Family:      "Dsv3",
			Category:    VMCategoryBalanced,
			VCPUs:       2,
			MemoryGB:    8,
			NetworkGbps: 1,
			HourlyCost:  0.096,
			Description: "Small benchmark runs",
		},
		{
			Name:        "Standard_D4s_v3",
			Family:      "Dsv3",
			Category:    VMCategoryBalanced,
			VCPUs:       4,
			MemoryGB:    16,
			NetworkGbps: 2,
			HourlyCost:  0.192,
			Description: "Recommended for most benchmarks",
		},
		{
			Name:        "Standard_D8s_v3",
			Family:      "Dsv3",
			Category:    VMCategoryBalanced,
			VCPUs:       8,
			MemoryGB:    32,
			NetworkGbps: 4,
			HourlyCost:  0.384,
			Description: "High-load benchmarks",
		},
		{
			Name:        "Standard_D16s_v3",
			Family:      "Dsv3",
			Category:    VMCategoryBalanced,
			VCPUs:       16,
			MemoryGB:    64,
			NetworkGbps: 8,
			HourlyCost:  0.768,
			Description: "Maximum load generation",
		},
		{
			Name:        "Standard_D4s_v5",
			Family:      "Dsv5",
			Category:    VMCategoryBalanced,
			VCPUs:       4,
			MemoryGB:    16,
			NetworkGbps: 12.5,
			HourlyCost:  0.192,
			Description: "Latest gen, better network",
		},
		{
			Name:        "Standard_D8s_v5",
			Family:      "Dsv5",
			Category:    VMCategoryBalanced,
			VCPUs:       8,
			MemoryGB:    32,
			NetworkGbps: 12.5,
			HourlyCost:  0.384,
			Description: "Latest gen, high performance",
		},

		// High Performance (F-series compute optimized)
		{
			Name:        "Standard_F4s_v2",
			Family:      "Fsv2",
			Category:    VMCategoryHighPerformance,
			VCPUs:       4,
			MemoryGB:    8,
			NetworkGbps: 5,
			HourlyCost:  0.169,
			Description: "CPU optimized, high throughput",
		},
		{
			Name:        "Standard_F8s_v2",
			Family:      "Fsv2",
			Category:    VMCategoryHighPerformance,
			VCPUs:       8,
			MemoryGB:    16,
			NetworkGbps: 10,
			HourlyCost:  0.338,
			Description: "CPU optimized, very high throughput",
		},
		{
			Name:        "Standard_F16s_v2",
			Family:      "Fsv2",
			Category:    VMCategoryHighPerformance,
			VCPUs:       16,
			MemoryGB:    32,
			NetworkGbps: 12.5,
			HourlyCost:  0.677,
			Description: "CPU optimized, maximum throughput",
		},

		// Network Optimized (for bandwidth testing)
		{
			Name:        "Standard_D8ds_v5",
			Family:      "Ddsv5",
			Category:    VMCategoryNetworkOptimized,
			VCPUs:       8,
			MemoryGB:    32,
			NetworkGbps: 12.5,
			HourlyCost:  0.452,
			Description: "High network bandwidth",
		},
		{
			Name:        "Standard_D16ds_v5",
			Family:      "Ddsv5",
			Category:    VMCategoryNetworkOptimized,
			VCPUs:       16,
			MemoryGB:    64,
			NetworkGbps: 12.5,
			HourlyCost:  0.904,
			Description: "Maximum network bandwidth",
		},
	}
}

// GetAzureVMSizesByCategory returns VM sizes filtered by category.
func GetAzureVMSizesByCategory(category string) []AzureVMSize {
	var result []AzureVMSize
	for _, size := range GetAzureVMSizes() {
		if size.Category == category {
			result = append(result, size)
		}
	}
	return result
}

// GetAzureVMSize returns a specific VM size by name, or nil if not found.
func GetAzureVMSize(name string) *AzureVMSize {
	for _, size := range GetAzureVMSizes() {
		if size.Name == name {
			return &size
		}
	}
	return nil
}

// ValidateAzureVMSize checks if a VM size is valid for benchmark runners.
func ValidateAzureVMSize(name string) error {
	if GetAzureVMSize(name) != nil {
		return nil
	}
	// Also allow any Standard_* size (user may know sizes we haven't cataloged)
	if len(name) > 9 && name[:9] == "Standard_" {
		return nil
	}
	return fmt.Errorf("unknown VM size: %s - use GetAzureVMSizes() to see recommended options", name)
}

// =============================================================================
// Enforced Runner OS Image
// =============================================================================

// Azure Runner VMs use Ubuntu 22.04 LTS (Jammy Jellyfish).
// This is enforced and not configurable to ensure memtier_benchmark
// installation works correctly.
const (
	AzureRunnerImagePublisher = "Canonical"
	AzureRunnerImageOffer     = "0001-com-ubuntu-server-jammy"
	AzureRunnerImageSKU       = "22_04-lts-gen2"
	AzureRunnerImageVersion   = "latest"
	AzureRunnerOSDescription  = "Ubuntu 22.04 LTS (Jammy Jellyfish)"
)

// =============================================================================
// Deployment State Management
// =============================================================================

// DeploymentState represents the current phase of a deployment.
type DeploymentState string

const (
	StateInitializing  DeploymentState = "initializing"
	StateProvisioning  DeploymentState = "provisioning"
	StateConfiguring   DeploymentState = "configuring"
	StateReady         DeploymentState = "ready"
	StateSynchronizing DeploymentState = "synchronizing"
	StateRunning       DeploymentState = "running"
	StateCollecting    DeploymentState = "collecting"
	StateCompleted     DeploymentState = "completed"
	StateFailed        DeploymentState = "failed"
	StateCleanup       DeploymentState = "cleanup"
)

// VMState represents the state of an individual runner VM.
type VMState string

const (
	VMStateProvisioning VMState = "provisioning"
	VMStateStarting     VMState = "starting"
	VMStateConfiguring  VMState = "configuring"
	VMStateReady        VMState = "ready"
	VMStateRunning      VMState = "running"
	VMStateCompleted    VMState = "completed"
	VMStateFailed       VMState = "failed"
)

// =============================================================================
// Azure Deployment Tracking
// =============================================================================

// AzureDeployment tracks a complete multi-VM deployment lifecycle.
type AzureDeployment struct {
	ID            string
	State         DeploymentState
	Spec          *InfraSpec
	ResourceGroup string
	Location      string

	// VM tracking
	VMs []*AzureVM
	mu  sync.RWMutex

	// Progress tracking
	StartTime time.Time
	EndTime   time.Time

	// Events for monitoring
	Events   []DeploymentEvent
	eventsMu sync.Mutex

	// Cancellation
	cancel context.CancelFunc
	done   chan struct{}

	// Error tracking
	Errors []error
}

// AzureVM tracks an individual runner VM.
type AzureVM struct {
	ID    string
	Name  string
	State VMState

	// Network
	PublicIP  string
	PrivateIP string
	NICId     string

	// SSH
	SSHReady  bool
	SSHClient interface{} // *ssh.Client when connected

	// Health monitoring
	LastHeartbeat time.Time
	FailureCount  int

	// Execution
	ExecutionID string
	StartTime   time.Time
	EndTime     time.Time

	// Results
	Output  string
	RawJSON []byte
	Error   error
}

// DeploymentEvent records significant events during deployment.
type DeploymentEvent struct {
	Time    time.Time
	VM      string // VM name or empty for deployment-level events
	Type    string // provision, configure, start, complete, fail, health
	Message string
	Error   error
}

// =============================================================================
// Azure Provider Implementation
// =============================================================================

// AzureProvider implements the ProviderPlugin interface for Azure.
type AzureProvider struct {
	config     AzureConfig
	credential *azidentity.DefaultAzureCredential

	// Azure clients
	rgClient     *armresources.ResourceGroupsClient
	vmClient     *armcompute.VirtualMachinesClient
	nicClient    *armnetwork.InterfacesClient
	pipClient    *armnetwork.PublicIPAddressesClient
	vnetClient   *armnetwork.VirtualNetworksClient
	subnetClient *armnetwork.SubnetsClient
	nsgClient    *armnetwork.SecurityGroupsClient

	// Active deployments
	deployments map[string]*AzureDeployment
	mu          sync.RWMutex

	// SSH executor for remote commands
	sshExecutor *SSHExecutor
}

// NewAzureProvider creates a new Azure provider instance.
func NewAzureProvider(config AzureConfig) (*AzureProvider, error) {
	// Use default Azure credentials (env vars, managed identity, CLI, etc.)
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get Azure credentials: %w", err)
	}

	// Set defaults
	if config.SSHUser == "" {
		config.SSHUser = "azureuser"
	}
	if config.Policy.ProvisionTimeout == 0 {
		config.Policy = DefaultExecutionPolicy()
	}

	// Initialize Azure clients
	rgClient, err := armresources.NewResourceGroupsClient(config.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource groups client: %w", err)
	}

	vmClient, err := armcompute.NewVirtualMachinesClient(config.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM client: %w", err)
	}

	nicClient, err := armnetwork.NewInterfacesClient(config.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create NIC client: %w", err)
	}

	pipClient, err := armnetwork.NewPublicIPAddressesClient(config.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create public IP client: %w", err)
	}

	vnetClient, err := armnetwork.NewVirtualNetworksClient(config.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create VNet client: %w", err)
	}

	subnetClient, err := armnetwork.NewSubnetsClient(config.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create subnet client: %w", err)
	}

	nsgClient, err := armnetwork.NewSecurityGroupsClient(config.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create NSG client: %w", err)
	}

	// Initialize SSH executor
	sshConfig := SSHConfig{
		User:           config.SSHUser,
		PrivateKeyPath: config.SSHPrivateKey,
		Port:           22,
		ConnectTimeout: config.Policy.SSHConnectTimeout,
	}
	sshExecutor := NewSSHExecutor(sshConfig)

	return &AzureProvider{
		config:       config,
		credential:   cred,
		rgClient:     rgClient,
		vmClient:     vmClient,
		nicClient:    nicClient,
		pipClient:    pipClient,
		vnetClient:   vnetClient,
		subnetClient: subnetClient,
		nsgClient:    nsgClient,
		deployments:  make(map[string]*AzureDeployment),
		sshExecutor:  sshExecutor,
	}, nil
}

// =============================================================================
// ProviderPlugin Interface Implementation
// =============================================================================

// Metadata returns information about the plugin.
func (p *AzureProvider) Metadata() plugin.Metadata {
	return plugin.Metadata{
		Name:        "azure",
		Version:     "1.0.0",
		Type:        plugin.TypeCloud,
		Description: "Azure cloud provider for VMs and Azure Managed Redis",
	}
}

// Initialize sets up the Azure provider with configuration.
func (p *AzureProvider) Initialize(ctx context.Context, cfg map[string]interface{}) error {
	// Parse configuration - most setup is done in NewAzureProvider
	if subscriptionID, ok := cfg["subscription_id"].(string); ok {
		p.config.SubscriptionID = subscriptionID
	}
	if sshUser, ok := cfg["ssh_user"].(string); ok {
		p.config.SSHUser = sshUser
	}
	if sshKey, ok := cfg["ssh_private_key"].(string); ok {
		p.config.SSHPrivateKey = sshKey
	}
	if sshPubKey, ok := cfg["ssh_public_key"].(string); ok {
		p.config.SSHPublicKey = sshPubKey
	}
	return nil
}

// HealthCheck verifies Azure connectivity.
func (p *AzureProvider) HealthCheck(ctx context.Context) plugin.HealthStatus {
	if p.rgClient == nil {
		return plugin.HealthStatus{
			Healthy: false,
			Message: "Azure client not initialized",
		}
	}

	// Try to list resource groups to verify connectivity
	pager := p.rgClient.NewListPager(nil)
	_, err := pager.NextPage(ctx)
	if err != nil {
		return plugin.HealthStatus{
			Healthy: false,
			Message: fmt.Sprintf("Azure connectivity failed: %v", err),
		}
	}

	return plugin.HealthStatus{
		Healthy: true,
		Message: fmt.Sprintf("Azure provider ready (subscription: %s)", p.config.SubscriptionID),
	}
}

// Shutdown gracefully stops the Azure provider.
func (p *AzureProvider) Shutdown(ctx context.Context) error {
	// Clean up any SSH connections
	if p.sshExecutor != nil {
		p.sshExecutor.Shutdown(ctx)
	}
	return nil
}

// Name returns the provider name.
func (p *AzureProvider) Name() string {
	return "azure"
}

// Provision creates cloud infrastructure for benchmark execution.
func (p *AzureProvider) Provision(ctx context.Context, spec *InfraSpec) (*Infrastructure, error) {
	deployID := fmt.Sprintf("rm-%s", uuid.New().String()[:8])

	// Create deployment tracking
	deployment := &AzureDeployment{
		ID:            deployID,
		State:         StateInitializing,
		Spec:          spec,
		ResourceGroup: fmt.Sprintf("redismeter-%s", deployID),
		Location:      azureRegionFromSpec(spec.Region),
		VMs:           make([]*AzureVM, 0, spec.LoadGenerator.Count),
		StartTime:     time.Now(),
		done:          make(chan struct{}),
	}

	// Create cancellable context
	ctx, deployment.cancel = context.WithCancel(ctx)

	p.mu.Lock()
	p.deployments[deployID] = deployment
	p.mu.Unlock()

	// Run provisioning in stages
	if err := p.runProvisioning(ctx, deployment); err != nil {
		deployment.State = StateFailed
		deployment.Errors = append(deployment.Errors, err)
		return nil, err
	}

	// Build infrastructure response
	loadGenerators := make([]Instance, 0, len(deployment.VMs))
	for _, vm := range deployment.VMs {
		if vm.State == VMStateReady {
			loadGenerators = append(loadGenerators, Instance{
				ID:         vm.ID,
				Name:       vm.Name,
				Type:       spec.LoadGenerator.InstanceType,
				State:      InstanceStateRunning,
				PublicIP:   vm.PublicIP,
				PrivateIP:  vm.PrivateIP,
				LaunchTime: deployment.StartTime,
				Role:       "load_generator",
			})
		}
	}

	return &Infrastructure{
		ID:             deployID,
		Name:           spec.Name,
		Provider:       "azure",
		Region:         spec.Region,
		State:          InfraStateReady,
		CreatedAt:      deployment.StartTime,
		LoadGenerators: loadGenerators,
		Tags: map[string]string{
			"managed-by": "redismeter",
			"deployment": deployID,
		},
		ProviderMetadata: map[string]interface{}{
			"resource_group": deployment.ResourceGroup,
			"location":       deployment.Location,
		},
	}, nil
}

// runProvisioning executes the full provisioning pipeline.
func (p *AzureProvider) runProvisioning(ctx context.Context, d *AzureDeployment) error {
	// Stage 1: Create resource group
	d.addEvent("", "provision", "Creating resource group", nil)
	if err := p.createResourceGroup(ctx, d); err != nil {
		return fmt.Errorf("failed to create resource group: %w", err)
	}

	// Stage 2: Create network infrastructure
	d.addEvent("", "provision", "Creating network infrastructure", nil)
	subnetID, err := p.createNetworkInfra(ctx, d)
	if err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}

	// Stage 3: Provision VMs (parallel or sequential)
	d.State = StateProvisioning
	if err := p.provisionVMs(ctx, d, subnetID); err != nil {
		return fmt.Errorf("failed to provision VMs: %w", err)
	}

	// Stage 4: Configure VMs (install memtier, verify connectivity)
	d.State = StateConfiguring
	if err := p.configureVMs(ctx, d); err != nil {
		return fmt.Errorf("failed to configure VMs: %w", err)
	}

	// Stage 5: Verify all ready
	d.State = StateReady
	readyCount := 0
	for _, vm := range d.VMs {
		if vm.State == VMStateReady {
			readyCount++
		}
	}

	minRequired := d.Spec.LoadGenerator.Count - p.config.Policy.FailureTolerance
	if readyCount < minRequired {
		return fmt.Errorf("only %d of %d VMs ready (minimum %d required)",
			readyCount, d.Spec.LoadGenerator.Count, minRequired)
	}

	d.addEvent("", "provision", fmt.Sprintf("Provisioning complete: %d VMs ready", readyCount), nil)
	return nil
}

// =============================================================================
// Stage 1: Resource Group
// =============================================================================

func (p *AzureProvider) createResourceGroup(ctx context.Context, d *AzureDeployment) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	_, err := p.rgClient.CreateOrUpdate(ctx, d.ResourceGroup, armresources.ResourceGroup{
		Location: to.Ptr(d.Location),
		Tags: map[string]*string{
			"managed-by": to.Ptr("redismeter"),
			"deployment": to.Ptr(d.ID),
			"created-at": to.Ptr(time.Now().Format(time.RFC3339)),
		},
	}, nil)

	return err
}

// =============================================================================
// Stage 2: Network Infrastructure
// =============================================================================

func (p *AzureProvider) createNetworkInfra(ctx context.Context, d *AzureDeployment) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	vnetName := fmt.Sprintf("%s-vnet", d.ID)
	subnetName := "runners"
	nsgName := fmt.Sprintf("%s-nsg", d.ID)

	// Create NSG with SSH access
	nsgPoller, err := p.nsgClient.BeginCreateOrUpdate(ctx, d.ResourceGroup, nsgName,
		armnetwork.SecurityGroup{
			Location: to.Ptr(d.Location),
			Properties: &armnetwork.SecurityGroupPropertiesFormat{
				SecurityRules: []*armnetwork.SecurityRule{
					{
						Name: to.Ptr("AllowSSH"),
						Properties: &armnetwork.SecurityRulePropertiesFormat{
							Priority:                 to.Ptr[int32](100),
							Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
							Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
							Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
							SourceAddressPrefix:      to.Ptr("*"),
							SourcePortRange:          to.Ptr("*"),
							DestinationAddressPrefix: to.Ptr("*"),
							DestinationPortRange:     to.Ptr("22"),
						},
					},
				},
			},
		}, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create NSG: %w", err)
	}

	nsgResp, err := nsgPoller.PollUntilDone(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to wait for NSG: %w", err)
	}

	// Create VNet
	vnetPoller, err := p.vnetClient.BeginCreateOrUpdate(ctx, d.ResourceGroup, vnetName,
		armnetwork.VirtualNetwork{
			Location: to.Ptr(d.Location),
			Properties: &armnetwork.VirtualNetworkPropertiesFormat{
				AddressSpace: &armnetwork.AddressSpace{
					AddressPrefixes: []*string{to.Ptr("10.0.0.0/16")},
				},
				Subnets: []*armnetwork.Subnet{
					{
						Name: to.Ptr(subnetName),
						Properties: &armnetwork.SubnetPropertiesFormat{
							AddressPrefix: to.Ptr("10.0.1.0/24"),
							NetworkSecurityGroup: &armnetwork.SecurityGroup{
								ID: nsgResp.ID,
							},
						},
					},
				},
			},
		}, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create VNet: %w", err)
	}

	vnetResp, err := vnetPoller.PollUntilDone(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to wait for VNet: %w", err)
	}

	// Return subnet ID
	return *vnetResp.Properties.Subnets[0].ID, nil
}

// =============================================================================
// Stage 3: VM Provisioning
// =============================================================================

func (p *AzureProvider) provisionVMs(ctx context.Context, d *AzureDeployment, subnetID string) error {
	count := d.Spec.LoadGenerator.Count

	// Create VM tracking objects
	for i := 0; i < count; i++ {
		vm := &AzureVM{
			ID:    fmt.Sprintf("%s-runner-%d", d.ID, i),
			Name:  fmt.Sprintf("%s-runner-%d", d.ID, i),
			State: VMStateProvisioning,
		}
		d.VMs = append(d.VMs, vm)
	}

	if p.config.Policy.ParallelProvision {
		return p.provisionVMsParallel(ctx, d, subnetID)
	}
	return p.provisionVMsSequential(ctx, d, subnetID)
}

func (p *AzureProvider) provisionVMsParallel(ctx context.Context, d *AzureDeployment, subnetID string) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(d.VMs))

	for _, vm := range d.VMs {
		wg.Add(1)
		go func(vm *AzureVM) {
			defer wg.Done()
			if err := p.provisionSingleVM(ctx, d, vm, subnetID); err != nil {
				d.addEvent(vm.Name, "fail", "VM provisioning failed", err)
				errChan <- fmt.Errorf("VM %s: %w", vm.Name, err)
			}
		}(vm)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	// Check if enough VMs succeeded
	successCount := len(d.VMs) - len(errors)
	minRequired := d.Spec.LoadGenerator.Count - p.config.Policy.FailureTolerance

	if successCount < minRequired {
		return fmt.Errorf("insufficient VMs provisioned: %d succeeded, %d required (errors: %v)",
			successCount, minRequired, errors)
	}

	if len(errors) > 0 && !p.config.Policy.ContinueOnPartial {
		return fmt.Errorf("VM provisioning errors: %v", errors)
	}

	return nil
}

func (p *AzureProvider) provisionVMsSequential(ctx context.Context, d *AzureDeployment, subnetID string) error {
	for _, vm := range d.VMs {
		if err := p.provisionSingleVM(ctx, d, vm, subnetID); err != nil {
			d.addEvent(vm.Name, "fail", "VM provisioning failed", err)

			if !p.config.Policy.ContinueOnPartial {
				return err
			}
		}
	}
	return nil
}

func (p *AzureProvider) provisionSingleVM(ctx context.Context, d *AzureDeployment, vm *AzureVM, subnetID string) error {
	ctx, cancel := context.WithTimeout(ctx, p.config.Policy.ProvisionTimeout)
	defer cancel()

	d.addEvent(vm.Name, "provision", "Starting VM provisioning", nil)

	// Create public IP
	pipName := fmt.Sprintf("%s-pip", vm.Name)
	pipPoller, err := p.pipClient.BeginCreateOrUpdate(ctx, d.ResourceGroup, pipName,
		armnetwork.PublicIPAddress{
			Location: to.Ptr(d.Location),
			SKU:      &armnetwork.PublicIPAddressSKU{Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard)},
			Properties: &armnetwork.PublicIPAddressPropertiesFormat{
				PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
			},
		}, nil)
	if err != nil {
		return fmt.Errorf("failed to create public IP: %w", err)
	}

	pipResp, err := pipPoller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to wait for public IP: %w", err)
	}
	vm.PublicIP = *pipResp.Properties.IPAddress

	// Create NIC
	nicName := fmt.Sprintf("%s-nic", vm.Name)
	nicPoller, err := p.nicClient.BeginCreateOrUpdate(ctx, d.ResourceGroup, nicName,
		armnetwork.Interface{
			Location: to.Ptr(d.Location),
			Properties: &armnetwork.InterfacePropertiesFormat{
				IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
					{
						Name: to.Ptr("primary"),
						Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
							Subnet:                    &armnetwork.Subnet{ID: to.Ptr(subnetID)},
							PrivateIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodDynamic),
							PublicIPAddress:           &armnetwork.PublicIPAddress{ID: pipResp.ID},
						},
					},
				},
			},
		}, nil)
	if err != nil {
		return fmt.Errorf("failed to create NIC: %w", err)
	}

	nicResp, err := nicPoller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to wait for NIC: %w", err)
	}
	vm.NICId = *nicResp.ID
	vm.PrivateIP = *nicResp.Properties.IPConfigurations[0].Properties.PrivateIPAddress

	// Create VM with cloud-init
	cloudInit := p.generateCloudInit()

	vmPoller, err := p.vmClient.BeginCreateOrUpdate(ctx, d.ResourceGroup, vm.Name,
		armcompute.VirtualMachine{
			Location: to.Ptr(d.Location),
			Properties: &armcompute.VirtualMachineProperties{
				HardwareProfile: &armcompute.HardwareProfile{
					VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes(d.Spec.LoadGenerator.InstanceType)),
				},
				StorageProfile: &armcompute.StorageProfile{
					ImageReference: &armcompute.ImageReference{
						Publisher: to.Ptr(AzureRunnerImagePublisher),
						Offer:     to.Ptr(AzureRunnerImageOffer),
						SKU:       to.Ptr(AzureRunnerImageSKU),
						Version:   to.Ptr(AzureRunnerImageVersion),
					},
					OSDisk: &armcompute.OSDisk{
						CreateOption: to.Ptr(armcompute.DiskCreateOptionTypesFromImage),
						ManagedDisk: &armcompute.ManagedDiskParameters{
							StorageAccountType: to.Ptr(armcompute.StorageAccountTypesPremiumLRS),
						},
					},
				},
				OSProfile: &armcompute.OSProfile{
					ComputerName:  to.Ptr(vm.Name),
					AdminUsername: to.Ptr(p.config.SSHUser),
					CustomData:    to.Ptr(cloudInit),
					LinuxConfiguration: &armcompute.LinuxConfiguration{
						DisablePasswordAuthentication: to.Ptr(true),
						SSH: &armcompute.SSHConfiguration{
							PublicKeys: []*armcompute.SSHPublicKey{
								{
									Path:    to.Ptr(fmt.Sprintf("/home/%s/.ssh/authorized_keys", p.config.SSHUser)),
									KeyData: to.Ptr(p.config.SSHPublicKey),
								},
							},
						},
					},
				},
				NetworkProfile: &armcompute.NetworkProfile{
					NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
						{
							ID: to.Ptr(vm.NICId),
							Properties: &armcompute.NetworkInterfaceReferenceProperties{
								Primary: to.Ptr(true),
							},
						},
					},
				},
				// Use spot instances if requested
				Priority:       p.getVMPriority(d.Spec.LoadGenerator.SpotInstances),
				EvictionPolicy: p.getEvictionPolicy(d.Spec.LoadGenerator.SpotInstances),
			},
			Tags: map[string]*string{
				"managed-by": to.Ptr("redismeter"),
				"deployment": to.Ptr(d.ID),
				"role":       to.Ptr("runner"),
			},
		}, nil)
	if err != nil {
		return fmt.Errorf("failed to create VM: %w", err)
	}

	vm.State = VMStateStarting
	d.addEvent(vm.Name, "provision", "VM creation started, waiting for completion", nil)

	_, err = vmPoller.PollUntilDone(ctx, nil)
	if err != nil {
		vm.State = VMStateFailed
		return fmt.Errorf("failed to wait for VM: %w", err)
	}

	vm.State = VMStateConfiguring
	d.addEvent(vm.Name, "provision", fmt.Sprintf("VM ready at %s", vm.PublicIP), nil)
	return nil
}

func (p *AzureProvider) getVMPriority(spot bool) *armcompute.VirtualMachinePriorityTypes {
	if spot {
		return to.Ptr(armcompute.VirtualMachinePriorityTypesSpot)
	}
	return to.Ptr(armcompute.VirtualMachinePriorityTypesRegular)
}

func (p *AzureProvider) getEvictionPolicy(spot bool) *armcompute.VirtualMachineEvictionPolicyTypes {
	if spot {
		return to.Ptr(armcompute.VirtualMachineEvictionPolicyTypesDeallocate)
	}
	return nil
}

// generateCloudInit creates cloud-init script to bootstrap the VM.
func (p *AzureProvider) generateCloudInit() string {
	// This script:
	// 1. Updates packages
	// 2. Installs memtier_benchmark dependencies
	// 3. Builds and installs memtier_benchmark
	// 4. Creates a ready marker file
	script := `#!/bin/bash
set -e

# Update and install dependencies
apt-get update
apt-get install -y build-essential autoconf automake libpcre3-dev libevent-dev pkg-config zlib1g-dev libssl-dev git

# Clone and build memtier_benchmark
cd /opt
git clone https://github.com/RedisLabs/memtier_benchmark.git
cd memtier_benchmark
autoreconf -ivf
./configure
make -j$(nproc)
make install

# Signal readiness
touch /tmp/memtier_ready
echo "memtier_benchmark installation complete" > /tmp/memtier_ready
`
	return base64Encode(script)
}

// =============================================================================
// Stage 4: VM Configuration & SSH Setup
// =============================================================================

func (p *AzureProvider) configureVMs(ctx context.Context, d *AzureDeployment) error {
	var wg sync.WaitGroup

	fmt.Printf("   🔧 Configuring %d VMs...\n", len(d.VMs))

	for _, vm := range d.VMs {
		if vm.State == VMStateFailed {
			fmt.Printf("   ⚠️  Skipping failed VM: %s\n", vm.Name)
			continue // Skip failed VMs
		}

		fmt.Printf("   📡 VM %s: State=%v, PublicIP=%s\n", vm.Name, vm.State, vm.PublicIP)

		wg.Add(1)
		go func(vm *AzureVM) {
			defer wg.Done()
			if err := p.configureVM(ctx, d, vm); err != nil {
				vm.State = VMStateFailed
				vm.Error = err
				fmt.Printf("   ❌ VM %s configuration failed: %v\n", vm.Name, err)
				d.addEvent(vm.Name, "fail", "Configuration failed", err)
			} else {
				fmt.Printf("   ✅ VM %s configured successfully\n", vm.Name)
			}
		}(vm)
	}

	wg.Wait()
	return nil
}

func (p *AzureProvider) configureVM(ctx context.Context, d *AzureDeployment, vm *AzureVM) error {
	ctx, cancel := context.WithTimeout(ctx, p.config.Policy.ConfigureTimeout)
	defer cancel()

	fmt.Printf("      [%s] Starting SSH configuration (timeout: %v)\n", vm.Name, p.config.Policy.ConfigureTimeout)
	fmt.Printf("      [%s] SSH Config: User=%s, PrivateKeyPath=%s, Port=%d\n",
		vm.Name, p.sshExecutor.config.User, p.sshExecutor.config.PrivateKeyPath, p.sshExecutor.config.Port)

	d.addEvent(vm.Name, "configure", "Waiting for SSH availability", nil)

	// Wait for SSH with exponential backoff
	var sshErr error
	for attempt := 0; attempt < p.config.Policy.SSHRetries; attempt++ {
		fmt.Printf("      [%s] SSH attempt %d/%d to %s...\n", vm.Name, attempt+1, p.config.Policy.SSHRetries, vm.PublicIP)
		if err := p.sshExecutor.TestConnection(ctx, vm.PublicIP); err != nil {
			sshErr = err
			fmt.Printf("      [%s] SSH failed: %v\n", vm.Name, err)
			delay := p.config.Policy.SSHRetryDelay * time.Duration(1<<attempt) // Exponential backoff
			if delay > 60*time.Second {
				delay = 60 * time.Second
			}
			d.addEvent(vm.Name, "configure", fmt.Sprintf("SSH not ready, retrying in %v (attempt %d/%d)", delay, attempt+1, p.config.Policy.SSHRetries), nil)

			select {
			case <-ctx.Done():
				fmt.Printf("      [%s] Context cancelled during SSH wait\n", vm.Name)
				return ctx.Err()
			case <-time.After(delay):
				continue
			}
		}
		fmt.Printf("      [%s] SSH connected!\n", vm.Name)
		sshErr = nil
		break
	}

	if sshErr != nil {
		return fmt.Errorf("SSH connection failed after %d attempts: %w", p.config.Policy.SSHRetries, sshErr)
	}

	vm.SSHReady = true
	d.addEvent(vm.Name, "configure", "SSH connection established", nil)

	// Wait for memtier installation (cloud-init)
	fmt.Printf("      [%s] Waiting for memtier_benchmark installation...\n", vm.Name)
	d.addEvent(vm.Name, "configure", "Waiting for memtier_benchmark installation", nil)

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("      [%s] Context cancelled during memtier wait\n", vm.Name)
			return ctx.Err()
		default:
		}

		output, err := p.sshExecutor.RunCommand(ctx, vm.PublicIP, "cat /tmp/memtier_ready 2>/dev/null || echo 'not_ready'")
		if err == nil && strings.TrimSpace(output) != "not_ready" {
			fmt.Printf("      [%s] memtier_ready found: %s\n", vm.Name, strings.TrimSpace(output))
			break
		}
		if err != nil {
			fmt.Printf("      [%s] Error checking memtier_ready: %v\n", vm.Name, err)
		}

		time.Sleep(5 * time.Second)
	}

	// Verify memtier works
	fmt.Printf("      [%s] Verifying memtier_benchmark...\n", vm.Name)
	output, err := p.sshExecutor.RunCommand(ctx, vm.PublicIP, "memtier_benchmark --version")
	if err != nil {
		return fmt.Errorf("memtier_benchmark verification failed: %w", err)
	}
	fmt.Printf("      [%s] memtier version: %s\n", vm.Name, strings.TrimSpace(output))

	vm.State = VMStateReady
	vm.LastHeartbeat = time.Now()
	d.addEvent(vm.Name, "configure", "VM fully configured and ready", nil)
	return nil
}

// =============================================================================
// Benchmark Execution with Synchronization
// =============================================================================

// ExecuteBenchmark runs a synchronized benchmark across all ready VMs.
func (p *AzureProvider) ExecuteBenchmark(ctx context.Context, deployID string, workload interface{}, target interface{}) error {
	p.mu.RLock()
	d, ok := p.deployments[deployID]
	p.mu.RUnlock()

	if !ok {
		return fmt.Errorf("deployment not found: %s", deployID)
	}

	if d.State != StateReady {
		return fmt.Errorf("deployment not ready: current state is %s", d.State)
	}

	// Collect ready VMs
	var readyVMs []*AzureVM
	for _, vm := range d.VMs {
		if vm.State == VMStateReady {
			readyVMs = append(readyVMs, vm)
		}
	}

	if len(readyVMs) == 0 {
		return fmt.Errorf("no VMs ready for execution")
	}

	// Phase 1: Prepare all runners (stage the command)
	d.State = StateSynchronizing
	d.addEvent("", "sync", fmt.Sprintf("Preparing %d runners for synchronized start", len(readyVMs)), nil)

	// Calculate synchronized start time
	startTime := time.Now().Add(p.config.Policy.PreStartDelay)
	startTimestamp := startTime.Unix()

	// Phase 2: Deploy start scripts to all VMs
	var wg sync.WaitGroup
	errChan := make(chan error, len(readyVMs))

	for _, vm := range readyVMs {
		wg.Add(1)
		go func(vm *AzureVM) {
			defer wg.Done()

			// Create a script that waits until the synchronized start time
			// then runs memtier_benchmark
			cmd := fmt.Sprintf(`
				#!/bin/bash
				TARGET_TIME=%d
				CURRENT_TIME=$(date +%%s)
				SLEEP_TIME=$((TARGET_TIME - CURRENT_TIME))
				if [ $SLEEP_TIME -gt 0 ]; then
					echo "Waiting $SLEEP_TIME seconds for synchronized start..."
					sleep $SLEEP_TIME
				fi
				echo "Starting benchmark at $(date)"
				memtier_benchmark %s --json-out-file=/tmp/results.json
				echo "Benchmark completed at $(date)"
			`, startTimestamp, buildMemtierArgs(workload, target))

			// Upload and execute the script
			_, err := p.sshExecutor.RunCommand(ctx, vm.PublicIP, fmt.Sprintf("echo '%s' > /tmp/run_benchmark.sh && chmod +x /tmp/run_benchmark.sh", cmd))
			if err != nil {
				errChan <- fmt.Errorf("VM %s: failed to stage script: %w", vm.Name, err)
				return
			}

			d.addEvent(vm.Name, "sync", "Benchmark script staged", nil)
		}(vm)
	}

	wg.Wait()
	close(errChan)

	// Check for staging errors
	for err := range errChan {
		return err
	}

	// Phase 3: Start all benchmarks
	d.State = StateRunning
	d.addEvent("", "start", fmt.Sprintf("Starting synchronized benchmark at %v", startTime), nil)

	// Start execution on all VMs
	for _, vm := range readyVMs {
		wg.Add(1)
		go func(vm *AzureVM) {
			defer wg.Done()
			vm.State = VMStateRunning
			vm.StartTime = time.Now()

			// Start the benchmark (non-blocking on the SSH side using nohup)
			_, err := p.sshExecutor.RunCommand(ctx, vm.PublicIP, "nohup /tmp/run_benchmark.sh > /tmp/benchmark.log 2>&1 &")
			if err != nil {
				vm.State = VMStateFailed
				vm.Error = err
				d.addEvent(vm.Name, "fail", "Failed to start benchmark", err)
			} else {
				d.addEvent(vm.Name, "start", "Benchmark started", nil)
			}
		}(vm)
	}

	wg.Wait()

	// Phase 4: Monitor execution with health checks
	return p.monitorExecution(ctx, d, readyVMs)
}

// monitorExecution monitors running benchmarks and handles failures.
func (p *AzureProvider) monitorExecution(ctx context.Context, d *AzureDeployment, vms []*AzureVM) error {
	ticker := time.NewTicker(p.config.Policy.HealthCheckInterval)
	defer ticker.Stop()

	timeout := time.After(p.config.Policy.ExecutionTimeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-timeout:
			d.addEvent("", "timeout", "Execution timeout reached", nil)
			return fmt.Errorf("execution timeout after %v", p.config.Policy.ExecutionTimeout)

		case <-ticker.C:
			allComplete := true
			failedCount := 0

			for _, vm := range vms {
				if vm.State == VMStateFailed {
					failedCount++
					continue
				}

				if vm.State == VMStateCompleted {
					continue
				}

				// Check if benchmark is still running
				output, err := p.sshExecutor.RunCommand(ctx, vm.PublicIP, "pgrep -f memtier_benchmark || echo 'done'")
				if err != nil {
					vm.FailureCount++
					if vm.FailureCount >= 3 {
						vm.State = VMStateFailed
						vm.Error = fmt.Errorf("lost connection: %w", err)
						d.addEvent(vm.Name, "fail", "Lost connection to VM", err)
						failedCount++
					}
					continue
				}

				vm.LastHeartbeat = time.Now()
				vm.FailureCount = 0

				if output == "done" {
					// Benchmark completed, collect results
					vm.State = VMStateCompleted
					vm.EndTime = time.Now()

					results, _ := p.sshExecutor.RunCommand(ctx, vm.PublicIP, "cat /tmp/results.json")
					vm.RawJSON = []byte(results)
					d.addEvent(vm.Name, "complete", "Benchmark completed", nil)
				} else {
					allComplete = false
				}
			}

			// Check failure tolerance
			if failedCount > p.config.Policy.FailureTolerance {
				return fmt.Errorf("too many VMs failed: %d > %d tolerance", failedCount, p.config.Policy.FailureTolerance)
			}

			if allComplete {
				d.State = StateCollecting
				d.addEvent("", "complete", "All benchmarks completed", nil)
				return nil
			}
		}
	}
}

// =============================================================================
// Teardown & Cleanup
// =============================================================================

// Teardown destroys all infrastructure for a deployment.
func (p *AzureProvider) Teardown(ctx context.Context, infra *Infrastructure) error {
	p.mu.RLock()
	d, ok := p.deployments[infra.ID]
	p.mu.RUnlock()

	if !ok {
		// Try to delete by resource group name directly
		return p.deleteResourceGroup(ctx, fmt.Sprintf("redismeter-%s", infra.ID))
	}

	d.State = StateCleanup
	d.addEvent("", "cleanup", "Starting infrastructure teardown", nil)

	// Cancel any running operations
	if d.cancel != nil {
		d.cancel()
	}

	// Delete resource group (deletes all resources)
	err := p.deleteResourceGroup(ctx, d.ResourceGroup)

	if err == nil {
		d.EndTime = time.Now()
		d.addEvent("", "cleanup", "Infrastructure teardown complete", nil)
	}

	return err
}

func (p *AzureProvider) deleteResourceGroup(ctx context.Context, rgName string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	poller, err := p.rgClient.BeginDelete(ctx, rgName, nil)
	if err != nil {
		return fmt.Errorf("failed to start resource group deletion: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

// =============================================================================
// Helper Functions
// =============================================================================

func (d *AzureDeployment) addEvent(vm, eventType, message string, err error) {
	d.eventsMu.Lock()
	defer d.eventsMu.Unlock()

	d.Events = append(d.Events, DeploymentEvent{
		Time:    time.Now(),
		VM:      vm,
		Type:    eventType,
		Message: message,
		Error:   err,
	})
}

func azureRegionFromSpec(region string) string {
	// Map common region names to Azure locations
	regionMap := map[string]string{
		"us-east-1":      "eastus",
		"us-east-2":      "eastus2",
		"us-west-1":      "westus",
		"us-west-2":      "westus2",
		"eu-west-1":      "westeurope",
		"eu-central-1":   "germanywestcentral",
		"ap-southeast-1": "southeastasia",
	}

	if mapped, ok := regionMap[region]; ok {
		return mapped
	}
	return region // Assume it's already an Azure location
}

func base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func buildMemtierArgs(workload interface{}, target interface{}) string {
	// Build memtier_benchmark arguments from workload and target
	w, wOk := workload.(*domain.Workload)
	t, tOk := target.(*domain.Target)

	if !wOk || !tOk {
		return "--server localhost --port 6379"
	}

	args := fmt.Sprintf("--server %s --port %d", t.Host, t.Port)

	if t.Password != "" {
		args += fmt.Sprintf(" --authenticate %s", t.Password)
	}

	if t.TLS != nil && t.TLS.Enabled {
		args += " --tls"
	}

	if t.Cluster {
		args += " --cluster-mode"
	}

	// Workload settings
	if w.Threads > 0 {
		args += fmt.Sprintf(" --threads %d", w.Threads)
	}
	if w.Clients > 0 {
		args += fmt.Sprintf(" --clients %d", w.Clients)
	}
	if w.Duration != "" {
		args += fmt.Sprintf(" --test-time %s", w.Duration)
	}
	if w.Pipeline > 0 {
		args += fmt.Sprintf(" --pipeline %d", w.Pipeline)
	}

	// Build ratio from operations if available
	if len(w.Operations) >= 2 {
		// Find SET and GET ratios
		var setRatio, getRatio float64
		for _, op := range w.Operations {
			switch op.Command {
			case "SET":
				setRatio = op.Ratio
			case "GET":
				getRatio = op.Ratio
			}
		}
		if setRatio > 0 || getRatio > 0 {
			// Convert to memtier format: SET:GET
			setInt := int(setRatio * 10)
			getInt := int(getRatio * 10)
			args += fmt.Sprintf(" --ratio %d:%d", setInt, getInt)
		}
	}

	// Data size
	if w.DataSize != nil {
		if w.DataSize.Fixed > 0 {
			args += fmt.Sprintf(" --data-size %d", w.DataSize.Fixed)
		} else if w.DataSize.Min > 0 && w.DataSize.Max > 0 {
			args += fmt.Sprintf(" --data-size-range %d-%d", w.DataSize.Min, w.DataSize.Max)
		}
	}

	// Key pattern
	if w.KeyPattern != nil {
		if w.KeyPattern.Prefix != "" {
			args += fmt.Sprintf(" --key-prefix %s", w.KeyPattern.Prefix)
		}
		if w.KeyPattern.KeyRange > 0 {
			args += fmt.Sprintf(" --key-maximum %d", w.KeyPattern.KeyRange)
		}
	}

	return args
}

// GetInfrastructure retrieves an existing infrastructure by ID.
func (p *AzureProvider) GetInfrastructure(ctx context.Context, id string) (*Infrastructure, error) {
	p.mu.RLock()
	d, ok := p.deployments[id]
	p.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("infrastructure not found: %s", id)
	}

	loadGenerators := make([]Instance, 0)
	for _, vm := range d.VMs {
		if vm.State == VMStateReady || vm.State == VMStateRunning || vm.State == VMStateCompleted {
			loadGenerators = append(loadGenerators, Instance{
				ID:        vm.ID,
				Name:      vm.Name,
				Type:      d.Spec.LoadGenerator.InstanceType,
				State:     InstanceState(vm.State),
				PublicIP:  vm.PublicIP,
				PrivateIP: vm.PrivateIP,
				Role:      "load_generator",
			})
		}
	}

	return &Infrastructure{
		ID:             d.ID,
		Name:           d.Spec.Name,
		Provider:       "azure",
		Region:         d.Location,
		State:          InfraState(d.State),
		CreatedAt:      d.StartTime,
		LoadGenerators: loadGenerators,
		ProviderMetadata: map[string]interface{}{
			"resource_group": d.ResourceGroup,
		},
	}, nil
}

// GetInstances returns the current state of instances in the infrastructure.
func (p *AzureProvider) GetInstances(ctx context.Context, infra *Infrastructure) ([]Instance, error) {
	p.mu.RLock()
	d, ok := p.deployments[infra.ID]
	p.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("infrastructure not found: %s", infra.ID)
	}

	instances := make([]Instance, 0, len(d.VMs))
	for _, vm := range d.VMs {
		instances = append(instances, Instance{
			ID:        vm.ID,
			Name:      vm.Name,
			Type:      d.Spec.LoadGenerator.InstanceType,
			State:     InstanceState(vm.State),
			PublicIP:  vm.PublicIP,
			PrivateIP: vm.PrivateIP,
			Role:      "load_generator",
		})
	}

	return instances, nil
}

// ListRegions returns available Azure regions.
func (p *AzureProvider) ListRegions(ctx context.Context) ([]Region, error) {
	return []Region{
		{ID: "eastus", Name: "East US", Available: true},
		{ID: "eastus2", Name: "East US 2", Available: true},
		{ID: "westus", Name: "West US", Available: true},
		{ID: "westus2", Name: "West US 2", Available: true},
		{ID: "westeurope", Name: "West Europe", Available: true},
		{ID: "northeurope", Name: "North Europe", Available: true},
		{ID: "southeastasia", Name: "Southeast Asia", Available: true},
	}, nil
}

// ListInstanceTypes returns available Azure VM sizes.
func (p *AzureProvider) ListInstanceTypes(ctx context.Context, region string) ([]InstanceType, error) {
	return []InstanceType{
		{ID: "Standard_D2s_v3", Name: "D2s v3", VCPUs: 2, MemoryGB: 8, NetworkGbps: 4},
		{ID: "Standard_D4s_v3", Name: "D4s v3", VCPUs: 4, MemoryGB: 16, NetworkGbps: 8},
		{ID: "Standard_D8s_v3", Name: "D8s v3", VCPUs: 8, MemoryGB: 32, NetworkGbps: 16},
		{ID: "Standard_F4s_v2", Name: "F4s v2 (Compute)", VCPUs: 4, MemoryGB: 8, NetworkGbps: 12.5},
		{ID: "Standard_F8s_v2", Name: "F8s v2 (Compute)", VCPUs: 8, MemoryGB: 16, NetworkGbps: 12.5},
		{ID: "Standard_F16s_v2", Name: "F16s v2 (Compute)", VCPUs: 16, MemoryGB: 32, NetworkGbps: 12.5},
	}, nil
}

// EstimateCost provides cost estimation for a deployment.
func (p *AzureProvider) EstimateCost(ctx context.Context, spec *InfraSpec, duration time.Duration) (*CostEstimate, error) {
	// Simplified cost estimation
	// Real implementation would use Azure Pricing API

	hourlyRates := map[string]float64{
		"Standard_D2s_v3":  0.096,
		"Standard_D4s_v3":  0.192,
		"Standard_D8s_v3":  0.384,
		"Standard_F4s_v2":  0.169,
		"Standard_F8s_v2":  0.338,
		"Standard_F16s_v2": 0.677,
	}

	rate, ok := hourlyRates[spec.LoadGenerator.InstanceType]
	if !ok {
		rate = 0.20 // Default estimate
	}

	hours := duration.Hours()
	if hours < 1 {
		hours = 1 // Minimum 1 hour
	}

	vmCost := rate * float64(spec.LoadGenerator.Count) * hours

	// Spot discount (~70%)
	if spec.LoadGenerator.SpotInstances {
		vmCost *= 0.3
	}

	hourlyRate := rate * float64(spec.LoadGenerator.Count)
	if spec.LoadGenerator.SpotInstances {
		hourlyRate *= 0.3
	}

	return &CostEstimate{
		Currency:   "USD",
		HourlyCost: hourlyRate,
		TotalCost:  vmCost,
		Duration:   duration,
		Breakdown: []CostBreakdownItem{
			{
				ResourceType: "compute",
				Description:  fmt.Sprintf("%dx %s VMs", spec.LoadGenerator.Count, spec.LoadGenerator.InstanceType),
				Count:        spec.LoadGenerator.Count,
				UnitCost:     rate,
				TotalCost:    vmCost,
			},
		},
	}, nil
}
