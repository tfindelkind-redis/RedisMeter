# Azure Provider Design Document

## Overview

This document describes the architecture and design decisions for RedisMeter's Azure cloud provider implementation, which enables distributed benchmark execution across multiple Azure VMs.

## Key Challenges

### 1. VM Provisioning Latency
Azure VMs take 2-5 minutes to fully provision. This includes:
- Resource allocation
- OS boot
- cloud-init execution
- Network configuration

**Solution**: State machine with clear phases, parallel provisioning, and progress tracking.

### 2. SSH Bootstrap
VMs need to be configured for SSH access and have memtier_benchmark installed.

**Solution**: cloud-init script that:
1. Configures SSH authorized_keys during VM creation
2. Installs memtier_benchmark dependencies
3. Builds memtier from source
4. Creates a ready marker file (`/tmp/memtier_ready`)

### 3. Synchronized Start
For accurate benchmark results, all runners must start at approximately the same time.

**Solution**: Two-phase execution with timestamp-based synchronization:
```
Phase 1: Stage scripts on all VMs (includes calculated start timestamp)
Phase 2: Scripts sleep until synchronized start time, then execute
```

### 4. Failure Handling
VMs can fail at any stage: provisioning, configuration, or execution.

**Solution**: 
- Configurable failure tolerance (`FailureTolerance` parameter)
- Exponential backoff for transient failures
- Health monitoring with heartbeat detection
- Graceful degradation (continue with N-1 runners)

## Architecture

### State Machine

```
┌──────────────┐
│ INITIALIZING │  Create deployment tracking, resource group
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ PROVISIONING │  Create VNet, NSG, VMs (parallel)
└──────┬───────┘
       │ Timeout: 10 minutes per VM
       ▼
┌──────────────┐
│ CONFIGURING  │  Wait for SSH, verify memtier installed
└──────┬───────┘
       │ Retry: 5 attempts with exponential backoff
       ▼
┌──────────────┐
│    READY     │  All VMs healthy and ready
└──────┬───────┘
       │
       ▼
┌──────────────┐
│SYNCHRONIZING │  Stage benchmark scripts with start timestamp
└──────┬───────┘
       │
       ▼
┌──────────────┐
│   RUNNING    │  Execute benchmarks, monitor health
└──────┬───────┘
       │ Health checks every 5 seconds
       ▼
┌──────────────┐
│  COLLECTING  │  Gather results from all VMs
└──────┬───────┘
       │
       ▼
┌──────────────┐
│  COMPLETED   │  Aggregate results
└──────┬───────┘
       │
       ▼
┌──────────────┐
│   CLEANUP    │  Delete resource group (all resources)
└──────────────┘
```

### Component Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                            AzureProvider                                    │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                         Azure SDK Clients                            │   │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │   │
│  │  │ Resource │ │    VM    │ │   NIC    │ │   VNet   │ │   NSG    │   │   │
│  │  │  Groups  │ │  Client  │ │  Client  │ │  Client  │ │  Client  │   │   │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ └──────────┘   │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                    │                                        │
│  ┌─────────────────────────────────┼─────────────────────────────────────┐ │
│  │                        Deployment Manager                              │ │
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌───────────────────────┐  │ │
│  │  │ State Machine   │  │ VM Tracker      │  │ Event Logger          │  │ │
│  │  │ - Phase control │  │ - Per-VM state  │  │ - Audit trail         │  │ │
│  │  │ - Transitions   │  │ - Health status │  │ - Progress reporting  │  │ │
│  │  └─────────────────┘  └─────────────────┘  └───────────────────────┘  │ │
│  └───────────────────────────────────────────────────────────────────────┘ │
│                                    │                                        │
│  ┌─────────────────────────────────┼─────────────────────────────────────┐ │
│  │                         SSH Executor                                   │ │
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌───────────────────────┐  │ │
│  │  │ Connection Pool │  │ Command Runner  │  │ Health Checker        │  │ │
│  │  │ - Keep-alive    │  │ - Sync commands │  │ - Heartbeat           │  │ │
│  │  │ - Retry logic   │  │ - Async start   │  │ - Failure detection   │  │ │
│  │  └─────────────────┘  └─────────────────┘  └───────────────────────┘  │ │
│  └───────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Synchronization Strategy

### The Challenge
If we SSH to each VM sequentially and start memtier, they won't start at the same time:
- VM1: starts at T+0s
- VM2: starts at T+1s (SSH overhead)
- VM3: starts at T+2s

This creates measurement skew, especially for short benchmarks.

### The Solution: Timestamp-Based Synchronization

