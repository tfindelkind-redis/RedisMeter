# Azure Deployment Design

## Overview

This document describes how RedisMeter deploys and runs benchmarks on Azure infrastructure.

## Deployment Model

**RedisMeter runs locally** on the user's machine as a CLI tool. It acts as the **control plane** that:
1. Provisions Azure infrastructure (VMs, networking, optionally AMR)
2. Connects to runner VMs via SSH
3. Executes memtier_benchmark on runners
4. Collects and aggregates results
5. Cleans up infrastructure (optional)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    Local Machine (Control Plane)                            │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                         RedisMeter CLI                                │  │
│  │                                                                       │  │
│  │  Responsibilities:                                                    │  │
│  │  • Azure authentication (az login / Service Principal)                │  │
│  │  • Resource provisioning via Azure SDK                                │  │
│  │  • SSH key generation and management                                  │  │
│  │  • Remote execution orchestration                                     │  │
│  │  • Result collection and aggregation                                  │  │
│  │  • Resource cleanup                                                   │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
                                     │
                     ┌───────────────┴───────────────┐
                     │                               │
            HTTPS (Azure API)                   SSH (Port 22)
                     │                               │
                     ▼                               ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Azure Subscription                                  │
│                                                                             │
│   Resources Created:                      Runner VMs:                       │
│   • Resource Group                        • Ubuntu 22.04 LTS                │
│   • Virtual Network                       • memtier_benchmark installed     │
│   • Subnets (runners, redis)              • Executes benchmarks             │
│   • Network Security Group                • Streams results back            │
│   • Public IPs (for SSH access)                                             │
│   • Private Endpoint (for AMR)                                              │
│   • Azure Managed Redis (optional)                                          │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Authentication & RBAC

### Authentication Methods (Priority Order)

RedisMeter supports multiple authentication methods, tried in order:

```go
// Authentication chain
1. Environment Variables (Service Principal)
   - AZURE_TENANT_ID
   - AZURE_CLIENT_ID  
   - AZURE_CLIENT_SECRET

2. Azure CLI (az login)
   - Uses cached credentials from `az login`
   - Interactive browser auth if needed

3. Managed Identity (when running on Azure)
   - System-assigned or user-assigned
   - Useful for CI/CD on Azure DevOps

4. Workload Identity Federation
   - GitHub Actions OIDC
   - No secrets stored
```

### Required RBAC Permissions

**Minimum Required Role:** Custom role or combination of built-in roles

```json
{
  "Name": "RedisMeter Benchmark Operator",
  "Description": "Allows RedisMeter to provision benchmark infrastructure",
  "Actions": [
    // Resource Group
    "Microsoft.Resources/subscriptions/resourceGroups/read",
    "Microsoft.Resources/subscriptions/resourceGroups/write",
    "Microsoft.Resources/subscriptions/resourceGroups/delete",
    
    // Virtual Machines
    "Microsoft.Compute/virtualMachines/*",
    "Microsoft.Compute/sshPublicKeys/*",
    
    // Networking
    "Microsoft.Network/virtualNetworks/*",
    "Microsoft.Network/networkSecurityGroups/*",
    "Microsoft.Network/publicIPAddresses/*",
    "Microsoft.Network/networkInterfaces/*",
    "Microsoft.Network/privateEndpoints/*",
    "Microsoft.Network/privateDnsZones/*",
    
    // Azure Managed Redis (if provisioning)
    "Microsoft.Cache/redisEnterprise/*",
    
    // Monitoring (optional)
    "Microsoft.Insights/metrics/read"
  ],
  "AssignableScopes": [
    "/subscriptions/{subscription-id}"
  ]
}
```

**Built-in Role Alternatives:**
- `Contributor` on Resource Group (broad but simple)
- `Virtual Machine Contributor` + `Network Contributor` + `Redis Enterprise Contributor`

### Service Principal Setup

