// Package cloud provides Azure Managed Redis (AMR) provisioning and connection management.
package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v5"
	"github.com/tfindelkind-redis/redismeter/internal/domain"
)

// =============================================================================
// Azure Managed Redis (AMR) Configuration
// =============================================================================

// AMRTargetMode defines how to connect to Azure Managed Redis.
type AMRTargetMode string

const (
	// AMRModeProvision creates a new AMR instance using a template.
	AMRModeProvision AMRTargetMode = "provision"
	// AMRModeExisting connects to an existing AMR instance by resource ID.
	AMRModeExisting AMRTargetMode = "existing"
	// AMRModeEndpoint connects to any Redis using a direct endpoint (user's responsibility for networking).
	AMRModeEndpoint AMRTargetMode = "endpoint"
)

// AMRConfig holds Azure Managed Redis configuration.
type AMRConfig struct {
	// Mode determines how to connect to Redis.
	Mode AMRTargetMode `json:"mode"`

	// For AMRModeProvision: Template to use for creating new AMR instance.
	// Templates define functional characteristics (HA, persistence, modules).
	Template AMRTemplate `json:"template,omitempty"`

	// For AMRModeProvision: SKU determines performance and cost.
	// User selects SKU independently from template.
	// Examples: Balanced_B5, ComputeOptimized_X10, MemoryOptimized_M20
	SKU string `json:"sku,omitempty"`

	// For AMRModeProvision: Cluster capacity (optional, defaults based on SKU).
	// For Balanced/Compute/Memory: 2, 4, 6, etc.
	// For Flash: 3, 9, 15, etc.
	Capacity int `json:"capacity,omitempty"`

	// For AMRModeExisting: Azure resource ID of existing AMR instance.
	// Format: /subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Cache/redisEnterprise/{name}
	ResourceID string `json:"resource_id,omitempty"`

	// For AMRModeEndpoint: Direct connection details.
	Endpoint string `json:"endpoint,omitempty"`
	Port     int    `json:"port,omitempty"`
	Password string `json:"password,omitempty"`

	// Network configuration (required for Provision and Existing modes).
	Network *AMRNetworkConfig `json:"network,omitempty"`
}

// AMRNetworkConfig defines networking for AMR connectivity.
type AMRNetworkConfig struct {
	// UseExistingVNet - if true, deploy runner VMs into an existing VNet.
	UseExistingVNet bool `json:"use_existing_vnet,omitempty"`

	// ExistingVNetID - Resource ID of existing VNet (required if UseExistingVNet=true).
	// Format: /subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Network/virtualNetworks/{name}
	ExistingVNetID string `json:"existing_vnet_id,omitempty"`

	// ExistingSubnetName - Name of subnet within the VNet for runner VMs.
	ExistingSubnetName string `json:"existing_subnet_name,omitempty"`

	// CreatePrivateEndpoint - Create a private endpoint for AMR in the runner VNet.
	// This is required if AMR doesn't have public access enabled.
	CreatePrivateEndpoint bool `json:"create_private_endpoint,omitempty"`

	// PrivateEndpointSubnetName - Subnet for the private endpoint (can be same as runner subnet).
	PrivateEndpointSubnetName string `json:"private_endpoint_subnet_name,omitempty"`

	// AllowPublicAccess - For new AMR instances, whether to allow public network access.
	// Recommended: false (use private endpoints instead).
	AllowPublicAccess bool `json:"allow_public_access,omitempty"`
}

// =============================================================================
// AMR Templates - Functional deployment patterns (SKU selected separately)
// =============================================================================

// AMRTemplate defines pre-configured AMR functional patterns.
// Templates focus on WHAT the Redis instance does (HA, persistence, modules).
// SKU is selected separately to control HOW MUCH capacity/performance.
type AMRTemplate string

const (
	// AMRTemplateDevTest - Development/testing setup (No HA, no persistence).
	// Use case: Local development, CI/CD testing, experiments.
	AMRTemplateDevTest AMRTemplate = "dev-test"

	// AMRTemplateStandard - Production-ready with HA but no persistence.
	// Use case: Caching, session storage where data loss is acceptable.
	AMRTemplateStandard AMRTemplate = "standard"

	// AMRTemplateDurable - Production with HA and RDB persistence (hourly snapshots).
	// Use case: Important cached data, acceptable to lose up to 1 hour of data.
	AMRTemplateDurable AMRTemplate = "durable"

	// AMRTemplateHighDurability - Production with HA and AOF persistence (1s writes).
	// Use case: Critical data, minimal acceptable data loss.
	AMRTemplateHighDurability AMRTemplate = "high-durability"

	// AMRTemplateSearch - Optimized for search workloads with RediSearch module.
	// Use case: Full-text search, vector search, document indexing.
	AMRTemplateSearch AMRTemplate = "search"
)

