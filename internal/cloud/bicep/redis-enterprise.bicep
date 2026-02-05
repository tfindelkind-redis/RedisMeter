// Azure Managed Redis (v2 with RediSearch support)
// Based on: https://github.com/Redislabs-Solution-Architects/workshop-transaction-processing-amr
// Adapted for RedisMeter benchmarking
// IMPORTANT: RediSearch requires Enterprise clustering policy

@description('Name of the Redis Enterprise cluster')
param name string

@description('Azure region for the resource')
param location string

@description('SKU name - supports both new Balanced_B* and legacy Enterprise_E* formats')
@allowed([
  // New Azure Managed Redis SKUs (v2) - Balanced
  'Balanced_B0'
  'Balanced_B1'
  'Balanced_B3'
  'Balanced_B5'
  'Balanced_B10'
  'Balanced_B20'
  'Balanced_B50'
  'Balanced_B100'
  'Balanced_B150'
  'Balanced_B250'
  'Balanced_B350'
  'Balanced_B500'
  'Balanced_B700'
  'Balanced_B1000'
  // Memory Optimized SKUs
  'MemoryOptimized_M10'
  'MemoryOptimized_M20'
  'MemoryOptimized_M50'
  'MemoryOptimized_M100'
  'MemoryOptimized_M150'
  'MemoryOptimized_M250'
  'MemoryOptimized_M350'
  'MemoryOptimized_M500'
  'MemoryOptimized_M700'
  'MemoryOptimized_M1000'
  'MemoryOptimized_M1500'
  'MemoryOptimized_M2000'
  // Compute Optimized SKUs
  'ComputeOptimized_X3'
  'ComputeOptimized_X5'
  'ComputeOptimized_X10'
  'ComputeOptimized_X20'
  'ComputeOptimized_X50'
  'ComputeOptimized_X100'
  'ComputeOptimized_X150'
  'ComputeOptimized_X250'
  'ComputeOptimized_X350'
  'ComputeOptimized_X500'
  'ComputeOptimized_X700'
  // Flash Optimized SKUs (NO RediSearch support!)
  'FlashOptimized_A250'
  'FlashOptimized_A500'
  'FlashOptimized_A700'
  'FlashOptimized_A1000'
  'FlashOptimized_A1500'
  'FlashOptimized_A2000'
  'FlashOptimized_A4500'
  // Legacy Enterprise SKUs (still supported)
  'Enterprise_E1'
  'Enterprise_E5'
  'Enterprise_E10'
  'Enterprise_E20'
  'Enterprise_E50'
  'Enterprise_E100'
  'Enterprise_E200'
  'Enterprise_E400'
])
param skuName string = 'Balanced_B5'

@description('Clustering policy - EnterpriseCluster for RediSearch, OSSCluster for max throughput')
@allowed(['EnterpriseCluster', 'OSSCluster'])
param clusteringPolicy string = 'OSSCluster'

@description('Enable high availability')
param highAvailability bool = false

@description('Minimum TLS version')
@allowed(['1.0', '1.1', '1.2'])
param minimumTlsVersion string = '1.2'

@description('Enable access keys authentication')
param accessKeysAuthenticationEnabled bool = true

@description('Eviction policy for the database')
@allowed(['NoEviction', 'AllKeysLRU', 'AllKeysRandom', 'VolatileLRU', 'VolatileRandom', 'VolatileTTL', 'AllKeysLFU', 'VolatileLFU'])
param evictionPolicy string = 'VolatileLRU'

@description('Enable RDB persistence')
param rdbEnabled bool = false

@description('RDB persistence frequency (1h, 6h, or 12h)')
@allowed(['1h', '6h', '12h'])
param rdbFrequency string = '1h'

@description('Enable AOF persistence')
param aofEnabled bool = false

@description('AOF persistence frequency (1s or always)')
@allowed(['1s', 'always'])
param aofFrequency string = '1s'

@description('Enable RediSearch module (requires EnterpriseCluster policy)')
param enableRediSearch bool = false

@description('Enable RedisJSON module')
param enableRedisJSON bool = false

@description('Enable RedisTimeSeries module')
param enableRedisTimeSeries bool = false

@description('Enable RedisBloom module')
param enableRedisBloom bool = false

@description('Tags to apply to resources')
param tags object = {}

// Build modules array based on enabled flags
var modulesArray = concat(
  enableRediSearch ? [{ name: 'RediSearch' }] : [],
  enableRedisJSON ? [{ name: 'RedisJSON' }] : [],
  enableRedisTimeSeries ? [{ name: 'RedisTimeSeries' }] : [],
  enableRedisBloom ? [{ name: 'RedisBloom' }] : []
)

// Persistence configuration - can't enable both RDB and AOF
var persistenceConfig = rdbEnabled ? {
  rdbEnabled: true
  rdbFrequency: rdbFrequency
  aofEnabled: false
} : aofEnabled ? {
  aofEnabled: true
  aofFrequency: aofFrequency
  rdbEnabled: false
} : {
  rdbEnabled: false
  aofEnabled: false
}

// Redis Enterprise Cluster - Using 2025-05-01-preview API for v2 support
resource redisEnterprise 'Microsoft.Cache/redisEnterprise@2025-05-01-preview' = {
  name: name
  location: location
  tags: union(tags, { 'managed-by': 'redismeter' })
  sku: {
    name: skuName
  }
  identity: {
    type: 'None'
  }
  properties: {
    minimumTlsVersion: minimumTlsVersion
    highAvailability: highAvailability ? 'Enabled' : 'Disabled'
  }
}

// Database resource with parent reference (matching Azure portal export pattern)
resource database 'Microsoft.Cache/redisEnterprise/databases@2025-05-01-preview' = {
  parent: redisEnterprise
  name: 'default'
  properties: {
    clientProtocol: 'Encrypted'
    port: 10000
    clusteringPolicy: clusteringPolicy
    evictionPolicy: evictionPolicy
    persistence: persistenceConfig
    deferUpgrade: 'NotDeferred'
    accessKeysAuthentication: accessKeysAuthenticationEnabled ? 'Enabled' : 'Disabled'
    modules: modulesArray
  }
}

// Outputs
output id string = redisEnterprise.id
output name string = redisEnterprise.name
output hostName string = redisEnterprise.properties.hostName
output databaseId string = database.id
output port int = 10000

// Output the primary key directly from the database resource
// This ensures the key is only retrieved AFTER the database is fully provisioned
// Critical: This avoids race conditions when using existing resource references
@secure()
output primaryKey string = database.listKeys().primaryKey

@secure()
output secondaryKey string = database.listKeys().secondaryKey
