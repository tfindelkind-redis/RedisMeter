import { 
  Form, 
  Select, 
  InputNumber, 
  Switch, 
  Input, 
  Row, 
  Col, 
  Card, 
  Typography,
  Tag,
  Tooltip,
  Space,
  Alert,
} from 'antd';
import { 
  CloudServerOutlined, 
  DatabaseOutlined,
  InfoCircleOutlined,
  ThunderboltOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import { ProviderFormProps } from '@/types';
import { PROVIDER_INFO } from './providerInfo';
import { useEffect, useState } from 'react';

const { Text, Paragraph } = Typography;
const { Option, OptGroup } = Select;

const azureInfo = PROVIDER_INFO.azure;

// Group SKUs by tier
const groupedSkus = azureInfo.redis_skus?.reduce((acc, sku) => {
  if (!acc[sku.tier]) acc[sku.tier] = [];
  acc[sku.tier].push(sku);
  return acc;
}, {} as Record<string, typeof azureInfo.redis_skus>);

// Group instance types by category
const groupedInstances = azureInfo.instance_types.reduce((acc, inst) => {
  if (!acc[inst.category]) acc[inst.category] = [];
  acc[inst.category].push(inst);
  return acc;
}, {} as Record<string, typeof azureInfo.instance_types>);

const categoryLabels: Record<string, string> = {
  general: 'General Purpose',
  compute: 'Compute Optimized',
  memory: 'Memory Optimized',
  storage: 'Storage Optimized',
};

const tierColors: Record<string, string> = {
  'Balanced': 'blue',
  'Memory Optimized': 'purple',
  'Compute Optimized': 'orange',
  'Flash Optimized': 'cyan',
};

// AMR validation rules based on Azure documentation
const AMR_RULES = {
  // Flash Optimized tier does NOT support RediSearch
  flashOptimizedNoSearch: true,
  // Modules (RediSearch, RedisJSON) require Enterprise Cluster policy
  modulesRequireEnterpriseCluster: ['RediSearch', 'RedisJSON'],
  // RediSearch requires minimum 3GB memory
  rediSearchMinMemoryGB: 3,
  // Modules available per tier
  modulesByTier: {
    'Balanced': ['RedisJSON', 'RediSearch', 'RedisTimeSeries', 'RedisBloom'],
    'Memory Optimized': ['RedisJSON', 'RediSearch', 'RedisTimeSeries', 'RedisBloom'],
    'Compute Optimized': ['RedisJSON', 'RediSearch', 'RedisTimeSeries', 'RedisBloom'],
    'Flash Optimized': ['RedisJSON', 'RedisTimeSeries', 'RedisBloom'], // NO RediSearch!
  } as Record<string, string[]>,
};

// Get SKU info by ID
const getSkuInfo = (skuId: string) => {
  return azureInfo.redis_skus?.find(s => s.id === skuId);
};

export default function AzureProvider({ form, disabled }: ProviderFormProps) {
  const provisionAMR = Form.useWatch('amr_enabled', form);
  const selectedSku = Form.useWatch('amr_sku', form);
  const selectedModules = Form.useWatch('amr_modules', form) || [];
  const clusteringPolicy = Form.useWatch('amr_clustering', form);
  
  const [validationWarnings, setValidationWarnings] = useState<string[]>([]);

  // Get current SKU info
  const currentSkuInfo = selectedSku ? getSkuInfo(selectedSku) : null;
  const currentTier = currentSkuInfo?.tier || '';
  const currentMemoryGB = currentSkuInfo?.memory_gb || 0;
  const isFlashOptimized = currentTier === 'Flash Optimized';

  // Available modules based on selected tier
  const availableModules = currentTier 
    ? AMR_RULES.modulesByTier[currentTier] || []
    : ['RedisJSON', 'RediSearch', 'RedisTimeSeries', 'RedisBloom'];

  // Validate selections and update warnings
  useEffect(() => {
    const warnings: string[] = [];
    
    if (selectedModules.length > 0) {
      // Check if modules require Enterprise Cluster
      const requiresEnterprise = selectedModules.some((m: string) => 
        AMR_RULES.modulesRequireEnterpriseCluster.includes(m)
      );
      if (requiresEnterprise && clusteringPolicy === 'OSSCluster') {
        warnings.push('RediSearch and RedisJSON require Enterprise Cluster policy. OSS Cluster is not compatible with these modules.');
        // Auto-switch to Enterprise Cluster
        form.setFieldValue('amr_clustering', 'EnterpriseCluster');
      }

      // Check RediSearch minimum memory
      if (selectedModules.includes('RediSearch') && currentMemoryGB < AMR_RULES.rediSearchMinMemoryGB) {
        warnings.push(`RediSearch requires minimum ${AMR_RULES.rediSearchMinMemoryGB}GB memory. Current SKU has ${currentMemoryGB}GB.`);
      }

      // Check Flash Optimized restrictions
      if (isFlashOptimized && selectedModules.includes('RediSearch')) {
        warnings.push('RediSearch is NOT available on Flash Optimized tier. Please select a different SKU or remove RediSearch.');
        // Remove RediSearch from selection
        const filtered = selectedModules.filter((m: string) => m !== 'RediSearch');
        form.setFieldValue('amr_modules', filtered);
      }
    }

    setValidationWarnings(warnings);
  }, [selectedModules, clusteringPolicy, currentMemoryGB, isFlashOptimized, form]);

  // Filter out unavailable modules when SKU changes
  useEffect(() => {
    if (selectedModules.length > 0 && currentTier) {
      const validModules = selectedModules.filter((m: string) => availableModules.includes(m));
      if (validModules.length !== selectedModules.length) {
        form.setFieldValue('amr_modules', validModules);
      }
    }
  }, [currentTier, selectedModules, availableModules, form]);

  return (
    <div>
      {/* Region Selection */}
      <Card
        title={
          <Space>
            <CloudServerOutlined style={{ color: '#0078d4' }} />
            <span>Azure Region</span>
          </Space>
        }
        style={{ background: '#1f1f1f', border: '1px solid #303030', marginBottom: 24 }}
        size="small"
      >
        <Form.Item
          name="region"
          label="Region"
          rules={[{ required: true, message: 'Please select a region' }]}
          extra="Choose a region close to your target users for best latency results"
        >
          <Select 
            placeholder="Select Azure region"
            disabled={disabled}
            showSearch
            optionFilterProp="children"
          >
            {azureInfo.regions.map(region => (
              <Option key={region.id} value={region.id} disabled={!region.available}>
                {region.name}
                {!region.available && ' (Unavailable)'}
              </Option>
            ))}
          </Select>
        </Form.Item>
      </Card>

      {/* Azure Managed Redis Configuration */}
      <Card
        title={
          <Space>
            <DatabaseOutlined style={{ color: '#DC382D' }} />
            <span>Azure Managed Redis</span>
            <Form.Item name="amr_enabled" valuePropName="checked" noStyle>
              <Switch size="small" disabled={disabled} />
            </Form.Item>
          </Space>
        }
        style={{ background: '#1f1f1f', border: '1px solid #303030', marginBottom: 24 }}
        size="small"
      >
        {provisionAMR ? (
          <>
            {validationWarnings.length > 0 && (
              <Alert
                type="warning"
                showIcon
                icon={<WarningOutlined />}
                message="Configuration Warning"
                description={
                  <ul style={{ margin: 0, paddingLeft: 16 }}>
                    {validationWarnings.map((w, i) => <li key={i}>{w}</li>)}
                  </ul>
                }
                style={{ marginBottom: 16 }}
              />
            )}

            <Form.Item
              name="amr_sku"
              label="SKU"
              rules={[{ required: provisionAMR, message: 'Please select an AMR SKU' }]}
              extra={
                <Space direction="vertical" size={0}>
                  <span>Choose based on your memory and throughput requirements.</span>
                  {isFlashOptimized && (
                    <Text type="warning" style={{ fontSize: 12 }}>
                      ⚠️ Flash Optimized does NOT support RediSearch
                    </Text>
                  )}
                </Space>
              }
            >
              <Select 
                placeholder="Select AMR SKU"
                disabled={disabled}
                showSearch
                optionFilterProp="children"
              >
                {groupedSkus && Object.entries(groupedSkus).map(([tier, skus]) => (
                  <OptGroup key={tier} label={<Tag color={tierColors[tier]}>{tier}</Tag>}>
                    {skus?.map(sku => (
                      <Option key={sku.id} value={sku.id}>
                        <Space>
                          <span>{sku.name}</span>
                          <Text type="secondary" style={{ fontSize: 12 }}>
                            {sku.memory_gb}GB {sku.throughput && `• ${sku.throughput}`}
                          </Text>
                        </Space>
                      </Option>
                    ))}
                  </OptGroup>
                ))}
              </Select>
            </Form.Item>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  name="amr_ha"
                  label={
                    <Space>
                      High Availability
                      <Tooltip title="Enable zone redundancy for 99.99% SLA">
                        <InfoCircleOutlined />
                      </Tooltip>
                    </Space>
                  }
                  valuePropName="checked"
                >
                  <Switch disabled={disabled} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  name="amr_clustering"
                  label={
                    <Space>
                      Clustering Policy
                      <Tooltip title="Enterprise Cluster is required for RediSearch and RedisJSON modules">
                        <InfoCircleOutlined />
                      </Tooltip>
                    </Space>
                  }
                  initialValue="EnterpriseCluster"
                  extra={selectedModules.some((m: string) => ['RediSearch', 'RedisJSON'].includes(m)) 
                    ? <Text type="warning" style={{ fontSize: 12 }}>Enterprise required for selected modules</Text>
                    : undefined
                  }
                >
                  <Select 
                    disabled={disabled || selectedModules.some((m: string) => ['RediSearch', 'RedisJSON'].includes(m))}
                  >
                    <Option value="EnterpriseCluster">Enterprise Cluster (supports all modules)</Option>
                    <Option value="OSSCluster">OSS Cluster (no RediSearch/JSON)</Option>
                  </Select>
                </Form.Item>
              </Col>
            </Row>

            <Form.Item
              name="amr_modules"
              label="Redis Modules"
              extra={
                <Space direction="vertical" size={0}>
                  <span>Optional modules to enable. Some modules have requirements:</span>
                  <Text type="secondary" style={{ fontSize: 11 }}>
                    • RediSearch: Requires Enterprise Cluster + min 3GB SKU + not Flash tier
                  </Text>
                  <Text type="secondary" style={{ fontSize: 11 }}>
                    • RedisJSON: Requires Enterprise Cluster policy
                  </Text>
                </Space>
              }
            >
              <Select
                mode="multiple"
                placeholder="Select modules"
                disabled={disabled}
                allowClear
              >
                {availableModules.includes('RedisJSON') && (
                  <Option value="RedisJSON">
                    RedisJSON
                    <Text type="secondary" style={{ fontSize: 11, marginLeft: 8 }}>
                      (requires Enterprise Cluster)
                    </Text>
                  </Option>
                )}
                {availableModules.includes('RediSearch') && (
                  <Option 
                    value="RediSearch"
                    disabled={currentMemoryGB > 0 && currentMemoryGB < AMR_RULES.rediSearchMinMemoryGB}
                  >
                    RediSearch
                    <Text type="secondary" style={{ fontSize: 11, marginLeft: 8 }}>
                      (min 3GB, Enterprise Cluster)
                    </Text>
                  </Option>
                )}
                {availableModules.includes('RedisTimeSeries') && (
                  <Option value="RedisTimeSeries">RedisTimeSeries</Option>
                )}
                {availableModules.includes('RedisBloom') && (
                  <Option value="RedisBloom">RedisBloom</Option>
                )}
              </Select>
            </Form.Item>

            <Form.Item
              name="amr_eviction"
              label="Eviction Policy"
              initialValue="VolatileLRU"
            >
              <Select disabled={disabled}>
                <Option value="VolatileLRU">Volatile LRU</Option>
                <Option value="AllKeysLRU">All Keys LRU</Option>
                <Option value="VolatileLFU">Volatile LFU</Option>
                <Option value="AllKeysLFU">All Keys LFU</Option>
                <Option value="VolatileRandom">Volatile Random</Option>
                <Option value="AllKeysRandom">All Keys Random</Option>
                <Option value="VolatileTTL">Volatile TTL</Option>
                <Option value="NoEviction">No Eviction</Option>
              </Select>
            </Form.Item>
          </>
        ) : (
          <Paragraph type="secondary">
            Enable to provision a new Azure Managed Redis instance for benchmarking.
            You can also connect to an existing Redis instance in the benchmark configuration.
          </Paragraph>
        )}
      </Card>

      {/* Runner VMs Configuration */}
      <Card
        title={
          <Space>
            <ThunderboltOutlined style={{ color: '#52c41a' }} />
            <span>Runner VMs</span>
          </Space>
        }
        style={{ background: '#1f1f1f', border: '1px solid #303030', marginBottom: 24 }}
        size="small"
      >
        <Form.Item
          name="runner_count"
          label="Number of Runners"
          rules={[{ required: true, message: 'Specify number of runner VMs' }]}
          extra="More runners = higher total load. Each runner executes memtier_benchmark in parallel."
        >
          <InputNumber 
            min={1} 
            max={20} 
            disabled={disabled}
            style={{ width: '100%' }}
          />
        </Form.Item>

        <Form.Item
          name="runner_type"
          label="VM Size"
          rules={[{ required: true, message: 'Select VM instance type' }]}
          extra="Larger VMs can generate more load per runner"
        >
          <Select 
            placeholder="Select VM size"
            disabled={disabled}
            showSearch
            optionFilterProp="children"
          >
            {Object.entries(groupedInstances).map(([category, instances]) => (
              <OptGroup key={category} label={categoryLabels[category] || category}>
                {instances.map(inst => (
                  <Option key={inst.id} value={inst.id}>
                    <Space>
                      <span>{inst.name}</span>
                      <Text type="secondary" style={{ fontSize: 12 }}>
                        {inst.vcpus} vCPU • {inst.memory_gb}GB RAM
                      </Text>
                    </Space>
                  </Option>
                ))}
              </OptGroup>
            ))}
          </Select>
        </Form.Item>

        <Row gutter={16}>
          <Col span={12}>
            <Form.Item
              name="ssh_user"
              label="SSH Username"
              initialValue="azureuser"
            >
              <Input disabled={disabled} />
            </Form.Item>
          </Col>
          <Col span={12}>
            {/* Reserved for future options */}
          </Col>
        </Row>

        <Form.Item
          name="ssh_key"
          label="SSH Public Key"
          extra="Leave empty to use default key from ~/.ssh/"
        >
          <Input.TextArea 
            rows={3} 
            placeholder="ssh-rsa AAAA... or ssh-ed25519 AAAA..."
            disabled={disabled}
          />
        </Form.Item>
      </Card>

      {/* Advanced Settings */}
      <Card
        title="Advanced Settings"
        style={{ background: '#1f1f1f', border: '1px solid #303030' }}
        size="small"
      >
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item
              name="ttl"
              label={
                <Space>
                  Auto-Destroy TTL
                  <Tooltip title="Automatically destroy infrastructure after this duration">
                    <InfoCircleOutlined />
                  </Tooltip>
                </Space>
              }
            >
              <Select placeholder="No auto-destroy" disabled={disabled} allowClear>
                <Option value="1h">1 hour</Option>
                <Option value="2h">2 hours</Option>
                <Option value="4h">4 hours</Option>
                <Option value="8h">8 hours</Option>
                <Option value="24h">24 hours</Option>
              </Select>
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="resource_group"
              label="Resource Group Name"
              extra="Custom name (auto-generated if empty)"
            >
              <Input placeholder="rg-redismeter-..." disabled={disabled} />
            </Form.Item>
          </Col>
        </Row>

        <Form.Item
          name="tags"
          label="Tags"
          extra="Key=Value pairs, comma-separated"
        >
          <Input placeholder="env=test, team=platform" disabled={disabled} />
        </Form.Item>
      </Card>
    </div>
  );
}
