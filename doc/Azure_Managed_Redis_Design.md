# Azure Managed Redis (AMR) Provider Design

## Overview

This document describes the Azure Managed Redis (AMR) integration for RedisMeter cloud benchmarking. AMR is distinct from Azure Cache for Redis and uses the Redis Enterprise API (v2025-07-01).

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          RedisMeter Control Plane                           │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                        Azure Provider                                │   │
│  │  ┌─────────────┐  ┌─────────────────┐  ┌───────────────────────┐   │   │
│  │  │   Runner    │  │   AMR Manager   │  │   Network Manager     │   │   │
│  │  │  Provision  │  │  (Cluster+DB)   │  │  (VNet/PE/NSG)        │   │   │
│  │  └─────────────┘  └─────────────────┘  └───────────────────────┘   │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
        ┌───────────────────────────┼───────────────────────────┐
        │                           │                           │
        ▼                           ▼                           ▼
┌───────────────┐          ┌───────────────┐          ┌───────────────┐
│   Runner VMs  │          │  Private      │          │ Azure Managed │
│   (memtier)   │◄────────►│  Endpoint     │◄────────►│    Redis      │
│               │          │               │          │               │
└───────────────┘          └───────────────┘          └───────────────┘
        │                                                      │
        │              ┌───────────────┐                       │
        └─────────────►│     VNet      │◄──────────────────────┘
                       │  (Shared or   │
                       │   Peered)     │
                       └───────────────┘
```

## Three Target Modes

### 1. Provision Mode (`mode: provision`)

Creates a new AMR instance using a pre-defined template plus user-selected SKU.

**Design Philosophy:**
- **Templates** define *functional* characteristics (HA, persistence, modules)
- **SKU** defines *performance* tier - user chooses based on their needs
- This separation allows any template to run on any SKU

**When to use:**
- Fresh benchmarking against a new Redis instance
- Testing specific AMR configurations (HA, persistence)
- Reproducible benchmark environments

**Templates (Functional Patterns):**

| Template | HA | Persistence | Modules | Use Case |
|----------|-----|-------------|---------|----------|
| `dev-test` | ❌ | None | - | Development, testing, CI/CD |
| `standard` | ✅ | None | - | Production caching, ephemeral data |
| `durable` | ✅ | RDB (1h) | - | Important data, acceptable hourly loss |
| `high-durability` | ✅ | AOF (1s) | - | Critical data, minimal loss tolerance |
| `search` | ✅ | RDB (1h) | RediSearch, RedisJSON | Full-text/vector search workloads |

**SKU Families:**

| Family | Description | Best For |
|--------|-------------|----------|
| `Balanced_B*` | General purpose | Most workloads |
| `ComputeOptimized_X*` | High CPU-to-memory | High throughput |
| `MemoryOptimized_M*` | High memory-to-CPU | Large datasets |
| `FlashOptimized_A*` | NVMe flash + RAM | Cost-effective large data |

**Example:**
```yaml
redis_target:
  type: azure_managed_redis
  azure_managed_redis:
    mode: provision
    template: durable          # Functional: HA + RDB persistence
    sku: Balanced_B10          # Performance: User's choice
    capacity: 2                # Cluster capacity
    network:
      allow_public_access: false
      create_private_endpoint: true
```

### 2. Existing Mode (`mode: existing`)

Connects to an existing AMR instance by resource ID.

**When to use:**
- Benchmarking production or staging instances
- Comparing performance across environments
- Testing against pre-configured Redis setups

**Example:**
```yaml
redis_target:
  type: azure_managed_redis
  azure_managed_redis:
    mode: existing
    resource_id: /subscriptions/xxx/resourceGroups/my-rg/providers/Microsoft.Cache/redisEnterprise/my-amr
    network:
      use_existing_vnet: true
      existing_vnet_id: /subscriptions/xxx/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet
      existing_subnet_name: runners
      create_private_endpoint: true  # If AMR doesn't have public access