// AMRTemplateSpec contains the functional specification for an AMR template.
// Note: SKU and Capacity are NOT part of templates - user selects separately.
type AMRTemplateSpec struct {
	Name        AMRTemplate `json:"name"`
	Description string      `json:"description"`
	UseCase     string      `json:"use_case"`

	// Cluster settings
	HighAvailability bool     `json:"high_availability"`
	Zones            []string `json:"zones,omitempty"` // Only if HA enabled

	// Database settings
	ClusteringPolicy string `json:"clustering_policy"` // OSSCluster, EnterpriseCluster, NoCluster
	EvictionPolicy   string `json:"eviction_policy"`   // VolatileLRU, AllKeysLRU, NoEviction, etc.

	// Persistence
	PersistenceType string `json:"persistence_type,omitempty"` // "", "rdb", "aof"
	RDBFrequency    string `json:"rdb_frequency,omitempty"`    // 1h, 6h, 12h
	AOFFrequency    string `json:"aof_frequency,omitempty"`    // 1s

	// Modules to enable
	Modules []string `json:"modules,omitempty"` // RedisJSON, RediSearch, RedisBloom, RedisTimeSeries
}

// GetAMRTemplates returns all available AMR functional templates.
func GetAMRTemplates() map[AMRTemplate]AMRTemplateSpec {
	return map[AMRTemplate]AMRTemplateSpec{
		AMRTemplateDevTest: {
			Name:             AMRTemplateDevTest,
			Description:      "Development/Test - No HA, no persistence, minimal setup",
			UseCase:          "Local development, CI/CD testing, experiments",
			HighAvailability: false,
			ClusteringPolicy: "OSSCluster",
			EvictionPolicy:   "VolatileLRU",
			PersistenceType:  "",
		},
		AMRTemplateStandard: {
			Name:             AMRTemplateStandard,
			Description:      "Standard - HA enabled, no persistence",
			UseCase:          "Caching, session storage, ephemeral data",
			HighAvailability: true,
			Zones:            []string{"1", "2", "3"},
			ClusteringPolicy: "OSSCluster",
			EvictionPolicy:   "VolatileLRU",
			PersistenceType:  "",
		},
		AMRTemplateDurable: {
			Name:             AMRTemplateDurable,
			Description:      "Durable - HA with RDB persistence (hourly snapshots)",
			UseCase:          "Important cached data, acceptable hourly data loss",
			HighAvailability: true,
			Zones:            []string{"1", "2", "3"},
			ClusteringPolicy: "OSSCluster",
			EvictionPolicy:   "VolatileLRU",
			PersistenceType:  "rdb",
			RDBFrequency:     "1h",
		},
		AMRTemplateHighDurability: {
			Name:             AMRTemplateHighDurability,
			Description:      "High Durability - HA with AOF persistence (1s writes)",
			UseCase:          "Critical data, minimal data loss tolerance",
			HighAvailability: true,
			Zones:            []string{"1", "2", "3"},
			ClusteringPolicy: "OSSCluster",
			EvictionPolicy:   "NoEviction",
			PersistenceType:  "aof",
			AOFFrequency:     "1s",
		},
		AMRTemplateSearch: {
			Name:             AMRTemplateSearch,
			Description:      "Search - HA with RediSearch for full-text and vector search",
			UseCase:          "Document search, vector similarity, semantic search",
			HighAvailability: true,
			Zones:            []string{"1", "2", "3"},
			ClusteringPolicy: "OSSCluster",
			EvictionPolicy:   "NoEviction",
			PersistenceType:  "rdb",
			RDBFrequency:     "1h",
			Modules:          []string{"RediSearch", "RedisJSON"},
		},
	}
}

// =============================================================================
// AMR SKU Catalog - Available SKUs with descriptions
// =============================================================================

// AMRSKU Family types
const (
	AMRSKUFamilyBalanced         = "Balanced"
	AMRSKUFamilyComputeOptimized = "ComputeOptimized"
	AMRSKUFamilyMemoryOptimized  = "MemoryOptimized"
	AMRSKUFamilyFlashOptimized   = "FlashOptimized"
)

// AMRSKUInfo provides details about an AMR SKU.
type AMRSKUInfo struct {
	Name        string `json:"name"`
	Family      string `json:"family"`
	Description string `json:"description"`
	vCPUs       int    `json:"vcpus"`
	MemoryGB    int    `json:"memory_gb"`
	FlashGB     int    `json:"flash_gb,omitempty"` // Only for Flash-Optimized SKUs
}

