// Package infraprofile manages reusable infrastructure configuration profiles.
package infraprofile

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Provider represents a cloud provider.
type Provider string

const (
	ProviderAzure  Provider = "azure"
	ProviderAWS    Provider = "aws"
	ProviderGCP    Provider = "gcp"
	ProviderLocal  Provider = "local"
	ProviderCustom Provider = "custom"
)

// ProfileStats contains store statistics.
type ProfileStats struct {
	TotalProfiles   int              `json:"total_profiles"`
	ByProvider      map[Provider]int `json:"by_provider"`
	MostUsed        *Profile         `json:"most_used,omitempty"`
	RecentlyCreated *Profile         `json:"recently_created,omitempty"`
	RecentlyUsed    *Profile         `json:"recently_used,omitempty"`
}

// Profile represents a reusable infrastructure configuration.
type Profile struct {
	// Metadata
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Provider    Provider  `json:"provider"`
	Tags        []string  `json:"tags,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Provider-specific configuration
	Config ProfileConfig `json:"config"`

	// Usage tracking
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	UseCount   int        `json:"use_count"`

	// Built-in flag (read-only profiles)
	IsBuiltin bool `json:"is_builtin,omitempty"`
}

// ProfileConfig holds provider-specific configuration.
type ProfileConfig struct {
	// Azure-specific
	Azure *AzureConfig `json:"azure,omitempty"`

	// AWS-specific
	AWS *AWSConfig `json:"aws,omitempty"`

	// GCP-specific
	GCP *GCPConfig `json:"gcp,omitempty"`

	// Local Redis
	Local *LocalConfig `json:"local,omitempty"`

	// Custom/manual configuration
	Custom *CustomConfig `json:"custom,omitempty"`
}

// AzureConfig holds Azure Cache for Redis configuration.
type AzureConfig struct {
	// Subscription and resource group
	SubscriptionID string `json:"subscription_id,omitempty"`
	ResourceGroup  string `json:"resource_group,omitempty"`
	Location       string `json:"location"`

	// Redis configuration
	SKU      string `json:"sku"`      // Basic, Standard, Premium, Enterprise
	Family   string `json:"family"`   // C (Basic/Standard), P (Premium), E (Enterprise)
	Capacity int    `json:"capacity"` // 0-6 for Basic/Standard, 1-5 for Premium

	// Networking
	EnableNonSSLPort bool   `json:"enable_non_ssl_port"`
	MinTLSVersion    string `json:"min_tls_version,omitempty"` // 1.0, 1.1, 1.2

	// Features (Premium only)
	ShardCount         int  `json:"shard_count,omitempty"`
	ReplicasPerPrimary int  `json:"replicas_per_primary,omitempty"`
	ZoneRedundant      bool `json:"zone_redundant,omitempty"`

	// Benchmark VM configuration
	VMSize     string `json:"vm_size,omitempty"`
	VMCount    int    `json:"vm_count,omitempty"`
	UseSpotVMs bool   `json:"use_spot_vms,omitempty"`
	SSHKeyPath string `json:"ssh_key_path,omitempty"`

	// Estimated costs
	EstimatedMonthlyCost float64 `json:"estimated_monthly_cost,omitempty"`
}

// AWSConfig holds AWS ElastiCache configuration.
type AWSConfig struct {
	Region string `json:"region"`

	// ElastiCache configuration
	NodeType             string `json:"node_type"` // cache.t3.micro, cache.r6g.large, etc.
	NumCacheNodes        int    `json:"num_cache_nodes"`
	Engine               string `json:"engine"` // redis
	EngineVersion        string `json:"engine_version,omitempty"`
	ParameterGroupFamily string `json:"parameter_group_family,omitempty"`

	// Cluster mode
	ClusterEnabled       bool `json:"cluster_enabled"`
	NumNodeGroups        int  `json:"num_node_groups,omitempty"`
	ReplicasPerNodeGroup int  `json:"replicas_per_node_group,omitempty"`

	// Networking
	SubnetGroupName  string   `json:"subnet_group_name,omitempty"`
	SecurityGroupIDs []string `json:"security_group_ids,omitempty"`

	// Benchmark EC2 configuration
	EC2InstanceType  string `json:"ec2_instance_type,omitempty"`
	EC2Count         int    `json:"ec2_count,omitempty"`
	UseSpotInstances bool   `json:"use_spot_instances,omitempty"`
	SSHKeyName       string `json:"ssh_key_name,omitempty"`

	// Estimated costs
	EstimatedMonthlyCost float64 `json:"estimated_monthly_cost,omitempty"`
}

// GCPConfig holds Google Cloud Memorystore configuration.
type GCPConfig struct {
	ProjectID string `json:"project_id,omitempty"`
	Region    string `json:"region"`
	Zone      string `json:"zone,omitempty"`

	// Memorystore configuration
	Tier         string `json:"tier"` // BASIC, STANDARD_HA
	MemorySizeGB int    `json:"memory_size_gb"`
	RedisVersion string `json:"redis_version,omitempty"`
	DisplayName  string `json:"display_name,omitempty"`

	// Networking
	AuthorizedNetwork string `json:"authorized_network,omitempty"`
	ConnectMode       string `json:"connect_mode,omitempty"` // DIRECT_PEERING, PRIVATE_SERVICE_ACCESS

	// Benchmark VM configuration
	MachineType string `json:"machine_type,omitempty"`
	VMCount     int    `json:"vm_count,omitempty"`
	Preemptible bool   `json:"preemptible,omitempty"`

	// Estimated costs
	EstimatedMonthlyCost float64 `json:"estimated_monthly_cost,omitempty"`
}

// LocalConfig holds configuration for local Redis instances.
type LocalConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Password string `json:"password,omitempty"`
	TLS      bool   `json:"tls"`
	Database int    `json:"database,omitempty"`
}

// CustomConfig holds custom/manual configuration.
type CustomConfig struct {
	// Connection details provided manually
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Password string `json:"password,omitempty"`
	Username string `json:"username,omitempty"`
	TLS      bool   `json:"tls"`
	Cluster  bool   `json:"cluster"`

	// Optional SSH tunnel
	SSHHost    string `json:"ssh_host,omitempty"`
	SSHPort    int    `json:"ssh_port,omitempty"`
	SSHUser    string `json:"ssh_user,omitempty"`
	SSHKeyPath string `json:"ssh_key_path,omitempty"`

	// Custom notes
	Notes string `json:"notes,omitempty"`
}

// Store is the interface for profile persistence.
type Store interface {
	// Create creates a new profile.
	Create(ctx context.Context, profile *Profile) error

	// Get retrieves a profile by ID.
	Get(ctx context.Context, id string) (*Profile, error)

	// GetByName retrieves a profile by name.
	GetByName(ctx context.Context, name string) (*Profile, error)

	// List returns all profiles.
	List(ctx context.Context) ([]*Profile, error)

	// ListByProvider returns profiles for a specific provider.
	ListByProvider(ctx context.Context, provider Provider) ([]*Profile, error)

	// ListByTag returns profiles with a specific tag.
	ListByTag(ctx context.Context, tag string) ([]*Profile, error)

	// Update updates an existing profile.
	Update(ctx context.Context, profile *Profile) error

	// Delete removes a profile.
	Delete(ctx context.Context, id string) error

	// RecordUsage records that a profile was used.
	RecordUsage(ctx context.Context, id string) error
}

// Predefined profiles for common scenarios.
var PredefinedProfiles = []Profile{
	{
		ID:          "azure-basic-dev",
		Name:        "Azure Basic (Development)",
		Description: "Low-cost Azure Cache for development and testing",
		Provider:    ProviderAzure,
		Tags:        []string{"development", "low-cost", "builtin"},
		IsBuiltin:   true,
		Config: ProfileConfig{
			Azure: &AzureConfig{
				Location:             "westus3",
				SKU:                  "Basic",
				Family:               "C",
				Capacity:             0,
				EnableNonSSLPort:     true,
				VMSize:               "Standard_B2s",
				VMCount:              1,
				EstimatedMonthlyCost: 20,
			},
		},
	},
	{
		ID:          "azure-standard-staging",
		Name:        "Azure Standard (Staging)",
		Description: "Azure Cache Standard for staging environments",
		Provider:    ProviderAzure,
		Tags:        []string{"staging", "ha", "builtin"},
		IsBuiltin:   true,
		Config: ProfileConfig{
			Azure: &AzureConfig{
				Location:             "westus3",
				SKU:                  "Standard",
				Family:               "C",
				Capacity:             1,
				EnableNonSSLPort:     false,
				MinTLSVersion:        "1.2",
				VMSize:               "Standard_D2s_v3",
				VMCount:              1,
				EstimatedMonthlyCost: 80,
			},
		},
	},
	{
		ID:          "azure-premium-production",
		Name:        "Azure Premium (Production)",
		Description: "High-performance Azure Cache for production workloads",
		Provider:    ProviderAzure,
		Tags:        []string{"production", "enterprise", "ha", "builtin"},
		IsBuiltin:   true,
		Config: ProfileConfig{
			Azure: &AzureConfig{
				Location:             "westus3",
				SKU:                  "Premium",
				Family:               "P",
				Capacity:             1,
				EnableNonSSLPort:     false,
				MinTLSVersion:        "1.2",
				ReplicasPerPrimary:   1,
				ZoneRedundant:        true,
				VMSize:               "Standard_D4s_v3",
				VMCount:              2,
				EstimatedMonthlyCost: 350,
			},
		},
	},
	{
		ID:          "aws-elasticache-dev",
		Name:        "AWS ElastiCache (Development)",
		Description: "Low-cost AWS ElastiCache for development",
		Provider:    ProviderAWS,
		Tags:        []string{"development", "low-cost", "builtin"},
		IsBuiltin:   true,
		Config: ProfileConfig{
			AWS: &AWSConfig{
				Region:               "us-east-1",
				NodeType:             "cache.t3.micro",
				NumCacheNodes:        1,
				Engine:               "redis",
				ClusterEnabled:       false,
				EC2InstanceType:      "t3.small",
				EC2Count:             1,
				EstimatedMonthlyCost: 15,
			},
		},
	},
	{
		ID:          "local-redis",
		Name:        "Local Redis",
		Description: "Local Redis instance for testing",
		Provider:    ProviderLocal,
		Tags:        []string{"local", "development", "builtin"},
		IsBuiltin:   true,
		Config: ProfileConfig{
			Local: &LocalConfig{
				Host: "localhost",
				Port: 6379,
			},
		},
	},
}

// Validate validates a profile configuration.
func (p *Profile) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("profile name is required")
	}
	if p.Provider == "" {
		return fmt.Errorf("provider is required")
	}

	switch p.Provider {
	case ProviderAzure:
		if p.Config.Azure == nil {
			return fmt.Errorf("azure configuration is required for azure provider")
		}
		return p.Config.Azure.Validate()
	case ProviderAWS:
		if p.Config.AWS == nil {
			return fmt.Errorf("aws configuration is required for aws provider")
		}
		return p.Config.AWS.Validate()
	case ProviderGCP:
		if p.Config.GCP == nil {
			return fmt.Errorf("gcp configuration is required for gcp provider")
		}
		return p.Config.GCP.Validate()
	case ProviderLocal:
		if p.Config.Local == nil {
			return fmt.Errorf("local configuration is required for local provider")
		}
		return p.Config.Local.Validate()
	case ProviderCustom:
		if p.Config.Custom == nil {
			return fmt.Errorf("custom configuration is required for custom provider")
		}
		return p.Config.Custom.Validate()
	default:
		return fmt.Errorf("unknown provider: %s", p.Provider)
	}
}

// Validate validates Azure configuration.
func (c *AzureConfig) Validate() error {
	if c.Location == "" {
		return fmt.Errorf("location is required")
	}
	if c.SKU == "" {
		return fmt.Errorf("sku is required")
	}
	validSKUs := map[string]bool{"Basic": true, "Standard": true, "Premium": true, "Enterprise": true}
	if !validSKUs[c.SKU] {
		return fmt.Errorf("invalid sku: %s (must be Basic, Standard, Premium, or Enterprise)", c.SKU)
	}
	if c.Family == "" {
		return fmt.Errorf("family is required")
	}
	if c.Capacity < 0 || c.Capacity > 6 {
		return fmt.Errorf("capacity must be between 0 and 6")
	}
	return nil
}

// Validate validates AWS configuration.
func (c *AWSConfig) Validate() error {
	if c.Region == "" {
		return fmt.Errorf("region is required")
	}
	if c.NodeType == "" {
		return fmt.Errorf("node_type is required")
	}
	if c.NumCacheNodes < 1 {
		return fmt.Errorf("num_cache_nodes must be at least 1")
	}
	return nil
}

// Validate validates GCP configuration.
func (c *GCPConfig) Validate() error {
	if c.Region == "" {
		return fmt.Errorf("region is required")
	}
	if c.Tier == "" {
		return fmt.Errorf("tier is required")
	}
	if c.MemorySizeGB < 1 {
		return fmt.Errorf("memory_size_gb must be at least 1")
	}
	return nil
}

// Validate validates local configuration.
func (c *LocalConfig) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("host is required")
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

// Validate validates custom configuration.
func (c *CustomConfig) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("host is required")
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

// ToJSON serializes the profile to JSON.
func (p *Profile) ToJSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// FromJSON deserializes a profile from JSON.
func FromJSON(data []byte) (*Profile, error) {
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// Clone creates a deep copy of the profile.
func (p *Profile) Clone() *Profile {
	data, _ := json.Marshal(p)
	var clone Profile
	json.Unmarshal(data, &clone)
	return &clone
}
