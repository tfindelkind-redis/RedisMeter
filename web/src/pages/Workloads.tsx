import { useState, useEffect } from 'react';
import {
  Card,
  Table,
  Button,
  Space,
  Typography,
  Tag,
  Modal,
  Form,
  Input,
  InputNumber,
  Select,
  Slider,
  Row,
  Col,
  Divider,
  message,
  Popconfirm,
  Tooltip,
  Empty,
  Tabs,
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  CopyOutlined,
  ThunderboltOutlined,
  LockOutlined,
  RadarChartOutlined,
} from '@ant-design/icons';
import { ColumnsType } from 'antd/es/table';
import api from '@/api/client';

const { Title, Text } = Typography;
const { Option } = Select;
const { TextArea } = Input;

interface Operation {
  command: string;
  ratio: number;
}

interface KeyPattern {
  prefix: string;
  pattern: string;
  key_range: number;
}

interface DataSize {
  min?: number;
  max?: number;
  fixed?: number;
  size_list?: string;
  size_pattern?: string;
}

interface WorkloadFull {
  name: string;
  description: string;
  type: string;
  operations: Operation[];
  key_pattern?: KeyPattern;
  data_size?: DataSize;
  // Execution settings moved to Run Profiles
  threads?: number;  // Deprecated - for backwards compatibility
  clients?: number;  // Deprecated - for backwards compatibility
  duration?: string; // Deprecated - for backwards compatibility
  pipeline?: number; // Deprecated - for backwards compatibility
  // New workload-specific fields
  random_data?: boolean;
  data_offset?: number;
  expiry_min?: number;
  expiry_max?: number;
  custom_commands?: string[];
  is_builtin?: boolean;
}

const COMMANDS = ['GET', 'SET', 'MGET', 'MSET', 'HGET', 'HSET', 'HMGET', 'HMSET', 'LPUSH', 'LPOP', 'RPUSH', 'RPOP', 'LRANGE', 'SADD', 'SMEMBERS', 'SREM', 'ZADD', 'ZRANGE', 'ZRANGEBYSCORE', 'INCR', 'DECR', 'INCRBY', 'APPEND', 'GETEX', 'SETEX', 'PFADD', 'PFCOUNT', 'XADD', 'XREAD', 'WAIT'];
const KEY_PATTERNS = ['random', 'sequential', 'gaussian', 'zipf'];

// ANN Benchmarks types and constants
interface ANNWorkload {
  name: string;
  description: string;
  dataset: string;
  distance_metric: string;
  dimensions: number;
  k: number;
  m?: number;
  ef_construction?: number;
  is_builtin?: boolean;
}

const ANN_DATASETS = [
  { value: 'sift-128-euclidean', label: 'SIFT (128D, L2)', dimensions: 128 },
  { value: 'gist-960-euclidean', label: 'GIST (960D, L2)', dimensions: 960 },
  { value: 'glove-25-angular', label: 'GloVe-25 (25D, Cosine)', dimensions: 25 },
  { value: 'glove-50-angular', label: 'GloVe-50 (50D, Cosine)', dimensions: 50 },
  { value: 'glove-100-angular', label: 'GloVe-100 (100D, Cosine)', dimensions: 100 },
  { value: 'glove-200-angular', label: 'GloVe-200 (200D, Cosine)', dimensions: 200 },
  { value: 'mnist-784-euclidean', label: 'MNIST (784D, L2)', dimensions: 784 },
  { value: 'fashion-mnist-784-euclidean', label: 'Fashion-MNIST (784D, L2)', dimensions: 784 },
  { value: 'deep-image-96-angular', label: 'Deep Image (96D, Cosine)', dimensions: 96 },
  { value: 'custom', label: 'Custom Dataset', dimensions: 0 },
];

const DISTANCE_METRICS = [
  { value: 'L2', label: 'L2 (Euclidean)' },
  { value: 'IP', label: 'IP (Inner Product)' },
  { value: 'COSINE', label: 'Cosine Similarity' },
];