// GetAMRSKUs returns the available AMR SKUs with descriptions.
// This list is based on Azure Managed Redis documentation.
func GetAMRSKUs() []AMRSKUInfo {
	return []AMRSKUInfo{
		// Balanced SKUs (B-series) - Good for general purpose workloads
		{Name: "Balanced_B0", Family: AMRSKUFamilyBalanced, Description: "Balanced entry-level (Dev/Test)", vCPUs: 1, MemoryGB: 3},
		{Name: "Balanced_B1", Family: AMRSKUFamilyBalanced, Description: "Balanced small", vCPUs: 1, MemoryGB: 6},
		{Name: "Balanced_B3", Family: AMRSKUFamilyBalanced, Description: "Balanced medium", vCPUs: 1, MemoryGB: 12},
		{Name: "Balanced_B5", Family: AMRSKUFamilyBalanced, Description: "Balanced production", vCPUs: 2, MemoryGB: 24},
		{Name: "Balanced_B10", Family: AMRSKUFamilyBalanced, Description: "Balanced large", vCPUs: 4, MemoryGB: 48},
		{Name: "Balanced_B20", Family: AMRSKUFamilyBalanced, Description: "Balanced xlarge", vCPUs: 8, MemoryGB: 96},
		{Name: "Balanced_B50", Family: AMRSKUFamilyBalanced, Description: "Balanced 2xlarge", vCPUs: 16, MemoryGB: 192},
		{Name: "Balanced_B100", Family: AMRSKUFamilyBalanced, Description: "Balanced 4xlarge", vCPUs: 32, MemoryGB: 384},
		{Name: "Balanced_B150", Family: AMRSKUFamilyBalanced, Description: "Balanced 6xlarge", vCPUs: 48, MemoryGB: 576},
		{Name: "Balanced_B250", Family: AMRSKUFamilyBalanced, Description: "Balanced 8xlarge", vCPUs: 64, MemoryGB: 768},
		{Name: "Balanced_B350", Family: AMRSKUFamilyBalanced, Description: "Balanced 12xlarge", vCPUs: 96, MemoryGB: 1152},
		{Name: "Balanced_B500", Family: AMRSKUFamilyBalanced, Description: "Balanced 16xlarge", vCPUs: 128, MemoryGB: 1536},
		{Name: "Balanced_B700", Family: AMRSKUFamilyBalanced, Description: "Balanced 24xlarge", vCPUs: 192, MemoryGB: 2304},
		{Name: "Balanced_B1000", Family: AMRSKUFamilyBalanced, Description: "Balanced 32xlarge", vCPUs: 256, MemoryGB: 3072},

		// Compute Optimized SKUs (X-series) - High CPU-to-memory ratio for compute-intensive workloads
		{Name: "ComputeOptimized_X3", Family: AMRSKUFamilyComputeOptimized, Description: "Compute-optimized small", vCPUs: 2, MemoryGB: 6},
		{Name: "ComputeOptimized_X5", Family: AMRSKUFamilyComputeOptimized, Description: "Compute-optimized medium", vCPUs: 4, MemoryGB: 12},
		{Name: "ComputeOptimized_X10", Family: AMRSKUFamilyComputeOptimized, Description: "Compute-optimized large", vCPUs: 8, MemoryGB: 24},
		{Name: "ComputeOptimized_X20", Family: AMRSKUFamilyComputeOptimized, Description: "Compute-optimized xlarge", vCPUs: 16, MemoryGB: 48},
		{Name: "ComputeOptimized_X50", Family: AMRSKUFamilyComputeOptimized, Description: "Compute-optimized 2xlarge", vCPUs: 32, MemoryGB: 96},
		{Name: "ComputeOptimized_X100", Family: AMRSKUFamilyComputeOptimized, Description: "Compute-optimized 4xlarge", vCPUs: 64, MemoryGB: 192},
		{Name: "ComputeOptimized_X150", Family: AMRSKUFamilyComputeOptimized, Description: "Compute-optimized 6xlarge", vCPUs: 96, MemoryGB: 288},
		{Name: "ComputeOptimized_X250", Family: AMRSKUFamilyComputeOptimized, Description: "Compute-optimized 8xlarge", vCPUs: 128, MemoryGB: 384},
		{Name: "ComputeOptimized_X350", Family: AMRSKUFamilyComputeOptimized, Description: "Compute-optimized 12xlarge", vCPUs: 192, MemoryGB: 576},
		{Name: "ComputeOptimized_X500", Family: AMRSKUFamilyComputeOptimized, Description: "Compute-optimized 16xlarge", vCPUs: 256, MemoryGB: 768},
		{Name: "ComputeOptimized_X700", Family: AMRSKUFamilyComputeOptimized, Description: "Compute-optimized 24xlarge", vCPUs: 384, MemoryGB: 1152},

		// Memory Optimized SKUs (M-series) - High memory-to-CPU ratio for memory-intensive workloads
		{Name: "MemoryOptimized_M10", Family: AMRSKUFamilyMemoryOptimized, Description: "Memory-optimized small", vCPUs: 2, MemoryGB: 32},
		{Name: "MemoryOptimized_M20", Family: AMRSKUFamilyMemoryOptimized, Description: "Memory-optimized medium", vCPUs: 4, MemoryGB: 64},
		{Name: "MemoryOptimized_M50", Family: AMRSKUFamilyMemoryOptimized, Description: "Memory-optimized large", vCPUs: 8, MemoryGB: 128},
		{Name: "MemoryOptimized_M100", Family: AMRSKUFamilyMemoryOptimized, Description: "Memory-optimized xlarge", vCPUs: 16, MemoryGB: 256},
		{Name: "MemoryOptimized_M150", Family: AMRSKUFamilyMemoryOptimized, Description: "Memory-optimized 2xlarge", vCPUs: 24, MemoryGB: 384},
		{Name: "MemoryOptimized_M250", Family: AMRSKUFamilyMemoryOptimized, Description: "Memory-optimized 4xlarge", vCPUs: 32, MemoryGB: 512},
		{Name: "MemoryOptimized_M350", Family: AMRSKUFamilyMemoryOptimized, Description: "Memory-optimized 6xlarge", vCPUs: 48, MemoryGB: 768},
		{Name: "MemoryOptimized_M500", Family: AMRSKUFamilyMemoryOptimized, Description: "Memory-optimized 8xlarge", vCPUs: 64, MemoryGB: 1024},
		{Name: "MemoryOptimized_M700", Family: AMRSKUFamilyMemoryOptimized, Description: "Memory-optimized 12xlarge", vCPUs: 80, MemoryGB: 1280},
		{Name: "MemoryOptimized_M1000", Family: AMRSKUFamilyMemoryOptimized, Description: "Memory-optimized 16xlarge", vCPUs: 96, MemoryGB: 1536},
		{Name: "MemoryOptimized_M1500", Family: AMRSKUFamilyMemoryOptimized, Description: "Memory-optimized 24xlarge", vCPUs: 128, MemoryGB: 2048},
		{Name: "MemoryOptimized_M2000", Family: AMRSKUFamilyMemoryOptimized, Description: "Memory-optimized 32xlarge", vCPUs: 176, MemoryGB: 2816},

		// Flash Optimized SKUs (A-series) - Uses NVMe flash for cost-effective large datasets
		{Name: "FlashOptimized_A250", Family: AMRSKUFamilyFlashOptimized, Description: "Flash-optimized entry (RAM + Flash)", vCPUs: 8, MemoryGB: 48, FlashGB: 672},
		{Name: "FlashOptimized_A500", Family: AMRSKUFamilyFlashOptimized, Description: "Flash-optimized medium (RAM + Flash)", vCPUs: 16, MemoryGB: 96, FlashGB: 1536},
		{Name: "FlashOptimized_A700", Family: AMRSKUFamilyFlashOptimized, Description: "Flash-optimized large (RAM + Flash)", vCPUs: 24, MemoryGB: 144, FlashGB: 2304},
		{Name: "FlashOptimized_A1000", Family: AMRSKUFamilyFlashOptimized, Description: "Flash-optimized xlarge (RAM + Flash)", vCPUs: 32, MemoryGB: 192, FlashGB: 3072},
		{Name: "FlashOptimized_A1500", Family: AMRSKUFamilyFlashOptimized, Description: "Flash-optimized 2xlarge (RAM + Flash)", vCPUs: 48, MemoryGB: 288, FlashGB: 4608},
		{Name: "FlashOptimized_A2250", Family: AMRSKUFamilyFlashOptimized, Description: "Flash-optimized 4xlarge (RAM + Flash)", vCPUs: 64, MemoryGB: 384, FlashGB: 6912},
		{Name: "FlashOptimized_A4500", Family: AMRSKUFamilyFlashOptimized, Description: "Flash-optimized 8xlarge (RAM + Flash)", vCPUs: 128, MemoryGB: 768, FlashGB: 13824},
	}
}

