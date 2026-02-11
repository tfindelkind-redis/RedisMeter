// Domain types matching the Go backend

export interface BenchmarkRun {
  id: string;
  created_at: string;
  updated_at: string;
  workload?: Workload;
  target?: Target;
  environment?: Environment;
  status: RunStatus;
  start_time?: string;
  end_time?: string;
  duration?: string;
  results?: Results;
  error?: string;
  name?: string;
  description?: string;
  tags?: string[];
  labels?: Record<string, string>;
}

export type RunStatus = 'pending' | 'running' | 'completed' | 'failed' | 'cancelled';

export interface Workload {
  name: string;
  description?: string;
  type: string;
  operations: Operation[];
  key_pattern?: KeyPattern;
  data_size?: DataSize;
  threads?: number;
  clients?: number;
  duration?: string;
  requests?: number;
  pipeline?: number;
  rate_limiting?: RateLimiting;
  custom?: Record<string, unknown>;
}

export interface Operation {
  command: string;
  ratio: number;
  args?: string[];
}

export interface KeyPattern {
  prefix?: string;
  pattern: string;
  key_range?: number;
  hotspot_fraction?: number;
}

export interface DataSize {
  min?: number;
  max?: number;
  fixed?: number;
  pattern?: string;
}

export interface RateLimiting {
  requests_per_second?: number;
  burst_size?: number;
}

export interface Target {
  host: string;
  port: number;
  username?: string;
  password?: string;
  tls?: boolean;
  cluster?: boolean;
}

export interface Environment {
  host?: HostInfo;
  redis?: RedisInfo;
  fingerprint?: string;
  network_latency_ms?: number;
  redismeter_version?: string;
  memtier_version?: string;
}

export interface HostInfo {
  hostname?: string;
  os?: string;
  arch?: string;
  kernel_version?: string;
  cpu_model?: string;
  cpus?: number;
  memory_gb?: number;
}

export interface RedisInfo {
  // Server info
  version?: string;
  mode?: string;
  os?: string;
  arch?: string;
  uptime_seconds?: number;

  // Memory
  memory_used?: number;
  memory_max?: number;
  memory_peak?: number;
  memory_frag_ratio?: number;
  memory_eviction_policy?: string;

  // Clients
  connected_clients?: number;
  blocked_clients?: number;

  // Stats
  total_connections_received?: number;
  total_commands_processed?: number;
  instantaneous_ops_per_sec?: number;
  evicted_keys?: number;
  expired_keys?: number;
  keyspace_hits?: number;
  keyspace_misses?: number;

  // Persistence
  rdb_enabled?: boolean;
  aof_enabled?: boolean;
  rdb_last_save_time?: number;
  rdb_last_bgsave_status?: string;

  // Replication
  role?: string;
  connected_slaves?: number;

  // Cluster
  cluster_enabled?: boolean;
  cluster_size?: number;

  // Keyspace
  total_keys?: number;
  total_expires?: number;
  keyspace?: Record<string, string>;

  // Modules
  modules?: string[];

  // Raw config
  config?: Record<string, string>;
}

export interface Results {
  summary?: SummaryMetrics;
  by_operation?: Record<string, OperationMetrics>;
  time_series?: TimeSeriesPoint[];
  latency_histogram?: Histogram;
  raw_output?: string;
}

export interface SummaryMetrics {
  total_requests: number;
  total_ops: number;
  ops_per_second: number;
  bytes_per_second?: number;
  avg_latency_ms: number;
  min_latency_ms: number;
  max_latency_ms: number;
  p50_latency_ms: number;
  p90_latency_ms: number;
  p95_latency_ms: number;
  p99_latency_ms: number;
  p999_latency_ms?: number;
  errors: number;
  error_rate: number;
  connections?: number;
}

export interface OperationMetrics {
  operation: string;
  count: number;
  ops_per_second: number;
  avg_latency_ms: number;
  p50_latency_ms: number;
  p90_latency_ms: number;
  p95_latency_ms: number;
  p99_latency_ms: number;
}

export interface TimeSeriesPoint {
  timestamp: string;
  ops_per_second: number;
  avg_latency_ms: number;
  p99_latency_ms: number;
  errors?: number;
}

export interface Histogram {
  buckets: HistogramBucket[];
}

export interface HistogramBucket {
  upper_bound_ms: number;
  count: number;
  cumulative: number;
}

