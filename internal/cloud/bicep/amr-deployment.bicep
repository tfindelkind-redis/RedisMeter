// RedisMeter AMR Deployment Template
// Deploys Azure Managed Redis with optional private endpoint
// Based on: https://github.com/Redislabs-Solution-Architects/workshop-transaction-processing-amr

targetScope = 'resourceGroup'

// ============================================================================
// PARAMETERS
// ============================================================================

@description('Name of the Redis Enterprise cluster')
param redisName string

@description('Azure region for the resource')
param location string

@description('SKU name for AMR')
param skuName string = 'Balanced_B5'

@description('Clustering policy')
@allowed(['EnterpriseCluster', 'OSSCluster'])
param clusteringPolicy string = 'OSSCluster'

@description('Enable high availability')
param highAvailability bool = false

@description('Eviction policy')
@allowed(['NoEviction', 'AllKeysLRU', 'AllKeysRandom', 'VolatileLRU', 'VolatileRandom', 'VolatileTTL', 'AllKeysLFU', 'VolatileLFU'])
param evictionPolicy string = 'VolatileLRU'

@description('Enable RDB persistence')
param rdbEnabled bool = false

@description('RDB frequency')
@allowed(['1h', '6h', '12h'])
param rdbFrequency string = '1h'

@description('Enable AOF persistence')
param aofEnabled bool = false

@description('AOF frequency')
@allowed(['1s', 'always'])
param aofFrequency string = '1s'

@description('Enable RediSearch module')
param enableRediSearch bool = false

@description('Enable RedisJSON module')
param enableRedisJSON bool = false

@description('Enable RedisTimeSeries module')
param enableRedisTimeSeries bool = false

@description('Enable RedisBloom module')
param enableRedisBloom bool = false

@description('Enable private endpoint')
param enablePrivateEndpoint bool = false

@description('VNet ID for private endpoint (required if enablePrivateEndpoint is true)')
param vnetId string = ''

@description('Subnet ID for private endpoint (required if enablePrivateEndpoint is true)')
param subnetId string = ''

@description('Tags to apply to resources')
param tags object = {}

// ============================================================================
// VARIABLES
// ============================================================================

var redisDnsZoneName = 'privatelink.redis.cache.windows.net'
var privateEndpointName = '${redisName}-pe'

// ============================================================================
// AZURE MANAGED REDIS
// ============================================================================

module redis 'redis-enterprise.bicep' = {
  name: 'redis-deployment'
  params: {
    name: redisName
    location: location
    tags: tags
    skuName: skuName
    clusteringPolicy: clusteringPolicy
    highAvailability: highAvailability
    evictionPolicy: evictionPolicy
    rdbEnabled: rdbEnabled
    rdbFrequency: rdbFrequency
    aofEnabled: aofEnabled
    aofFrequency: aofFrequency
    enableRediSearch: enableRediSearch
    enableRedisJSON: enableRedisJSON
    enableRedisTimeSeries: enableRedisTimeSeries
    enableRedisBloom: enableRedisBloom
  }
}

// ============================================================================
// PRIVATE ENDPOINT (Optional)
// ============================================================================

module redisDnsZone 'private-dns-zone.bicep' = if (enablePrivateEndpoint && !empty(vnetId)) {
  name: 'redis-dns-zone-deployment'
  params: {
    name: redisDnsZoneName
    vnetId: vnetId
    tags: tags
  }
}

module redisPrivateEndpoint 'private-endpoint.bicep' = if (enablePrivateEndpoint && !empty(subnetId) && !empty(vnetId)) {
  name: 'redis-pe-deployment'
  params: {
    name: privateEndpointName
    location: location
    tags: tags
    subnetId: subnetId
    privateLinkServiceId: redis.outputs.id
    groupIds: ['redisEnterprise']
    privateDnsZoneId: enablePrivateEndpoint && !empty(vnetId) ? redisDnsZone.outputs.id : ''
    databaseId: redis.outputs.databaseId
  }
}

// ============================================================================
// OUTPUTS
// ============================================================================

output redisId string = redis.outputs.id
output redisName string = redis.outputs.name
output redisHostName string = redis.outputs.hostName
output redisDatabaseId string = redis.outputs.databaseId
output redisPort int = redis.outputs.port

@secure()
output redisPrimaryKey string = redis.outputs.primaryKey

@secure()
output redisSecondaryKey string = redis.outputs.secondaryKey

output privateEndpointId string = enablePrivateEndpoint && !empty(subnetId) && !empty(vnetId) ? redisPrivateEndpoint.outputs.id : ''
output privateEndpointIP string = enablePrivateEndpoint && !empty(subnetId) && !empty(vnetId) ? redisPrivateEndpoint.outputs.privateIPAddress : ''