// ValidateAMRSKU checks if a SKU name is valid.
func ValidateAMRSKU(sku string) error {
	for _, s := range GetAMRSKUs() {
		if s.Name == sku {
			return nil
		}
	}
	return fmt.Errorf("invalid SKU: %s - use GetAMRSKUs() to see available options", sku)
}

// GetAMRSKUsByFamily returns SKUs filtered by family.
func GetAMRSKUsByFamily(family string) []AMRSKUInfo {
	var result []AMRSKUInfo
	for _, sku := range GetAMRSKUs() {
		if sku.Family == family {
			result = append(result, sku)
		}
	}
	return result
}

// =============================================================================
// AMR API Types (matching Azure REST API 2025-07-01)
// =============================================================================

// AMRCluster represents an Azure Managed Redis cluster.
type AMRCluster struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Location   string            `json:"location"`
	Tags       map[string]string `json:"tags,omitempty"`
	SKU        *AMRSKU           `json:"sku"`
	Zones      []string          `json:"zones,omitempty"`
	Properties *AMRClusterProps  `json:"properties"`
}

// AMRSKU defines the SKU for AMR.
type AMRSKU struct {
	Name     string `json:"name"`     // e.g., Balanced_B5, ComputeOptimized_X10
	Capacity int    `json:"capacity"` // Cluster capacity
}

// AMRClusterProps contains cluster properties.
type AMRClusterProps struct {
	ProvisioningState    string `json:"provisioningState,omitempty"`
	ResourceState        string `json:"resourceState,omitempty"`
	HostName             string `json:"hostName,omitempty"`
	RedisVersion         string `json:"redisVersion,omitempty"`
	MinimumTlsVersion    string `json:"minimumTlsVersion,omitempty"`
	HighAvailability     string `json:"highAvailability,omitempty"` // Enabled, Disabled
	PublicNetworkAccess  string `json:"publicNetworkAccess,omitempty"`
	PrivateEndpointConns []struct {
		ID string `json:"id"`
	} `json:"privateEndpointConnections,omitempty"`
}

