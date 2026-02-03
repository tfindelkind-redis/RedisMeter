"""
RedisMeter API client implementation.
"""

import time
from typing import Any, Dict, List, Optional, Union
from urllib.parse import urljoin

try:
    import httpx
    HAS_HTTPX = True
except ImportError:
    HAS_HTTPX = False

try:
    import requests
    HAS_REQUESTS = True
except ImportError:
    HAS_REQUESTS = False

from .models import (
    BenchmarkRun,
    Baseline,
    ComparisonResult,
    AnalysisResult,
    Workload,
    Target,
)
from .exceptions import (
    APIError,
    AuthenticationError,
    NotFoundError,
    ValidationError,
    RateLimitError,
    ConnectionError,
)


class Client:
    """
    Synchronous client for the RedisMeter API.
    
    Example:
        client = Client("http://localhost:8080", api_key="your-api-key")
        runs = client.list_runs(limit=10)
    """

    def __init__(
        self,
        base_url: str,
        api_key: Optional[str] = None,
        timeout: float = 30.0,
        verify_ssl: bool = True,
        headers: Optional[Dict[str, str]] = None,
    ):
        """
        Initialize the RedisMeter client.
        
        Args:
            base_url: Base URL of the RedisMeter API (e.g., "http://localhost:8080")
            api_key: API key for authentication
            timeout: Request timeout in seconds
            verify_ssl: Whether to verify SSL certificates
            headers: Additional headers to include in requests
        """
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.timeout = timeout
        self.verify_ssl = verify_ssl
        
        self._headers = {
            "Content-Type": "application/json",
            "Accept": "application/json",
            "User-Agent": "redismeter-python/1.0.0",
        }
        if api_key:
            self._headers["X-API-Key"] = api_key
        if headers:
            self._headers.update(headers)

        # Use httpx if available, fallback to requests
        if HAS_HTTPX:
            self._client = httpx.Client(
                base_url=self.base_url,
                headers=self._headers,
                timeout=timeout,
                verify=verify_ssl,
            )
            self._use_httpx = True
        elif HAS_REQUESTS:
            self._session = requests.Session()
            self._session.headers.update(self._headers)
            self._session.verify = verify_ssl
            self._use_httpx = False
        else:
            raise ImportError(
                "Either 'httpx' or 'requests' must be installed. "
                "Install with: pip install httpx  or  pip install requests"
            )

    def close(self):
        """Close the client and release resources."""
        if self._use_httpx and hasattr(self, "_client"):
            self._client.close()
        elif hasattr(self, "_session"):
            self._session.close()

    def __enter__(self):
        return self

    def __exit__(self, *args):
        self.close()

    def _request(
        self,
        method: str,
        path: str,
        params: Optional[Dict[str, Any]] = None,
        json: Optional[Dict[str, Any]] = None,
    ) -> Dict[str, Any]:
        """Make an HTTP request to the API."""
        url = f"{self.base_url}{path}"

        try:
            if self._use_httpx:
                response = self._client.request(
                    method=method,
                    url=path,
                    params=params,
                    json=json,
                )
                status_code = response.status_code
                try:
                    data = response.json()
                except Exception:
                    data = {"message": response.text}
            else:
                response = self._session.request(
                    method=method,
                    url=url,
                    params=params,
                    json=json,
                    timeout=self.timeout,
                )
                status_code = response.status_code
                try:
                    data = response.json()
                except Exception:
                    data = {"message": response.text}

        except Exception as e:
            raise ConnectionError(f"Failed to connect to RedisMeter API: {e}")

        # Handle errors
        if status_code == 401:
            raise AuthenticationError(data.get("message", "Authentication failed"))
        elif status_code == 404:
            raise NotFoundError(data.get("message", "Resource not found"))
        elif status_code == 422:
            raise ValidationError(
                data.get("message", "Validation failed"),
                field=data.get("field"),
                details=data.get("details"),
            )
        elif status_code == 429:
            raise RateLimitError(
                data.get("message", "Rate limit exceeded"),
                retry_after=data.get("retry_after"),
            )
        elif status_code >= 400:
            raise APIError(
                data.get("message", f"API error: {status_code}"),
                status_code=status_code,
                code=data.get("code"),
                details=data.get("details"),
            )

        return data

    # ==================== Runs ====================

    def list_runs(
        self,
        limit: int = 20,
        offset: int = 0,
        status: Optional[str] = None,
        workload: Optional[str] = None,
        tags: Optional[List[str]] = None,
    ) -> List[BenchmarkRun]:
        """
        List benchmark runs.
        
        Args:
            limit: Maximum number of runs to return
            offset: Number of runs to skip
            status: Filter by status (pending, running, completed, failed)
            workload: Filter by workload name
            tags: Filter by tags (runs with any of these tags)
        
        Returns:
            List of BenchmarkRun objects
        """
        params = {"limit": limit, "offset": offset}
        if status:
            params["status"] = status
        if workload:
            params["workload"] = workload
        if tags:
            params["tags"] = ",".join(tags)

        data = self._request("GET", "/api/v1/runs", params=params)
        return [BenchmarkRun.from_dict(r) for r in data.get("runs", [])]

    def get_run(self, run_id: str) -> BenchmarkRun:
        """
        Get a benchmark run by ID.
        
        Args:
            run_id: The run ID
        
        Returns:
            BenchmarkRun object
        """
        data = self._request("GET", f"/api/v1/runs/{run_id}")
        return BenchmarkRun.from_dict(data)

    def delete_run(self, run_id: str) -> None:
        """
        Delete a benchmark run.
        
        Args:
            run_id: The run ID to delete
        """
        self._request("DELETE", f"/api/v1/runs/{run_id}")

    def run_benchmark(
        self,
        target: Union[str, Target],
        workload: Union[str, Workload, Dict[str, Any]],
        duration: Optional[str] = None,
        clients: Optional[int] = None,
        threads: Optional[int] = None,
        name: Optional[str] = None,
        description: Optional[str] = None,
        tags: Optional[List[str]] = None,
        labels: Optional[Dict[str, str]] = None,
        wait: bool = True,
        poll_interval: float = 1.0,
        timeout: Optional[float] = None,
    ) -> BenchmarkRun:
        """
        Run a benchmark.
        
        Args:
            target: Redis target (URL string or Target object)
            workload: Workload name, Workload object, or dict
            duration: Benchmark duration (e.g., "30s", "1m")
            clients: Number of client connections
            threads: Number of threads
            name: Name for this run
            description: Description for this run
            tags: Tags to apply to the run
            labels: Key-value labels for the run
            wait: Whether to wait for completion
            poll_interval: Seconds between status checks when waiting
            timeout: Maximum time to wait for completion
        
        Returns:
            BenchmarkRun object (may be in-progress if wait=False)
        """
        # Build target
        if isinstance(target, str):
            if target.startswith("redis://") or target.startswith("rediss://"):
                target_obj = Target.from_url(target)
            else:
                parts = target.split(":")
                host = parts[0]
                port = int(parts[1]) if len(parts) > 1 else 6379
                target_obj = Target(host=host, port=port)
            target_dict = {
                "host": target_obj.host,
                "port": target_obj.port,
                "password": target_obj.password,
                "tls": target_obj.tls,
            }
        else:
            target_dict = {
                "host": target.host,
                "port": target.port,
                "password": target.password,
                "tls": target.tls,
            }

        # Build workload
        if isinstance(workload, str):
            workload_dict = {"name": workload}
        elif isinstance(workload, Workload):
            workload_dict = {
                "name": workload.name,
                "type": workload.type,
                "operations": [
                    {"command": op.command, "ratio": op.ratio, "args": op.args}
                    for op in workload.operations
                ],
            }
        else:
            workload_dict = workload

        # Add duration/clients/threads
        if duration:
            workload_dict["duration"] = duration
        if clients:
            workload_dict["clients"] = clients
        if threads:
            workload_dict["threads"] = threads

        body = {
            "target": target_dict,
            "workload": workload_dict,
        }
        if name:
            body["name"] = name
        if description:
            body["description"] = description
        if tags:
            body["tags"] = tags
        if labels:
            body["labels"] = labels

        data = self._request("POST", "/api/v1/benchmark", json=body)
        run = BenchmarkRun.from_dict(data)

        if not wait:
            return run

        # Poll for completion
        start_time = time.time()
        while run.status in ("pending", "running"):
            if timeout and (time.time() - start_time) > timeout:
                raise TimeoutError(f"Benchmark did not complete within {timeout} seconds")
            
            time.sleep(poll_interval)
            run = self.get_run(run.id)

        return run

    # ==================== Baselines ====================

    def list_baselines(
        self,
        limit: int = 20,
        offset: int = 0,
        active_only: bool = False,
        name: Optional[str] = None,
    ) -> List[Baseline]:
        """
        List baselines.
        
        Args:
            limit: Maximum number of baselines to return
            offset: Number of baselines to skip
            active_only: Only return active baselines
            name: Filter by name
        
        Returns:
            List of Baseline objects
        """
        params = {"limit": limit, "offset": offset}
        if active_only:
            params["active"] = "true"
        if name:
            params["name"] = name

        data = self._request("GET", "/api/v1/baselines", params=params)
        return [Baseline.from_dict(b) for b in data.get("baselines", [])]

    def get_baseline(self, baseline_id: str) -> Baseline:
        """
        Get a baseline by ID.
        
        Args:
            baseline_id: The baseline ID
        
        Returns:
            Baseline object
        """
        data = self._request("GET", f"/api/v1/baselines/{baseline_id}")
        return Baseline.from_dict(data)

    def create_baseline(
        self,
        name: str,
        run_id: str,
        description: Optional[str] = None,
        thresholds: Optional[Dict[str, float]] = None,
        tags: Optional[List[str]] = None,
    ) -> Baseline:
        """
        Create a baseline from a benchmark run.
        
        Args:
            name: Name for the baseline
            run_id: ID of the run to use
            description: Description for the baseline
            thresholds: Regression thresholds
            tags: Tags to apply
        
        Returns:
            Baseline object
        """
        body = {
            "name": name,
            "run_id": run_id,
        }
        if description:
            body["description"] = description
        if thresholds:
            body["thresholds"] = thresholds
        if tags:
            body["tags"] = tags

        data = self._request("POST", "/api/v1/baselines", json=body)
        return Baseline.from_dict(data)

    def delete_baseline(self, baseline_id: str) -> None:
        """
        Delete a baseline.
        
        Args:
            baseline_id: The baseline ID to delete
        """
        self._request("DELETE", f"/api/v1/baselines/{baseline_id}")

    def set_active_baseline(self, baseline_id: str) -> Baseline:
        """
        Set a baseline as the active baseline for its workload.
        
        Args:
            baseline_id: The baseline ID
        
        Returns:
            Updated Baseline object
        """
        data = self._request("PUT", f"/api/v1/baselines/{baseline_id}/active")
        return Baseline.from_dict(data)

    # ==================== Comparison ====================

    def compare_runs(
        self,
        run_id: str,
        other_run_id: str,
    ) -> ComparisonResult:
        """
        Compare two benchmark runs.
        
        Args:
            run_id: First run ID
            other_run_id: Second run ID to compare against
        
        Returns:
            ComparisonResult object
        """
        data = self._request(
            "GET",
            "/api/v1/compare",
            params={"run_id": run_id, "other_run_id": other_run_id},
        )
        return ComparisonResult.from_dict(data)

    def compare_to_baseline(
        self,
        run_id: str,
        baseline_id: Optional[str] = None,
    ) -> ComparisonResult:
        """
        Compare a run to a baseline.
        
        Args:
            run_id: Run ID to compare
            baseline_id: Baseline ID (uses active baseline if not specified)
        
        Returns:
            ComparisonResult object
        """
        params = {"run_id": run_id}
        if baseline_id:
            params["baseline_id"] = baseline_id

        data = self._request("GET", "/api/v1/compare/baseline", params=params)
        return ComparisonResult.from_dict(data)

    # ==================== Analysis ====================

    def analyze_run(
        self,
        run_id: str,
        analyzers: Optional[List[str]] = None,
    ) -> List[AnalysisResult]:
        """
        Analyze a benchmark run.
        
        Args:
            run_id: Run ID to analyze
            analyzers: Specific analyzers to run (runs all if not specified)
        
        Returns:
            List of AnalysisResult objects
        """
        params = {"run_id": run_id}
        if analyzers:
            params["analyzers"] = ",".join(analyzers)

        data = self._request("GET", "/api/v1/analyze", params=params)
        return [AnalysisResult.from_dict(r) for r in data.get("results", [])]

    # ==================== Workloads ====================

    def list_workloads(self) -> List[Dict[str, Any]]:
        """
        List available workloads.
        
        Returns:
            List of workload definitions
        """
        data = self._request("GET", "/api/v1/workloads")
        return data.get("workloads", [])

    def get_workload(self, name: str) -> Dict[str, Any]:
        """
        Get a workload by name.
        
        Args:
            name: Workload name
        
        Returns:
            Workload definition
        """
        return self._request("GET", f"/api/v1/workloads/{name}")

    # ==================== Health ====================

    def health(self) -> Dict[str, Any]:
        """
        Check API health.
        
        Returns:
            Health status
        """
        return self._request("GET", "/health")


