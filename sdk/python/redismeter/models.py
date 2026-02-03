"""
Data models for the RedisMeter SDK.
"""

from dataclasses import dataclass, field
from datetime import datetime
from enum import Enum
from typing import Any, Dict, List, Optional


class RunStatus(str, Enum):
    """Status of a benchmark run."""
    PENDING = "pending"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"
    CANCELLED = "cancelled"


@dataclass
class Operation:
    """A Redis operation in a workload."""
    command: str
    ratio: float
    args: Optional[List[str]] = None


@dataclass
class KeyPattern:
    """Configuration for key generation."""
    prefix: Optional[str] = None
    pattern: str = "random"
    key_range: Optional[int] = None
    hotspot_fraction: Optional[float] = None


@dataclass
class DataSize:
    """Configuration for value sizes."""
    min: Optional[int] = None
    max: Optional[int] = None
    fixed: Optional[int] = None
    pattern: Optional[str] = None


@dataclass
class Workload:
    """Benchmark workload configuration."""
    name: str
    type: str
    description: Optional[str] = None
    operations: List[Operation] = field(default_factory=list)
    key_pattern: Optional[KeyPattern] = None
    data_size: Optional[DataSize] = None
    threads: Optional[int] = None
    clients: Optional[int] = None
    duration: Optional[str] = None
    requests: Optional[int] = None
    pipeline: Optional[int] = None
    custom: Optional[Dict[str, Any]] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Workload":
        """Create a Workload from a dictionary."""
        operations = []
        for op in data.get("operations", []):
            operations.append(Operation(
                command=op["command"],
                ratio=op["ratio"],
                args=op.get("args"),
            ))

        key_pattern = None
        if kp := data.get("key_pattern"):
            key_pattern = KeyPattern(
                prefix=kp.get("prefix"),
                pattern=kp.get("pattern", "random"),
                key_range=kp.get("key_range"),
                hotspot_fraction=kp.get("hotspot_fraction"),
            )

        data_size = None
        if ds := data.get("data_size"):
            data_size = DataSize(
                min=ds.get("min"),
                max=ds.get("max"),
                fixed=ds.get("fixed"),
                pattern=ds.get("pattern"),
            )

        return cls(
            name=data["name"],
            type=data.get("type", "custom"),
            description=data.get("description"),
            operations=operations,
            key_pattern=key_pattern,
            data_size=data_size,
            threads=data.get("threads"),
            clients=data.get("clients"),
            duration=data.get("duration"),
            requests=data.get("requests"),
            pipeline=data.get("pipeline"),
            custom=data.get("custom"),
        )


@dataclass
class Target:
    """Redis target configuration."""
    host: str
    port: int = 6379
    url: Optional[str] = None
    password: Optional[str] = None
    tls: bool = False
    cluster: bool = False

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Target":
        """Create a Target from a dictionary."""
        return cls(
            host=data["host"],
            port=data.get("port", 6379),
            url=data.get("url"),
            password=data.get("password"),
            tls=data.get("tls", False),
            cluster=data.get("cluster", False),
        )

    @classmethod
    def from_url(cls, url: str) -> "Target":
        """Create a Target from a Redis URL."""
        # Parse redis://host:port or rediss://host:port
        tls = url.startswith("rediss://")
        url_part = url.replace("redis://", "").replace("rediss://", "")
        
        # Handle auth
        password = None
        if "@" in url_part:
            auth, url_part = url_part.rsplit("@", 1)
            if ":" in auth:
                _, password = auth.split(":", 1)
            else:
                password = auth

        # Handle host:port
        if ":" in url_part:
            host, port_str = url_part.split(":", 1)
            port = int(port_str.split("/")[0])  # Remove database number if present
        else:
            host = url_part.split("/")[0]
            port = 6379

        return cls(host=host, port=port, password=password, tls=tls, url=url)


@dataclass
class Environment:
    """Environment information captured during benchmark."""
    hostname: Optional[str] = None
    os: Optional[str] = None
    arch: Optional[str] = None
    cpu_model: Optional[str] = None
    cpu_cores: Optional[int] = None
    memory_gb: Optional[float] = None
    fingerprint: Optional[str] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Environment":
        """Create an Environment from a dictionary."""
        return cls(
            hostname=data.get("hostname"),
            os=data.get("os"),
            arch=data.get("arch"),
            cpu_model=data.get("cpu_model"),
            cpu_cores=data.get("cpu_cores"),
            memory_gb=data.get("memory_gb"),
            fingerprint=data.get("fingerprint"),
        )


