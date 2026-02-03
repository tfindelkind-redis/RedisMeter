# RedisMeter Python SDK

Python client library for the RedisMeter benchmarking tool.

## Installation

```bash
# Basic installation (requires either httpx or requests)
pip install redismeter

# With httpx (recommended)
pip install redismeter[httpx]

# With requests
pip install redismeter[requests]

# For async support
pip install redismeter[async]

# Install all optional dependencies
pip install redismeter[all]
```

## Quick Start

```python
from redismeter import Client

# Create a client
client = Client("http://localhost:8080", api_key="your-api-key")

# Run a benchmark
run = client.run_benchmark(
    target="localhost:6379",
    workload="mixed",
    duration="30s",
    clients=50,
)

print(f"Run completed: {run.id}")
print(f"Throughput: {run.results.summary.ops_per_sec:.2f} ops/sec")
print(f"P99 Latency: {run.results.summary.latency_p99_us:.2f} µs")

# Compare to baseline
comparison = client.compare_to_baseline(run.id)
if comparison.regression_detected:
    print(f"⚠️  Regression detected!")
    for diff in comparison.differences:
        print(f"  {diff['metric']}: {diff['change_percent']:.1f}%")
```

## Async Support

```python
import asyncio
from redismeter import AsyncClient

async def main():
    async with AsyncClient("http://localhost:8080", api_key="key") as client:
        runs = await client.list_runs(limit=10)
        for run in runs:
            print(f"{run.id}: {run.status}")

asyncio.run(main())
```

## API Reference

### Client

```python
Client(
    base_url: str,              # RedisMeter API URL
    api_key: str = None,        # API key for authentication
    timeout: float = 30.0,      # Request timeout in seconds
    verify_ssl: bool = True,    # Verify SSL certificates
    headers: dict = None,       # Additional headers
)
```

### Methods

#### Runs

- `list_runs(limit, offset, status, workload, tags)` - List benchmark runs
- `get_run(run_id)` - Get a specific run
- `delete_run(run_id)` - Delete a run
- `run_benchmark(target, workload, ...)` - Run a new benchmark

#### Baselines

- `list_baselines(limit, offset, active_only, name)` - List baselines
- `get_baseline(baseline_id)` - Get a specific baseline
- `create_baseline(name, run_id, ...)` - Create a baseline from a run
- `delete_baseline(baseline_id)` - Delete a baseline
- `set_active_baseline(baseline_id)` - Set as active baseline

#### Comparison & Analysis

- `compare_runs(run_id, other_run_id)` - Compare two runs
- `compare_to_baseline(run_id, baseline_id)` - Compare run to baseline
- `analyze_run(run_id, analyzers)` - Analyze a run

#### Workloads

- `list_workloads()` - List available workloads
- `get_workload(name)` - Get workload details

#### Health

- `health()` - Check API health

## Models

### BenchmarkRun

```python
@dataclass
class BenchmarkRun:
    id: str
    name: str
    description: str
    status: str  # pending, running, completed, failed
    workload: Workload
    target: Target
    results: Results
    tags: List[str]
    labels: Dict[str, str]
    created_at: datetime
    started_at: datetime
    completed_at: datetime
    duration: str
    error: str
```

### Results

```python
@dataclass
class Results:
    summary: SummaryMetrics
    latency_histogram: List[HistogramBucket]
    time_series: List[TimeSeriesPoint]
```

### SummaryMetrics

```python
@dataclass
class SummaryMetrics:
    total_ops: int
    ops_per_sec: float
    total_bytes: int
    bytes_per_sec: float
    latency_avg_us: float
    latency_min_us: float
    latency_max_us: float
    latency_p50_us: float
    latency_p95_us: float
    latency_p99_us: float
    latency_p999_us: float
    errors: int
    error_rate: float
```

## Exceptions

```python
from redismeter.exceptions import (
    RedisMeterError,      # Base exception
    APIError,             # General API error
    AuthenticationError,  # Authentication failed
    NotFoundError,        # Resource not found
    ValidationError,      # Validation error
    RateLimitError,       # Rate limit exceeded
    ConnectionError,      # Connection failed
)
```

## CI/CD Integration

### GitHub Actions

```yaml
- name: Run Benchmark
  run: |
    pip install redismeter[httpx]
    python -c "
    from redismeter import Client
    
    client = Client('${{ secrets.REDISMETER_URL }}', api_key='${{ secrets.REDISMETER_KEY }}')
    
    run = client.run_benchmark(
        target='localhost:6379',
        workload='mixed',
        duration='30s',
    )
    
    comparison = client.compare_to_baseline(run.id)
    if comparison.regression_detected:
        print('Regression detected!')
        exit(1)
    "
```

## License

MIT License
