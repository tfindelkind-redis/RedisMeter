# RedisMeter Architecture

## Overview

RedisMeter is a comprehensive Redis benchmarking tool designed for enterprise-grade performance testing. It provides a scalable architecture that supports distributed load generation, flexible workload definitions, and multiple persistence backends.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              RedisMeter CLI                                 │
│                         (cmd/redismeter/main.go)                            │
└────────────────────────────────────┬────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Benchmark Engine                                  │
│                     (internal/engine/engine.go)                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │  Workload   │  │  Memtier    │  │ Environment │  │      Storage        │ │
│  │  Registry   │  │  Executor   │  │  Capturer   │  │     Interface       │ │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────┘
                                     │
            ┌────────────────────────┼────────────────────────┐
            ▼                        ▼                        ▼
┌───────────────────┐  ┌───────────────────┐  ┌───────────────────────────────┐
│   Local Executor  │  │  Cloud Provider   │  │       Storage Backends        │
│    (memtier)      │  │     (AWS/GCP)     │  │  (File / SQLite / MongoDB)    │
└───────────────────┘  └─────────┬─────────┘  └───────────────────────────────┘
                                 │
                                 ▼
                    ┌───────────────────────┐
                    │   Multi-Node SSH      │
                    │      Executor         │
                    │ (Distributed Runners) │
                    └───────────────────────┘
```

---

## Table of Contents

1. [Core Components](#core-components)
2. [Scaling Architecture](#scaling-architecture)
3. [Data Aggregation](#data-aggregation)
4. [Default Workload Patterns](#default-workload-patterns)
5. [Persistence Layer](#persistence-layer)
6. [Domain Model](#domain-model)

---

## Core Components

### 1. Benchmark Engine (`internal/engine/engine.go`)

The `Engine` is the central orchestrator for benchmark execution. It coordinates:

- **Workload Loading**: Retrieves workload definitions from the registry or YAML files
- **Target Parsing**: Validates and parses Redis connection URLs
- **Environment Capture**: Fingerprints the execution environment for reproducibility
- **Execution Management**: Delegates to the appropriate executor (local or distributed)
- **Result Storage**: Persists benchmark runs to the configured storage backend

```go
type Engine struct {
    storage    storage.RunStorage    // Persistence backend
    memtier    *memtier.Executor     // Local benchmark executor
    envCapture *environment.Capturer // Environment fingerprinting
}
```

**Key Responsibilities:**
- Generate unique run IDs (format: `YYYYMMDD-HHMMSS-XXXX`)
- Manage benchmark lifecycle (pending → running → completed/failed)
- Apply workload parameter overrides from CLI flags
- Handle progress callbacks for real-time feedback

### 2. Memtier Executor (`internal/memtier/executor.go`)

Wraps the `memtier_benchmark` tool from Redis Labs, providing:

- **Command Building**: Translates workload definitions to memtier CLI arguments
- **Process Management**: Spawns and monitors benchmark processes
- **Output Parsing**: Converts JSON output to structured results
- **Streaming Progress**: Real-time ops/sec updates during execution

```go
type Config struct {
    // Connection settings
    Host, Port, Password, Username string
    TLS, Cluster bool
    
    // Workload parameters
    Ratio, KeyPattern string
    KeyMinimum, KeyMaximum int64
    DataSize, Threads, Clients int
    
    // Execution
    Duration time.Duration
    Pipeline, RateLimit int
}
```

### 3. Workload Registry (`internal/workload/registry.go`)

A thread-safe registry for managing workload definitions:

- **Built-in Workloads**: Pre-defined patterns for common use cases
- **Custom Workloads**: Load from YAML files
- **Runtime Overrides**: CLI flags can override workload parameters

---

## Scaling Architecture

RedisMeter supports two scaling modes for generating load at scale:

### Local Mode (Single Machine)

For development and small-scale testing:

```
┌─────────────────────────────────────────┐
│            Local Machine                │
│  ┌────────────────────────────────────┐ │
│  │         RedisMeter CLI             │ │
│  │              │                     │ │
│  │              ▼                     │ │
│  │     memtier_benchmark              │ │
│  │    (threads × clients)             │ │
│  └────────────────────────────────────┘ │
└─────────────────────────────────────────┘
                    │
                    ▼
            ┌───────────────┐
            │  Redis Target │
            └───────────────┘
