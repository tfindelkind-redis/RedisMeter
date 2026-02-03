// Package domain contains core business entities for RedisMeter.
package domain

// Target represents a Redis deployment to benchmark.
type Target struct {
	// Connection
	URL      string `json:"url"` // redis://host:port or redis+cluster://host:port
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	
	// Authentication
	Password string `json:"password,omitempty"`
	Username string `json:"username,omitempty"`
	
	// TLS
	TLS      *TLSConfig `json:"tls,omitempty"`
	
	// Topology
	Cluster  bool   `json:"cluster,omitempty"`
	Database int    `json:"database,omitempty"`
	
	// Metadata (discovered)
	Version  string `json:"version,omitempty"`
	Modules  []string `json:"modules,omitempty"`
	
	// Labels
	Name     string            `json:"name,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
}

// TLSConfig holds TLS/SSL configuration for Redis connections.
type TLSConfig struct {
	Enabled            bool   `json:"enabled"`
	CertFile           string `json:"cert_file,omitempty"`
	KeyFile            string `json:"key_file,omitempty"`
	CAFile             string `json:"ca_file,omitempty"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify,omitempty"`
}

// Environment captures the execution environment for reproducibility.
type Environment struct {
	// Fingerprint is a hash of environment details for comparison.
	Fingerprint string `json:"fingerprint"`
	
	// Host information
	Hostname    string `json:"hostname,omitempty"`
	OS          string `json:"os,omitempty"`
	Arch        string `json:"arch,omitempty"`
	KernelVersion string `json:"kernel_version,omitempty"`
	
	// Hardware
	CPUModel    string `json:"cpu_model,omitempty"`
	CPUCores    int    `json:"cpu_cores,omitempty"`
	MemoryGB    float64 `json:"memory_gb,omitempty"`
	
	// Network
	NetworkLatencyMs float64 `json:"network_latency_ms,omitempty"`
	
	// Redis target details
	RedisVersion string            `json:"redis_version,omitempty"`
	RedisConfig  map[string]string `json:"redis_config,omitempty"`
	RedisModules []string          `json:"redis_modules,omitempty"`
	
	// Cloud provider details (if applicable)
	Cloud       *CloudEnvironment `json:"cloud,omitempty"`
	
	// Container details (if applicable)
	Container   *ContainerEnvironment `json:"container,omitempty"`
	
	// Tool version
	RedisMeterVersion string `json:"redismeter_version,omitempty"`
	MemtierVersion    string `json:"memtier_version,omitempty"`
}

// CloudEnvironment captures cloud-specific details.
type CloudEnvironment struct {
	Provider     string `json:"provider,omitempty"` // aws, gcp, azure
	Region       string `json:"region,omitempty"`
	Zone         string `json:"zone,omitempty"`
	InstanceType string `json:"instance_type,omitempty"`
	InstanceID   string `json:"instance_id,omitempty"`
}

// ContainerEnvironment captures container-specific details.
type ContainerEnvironment struct {
	Runtime     string `json:"runtime,omitempty"` // docker, containerd, cri-o
	Image       string `json:"image,omitempty"`
	Orchestrator string `json:"orchestrator,omitempty"` // kubernetes, swarm, ecs
	Namespace   string `json:"namespace,omitempty"`
	PodName     string `json:"pod_name,omitempty"`
}
