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

// Vector DB Benchmark types and constants
interface VectorWorkload {
  name: string;
  description: string;
  dataset: string;
  engine: string;
  distance_metric: string;
  dimensions: number;
  k: number;
  m?: number;
  ef_construction?: number;
  data_type?: string;
  is_builtin?: boolean;
}

// Datasets from redis/vector-db-benchmark (grouped by size/type)
const VECTOR_DATASETS = [
  // Quick tests (small)
  { value: 'random-100', label: 'Random-100 (100 vectors, testing)', dimensions: 100, size: '~1KB', time: '~1 min' },
  // Standard benchmarks (medium)
  { value: 'glove-25-angular', label: 'GloVe-25 (1.2M vectors)', dimensions: 25, size: '121MB', time: '~5 min' },
  { value: 'glove-100-angular', label: 'GloVe-100 (1.2M vectors)', dimensions: 100, size: '463MB', time: '~15 min' },
  { value: 'gist-960-euclidean', label: 'GIST-960 (1M vectors)', dimensions: 960, size: '3.6GB', time: '~30 min' },
  { value: 'deep-image-96-angular', label: 'Deep Image (10M vectors)', dimensions: 96, size: '3.6GB', time: '~45 min' },
  // LAION datasets (large)
  { value: 'laion-small-clip', label: 'LAION Small (100K vectors)', dimensions: 512, size: '200MB', time: '~10 min' },
  { value: 'laion-1m-768', label: 'LAION-1M (1M vectors, 768D)', dimensions: 768, size: '3GB', time: '~1 hour' },
  { value: 'laion-10m-512', label: 'LAION-10M (10M vectors)', dimensions: 512, size: '20GB', time: '~3 hours' },
  // Text embeddings
  { value: 'dbpedia-openai-1m', label: 'DBpedia OpenAI (1M, 1536D)', dimensions: 1536, size: '6GB', time: '~2 hours' },
  // Filtered search
  { value: 'h-and-m-2048-filtered', label: 'H&M Fashion (105K, filtered)', dimensions: 2048, size: '800MB', time: '~20 min' },
  { value: 'arxiv-384-filtered', label: 'ArXiv Papers (2.2M, filtered)', dimensions: 384, size: '3.2GB', time: '~1 hour' },
];

// Redis engines from vector-db-benchmark
const VECTOR_ENGINES = [
  { value: 'redis-default-simple', label: 'Redis RediSearch (default)' },
  { value: 'redis-hnsw-m-16-ef-200', label: 'Redis HNSW (M=16, ef=200)' },
  { value: 'redis-hnsw-m-32-ef-400', label: 'Redis HNSW (M=32, ef=400)' },
  { value: 'vectorsets-fp32-default', label: 'Redis VectorSets (FP32)' },
  { value: 'vectorsets-q8-default', label: 'Redis VectorSets (INT8)' },
];

const DISTANCE_METRICS = [
  { value: 'L2', label: 'L2 (Euclidean)' },
  { value: 'IP', label: 'IP (Inner Product)' },
  { value: 'COSINE', label: 'Cosine Similarity' },
];

const DATA_TYPES = [
  { value: 'FLOAT32', label: 'FLOAT32 (default)' },
  { value: 'FLOAT16', label: 'FLOAT16 (half precision)' },
  { value: 'INT8', label: 'INT8 (quantized)' },
];