class AsyncClient:
    """
    Asynchronous client for the RedisMeter API.
    
    Requires httpx to be installed with async support.
    
    Example:
        async with AsyncClient("http://localhost:8080", api_key="key") as client:
            runs = await client.list_runs(limit=10)
    """

    def __init__(
        self,
        base_url: str,
        api_key: Optional[str] = None,
        timeout: float = 30.0,
        verify_ssl: bool = True,
        headers: Optional[Dict[str, str]] = None,
    ):
        if not HAS_HTTPX:
            raise ImportError(
                "The 'httpx' library is required for async support. "
                "Install with: pip install httpx"
            )

        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.timeout = timeout

        self._headers = {
            "Content-Type": "application/json",
            "Accept": "application/json",
            "User-Agent": "redismeter-python/1.0.0",
        }
        if api_key:
            self._headers["X-API-Key"] = api_key
        if headers:
            self._headers.update(headers)

        self._client = httpx.AsyncClient(
            base_url=self.base_url,
            headers=self._headers,
            timeout=timeout,
            verify=verify_ssl,
        )

    async def close(self):
        """Close the client and release resources."""
        await self._client.aclose()

    async def __aenter__(self):
        return self

    async def __aexit__(self, *args):
        await self.close()

    async def _request(
        self,
        method: str,
        path: str,
        params: Optional[Dict[str, Any]] = None,
        json: Optional[Dict[str, Any]] = None,
    ) -> Dict[str, Any]:
        """Make an HTTP request to the API."""
        try:
            response = await self._client.request(
                method=method,
                url=path,
                params=params,
                json=json,
            )
            status_code = response.status_code
            try:
                data = response.json()
            except Exception:
                data = {"message": response.text}

        except Exception as e:
            raise ConnectionError(f"Failed to connect to RedisMeter API: {e}")

        # Handle errors (same as sync client)
        if status_code == 401:
            raise AuthenticationError(data.get("message", "Authentication failed"))
        elif status_code == 404:
            raise NotFoundError(data.get("message", "Resource not found"))
        elif status_code == 422:
            raise ValidationError(
                data.get("message", "Validation failed"),
                field=data.get("field"),
                details=data.get("details"),
            )
        elif status_code == 429:
            raise RateLimitError(
                data.get("message", "Rate limit exceeded"),
                retry_after=data.get("retry_after"),
            )
        elif status_code >= 400:
            raise APIError(
                data.get("message", f"API error: {status_code}"),
                status_code=status_code,
                code=data.get("code"),
                details=data.get("details"),
            )

        return data

    # Async versions of all methods...
    
    async def list_runs(
        self,
        limit: int = 20,
        offset: int = 0,
        status: Optional[str] = None,
        workload: Optional[str] = None,
        tags: Optional[List[str]] = None,
    ) -> List[BenchmarkRun]:
        """List benchmark runs."""
        params = {"limit": limit, "offset": offset}
        if status:
            params["status"] = status
        if workload:
            params["workload"] = workload
        if tags:
            params["tags"] = ",".join(tags)

        data = await self._request("GET", "/api/v1/runs", params=params)
        return [BenchmarkRun.from_dict(r) for r in data.get("runs", [])]

    async def get_run(self, run_id: str) -> BenchmarkRun:
        """Get a benchmark run by ID."""
        data = await self._request("GET", f"/api/v1/runs/{run_id}")
        return BenchmarkRun.from_dict(data)

    async def delete_run(self, run_id: str) -> None:
        """Delete a benchmark run."""
        await self._request("DELETE", f"/api/v1/runs/{run_id}")

    async def list_baselines(
        self,
        limit: int = 20,
        offset: int = 0,
        active_only: bool = False,
    ) -> List[Baseline]:
        """List baselines."""
        params = {"limit": limit, "offset": offset}
        if active_only:
            params["active"] = "true"

        data = await self._request("GET", "/api/v1/baselines", params=params)
        return [Baseline.from_dict(b) for b in data.get("baselines", [])]

    async def get_baseline(self, baseline_id: str) -> Baseline:
        """Get a baseline by ID."""
        data = await self._request("GET", f"/api/v1/baselines/{baseline_id}")
        return Baseline.from_dict(data)

    async def compare_runs(self, run_id: str, other_run_id: str) -> ComparisonResult:
        """Compare two benchmark runs."""
        data = await self._request(
            "GET",
            "/api/v1/compare",
            params={"run_id": run_id, "other_run_id": other_run_id},
        )
        return ComparisonResult.from_dict(data)

    async def analyze_run(
        self,
        run_id: str,
        analyzers: Optional[List[str]] = None,
    ) -> List[AnalysisResult]:
        """Analyze a benchmark run."""
        params = {"run_id": run_id}
        if analyzers:
            params["analyzers"] = ",".join(analyzers)

        data = await self._request("GET", "/api/v1/analyze", params=params)
        return [AnalysisResult.from_dict(r) for r in data.get("results", [])]

    async def health(self) -> Dict[str, Any]:
        """Check API health."""
        return await self._request("GET", "/health")