```bash
# Create Service Principal with required permissions
az ad sp create-for-rbac \
  --name "RedisMeter-Benchmark" \
  --role "Contributor" \
  --scopes "/subscriptions/{subscription-id}/resourceGroups/{resource-group}"

# Output (save these securely):
{
  "appId": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",      # AZURE_CLIENT_ID
  "password": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",       # AZURE_CLIENT_SECRET
  "tenant": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"     # AZURE_TENANT_ID
}

# Set environment variables
export AZURE_TENANT_ID="..."
export AZURE_CLIENT_ID="..."
export AZURE_CLIENT_SECRET="..."
export AZURE_SUBSCRIPTION_ID="..."
```

## Interactive CLI Flow

### Command Structure

```bash
# Interactive mode (recommended for first-time users)
redismeter cloud run --provider azure --interactive

# Non-interactive with config file
redismeter cloud run --provider azure --config benchmark-config.yaml

# Non-interactive with flags
redismeter cloud run --provider azure \
  --subscription $SUB_ID \
  --region eastus \
  --resource-group my-benchmark-rg \
  --runners 3 \
  --runner-size Standard_D4s_v3 \
  --redis-mode provision \
  --redis-template durable \
  --redis-sku Balanced_B10 \
  --workload high-throughput \
  --duration 5m
```

### Interactive Menu Flow