export interface Baseline {
  id: string;
  created_at: string;
  updated_at: string;
  run_id: string;
  name: string;
  description?: string;
  tags?: string[];
  labels?: Record<string, string>;
  active: boolean;
  valid_from?: string;
  valid_until?: string;
  metrics?: SummaryMetrics;
  workload?: Workload;
  environment?: Environment;
  thresholds?: BaselineThresholds;
}

export interface BaselineThresholds {
  max_throughput_regression?: number;
  max_latency_regression?: number;
  max_p99_regression?: number;
  max_error_rate_increase?: number;
}

export interface ComparisonResult {
  run_id: string;
  baseline_id: string;
  pass?: boolean;
  passed?: boolean;
  verdict: string;
  metrics?: MetricsComparison;
  changes?: {
    throughput_pct?: number;
    avg_latency_pct?: number;
    p99_latency_pct?: number;
    error_rate_pct?: number;
  };
  environment_match: boolean;
  environment_diffs?: string[];
  violations?: ThresholdViolation[];
}

export interface MetricsComparison {
  throughput_change: number;
  throughput_change_pct: number;
  latency_change: number;
  latency_change_pct: number;
  p99_change: number;
  p99_change_pct: number;
  error_rate_change: number;
}

export interface ThresholdViolation {
  metric: string;
  threshold: number | string;
  actual: number | string;
  status?: 'pass' | 'warn' | 'fail';
  message: string;
}

export interface AnalysisResult {
  run_id: string;
  analyzers: AnalyzerResult[];
  overall_score?: number;
  recommendations?: string[];
}

export interface AnalyzerResult {
  name: string;
  score: number;
  findings: Finding[];
  recommendations: string[];
}

export interface Finding {
  severity: 'info' | 'warning' | 'critical';
  category: string;
  message: string;
  metric?: string;
  value?: number;
  threshold?: number;
}

// API Response types
export interface ApiResponse<T> {
  data?: T;
  error?: string;
  message?: string;
}

export interface PaginatedResponse<T> {
  items: T[];
  total: number;
  page: number;
  page_size: number;
}

// WebSocket message types
export interface WSMessage {
  type: 'benchmark_started' | 'benchmark_progress' | 'benchmark_completed' | 'benchmark_failed' | 'metrics';
  benchmark_id: string;
  data: unknown;
}

export interface BenchmarkProgress {
  progress: number;
  current_ops: number;
  current_latency: number;
  elapsed: string;
}

// Benchmark configuration for starting a new benchmark
export interface BenchmarkConfig {
  name?: string;
  description?: string;
  target: {
    host: string;
    port: number;
    password?: string;
    tls?: boolean;
    cluster?: boolean;
  };
  tool?: BenchmarkToolType;
  workload?: string;
  run_profile?: string;
  tool_config?: Record<string, unknown>;
  // Optional overrides (when specified, override run_profile settings)
  threads?: number;
  clients?: number;
  duration?: string;
  tags?: string[];
}

// ============================================
// Infrastructure Management Types
// ============================================

export type CloudProvider = 'azure' | 'aws' | 'gcp' | 'kubernetes' | 'vmware' | 'local' | 'self_managed';

// ============================================
// Self-Managed Infrastructure Types
// ============================================

// SSH Authentication method
export type SSHAuthMethod = 'password' | 'key' | 'key_file';

// SSH Credentials for connecting to runner machines
export interface SSHCredentials {
  auth_method: SSHAuthMethod;
  username: string;
  // Password authentication (encrypted)
  password?: string;
  // SSH key authentication
  private_key?: string; // PEM-encoded private key content (encrypted)
  private_key_path?: string; // Path to private key file on server
  passphrase?: string; // Passphrase for encrypted private key (encrypted)
  // Connection settings
  port?: number; // Default: 22
  connect_timeout_seconds?: number; // Default: 30
  strict_host_key_checking?: boolean;
}

// Redis Authentication credentials
export interface RedisCredentials {
  username?: string; // Default: 'default' for ACL, empty for legacy
  password?: string; // Redis password (encrypted)
  tls_enabled?: boolean;
  tls_skip_verify?: boolean; // Skip TLS certificate verification
  tls_cert?: string; // Client certificate (for mTLS)
  tls_key?: string; // Client private key (for mTLS)
  tls_ca?: string; // CA certificate for verification
}

// Runner machine definition for self-managed infrastructure
export interface RunnerMachine {
  id: string;
  name?: string; // Friendly name for the runner
  host: string; // IP address or hostname
  ssh_port?: number; // Override default SSH port
  labels?: Record<string, string>; // Custom labels for runner selection
  available?: boolean; // Set by health check
  last_health_check?: string;
}

