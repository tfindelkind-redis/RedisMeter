# RedisMeter

<p align="center">
  <img src="docs/logo.svg" alt="RedisMeter Logo" width="200">
</p>

A scalable Redis performance benchmarking and baseline tool built on memtier_benchmark, designed for repeatable load testing, baseline comparison, and performance regression analysis.

## 📚 Documentation

Start here for a beginner-friendly learning path and concept guide:

- **[Documentation Hub](doc/Documentation_Index.md)** - Central index/agenda covering Benchmark, Baseline, job handling, cloud runners, and Azure execution.
- **[Beginner UI Workflow](doc/Beginner_UI_Workflow.md)** - First-time walkthrough from starting a benchmark to creating baselines and comparing runs.

## ✨ Features

### Core Benchmarking
- **memtier_benchmark Integration** - Leverages the industry-standard Redis benchmarking tool
- **Predefined Workloads** - Common patterns (GET-heavy, SET-heavy, mixed, etc.)
- **Custom Workloads** - Full flexibility with YAML-based configuration
- **Cluster Support** - Benchmark Redis Cluster with automatic slot discovery

### Baseline & Comparison
- **Environment Fingerprinting** - Captures host specs, Redis config, and network conditions
- **Threshold-based Comparisons** - Define acceptable regression percentages
- **Statistical Analysis** - Detect significant performance changes
- **CI/CD Integration** - Exit codes for pipeline automation

### Analytics & Visualization
- **Web Dashboard** - Beautiful, Grafana-style analytics UI
- **Real-time Monitoring** - WebSocket-based live benchmark progress
- **Historical Trends** - Track performance over time
- **Percentile Charts** - P50, P90, P95, P99, P99.9 visualizations

### Enterprise Features
- **Multi-tenant Organizations** - Team management with RBAC
- **API Key Authentication** - Secure programmatic access
- **Audit Logging** - Track all operations
- **Storage Backends** - PostgreSQL, SQLite, or in-memory

### Operational Features
- **Structured Logging** - Comprehensive logging with filtering and export
- **Infrastructure Profiles** - Save and reuse cloud infrastructure configurations
- **Data Bundles** - Export/import all data for backup and migration

## 🚀 Quick Start

### Prerequisites
- Go 1.21+
- Node.js 18+ (for Web UI)
- memtier_benchmark
- Redis (for testing)

### Installation

```bash
# Clone the repository
git clone https://github.com/your-org/redismeter.git
cd redismeter

# Build the server
make build

# Start the server
./bin/redismeter serve

# In another terminal, start the Web UI
cd web
npm install
npm run dev
```

### Run Your First Benchmark

```bash
# Using the CLI
./bin/redismeter run --target localhost:6379 --workload get-heavy --duration 60s

# Using the API
curl -X POST http://localhost:8080/api/v1/benchmark \
  -H "Content-Type: application/json" \
  -d '{
    "target": {"host": "localhost", "port": 6379},
    "workload": "get-heavy",
    "duration": "30s"
  }'
```

## 📊 Web Dashboard

RedisMeter includes a modern, dark-themed analytics dashboard built with React and Ant Design.

### Dashboard Features

| Page | Description |
|------|-------------|
| **Dashboard** | Overview with key metrics, recent runs, and summary charts |
| **Runs** | Browse, search, and filter all benchmark runs |
| **Run Detail** | Deep-dive into individual run results with charts |
| **Baselines** | Manage performance baselines for comparison |
| **Compare** | Side-by-side comparison with radar charts |
| **New Benchmark** | Launch benchmarks with real-time progress |
| **Analytics** | Historical trends and workload distribution |

### Visualizations

- **Throughput Charts** - Area charts with gradient fills
- **Latency Distribution** - Horizontal bar charts for percentiles
- **Histogram** - Latency distribution with P99 reference line
- **Radar Charts** - Multi-metric comparison at a glance
- **Sparklines** - Mini trend indicators in metric cards

### Starting the UI

```bash
cd web
npm install
npm run dev
```

The UI will be available at `http://localhost:5173` and automatically proxies API requests to the backend.

## 🔧 Configuration

### Server Configuration

```yaml
# config.yaml
server:
  port: 8080
  host: "0.0.0.0"

storage:
  type: "postgres"  # postgres, sqlite, memory
  postgres:
    host: "localhost"
    port: 5432
    database: "redismeter"
    
auth:
  enabled: true
  api_key_header: "X-API-Key"
```

### Workload Definition

```yaml
# workloads/custom.yaml
name: my-custom-workload
type: mixed
operations:
  - command: GET
    ratio: 0.7
  - command: SET
    ratio: 0.3
key_pattern:
  prefix: "bench:"
  pattern: "random"
  key_range: 10000
data_size:
  min: 64
  max: 1024
threads: 4
clients: 50
duration: "60s"
```

## 📈 API Reference