export default function Workloads() {
  // Memtier state
  const [workloads, setWorkloads] = useState<WorkloadFull[]>([]);
  const [loading, setLoading] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [editingWorkload, setEditingWorkload] = useState<WorkloadFull | null>(null);
  const [form] = Form.useForm();
  const [operations, setOperations] = useState<Operation[]>([{ command: 'GET', ratio: 0.8 }, { command: 'SET', ratio: 0.2 }]);

  // Vector DB Benchmark state
  const [vectorWorkloads, setVectorWorkloads] = useState<VectorWorkload[]>([]);
  const [vectorLoading, setVectorLoading] = useState(true);
  const [vectorModalOpen, setVectorModalOpen] = useState(false);
  const [editingVectorWorkload, setEditingVectorWorkload] = useState<VectorWorkload | null>(null);
  const [vectorForm] = Form.useForm();

  // Tab state
  const [activeTab, setActiveTab] = useState('memtier');

  useEffect(() => {
    loadWorkloads();
    loadVectorWorkloads();
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

  const loadVectorWorkloads = async () => {
    try {
      // TODO: Replace with actual API call when backend supports it
      // For now, use mock data
      setVectorWorkloads([
        {
          name: 'quick-test',
          description: 'Quick validation with small dataset (~1 min)',
          dataset: 'random-100',
          engine: 'redis-default-simple',
          distance_metric: 'COSINE',
          dimensions: 100,
          k: 10,
          m: 16,
          ef_construction: 200,
          data_type: 'FLOAT32',
          is_builtin: true,
        },
        {
          name: 'glove-standard',
          description: 'GloVe-100 word embeddings (~15 min)',
          dataset: 'glove-100-angular',
          engine: 'redis-hnsw-m-16-ef-200',
          distance_metric: 'COSINE',
          dimensions: 100,
          k: 10,
          m: 16,
          ef_construction: 200,
          data_type: 'FLOAT32',
          is_builtin: true,
        },
        {
          name: 'vectorsets-benchmark',
          description: 'Redis VectorSets with GloVe-25 (~5 min)',
          dataset: 'glove-25-angular',
          engine: 'vectorsets-fp32-default',
          distance_metric: 'COSINE',
          dimensions: 25,
          k: 10,
          m: 16,
          ef_construction: 200,
          data_type: 'FLOAT32',
          is_builtin: true,
        },
      ]);
    } catch (error) {
      console.error('Failed to load vector workloads', error);
    } finally {
      setVectorLoading(false);
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

  // Vector DB Benchmark handlers
  const handleCreateVector = () => {
    setEditingVectorWorkload(null);
    vectorForm.resetFields();
    vectorForm.setFieldsValue({
      dataset: 'random-100',
      engine: 'redis-default-simple',
      distance_metric: 'COSINE',
      dimensions: 100,
      k: 10,
      m: 16,
      ef_construction: 200,
      data_type: 'FLOAT32',
    });
    setVectorModalOpen(true);
  };

  const handleEditVector = (workload: VectorWorkload) => {
    if (workload.is_builtin) {
      message.warning('Cannot edit built-in workloads. Use duplicate instead.');
      return;
    }
    setEditingVectorWorkload(workload);
    vectorForm.setFieldsValue(workload);
    setVectorModalOpen(true);
  };

  const handleDuplicateVector = (workload: VectorWorkload) => {
    setEditingVectorWorkload(null);
    vectorForm.setFieldsValue({
      ...workload,
      name: `${workload.name}-copy`,
    });
    setVectorModalOpen(true);
  };

  const handleDeleteVector = async (name: string) => {
    try {
      // TODO: Replace with actual API call when backend supports it
      setVectorWorkloads(vectorWorkloads.filter(w => w.name !== name));
      message.success('Vector workload deleted');
    } catch (error: unknown) {
      const err = error as { message?: string };
      message.error(err.message || 'Failed to delete vector workload');
    }
  };

  const handleSaveVector = async () => {
    try {
      const values = await vectorForm.validateFields();
      
      const workload: VectorWorkload = {
        name: values.name,
        description: values.description || '',
        dataset: values.dataset,
        engine: values.engine,
        distance_metric: values.distance_metric,
        dimensions: values.dimensions,
        k: values.k,
        m: values.m,
        ef_construction: values.ef_construction,
        data_type: values.data_type,
      };

      if (editingVectorWorkload) {
        // TODO: Replace with actual API call
        setVectorWorkloads(vectorWorkloads.map(w => w.name === editingVectorWorkload.name ? workload : w));
        message.success('Vector workload updated');
      } else {
        // TODO: Replace with actual API call
        setVectorWorkloads([...vectorWorkloads, workload]);
        message.success('Vector workload created');
      }

      setVectorModalOpen(false);
    } catch (error: unknown) {
      const err = error as { message?: string };
      message.error(err.message || 'Failed to save vector workload');
    }
  };

  const handleDatasetChange = (dataset: string) => {
    const datasetInfo = VECTOR_DATASETS.find(d => d.value === dataset);
    if (datasetInfo && datasetInfo.dimensions > 0) {
      vectorForm.setFieldsValue({ dimensions: datasetInfo.dimensions });
      // Set default distance metric based on dataset name
      if (dataset.includes('angular') || dataset.includes('cosine')) {
        vectorForm.setFieldsValue({ distance_metric: 'COSINE' });
      } else if (dataset.includes('euclidean')) {
        vectorForm.setFieldsValue({ distance_metric: 'L2' });
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

  // Vector DB Benchmark columns
  const vectorColumns: ColumnsType<VectorWorkload> = [
    {
      title: 'Name',
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: VectorWorkload) => (
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
        const info = VECTOR_DATASETS.find(d => d.value === dataset);
        return (
          <Tooltip title={info ? `${info.size} · ${info.time}` : ''}>
            <Tag color="purple">{info?.label || dataset}</Tag>
          </Tooltip>
        );
      },
    },
    {
      title: 'Engine',
      dataIndex: 'engine',
      key: 'engine',
      width: 180,
      render: (engine: string) => {
        const info = VECTOR_ENGINES.find(e => e.value === engine);
        return <Tag color="geekblue">{info?.label || engine}</Tag>;
      },
    },
    {
      title: 'Dims',
      dataIndex: 'dimensions',
      key: 'dimensions',
      width: 70,
      render: (dim: number) => <Text>{dim}D</Text>,
    },
    {
      title: 'Index Params',
      key: 'index_params',
      width: 120,
      render: (_, record: VectorWorkload) => (
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
      render: (_, record: VectorWorkload) => (
        <Space>
          <Tooltip title="Duplicate">
            <Button type="text" icon={<CopyOutlined />} onClick={() => handleDuplicateVector(record)} />
          </Tooltip>
          {!record.is_builtin && (
            <>
              <Tooltip title="Edit">
                <Button type="text" icon={<EditOutlined />} onClick={() => handleEditVector(record)} />
              </Tooltip>
              <Popconfirm
                title="Delete Vector Workload"
                description="Are you sure you want to delete this workload?"
                onConfirm={() => handleDeleteVector(record.name)}
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

  // Vector DB Benchmark Pane Content
  const VectorPane = () => (
    <>
      <Row justify="space-between" align="middle" style={{ marginBottom: 16 }}>
        <Col>
          <Text type="secondary">
            Configure vector search workloads using redis/vector-db-benchmark (Docker)
          </Text>
        </Col>
        <Col>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreateVector}>
            Create Vector Workload
          </Button>
        </Col>
      </Row>
      <Table
        columns={vectorColumns}
        dataSource={vectorWorkloads}
        rowKey="name"
        loading={vectorLoading}
        pagination={false}
        locale={{
          emptyText: (
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description="No vector workloads found"
            >
              <Button type="primary" icon={<PlusOutlined />} onClick={handleCreateVector}>
                Create Your First Vector Workload
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
              key: 'vector',
              label: (
                <span>
                  <RadarChartOutlined />
                  vector-db-benchmark
                </span>
              ),
              children: <VectorPane />,
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

      {/* Vector DB Benchmark Workload Editor Modal */}
      <Modal
        title={editingVectorWorkload ? 'Edit Vector Workload' : 'Create Vector Workload'}
        open={vectorModalOpen}
        onCancel={() => setVectorModalOpen(false)}
        onOk={handleSaveVector}
        width={800}
        okText={editingVectorWorkload ? 'Update' : 'Create'}
      >
        <Form form={vectorForm} layout="vertical">
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
                <Input placeholder="my-vector-workload" disabled={!!editingVectorWorkload} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="engine" label="Redis Engine" rules={[{ required: true }]}>
                <Select>
                  {VECTOR_ENGINES.map(e => (
                    <Option key={e.value} value={e.value}>{e.label}</Option>
                  ))}
                </Select>
              </Form.Item>
            </Col>
          </Row>

          <Form.Item name="description" label="Description">
            <TextArea rows={2} placeholder="Describe what this workload tests..." />
          </Form.Item>

          <Divider>Dataset</Divider>

          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="dataset" label="Dataset" rules={[{ required: true }]}>
                <Select onChange={handleDatasetChange}>
                  {VECTOR_DATASETS.map(d => (
                    <Option key={d.value} value={d.value}>
                      {d.label} ({d.time})
                    </Option>
                  ))}
                </Select>
              </Form.Item>
            </Col>
            <Col span={6}>
              <Form.Item name="dimensions" label="Dimensions" rules={[{ required: true }]}>
                <InputNumber min={1} max={10000} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={6}>
              <Form.Item name="k" label="K (Neighbors)" rules={[{ required: true }]} tooltip="Number of nearest neighbors to return">
                <InputNumber min={1} max={1000} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>

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
              <Form.Item name="data_type" label="Data Type">
                <Select>
                  {DATA_TYPES.map(t => (
                    <Option key={t.value} value={t.value}>{t.label}</Option>
                  ))}
                </Select>
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
            💡 Search-time parameters (ef_search, parallelism) are configured in <strong>Run Profiles</strong>.
          </Text>
          <Text type="secondary" style={{ display: 'block', marginTop: 8 }}>
            🐳 Runs via: <code>docker run redis/vector-db-benchmark:latest</code>
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
