# Azure Deployment Methods Analysis

## Current Implementation (Updated)

### VMs (Runner VMs)
- **Method**: Azure SDK for Go (`armcompute`, `armnetwork`)
- **Status**: ✅ Good - SDK handles retries, auth, and error handling

### Azure Managed Redis (AMR)
- **Method**: **Bicep templates via ARM deployment** (new)
- **Legacy Method**: Raw HTTP calls to ARM REST API (still available)
- **Status**: ✅ **Improved** - Using production-tested Bicep from workshop

## Implementation Files

### Bicep Templates (`internal/cloud/bicep/`)
- `redis-enterprise.bicep` - AMR cluster and database creation
- `private-endpoint.bicep` - Private endpoint with DNS zone group
- `private-dns-zone.bicep` - Private DNS zone with VNet link
- `amr-deployment.bicep` - Main deployment orchestrator
- `amr-deployment.json` - Compiled ARM template (embedded in Go binary)

### Go Implementation
- `azure_amr_bicep.go` - New Bicep-based deployer using ARM SDK
- `azure_managed_redis.go` - Original HTTP-based implementation (kept as fallback)

## Why AMR Deployment is Tricky

Based on real-world experience deploying AMR, here are the common issues:

### 1. Two-Phase Creation (Cluster + Database)
```
AMR Cluster (Microsoft.Cache/redisEnterprise)
    └── Database (Microsoft.Cache/redisEnterprise/databases)
```
- Cluster takes 10-15 minutes to provision
- Database can only be created AFTER cluster is `Succeeded`
- Database takes another 2-5 minutes

### 2. SKU + Capacity Validation Rules
```
Balanced/Compute/Memory SKUs:
  - Capacity must be even: 2, 4, 6, 8...
  - Minimum capacity: 2

Flash Optimized SKUs:
  - Capacity must be multiple of 3: 3, 9, 15...
  - Minimum capacity: 3

HA + Zones:
  - If HighAvailability=Enabled, Zones must have 3 values
  - Zones must match region availability
```

### 3. Persistence Configuration Gotchas
```json
// RDB persistence - frequency must be EXACT string
"persistence": {
    "rdbEnabled": true,
    "rdbFrequency": "1h"  // Must be "1h", "6h", or "12h" - not "1 hour"!
}

// AOF persistence - frequency must be EXACT string  
"persistence": {
    "aofEnabled": true,
    "aofFrequency": "1s"  // Must be "1s" or "always"
}

// Cannot enable BOTH RDB and AOF
```

### 4. Module Installation Timing
- Modules (RediSearch, RedisJSON) can only be specified at database creation
- Cannot add modules after database exists
- Module names are case-sensitive: "RediSearch" not "redisearch"

### 5. Networking Complexity
```
┌─────────────────────────────────────────────────────────────────┐
│ AMR Cluster                                                     │
│   publicNetworkAccess: Disabled                                 │
│                                                                 │
│   How to connect?                                               │
│   1. Private Endpoint in SAME VNet as cluster                   │
│   2. Private Endpoint in DIFFERENT VNet + VNet Peering          │
│   3. Private Endpoint + Private DNS Zone for name resolution    │
└─────────────────────────────────────────────────────────────────┘

Private Endpoint creation requires:
1. Microsoft.Network/privateEndpoints
2. Microsoft.Network/privateDnsZones (redis.cache.azure.net)
3. Microsoft.Network/privateDnsZones/virtualNetworkLinks
4. Microsoft.Network/privateEndpoints/privateDnsZoneGroups
```

### 6. Access Keys Retrieval
- Keys endpoint: `POST .../listKeys` (not GET!)
- Returns `primaryKey` and `secondaryKey`
- Database must be fully provisioned before keys work

### 7. Connection String Format
```
# AMR uses port 10000, NOT 6379!
redis://:password@hostname.region.redis.azure.net:10000

# TLS is ALWAYS required (Encrypted protocol)
# Must use --tls flag with memtier
```

### 8. Resource Provider Registration
- `Microsoft.Cache` provider must be registered on subscription
- First deployment in a subscription may fail if not registered

### 9. Eventual Consistency Issues
- Cluster shows "Succeeded" but database creation fails
- Need to wait additional time after cluster "Succeeded"
- Private endpoint shows "Succeeded" but DNS not propagated

---

## Recommended Approach: Hybrid SDK + Bicep

### Option A: Bicep Templates (Recommended for AMR)

**Why Bicep?**
1. Declarative - Azure handles the deployment orchestration
2. Idempotent - Safe to re-run
3. Validated - Azure validates before deployment
4. Handles dependencies automatically
5. Better error messages
6. Tracks deployment state

