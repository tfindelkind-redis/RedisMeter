import { useState, useEffect } from 'react';
import {
  Card,
  Table,
  Button,
  Space,
  Tag,
  Typography,
  Modal,
  Form,
  Input,
  Select,
  InputNumber,
  Switch,
  Tooltip,
  message,
  Popconfirm,
  Statistic,
  Row,
  Col,
  Divider,
  Upload,
  Alert,
  Tabs,
  Radio,
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  CopyOutlined,
  CloudOutlined,
  AmazonOutlined,
  GoogleOutlined,
  DesktopOutlined,
  SettingOutlined,
  ExportOutlined,
  ImportOutlined,
  QuestionCircleOutlined,
  LockOutlined,
  GlobalOutlined,
  ApiOutlined,
  SafetyOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import type { UploadFile } from 'antd/es/upload/interface';
import api from '@/api/client';
import type {
  InfraProfile,
  InfraProfileProvider,
  InfraProfileStats,
} from '@/types';

const { Title, Text, Paragraph } = Typography;
const { TextArea } = Input;

// Provider options
const providerOptions: { value: InfraProfileProvider; label: string; icon: React.ReactNode }[] = [
  { value: 'azure', label: 'Azure', icon: <CloudOutlined style={{ color: '#0078d4' }} /> },
  { value: 'aws', label: 'AWS', icon: <AmazonOutlined style={{ color: '#ff9900' }} /> },
  { value: 'gcp', label: 'Google Cloud', icon: <GoogleOutlined style={{ color: '#4285f4' }} /> },
  { value: 'local', label: 'Local', icon: <DesktopOutlined style={{ color: '#52c41a' }} /> },
  { value: 'custom', label: 'Custom', icon: <SettingOutlined style={{ color: '#faad14' }} /> },
];

// =============================================================================
// AZURE MANAGED REDIS CONFIGURATION OPTIONS
// =============================================================================

// All Azure regions where AMR is available
const azureRegions = [
  // Americas
  { value: 'eastus', label: 'East US', group: 'Americas' },
  { value: 'eastus2', label: 'East US 2', group: 'Americas' },
  { value: 'centralus', label: 'Central US', group: 'Americas' },
  { value: 'northcentralus', label: 'North Central US', group: 'Americas' },
  { value: 'southcentralus', label: 'South Central US', group: 'Americas' },
  { value: 'westus', label: 'West US', group: 'Americas' },
  { value: 'westus2', label: 'West US 2', group: 'Americas' },
  { value: 'westus3', label: 'West US 3', group: 'Americas' },
  { value: 'canadacentral', label: 'Canada Central', group: 'Americas' },
  { value: 'canadaeast', label: 'Canada East', group: 'Americas' },
  { value: 'brazilsouth', label: 'Brazil South', group: 'Americas' },
  // Europe
  { value: 'northeurope', label: 'North Europe', group: 'Europe' },
  { value: 'westeurope', label: 'West Europe', group: 'Europe' },
  { value: 'uksouth', label: 'UK South', group: 'Europe' },
  { value: 'ukwest', label: 'UK West', group: 'Europe' },
  { value: 'francecentral', label: 'France Central', group: 'Europe' },
  { value: 'francesouth', label: 'France South', group: 'Europe' },
  { value: 'germanywestcentral', label: 'Germany West Central', group: 'Europe' },
  { value: 'switzerlandnorth', label: 'Switzerland North', group: 'Europe' },
  { value: 'switzerlandwest', label: 'Switzerland West', group: 'Europe' },
  { value: 'norwayeast', label: 'Norway East', group: 'Europe' },
  { value: 'norwaywest', label: 'Norway West', group: 'Europe' },
  { value: 'swedencentral', label: 'Sweden Central', group: 'Europe' },
  { value: 'polandcentral', label: 'Poland Central', group: 'Europe' },
  { value: 'italynorth', label: 'Italy North', group: 'Europe' },
  // Asia Pacific
  { value: 'eastasia', label: 'East Asia', group: 'Asia Pacific' },
  { value: 'southeastasia', label: 'Southeast Asia', group: 'Asia Pacific' },
  { value: 'australiaeast', label: 'Australia East', group: 'Asia Pacific' },
  { value: 'australiasoutheast', label: 'Australia Southeast', group: 'Asia Pacific' },
  { value: 'australiacentral', label: 'Australia Central', group: 'Asia Pacific' },
  { value: 'japaneast', label: 'Japan East', group: 'Asia Pacific' },
  { value: 'japanwest', label: 'Japan West', group: 'Asia Pacific' },
  { value: 'koreacentral', label: 'Korea Central', group: 'Asia Pacific' },
  { value: 'koreasouth', label: 'Korea South', group: 'Asia Pacific' },
  { value: 'centralindia', label: 'Central India', group: 'Asia Pacific' },
  { value: 'southindia', label: 'South India', group: 'Asia Pacific' },
  { value: 'westindia', label: 'West India', group: 'Asia Pacific' },
  // Middle East & Africa
  { value: 'uaenorth', label: 'UAE North', group: 'Middle East & Africa' },
  { value: 'uaecentral', label: 'UAE Central', group: 'Middle East & Africa' },
  { value: 'southafricanorth', label: 'South Africa North', group: 'Middle East & Africa' },
  { value: 'southafricawest', label: 'South Africa West', group: 'Middle East & Africa' },
  { value: 'qatarcentral', label: 'Qatar Central', group: 'Middle East & Africa' },
  { value: 'israelcentral', label: 'Israel Central', group: 'Middle East & Africa' },
];

// In-memory SKUs - Balanced (general purpose)
const balancedSkus = [
  { value: 'Balanced_B0', label: 'Balanced B0', vcpus: 2, cacheGB: 0.5, description: 'Dev/Test' },
  { value: 'Balanced_B1', label: 'Balanced B1', vcpus: 2, cacheGB: 1, description: '2 vCPUs, 1 GB' },
  { value: 'Balanced_B3', label: 'Balanced B3', vcpus: 2, cacheGB: 3, description: '2 vCPUs, 3 GB' },
  { value: 'Balanced_B5', label: 'Balanced B5', vcpus: 2, cacheGB: 6, description: '2 vCPUs, 6 GB' },
  { value: 'Balanced_B10', label: 'Balanced B10', vcpus: 4, cacheGB: 12, description: '4 vCPUs, 12 GB' },
  { value: 'Balanced_B20', label: 'Balanced B20', vcpus: 4, cacheGB: 24, description: '4 vCPUs, 24 GB' },
  { value: 'Balanced_B50', label: 'Balanced B50', vcpus: 8, cacheGB: 48, description: '8 vCPUs, 48 GB' },
  { value: 'Balanced_B100', label: 'Balanced B100', vcpus: 16, cacheGB: 96, description: '16 vCPUs, 96 GB' },
  { value: 'Balanced_B150', label: 'Balanced B150', vcpus: 24, cacheGB: 144, description: '24 vCPUs, 144 GB' },
  { value: 'Balanced_B250', label: 'Balanced B250', vcpus: 32, cacheGB: 192, description: '32 vCPUs, 192 GB' },
  { value: 'Balanced_B350', label: 'Balanced B350', vcpus: 48, cacheGB: 288, description: '48 vCPUs, 288 GB' },
  { value: 'Balanced_B500', label: 'Balanced B500', vcpus: 64, cacheGB: 384, description: '64 vCPUs, 384 GB' },
  { value: 'Balanced_B700', label: 'Balanced B700', vcpus: 80, cacheGB: 512, description: '80 vCPUs, 512 GB' },
  { value: 'Balanced_B1000', label: 'Balanced B1000', vcpus: 112, cacheGB: 672, description: '112 vCPUs, 672 GB' },
];

// In-memory SKUs - Memory Optimized
const memoryOptimizedSkus = [
  { value: 'MemoryOptimized_M10', label: 'Memory M10', vcpus: 2, cacheGB: 32, description: '2 vCPUs, 32 GB' },
  { value: 'MemoryOptimized_M20', label: 'Memory M20', vcpus: 4, cacheGB: 64, description: '4 vCPUs, 64 GB' },
  { value: 'MemoryOptimized_M50', label: 'Memory M50', vcpus: 8, cacheGB: 128, description: '8 vCPUs, 128 GB' },
  { value: 'MemoryOptimized_M100', label: 'Memory M100', vcpus: 16, cacheGB: 256, description: '16 vCPUs, 256 GB' },
  { value: 'MemoryOptimized_M150', label: 'Memory M150', vcpus: 24, cacheGB: 384, description: '24 vCPUs, 384 GB' },
  { value: 'MemoryOptimized_M250', label: 'Memory M250', vcpus: 32, cacheGB: 512, description: '32 vCPUs, 512 GB' },
  { value: 'MemoryOptimized_M350', label: 'Memory M350', vcpus: 48, cacheGB: 672, description: '48 vCPUs, 672 GB' },
  { value: 'MemoryOptimized_M500', label: 'Memory M500', vcpus: 64, cacheGB: 896, description: '64 vCPUs, 896 GB' },
  { value: 'MemoryOptimized_M700', label: 'Memory M700', vcpus: 80, cacheGB: 1024, description: '80 vCPUs, 1 TB' },
  { value: 'MemoryOptimized_M1000', label: 'Memory M1000', vcpus: 112, cacheGB: 1408, description: '112 vCPUs, 1.4 TB' },
];

// In-memory SKUs - Compute Optimized
const computeOptimizedSkus = [
  { value: 'ComputeOptimized_X3', label: 'Compute X3', vcpus: 2, cacheGB: 3, description: '2 vCPUs, 3 GB' },
  { value: 'ComputeOptimized_X5', label: 'Compute X5', vcpus: 4, cacheGB: 6, description: '4 vCPUs, 6 GB' },
  { value: 'ComputeOptimized_X10', label: 'Compute X10', vcpus: 8, cacheGB: 12, description: '8 vCPUs, 12 GB' },
  { value: 'ComputeOptimized_X20', label: 'Compute X20', vcpus: 16, cacheGB: 24, description: '16 vCPUs, 24 GB' },
  { value: 'ComputeOptimized_X50', label: 'Compute X50', vcpus: 32, cacheGB: 48, description: '32 vCPUs, 48 GB' },
  { value: 'ComputeOptimized_X100', label: 'Compute X100', vcpus: 64, cacheGB: 96, description: '64 vCPUs, 96 GB' },
  { value: 'ComputeOptimized_X150', label: 'Compute X150', vcpus: 96, cacheGB: 144, description: '96 vCPUs, 144 GB' },
  { value: 'ComputeOptimized_X250', label: 'Compute X250', vcpus: 128, cacheGB: 192, description: '128 vCPUs, 192 GB' },
  { value: 'ComputeOptimized_X350', label: 'Compute X350', vcpus: 176, cacheGB: 288, description: '176 vCPUs, 288 GB' },
  { value: 'ComputeOptimized_X500', label: 'Compute X500', vcpus: 256, cacheGB: 384, description: '256 vCPUs, 384 GB' },
  { value: 'ComputeOptimized_X700', label: 'Compute X700', vcpus: 320, cacheGB: 512, description: '320 vCPUs, 512 GB' },
];

// Flash SKUs
const flashOptimizedSkus = [
  { value: 'FlashOptimized_F300', label: 'Flash F300', vcpus: 6, cacheGB: 345, description: '6 vCPUs, ~345 GB usable' },
  { value: 'FlashOptimized_F700', label: 'Flash F700', vcpus: 12, cacheGB: 715, description: '12 vCPUs, ~715 GB usable' },
  { value: 'FlashOptimized_F1500', label: 'Flash F1500', vcpus: 24, cacheGB: 1455, description: '24 vCPUs, ~1.4 TB usable' },
];

// Eviction policies
const evictionPolicies = [
  { value: 'noeviction', label: 'No Eviction', description: 'Return error when memory limit reached' },
  { value: 'allkeys-lru', label: 'All Keys - LRU', description: 'Evict any key using approximated LRU' },
  { value: 'allkeys-lfu', label: 'All Keys - LFU', description: 'Evict any key using approximated LFU' },
  { value: 'volatile-lru', label: 'Volatile - LRU', description: 'Evict keys with TTL using approximated LRU' },
  { value: 'volatile-lfu', label: 'Volatile - LFU', description: 'Evict keys with TTL using approximated LFU' },
  { value: 'allkeys-random', label: 'All Keys - Random', description: 'Evict any key randomly' },
  { value: 'volatile-random', label: 'Volatile - Random', description: 'Evict keys with TTL randomly' },
  { value: 'volatile-ttl', label: 'Volatile - TTL', description: 'Evict keys with nearest TTL' },
];

// Clustering policies
const clusteringPolicies = [
  { value: 'non-clustered', label: 'Non-clustered', description: 'Single Redis instance' },
  { value: 'oss', label: 'OSS', description: 'Redis Cluster (OSS Cluster API)' },
  { value: 'enterprise', label: 'Enterprise', description: 'Enterprise clustering' },
];

// Redis modules available
const redisModules = [
  { value: 'RediSearch', label: 'RediSearch', description: 'Full-text search and secondary indexing' },
  { value: 'RedisJSON', label: 'RedisJSON', description: 'Native JSON data type' },
  { value: 'RedisBloom', label: 'RedisBloom', description: 'Bloom filters and probabilistic data structures' },
  { value: 'RedisTimeSeries', label: 'RedisTimeSeries', description: 'Time-series data structure' },
];

// AWS regions
const awsRegions = [
  'us-east-1', 'us-east-2', 'us-west-1', 'us-west-2',
  'eu-west-1', 'eu-west-2', 'eu-central-1',
  'ap-southeast-1', 'ap-southeast-2', 'ap-northeast-1',
];

// AWS node types
const awsNodeTypes = [
  'cache.t3.micro', 'cache.t3.small', 'cache.t3.medium',
  'cache.m6g.large', 'cache.m6g.xlarge', 'cache.m6g.2xlarge',
  'cache.r6g.large', 'cache.r6g.xlarge', 'cache.r6g.2xlarge',
];

// GCP regions
const gcpRegions = [
  'us-central1', 'us-east1', 'us-west1',
  'europe-west1', 'europe-west2', 'europe-west3',
  'asia-east1', 'asia-southeast1', 'asia-northeast1',
];

// GCP tiers
const gcpTiers = [
  { value: 'BASIC', label: 'Basic' },
  { value: 'STANDARD_HA', label: 'Standard HA' },
];

export default function InfraProfiles() {
  const [profiles, setProfiles] = useState<InfraProfile[]>([]);
  const [stats, setStats] = useState<InfraProfileStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [editingProfile, setEditingProfile] = useState<InfraProfile | null>(null);
  const [selectedProvider, setSelectedProvider] = useState<InfraProfileProvider>('azure');
  const [importModalOpen, setImportModalOpen] = useState(false);
  const [importOverwrite, setImportOverwrite] = useState(false);
  const [form] = Form.useForm();

  useEffect(() => {
    loadProfiles();
    loadStats();
  }, []);

  const loadProfiles = async () => {
    try {
      setLoading(true);
      const result = await api.getInfraProfiles();
      setProfiles(result.profiles || []);
    } catch (error) {
      console.error('Failed to load infrastructure profiles:', error);
      message.error('Failed to load infrastructure profiles');
    } finally {
      setLoading(false);
    }
  };

  const loadStats = async () => {
    try {
      const result = await api.getInfraProfileStats();
      setStats(result);
    } catch (error) {
      console.error('Failed to load profile stats:', error);
    }
  };

  const handleCreate = () => {
    setEditingProfile(null);
    setSelectedProvider('azure');
    form.resetFields();
    form.setFieldsValue({
      provider: 'azure',
      config: {
        azure: {
          location: 'eastus',
          sku: 'Balanced_B0',
        },
      },
    });
    setModalOpen(true);
  };

  const handleEdit = (profile: InfraProfile) => {
    setEditingProfile(profile);
    setSelectedProvider(profile.provider);
    form.setFieldsValue({
      name: profile.name,
      description: profile.description,
      provider: profile.provider,
      tags: profile.tags?.join(', '),
      config: profile.config,
    });
    setModalOpen(true);
  };

  const handleDuplicate = (profile: InfraProfile) => {
    setEditingProfile(null);
    setSelectedProvider(profile.provider);
    form.setFieldsValue({
      name: `${profile.name}-copy`,
      description: profile.description,
      provider: profile.provider,
      tags: profile.tags?.join(', '),
      config: profile.config,
    });
    setModalOpen(true);
  };

  const handleDelete = async (id: string) => {
    try {
      await api.deleteInfraProfile(id);
      message.success('Infrastructure profile deleted');
      loadProfiles();
      loadStats();
    } catch (error) {
      message.error('Failed to delete infrastructure profile');
    }
  };

  const handleSave = async () => {
    try {
      const values = await form.validateFields();

      // Parse tags
      const tags = values.tags
        ? values.tags.split(',').map((t: string) => t.trim()).filter((t: string) => t)
        : [];

      const profileData: Partial<InfraProfile> = {
        name: values.name,
        description: values.description || '',
        provider: values.provider,
        tags,
        config: {},
      };

      // Set provider-specific config
      switch (values.provider) {
        case 'azure':
          profileData.config = { azure: values.config?.azure };
          break;
        case 'aws':
          profileData.config = { aws: values.config?.aws };
          break;
        case 'gcp':
          profileData.config = { gcp: values.config?.gcp };
          break;
        case 'local':
          profileData.config = { local: values.config?.local };
          break;
        case 'custom':
          profileData.config = { custom: values.config?.custom };
          break;
      }

      if (editingProfile) {
        await api.updateInfraProfile(editingProfile.id, profileData);
        message.success('Infrastructure profile updated');
      } else {
        await api.createInfraProfile(profileData);
        message.success('Infrastructure profile created');
      }

      setModalOpen(false);
      loadProfiles();
      loadStats();
    } catch (error: unknown) {
      const err = error as { message?: string };
      message.error(err.message || 'Failed to save infrastructure profile');
    }
  };

  const handleExport = async () => {
    try {
      const blob = await api.exportInfraProfiles();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `infra-profiles-${new Date().toISOString().split('T')[0]}.json`;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);
      message.success('Infrastructure profiles exported');
    } catch (error) {
      message.error('Failed to export profiles');
    }
  };

  const handleImport = async (file: UploadFile) => {
    try {
      if (!file.originFileObj) return;
      const result = await api.importInfraProfiles(file.originFileObj, importOverwrite);
      message.success(`Imported ${result.imported} profiles`);
      setImportModalOpen(false);
      loadProfiles();
      loadStats();
    } catch (error) {
      message.error('Failed to import profiles');
    }
    return false;
  };

  const getProviderIcon = (provider: InfraProfileProvider) => {
    switch (provider) {
      case 'azure':
        return <CloudOutlined style={{ color: '#0078d4' }} />;
      case 'aws':
        return <AmazonOutlined style={{ color: '#ff9900' }} />;
      case 'gcp':
        return <GoogleOutlined style={{ color: '#4285f4' }} />;
      case 'local':
        return <DesktopOutlined style={{ color: '#52c41a' }} />;
      case 'custom':
        return <SettingOutlined style={{ color: '#faad14' }} />;
      default:
        return <CloudOutlined />;
    }
  };

  const getProviderColor = (provider: InfraProfileProvider) => {
    switch (provider) {
      case 'azure':
        return 'blue';
      case 'aws':
        return 'orange';
      case 'gcp':
        return 'red';
      case 'local':
        return 'green';
      case 'custom':
        return 'gold';
      default:
        return 'default';
    }
  };

  const getConfigSummary = (profile: InfraProfile) => {
    const config = profile.config;
    if (!config) return '-';

    switch (profile.provider) {
      case 'azure':
        if (config.azure) {
          return `${config.azure.location} • ${config.azure.sku}`;
        }
        break;
      case 'aws':
        if (config.aws) {
          return `${config.aws.region} • ${config.aws.node_type} • ${config.aws.num_cache_nodes} nodes`;
        }
        break;
      case 'gcp':
        if (config.gcp) {
          return `${config.gcp.region} • ${config.gcp.tier} • ${config.gcp.memory_size_gb}GB`;
        }
        break;
      case 'local':
        if (config.local) {
          return `${config.local.host}:${config.local.port}`;
        }
        break;
      case 'custom':
        if (config.custom) {
          return `${config.custom.host}:${config.custom.port}`;
        }
        break;
    }
    return '-';
  };

  const columns: ColumnsType<InfraProfile> = [
    {
      title: 'Name',
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: InfraProfile) => (
        <Space>
          {getProviderIcon(record.provider)}
          <Text strong>{name}</Text>
          {record.is_builtin && (
            <Tooltip title="Built-in profile (read-only)">
              <LockOutlined style={{ color: '#999' }} />
            </Tooltip>
          )}
        </Space>
      ),
    },
    {
      title: 'Provider',
      dataIndex: 'provider',
      key: 'provider',
      width: 120,
      render: (provider: InfraProfileProvider) => (
        <Tag color={getProviderColor(provider)}>{provider.toUpperCase()}</Tag>
      ),
      filters: providerOptions.map((p) => ({ text: p.label, value: p.value })),
      onFilter: (value, record) => record.provider === value,
    },
    {
      title: 'Configuration',
      key: 'config',
      ellipsis: true,
      render: (_, record: InfraProfile) => (
        <Text type="secondary">{getConfigSummary(record)}</Text>
      ),
    },
    {
      title: 'Tags',
      dataIndex: 'tags',
      key: 'tags',
      width: 200,
      render: (tags: string[]) =>
        tags?.length ? (
          <Space size={[0, 4]} wrap>
            {tags.slice(0, 3).map((tag) => (
              <Tag key={tag}>{tag}</Tag>
            ))}
            {tags.length > 3 && <Tag>+{tags.length - 3}</Tag>}
          </Space>
        ) : (
          <Text type="secondary">-</Text>
        ),
    },
    {
      title: 'Uses',
      dataIndex: 'use_count',
      key: 'use_count',
      width: 80,
      sorter: (a, b) => (a.use_count || 0) - (b.use_count || 0),
      render: (count: number) => (
        <Tag color={count > 0 ? 'blue' : 'default'}>{count || 0}</Tag>
      ),
    },
    {
      title: 'Actions',
      key: 'actions',
      width: 150,
      render: (_, record: InfraProfile) => (
        <Space>
          <Tooltip title="Duplicate">
            <Button type="text" icon={<CopyOutlined />} onClick={() => handleDuplicate(record)} />
          </Tooltip>
          {!record.is_builtin && (
            <>
              <Tooltip title="Edit">
                <Button type="text" icon={<EditOutlined />} onClick={() => handleEdit(record)} />
              </Tooltip>
              <Popconfirm
                title="Delete Infrastructure Profile"
                description="Are you sure you want to delete this profile?"
                onConfirm={() => handleDelete(record.id)}
                okText="Delete"
                okButtonProps={{ danger: true }}
              >
                <Tooltip title="Delete">
                  <Button type="text" danger icon={<DeleteOutlined />} />
                </Tooltip>
              </Popconfirm>
            </>
          )}
        </Space>
      ),
    },
  ];

  const renderProviderConfigForm = () => {
    switch (selectedProvider) {
      case 'azure':
        return (
          <Tabs
            defaultActiveKey="basics"
            items={[
              {
                key: 'basics',
                label: (
                  <span>
                    <CloudOutlined /> Basics
                  </span>
                ),
                children: (
                  <>
                    <Alert
                      message="Azure Managed Redis (AMR)"
                      description="Configure your Azure Managed Redis instance. This creates a high-performance Redis cache in Azure."
                      type="info"
                      showIcon
                      style={{ marginBottom: 16 }}
                    />
                    <Divider orientation="left">Instance Details</Divider>
                    <Row gutter={16}>
                      <Col span={12}>
                        <Form.Item
                          name={['config', 'azure', 'instance_name']}
                          label="Name"
                          tooltip="Instance name (will be: <name>.<region>.redis.azure.net)"
                        >
                          <Input placeholder="my-redis-instance" addonAfter=".redis.azure.net" />
                        </Form.Item>
                      </Col>
                      <Col span={12}>
                        <Form.Item
                          name={['config', 'azure', 'location']}
                          label="Region"
                          rules={[{ required: true, message: 'Please select a region' }]}
                        >
                          <Select
                            placeholder="Select region"
                            showSearch
                            optionFilterProp="label"
                            options={azureRegions.map((r) => ({
                              value: r.value,
                              label: `${r.label} (${r.value})`,
                            }))}
                          />
                        </Form.Item>
                      </Col>
                    </Row>
                    <Divider orientation="left">Performance Tier</Divider>
                    <Paragraph type="secondary" style={{ marginBottom: 16 }}>
                      In-memory tiers use RAM for high-performance caching. Flash tier uses both RAM and SSD for very large datasets.
                    </Paragraph>
                    <Form.Item
                      name={['config', 'azure', 'data_tier']}
                      label="Data Tier"
                      initialValue="in-memory"
                    >
                      <Radio.Group
                        onChange={(e) => {
                          // Reset SKU when data tier changes
                          const newTier = e.target.value;
                          const defaultSku = newTier === 'flash' ? 'FlashOptimized_F300' : 'Balanced_B0';
                          form.setFieldValue(['config', 'azure', 'sku'], defaultSku);
                        }}
                      >
                        <Radio.Button value="in-memory">
                          <strong>In-memory</strong> (Recommended)
                          <br />
                          <Text type="secondary" style={{ fontSize: 12 }}>High-performance caches powered by Redis</Text>
                        </Radio.Button>
                        <Radio.Button value="flash">
                          <strong>Flash</strong>
                          <br />
                          <Text type="secondary" style={{ fontSize: 12 }}>Lower performance, intended for very large datasets</Text>
                        </Radio.Button>
                      </Radio.Group>
                    </Form.Item>
                    <Form.Item
                      noStyle
                      shouldUpdate={(prevValues, currentValues) =>
                        prevValues?.config?.azure?.data_tier !== currentValues?.config?.azure?.data_tier
                      }
                    >
                      {({ getFieldValue }) => {
                        const dataTier = getFieldValue(['config', 'azure', 'data_tier']) || 'in-memory';
                        const allSkus = dataTier === 'flash'
                          ? flashOptimizedSkus
                          : [...balancedSkus, ...memoryOptimizedSkus, ...computeOptimizedSkus];
                        
                        return (
                          <Form.Item
                            name={['config', 'azure', 'sku']}
                            label="Performance (SKU)"
                            rules={[{ required: true, message: 'Please select SKU' }]}
                            tooltip="Determines vCPUs and memory allocation"
                          >
                            <Select
                              placeholder="Select performance tier"
                              showSearch
                              optionFilterProp="label"
                              options={allSkus.map((sku) => ({
                                value: sku.value,
                                label: `${sku.label} - ${sku.description}`,
                              }))}
                            />
                          </Form.Item>
                        );
                      }}
                    </Form.Item>
                  </>
                ),
              },
              {
                key: 'networking',
                label: (
                  <span>
                    <GlobalOutlined /> Networking
                  </span>
                ),
                children: (
                  <>
                    <Divider orientation="left">Network Access</Divider>
                    <Paragraph type="secondary" style={{ marginBottom: 16 }}>
                      Enable access to the Redis instance either publicly using a public IP address or privately using Private Endpoints.
                    </Paragraph>
                    <Form.Item
                      name={['config', 'azure', 'use_private_endpoint']}
                      label="Network Access"
                      initialValue={true}
                    >
                      <Radio.Group>
                        <Space direction="vertical">
                          <Radio value={true}>
                            <strong>Disable public access and use private access</strong>
                            <br />
                            <Text type="secondary">A Private Endpoint is required to reach the instance from within your virtual network.</Text>
                          </Radio>
                          <Radio value={false}>
                            <strong>Enable public access from all networks</strong>
                            <br />
                            <Text type="secondary">Easier to connect, but exposes a public endpoint. Review security posture before enabling.</Text>
                          </Radio>
                        </Space>
                      </Radio.Group>
                    </Form.Item>
                  </>
                ),
              },
              {
                key: 'geo-replication',
                label: (
                  <span>
                    <ApiOutlined /> Active Geo-Replication
                  </span>
                ),
                children: (
                  <>
                    <Divider orientation="left">Active Geo-Replication</Divider>
                    <Paragraph type="secondary" style={{ marginBottom: 16 }}>
                      Azure active geo-replication keeps your cache synchronized for high availability and minimal downtime.
                      It must be enabled during provisioning—caches without it cannot be added to or join active geo-replication groups later.
                    </Paragraph>
                    <Form.Item
                      name={['config', 'azure', 'active_geo_replication']}
                      label="Enable Geo-Replication"
                      valuePropName="checked"
                    >
                      <Switch />
                    </Form.Item>
                    <Form.Item
                      name={['config', 'azure', 'geo_replication_group_name']}
                      label="Geo-Replication Group Name"
                      tooltip="Name of the geo-replication group to create or join"
                    >
                      <Input placeholder="my-geo-group" />
                    </Form.Item>
                  </>
                ),
              },
              {
                key: 'advanced',
                label: (
                  <span>
                    <SafetyOutlined /> Advanced
                  </span>
                ),
                children: (
                  <>
                    <Divider orientation="left">Modules</Divider>
                    <Form.Item
                      name={['config', 'azure', 'modules']}
                      label="Redis Modules"
                      tooltip="Enable additional Redis data structures and capabilities"
                    >
                      <Select
                        mode="multiple"
                        placeholder="Select modules"
                        allowClear
                        options={redisModules.map((m) => ({
                          value: m.value,
                          label: `${m.label} - ${m.description}`,
                        }))}
                      />
                    </Form.Item>
                    
                    <Divider orientation="left">Redis Settings</Divider>
                    <Row gutter={16}>
                      <Col span={12}>
                        <Form.Item
                          name={['config', 'azure', 'eviction_policy']}
                          label="Eviction Policy"
                          tooltip="How Redis handles memory pressure"
                          initialValue="noeviction"
                        >
                          <Select
                            placeholder="Select eviction policy"
                            options={evictionPolicies.map((p) => ({
                              value: p.value,
                              label: p.label,
                              title: p.description,
                            }))}
                          />
                        </Form.Item>
                      </Col>
                      <Col span={12}>
                        <Form.Item
                          name={['config', 'azure', 'clustering_policy']}
                          label="Clustering Policy"
                          tooltip="Client connection mode"
                          initialValue="oss"
                        >
                          <Select
                            placeholder="Select clustering policy"
                            options={clusteringPolicies.map((p) => ({
                              value: p.value,
                              label: `${p.label} - ${p.description}`,
                            }))}
                          />
                        </Form.Item>
                      </Col>
                    </Row>
                    <Row gutter={16}>
                      <Col span={8}>
                        <Form.Item
                          name={['config', 'azure', 'high_availability']}
                          label="High Availability"
                          valuePropName="checked"
                          tooltip="Zone redundancy for high availability"
                          initialValue={true}
                        >
                          <Switch />
                        </Form.Item>
                      </Col>
                      <Col span={8}>
                        <Form.Item
                          name={['config', 'azure', 'non_tls_access_only']}
                          label="Non-TLS Access"
                          valuePropName="checked"
                          tooltip="Allow non-TLS (unencrypted) connections"
                        >
                          <Switch />
                        </Form.Item>
                      </Col>
                      <Col span={8}>
                        <Form.Item
                          name={['config', 'azure', 'access_keys_auth']}
                          label="Access Key Auth"
                          valuePropName="checked"
                          tooltip="Enable access key authentication"
                        >
                          <Switch />
                        </Form.Item>
                      </Col>
                    </Row>
                    
                    <Divider orientation="left">Data Persistence (Preview)</Divider>
                    <Paragraph type="secondary" style={{ marginBottom: 16 }}>
                      Data persistence allows you to persist data stored in Redis. You can take snapshots or write to an append-only file to back up the data which you can load in case of a failure.
                    </Paragraph>
                    <Form.Item
                      name={['config', 'azure', 'persistence_type']}
                      label="Backup File"
                      initialValue=""
                    >
                      <Radio.Group>
                        <Space direction="vertical">
                          <Radio value="">No Persistence</Radio>
                          <Radio value="rdb">Redis Database (RDB)</Radio>
                          <Radio value="aof">Append-only file (AOF)</Radio>
                        </Space>
                      </Radio.Group>
                    </Form.Item>
                    <Form.Item
                      noStyle
                      shouldUpdate={(prevValues, currentValues) =>
                        prevValues?.config?.azure?.persistence_type !== currentValues?.config?.azure?.persistence_type
                      }
                    >
                      {({ getFieldValue }) => {
                        const persistenceType = getFieldValue(['config', 'azure', 'persistence_type']);
                        
                        if (persistenceType === 'rdb') {
                          return (
                            <Form.Item
                              name={['config', 'azure', 'rdb_frequency']}
                              label="RDB Snapshot Frequency"
                              tooltip="How often to create RDB snapshots"
                              initialValue="1h"
                            >
                              <Select
                                placeholder="Select frequency"
                                options={[
                                  { value: '1h', label: 'Every 1 hour' },
                                  { value: '6h', label: 'Every 6 hours' },
                                  { value: '12h', label: 'Every 12 hours' },
                                ]}
                              />
                            </Form.Item>
                          );
                        }
                        
                        if (persistenceType === 'aof') {
                          return (
                            <Form.Item
                              name={['config', 'azure', 'aof_frequency']}
                              label="AOF Sync Frequency"
                              tooltip="How often to sync AOF to disk"
                              initialValue="1s"
                            >
                              <Select
                                placeholder="Select frequency"
                                options={[
                                  { value: '1s', label: 'Every second (fsync every second)' },
                                  { value: 'always', label: 'Always (fsync on every write)' },
                                ]}
                              />
                            </Form.Item>
                          );
                        }
                        
                        return null;
                      }}
                    </Form.Item>
                    
                    <Divider orientation="left">Encryption & Updates</Divider>
                    <Row gutter={16}>
                      <Col span={12}>
                        <Form.Item
                          name={['config', 'azure', 'customer_managed_key']}
                          label="Customer-managed Key"
                          valuePropName="checked"
                          tooltip="Use a customer-managed key for encryption at rest"
                        >
                          <Switch />
                        </Form.Item>
                      </Col>
                      <Col span={12}>
                        <Form.Item
                          name={['config', 'azure', 'defer_version_updates']}
                          label="Defer Version Updates"
                          valuePropName="checked"
                          tooltip="Defer automatic major Redis version updates (Preview)"
                        >
                          <Switch />
                        </Form.Item>
                      </Col>
                    </Row>
                    <Form.Item
                      noStyle
                      shouldUpdate={(prevValues, currentValues) =>
                        prevValues?.config?.azure?.customer_managed_key !== currentValues?.config?.azure?.customer_managed_key ||
                        prevValues?.config?.azure?.key_input_method !== currentValues?.config?.azure?.key_input_method
                      }
                    >
                      {({ getFieldValue }) => {
                        const cmkEnabled = getFieldValue(['config', 'azure', 'customer_managed_key']);
                        
                        if (!cmkEnabled) return null;
                        
                        const keyInputMethod = getFieldValue(['config', 'azure', 'key_input_method']) || 'select';
                        
                        return (
                          <>
                            <Alert
                              message="Select identity and key"
                              description="A user-assigned managed identity is required with Key Vault Crypto User permissions on the selected key vault."
                              type="info"
                              showIcon
                              style={{ marginBottom: 16 }}
                            />
                            
                            <Form.Item
                              name={['config', 'azure', 'user_assigned_identity_id']}
                              label="Select user assigned managed identity"
                              rules={[{ required: cmkEnabled, message: 'User-assigned managed identity is required' }]}
                              tooltip="Resource ID of the user-assigned managed identity with Key Vault Crypto User permissions"
                            >
                              <Input placeholder="/subscriptions/{sub-id}/resourceGroups/{rg}/providers/Microsoft.ManagedIdentity/userAssignedIdentities/{name}" />
                            </Form.Item>
                            
                            <Form.Item
                              name={['config', 'azure', 'key_input_method']}
                              label="Key input method"
                              initialValue="select"
                              tooltip="Choose how to specify the encryption key"
                            >
                              <Radio.Group>
                                <Space direction="vertical">
                                  <Radio value="select">Select Azure key vault and key</Radio>
                                  <Radio value="uri">Enter key from URI</Radio>
                                </Space>
                              </Radio.Group>
                            </Form.Item>
                            
                            {keyInputMethod === 'select' ? (
                              <>
                                <Form.Item
                                  name={['config', 'azure', 'key_vault_subscription_id']}
                                  label="Subscription"
                                  tooltip="Azure subscription containing the Key Vault"
                                >
                                  <Input placeholder="Enter subscription ID (e.g., 12345678-1234-1234-1234-123456789abc)" />
                                </Form.Item>
                                <Form.Item
                                  name={['config', 'azure', 'key_vault_name']}
                                  label="Key vault"
                                  rules={[{ required: cmkEnabled && keyInputMethod === 'select', message: 'Key Vault name is required' }]}
                                  tooltip="Name of the Azure Key Vault"
                                >
                                  <Input placeholder="my-keyvault" />
                                </Form.Item>
                                <Form.Item
                                  name={['config', 'azure', 'key_name']}
                                  label="Customer-managed key (RSA)"
                                  rules={[{ required: cmkEnabled && keyInputMethod === 'select', message: 'Key name is required' }]}
                                  tooltip="Name of the RSA key in the Key Vault"
                                >
                                  <Input placeholder="redis-encryption-key" />
                                </Form.Item>
                                <Form.Item
                                  name={['config', 'azure', 'key_version']}
                                  label="Version"
                                  tooltip="Optional: specific key version (leave empty for latest)"
                                >
                                  <Input placeholder="Leave empty for latest version" />
                                </Form.Item>
                              </>
                            ) : (
                              <Form.Item
                                name={['config', 'azure', 'key_identifier_uri']}
                                label="Key Identifier URI"
                                rules={[{ required: cmkEnabled && keyInputMethod === 'uri', message: 'Key Identifier URI is required' }]}
                                tooltip="Full URI to the key, e.g., https://myvault.vault.azure.net/keys/mykey/abc123..."
                              >
                                <Input placeholder="https://myvault.vault.azure.net/keys/redis-key/abc123def456..." />
                              </Form.Item>
                            )}
                          </>
                        );
                      }}
                    </Form.Item>
                  </>
                ),
              },
              {
                key: 'benchmark-vm',
                label: (
                  <span>
                    <DesktopOutlined /> Benchmark VM
                  </span>
                ),
                children: (
                  <>
                    <Divider orientation="left">Benchmark VM Configuration</Divider>
                    <Paragraph type="secondary" style={{ marginBottom: 16 }}>
                      Configure the virtual machine that will run memtier_benchmark against your Redis instance.
                    </Paragraph>
                    <Row gutter={16}>
                      <Col span={12}>
                        <Form.Item name={['config', 'azure', 'vm_size']} label="VM Size">
                          <Select
                            placeholder="Standard_D2s_v3"
                            allowClear
                            options={[
                              { value: 'Standard_B2s', label: 'B2s (2 vCPU, 4GB) - Burstable' },
                              { value: 'Standard_D2s_v3', label: 'D2s v3 (2 vCPU, 8GB)' },
                              { value: 'Standard_D4s_v3', label: 'D4s v3 (4 vCPU, 16GB)' },
                              { value: 'Standard_D8s_v3', label: 'D8s v3 (8 vCPU, 32GB)' },
                              { value: 'Standard_D16s_v3', label: 'D16s v3 (16 vCPU, 64GB)' },
                              { value: 'Standard_F4s_v2', label: 'F4s v2 (4 vCPU, 8GB) - Compute' },
                              { value: 'Standard_F8s_v2', label: 'F8s v2 (8 vCPU, 16GB) - Compute' },
                              { value: 'Standard_F16s_v2', label: 'F16s v2 (16 vCPU, 32GB) - Compute' },
                            ]}
                          />
                        </Form.Item>
                      </Col>
                      <Col span={12}>
                        <Form.Item name={['config', 'azure', 'vm_count']} label="VM Count">
                          <InputNumber min={1} max={10} placeholder="1" style={{ width: '100%' }} />
                        </Form.Item>
                      </Col>
                    </Row>
                  </>
                ),
              },
            ]}
          />
        );

      case 'aws':
        return (
          <>
            <Divider orientation="left">AWS ElastiCache Configuration</Divider>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  name={['config', 'aws', 'region']}
                  label="Region"
                  rules={[{ required: true }]}
                >
                  <Select
                    placeholder="Select region"
                    showSearch
                    options={awsRegions.map((r) => ({ value: r, label: r }))}
                  />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  name={['config', 'aws', 'node_type']}
                  label="Node Type"
                  rules={[{ required: true }]}
                >
                  <Select
                    placeholder="Select node type"
                    showSearch
                    options={awsNodeTypes.map((t) => ({ value: t, label: t }))}
                  />
                </Form.Item>
              </Col>
            </Row>
            <Row gutter={16}>
              <Col span={8}>
                <Form.Item
                  name={['config', 'aws', 'num_cache_nodes']}
                  label="Number of Nodes"
                  rules={[{ required: true }]}
                >
                  <InputNumber min={1} max={20} style={{ width: '100%' }} />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item name={['config', 'aws', 'engine_version']} label="Engine Version">
                  <Select
                    placeholder="Latest"
                    allowClear
                    options={[
                      { value: '7.0', label: '7.0' },
                      { value: '6.2', label: '6.2' },
                      { value: '6.0', label: '6.0' },
                    ]}
                  />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item
                  name={['config', 'aws', 'cluster_enabled']}
                  label="Cluster Mode"
                  valuePropName="checked"
                >
                  <Switch />
                </Form.Item>
              </Col>
            </Row>
            <Form.Item
              noStyle
              shouldUpdate={(prevValues, currentValues) =>
                prevValues?.config?.aws?.cluster_enabled !== currentValues?.config?.aws?.cluster_enabled
              }
            >
              {({ getFieldValue }) =>
                getFieldValue(['config', 'aws', 'cluster_enabled']) && (
                  <Row gutter={16}>
                    <Col span={12}>
                      <Form.Item
                        name={['config', 'aws', 'num_node_groups']}
                        label="Node Groups (Shards)"
                      >
                        <InputNumber min={1} max={90} style={{ width: '100%' }} />
                      </Form.Item>
                    </Col>
                    <Col span={12}>
                      <Form.Item
                        name={['config', 'aws', 'replicas_per_node_group']}
                        label="Replicas per Group"
                      >
                        <InputNumber min={0} max={5} style={{ width: '100%' }} />
                      </Form.Item>
                    </Col>
                  </Row>
                )
              }
            </Form.Item>
            <Divider orientation="left">Benchmark EC2 Configuration</Divider>
            <Row gutter={16}>
              <Col span={8}>
                <Form.Item name={['config', 'aws', 'ec2_instance_type']} label="Instance Type">
                  <Select
                    placeholder="m5.large"
                    allowClear
                    options={[
                      { value: 'm5.large', label: 'm5.large (2 vCPU, 8GB)' },
                      { value: 'm5.xlarge', label: 'm5.xlarge (4 vCPU, 16GB)' },
                      { value: 'c5.xlarge', label: 'c5.xlarge (4 vCPU, 8GB)' },
                      { value: 'c5.2xlarge', label: 'c5.2xlarge (8 vCPU, 16GB)' },
                    ]}
                  />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item name={['config', 'aws', 'ec2_count']} label="EC2 Count">
                  <InputNumber min={1} max={10} placeholder="1" style={{ width: '100%' }} />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item
                  name={['config', 'aws', 'use_spot_instances']}
                  label="Spot Instances"
                  valuePropName="checked"
                >
                  <Switch />
                </Form.Item>
              </Col>
            </Row>
          </>
        );

      case 'gcp':
        return (
          <>
            <Divider orientation="left">Google Cloud Memorystore Configuration</Divider>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  name={['config', 'gcp', 'region']}
                  label="Region"
                  rules={[{ required: true }]}
                >
                  <Select
                    placeholder="Select region"
                    showSearch
                    options={gcpRegions.map((r) => ({ value: r, label: r }))}
                  />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  name={['config', 'gcp', 'tier']}
                  label="Tier"
                  rules={[{ required: true }]}
                >
                  <Select placeholder="Select tier" options={gcpTiers} />
                </Form.Item>
              </Col>
            </Row>
            <Row gutter={16}>
              <Col span={8}>
                <Form.Item
                  name={['config', 'gcp', 'memory_size_gb']}
                  label="Memory (GB)"
                  rules={[{ required: true }]}
                >
                  <InputNumber min={1} max={300} style={{ width: '100%' }} />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item name={['config', 'gcp', 'redis_version']} label="Redis Version">
                  <Select
                    placeholder="Latest"
                    allowClear
                    options={[
                      { value: 'REDIS_7_0', label: '7.0' },
                      { value: 'REDIS_6_X', label: '6.x' },
                      { value: 'REDIS_5_0', label: '5.0' },
                    ]}
                  />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item name={['config', 'gcp', 'connect_mode']} label="Connect Mode">
                  <Select
                    placeholder="Direct Peering"
                    options={[
                      { value: 'DIRECT_PEERING', label: 'Direct Peering' },
                      { value: 'PRIVATE_SERVICE_ACCESS', label: 'Private Service Access' },
                    ]}
                  />
                </Form.Item>
              </Col>
            </Row>
            <Divider orientation="left">Benchmark VM Configuration</Divider>
            <Row gutter={16}>
              <Col span={8}>
                <Form.Item name={['config', 'gcp', 'machine_type']} label="Machine Type">
                  <Select
                    placeholder="n2-standard-2"
                    allowClear
                    options={[
                      { value: 'n2-standard-2', label: 'n2-standard-2 (2 vCPU, 8GB)' },
                      { value: 'n2-standard-4', label: 'n2-standard-4 (4 vCPU, 16GB)' },
                      { value: 'c2-standard-4', label: 'c2-standard-4 (4 vCPU, 16GB)' },
                    ]}
                  />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item name={['config', 'gcp', 'vm_count']} label="VM Count">
                  <InputNumber min={1} max={10} placeholder="1" style={{ width: '100%' }} />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item
                  name={['config', 'gcp', 'preemptible']}
                  label="Preemptible"
                  valuePropName="checked"
                >
                  <Switch />
                </Form.Item>
              </Col>
            </Row>
          </>
        );

      case 'local':
        return (
          <>
            <Divider orientation="left">Local Redis Configuration</Divider>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  name={['config', 'local', 'host']}
                  label="Host"
                  rules={[{ required: true }]}
                >
                  <Input placeholder="localhost" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  name={['config', 'local', 'port']}
                  label="Port"
                  rules={[{ required: true }]}
                >
                  <InputNumber min={1} max={65535} placeholder="6379" style={{ width: '100%' }} />
                </Form.Item>
              </Col>
            </Row>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item name={['config', 'local', 'password']} label="Password">
                  <Input.Password placeholder="Optional" />
                </Form.Item>
              </Col>
              <Col span={6}>
                <Form.Item name={['config', 'local', 'database']} label="Database">
                  <InputNumber min={0} max={15} placeholder="0" style={{ width: '100%' }} />
                </Form.Item>
              </Col>
              <Col span={6}>
                <Form.Item
                  name={['config', 'local', 'tls']}
                  label="TLS"
                  valuePropName="checked"
                >
                  <Switch />
                </Form.Item>
              </Col>
            </Row>
          </>
        );

      case 'custom':
        return (
          <>
            <Divider orientation="left">Custom Redis Configuration</Divider>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  name={['config', 'custom', 'host']}
                  label="Host"
                  rules={[{ required: true }]}
                >
                  <Input placeholder="redis.example.com" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  name={['config', 'custom', 'port']}
                  label="Port"
                  rules={[{ required: true }]}
                >
                  <InputNumber min={1} max={65535} placeholder="6379" style={{ width: '100%' }} />
                </Form.Item>
              </Col>
            </Row>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item name={['config', 'custom', 'username']} label="Username">
                  <Input placeholder="Optional" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item name={['config', 'custom', 'password']} label="Password">
                  <Input.Password placeholder="Optional" />
                </Form.Item>
              </Col>
            </Row>
            <Row gutter={16}>
              <Col span={8}>
                <Form.Item name={['config', 'custom', 'database']} label="Database">
                  <InputNumber min={0} max={15} placeholder="0" style={{ width: '100%' }} />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item
                  name={['config', 'custom', 'tls']}
                  label="TLS"
                  valuePropName="checked"
                >
                  <Switch />
                </Form.Item>
              </Col>
            </Row>
            <Form.Item name={['config', 'custom', 'description']} label="Description">
              <TextArea rows={2} placeholder="Additional notes about this configuration" />
            </Form.Item>
          </>
        );

      default:
        return null;
    }
  };

  return (
    <div>
      <Row justify="space-between" align="middle" style={{ marginBottom: 24 }}>
        <Col>
          <Title level={2} style={{ marginBottom: 0 }}>
            <CloudOutlined style={{ marginRight: 12 }} />
            Infrastructure Profiles
          </Title>
          <Paragraph type="secondary" style={{ marginBottom: 0 }}>
            Saved cloud infrastructure configurations for Redis benchmarking
          </Paragraph>
        </Col>
        <Col>
          <Space>
            <Button icon={<ImportOutlined />} onClick={() => setImportModalOpen(true)}>
              Import
            </Button>
            <Button icon={<ExportOutlined />} onClick={handleExport} disabled={profiles.length === 0}>
              Export
            </Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
              New Profile
            </Button>
          </Space>
        </Col>
      </Row>

      {/* Stats Cards */}
      {stats && (
        <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
          <Col xs={12} sm={6}>
            <Card size="small">
              <Statistic
                title="Total Profiles"
                value={stats.total_profiles}
                prefix={<CloudOutlined />}
              />
            </Card>
          </Col>
          <Col xs={12} sm={6}>
            <Card size="small">
              <Statistic
                title="Azure"
                value={stats.by_provider?.azure || 0}
                valueStyle={{ color: '#0078d4' }}
              />
            </Card>
          </Col>
          <Col xs={12} sm={6}>
            <Card size="small">
              <Statistic
                title="AWS"
                value={stats.by_provider?.aws || 0}
                valueStyle={{ color: '#ff9900' }}
              />
            </Card>
          </Col>
          <Col xs={12} sm={6}>
            <Card size="small">
              <Statistic
                title="GCP"
                value={stats.by_provider?.gcp || 0}
                valueStyle={{ color: '#4285f4' }}
              />
            </Card>
          </Col>
        </Row>
      )}

      {/* Profiles Table */}
      <Card>
        <Table
          columns={columns}
          dataSource={profiles}
          rowKey="id"
          loading={loading}
          pagination={{
            pageSize: 10,
            showSizeChanger: true,
            showTotal: (total) => `${total} profiles`,
          }}
          locale={{
            emptyText: (
              <div style={{ padding: 40 }}>
                <CloudOutlined style={{ fontSize: 48, color: '#ccc' }} />
                <p style={{ marginTop: 16 }}>No infrastructure profiles yet</p>
                <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
                  Create Your First Profile
                </Button>
              </div>
            ),
          }}
        />
      </Card>

      {/* Create/Edit Modal */}
      <Modal
        title={editingProfile ? 'Edit Infrastructure Profile' : 'New Infrastructure Profile'}
        open={modalOpen}
        onOk={handleSave}
        onCancel={() => setModalOpen(false)}
        width={800}
        okText={editingProfile ? 'Update' : 'Create'}
      >
        <Form
          form={form}
          layout="vertical"
          initialValues={{
            provider: 'azure',
            config: {
              azure: { location: 'eastus', sku: 'Balanced_B0' },
            },
          }}
        >
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="name"
                label="Profile Name"
                rules={[
                  { required: true, message: 'Please enter a profile name' },
                  { pattern: /^[a-zA-Z0-9_-]+$/, message: 'Name can only contain letters, numbers, - and _' },
                ]}
              >
                <Input placeholder="my-azure-profile" disabled={!!editingProfile} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="provider"
                label="Cloud Provider"
                rules={[{ required: true }]}
              >
                <Select
                  placeholder="Select provider"
                  disabled={!!editingProfile}
                  onChange={(value: InfraProfileProvider) => setSelectedProvider(value)}
                  options={providerOptions.map((p) => ({
                    value: p.value,
                    label: (
                      <Space>
                        {p.icon}
                        {p.label}
                      </Space>
                    ),
                  }))}
                />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item name="description" label="Description">
            <TextArea rows={2} placeholder="Description of this infrastructure profile" />
          </Form.Item>

          <Form.Item
            name="tags"
            label={
              <Space>
                Tags
                <Tooltip title="Comma-separated list of tags">
                  <QuestionCircleOutlined />
                </Tooltip>
              </Space>
            }
          >
            <Input placeholder="production, high-performance, shard-cluster" />
          </Form.Item>

          {renderProviderConfigForm()}
        </Form>
      </Modal>

      {/* Import Modal */}
      <Modal
        title="Import Infrastructure Profiles"
        open={importModalOpen}
        onCancel={() => setImportModalOpen(false)}
        footer={null}
      >
        <Alert
          message="Import profiles from a JSON file"
          description="Upload a previously exported infrastructure profiles file."
          type="info"
          style={{ marginBottom: 16 }}
        />
        <Form.Item label="Overwrite existing profiles">
          <Switch checked={importOverwrite} onChange={setImportOverwrite} />
          <Text type="secondary" style={{ marginLeft: 8 }}>
            If enabled, profiles with the same name will be replaced
          </Text>
        </Form.Item>
        <Upload.Dragger
          accept=".json"
          maxCount={1}
          beforeUpload={(file) => {
            handleImport({ originFileObj: file } as UploadFile);
            return false;
          }}
        >
          <p className="ant-upload-drag-icon">
            <ImportOutlined />
          </p>
          <p className="ant-upload-text">Click or drag file to import</p>
          <p className="ant-upload-hint">Supports JSON files exported from RedisMeter</p>
        </Upload.Dragger>
      </Modal>
    </div>
  );
}