export default function Workloads() {
  // Memtier state
  const [workloads, setWorkloads] = useState<WorkloadFull[]>([]);
  const [loading, setLoading] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [editingWorkload, setEditingWorkload] = useState<WorkloadFull | null>(null);
  const [form] = Form.useForm();
  const [operations, setOperations] = useState<Operation[]>([{ command: 'GET', ratio: 0.8 }, { command: 'SET', ratio: 0.2 }]);

  // ANN Benchmarks state
  const [annWorkloads, setAnnWorkloads] = useState<ANNWorkload[]>([]);
  const [annLoading, setAnnLoading] = useState(true);
  const [annModalOpen, setAnnModalOpen] = useState(false);
  const [editingAnnWorkload, setEditingAnnWorkload] = useState<ANNWorkload | null>(null);
  const [annForm] = Form.useForm();

  // Tab state
  const [activeTab, setActiveTab] = useState('memtier');

  useEffect(() => {
    loadWorkloads();
    loadAnnWorkloads();
  }, []);

  const loadWorkloads = async () => {
    try {
      const data = await api.getWorkloadsFull();
      setWorkloads(data);
    } catch (error) {
      console.error('Failed to load workloads', error);
      // Fall back to simple list
      try {
        const simple = await api.getWorkloads();
        setWorkloads(simple.map(w => ({
          name: w.name,
          description: w.description || '',
          type: 'builtin',
          operations: [],
          threads: 4,
          clients: 50,
          duration: '30s',
          pipeline: 1,
          is_builtin: true,
        })));
      } catch (e) {
        console.error('Failed to load workloads', e);
      }
    } finally {
      setLoading(false);
    }
  };

  const loadAnnWorkloads = async () => {
    try {
      // TODO: Replace with actual API call when backend supports it
      // For now, use mock data
      setAnnWorkloads([
        {
          name: 'sift-baseline',
          description: 'SIFT 1M baseline benchmark with HNSW index',
          dataset: 'sift-128-euclidean',
          distance_metric: 'L2',
          dimensions: 128,
          k: 10,
          m: 16,
          ef_construction: 200,
          is_builtin: true,
        },
        {
          name: 'glove-semantic',
          description: 'GloVe word embeddings similarity search',
          dataset: 'glove-100-angular',
          distance_metric: 'COSINE',
          dimensions: 100,
          k: 10,
          m: 16,
          ef_construction: 200,
          is_builtin: true,
        },
      ]);
    } catch (error) {
      console.error('Failed to load ANN workloads', error);
    } finally {
      setAnnLoading(false);
    }
  };

  const handleCreate = () => {
    setEditingWorkload(null);
    setOperations([{ command: 'GET', ratio: 0.8 }, { command: 'SET', ratio: 0.2 }]);
    form.resetFields();
    form.setFieldsValue({
      type: 'custom',
      key_prefix: 'custom:',
      key_pattern: 'random',
      key_range: 100000,
      data_size_type: 'fixed',
      data_size_fixed: 256,
      random_data: false,
      data_offset: 0,
      expiry_enabled: false,
    });
    setModalOpen(true);
  };

  const handleEdit = (workload: WorkloadFull) => {
    if (workload.is_builtin) {
      message.warning('Cannot edit built-in workloads. Use duplicate instead.');
      return;
    }
    setEditingWorkload(workload);
    setOperations(workload.operations || [{ command: 'GET', ratio: 0.8 }, { command: 'SET', ratio: 0.2 }]);
    form.setFieldsValue({
      name: workload.name,
      description: workload.description,
      type: workload.type,
      key_prefix: workload.key_pattern?.prefix || '',
      key_pattern: workload.key_pattern?.pattern || 'random',
      key_range: workload.key_pattern?.key_range || 100000,
      data_size_type: workload.data_size?.fixed ? 'fixed' : 'range',
      data_size_fixed: workload.data_size?.fixed || 256,
      data_size_min: workload.data_size?.min || 64,
      data_size_max: workload.data_size?.max || 1024,
      random_data: workload.random_data || false,
      data_offset: workload.data_offset || 0,
      expiry_enabled: (workload.expiry_min && workload.expiry_min > 0) || false,
      expiry_min: workload.expiry_min || 0,
      expiry_max: workload.expiry_max || 0,
    });
    setModalOpen(true);
  };

  const handleDuplicate = (workload: WorkloadFull) => {
    setEditingWorkload(null);
    setOperations(workload.operations || [{ command: 'GET', ratio: 0.8 }, { command: 'SET', ratio: 0.2 }]);
    form.setFieldsValue({
      name: `${workload.name}-copy`,
      description: workload.description,
      type: 'custom',
      key_prefix: workload.key_pattern?.prefix || '',
      key_pattern: workload.key_pattern?.pattern || 'random',
      key_range: workload.key_pattern?.key_range || 100000,
      data_size_type: workload.data_size?.fixed ? 'fixed' : 'range',
      data_size_fixed: workload.data_size?.fixed || 256,
      data_size_min: workload.data_size?.min || 64,
      data_size_max: workload.data_size?.max || 1024,
      random_data: workload.random_data || false,
      data_offset: workload.data_offset || 0,
      expiry_enabled: (workload.expiry_min && workload.expiry_min > 0) || false,
      expiry_min: workload.expiry_min || 0,
      expiry_max: workload.expiry_max || 0,
    });
    setModalOpen(true);
  };

  const handleDelete = async (name: string) => {
    try {
      await api.deleteWorkload(name);
      message.success('Workload deleted');
      loadWorkloads();
    } catch (error: any) {
      message.error(error.message || 'Failed to delete workload');
    }
  };

  const handleSave = async () => {
    try {
      const values = await form.validateFields();
      
      // Validate operations sum to ~1.0
      const totalRatio = operations.reduce((sum, op) => sum + op.ratio, 0);
      if (Math.abs(totalRatio - 1.0) > 0.01) {
        message.error(`Operation ratios must sum to 1.0 (current: ${totalRatio.toFixed(2)})`);
        return;
      }

      const workload: WorkloadFull = {
        name: values.name,
        description: values.description,
        type: values.type || 'custom',
        operations: operations,
        key_pattern: {
          prefix: values.key_prefix,
          pattern: values.key_pattern,
          key_range: values.key_range,
        },
        data_size: values.data_size_type === 'fixed' 
          ? { fixed: values.data_size_fixed }
          : { min: values.data_size_min, max: values.data_size_max },
        random_data: values.random_data,
        data_offset: values.data_offset || 0,
        expiry_min: values.expiry_enabled ? values.expiry_min : 0,
        expiry_max: values.expiry_enabled ? values.expiry_max : 0,
      };

      if (editingWorkload) {
        await api.updateWorkload(editingWorkload.name, workload);
        message.success('Workload updated');
      } else {
        await api.createWorkload(workload);
        message.success('Workload created');
      }

      setModalOpen(false);
      loadWorkloads();
    } catch (error: any) {
      message.error(error.message || 'Failed to save workload');
    }
  };

  const addOperation = () => {
    setOperations([...operations, { command: 'GET', ratio: 0 }]);
  };

  const removeOperation = (index: number) => {
    setOperations(operations.filter((_, i) => i !== index));
  };

  const updateOperation = (index: number, field: 'command' | 'ratio', value: string | number) => {
    const updated = [...operations];
    updated[index] = { ...updated[index], [field]: value };
    setOperations(updated);
  };

  const normalizeRatios = () => {
    const total = operations.reduce((sum, op) => sum + op.ratio, 0);
    if (total === 0) return;
    const normalized = operations.map(op => ({ ...op, ratio: Math.round((op.ratio / total) * 100) / 100 }));
    // Adjust for rounding errors
    const newTotal = normalized.reduce((sum, op) => sum + op.ratio, 0);
    if (Math.abs(newTotal - 1) > 0.001) {
      normalized[0].ratio += (1 - newTotal);
      normalized[0].ratio = Math.round(normalized[0].ratio * 100) / 100;
    }
    setOperations(normalized);
  };

  // ANN Benchmarks handlers
  const handleCreateAnn = () => {
    setEditingAnnWorkload(null);
    annForm.resetFields();
    annForm.setFieldsValue({
      dataset: 'sift-128-euclidean',
      distance_metric: 'L2',
      dimensions: 128,
      k: 10,
      m: 16,
      ef_construction: 200,
    });
    setAnnModalOpen(true);
  };

  const handleEditAnn = (workload: ANNWorkload) => {
    if (workload.is_builtin) {
      message.warning('Cannot edit built-in workloads. Use duplicate instead.');
      return;
    }
    setEditingAnnWorkload(workload);
    annForm.setFieldsValue(workload);
    setAnnModalOpen(true);
  };

  const handleDuplicateAnn = (workload: ANNWorkload) => {
    setEditingAnnWorkload(null);
    annForm.setFieldsValue({
      ...workload,
      name: `${workload.name}-copy`,
    });
    setAnnModalOpen(true);
  };

  const handleDeleteAnn = async (name: string) => {
    try {
      // TODO: Replace with actual API call when backend supports it
      setAnnWorkloads(annWorkloads.filter(w => w.name !== name));
      message.success('ANN workload deleted');
    } catch (error: unknown) {
      const err = error as { message?: string };
      message.error(err.message || 'Failed to delete ANN workload');
    }
  };

  const handleSaveAnn = async () => {
    try {
      const values = await annForm.validateFields();
      
      const workload: ANNWorkload = {
        name: values.name,
        description: values.description || '',
        dataset: values.dataset,
        distance_metric: values.distance_metric,
        dimensions: values.dimensions,
        k: values.k,
        m: values.m,
        ef_construction: values.ef_construction,
      };

      if (editingAnnWorkload) {
        // TODO: Replace with actual API call
        setAnnWorkloads(annWorkloads.map(w => w.name === editingAnnWorkload.name ? workload : w));
        message.success('ANN workload updated');
      } else {
        // TODO: Replace with actual API call
        setAnnWorkloads([...annWorkloads, workload]);
        message.success('ANN workload created');
      }

      setAnnModalOpen(false);
    } catch (error: unknown) {
      const err = error as { message?: string };
      message.error(err.message || 'Failed to save ANN workload');
    }
  };

  const handleDatasetChange = (dataset: string) => {
    const datasetInfo = ANN_DATASETS.find(d => d.value === dataset);
    if (datasetInfo && datasetInfo.dimensions > 0) {
      annForm.setFieldsValue({ dimensions: datasetInfo.dimensions });
      // Set default distance metric based on dataset name
      if (dataset.includes('angular') || dataset.includes('cosine')) {
        annForm.setFieldsValue({ distance_metric: 'COSINE' });
      } else if (dataset.includes('euclidean')) {
        annForm.setFieldsValue({ distance_metric: 'L2' });
      }
    }
  };

  const columns: ColumnsType<WorkloadFull> = [
    {
      title: 'Name',
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: WorkloadFull) => (
        <Space>
          <ThunderboltOutlined style={{ color: record.is_builtin ? '#1890ff' : '#52c41a' }} />
          <Text strong>{name}</Text>
          {record.is_builtin && (
            <Tooltip title="Built-in workload (read-only)">
              <LockOutlined style={{ color: '#999' }} />
            </Tooltip>
          )}
        </Space>
      ),
    },
    {
      title: 'Description',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: 'Operations',
      key: 'operations',
      width: 250,
      render: (_, record: WorkloadFull) => (
        <Space wrap>
          {record.operations?.map((op, i) => (
            <Tag key={i} color={op.command === 'GET' ? 'blue' : op.command === 'SET' ? 'green' : 'default'}>
              {op.command} ({(op.ratio * 100).toFixed(0)}%)
            </Tag>
          )) || <Text type="secondary">-</Text>}
        </Space>
      ),
    },
    {
      title: 'Key Pattern',
      key: 'key_pattern',
      width: 150,
      render: (_, record: WorkloadFull) => {
        if (!record.key_pattern) return <Text type="secondary">-</Text>;
        return (
          <Space direction="vertical" size={0}>
            <Tag>{record.key_pattern.pattern || 'random'}</Tag>
            <Text type="secondary" style={{ fontSize: 11 }}>
              {record.key_pattern.key_range?.toLocaleString() || '100K'} keys
            </Text>
          </Space>
        );
      },
    },
    {
      title: 'Data Size',
      key: 'data_size',
      width: 120,
      render: (_, record: WorkloadFull) => {
        if (!record.data_size) return <Text type="secondary">-</Text>;
        if (record.data_size.fixed) return `${formatBytes(record.data_size.fixed)}`;
        return `${formatBytes(record.data_size.min || 0)} - ${formatBytes(record.data_size.max || 0)}`;
      },
    },
    {
      title: 'Actions',
      key: 'actions',
      width: 150,
      render: (_, record: WorkloadFull) => (
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
                title="Delete Workload"
                description="Are you sure you want to delete this workload?"
                onConfirm={() => handleDelete(record.name)}
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

  // ANN Benchmarks columns
  const annColumns: ColumnsType<ANNWorkload> = [
    {
      title: 'Name',
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: ANNWorkload) => (
        <Space>
          <RadarChartOutlined style={{ color: record.is_builtin ? '#1890ff' : '#52c41a' }} />
          <Text strong>{name}</Text>
          {record.is_builtin && (
            <Tooltip title="Built-in workload (read-only)">
              <LockOutlined style={{ color: '#999' }} />
            </Tooltip>
          )}
        </Space>
      ),
    },
    {
      title: 'Description',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: 'Dataset',
      dataIndex: 'dataset',
      key: 'dataset',
      width: 180,
      render: (dataset: string) => {
        const info = ANN_DATASETS.find(d => d.value === dataset);
        return <Tag color="purple">{info?.label || dataset}</Tag>;
      },
    },
    {
      title: 'Distance',
      dataIndex: 'distance_metric',
      key: 'distance_metric',
      width: 100,
      render: (metric: string) => <Tag color="cyan">{metric}</Tag>,
    },
    {
      title: 'Dimensions',
      dataIndex: 'dimensions',
      key: 'dimensions',
      width: 100,
      render: (dim: number) => <Text>{dim}D</Text>,
    },
    {
      title: 'K (neighbors)',
      dataIndex: 'k',
      key: 'k',
      width: 100,
    },
    {
      title: 'Index Params',
      key: 'index_params',
      width: 150,
      render: (_, record: ANNWorkload) => (
        <Space direction="vertical" size={0}>
          <Text type="secondary" style={{ fontSize: 12 }}>M: {record.m || 16}</Text>
          <Text type="secondary" style={{ fontSize: 12 }}>ef_c: {record.ef_construction || 200}</Text>
        </Space>
      ),
    },
    {
      title: 'Actions',
      key: 'actions',
      width: 150,
      render: (_, record: ANNWorkload) => (
        <Space>
          <Tooltip title="Duplicate">
            <Button type="text" icon={<CopyOutlined />} onClick={() => handleDuplicateAnn(record)} />
          </Tooltip>
          {!record.is_builtin && (
            <>
              <Tooltip title="Edit">
                <Button type="text" icon={<EditOutlined />} onClick={() => handleEditAnn(record)} />
              </Tooltip>
              <Popconfirm
                title="Delete ANN Workload"
                description="Are you sure you want to delete this workload?"
                onConfirm={() => handleDeleteAnn(record.name)}
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

  // Memtier Pane Content
  const MemtierPane = () => (
    <>
      <Row justify="space-between" align="middle" style={{ marginBottom: 16 }}>
        <Col>
          <Text type="secondary">
            Configure Redis commands, key patterns, and data sizes for memtier_benchmark
          </Text>
        </Col>
        <Col>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
            Create Workload
          </Button>
        </Col>
      </Row>
      <Table
        columns={columns}
        dataSource={workloads}
        rowKey="name"
        loading={loading}
        pagination={false}
        locale={{
          emptyText: (
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description="No memtier workloads found"
            >
              <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
                Create Your First Workload
              </Button>
            </Empty>
          ),
        }}
      />
    </>
  );

  // ANN Benchmarks Pane Content
  const AnnPane = () => (
    <>
      <Row justify="space-between" align="middle" style={{ marginBottom: 16 }}>
        <Col>
          <Text type="secondary">
            Configure vector search workloads for ann_benchmarks
          </Text>
        </Col>
        <Col>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreateAnn}>
            Create ANN Workload
          </Button>
        </Col>
      </Row>
      <Table
        columns={annColumns}
        dataSource={annWorkloads}
        rowKey="name"
        loading={annLoading}
        pagination={false}
        locale={{
          emptyText: (
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description="No ANN workloads found"
            >
              <Button type="primary" icon={<PlusOutlined />} onClick={handleCreateAnn}>
                Create Your First ANN Workload
              </Button>
            </Empty>
          ),
        }}
      />
    </>
  );

  return (
    <div>
      <Row justify="space-between" align="middle" style={{ marginBottom: 24 }}>
        <Col>
          <Title level={3} style={{ color: 'rgba(255,255,255,0.85)', margin: 0 }}>
            <ThunderboltOutlined style={{ marginRight: 8, color: '#faad14' }} />
            Workload Profiles
          </Title>
        </Col>
      </Row>

      <Card style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          items={[
            {
              key: 'memtier',
              label: (
                <span>
                  <ThunderboltOutlined />
                  memtier_benchmark
                </span>
              ),
              children: <MemtierPane />,
            },
            {
              key: 'ann',
              label: (
                <span>
                  <RadarChartOutlined />
                  ann_benchmarks
                </span>
              ),
              children: <AnnPane />,
            },
          ]}
        />
      </Card>

      {/* Memtier Workload Editor Modal */}
      <Modal
        title={editingWorkload ? 'Edit Workload' : 'Create Workload'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={handleSave}
        width={800}
        okText={editingWorkload ? 'Update' : 'Create'}
      >
        <Form form={form} layout="vertical">
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="name"
                label="Workload Name"
                rules={[
                  { required: true, message: 'Please enter a name' },
                  { pattern: /^[a-z0-9-]+$/, message: 'Only lowercase letters, numbers, and hyphens' }
                ]}
              >
                <Input placeholder="my-custom-workload" disabled={!!editingWorkload} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="type" label="Type">
                <Select>
                  <Option value="custom">Custom</Option>
                  <Option value="cache">Cache</Option>
                  <Option value="session">Session</Option>
                  <Option value="bandwidth">Bandwidth</Option>
                  <Option value="latency">Latency</Option>
                </Select>
              </Form.Item>
            </Col>
          </Row>

          <Form.Item name="description" label="Description">
            <TextArea rows={2} placeholder="Describe what this workload tests..." />
          </Form.Item>

          <Divider>Operations</Divider>
          
          <div style={{ marginBottom: 16 }}>
            <Space direction="vertical" style={{ width: '100%' }}>
              {operations.map((op, index) => (
                <Row key={index} gutter={8} align="middle">
                  <Col span={8}>
                    <Select
                      value={op.command}
                      onChange={(v) => updateOperation(index, 'command', v)}
                      style={{ width: '100%' }}
                    >
                      {COMMANDS.map(cmd => (
                        <Option key={cmd} value={cmd}>{cmd}</Option>
                      ))}
                    </Select>
                  </Col>
                  <Col span={12}>
                    <Slider
                      min={0}
                      max={100}
                      value={Math.round(op.ratio * 100)}
                      onChange={(v) => updateOperation(index, 'ratio', v / 100)}
                      tooltip={{ formatter: (v) => `${v}%` }}
                    />
                  </Col>
                  <Col span={2}>
                    <Text>{(op.ratio * 100).toFixed(0)}%</Text>
                  </Col>
                  <Col span={2}>
                    {operations.length > 1 && (
                      <Button
                        type="text"
                        danger
                        icon={<DeleteOutlined />}
                        onClick={() => removeOperation(index)}
                      />
                    )}
                  </Col>
                </Row>
              ))}
            </Space>
            <Space style={{ marginTop: 8 }}>
              <Button type="dashed" icon={<PlusOutlined />} onClick={addOperation}>
                Add Operation
              </Button>
              <Button onClick={normalizeRatios}>
                Normalize to 100%
              </Button>
              <Text type="secondary">
                Total: {(operations.reduce((sum, op) => sum + op.ratio, 0) * 100).toFixed(0)}%
              </Text>
            </Space>
          </div>

          <Divider>Key Pattern</Divider>

          <Row gutter={16}>
            <Col span={8}>
              <Form.Item name="key_prefix" label="Key Prefix">
                <Input placeholder="myapp:" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="key_pattern" label="Pattern">
                <Select>
                  {KEY_PATTERNS.map(p => (
                    <Option key={p} value={p}>{p}</Option>
                  ))}
                </Select>
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="key_range" label="Key Range">
                <InputNumber min={100} max={100000000} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>

          <Divider>Data Size</Divider>

          <Row gutter={16}>
            <Col span={8}>
              <Form.Item name="data_size_type" label="Size Type">
                <Select>
                  <Option value="fixed">Fixed</Option>
                  <Option value="range">Range</Option>
                </Select>
              </Form.Item>
            </Col>
            <Form.Item noStyle shouldUpdate={(prev, curr) => prev.data_size_type !== curr.data_size_type}>
              {({ getFieldValue }) => 
                getFieldValue('data_size_type') === 'fixed' ? (
                  <Col span={8}>
                    <Form.Item name="data_size_fixed" label="Size (bytes)">
                      <InputNumber min={1} max={10485760} style={{ width: '100%' }} />
                    </Form.Item>
                  </Col>
                ) : (
                  <>
                    <Col span={8}>
                      <Form.Item name="data_size_min" label="Min (bytes)">
                        <InputNumber min={1} max={10485760} style={{ width: '100%' }} />
                      </Form.Item>
                    </Col>
                    <Col span={8}>
                      <Form.Item name="data_size_max" label="Max (bytes)">
                        <InputNumber min={1} max={10485760} style={{ width: '100%' }} />
                      </Form.Item>
                    </Col>
                  </>
                )
              }
            </Form.Item>
          </Row>

          <Divider>Data Options</Divider>

          <Row gutter={16}>
            <Col span={8}>
              <Form.Item name="random_data" label="Random Data" valuePropName="checked">
                <Select>
                  <Option value={false}>Compressible (default)</Option>
                  <Option value={true}>Random (incompressible)</Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="data_offset" label="Data Offset" tooltip="Offset in value buffer to start from">
                <InputNumber min={0} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>

          <Divider>Key Expiry (TTL)</Divider>

          <Row gutter={16}>
            <Col span={8}>
              <Form.Item name="expiry_enabled" label="Enable Expiry" valuePropName="checked">
                <Select>
                  <Option value={false}>Disabled</Option>
                  <Option value={true}>Enabled</Option>
                </Select>
              </Form.Item>
            </Col>
            <Form.Item noStyle shouldUpdate={(prev, curr) => prev.expiry_enabled !== curr.expiry_enabled}>
              {({ getFieldValue }) => 
                getFieldValue('expiry_enabled') && (
                  <>
                    <Col span={8}>
                      <Form.Item name="expiry_min" label="Min TTL (seconds)">
                        <InputNumber min={1} style={{ width: '100%' }} />
                      </Form.Item>
                    </Col>
                    <Col span={8}>
                      <Form.Item name="expiry_max" label="Max TTL (seconds)">
                        <InputNumber min={1} style={{ width: '100%' }} />
                      </Form.Item>
                    </Col>
                  </>
                )
              }
            </Form.Item>
          </Row>

          <Text type="secondary" style={{ display: 'block', marginTop: 16 }}>
            💡 Execution settings (threads, clients, duration, pipeline) have moved to <strong>Run Profiles</strong>.
          </Text>
        </Form>
      </Modal>

      {/* ANN Benchmarks Workload Editor Modal */}
      <Modal
        title={editingAnnWorkload ? 'Edit ANN Workload' : 'Create ANN Workload'}
        open={annModalOpen}
        onCancel={() => setAnnModalOpen(false)}
        onOk={handleSaveAnn}
        width={700}
        okText={editingAnnWorkload ? 'Update' : 'Create'}
      >
        <Form form={annForm} layout="vertical">
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="name"
                label="Workload Name"
                rules={[
                  { required: true, message: 'Please enter a name' },
                  { pattern: /^[a-z0-9-]+$/, message: 'Only lowercase letters, numbers, and hyphens' }
                ]}
              >
                <Input placeholder="my-ann-workload" disabled={!!editingAnnWorkload} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="dataset" label="Dataset" rules={[{ required: true }]}>
                <Select onChange={handleDatasetChange}>
                  {ANN_DATASETS.map(d => (
                    <Option key={d.value} value={d.value}>{d.label}</Option>
                  ))}
                </Select>
              </Form.Item>
            </Col>
          </Row>

          <Form.Item name="description" label="Description">
            <TextArea rows={2} placeholder="Describe what this workload tests..." />
          </Form.Item>

          <Divider>Vector Configuration</Divider>

          <Row gutter={16}>
            <Col span={8}>
              <Form.Item name="distance_metric" label="Distance Metric" rules={[{ required: true }]}>
                <Select>
                  {DISTANCE_METRICS.map(m => (
                    <Option key={m.value} value={m.value}>{m.label}</Option>
                  ))}
                </Select>
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="dimensions" label="Dimensions" rules={[{ required: true }]}>
                <InputNumber min={1} max={10000} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="k" label="K (Neighbors)" rules={[{ required: true }]} tooltip="Number of nearest neighbors to return">
                <InputNumber min={1} max={1000} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>

          <Divider>HNSW Index Parameters</Divider>

          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="m" label="M (Max connections per layer)" tooltip="Higher M = better recall but more memory">
                <InputNumber min={2} max={100} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="ef_construction" label="ef_construction" tooltip="Higher = better index quality but slower build">
                <InputNumber min={10} max={1000} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>

          <Text type="secondary" style={{ display: 'block', marginTop: 16 }}>
            💡 Query-time parameters like ef_search are configured in <strong>Run Profiles</strong>.
          </Text>
        </Form>
      </Modal>
    </div>
  );
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes}B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)}KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)}MB`;
}