```

### Distributed Mode (Cloud Infrastructure)

For production-grade benchmarks requiring massive scale:

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                           Control Plane                                      │
│  ┌────────────────────────────────────────────────────────────────────────┐  │
│  │                        RedisMeter CLI                                  │  │
│  │                              │                                         │  │
│  │     ┌────────────────────────┼────────────────────────┐                │  │
│  │     ▼                        ▼                        ▼                │  │
│  │ Cloud Provider      SSH Executor          Multi-Node Executor          │  │
│  │ (Provisioning)    (Connections)          (Orchestration)               │  │
│  └────────────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────────┘
                                    │
                    ┌───────────────┼───────────────┐
                    │               │               │
                    ▼               ▼               ▼
            ┌───────────┐   ┌───────────┐   ┌───────────┐
            │  Runner 1 │   │  Runner 2 │   │  Runner N │
            │  (Cloud)  │   │  (Cloud)  │   │  (Cloud)  │
            │           │   │           │   │           │
            │ memtier   │   │ memtier   │   │ memtier   │
            └─────┬─────┘   └─────┬─────┘   └─────┬─────┘
                  │               │               │
                  └───────────────┼───────────────┘
                                  ▼
                          ┌───────────────┐
                          │  Redis Target │
                          │  (Cluster)    │
                          └───────────────┘
```

### Cloud Provider Integration (`internal/cloud/provider.go`)

The `ProviderPlugin` interface abstracts cloud infrastructure management:

```go
type ProviderPlugin interface {
    // Infrastructure lifecycle
    Provision(ctx context.Context, spec *InfraSpec) (*Infrastructure, error)
    Teardown(ctx context.Context, infra *Infrastructure) error
    
    // Resource discovery
    ListRegions(ctx context.Context) ([]Region, error)
    ListInstanceTypes(ctx context.Context, region string) ([]InstanceType, error)
    
    // Cost management
    EstimateCost(ctx context.Context, spec *InfraSpec, duration time.Duration) (*CostEstimate, error)
}
```

**Infrastructure Specification:**
```go
type InfraSpec struct {
    Name          string         // Human-readable identifier
    Provider      string         // aws, gcp, azure
    Region        string         // Deployment region
    LoadGenerator *NodeGroupSpec // Runner instance configuration
    RedisTarget   *RedisTargetSpec
    TTL           time.Duration  // Auto-cleanup timeout
}
```

### SSH Executor (`internal/cloud/ssh_executor.go`)

Manages remote benchmark execution over SSH:

**Key Features:**
- Connection pooling with keep-alive validation
- Private key and password authentication
- Automatic memtier installation on runners
- Real-time output streaming via channels
- Graceful termination with SIGTERM

```go
type SSHExecutor struct {
    connections map[string]*sshConnection  // Host → connection pool
    executions  map[string]*sshExecution   // ID → execution state
    config      SSHConfig
}
```

### Multi-Node Executor (`internal/cloud/ssh_executor.go`)

Coordinates parallel execution across multiple runner instances:

```go
type MultiNodeExecutor struct {
    sshExecutor *SSHExecutor
    executions  map[string]*multiNodeExecution
}

func (m *MultiNodeExecutor) ExecuteOnHosts(
    ctx context.Context, 
    hosts []string, 
    workload *domain.Workload, 
    target *domain.Target,
) (string, error)
```

**Execution Flow:**
1. Connect to all runner hosts via SSH
2. Launch memtier_benchmark in parallel on each host
3. Stream real-time metrics from all runners
4. Wait for completion with configurable timeout
5. Collect and aggregate results from all hosts

---

## Data Aggregation

### How Results Are Collected