### Runs

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/runs` | List all runs |
| GET | `/api/v1/runs/:id` | Get run details |
| DELETE | `/api/v1/runs/:id` | Delete a run |

### Baselines

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/baselines` | List baselines |
| POST | `/api/v1/baselines` | Create baseline from run |
| DELETE | `/api/v1/baselines/:id` | Delete baseline |

### Benchmark

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/benchmark` | Start new benchmark |
| GET | `/api/v1/benchmark/:id` | Get status |
| DELETE | `/api/v1/benchmark/:id` | Cancel benchmark |

### Analysis

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/compare` | Compare run vs baseline |
| GET | `/api/v1/analyze` | Run performance analysis |

### Logs

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/logs` | Query logs with filters |
| DELETE | `/api/v1/logs` | Delete logs matching filter |
| GET | `/api/v1/logs/stats` | Get log statistics |
| GET | `/api/v1/logs/export` | Export logs in JSON/CSV format |

### Infrastructure Profiles

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/infra-profiles` | List all profiles |
| POST | `/api/v1/infra-profiles` | Create new profile |
| GET | `/api/v1/infra-profiles/:id` | Get profile details |
| PUT | `/api/v1/infra-profiles/:id` | Update profile |
| DELETE | `/api/v1/infra-profiles/:id` | Delete profile |
| GET | `/api/v1/infra-profiles/stats` | Get profile statistics |

### Bundle Export/Import

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/bundle/export` | Export data bundle |
| POST | `/api/v1/bundle/import` | Import data bundle |
| POST | `/api/v1/bundle/info` | Get bundle metadata |

## 🧪 CI/CD Integration

```yaml
# .github/workflows/benchmark.yml
- name: Run Benchmark
  run: |
    redismeter run \
      --target ${{ secrets.REDIS_HOST }}:6379 \
      --workload production-like \
      --compare-baseline latest \
      --fail-on-regression 10
```

## 🔧 CLI Commands

### Logging Commands

```bash
# List recent logs
redismeter logs list --limit 50

# Show specific log entry
redismeter logs show <log-id>

# Export logs to file
redismeter logs export --output logs.json --format json

# Clear old logs
redismeter logs clear --before 2024-01-01

# View log statistics
redismeter logs stats
```

### Infrastructure Profile Commands

```bash
# List all profiles
redismeter profiles list

# Show profile details
redismeter profiles show my-aws-profile

# Create a new profile
redismeter profiles create --name prod-aws --provider aws \
  --region us-east-1 --instance-type r6g.xlarge

# Delete a profile
redismeter profiles delete my-old-profile

# Export profiles
redismeter profiles export --output profiles.json

# Import profiles
redismeter profiles import --file profiles.json

# View profile statistics
redismeter profiles stats
```

### Bundle Export/Import Commands

```bash
# Export all data to a bundle
redismeter bundle export --output backup.tar.gz

# Export with specific data types
redismeter bundle export --output partial.tar.gz \
  --include-runs --include-baselines --exclude-logs

# Import a bundle
redismeter bundle import --file backup.tar.gz

# Import with merge strategy (don't overwrite existing)
redismeter bundle import --file backup.tar.gz --merge

# View bundle information without importing
redismeter bundle info --file backup.tar.gz
```

## 📁 Project Structure

```
redismeter/
├── cmd/                    # CLI commands
├── internal/
│   ├── api/               # REST API server
│   ├── alerting/          # Alert system
│   ├── analysis/          # Performance analyzers
│   ├── auth/              # Authentication (OIDC, API keys)
│   ├── bundle/            # Export/import bundles
│   ├── cicd/              # CI/CD integration
│   ├── cli/               # CLI command handlers
│   ├── cloud/             # Cloud providers (AWS, etc.)
│   ├── cluster/           # Redis Cluster support
│   ├── domain/            # Domain models
│   ├── engine/            # Benchmark engine
│   ├── environment/       # Environment capture
│   ├── export/            # Data export
│   ├── importer/          # Data import
│   ├── infraprofile/      # Infrastructure profiles
│   ├── logging/           # Structured logging
│   ├── memtier/           # memtier_benchmark integration
│   ├── observability/     # Metrics and tracing
│   ├── org/               # Organization/team management
│   ├── plugin/            # Plugin system
│   ├── reporter/          # Report generation
│   ├── storage/           # Storage backends
│   ├── terraform/         # Terraform integration
│   ├── workload/          # Workload registry
│   └── workloads/         # Built-in workload modules
├── pkg/
│   └── pluginkit/         # Plugin development kit
├── sdk/
│   ├── go/                # Go SDK
│   └── python/            # Python SDK
├── web/                   # React Web UI
│   ├── src/
│   │   ├── pages/        # Page components
│   │   ├── components/   # Reusable components
│   │   ├── api/          # API client
│   │   └── store/        # State management
│   └── package.json
└── workloads/            # Predefined workloads
```

## 🤝 Contributing

Contributions are welcome! Please read our [Contributing Guide](CONTRIBUTING.md) for details.

## 📄 License

MIT License - see [LICENSE](LICENSE) for details.
