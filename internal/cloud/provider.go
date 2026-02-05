// Package cloud provides cloud provider infrastructure management for RedisMeter.
package cloud

import (
	"context"
	"time"

	"github.com/tfindelkind-redis/redismeter/internal/plugin"
)

// ProviderPlugin defines the interface for cloud provider plugins.
type ProviderPlugin interface {
	plugin.Plugin

	// ListRegions returns available regions for this provider.
	ListRegions(ctx context.Context) ([]Region, error)

	// ListInstanceTypes returns available instance types, optionally filtered by region.
	ListInstanceTypes(ctx context.Context, region string) ([]InstanceType, error)

	// Provision creates infrastructure according to the specification.
	Provision(ctx context.Context, spec *InfraSpec) (*Infrastructure, error)

	// Teardown destroys the specified infrastructure.
	Teardown(ctx context.Context, infra *Infrastructure) error

	// GetInstances returns the current state of instances in the infrastructure.
	GetInstances(ctx context.Context, infra *Infrastructure) ([]Instance, error)

	// GetInfrastructure retrieves infrastructure by ID.
	GetInfrastructure(ctx context.Context, id string) (*Infrastructure, error)

	// EstimateCost provides a cost estimate for the given specification.
	EstimateCost(ctx context.Context, spec *InfraSpec, duration time.Duration) (*CostEstimate, error)
}

// Region represents a cloud provider region.
type Region struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Available   bool   `json:"available"`
}

// InstanceType represents a cloud instance type/size.
type InstanceType struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	VCPUs       int     `json:"vcpus"`
	MemoryGB    float64 `json:"memory_gb"`
	NetworkGbps float64 `json:"network_gbps,omitempty"`
	HourlyPrice float64 `json:"hourly_price,omitempty"`
	Currency    string  `json:"currency,omitempty"`
	Category    string  `json:"category,omitempty"` // general, compute, memory, etc.
}

// InfraSpec defines the desired infrastructure configuration.
type InfraSpec struct {
	// Name is a human-readable identifier for this infrastructure.
	Name string `json:"name"`

	// Provider is the cloud provider (aws, gcp, azure).
	Provider string `json:"provider"`

	// Region is the deployment region.
	Region string `json:"region"`

	// LoadGenerator configuration for benchmark client instances.
	LoadGenerator *NodeGroupSpec `json:"load_generator"`

	// RedisTarget configuration (optional, for provisioning Redis).
	RedisTarget *RedisTargetSpec `json:"redis_target,omitempty"`

	// Network configuration.
	Network *NetworkSpec `json:"network,omitempty"`

	// Tags/labels to apply to all resources.
	Tags map[string]string `json:"tags,omitempty"`

	// TTL is the maximum lifetime of the infrastructure.
	// After this duration, resources will be automatically cleaned up.
	TTL time.Duration `json:"ttl,omitempty"`
}

// NodeGroupSpec defines a group of instances.
type NodeGroupSpec struct {
	// InstanceType is the instance type/size (e.g., Standard_D4s_v3 for Azure).
	InstanceType string `json:"instance_type"`

	// Count is the number of instances.
	Count int `json:"count"`

	// SpotInstances enables use of spot/preemptible instances.
	SpotInstances bool `json:"spot_instances,omitempty"`

	// MaxSpotPrice is the maximum price for spot instances (provider currency).
	MaxSpotPrice float64 `json:"max_spot_price,omitempty"`

	// DiskSizeGB is the root disk size.
	DiskSizeGB int `json:"disk_size_gb,omitempty"`

	// SSHKeyName is the SSH key to use (must exist in provider).
	SSHKeyName string `json:"ssh_key_name,omitempty"`

	// SSHPublicKey is the SSH public key content (alternative to SSHKeyName).
	SSHPublicKey string `json:"ssh_public_key,omitempty"`

	// UserData is cloud-init or startup script (appended to default memtier setup).
	UserData string `json:"user_data,omitempty"`

	// Note: OS Image is NOT configurable - Ubuntu 22.04 LTS is enforced
	// for compatibility with memtier_benchmark installation.
}

// RedisTargetSpec defines Redis target configuration.
type RedisTargetSpec struct {
	// Type is the Redis deployment type.
	Type RedisTargetType `json:"type"`

	// Managed configuration for managed Redis services (legacy: AWS ElastiCache, GCP Memorystore).
	Managed *ManagedRedisSpec `json:"managed,omitempty"`

	// AzureManagedRedis configuration for Azure Managed Redis (AMR).
	// This is the preferred way to configure AMR targets.
	AzureManagedRedis *AzureManagedRedisSpec `json:"azure_managed_redis,omitempty"`

	// SelfHosted configuration for self-managed Redis.
	SelfHosted *SelfHostedRedisSpec `json:"self_hosted,omitempty"`

	// ExistingEndpoint connects to an existing Redis instance.
	// For AMR, use AzureManagedRedis with Mode=existing instead.
	ExistingEndpoint string `json:"existing_endpoint,omitempty"`
}