// Redis target for self-managed infrastructure
export interface RedisTarget {
  id: string;
  name?: string; // Friendly name
  host: string; // IP address or hostname
  port: number; // Default: 6379
  is_cluster?: boolean;
  cluster_nodes?: string[]; // Additional cluster nodes (host:port)
}

// Self-managed infrastructure configuration
export interface SelfManagedConfig {
  // Runner machines
  runners: RunnerMachine[];
  ssh_credentials: SSHCredentials;
  
  // Redis target(s)
  redis_targets: RedisTarget[];
  redis_credentials: RedisCredentials;
  
  // Benchmark tool paths (on runners)
  memtier_path?: string; // Default: memtier_benchmark
  redis_cli_path?: string; // Default: redis-cli
  custom_tools?: Record<string, string>; // tool_name -> path
}

export type InfraStatus = 'pending' | 'provisioning' | 'ready' | 'failed' | 'destroying' | 'destroyed';

export interface Infrastructure {
  id: string;
  name: string;
  provider: CloudProvider;
  region: string;
  status: InfraStatus;
  created_at: string;
  updated_at: string;
  expires_at?: string;
  config: InfraConfig;
  outputs?: InfraOutputs;
  error?: string;
  workspace_path?: string;
}

export interface InfraConfig {
  name: string;
  provider: CloudProvider;
  region: string;
  ttl?: string;
  tags?: Record<string, string>;
  amr?: AMRConfig;
  runners?: RunnersConfig;
  // Provider-specific configs
  azure?: AzureConfig;
  aws?: AWSConfig;
  gcp?: GCPConfig;
  kubernetes?: KubernetesConfig;
  self_managed?: SelfManagedConfig;
}

// Azure Managed Redis Config
export interface AMRConfig {
  sku: string;
  modules?: string[];
  high_availability?: boolean;
  clustering_policy?: 'OSSCluster' | 'EnterpriseCluster';
  eviction_policy?: string;
  zones?: string[];
}

// Runner VMs Config
export interface RunnersConfig {
  count: number;
  instance_type: string;
  spot_instances?: boolean;
  ssh_public_key?: string;
  ssh_user?: string;
}

// Azure-specific settings
export interface AzureConfig {
  subscription_id?: string;
  resource_group?: string;
  vnet_address_space?: string;
  subnet_address_prefix?: string;
}

// AWS-specific settings
export interface AWSConfig {
  account_id?: string;
  vpc_id?: string;
  subnet_ids?: string[];
  security_group_ids?: string[];
  iam_role?: string;
}

// GCP-specific settings
export interface GCPConfig {
  project_id?: string;
  network?: string;
  subnetwork?: string;
  service_account?: string;
}

// Kubernetes-specific settings
export interface KubernetesConfig {
  context?: string;
  namespace?: string;
  storage_class?: string;
  node_selector?: Record<string, string>;
}

// Infrastructure outputs after provisioning
export interface InfraOutputs {
  resource_group_name?: string;
  redis_hostname?: string;
  redis_port?: number;
  redis_password?: string;
  runner_ips?: string[];
  runner_private_ips?: string[];
  // Additional outputs by provider
  [key: string]: unknown;
}

// Cloud benchmark request
export interface CloudBenchmarkConfig {
  infrastructure_id: string;
  workload?: string;
  run_profile?: string;
  requests?: number;
  clients?: number;
  threads?: number;
  duration?: string;
  pipeline?: number;
  key_pattern?: string;
  data_size?: number;
  ratio?: string;
}

// Provider metadata for UI
export interface ProviderInfo {
  id: CloudProvider;
  name: string;
  icon: string;
  description: string;
  enabled: boolean;
  regions: ProviderRegion[];
  instance_types: InstanceType[];
  redis_skus?: RedisSKU[];
}

export interface ProviderRegion {
  id: string;
  name: string;
  available: boolean;
}

export interface InstanceType {
  id: string;
  name: string;
  vcpus: number;
  memory_gb: number;
  category: 'general' | 'compute' | 'memory' | 'storage';
}

export interface RedisSKU {
  id: string;
  name: string;
  description: string;
  tier: string;
  memory_gb?: number;
  throughput?: string;
}

// Provider form props for cloud provider configuration forms
export interface ProviderFormProps {
  form: any; // Ant Design FormInstance
  initialValues?: Record<string, any>;
  onValuesChange?: (changedValues: any, allValues: any) => void;
  disabled?: boolean;
}

// ==========================================
// Infrastructure Profiles
// ==========================================