```

### 3. Endpoint Mode (`mode: endpoint`)

Direct connection to any Redis endpoint. User is responsible for network connectivity.

**When to use:**
- Testing against non-Azure Redis (Redis Enterprise Cloud, self-hosted)
- Complex networking scenarios where RedisMeter shouldn't manage connectivity
- Quick tests against known endpoints

**Example:**
```yaml
redis_target:
  type: azure_managed_redis
  azure_managed_redis:
    mode: endpoint
    endpoint: my-redis.eastus.redis.azure.net
    port: 10000
    password: ${REDIS_PASSWORD}
```

## Networking Deep Dive

### The Challenge

AMR typically uses private endpoints for security. Runner VMs must be able to reach AMR, which requires:

1. **Same VNet**: Runners and AMR private endpoint in same VNet
2. **VNet Peering**: Different VNets but peered
3. **Private Endpoint in Runner VNet**: Create PE in runner's VNet pointing to AMR
4. **Public Access**: If AMR allows public access (not recommended for production)

### Network Configuration Options

```
┌─────────────────────────────────────────────────────────────────┐
│ Scenario 1: New VNet (RedisMeter creates everything)           │
│                                                                 │
│  ┌─────────────────────────────────────────────────┐           │
│  │  New VNet (created by RedisMeter)               │           │
│  │  ┌─────────────┐       ┌─────────────────┐      │           │
│  │  │   Runner    │       │  AMR Private    │      │           │
│  │  │   Subnet    │◄─────►│  Endpoint       │      │           │
│  │  └─────────────┘       └─────────────────┘      │           │
│  └─────────────────────────────────────────────────┘           │
│                                    │                            │
│                                    ▼                            │
│                           ┌─────────────────┐                   │
│                           │      AMR        │                   │
│                           └─────────────────┘                   │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│ Scenario 2: Existing VNet (deploy into customer VNet)          │
│                                                                 │
│  ┌─────────────────────────────────────────────────┐           │
│  │  Existing VNet (customer-owned)                 │           │
│  │  ┌─────────────┐       ┌─────────────────┐      │           │
│  │  │   Runner    │       │  Existing AMR   │      │           │
│  │  │   Subnet    │◄─────►│  Private EP     │      │           │
│  │  │  (new VMs)  │       │  (or new PE)    │      │           │
│  │  └─────────────┘       └─────────────────┘      │           │
│  └─────────────────────────────────────────────────┘           │
│                                    │                            │
│                                    ▼                            │
│                           ┌─────────────────┐                   │
│                           │  Existing AMR   │                   │
│                           └─────────────────┘                   │
└─────────────────────────────────────────────────────────────────┘
```

### Network Configuration Fields

```go
type AzureManagedRedisNetworkSpec struct {
    // Deploy runners into existing VNet
    UseExistingVNet    bool   `json:"use_existing_vnet"`
    ExistingVNetID     string `json:"existing_vnet_id"`      // Full ARM resource ID
    ExistingSubnetName string `json:"existing_subnet_name"`  // Subnet for runner VMs

    // Private endpoint configuration
    CreatePrivateEndpoint     bool   `json:"create_private_endpoint"`
    PrivateEndpointSubnetName string `json:"private_endpoint_subnet_name"`

    // For new AMR instances only
    AllowPublicAccess bool `json:"allow_public_access"`
}
```

### Decision Matrix

| AMR Location | AMR Public Access | Runner VNet | Solution |
|-------------|-------------------|-------------|----------|
| New (provision) | Disabled | New | Create PE in runner VNet |
| New (provision) | Enabled | New | Direct connection (not recommended) |
| Existing | Disabled | Same VNet as AMR PE | Direct connection via PE |
| Existing | Disabled | Different VNet | Create new PE in runner VNet |
| Existing | Enabled | Any | Direct connection |

## AMR API Details

### Two-Level Hierarchy

AMR uses a cluster + database model:

```
Microsoft.Cache/redisEnterprise/{clusterName}           ← Cluster (SKU, HA, zones)
    └── databases/{databaseName}                        ← Database (persistence, modules, eviction)
```

### API Version

We use the latest API version: `2025-07-01`

This version includes:
- `publicNetworkAccess` property for network security
- New SKU names (Balanced_B*, ComputeOptimized_X*, MemoryOptimized_M*, FlashOptimized_A*)
- Enhanced persistence options

### Cluster Creation

```http
PUT https://management.azure.com/subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.Cache/redisEnterprise/{name}?api-version=2025-07-01