```bicep
// amr.bicep
@description('Name of the AMR cluster')
param clusterName string

@description('Location')
param location string = resourceGroup().location

@description('SKU name')
@allowed([
  'Balanced_B0'
  'Balanced_B5'
  'Balanced_B10'
  'ComputeOptimized_X10'
  'MemoryOptimized_M20'
])
param skuName string = 'Balanced_B5'

@description('Cluster capacity')
param capacity int = 2

@description('Enable High Availability')
param highAvailability bool = true

@description('Persistence type: none, rdb, aof')
@allowed(['none', 'rdb', 'aof'])
param persistenceType string = 'none'

@description('VNet ID for private endpoint')
param vnetId string = ''

@description('Subnet name for private endpoint')
param subnetName string = 'default'

// AMR Cluster
resource amrCluster 'Microsoft.Cache/redisEnterprise@2024-10-01' = {
  name: clusterName
  location: location
  sku: {
    name: skuName
    capacity: capacity
  }
  zones: highAvailability ? ['1', '2', '3'] : []
  properties: {
    highAvailability: highAvailability ? 'Enabled' : 'Disabled'
    minimumTlsVersion: '1.2'
    publicNetworkAccess: empty(vnetId) ? 'Enabled' : 'Disabled'
  }
  tags: {
    'managed-by': 'redismeter'
  }
}

// AMR Database
resource amrDatabase 'Microsoft.Cache/redisEnterprise/databases@2024-10-01' = {
  parent: amrCluster
  name: 'default'
  properties: {
    clientProtocol: 'Encrypted'
    port: 10000
    clusteringPolicy: 'OSSCluster'
    evictionPolicy: 'VolatileLRU'
    persistence: persistenceType == 'rdb' ? {
      rdbEnabled: true
      rdbFrequency: '1h'
    } : persistenceType == 'aof' ? {
      aofEnabled: true
      aofFrequency: '1s'
    } : {}
  }
}

// Private Endpoint (if VNet provided)
resource privateEndpoint 'Microsoft.Network/privateEndpoints@2023-09-01' = if (!empty(vnetId)) {
  name: '${clusterName}-pe'
  location: location
  properties: {
    subnet: {
      id: '${vnetId}/subnets/${subnetName}'
    }
    privateLinkServiceConnections: [
      {
        name: '${clusterName}-connection'
        properties: {
          privateLinkServiceId: amrCluster.id
          groupIds: ['redisEnterprise']
        }
      }
    ]
  }
}

// Private DNS Zone
resource privateDnsZone 'Microsoft.Network/privateDnsZones@2020-06-01' = if (!empty(vnetId)) {
  name: 'privatelink.redisenterprise.cache.azure.net'
  location: 'global'
}

// DNS Zone Link to VNet
resource dnsZoneLink 'Microsoft.Network/privateDnsZones/virtualNetworkLinks@2020-06-01' = if (!empty(vnetId)) {
  parent: privateDnsZone
  name: '${clusterName}-link'
  location: 'global'
  properties: {
    virtualNetwork: {
      id: vnetId
    }
    registrationEnabled: false
  }
}

// DNS Zone Group for Private Endpoint
resource dnsZoneGroup 'Microsoft.Network/privateEndpoints/privateDnsZoneGroups@2023-09-01' = if (!empty(vnetId)) {
  parent: privateEndpoint
  name: 'default'
  properties: {
    privateDnsZoneConfigs: [
      {
        name: 'config1'
        properties: {
          privateDnsZoneId: privateDnsZone.id
        }
      }
    ]
  }
}

// Outputs
output clusterHostname string = amrCluster.properties.hostName
output databasePort int = amrDatabase.properties.port
output clusterId string = amrCluster.id
```

### Option B: Azure SDK with Proper Orchestration

If sticking with SDK, need proper state machine:

```go
// AMR Deployment State Machine
type AMRDeploymentState string

const (
    AMRStateValidating          AMRDeploymentState = "validating"
    AMRStateCheckingProvider    AMRDeploymentState = "checking_provider"
    AMRStateCreatingCluster     AMRDeploymentState = "creating_cluster"
    AMRStateWaitingCluster      AMRDeploymentState = "waiting_cluster"
    AMRStateClusterStabilizing  AMRDeploymentState = "stabilizing"  // Extra wait!
    AMRStateCreatingDatabase    AMRDeploymentState = "creating_database"
    AMRStateWaitingDatabase     AMRDeploymentState = "waiting_database"
    AMRStateCreatingEndpoint    AMRDeploymentState = "creating_endpoint"
    AMRStateCreatingDNS         AMRDeploymentState = "creating_dns"
    AMRStateWaitingDNS          AMRDeploymentState = "waiting_dns"
    AMRStateRetrievingKeys      AMRDeploymentState = "retrieving_keys"
    AMRStateValidatingConnection AMRDeploymentState = "validating_connection"
    AMRStateReady               AMRDeploymentState = "ready"
    AMRStateFailed              AMRDeploymentState = "failed"
)

// Key insight: Add stabilization delay after cluster "Succeeded"
func (m *AMRManager) waitForClusterStabilization(ctx context.Context) error {
    // Even after cluster shows "Succeeded", wait 30-60 seconds
    // before creating database to avoid race conditions
    time.Sleep(30 * time.Second)
    return nil
}
```