Each runner produces a JSON results file from memtier_benchmark:

```json
{
  "ALL STATS": {
    "Totals": {
      "Ops/sec": 125000.00,
      "Latency": 0.750,
      "KB/sec": 15000.00
    },
    "GET": { ... },
    "SET": { ... }
  }
}
```

### Aggregation Process

The `MultiNodeExecutor.GetAggregatedResults()` method collects output from all runners:

```
Runner 1: 125,000 ops/sec, 0.8ms p99
Runner 2: 118,000 ops/sec, 0.9ms p99
Runner 3: 122,000 ops/sec, 0.85ms p99
────────────────────────────────────────
Aggregate: ~365,000 ops/sec total
           0.85ms avg p99 latency
```

**Current Aggregation:**
- Raw output from each host is collected separately
- Results are stored per-host in `CloudRunResult.HostResults`
- Throughput is additive across runners
- Latency metrics are reported per-runner for analysis

**Result Structure:**
```go
type CloudRunResult struct {
    InfraID       string
    Provider      string
    Region        string
    InstanceType  string
    InstanceCount int
    HostResults   map[string]string  // host → raw output
}
```

### Metrics Collected

The domain model captures comprehensive metrics:

```go
type SummaryMetrics struct {
    // Throughput
    TotalRequests  int64
    OpsPerSecond   float64
    BytesPerSecond float64
    
    // Latency (milliseconds)
    AvgLatencyMs float64
    MinLatencyMs float64
    MaxLatencyMs float64
    
    // Percentiles
    P50LatencyMs  float64
    P90LatencyMs  float64
    P95LatencyMs  float64
    P99LatencyMs  float64
    P999LatencyMs float64
    
    // Errors
    Errors    int64
    ErrorRate float64
}
```

---

## Default Workload Patterns

RedisMeter includes 8 pre-defined workload patterns optimized for different use cases:

### 1. **cache** - Balanced Cache Pattern
```yaml
name: cache
description: Balanced GET/SET workload simulating typical cache usage
operations:
  - command: GET
    ratio: 0.8      # 80% reads
  - command: SET
    ratio: 0.2      # 20% writes
key_pattern:
  prefix: "cache:"
  pattern: random
  key_range: 1000000
data_size:
  fixed: 256        # 256 bytes
threads: 4
clients: 50
duration: 30s
```
**Use Case:** General-purpose caching, API response caching

### 2. **write-heavy** - Write-Intensive Pattern
```yaml
name: write-heavy
operations:
  - command: GET
    ratio: 0.2      # 20% reads
  - command: SET
    ratio: 0.8      # 80% writes
key_pattern:
  prefix: "write:"
  pattern: random
  key_range: 1000000
data_size:
  fixed: 256
```
**Use Case:** Write-heavy applications, logging, event streaming

### 3. **read-only** - Pure Read Pattern
```yaml
name: read-only
operations:
  - command: GET
    ratio: 1.0      # 100% reads
key_pattern:
  prefix: "read:"
  pattern: random
  key_range: 1000000
```
**Use Case:** Read replicas, cache warming validation, read scaling tests

### 4. **mixed** - Variable Data Size Pattern
```yaml
name: mixed
operations:
  - command: GET
    ratio: 0.5
  - command: SET
    ratio: 0.5
data_size:
  min: 64           # Variable size
  max: 1024         # 64B - 1KB
key_pattern:
  key_range: 500000
```
**Use Case:** Real-world mixed workloads with varying payload sizes

### 5. **high-throughput** - Maximum Operations Pattern
```yaml
name: high-throughput
operations:
  - command: GET
    ratio: 0.8
  - command: SET
    ratio: 0.2
data_size:
  fixed: 100        # Small payloads
clients: 100        # More connections
pipeline: 10        # Pipelining enabled
```
**Use Case:** Maximum ops/sec testing, capacity planning

