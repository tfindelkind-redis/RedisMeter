"""
RedisMeter Python SDK

A Python client library for interacting with the RedisMeter API.

Example usage:
    from redismeter import Client

    client = Client("http://localhost:8080", api_key="your-api-key")
    
    # Run a benchmark
    result = client.run_benchmark(
        target="localhost:6379",
        workload="cache",
        duration="30s"
    )
    
    # Get results
    run = client.get_run(result.id)
    print(f"Throughput: {run.results.summary.ops_per_second} ops/sec")
"""

from .client import Client, AsyncClient
from .models import (
    BenchmarkRun,
    RunStatus,
    Workload,
    Target,
    Results,
    SummaryMetrics,
    Baseline,
    ComparisonResult,
    AnalysisResult,
)
from .exceptions import (
    RedisMeterError,
    APIError,
    AuthenticationError,
    NotFoundError,
    ValidationError,
    RateLimitError,
)

__version__ = "1.0.0"
__all__ = [
    "Client",
    "AsyncClient",
    "BenchmarkRun",
    "RunStatus",
    "Workload",
    "Target",
    "Results",
    "SummaryMetrics",
    "Baseline",
    "ComparisonResult",
    "AnalysisResult",
    "RedisMeterError",
    "APIError",
    "AuthenticationError",
    "NotFoundError",
    "ValidationError",
    "RateLimitError",
]