// AMRDatabase represents a database within an AMR cluster.
type AMRDatabase struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	Type       string           `json:"type"`
	Properties *AMRDatabaseProps `json:"properties"`
}

// AMRDatabaseProps contains database properties.
type AMRDatabaseProps struct {
	ProvisioningState        string          `json:"provisioningState,omitempty"`
	ResourceState            string          `json:"resourceState,omitempty"`
	ClientProtocol           string          `json:"clientProtocol,omitempty"` // Encrypted, Plaintext
	Port                     int             `json:"port,omitempty"`
	ClusteringPolicy         string          `json:"clusteringPolicy,omitempty"`
	EvictionPolicy           string          `json:"evictionPolicy,omitempty"`
	Persistence              *AMRPersistence `json:"persistence,omitempty"`
	Modules                  []AMRModule     `json:"modules,omitempty"`
	AccessKeysAuthentication string          `json:"accessKeysAuthentication,omitempty"`
	RedisVersion             string          `json:"redisVersion,omitempty"`
}

// AMRPersistence defines persistence settings.
type AMRPersistence struct {
	AOFEnabled   bool   `json:"aofEnabled,omitempty"`
	AOFFrequency string `json:"aofFrequency,omitempty"`
	RDBEnabled   bool   `json:"rdbEnabled,omitempty"`
	RDBFrequency string `json:"rdbFrequency,omitempty"`
}

// AMRModule defines a Redis module.
type AMRModule struct {
	Name    string `json:"name"`
	Args    string `json:"args,omitempty"`
	Version string `json:"version,omitempty"`
}

// AMRAccessKeys contains the access keys for an AMR database.
type AMRAccessKeys struct {
	PrimaryKey   string `json:"primaryKey"`
	SecondaryKey string `json:"secondaryKey"`
}

// =============================================================================
// AMR Manager - Handles all AMR operations
// =============================================================================

// AMRManager manages Azure Managed Redis operations.
type AMRManager struct {
	subscriptionID string
	credential     *azidentity.DefaultAzureCredential
	httpClient     *http.Client
	apiVersion     string
}

// NewAMRManager creates a new AMR manager.
func NewAMRManager(subscriptionID string) (*AMRManager, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get Azure credentials: %w", err)
	}

	return &AMRManager{
		subscriptionID: subscriptionID,
		credential:     cred,
		httpClient:     &http.Client{Timeout: 5 * time.Minute},
		apiVersion:     "2025-07-01",
	}, nil
}

// =============================================================================
// AMR Provisioning
// =============================================================================

// ProvisionAMR creates a new Azure Managed Redis instance.
// Parameters:
//   - template: Defines functional characteristics (HA, persistence, modules)
//   - sku: Defines performance tier (user selects from GetAMRSKUs())
//   - capacity: Cluster capacity (typically 2 for non-Flash, 3/9/15 for Flash)
//   - network: Optional network configuration for private access
func (m *AMRManager) ProvisionAMR(ctx context.Context, resourceGroup, name, location string, template AMRTemplate, sku string, capacity int, network *AMRNetworkConfig) (*AMRCluster, *AMRDatabase, error) {
	// Validate template
	templates := GetAMRTemplates()
	spec, ok := templates[template]
	if !ok {
		return nil, nil, fmt.Errorf("unknown AMR template: %s", template)
	}

	// Validate SKU
	if err := ValidateAMRSKU(sku); err != nil {
		return nil, nil, err
	}

	// Default capacity if not specified
	if capacity == 0 {
		// Flash SKUs require capacity in multiples of 3 (3, 9, 15...)
		if strings.HasPrefix(sku, "FlashOptimized_") {
			capacity = 3
		} else {
			capacity = 2
		}
	}

	// Step 1: Create the cluster
	cluster, err := m.createCluster(ctx, resourceGroup, name, location, &spec, sku, capacity, network)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create AMR cluster: %w", err)
	}

	// Step 2: Wait for cluster to be ready
	cluster, err = m.waitForClusterReady(ctx, resourceGroup, name, 15*time.Minute)
	if err != nil {
		return nil, nil, fmt.Errorf("cluster failed to become ready: %w", err)
	}

	// Step 3: Create the database
	database, err := m.createDatabase(ctx, resourceGroup, name, "default", &spec)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create AMR database: %w", err)
	}

	// Step 4: Wait for database to be ready
	database, err = m.waitForDatabaseReady(ctx, resourceGroup, name, "default", 10*time.Minute)
	if err != nil {
		return nil, nil, fmt.Errorf("database failed to become ready: %w", err)
	}

	return cluster, database, nil
}