---

## Recommendation for RedisMeter

### Use Bicep for AMR, SDK for VMs

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                           RedisMeter Provisioning                            │
│                                                                              │
│  ┌─────────────────────────┐    ┌──────────────────────────────────────────┐│
│  │     Runner VMs          │    │     Azure Managed Redis                  ││
│  │     ───────────         │    │     ────────────────────                 ││
│  │                         │    │                                          ││
│  │  Method: Azure SDK      │    │  Method: Bicep Deployment                ││
│  │  - armcompute           │    │  - Embedded bicep template               ││
│  │  - armnetwork           │    │  - az deployment create                  ││
│  │                         │    │  - OR armresources.DeploymentClient      ││
│  │  Why: Simple, fast      │    │                                          ││
│  │  VMs are straightforward│    │  Why: AMR is complex, Bicep handles:     ││
│  │                         │    │  - Two-phase creation                    ││
│  └─────────────────────────┘    │  - Dependency ordering                   ││
│                                 │  - Validation                            ││
│                                 │  - Private endpoint + DNS                ││
│                                 │  - Idempotent retries                    ││
│                                 └──────────────────────────────────────────┘│
└──────────────────────────────────────────────────────────────────────────────┘
```

### Implementation Plan

1. **Embed Bicep templates** in the Go binary
2. **Deploy via Azure SDK DeploymentClient** (not CLI dependency)
3. **Poll deployment status** until complete
4. **Extract outputs** (hostname, port, cluster ID)
5. **Retrieve access keys** via separate API call
6. **Validate connectivity** before returning

```go
// Using Azure SDK to deploy Bicep (no az CLI needed)
func (m *AMRManager) deployWithBicep(ctx context.Context, rg, name string, params map[string]interface{}) error {
    deploymentsClient, err := armresources.NewDeploymentsClient(m.subscriptionID, m.credential, nil)
    if err != nil {
        return err
    }

    // Bicep compiles to ARM JSON - embed pre-compiled template
    template := getEmbeddedAMRTemplate()
    
    poller, err := deploymentsClient.BeginCreateOrUpdate(ctx, rg, name+"-deployment",
        armresources.Deployment{
            Properties: &armresources.DeploymentProperties{
                Template:   template,
                Parameters: params,
                Mode:       to.Ptr(armresources.DeploymentModeIncremental),
            },
        }, nil)
    
    if err != nil {
        return err
    }
    
    // Wait for deployment (handles all the orchestration)
    result, err := poller.PollUntilDone(ctx, nil)
    if err != nil {
        return err
    }
    
    // Extract outputs
    outputs := result.Properties.Outputs
    // ...
}
```

---

## Error Handling Checklist for AMR

1. **Provider not registered**: Register Microsoft.Cache before deployment
2. **SKU not available in region**: Check availability first
3. **Capacity validation**: Validate before deployment
4. **Quota exceeded**: Check quota, suggest smaller SKU
5. **VNet/Subnet not found**: Validate network resources exist
6. **Private endpoint limit**: Check subscription limits
7. **DNS propagation**: Wait and verify DNS resolution
8. **TLS certificate issues**: Ensure proper TLS configuration

---

## Summary

| Resource | Current Method | Recommended Method | Reason |
|----------|---------------|-------------------|---------|
| Resource Group | SDK | SDK | Simple |
| VNet/Subnet | SDK | SDK | Simple |
| NSG | SDK | SDK | Simple |
| Public IP | SDK | SDK | Simple |
| Runner VMs | SDK | SDK | Simple, parallel creation works |
| **AMR Cluster** | Raw HTTP | **Bicep** | Complex orchestration |
| **AMR Database** | Raw HTTP | **Bicep** | Depends on cluster state |
| **Private Endpoint** | SDK | **Bicep** | Part of AMR deployment |
| **Private DNS** | Not implemented | **Bicep** | Required for private endpoint |

The key insight is: **VMs are "fire and forget" but AMR is a complex orchestrated deployment**. Bicep handles that complexity much better than imperative SDK calls.
