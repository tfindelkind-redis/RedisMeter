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
}

export interface HostInfo {
  hostname?: string;
  os?: string;
  arch?: string;
  cpus?: number;
  memory_gb?: number;
}

export interface RedisInfo {
  version?: string;
  mode?: string;
  os?: string;
  arch?: string;
  memory_used?: number;
  memory_max?: number;
  connected_clients?: number;
  cluster_enabled?: boolean;
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
  workload: string;
  threads?: number;
  clients?: number;
  duration?: string;
  tags?: string[];
}

// ============================================
// Infrastructure Management Types
// ============================================

export type CloudProvider = 'azure' | 'aws' | 'gcp' | 'kubernetes' | 'vmware' | 'local';

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