// createCluster creates an AMR cluster using the REST API.
func (m *AMRManager) createCluster(ctx context.Context, resourceGroup, name, location string, spec *AMRTemplateSpec, sku string, capacity int, network *AMRNetworkConfig) (*AMRCluster, error) {
	url := fmt.Sprintf(
		"https://management.azure.com/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Cache/redisEnterprise/%s?api-version=%s",
		m.subscriptionID, resourceGroup, name, m.apiVersion,
	)

	// Build cluster request
	haValue := "Disabled"
	if spec.HighAvailability {
		haValue = "Enabled"
	}

	publicAccess := "Disabled"
	if network != nil && network.AllowPublicAccess {
		publicAccess = "Enabled"
	}

	cluster := &AMRCluster{
		Location: location,
		SKU: &AMRSKU{
			Name:     sku,
			Capacity: capacity,
		},
		Zones: spec.Zones,
		Properties: &AMRClusterProps{
			HighAvailability:    haValue,
			MinimumTlsVersion:   "1.2",
			PublicNetworkAccess: publicAccess,
		},
		Tags: map[string]string{
			"managed-by": "redismeter",
			"template":   string(spec.Name),
			"sku":        sku,
		},
	}

	body, err := json.Marshal(cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cluster: %w", err)
	}

	resp, err := m.doRequest(ctx, "PUT", url, body)
	if err != nil {
		return nil, err
	}

	var result AMRCluster
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// createDatabase creates a database within an AMR cluster.
func (m *AMRManager) createDatabase(ctx context.Context, resourceGroup, clusterName, dbName string, spec *AMRTemplateSpec) (*AMRDatabase, error) {
	url := fmt.Sprintf(
		"https://management.azure.com/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Cache/redisEnterprise/%s/databases/%s?api-version=%s",
		m.subscriptionID, resourceGroup, clusterName, dbName, m.apiVersion,
	)

	// Build persistence settings
	var persistence *AMRPersistence
	if spec.PersistenceType == "rdb" {
		persistence = &AMRPersistence{
			RDBEnabled:   true,
			RDBFrequency: spec.RDBFrequency,
		}
	} else if spec.PersistenceType == "aof" {
		persistence = &AMRPersistence{
			AOFEnabled:   true,
			AOFFrequency: spec.AOFFrequency,
		}
	}

	// Build modules
	var modules []AMRModule
	for _, mod := range spec.Modules {
		modules = append(modules, AMRModule{Name: mod})
	}

	database := &AMRDatabase{
		Properties: &AMRDatabaseProps{
			ClientProtocol:           "Encrypted",
			Port:                     10000,
			ClusteringPolicy:         spec.ClusteringPolicy,
			EvictionPolicy:           spec.EvictionPolicy,
			Persistence:              persistence,
			Modules:                  modules,
			AccessKeysAuthentication: "Enabled",
		},
	}

	body, err := json.Marshal(database)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal database: %w", err)
	}

	resp, err := m.doRequest(ctx, "PUT", url, body)
	if err != nil {
		return nil, err
	}

	var result AMRDatabase
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// =============================================================================
// AMR Lookup (Existing Instances)
// =============================================================================

// GetExistingAMR retrieves an existing AMR instance by resource ID.
func (m *AMRManager) GetExistingAMR(ctx context.Context, resourceID string) (*AMRCluster, *AMRDatabase, error) {
	// Parse resource ID to extract components
	parts := strings.Split(resourceID, "/")
	if len(parts) < 9 {
		return nil, nil, fmt.Errorf("invalid resource ID format: %s", resourceID)
	}

	// Extract subscription, resource group, and cluster name
	var subscriptionID, resourceGroup, clusterName string
	for i, part := range parts {
		switch part {
		case "subscriptions":
			if i+1 < len(parts) {
				subscriptionID = parts[i+1]
			}
		case "resourceGroups":
			if i+1 < len(parts) {
				resourceGroup = parts[i+1]
			}
		case "redisEnterprise":
			if i+1 < len(parts) {
				clusterName = parts[i+1]
			}
		}
	}

	if subscriptionID == "" || resourceGroup == "" || clusterName == "" {
		return nil, nil, fmt.Errorf("could not parse resource ID: %s", resourceID)
	}

	// Get cluster
	cluster, err := m.getCluster(ctx, resourceGroup, clusterName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	// Get default database
	database, err := m.getDatabase(ctx, resourceGroup, clusterName, "default")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get database: %w", err)
	}

	return cluster, database, nil
}

// getCluster retrieves an AMR cluster.
func (m *AMRManager) getCluster(ctx context.Context, resourceGroup, name string) (*AMRCluster, error) {
	url := fmt.Sprintf(
		"https://management.azure.com/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Cache/redisEnterprise/%s?api-version=%s",
		m.subscriptionID, resourceGroup, name, m.apiVersion,
	)

	resp, err := m.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var cluster AMRCluster
	if err := json.Unmarshal(resp, &cluster); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cluster: %w", err)
	}

	return &cluster, nil
}

// getDatabase retrieves an AMR database.
func (m *AMRManager) getDatabase(ctx context.Context, resourceGroup, clusterName, dbName string) (*AMRDatabase, error) {
	url := fmt.Sprintf(
		"https://management.azure.com/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Cache/redisEnterprise/%s/databases/%s?api-version=%s",
		m.subscriptionID, resourceGroup, clusterName, dbName, m.apiVersion,
	)

	resp, err := m.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var database AMRDatabase
	if err := json.Unmarshal(resp, &database); err != nil {
		return nil, fmt.Errorf("failed to unmarshal database: %w", err)
	}

	return &database, nil
}