### 6. **low-latency** - Latency-Sensitive Pattern
```yaml
name: low-latency
operations:
  - command: GET
    ratio: 0.9
  - command: SET
    ratio: 0.1
data_size:
  fixed: 64         # Minimal payload
threads: 2          # Fewer threads
clients: 10         # Fewer clients
pipeline: 1         # No pipelining
```
**Use Case:** Latency-critical applications, trading systems

### 7. **session** - Session Store Pattern
```yaml
name: session
operations:
  - command: GET
    ratio: 0.7      # Read session
  - command: SET
    ratio: 0.3      # Update session
key_pattern:
  prefix: "session:"
  key_range: 100000
data_size:
  fixed: 512        # Session data
```
**Use Case:** Web session management, authentication tokens

### 8. **large-values** - Bandwidth Testing Pattern
```yaml
name: large-values
operations:
  - command: GET
    ratio: 0.5
  - command: SET
    ratio: 0.5
data_size:
  min: 4096         # 4KB
  max: 16384        # 16KB
key_pattern:
  key_range: 10000  # Fewer keys
clients: 20         # Fewer clients
```
**Use Case:** Large object caching, serialization overhead testing

### Custom Workloads

Load custom workloads from YAML files:

```bash
redismeter run my-workload.yaml --target redis:6379
```

```yaml
# my-workload.yaml
name: custom-workload
description: Custom benchmark configuration
type: custom
operations:
  - command: HGET
    ratio: 0.5
  - command: HSET
    ratio: 0.5
key_pattern:
  prefix: "user:"
  pattern: sequential
  key_range: 50000
data_size:
  fixed: 1024
threads: 8
clients: 100
duration: 60s
pipeline: 5
```

---

## Persistence Layer

RedisMeter supports pluggable storage backends for benchmark results:

### Storage Interface

```go
type RunStorage interface {
    plugin.StoragePlugin
    
    SaveRun(ctx context.Context, run *domain.BenchmarkRun) error
    ListRuns(ctx context.Context, limit int) ([]*domain.BenchmarkRun, error)
    GetRun(ctx context.Context, id string) (*domain.BenchmarkRun, error)
    DeleteRun(ctx context.Context, id string) error
}
```

### Storage Backends

#### 1. File Storage (Default)
**Location:** `~/.redismeter/`

```
~/.redismeter/
├── runs/           # Benchmark run JSON files
│   ├── 20240115-143022-abc123.json
│   └── 20240115-150045-def456.json
├── baselines/      # Baseline comparisons
├── workloads/      # Custom workload definitions
└── config/         # Configuration files
```

**Characteristics:**
- Simple, portable, human-readable
- Each run stored as a separate JSON file
- No external dependencies
- Suitable for individual use and small teams

**Implementation:**
```go
type FileStorage struct {
    baseDir string
    mu      sync.RWMutex
}
```

#### 2. SQLite Storage
**Location:** `~/.redismeter/redismeter.db`

**Schema:**
```sql
-- Benchmark runs with indexed fields
CREATE TABLE benchmark_runs (
    id TEXT PRIMARY KEY,
    created_at DATETIME NOT NULL,
    status TEXT NOT NULL,
    workload_name TEXT,
    target_host TEXT,
    target_port INTEGER,
    -- JSON fields for complex data
    workload_json TEXT,
    results_json TEXT,
    environment_json TEXT
);

-- Indexes for efficient queries
CREATE INDEX idx_runs_status ON benchmark_runs(status);
CREATE INDEX idx_runs_workload ON benchmark_runs(workload_name);
CREATE INDEX idx_runs_created ON benchmark_runs(created_at);

-- Tag support for filtering
CREATE TABLE run_tags (
    run_id TEXT NOT NULL,
    tag TEXT NOT NULL,
    PRIMARY KEY (run_id, tag),
    FOREIGN KEY (run_id) REFERENCES benchmark_runs(id)
);

-- Baseline management
CREATE TABLE baselines (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    run_id TEXT,
    active INTEGER DEFAULT 1,
    metrics_json TEXT,
    thresholds_json TEXT
);
```

**Characteristics:**
- WAL mode for concurrent read/write
- Foreign keys for referential integrity
- Efficient querying with indexes
- Suitable for larger datasets and teams