@dataclass
class LatencyPercentiles:
    """Latency percentile values."""
    p50: float
    p90: float
    p95: float
    p99: float
    p999: Optional[float] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "LatencyPercentiles":
        """Create LatencyPercentiles from a dictionary."""
        return cls(
            p50=data.get("p50", 0),
            p90=data.get("p90", 0),
            p95=data.get("p95", 0),
            p99=data.get("p99", 0),
            p999=data.get("p999"),
        )


@dataclass
class SummaryMetrics:
    """Summary metrics for a benchmark run."""
    ops_per_second: float
    avg_latency_ms: float
    p50_latency_ms: float
    p90_latency_ms: float
    p95_latency_ms: float
    p99_latency_ms: float
    p999_latency_ms: Optional[float] = None
    min_latency_ms: Optional[float] = None
    max_latency_ms: Optional[float] = None
    total_requests: Optional[int] = None
    total_errors: Optional[int] = None
    error_rate: Optional[float] = None
    bytes_per_second: Optional[float] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "SummaryMetrics":
        """Create SummaryMetrics from a dictionary."""
        return cls(
            ops_per_second=data.get("ops_per_second", 0),
            avg_latency_ms=data.get("avg_latency_ms", 0),
            p50_latency_ms=data.get("p50_latency_ms", 0),
            p90_latency_ms=data.get("p90_latency_ms", 0),
            p95_latency_ms=data.get("p95_latency_ms", 0),
            p99_latency_ms=data.get("p99_latency_ms", 0),
            p999_latency_ms=data.get("p999_latency_ms"),
            min_latency_ms=data.get("min_latency_ms"),
            max_latency_ms=data.get("max_latency_ms"),
            total_requests=data.get("total_requests"),
            total_errors=data.get("total_errors"),
            error_rate=data.get("error_rate"),
            bytes_per_second=data.get("bytes_per_second"),
        )


@dataclass
class Results:
    """Benchmark results."""
    summary: Optional[SummaryMetrics] = None
    raw: Optional[Dict[str, Any]] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Results":
        """Create Results from a dictionary."""
        summary = None
        if s := data.get("summary"):
            summary = SummaryMetrics.from_dict(s)

        return cls(
            summary=summary,
            raw=data.get("raw"),
        )


@dataclass
class BenchmarkRun:
    """A benchmark run with its results."""
    id: str
    created_at: datetime
    updated_at: datetime
    status: RunStatus
    workload: Optional[Workload] = None
    target: Optional[Target] = None
    environment: Optional[Environment] = None
    start_time: Optional[datetime] = None
    end_time: Optional[datetime] = None
    duration: Optional[str] = None
    results: Optional[Results] = None
    error: Optional[str] = None
    name: Optional[str] = None
    description: Optional[str] = None
    tags: List[str] = field(default_factory=list)
    labels: Dict[str, str] = field(default_factory=dict)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "BenchmarkRun":
        """Create a BenchmarkRun from a dictionary."""
        workload = None
        if w := data.get("workload"):
            workload = Workload.from_dict(w)

        target = None
        if t := data.get("target"):
            target = Target.from_dict(t)

        environment = None
        if e := data.get("environment"):
            environment = Environment.from_dict(e)

        results = None
        if r := data.get("results"):
            results = Results.from_dict(r)

        return cls(
            id=data["id"],
            created_at=_parse_datetime(data["created_at"]),
            updated_at=_parse_datetime(data["updated_at"]),
            status=RunStatus(data["status"]),
            workload=workload,
            target=target,
            environment=environment,
            start_time=_parse_datetime(data.get("start_time")),
            end_time=_parse_datetime(data.get("end_time")),
            duration=data.get("duration"),
            results=results,
            error=data.get("error"),
            name=data.get("name"),
            description=data.get("description"),
            tags=data.get("tags", []),
            labels=data.get("labels", {}),
        )