// GetAccessKeys retrieves the access keys for an AMR database.
func (m *AMRManager) GetAccessKeys(ctx context.Context, resourceGroup, clusterName, dbName string) (*AMRAccessKeys, error) {
	url := fmt.Sprintf(
		"https://management.azure.com/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Cache/redisEnterprise/%s/databases/%s/listKeys?api-version=%s",
		m.subscriptionID, resourceGroup, clusterName, dbName, m.apiVersion,
	)

	resp, err := m.doRequest(ctx, "POST", url, nil)
	if err != nil {
		return nil, err
	}

	var keys AMRAccessKeys
	if err := json.Unmarshal(resp, &keys); err != nil {
		return nil, fmt.Errorf("failed to unmarshal keys: %w", err)
	}

	return &keys, nil
}

// =============================================================================
// Network Operations
// =============================================================================

// CreatePrivateEndpoint creates a private endpoint for AMR in the specified VNet/subnet.
func (m *AMRManager) CreatePrivateEndpoint(ctx context.Context, resourceGroup, name, location string, clusterID string, vnetID, subnetName string) error {
	// Use the network client to create private endpoint
	networkClient, err := armnetwork.NewPrivateEndpointsClient(m.subscriptionID, m.credential, nil)
	if err != nil {
		return fmt.Errorf("failed to create private endpoints client: %w", err)
	}

	subnetID := fmt.Sprintf("%s/subnets/%s", vnetID, subnetName)

	poller, err := networkClient.BeginCreateOrUpdate(ctx, resourceGroup, name, armnetwork.PrivateEndpoint{
		Location: to.Ptr(location),
		Properties: &armnetwork.PrivateEndpointProperties{
			Subnet: &armnetwork.Subnet{
				ID: to.Ptr(subnetID),
			},
			PrivateLinkServiceConnections: []*armnetwork.PrivateLinkServiceConnection{
				{
					Name: to.Ptr(fmt.Sprintf("%s-connection", name)),
					Properties: &armnetwork.PrivateLinkServiceConnectionProperties{
						PrivateLinkServiceID: to.Ptr(clusterID),
						GroupIDs:             []*string{to.Ptr("redisEnterprise")},
					},
				},
			},
		},
		Tags: map[string]*string{
			"managed-by": to.Ptr("redismeter"),
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to start private endpoint creation: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create private endpoint: %w", err)
	}

	return nil
}

// =============================================================================
// Wait Operations
// =============================================================================

// waitForClusterReady polls until the cluster is in Running state.
func (m *AMRManager) waitForClusterReady(ctx context.Context, resourceGroup, name string, timeout time.Duration) (*AMRCluster, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		cluster, err := m.getCluster(ctx, resourceGroup, name)
		if err != nil {
			return nil, err
		}

		if cluster.Properties.ResourceState == "Running" {
			return cluster, nil
		}

		if cluster.Properties.ResourceState == "CreateFailed" {
			return nil, fmt.Errorf("cluster creation failed")
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(30 * time.Second):
			// Continue polling
		}
	}

	return nil, fmt.Errorf("timeout waiting for cluster to be ready")
}

// waitForDatabaseReady polls until the database is in Running state.
func (m *AMRManager) waitForDatabaseReady(ctx context.Context, resourceGroup, clusterName, dbName string, timeout time.Duration) (*AMRDatabase, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		database, err := m.getDatabase(ctx, resourceGroup, clusterName, dbName)
		if err != nil {
			return nil, err
		}

		if database.Properties.ResourceState == "Running" {
			return database, nil
		}

		if database.Properties.ResourceState == "CreateFailed" {
			return nil, fmt.Errorf("database creation failed")
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(15 * time.Second):
			// Continue polling
		}
	}

	return nil, fmt.Errorf("timeout waiting for database to be ready")
}

// =============================================================================
// Delete Operations
// =============================================================================

// DeleteAMR deletes an AMR cluster (and all its databases).
func (m *AMRManager) DeleteAMR(ctx context.Context, resourceGroup, name string) error {
	url := fmt.Sprintf(
		"https://management.azure.com/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Cache/redisEnterprise/%s?api-version=%s",
		m.subscriptionID, resourceGroup, name, m.apiVersion,
	)

	_, err := m.doRequest(ctx, "DELETE", url, nil)
	return err
}

// =============================================================================
// HTTP Helpers
// =============================================================================

// doRequest performs an authenticated HTTP request to the Azure REST API.
func (m *AMRManager) doRequest(ctx context.Context, method, url string, body []byte) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = strings.NewReader(string(body))
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Get access token
	token, err := m.credential.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{"https://management.azure.com/.default"},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// =============================================================================
// Integration with Azure Provider
// =============================================================================