export type InfraProfileProvider = 'azure' | 'aws' | 'gcp' | 'local' | 'custom' | 'self_managed';

export interface InfraProfile {
  id: string;
  name: string;
  description?: string;
  provider: InfraProfileProvider;
  tags?: string[];
  config?: InfraProfileConfig;
  use_count?: number;
  created_at?: string;
  updated_at?: string;
  is_builtin?: boolean;
}

export interface InfraProfileConfig {
  azure?: AzureInfraConfig;
  aws?: AWSInfraConfig;
  gcp?: GCPInfraConfig;
  local?: LocalInfraConfig;
  custom?: CustomInfraConfig;
  self_managed?: SelfManagedInfraConfig;
}

// Self-managed infrastructure profile configuration
export interface SelfManagedInfraConfig {
  // Runner machines
  runners: {
    machines: Array<{
      name?: string;
      host: string;
      port?: number;
      labels?: Record<string, string>;
    }>;
    ssh: {
      auth_method: 'password' | 'key' | 'key_file';
      username: string;
      password_encrypted?: string;
      private_key_encrypted?: string;
      private_key_path?: string;
      passphrase_encrypted?: string;
      port?: number;
      connect_timeout?: number;
      strict_host_key_checking?: boolean;
    };
  };
  // Redis target configuration
  redis: {
    targets: Array<{
      name?: string;
      host: string;
      port: number;
      is_cluster?: boolean;
      cluster_nodes?: string[];
    }>;
    credentials: {
      username?: string;
      password_encrypted?: string;
      tls_enabled?: boolean;
      tls_skip_verify?: boolean;
      tls_cert?: string;
      tls_key_encrypted?: string;
      tls_ca?: string;
    };
  };
  // Tool paths on runners
  tool_paths?: Record<string, string>;
}

export interface AzureInfraConfig {
  subscription_id?: string;
  resource_group?: string;
  location: string;
  
  // Instance name (without .redis.azure.net suffix)
  instance_name?: string;
  
  // Performance tier configuration
  data_tier?: string; // "in-memory" (default) or "flash"
  
  // Azure Managed Redis configuration
  sku: string; // Family_Size format (e.g., Balanced_B5)
  high_availability?: boolean;
  persistence_type?: string; // "", "rdb", "aof"
  rdb_frequency?: string; // "1h", "6h", "12h"
  aof_frequency?: string; // "1s", "always"
  modules?: string[]; // RedisJSON, RediSearch, RedisBloom, RedisTimeSeries
  
  // Advanced settings
  eviction_policy?: string; // noeviction, allkeys-lru, volatile-lru, etc.
  clustering_policy?: string; // non-clustered, oss, enterprise
  non_tls_access_only?: boolean;
  access_keys_auth?: boolean;
  customer_managed_key?: boolean;
  defer_version_updates?: boolean;
  
  // Customer-managed key configuration (when customer_managed_key=true)
  user_assigned_identity_id?: string; // Resource ID of user-assigned managed identity
  key_input_method?: string; // "select" or "uri"
  // For "select" method:
  key_vault_subscription_id?: string; // Subscription ID containing the Key Vault
  key_vault_name?: string; // Name of the Key Vault
  key_name?: string; // Name of the encryption key (RSA)
  key_version?: string; // Optional: specific key version (empty = latest)
  // For "uri" method:
  key_identifier_uri?: string; // Full key identifier URI (e.g., https://vault.vault.azure.net/keys/keyname/version)
  
  // Active geo-replication
  active_geo_replication?: boolean;
  geo_replication_group_name?: string;
  
  // Networking
  use_private_endpoint?: boolean;
  allow_public_access?: boolean;
  
  // Benchmark VM configuration
  vm_size?: string;
  vm_count?: number;
  ssh_key_path?: string;
  estimated_monthly_cost?: number;
}

export interface AWSInfraConfig {
  region: string;
  // ElastiCache configuration
  node_type: string;
  num_cache_nodes: number;
  engine?: string;
  engine_version?: string;
  parameter_group_family?: string;
  // Cluster mode
  cluster_enabled?: boolean;
  num_node_groups?: number;
  replicas_per_node_group?: number;
  // Networking
  subnet_group_name?: string;
  security_group_ids?: string[];
  // Benchmark EC2 configuration
  ec2_instance_type?: string;
  ec2_count?: number;
  use_spot_instances?: boolean;
  ssh_key_name?: string;
  estimated_monthly_cost?: number;
}