@dataclass
class BaselineMetrics:
    """Metrics stored in a baseline."""
    ops_per_second: float
    avg_latency_ms: float
    p99_latency_ms: float
    error_rate: Optional[float] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "BaselineMetrics":
        """Create BaselineMetrics from a dictionary."""
        return cls(
            ops_per_second=data.get("ops_per_second", 0),
            avg_latency_ms=data.get("avg_latency_ms", 0),
            p99_latency_ms=data.get("p99_latency_ms", 0),
            error_rate=data.get("error_rate"),
        )


@dataclass
class Baseline:
    """A performance baseline for comparison."""
    id: str
    name: str
    created_at: datetime
    updated_at: datetime
    run_id: Optional[str] = None
    description: Optional[str] = None
    active: bool = True
    valid_from: Optional[datetime] = None
    valid_until: Optional[datetime] = None
    metrics: Optional[BaselineMetrics] = None
    thresholds: Optional[Dict[str, float]] = None
    tags: List[str] = field(default_factory=list)
    labels: Dict[str, str] = field(default_factory=dict)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Baseline":
        """Create a Baseline from a dictionary."""
        metrics = None
        if m := data.get("metrics"):
            metrics = BaselineMetrics.from_dict(m)

        return cls(
            id=data["id"],
            name=data["name"],
            created_at=_parse_datetime(data["created_at"]),
            updated_at=_parse_datetime(data["updated_at"]),
            run_id=data.get("run_id"),
            description=data.get("description"),
            active=data.get("active", True),
            valid_from=_parse_datetime(data.get("valid_from")),
            valid_until=_parse_datetime(data.get("valid_until")),
            metrics=metrics,
            thresholds=data.get("thresholds"),
            tags=data.get("tags", []),
            labels=data.get("labels", {}),
        )


@dataclass
class MetricComparison:
    """Comparison of a single metric."""
    name: str
    current: float
    baseline: float
    diff: float
    diff_percent: float
    significant: bool
    improved: bool


@dataclass
class ComparisonResult:
    """Result of comparing two runs or a run to a baseline."""
    run_id: str
    baseline_id: Optional[str] = None
    compared_run_id: Optional[str] = None
    timestamp: Optional[datetime] = None
    metrics: List[MetricComparison] = field(default_factory=list)
    regression_detected: bool = False
    improvement_detected: bool = False
    summary: Optional[str] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "ComparisonResult":
        """Create a ComparisonResult from a dictionary."""
        metrics = []
        for m in data.get("metrics", []):
            metrics.append(MetricComparison(
                name=m["name"],
                current=m["current"],
                baseline=m["baseline"],
                diff=m["diff"],
                diff_percent=m["diff_percent"],
                significant=m.get("significant", False),
                improved=m.get("improved", False),
            ))

        return cls(
            run_id=data["run_id"],
            baseline_id=data.get("baseline_id"),
            compared_run_id=data.get("compared_run_id"),
            timestamp=_parse_datetime(data.get("timestamp")),
            metrics=metrics,
            regression_detected=data.get("regression_detected", False),
            improvement_detected=data.get("improvement_detected", False),
            summary=data.get("summary"),
        )


@dataclass
class Finding:
    """An analysis finding."""
    type: str
    severity: str
    message: str
    metric: Optional[str] = None
    value: Optional[float] = None
    threshold: Optional[float] = None
    recommendation: Optional[str] = None


@dataclass
class AnalysisResult:
    """Result of analyzing a benchmark run."""
    run_id: str
    analyzer: str
    timestamp: datetime
    findings: List[Finding] = field(default_factory=list)
    score: Optional[float] = None
    summary: Optional[str] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "AnalysisResult":
        """Create an AnalysisResult from a dictionary."""
        findings = []
        for f in data.get("findings", []):
            findings.append(Finding(
                type=f["type"],
                severity=f["severity"],
                message=f["message"],
                metric=f.get("metric"),
                value=f.get("value"),
                threshold=f.get("threshold"),
                recommendation=f.get("recommendation"),
            ))

        return cls(
            run_id=data["run_id"],
            analyzer=data["analyzer"],
            timestamp=_parse_datetime(data["timestamp"]),
            findings=findings,
            score=data.get("score"),
            summary=data.get("summary"),
        )


def _parse_datetime(value: Optional[str]) -> Optional[datetime]:
    """Parse an ISO format datetime string."""
    if not value:
        return None
    # Handle various formats
    try:
        return datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return None