// ResolveAMRTarget resolves AMR configuration to a domain.Target for benchmarking.
func (m *AMRManager) ResolveAMRTarget(ctx context.Context, config *AMRConfig, resourceGroup, location string, runnerVNetID string) (*domain.Target, *AMRCluster, error) {
	switch config.Mode {
	case AMRModeProvision:
		return m.resolveProvisionMode(ctx, config, resourceGroup, location, runnerVNetID)

	case AMRModeExisting:
		return m.resolveExistingMode(ctx, config, resourceGroup, location, runnerVNetID)

	case AMRModeEndpoint:
		return m.resolveEndpointMode(config)

	default:
		return nil, nil, fmt.Errorf("unknown AMR mode: %s", config.Mode)
	}
}

func (m *AMRManager) resolveProvisionMode(ctx context.Context, config *AMRConfig, resourceGroup, location string, runnerVNetID string) (*domain.Target, *AMRCluster, error) {
	// Generate a unique name for the AMR instance
	name := fmt.Sprintf("rm-amr-%d", time.Now().Unix())

	// Determine SKU - use config.SKU or default to a dev-test SKU
	sku := config.SKU
	if sku == "" {
		sku = "Balanced_B0" // Default to smallest for safety
	}

	// Determine capacity - use config.Capacity or let ProvisionAMR set default
	capacity := config.Capacity

	// Provision new AMR
	cluster, database, err := m.ProvisionAMR(ctx, resourceGroup, name, location, config.Template, sku, capacity, config.Network)
	if err != nil {
		return nil, nil, err
	}

	// Create private endpoint if needed
	if config.Network != nil && config.Network.CreatePrivateEndpoint && runnerVNetID != "" {
		subnetName := config.Network.PrivateEndpointSubnetName
		if subnetName == "" {
			subnetName = "default"
		}
		err = m.CreatePrivateEndpoint(ctx, resourceGroup, name+"-pe", location, cluster.ID, runnerVNetID, subnetName)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create private endpoint: %w", err)
		}
	}

	// Get access keys
	keys, err := m.GetAccessKeys(ctx, resourceGroup, name, "default")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get access keys: %w", err)
	}

	// Build target
	target := &domain.Target{
		Host:     cluster.Properties.HostName,
		Port:     database.Properties.Port,
		Password: keys.PrimaryKey,
		TLS: &domain.TLSConfig{
			Enabled: database.Properties.ClientProtocol == "Encrypted",
		},
		Cluster: database.Properties.ClusteringPolicy != "NoCluster",
		Name:    name,
		Labels: map[string]string{
			"provider":  "azure-managed-redis",
			"sku":       cluster.SKU.Name,
			"template":  string(config.Template),
			"location":  location,
			"clusterId": cluster.ID,
		},
	}

	return target, cluster, nil
}

func (m *AMRManager) resolveExistingMode(ctx context.Context, config *AMRConfig, resourceGroup, location string, runnerVNetID string) (*domain.Target, *AMRCluster, error) {
	// Get existing AMR instance
	cluster, database, err := m.GetExistingAMR(ctx, config.ResourceID)
	if err != nil {
		return nil, nil, err
	}

	// Parse resource ID for resource group and name
	parts := strings.Split(config.ResourceID, "/")
	var rg, clusterName string
	for i, part := range parts {
		if part == "resourceGroups" && i+1 < len(parts) {
			rg = parts[i+1]
		}
		if part == "redisEnterprise" && i+1 < len(parts) {
			clusterName = parts[i+1]
		}
	}

	// Create private endpoint if needed and AMR doesn't have public access
	if config.Network != nil && config.Network.CreatePrivateEndpoint && runnerVNetID != "" {
		if cluster.Properties.PublicNetworkAccess != "Enabled" {
			subnetName := config.Network.PrivateEndpointSubnetName
			if subnetName == "" {
				subnetName = "default"
			}
			err = m.CreatePrivateEndpoint(ctx, resourceGroup, clusterName+"-pe-rm", location, cluster.ID, runnerVNetID, subnetName)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create private endpoint: %w", err)
			}
		}
	}

	// Get access keys
	keys, err := m.GetAccessKeys(ctx, rg, clusterName, "default")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get access keys: %w", err)
	}

	// Build target
	target := &domain.Target{
		Host:     cluster.Properties.HostName,
		Port:     database.Properties.Port,
		Password: keys.PrimaryKey,
		TLS: &domain.TLSConfig{
			Enabled: database.Properties.ClientProtocol == "Encrypted",
		},
		Cluster: database.Properties.ClusteringPolicy != "NoCluster",
		Name:    clusterName,
		Labels: map[string]string{
			"provider":  "azure-managed-redis",
			"sku":       cluster.SKU.Name,
			"location":  cluster.Location,
			"clusterId": cluster.ID,
		},
	}

	return target, cluster, nil
}

func (m *AMRManager) resolveEndpointMode(config *AMRConfig) (*domain.Target, *AMRCluster, error) {
	// Direct endpoint mode - user provides connection details
	port := config.Port
	if port == 0 {
		port = 10000 // Default AMR port
	}

	target := &domain.Target{
		Host:     config.Endpoint,
		Port:     port,
		Password: config.Password,
		TLS: &domain.TLSConfig{
			Enabled: true, // AMR always uses TLS
		},
		Name: "external-redis",
		Labels: map[string]string{
			"provider": "external",
		},
	}

	return target, nil, nil
}