```
Control Plane                VM1              VM2              VM3
     │                        │                │                │
     │  Stage script          │                │                │
     │  (start at T+5s)       │                │                │
     ├───────────────────────►│                │                │
     │                        │                │                │
     │  Stage script          │                │                │
     │  (start at T+5s)       │                │                │
     ├───────────────────────►├───────────────►│                │
     │                        │                │                │
     │  Stage script          │                │                │
     │  (start at T+5s)       │                │                │
     ├───────────────────────►├───────────────►├───────────────►│
     │                        │                │                │
     │                        │   Wait...      │   Wait...      │   Wait...
     │                        │                │                │
     │                     T+5s ═══════════════════════════════════════
     │                        │                │                │
     │                        │  START         │  START         │  START
     │                        │  memtier       │  memtier       │  memtier
     │                        │                │                │
```

### Script Template
```bash
#!/bin/bash
TARGET_TIME=1738600800  # Unix timestamp
CURRENT_TIME=$(date +%s)
SLEEP_TIME=$((TARGET_TIME - CURRENT_TIME))

if [ $SLEEP_TIME -gt 0 ]; then
    echo "Waiting $SLEEP_TIME seconds for synchronized start..."
    sleep $SLEEP_TIME
fi

echo "Starting benchmark at $(date)"
memtier_benchmark --server redis.example.com --port 6379 --json-out-file=/tmp/results.json
echo "Benchmark completed at $(date)"
```

## Failure Handling

### Failure Scenarios and Responses

| Scenario | Detection | Response |
|----------|-----------|----------|
| VM fails to provision | Azure API error | Retry up to N times, then mark failed |
| SSH connection fails | Connection timeout | Retry with exponential backoff |
| memtier not installed | Ready file missing | Wait for cloud-init (timeout 5m) |
| VM becomes unreachable | Heartbeat timeout | Mark failed, continue if within tolerance |
| Benchmark hangs | No process found after expected duration | Kill and collect partial results |
| Spot VM evicted | Azure eviction notice | Mark failed, adjust tolerance |

### Execution Policy Configuration

```go
type ExecutionPolicy struct {
    // How many VMs can fail and still consider the benchmark valid
    FailureTolerance int  // Default: 0 (all must succeed)
    
    // Continue provisioning if some VMs fail
    ContinueOnPartial bool  // Default: false
    
    // Maximum time for entire benchmark
    ExecutionTimeout time.Duration  // Default: 1 hour
    
    // Health check frequency
    HealthCheckInterval time.Duration  // Default: 5 seconds
    
    // Max time without heartbeat before marking VM failed
    HeartbeatTimeout time.Duration  // Default: 30 seconds
}
```

### Example: 3 VMs with 1 Failure Tolerance

```
Deployment: 3 VMs requested, FailureTolerance=1

VM1: ✓ Provisioned → ✓ Configured → ✓ Running → ✓ Completed
VM2: ✓ Provisioned → ✓ Configured → ✗ Lost connection (marked failed)
VM3: ✓ Provisioned → ✓ Configured → ✓ Running → ✓ Completed

Result: SUCCESS (2/3 VMs completed, 1 failed ≤ 1 tolerance)
Aggregated throughput from VM1 + VM3
```

## Health Monitoring

### Heartbeat Mechanism

During benchmark execution, the control plane periodically checks each VM:

```go
// Every 5 seconds:
for _, vm := range runningVMs {
    // Check if memtier process is still running
    output, err := ssh.Run(vm.IP, "pgrep -f memtier_benchmark || echo 'done'")
    
    if err != nil {
        vm.FailureCount++
        if vm.FailureCount >= 3 {
            vm.State = Failed
        }
    } else {
        vm.LastHeartbeat = time.Now()
        vm.FailureCount = 0
        
        if output == "done" {
            vm.State = Completed
            collectResults(vm)
        }
    }
}
```

### Progress Reporting

Real-time progress via event stream:

```
[2026-02-03 16:45:00] PROVISION  Creating resource group redismeter-rm-abc123
[2026-02-03 16:45:02] PROVISION  rm-abc123-runner-0: Starting VM provisioning
[2026-02-03 16:45:02] PROVISION  rm-abc123-runner-1: Starting VM provisioning
[2026-02-03 16:45:02] PROVISION  rm-abc123-runner-2: Starting VM provisioning
[2026-02-03 16:48:15] PROVISION  rm-abc123-runner-0: VM ready at 20.185.67.123
[2026-02-03 16:48:22] PROVISION  rm-abc123-runner-1: VM ready at 20.185.67.124
[2026-02-03 16:48:30] PROVISION  rm-abc123-runner-2: VM ready at 20.185.67.125
[2026-02-03 16:48:35] CONFIGURE  rm-abc123-runner-0: SSH connection established
[2026-02-03 16:48:36] CONFIGURE  rm-abc123-runner-1: SSH connection established
[2026-02-03 16:48:37] CONFIGURE  rm-abc123-runner-2: SSH connection established
[2026-02-03 16:50:00] CONFIGURE  rm-abc123-runner-0: VM fully configured and ready
[2026-02-03 16:50:02] CONFIGURE  rm-abc123-runner-1: VM fully configured and ready
[2026-02-03 16:50:04] CONFIGURE  rm-abc123-runner-2: VM fully configured and ready
[2026-02-03 16:50:05] SYNC       Preparing 3 runners for synchronized start
[2026-02-03 16:50:10] START      Starting synchronized benchmark at 2026-02-03 16:50:15
[2026-02-03 16:51:15] COMPLETE   rm-abc123-runner-0: Benchmark completed
[2026-02-03 16:51:15] COMPLETE   rm-abc123-runner-1: Benchmark completed
[2026-02-03 16:51:16] COMPLETE   rm-abc123-runner-2: Benchmark completed
[2026-02-03 16:51:16] COMPLETE   All benchmarks completed
```

## Resource Cleanup

### Automatic Cleanup

All Azure resources are created within a single resource group:
- Deleting the resource group deletes ALL resources
- No orphaned resources possible
- Simple and reliable

### TTL-Based Auto-Cleanup

Deployments have a TTL (default 2 hours). If not explicitly torn down:
1. A background job checks for expired deployments
2. Automatically deletes the resource group
3. Prevents runaway costs from forgotten infrastructure

### Resource Tagging

All resources are tagged for identification:
```json
{
    "managed-by": "redismeter",
    "deployment": "rm-abc123",
    "created-at": "2026-02-03T16:45:00Z",
    "role": "runner"
}
```

## Usage Examples

### Basic Usage

```bash
# Run on 2 Azure VMs
redismeter cloud run cache \
    --provider azure \
    --count 2 \
    --region eastus \
    --instance-type Standard_D4s_v3 \
    --target redis.example.com:6379

# Use spot instances (70% cheaper)
redismeter cloud run high-throughput \
    --provider azure \
    --count 3 \
    --spot \
    --target redis:6379
```

### With Failure Tolerance

```bash
# Allow 1 of 4 VMs to fail
redismeter cloud run cache \
    --provider azure \
    --count 4 \
    --failure-tolerance 1 \
    --target redis:6379
```

### Keep Infrastructure for Multiple Runs

```bash
# First run - keep infrastructure
redismeter cloud run cache \
    --provider azure \
    --count 3 \
    --keep \
    --target redis:6379

# Output: Infrastructure ID: rm-abc123

# Second run - reuse infrastructure
redismeter cloud run high-throughput \
    --infra rm-abc123 \
    --target redis:6379

# Cleanup when done
redismeter cloud teardown rm-abc123
```

## Cost Optimization

### Spot Instances

Azure Spot VMs offer up to 90% discount:
- Best for fault-tolerant workloads
- Use with `--spot` flag
- Combine with `--failure-tolerance` for reliability

### Right-Sizing

| Workload Type | Recommended Instance | Why |
|---------------|---------------------|-----|
| High throughput | F-series (F8s_v2) | Compute optimized, high network |
| General | D-series (D4s_v3) | Balanced |
| Memory-heavy | E-series (E4s_v3) | More RAM for large key sets |

### Example Cost Estimation

```bash
redismeter cloud estimate \
    --provider azure \
    --count 3 \
    --instance-type Standard_F8s_v2 \
    --duration 1h

# Output:
# Estimated cost for 3x Standard_F8s_v2 for 1h:
#   Regular: $1.01/hour = $1.01 total
#   Spot:    $0.30/hour = $0.30 total (70% savings)
```

## Security Considerations

1. **SSH Keys**: Use dedicated SSH keys for benchmark infrastructure
2. **Network Security**: NSG allows only SSH (port 22) inbound
3. **No Persistent Storage**: Results collected then VMs destroyed
4. **Resource Group Isolation**: Each deployment in its own resource group
5. **Managed Identity**: Use Azure managed identity for authentication when possible

## Future Enhancements

1. **Accelerated Networking**: Enable for higher network throughput
2. **Proximity Placement Groups**: Co-locate VMs for lower latency
3. **Pre-built VM Images**: Skip memtier compilation (save 2-3 minutes)
4. **Azure Container Instances**: Faster startup alternative to VMs
5. **ARM Templates**: Declarative infrastructure for reproducibility