```
╔══════════════════════════════════════════════════════════════════════════════╗
║                    RedisMeter Azure Benchmark Setup                          ║
╚══════════════════════════════════════════════════════════════════════════════╝

Step 1 of 8: Authentication
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  ✓ Authenticated via Azure CLI
  ✓ User: thomas@example.com
  ✓ Tenant: My Company (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)

  Press Enter to continue...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Step 2 of 8: Subscription Selection
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  Available subscriptions:
  
  [1] Production (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)
  [2] Development (yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy)  
  [3] Testing (zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz)

  Select subscription [1-3]: █

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Step 3 of 8: Region Selection  
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  Select Azure region:
  
  Americas:
    [1] eastus          East US (Virginia)
    [2] eastus2         East US 2 (Virginia)
    [3] westus2         West US 2 (Washington)
    [4] westus3         West US 3 (Arizona)
    [5] centralus       Central US (Iowa)
  
  Europe:
    [6] northeurope     North Europe (Ireland)
    [7] westeurope      West Europe (Netherlands)
    [8] uksouth         UK South (London)
    [9] germanywestcentral  Germany West Central (Frankfurt)
  
  Asia Pacific:
    [10] southeastasia  Southeast Asia (Singapore)
    [11] australiaeast  Australia East (Sydney)
  
  [Enter region name or number]: █

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Step 4 of 8: Resource Group
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  Resource Group Options:
  
  [1] Create new resource group
  [2] Use existing resource group
  
  Select option [1-2]: 1

  Enter resource group name [rm-benchmark-20260203]: █
  
  ℹ️  Resource group will be created in: eastus
  ℹ️  All resources will be tagged with: managed-by=redismeter

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Step 5 of 8: Network Configuration
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  Network Options:
  
  [1] Create new VNet (recommended for isolated benchmarks)
  [2] Use existing VNet (required if connecting to existing AMR)
  
  Select option [1-2]: 1

  New VNet Configuration:
  ┌────────────────────────────────────────────────────────────────────────────┐
  │  VNet Name:        rm-vnet                                                 │
  │  Address Space:    10.0.0.0/16                                             │
  │  Runner Subnet:    10.0.1.0/24 (runners)                                   │
  │  Redis Subnet:     10.0.2.0/24 (private-endpoints)                         │
  └────────────────────────────────────────────────────────────────────────────┘
  
  Accept defaults? [Y/n]: █

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Step 6 of 8: Runner Configuration
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  How many runner VMs? [1-10]: 3

  Select runner VM size:
  
  Cost-Optimized:
    [1] Standard_B2s      2 vCPU,  4 GB RAM   ~$0.04/hr   (dev/test)
    [2] Standard_D2s_v3   2 vCPU,  8 GB RAM   ~$0.10/hr   (light load)
  
  Balanced:
    [3] Standard_D4s_v3   4 vCPU,  16 GB RAM  ~$0.19/hr   (recommended)
    [4] Standard_D8s_v3   8 vCPU,  32 GB RAM  ~$0.38/hr   (high load)
  
  High Performance:
    [5] Standard_D16s_v3  16 vCPU, 64 GB RAM  ~$0.77/hr   (max throughput)
    [6] Standard_F8s_v2   8 vCPU,  16 GB RAM  ~$0.34/hr   (CPU optimized)
  
  Select size [1-6]: 3

  Runner Summary:
  ┌────────────────────────────────────────────────────────────────────────────┐
  │  Count:        3 VMs                                                       │
  │  Size:         Standard_D4s_v3 (4 vCPU, 16 GB RAM)                         │
  │  OS:           Ubuntu 22.04 LTS                                            │
  │  Software:     memtier_benchmark (auto-installed)                          │
  │  Est. Cost:    ~$0.57/hr ($0.19 × 3)                                       │
  └────────────────────────────────────────────────────────────────────────────┘

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Step 7 of 8: Redis Target Configuration
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  Redis Target Mode:
  
  [1] Provision new Azure Managed Redis
      → Creates a new AMR instance for benchmarking
      → Best for: Fresh benchmarks, reproducible tests
  
  [2] Connect to existing Azure Managed Redis  
      → Uses an existing AMR by resource ID
      → Best for: Production/staging benchmarks
  
  [3] Direct endpoint connection
      → Connect to any Redis endpoint
      → Best for: Non-Azure Redis, complex networking
  
  Select mode [1-3]: 1

  ─────────────────────────────────────────────────────────────────────────────

  Select AMR Template (functional characteristics):
  
  [1] dev-test         No HA, no persistence (cheapest)
  [2] standard         HA enabled, no persistence
  [3] durable          HA + RDB persistence (hourly snapshots)
  [4] high-durability  HA + AOF persistence (1s writes)
  [5] search           HA + RDB + RediSearch module
  
  Select template [1-5]: 3

  ─────────────────────────────────────────────────────────────────────────────

  Select AMR SKU (performance tier):
  
  Balanced (General Purpose):
    [1] Balanced_B0      3 GB,  1 vCPU    ~$0.10/hr   (dev/test)
    [2] Balanced_B5      24 GB, 2 vCPU   ~$0.50/hr   (small prod)
    [3] Balanced_B10     48 GB, 4 vCPU   ~$1.00/hr   (medium prod)
    [4] Balanced_B20     96 GB, 8 vCPU   ~$2.00/hr   (large prod)
  
  Compute Optimized (High Throughput):
    [5] ComputeOptimized_X10  24 GB, 8 vCPU   ~$1.50/hr
    [6] ComputeOptimized_X20  48 GB, 16 vCPU  ~$3.00/hr
  
  Memory Optimized (Large Datasets):
    [7] MemoryOptimized_M20   64 GB, 4 vCPU   ~$2.50/hr
    [8] MemoryOptimized_M50  128 GB, 8 vCPU   ~$5.00/hr
  
  [Enter SKU name or number]: 3

  AMR Summary:
  ┌────────────────────────────────────────────────────────────────────────────┐
  │  Template:     durable (HA + RDB persistence)                              │
  │  SKU:          Balanced_B10 (48 GB, 4 vCPU)                                │
  │  Network:      Private endpoint (no public access)                         │
  │  Est. Cost:    ~$1.00/hr                                                   │
  │  Provision:    ~10-15 minutes                                              │
  └────────────────────────────────────────────────────────────────────────────┘

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Step 8 of 8: Workload & Execution
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  Select benchmark workload:
  
  [1] cache            80% GET, 20% SET (typical cache)
  [2] write-heavy      20% GET, 80% SET (write intensive)  
  [3] read-only        100% GET (read replicas)
  [4] high-throughput  Pipelined, max ops/sec
  [5] low-latency      Minimal clients, no pipelining
  [6] session          Session store pattern
  [7] Custom workload file...
  
  Select workload [1-7]: 4

  Benchmark duration [5m]: █
  
  Cleanup after benchmark?
  [1] Yes - Delete all resources when done (recommended)
  [2] No  - Keep resources for further testing
  
  Select [1-2]: 1

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Review Configuration
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  ┌────────────────────────────────────────────────────────────────────────────┐
  │  AZURE CONFIGURATION                                                       │
  │  ─────────────────────────────────────────────────────────────────────────│
  │  Subscription:    Development (yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy)       │
  │  Region:          eastus                                                   │
  │  Resource Group:  rm-benchmark-20260203 (new)                              │
  │  VNet:            rm-vnet (10.0.0.0/16) (new)                              │
  │                                                                            │
  │  RUNNERS                                                                   │
  │  ─────────────────────────────────────────────────────────────────────────│
  │  Count:           3 VMs                                                    │
  │  Size:            Standard_D4s_v3 (4 vCPU, 16 GB)                          │
  │  Cost:            ~$0.57/hr                                                │
  │                                                                            │
  │  REDIS TARGET                                                              │
  │  ─────────────────────────────────────────────────────────────────────────│
  │  Mode:            Provision new AMR                                        │
  │  Template:        durable (HA + RDB)                                       │
  │  SKU:             Balanced_B10 (48 GB, 4 vCPU)                             │
  │  Cost:            ~$1.00/hr                                                │
  │                                                                            │
  │  BENCHMARK                                                                 │
  │  ─────────────────────────────────────────────────────────────────────────│
  │  Workload:        high-throughput                                          │
  │  Duration:        5m                                                       │
  │  Cleanup:         Yes (auto-delete after)                                  │
  │                                                                            │
  │  ESTIMATED TOTAL COST                                                      │
  │  ─────────────────────────────────────────────────────────────────────────│
  │  Infrastructure:  ~$1.57/hr                                                │
  │  Provision time:  ~15 min                                                  │
  │  Benchmark time:  ~5 min                                                   │
  │  Total estimate:  ~$0.52 (for this run)                                    │
  └────────────────────────────────────────────────────────────────────────────┘

  Proceed with deployment? [Y/n]: █

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Deploying...
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  [1/7] Creating resource group...                    ✓ Done (2s)
  [2/7] Creating virtual network...                   ✓ Done (5s)
  [3/7] Creating network security group...            ✓ Done (3s)
  [4/7] Provisioning Azure Managed Redis...           ⠋ In progress (8m 23s)
        └─ Cluster status: Creating...
  [5/7] Provisioning runner VMs...                    ○ Pending
  [6/7] Installing memtier on runners...              ○ Pending
  [7/7] Creating private endpoint...                  ○ Pending

  ⏱️  Elapsed: 8m 28s
  💡 Tip: AMR provisioning takes 10-15 minutes

```

