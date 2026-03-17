// Package domain contains core business entities for RedisMeter.
package domain

// Target represents a Redis deployment to benchmark.
type Target struct {
	// Connection
	URL  string `json:"url"` // redis://host:port or redis+cluster://host:port
	Host string `json:"host,omitempty"`
	Port int    `json:"port,omitempty"`

	// Authentication
	Password string `json:"password,omitempty"`
	Username string `json:"username,omitempty"`

	// TLS
	TLS *TLSConfig `json:"tls,omitempty"`

	// Topology
	Cluster  bool `json:"cluster,omitempty"`
	Database int  `json:"database,omitempty"`

	// Metadata (discovered)
	Version string   `json:"version,omitempty"`
	Modules []string `json:"modules,omitempty"`

	// Labels
	Name   string            `json:"name,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
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

	// Nested structures matching frontend expectations
	Host  *HostInfo  `json:"host,omitempty"`
	Redis *RedisInfo `json:"redis,omitempty"`

	// Network
	NetworkLatencyMs float64 `json:"network_latency_ms,omitempty"`

	// Cloud provider details (if applicable)
	Cloud *CloudEnvironment `json:"cloud,omitempty"`

	// Container details (if applicable)
	Container *ContainerEnvironment `json:"container,omitempty"`

	// Tool version
	RedisMeterVersion string `json:"redismeter_version,omitempty"`
	MemtierVersion    string `json:"memtier_version,omitempty"`
}

// HostInfo captures host/client system information.
type HostInfo struct {
	Hostname      string  `json:"hostname,omitempty"`
	OS            string  `json:"os,omitempty"`
	Arch          string  `json:"arch,omitempty"`
	KernelVersion string  `json:"kernel_version,omitempty"`
	CPUModel      string  `json:"cpu_model,omitempty"`
	CPUs          int     `json:"cpus,omitempty"`
	MemoryGB      float64 `json:"memory_gb,omitempty"`
}

// RedisInfo captures comprehensive Redis server information.
type RedisInfo struct {
	// Server info
	Version string `json:"version,omitempty"`
	Mode    string `json:"mode,omitempty"` // standalone, cluster, sentinel
	OS      string `json:"os,omitempty"`
	Arch    string `json:"arch,omitempty"`
	Uptime  int64  `json:"uptime_seconds,omitempty"`

	// Memory
	MemoryUsed           int64   `json:"memory_used,omitempty"`            // used_memory in bytes
	MemoryMax            int64   `json:"memory_max,omitempty"`             // maxmemory in bytes (0=unlimited)
	MemoryPeak           int64   `json:"memory_peak,omitempty"`            // used_memory_peak
	MemoryFragRatio      float64 `json:"memory_frag_ratio,omitempty"`      // mem_fragmentation_ratio
	MemoryEvictionPolicy string  `json:"memory_eviction_policy,omitempty"` // maxmemory-policy

	// Clients
	ConnectedClients int `json:"connected_clients,omitempty"`
	BlockedClients   int `json:"blocked_clients,omitempty"`

	// Stats (cumulative since server start)
	TotalConnectionsReceived int64 `json:"total_connections_received,omitempty"`
	TotalCommandsProcessed   int64 `json:"total_commands_processed,omitempty"`
	InstantaneousOpsPerSec   int64 `json:"instantaneous_ops_per_sec,omitempty"` // baseline load
	EvictedKeys              int64 `json:"evicted_keys,omitempty"`
	ExpiredKeys              int64 `json:"expired_keys,omitempty"`
	KeyspaceHits             int64 `json:"keyspace_hits,omitempty"`
	KeyspaceMisses           int64 `json:"keyspace_misses,omitempty"`

	// Persistence
	RDBEnabled          bool   `json:"rdb_enabled,omitempty"`
	AOFEnabled          bool   `json:"aof_enabled,omitempty"`
	RDBLastSaveTime     int64  `json:"rdb_last_save_time,omitempty"`
	RDBLastBgSaveStatus string `json:"rdb_last_bgsave_status,omitempty"`

	// Replication
	Role            string `json:"role,omitempty"` // master, slave
	ConnectedSlaves int    `json:"connected_slaves,omitempty"`

	// Cluster
	ClusterEnabled bool `json:"cluster_enabled,omitempty"`
	ClusterSize    int  `json:"cluster_size,omitempty"` // number of shards

	// Keyspace (pre-benchmark state)
	TotalKeys    int64             `json:"total_keys,omitempty"`    // DBSIZE result
	TotalExpires int64             `json:"total_expires,omitempty"` // keys with TTL
	Keyspace     map[string]string `json:"keyspace,omitempty"`      // db0:keys=N,expires=M

	// Modules
	Modules []string `json:"modules,omitempty"`

	// Raw config values
	Config map[string]string `json:"config,omitempty"`
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
	Runtime      string `json:"runtime,omitempty"` // docker, containerd, cri-o
	Image        string `json:"image,omitempty"`
	Orchestrator string `json:"orchestrator,omitempty"` // kubernetes, swarm, ecs
	Namespace    string `json:"namespace,omitempty"`
	PodName      string `json:"pod_name,omitempty"`
}