// RedisTargetType defines the type of Redis target.
type RedisTargetType string

const (
	RedisTargetManaged           RedisTargetType = "managed"             // AWS ElastiCache, GCP Memorystore, etc.
	RedisTargetAzureManagedRedis RedisTargetType = "azure_managed_redis" // Azure Managed Redis (AMR)
	RedisTargetSelfHosted        RedisTargetType = "self_hosted"         // Redis on VMs
	RedisTargetExisting          RedisTargetType = "existing"            // Connect to existing (direct endpoint)
)

// AzureManagedRedisSpec defines Azure Managed Redis (AMR) configuration.
// AMR is based on Redis Enterprise and provides enterprise-grade features.
type AzureManagedRedisSpec struct {
	// Mode determines how to connect to Redis.
	// - "provision": Create a new AMR instance using Template
	// - "existing": Connect to an existing AMR instance using ResourceID
	// - "endpoint": Connect to any Redis using direct Endpoint (user handles networking)
	Mode string `json:"mode"` // provision, existing, endpoint

	// Template to use for creating new AMR instance (for mode=provision).
	// Available templates: dev-test, balanced, balanced-ha, high-performance, memory-optimized
	Template string `json:"template,omitempty"`

	// ResourceID of existing AMR cluster (for mode=existing).
	// Format: /subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Cache/redisEnterprise/{name}
	ResourceID string `json:"resource_id,omitempty"`

	// Endpoint for direct connection (for mode=endpoint).
	Endpoint string `json:"endpoint,omitempty"`
	Port     int    `json:"port,omitempty"`
	Password string `json:"password,omitempty"`

	// Network configuration for AMR connectivity.
	Network *AzureManagedRedisNetworkSpec `json:"network,omitempty"`
}

// AzureManagedRedisNetworkSpec defines networking for AMR connectivity.
type AzureManagedRedisNetworkSpec struct {
	// UseExistingVNet - if true, deploy runner VMs into an existing VNet.
	// This is required when AMR doesn't have public access enabled.
	UseExistingVNet bool `json:"use_existing_vnet,omitempty"`

	// ExistingVNetID - Resource ID of existing VNet (required if UseExistingVNet=true).
	// Format: /subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Network/virtualNetworks/{name}
	ExistingVNetID string `json:"existing_vnet_id,omitempty"`

	// ExistingSubnetName - Name of subnet within the VNet for runner VMs.
	ExistingSubnetName string `json:"existing_subnet_name,omitempty"`

	// CreatePrivateEndpoint - Create a private endpoint for AMR in the runner VNet.
	// Required if AMR doesn't have public access enabled and VNets are different.
	CreatePrivateEndpoint bool `json:"create_private_endpoint,omitempty"`

	// PrivateEndpointSubnetName - Subnet for the private endpoint.
	PrivateEndpointSubnetName string `json:"private_endpoint_subnet_name,omitempty"`

	// AllowPublicAccess - For new AMR instances, whether to allow public network access.
	// Recommended: false (use private endpoints instead for production).
	AllowPublicAccess bool `json:"allow_public_access,omitempty"`
}

// ManagedRedisSpec defines managed Redis service configuration.
type ManagedRedisSpec struct {
	// NodeType is the managed service node type.
	NodeType string `json:"node_type"`

	// NumNodes is the number of nodes (for cluster mode).
	NumNodes int `json:"num_nodes,omitempty"`

	// ReplicationEnabled enables replication.
	ReplicationEnabled bool `json:"replication_enabled,omitempty"`

	// ClusterEnabled enables cluster mode.
	ClusterEnabled bool `json:"cluster_enabled,omitempty"`

	// EngineVersion is the Redis version.
	EngineVersion string `json:"engine_version,omitempty"`
}

// SelfHostedRedisSpec defines self-hosted Redis configuration.
type SelfHostedRedisSpec struct {
	// Nodes is the node group for Redis servers.
	Nodes *NodeGroupSpec `json:"nodes"`

	// Version is the Redis version to install.
	Version string `json:"version,omitempty"`

	// ClusterEnabled enables Redis cluster mode.
	ClusterEnabled bool `json:"cluster_enabled,omitempty"`
}

// NetworkSpec defines network configuration.
type NetworkSpec struct {
	// VPCID is an existing VPC to use (optional).
	VPCID string `json:"vpc_id,omitempty"`

	// SubnetID is an existing subnet to use (optional).
	SubnetID string `json:"subnet_id,omitempty"`

	// CIDR for new VPC (if not using existing).
	CIDR string `json:"cidr,omitempty"`

	// AllowSSHFrom restricts SSH access to these CIDRs.
	AllowSSHFrom []string `json:"allow_ssh_from,omitempty"`

	// AllowRedisFrom restricts Redis access to these CIDRs.
	AllowRedisFrom []string `json:"allow_redis_from,omitempty"`
}

