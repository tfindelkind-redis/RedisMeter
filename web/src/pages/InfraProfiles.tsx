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

// Azure SKU options
const azureSkuOptions = [
  { value: 'Basic', label: 'Basic' },
  { value: 'Standard', label: 'Standard' },
  { value: 'Premium', label: 'Premium' },
  { value: 'Enterprise', label: 'Enterprise' },
  { value: 'EnterpriseFlash', label: 'Enterprise Flash' },
];

// Azure locations
const azureLocations = [
  'eastus', 'eastus2', 'westus', 'westus2', 'centralus',
  'northeurope', 'westeurope', 'uksouth', 'ukwest',
  'southeastasia', 'eastasia', 'japaneast', 'australiaeast',
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
          sku: 'Standard',
          capacity: 1,
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
          return `${config.azure.location} • ${config.azure.sku} • ${config.azure.capacity} capacity`;
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
          <>
            <Divider orientation="left">Azure Redis Cache Configuration</Divider>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  name={['config', 'azure', 'location']}
                  label="Location"
                  rules={[{ required: true }]}
                >
                  <Select
                    placeholder="Select location"
                    showSearch
                    options={azureLocations.map((loc) => ({ value: loc, label: loc }))}
                  />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  name={['config', 'azure', 'sku']}
                  label="SKU"
                  rules={[{ required: true }]}
                >
                  <Select placeholder="Select SKU" options={azureSkuOptions} />
                </Form.Item>
              </Col>
            </Row>
            <Row gutter={16}>
              <Col span={8}>
                <Form.Item
                  name={['config', 'azure', 'capacity']}
                  label="Capacity"
                  rules={[{ required: true }]}
                >
                  <InputNumber min={1} max={10} style={{ width: '100%' }} />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item name={['config', 'azure', 'family']} label="Family">
                  <Select
                    placeholder="Auto"
                    allowClear
                    options={[
                      { value: 'C', label: 'C (Basic/Standard)' },
                      { value: 'P', label: 'P (Premium)' },
                    ]}
                  />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item name={['config', 'azure', 'shard_count']} label="Shard Count">
                  <InputNumber min={1} max={10} placeholder="1" style={{ width: '100%' }} />
                </Form.Item>
              </Col>
            </Row>
            <Row gutter={16}>
              <Col span={8}>
                <Form.Item name={['config', 'azure', 'redis_version']} label="Redis Version">
                  <Select
                    placeholder="Latest"
                    allowClear
                    options={[
                      { value: '6', label: '6.x' },
                      { value: '4', label: '4.x' },
                    ]}
                  />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item name={['config', 'azure', 'minimum_tls_version']} label="Min TLS">
                  <Select
                    placeholder="1.2"
                    options={[
                      { value: '1.0', label: 'TLS 1.0' },
                      { value: '1.1', label: 'TLS 1.1' },
                      { value: '1.2', label: 'TLS 1.2' },
                    ]}
                  />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item
                  name={['config', 'azure', 'enable_non_ssl_port']}
                  label="Non-SSL Port"
                  valuePropName="checked"
                >
                  <Switch />
                </Form.Item>
              </Col>
            </Row>
            <Divider orientation="left">Benchmark VM Configuration</Divider>
            <Row gutter={16}>
              <Col span={8}>
                <Form.Item name={['config', 'azure', 'vm_size']} label="VM Size">
                  <Select
                    placeholder="Standard_D2s_v3"
                    allowClear
                    options={[
                      { value: 'Standard_D2s_v3', label: 'D2s v3 (2 vCPU, 8GB)' },
                      { value: 'Standard_D4s_v3', label: 'D4s v3 (4 vCPU, 16GB)' },
                      { value: 'Standard_D8s_v3', label: 'D8s v3 (8 vCPU, 32GB)' },
                      { value: 'Standard_F4s_v2', label: 'F4s v2 (4 vCPU, 8GB)' },
                    ]}
                  />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item name={['config', 'azure', 'vm_count']} label="VM Count">
                  <InputNumber min={1} max={10} placeholder="1" style={{ width: '100%' }} />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item
                  name={['config', 'azure', 'use_spot_vms']}
                  label="Use Spot VMs"
                  valuePropName="checked"
                >
                  <Switch />
                </Form.Item>
              </Col>
            </Row>
          </>
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
              azure: { location: 'eastus', sku: 'Standard', capacity: 1 },
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