{
  "location": "eastus",
  "sku": {
    "name": "Balanced_B10",
    "capacity": 2
  },
  "zones": ["1", "2", "3"],
  "properties": {
    "highAvailability": "Enabled",
    "minimumTlsVersion": "1.2",
    "publicNetworkAccess": "Disabled"
  }
}
```

### Database Creation

```http
PUT .../redisEnterprise/{name}/databases/default?api-version=2025-07-01

{
  "properties": {
    "clientProtocol": "Encrypted",
    "port": 10000,
    "clusteringPolicy": "OSSCluster",
    "evictionPolicy": "VolatileLRU",
    "persistence": {
      "rdbEnabled": true,
      "rdbFrequency": "1h"
    },
    "accessKeysAuthentication": "Enabled"
  }
}
```

### Connection Details

- **Endpoint**: `{clusterName}.{region}.redis.azure.net`
- **Port**: 10000 (default)
- **TLS**: Always enabled (Encrypted protocol)
- **Authentication**: Access keys (retrieved via listKeys API)

## Integration with Azure Provider

The `AzureProvider` uses `AMRManager` for Redis target management:

```go
// In azure_provider.go Provision() method
if spec.RedisTarget != nil && spec.RedisTarget.AzureManagedRedis != nil {
    amrManager, err := NewAMRManager(p.config.SubscriptionID)
    if err != nil {
        return nil, err
    }
    
    target, cluster, err := amrManager.ResolveAMRTarget(
        ctx,
        spec.RedisTarget.AzureManagedRedis,
        deployment.ResourceGroup,
        deployment.Location,
        vnetID,  // Runner VNet for private endpoint
    )
    if err != nil {
        return nil, err
    }
    
    infra.RedisEndpoint = target.Host
    infra.RedisPort = target.Port
}
```

## Example Configurations

### Full Cloud Benchmark with New AMR

```yaml
# benchmark-config.yaml
name: amr-benchmark
provider: azure
region: eastus

load_generator:
  instance_type: Standard_D4s_v3
  count: 3
  spot_instances: true

redis_target:
  type: azure_managed_redis
  azure_managed_redis:
    mode: provision
    template: high-performance
    network:
      allow_public_access: false
      create_private_endpoint: true
      private_endpoint_subnet_name: default

workload: high-throughput
duration: 5m
```

### Benchmark Against Existing Production AMR

```yaml
# benchmark-config.yaml
name: production-benchmark
provider: azure
region: eastus

load_generator:
  instance_type: Standard_D4s_v3
  count: 5

redis_target:
  type: azure_managed_redis
  azure_managed_redis:
    mode: existing
    resource_id: /subscriptions/xxx/resourceGroups/prod-rg/providers/Microsoft.Cache/redisEnterprise/prod-redis
    network:
      use_existing_vnet: true
      existing_vnet_id: /subscriptions/xxx/resourceGroups/prod-rg/providers/Microsoft.Network/virtualNetworks/prod-vnet
      existing_subnet_name: benchmark-runners
      # AMR already has private endpoint, so we don't need to create one
      create_private_endpoint: false

workload: cache
duration: 10m
```

## Cleanup

When tearing down infrastructure:

1. **Provisioned AMR**: Deleted along with the resource group
2. **Existing AMR**: Only the private endpoint (if created) is deleted
3. **Endpoint Mode**: No cleanup needed

## Cost Considerations

| SKU | Approximate Monthly Cost* |
|-----|--------------------------|
| Balanced_B0 | ~$60 |
| Balanced_B5 | ~$200 |
| Balanced_B10 | ~$400 |
| ComputeOptimized_X10 | ~$600 |
| MemoryOptimized_M20 | ~$800 |

*Prices vary by region and are subject to change. Check Azure pricing for current rates.

## Limitations

1. **AMR Provisioning Time**: Creating a new AMR instance takes 10-20 minutes
2. **Private Endpoint DNS**: May require private DNS zone configuration for resolution
3. **SKU Changes**: Cannot scale down certain configurations without recreation
4. **Persistence + Geo-Replication**: Cannot use both simultaneously
5. **Flash Optimized**: In preview, availability varies by region