// Infrastructure represents provisioned cloud infrastructure.
type Infrastructure struct {
	// ID is a unique identifier for this infrastructure.
	ID string `json:"id"`

	// Name is the human-readable name.
	Name string `json:"name"`

	// Provider is the cloud provider.
	Provider string `json:"provider"`

	// Region is the deployment region.
	Region string `json:"region"`

	// State is the current state of the infrastructure.
	State InfraState `json:"state"`

	// CreatedAt is when the infrastructure was created.
	CreatedAt time.Time `json:"created_at"`

	// ExpiresAt is when the infrastructure will be automatically torn down.
	ExpiresAt *time.Time `json:"expires_at,omitempty"`

	// LoadGenerators are the benchmark client instances.
	LoadGenerators []Instance `json:"load_generators,omitempty"`

	// RedisEndpoint is the Redis connection endpoint.
	RedisEndpoint string `json:"redis_endpoint,omitempty"`

	// RedisPort is the Redis port.
	RedisPort int `json:"redis_port,omitempty"`

	// Network contains network resource IDs.
	Network *NetworkInfo `json:"network,omitempty"`

	// Tags applied to resources.
	Tags map[string]string `json:"tags,omitempty"`

	// ProviderMetadata contains provider-specific information.
	ProviderMetadata map[string]interface{} `json:"provider_metadata,omitempty"`

	// Cost tracking information.
	Cost *CostInfo `json:"cost,omitempty"`
}

// InfraState represents the lifecycle state of infrastructure.
type InfraState string

const (
	InfraStateProvisioning InfraState = "provisioning"
	InfraStateReady        InfraState = "ready"
	InfraStateTearingDown  InfraState = "tearing_down"
	InfraStateTerminated   InfraState = "terminated"
	InfraStateFailed       InfraState = "failed"
)

// Instance represents a cloud instance.
type Instance struct {
	// ID is the provider's instance ID.
	ID string `json:"id"`

	// Name is the instance name/hostname.
	Name string `json:"name"`

	// Type is the instance type.
	Type string `json:"type"`

	// State is the instance state.
	State InstanceState `json:"state"`

	// PublicIP is the public IP address.
	PublicIP string `json:"public_ip,omitempty"`

	// PrivateIP is the private IP address.
	PrivateIP string `json:"private_ip,omitempty"`

	// LaunchTime is when the instance was launched.
	LaunchTime time.Time `json:"launch_time"`

	// Role describes the instance purpose (load_generator, redis, etc.).
	Role string `json:"role"`
}

// InstanceState represents the state of a cloud instance.
type InstanceState string

const (
	InstanceStatePending    InstanceState = "pending"
	InstanceStateRunning    InstanceState = "running"
	InstanceStateStopping   InstanceState = "stopping"
	InstanceStateStopped    InstanceState = "stopped"
	InstanceStateTerminated InstanceState = "terminated"
)

// NetworkInfo contains network resource information.
type NetworkInfo struct {
	VPCID           string   `json:"vpc_id,omitempty"`
	SubnetIDs       []string `json:"subnet_ids,omitempty"`
	SecurityGroupID string   `json:"security_group_id,omitempty"`
}

// CostInfo tracks infrastructure costs.
type CostInfo struct {
	// Estimated hourly cost.
	HourlyCost float64 `json:"hourly_cost"`

	// Currency code.
	Currency string `json:"currency"`

	// AccumulatedCost since creation.
	AccumulatedCost float64 `json:"accumulated_cost"`

	// LastUpdated is when cost was last calculated.
	LastUpdated time.Time `json:"last_updated"`
}

// CostEstimate provides a cost estimate for infrastructure.
type CostEstimate struct {
	// HourlyCost is the estimated hourly cost.
	HourlyCost float64 `json:"hourly_cost"`

	// TotalCost is the total estimated cost for the duration.
	TotalCost float64 `json:"total_cost"`

	// Duration is the estimation period.
	Duration time.Duration `json:"duration"`

	// Currency code.
	Currency string `json:"currency"`

	// Breakdown by resource type.
	Breakdown []CostBreakdownItem `json:"breakdown,omitempty"`
}

// CostBreakdownItem shows cost for a specific resource type.
type CostBreakdownItem struct {
	ResourceType string  `json:"resource_type"`
	Description  string  `json:"description"`
	Count        int     `json:"count"`
	UnitCost     float64 `json:"unit_cost"`
	TotalCost    float64 `json:"total_cost"`
}