**Configuration:**
```go
type SQLiteStorage struct {
    db     *sql.DB
    dbPath string
    mu     sync.RWMutex
}
```

### Storage Configuration

```go
type StorageConfig struct {
    Type StorageType  // "file" or "sqlite"
    Path string       // Base directory or database path
}

// Default configuration
func DefaultConfig() StorageConfig {
    return StorageConfig{
        Type: StorageTypeFile,
        Path: "~/.redismeter",
    }
}
```

**Via CLI:**
```bash
# Use SQLite backend
redismeter run cache --storage-type sqlite --storage-path ./data

# Use file backend (default)
redismeter run cache --storage-path /custom/path
```

### Data Model Persistence

**BenchmarkRun** - Complete benchmark execution record:
```go
type BenchmarkRun struct {
    ID          string
    CreatedAt   time.Time
    UpdatedAt   time.Time
    
    // Configuration
    Workload    *Workload
    Target      *Target
    Environment *Environment
    
    // Execution
    Status    RunStatus  // pending, running, completed, failed
    StartTime time.Time
    EndTime   time.Time
    Duration  string
    
    // Results
    Results *Results
    Error   string
    
    // Metadata
    Name   string
    Tags   []string
    Labels map[string]string
}
```

---

## Domain Model

### Key Entities

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                            BenchmarkRun                                     │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │  Workload   │  │   Target    │  │ Environment │  │      Results        │ │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────────────┘ │
│        │                │                │                    │             │
│        ▼                ▼                ▼                    ▼             │
│  ┌───────────┐   ┌───────────┐   ┌───────────────┐   ┌─────────────────┐   │
│  │Operations │   │Connection │   │  Fingerprint  │   │ SummaryMetrics  │   │
│  │KeyPattern │   │  Details  │   │  Timestamps   │   │OperationMetrics │   │
│  │ DataSize  │   │   Auth    │   │   Version     │   │   TimeSeries    │   │
│  └───────────┘   └───────────┘   └───────────────┘   └─────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Run Lifecycle

```
    ┌─────────┐
    │ PENDING │
    └────┬────┘
         │ Start execution
         ▼
    ┌─────────┐
    │ RUNNING │ ─────────┐
    └────┬────┘          │
         │               │ Error/Timeout
         │ Success       │
         ▼               ▼
    ┌───────────┐   ┌────────┐
    │ COMPLETED │   │ FAILED │
    └───────────┘   └────────┘
         │
         │ User cancel
         ▼
    ┌───────────┐
    │ CANCELLED │
    └───────────┘
```

---

## Configuration

### Environment Variables

```bash
REDISMETER_STORAGE_TYPE=sqlite
REDISMETER_STORAGE_PATH=~/.redismeter
REDISMETER_SSH_KEY_PATH=~/.ssh/id_rsa
```

### Config File (`~/.redismeter/config.yaml`)

```yaml
storage:
  type: sqlite
  path: ~/.redismeter

ssh:
  user: ubuntu
  private_key_path: ~/.ssh/id_rsa
  port: 22
  connect_timeout: 30s

cloud:
  default_provider: aws
  default_region: us-east-1
  default_instance_type: c5.xlarge
```

---

## Summary

RedisMeter provides a flexible, scalable architecture for Redis benchmarking:

| Feature | Implementation |
|---------|----------------|
| **Single-Node Execution** | Direct memtier_benchmark integration |
| **Distributed Execution** | Multi-node SSH executor with parallel execution |
| **Cloud Infrastructure** | Pluggable provider interface (AWS, GCP, Azure) |
| **Workload Patterns** | 8 built-in patterns + custom YAML support |
| **Persistence** | File (JSON) and SQLite backends |
| **Result Aggregation** | Per-host collection with combined throughput |
| **Real-Time Metrics** | Streaming via channels during execution |

The architecture is designed to scale from simple local benchmarks to distributed tests generating millions of operations per second across multiple cloud instances.