export interface GCPInfraConfig {
  project_id?: string;
  region: string;
  zone?: string;
  // Memorystore configuration
  tier: string;
  memory_size_gb: number;
  redis_version?: string;
  display_name?: string;
  // Networking
  authorized_network?: string;
  connect_mode?: string;
  // Benchmark VM configuration
  machine_type?: string;
  vm_count?: number;
  preemptible?: boolean;
  estimated_monthly_cost?: number;
}

export interface LocalInfraConfig {
  host: string;
  port: number;
  password?: string;
  tls?: boolean;
  database?: number;
}

export interface CustomInfraConfig {
  host: string;
  port: number;
  password?: string;
  username?: string;
  tls?: boolean;
  database?: number;
  description?: string;
}

export interface InfraProfileStats {
  total_profiles: number;
  by_provider: Record<string, number>;
  most_used?: InfraProfile;
  recently_created?: InfraProfile;
  recently_used?: InfraProfile;
}

// ============================================
// Benchmark Tools Types
// ============================================

// Supported benchmark tools
export type BenchmarkToolType = 
  | 'memtier_benchmark'   // Classic Redis benchmark tool (GET/SET, basic commands)
  | 'ann_benchmarks'      // Vector/ANN performance (HNSW, vector search)
  | 'ftsb'               // Full-text search benchmark (RediSearch)
  | 'vectordb_bench';    // VectorDBBench for comprehensive vector testing

// Benchmark tool metadata
export interface BenchmarkTool {
  id: BenchmarkToolType;
  name: string;
  description: string;
  icon?: string;
  category: 'core' | 'search' | 'vector';
  supported_workloads: string[];
  requires_module?: string[];  // Redis modules required (e.g., 'search', 'json')
  documentation_url?: string;
  available: boolean;  // Whether this tool is installed/available
  version?: string;
}

// Tool-specific configuration interfaces

export interface MemtierConfig {
  threads: number;
  clients: number;
  pipeline: number;
  duration?: string;
  requests?: number;
  ratio?: string;  // e.g., "1:10" for SET:GET
  data_size?: number;
  key_pattern?: string;
  key_prefix?: string;
  random_data?: boolean;
  distinct_client_seed?: boolean;
  protocol?: 'redis' | 'resp3';
  hide_histogram?: boolean;
  json_out_file?: string;
}

export interface ANNBenchmarksConfig {
  algorithm: string;  // e.g., 'redis', 'hnsw'
  dataset: string;    // e.g., 'glove-100-angular', 'sift-128-euclidean'
  k: number;          // Number of nearest neighbors
  runs: number;       // Number of benchmark runs
  batch_mode?: boolean;
  parallelism?: number;
  // HNSW-specific parameters
  ef_construction?: number;
  ef_search?: number;
  m?: number;
}

export interface FTSBConfig {
  use_case: 'nyc_taxis' | 'enwiki_abstract' | 'enwiki_pages' | 'ecommerce_inventory' | 'custom';
  workers: number;
  pipeline: number;
  duration?: string;
  requests?: number;
  rate_limit?: number;
  cluster_mode?: boolean;
  // Data generation
  doc_count?: number;
  query_count?: number;
}

export interface VectorDBBenchConfig {
  case_type: string;  // e.g., 'Performance768D1M', 'CapacityDim960'
  k: number;
  concurrency: number[];
  drop_old?: boolean;
  load?: boolean;
  search_serial?: boolean;
  search_concurrent?: boolean;
  // Index parameters
  m?: number;
  ef_construction?: number;
  ef_search?: number;
}

// Union type for all tool configs
export type ToolConfig = 
  | { tool: 'memtier_benchmark'; config: MemtierConfig }
  | { tool: 'ann_benchmarks'; config: ANNBenchmarksConfig }
  | { tool: 'ftsb'; config: FTSBConfig }
  | { tool: 'vectordb_bench'; config: VectorDBBenchConfig };

// Extended benchmark configuration with tool support
export interface BenchmarkConfigV2 {
  name?: string;
  description?: string;
  tool: BenchmarkToolType;
  target: {
    host: string;
    port: number;
    password?: string;
    tls?: boolean;
    cluster?: boolean;
  };
  workload?: string;        // For memtier/ftsb
  run_profile?: string;     // For memtier
  tool_config?: ToolConfig['config'];  // Tool-specific configuration
  tags?: string[];
}

// Benchmark tool availability check result
export interface ToolAvailability {
  tool: BenchmarkToolType;
  available: boolean;
  version?: string;
  path?: string;
  error?: string;
}
