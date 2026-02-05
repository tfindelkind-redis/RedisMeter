// Private DNS Zone module
// Creates a private DNS zone and links it to a VNet
// Based on: https://github.com/Redislabs-Solution-Architects/workshop-transaction-processing-amr

@description('Name of the private DNS zone (e.g., privatelink.redis.cache.windows.net)')
param name string

@description('Resource ID of the VNet to link')
param vnetId string

@description('Tags to apply to resources')
param tags object = {}

resource privateDnsZone 'Microsoft.Network/privateDnsZones@2020-06-01' = {
  name: name
  location: 'global'
  tags: union(tags, { 'managed-by': 'redismeter' })
  properties: {}
}

resource vnetLink 'Microsoft.Network/privateDnsZones/virtualNetworkLinks@2020-06-01' = {
  parent: privateDnsZone
  name: '${split(vnetId, '/')[8]}-link'
  location: 'global'
  tags: tags
  properties: {
    virtualNetwork: {
      id: vnetId
    }
    registrationEnabled: false
  }
}

output id string = privateDnsZone.id
output name string = privateDnsZone.name