## Provisioning Workflow

### Resource Creation Order

```
1. Resource Group
   └── Tags: managed-by=redismeter, created-at=timestamp

2. Virtual Network
   ├── Address space: 10.0.0.0/16
   ├── Subnet: runners (10.0.1.0/24)
   └── Subnet: private-endpoints (10.0.2.0/24)

3. Network Security Group
   ├── Inbound: SSH (22) from user's IP
   ├── Inbound: Allow VNet internal
   └── Outbound: Allow all

4. SSH Key Pair
   └── Generated and stored locally (~/.redismeter/keys/)

5. Azure Managed Redis (if mode=provision)
   ├── Cluster creation (~10-15 min)
   ├── Database creation (~2-3 min)
   └── Private endpoint creation

6. Runner VMs (parallel)
   ├── Public IP (for SSH access)
   ├── Network interface
   ├── VM creation
   └── Cloud-init: install memtier

7. Private Endpoint (connects VNet to AMR)
   └── Private DNS zone for name resolution
```

### SSH Access Strategy

```
┌─────────────────┐         ┌─────────────────┐
│  User Machine   │   SSH   │   Runner VM     │
│                 │────────▶│   (Public IP)   │
│  ~/.redismeter/ │         │                 │
│  └── keys/      │         │  memtier runs   │
│      └── id_rsa │         │  here           │
└─────────────────┘         └─────────────────┘

Security:
• SSH key generated per-run (or reuse existing)
• NSG limits SSH to user's public IP
• Keys stored in ~/.redismeter/keys/
• Public IPs can be removed after setup (use Bastion)
```

## Configuration File Format

For non-interactive or repeatable runs:

```yaml
# benchmark-config.yaml
provider: azure

azure:
  subscription_id: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
  region: eastus
  
  resource_group:
    name: rm-benchmark-prod
    create: true  # false to use existing
  
  network:
    create_new: true
    vnet_name: rm-vnet
    address_space: "10.0.0.0/16"
    runner_subnet: "10.0.1.0/24"
    redis_subnet: "10.0.2.0/24"
    # For existing VNet:
    # existing_vnet_id: "/subscriptions/.../virtualNetworks/my-vnet"
    # existing_subnet_name: "runners"

runners:
  count: 3
  size: Standard_D4s_v3
  os: Ubuntu2204
  # Advanced options:
  # spot_instances: true
  # availability_zones: ["1", "2", "3"]

redis_target:
  mode: provision  # provision | existing | endpoint
  
  # For mode: provision
  template: durable
  sku: Balanced_B10
  capacity: 2
  
  # For mode: existing
  # resource_id: "/subscriptions/.../redisEnterprise/my-amr"
  
  # For mode: endpoint
  # endpoint: "my-redis.eastus.redis.azure.net"
  # port: 10000
  # password: "${REDIS_PASSWORD}"

benchmark:
  workload: high-throughput
  duration: 5m
  # Override workload parameters:
  # clients: 100
  # threads: 4
  # pipeline: 10

cleanup:
  auto: true  # Delete resources after benchmark
  ttl: 2h    # Safety timeout: delete even if benchmark fails
```

## Error Handling & Recovery

### Common Failure Scenarios

| Scenario | Detection | Recovery |
|----------|-----------|----------|
| Auth failure | API 401/403 | Prompt for re-auth |
| Quota exceeded | API 409 | Suggest different region/size |
| AMR provision timeout | 20min timeout | Offer retry or manual cleanup |
| VM SSH unreachable | Connection timeout | Retry with backoff |
| Benchmark failure | Non-zero exit | Collect logs, report error |
| User interrupt (Ctrl+C) | SIGINT | Graceful cleanup prompt |

### Cleanup on Failure

```
Deployment failed at step 5 (Runner VMs)

Error: QuotaExceeded - Not enough cores available in region eastus

Options:
  [1] Retry in different region
  [2] Retry with smaller VM size
  [3] Clean up created resources and exit
  [4] Keep resources for debugging

Select [1-4]: █

Note: Resources created so far:
  • Resource group: rm-benchmark-20260203
  • Virtual network: rm-vnet  
  • AMR cluster: rm-amr-1738590000 (still provisioning)

⚠️  Warning: AMR costs ~$1/hr even when not in use
```

## Cost Management

### Cost Estimation

Before deployment, show estimated costs:

```go
type CostEstimate struct {
    Runners     CostItem  // Per-runner × count
    AMR         CostItem  // Based on SKU
    Networking  CostItem  // Usually minimal
    Storage     CostItem  // OS disks
    
    HourlyCost  float64
    TotalEstimate float64  // For expected duration
}
```

### Auto-Cleanup (TTL)

Resources are tagged with TTL for safety:

```go
tags := map[string]string{
    "managed-by":   "redismeter",
    "created-at":   time.Now().Format(time.RFC3339),
    "ttl":          "2h",
    "auto-cleanup": "true",
}
```

A background process or Azure Automation can clean up orphaned resources.

## Security Considerations

### Network Security

1. **Runner VMs**: Public IP only for SSH, restricted to user's IP via NSG
2. **AMR**: Private endpoint only, no public access
3. **Communication**: All Redis traffic stays within VNet

### Credential Management

1. **Azure Auth**: Use `az login` or Service Principal (env vars)
2. **SSH Keys**: Generated per-run, stored in `~/.redismeter/keys/`
3. **Redis Password**: Retrieved via Azure API, never stored locally
4. **No secrets in config files**: Use environment variables or Azure Key Vault

### Audit Trail

All operations are logged:
```
~/.redismeter/logs/
├── azure-20260203-143022.log    # API calls
├── ssh-20260203-143022.log      # SSH sessions
└── benchmark-20260203-143022.log # Benchmark output
```
