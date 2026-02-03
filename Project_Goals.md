Project Goals

The goal of this project is to provide a Redis-native performance benchmarking and baselining framework that enables engineers to reliably measure, compare, and understand Redis performance at scale.

Specifically, this project aims to:

1. Enable repeatable and comparable Redis benchmarks

Provide a structured way to define workloads, datasets, and execution environments so that benchmark results are reproducible and comparable over time. The tool should make it easy to establish trusted performance baselines and detect regressions caused by configuration changes, version upgrades, or infrastructure differences.

2. Support scale-out load generation

Allow benchmarks to be executed using distributed load generators to accurately simulate real-world traffic patterns and high-throughput scenarios that exceed the capacity of a single client node. Scale-out execution should be simple to configure and transparent to the user.

3. Build on proven Redis benchmarking tools

Leverage memtier_benchmark as the core load-generation engine to remain aligned with Redis-recommended benchmarking practices, while extending it with orchestration, result aggregation, and higher-level workflows.

4. Treat baselines as first-class artifacts

Introduce explicit support for performance baselines, including storage of benchmark metadata (workload, Redis version, configuration, topology, environment) and automated comparison of new runs against historical results.

5. Provide actionable performance insights

Go beyond raw throughput and latency numbers by correlating benchmark results with Redis-level signals (e.g., latency events, slow commands, resource utilization) to help users understand why performance changes occur, not just that they occur.

6. Offer both automation and usability

Expose functionality through a CLI suitable for automation and CI/CD pipelines, while also providing a UI for exploration, visualization, and analysis of benchmark results and trends.

7. Integrate with existing observability ecosystems

Ensure benchmark results and metadata can be exported or integrated with common monitoring and observability tools, enabling teams to correlate benchmark data with system metrics and production telemetry.
