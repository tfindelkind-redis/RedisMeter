# RedisMeter Documentation Hub

This page is the central index for understanding RedisMeter from beginner to advanced usage.

If you are new, follow the sections in order.

## Start Here by Role

| I am a... | Read this first | Then |
|-----------|-----------------|------|
| Beginner | [Beginner_UI_Workflow](./Beginner_UI_Workflow.md) | [Architecture](./Architecture.md) |
| Operator | [README](../README.md) | [Azure_Managed_Redis_Design](./Azure_Managed_Redis_Design.md) |
| Contributor | [Architecture](./Architecture.md) | [PROJECT_STATUS](../PROJECT_STATUS.md) |

## Find Docs by Question

| Question | Best Document |
|----------|---------------|
| What is RedisMeter and how do I start? | [README](../README.md) |
| How does the UI workflow work end-to-end? | [Beginner_UI_Workflow](./Beginner_UI_Workflow.md) |
| What is a benchmark, baseline, or runner? | [Beginner_UI_Workflow](./Beginner_UI_Workflow.md) |
| How do cloud runs and Azure runners work? | [Azure_Deployment_Design](./Azure_Deployment_Design.md) |
| How does AMR targeting and networking work? | [Azure_Managed_Redis_Design](./Azure_Managed_Redis_Design.md) |
| How is execution orchestrated under the hood? | [Architecture](./Architecture.md) |

## Learning Path

1. Start Here: What RedisMeter is and the key terms
2. Run Your First Benchmark: End-to-end UI workflow
3. Understand Core Concepts: Benchmark, Baseline, Compare, Analysis
4. Understand Job Handling: How benchmark jobs are tracked and executed
5. Move to Cloud Execution: What a runner is and how distributed runs work
6. Azure Deep Dive: Deployment model, AMR, networking, authentication
7. Architecture and internals: Components and data flow
8. SDK and extensibility: Automating and integrating RedisMeter

---

## 1) Start Here

- [README](../README.md): Product overview, quick start, main capabilities, API and CLI overview.
- [Beginner_UI_Workflow](./Beginner_UI_Workflow.md): Hands-on UI-first tutorial covering benchmarks, baselines, job lifecycle, runners, and Azure cloud execution basics.
- [Project_Goals](../Project_Goals.md): Why RedisMeter exists and long-term objectives.
- [PROJECT_STATUS](../PROJECT_STATUS.md): What is implemented today.

Recommended for beginners:
- Read README first.
- Skim Project Goals for context.
- Use Project Status only as reference.

---

## 2) Core Concepts (Beginner Essentials)

These concepts appear across the CLI, API, and UI.

### Benchmark
A benchmark is one execution of memtier-based load generation against a Redis target.

Includes:
- Workload definition (for example cache, mixed, read-heavy)
- Target details (host, port, auth, TLS)
- Runtime settings (duration or requests, clients, threads)
- Result metrics (throughput, latency percentiles, errors, histogram)

### Baseline
A baseline is a selected benchmark run treated as the reference point.

Purpose:
- Compare newer runs against a known good run
- Detect regressions and performance drift
- Support CI gates and release checks

### Compare and Analysis
Comparison computes differences between runs or between run and baseline.
Analysis adds statistical interpretation and anomaly detection.

### Job Handling
A benchmark job is the runtime lifecycle of a benchmark request.
Typical lifecycle:
- Pending/Created
- Provisioning or preparation
- Running
- Collecting/Aggregating results
- Completed or Failed

In the UI this appears in New Benchmark progress and Run Detail status data.

### Runner
A runner is a machine that executes memtier_benchmark.

- Local runs: your local machine is the runner.
- Cloud runs: one or more cloud VMs are runners.
- Distributed runs: multiple runners execute in parallel and results are aggregated.

---

## 3) UI Guide (What you see in the Web App)

The UI mirrors backend concepts and is best learned in this order:

1. Dashboard: high-level health and recent activity
2. New Benchmark: launch benchmark jobs and watch progress
3. Runs: browse historical executions
4. Run Detail: inspect metrics, environment, and cloud metadata
5. Baselines: create and manage reference runs
6. Compare: run-vs-run and run-vs-baseline differences
7. Analytics: trend and distribution views

Use this mental model:
- New Benchmark creates jobs
- Runs stores outcomes
- Baselines marks trusted references
- Compare and Analytics explain performance changes

---

## 4) Architecture and Execution Model

- [Architecture](./Architecture.md): Engine, memtier executor, workload registry, storage, distributed execution.

Focus areas for first architecture pass:
- Core components and responsibilities
- Local vs distributed execution
- Data aggregation and persistence

---

## 5) Azure and Cloud Benchmarking

Start with these in order:

1. [Azure_Deployment_Design](./Azure_Deployment_Design.md): Full control-plane model, auth, RBAC, and deployment flow.
2. [Azure_Managed_Redis_Design](./Azure_Managed_Redis_Design.md): AMR target modes, templates, networking, and AMR specifics.
3. [Azure_Provider_Design](./Azure_Provider_Design.md): Provider state machine, synchronization, retries, and failure handling.
4. [Azure_Deployment_Methods](./Azure_Deployment_Methods.md): Why Bicep plus SDK is used and AMR deployment caveats.

Cloud terms in plain language:
- Control plane: RedisMeter process running on your machine.
- Data plane: traffic from runners to Redis target.
- Runner VM: remote machine executing memtier.
- Target: Redis endpoint under test.
- Aggregation: merging metrics from all runners into one run result.

---

## 6) API and CLI Documentation Entry Points

- [README](../README.md): API endpoint tables and CLI command examples.

Suggested API first steps:
1. Create a benchmark job
2. Poll benchmark status
3. Fetch run details
4. Create baseline from run
5. Compare against baseline

---

## 7) SDK and Extensibility

- [Go SDK](../sdk/go/README.md)
- [Python SDK](../sdk/python/README.md)
- [Plugin Kit](../pkg/pluginkit/README.md)

Use when:
- Integrating RedisMeter into automation scripts
- Building custom extensions and workflows
- Embedding benchmark orchestration into internal tooling

---

## 8) Implementation and Roadmap References

- [IMPLEMENTATION_PLAN](../IMPLEMENTATION_PLAN.md): phased implementation reference.
- [ROADMAP](../ROADMAP.md): future direction.
- [TASKS](../TASKS.md): work items and execution tracking.

These are supporting references, not beginner-first documents.

---

## Recommended Reading Sequences

### Beginner (about 30 minutes)
1. [README](../README.md)
2. [Beginner_UI_Workflow](./Beginner_UI_Workflow.md)
3. [Architecture](./Architecture.md)
4. [Azure_Deployment_Design](./Azure_Deployment_Design.md) (only if using cloud)

### Operator (running regular tests)
1. [README](../README.md)
2. [Architecture](./Architecture.md)
3. [Azure_Managed_Redis_Design](./Azure_Managed_Redis_Design.md)
4. [Azure_Provider_Design](./Azure_Provider_Design.md)

### Contributor (working on code)
1. [Architecture](./Architecture.md)
2. [PROJECT_STATUS](../PROJECT_STATUS.md)
3. [IMPLEMENTATION_PLAN](../IMPLEMENTATION_PLAN.md)
4. [TASKS](../TASKS.md)

---

## Quick Glossary

- Benchmark run: one executed performance test.
- Baseline: benchmark run used as comparison reference.
- Runner: machine running memtier_benchmark.
- Workload: operation mix and parameters used in a run.
- Compare: metric deltas between two runs.
- Analysis: statistical interpretation of run data.
- AMR: Azure Managed Redis.
- Infrastructure profile: reusable cloud deployment configuration.
