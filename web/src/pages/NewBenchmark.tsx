import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams, Link } from 'react-router-dom';
import {
  Card,
  Form,
  Input,
  InputNumber,
  Select,
  Button,
  Space,
  Typography,
  Row,
  Col,
  Divider,
  message,
  Switch,
  Progress,
  Steps,
  Tag,
  Alert,
  Descriptions,
  Spin,
  Tooltip,
  Empty,
  Radio,
  Badge,
} from 'antd';
import {
  ThunderboltOutlined,
  SettingOutlined,
  CheckCircleOutlined,
  LoadingOutlined,
  CloudOutlined,
  DownOutlined,
  RightOutlined,
  SearchOutlined,
  DatabaseOutlined,
  ExperimentOutlined,
  RadarChartOutlined,
  PlusOutlined,
  GlobalOutlined,
  SafetyCertificateOutlined,
} from '@ant-design/icons';
import api from '@/api/client';
import { 
  Workload, 
  Infrastructure, 
  BenchmarkToolType, 
  BenchmarkTool,
} from '@/types';
import useWebSocket from '@/hooks/useWebSocket';

// Define RunProfile type locally until types are consolidated
interface RunProfile {
  name: string;
  description?: string;
  threads: number;
  clients: number;
  duration?: string;
  requests?: number;
  pipeline: number;
  rate_limit?: number;
  protocol?: string;
  run_count?: number;
  is_builtin?: boolean;
}

// Benchmark tools configuration
const BENCHMARK_TOOLS: BenchmarkTool[] = [
  {
    id: 'memtier_benchmark',
    name: 'memtier_benchmark',
    description: 'Classic Redis performance testing for GET/SET and basic commands. Ideal for throughput and latency measurements.',
    icon: 'database',
    category: 'core',
    supported_workloads: ['cache', 'write-heavy', 'read-only', 'mixed', 'session', 'large-values'],
    available: true,
    documentation_url: 'https://github.com/RedisLabs/memtier_benchmark',
  },
  {
    id: 'ann_benchmarks',
    name: 'ANN Benchmarks',
    description: 'Industry-standard vector search benchmarking. Measures recall vs QPS tradeoffs for HNSW and other ANN algorithms.',
    icon: 'radar',
    category: 'vector',
    supported_workloads: ['vector-search', 'hnsw', 'similarity'],
    requires_module: ['search'],
    available: true,
    documentation_url: 'https://github.com/erikbern/ann-benchmarks',
  },
  {
    id: 'ftsb',
    name: 'FTSB (Full-Text Search)',
    description: 'Full-text search benchmarking for RediSearch. Tests FT.SEARCH, FT.AGGREGATE, and FT.ADD performance.',
    icon: 'search',
    category: 'search',
    supported_workloads: ['search', 'full-text', 'aggregation'],
    requires_module: ['search'],
    available: false, // Coming soon
    documentation_url: 'https://github.com/RediSearch/ftsb',
  },
  {
    id: 'vectordb_bench',
    name: 'VectorDB Bench',
    description: 'Comprehensive vector database benchmarking with production-like scenarios. Supports insertion, search, and filtered search.',
    icon: 'experiment',
    category: 'vector',
    supported_workloads: ['vector-search', 'vector-insert', 'filtered-search'],
    requires_module: ['search'],
    available: false, // Coming soon
    documentation_url: 'https://github.com/zilliztech/VectorDBBench',
  },
];

// Get icon component for tool category
const getToolIcon = (tool: BenchmarkTool) => {
  switch (tool.icon) {
    case 'database': return <DatabaseOutlined />;
    case 'radar': return <RadarChartOutlined />;
    case 'search': return <SearchOutlined />;
    case 'experiment': return <ExperimentOutlined />;
    default: return <ThunderboltOutlined />;
  }
};

// Get category color
const getCategoryColor = (category: BenchmarkTool['category']) => {
  switch (category) {
    case 'core': return 'blue';
    case 'search': return 'green';
    case 'vector': return 'purple';
    default: return 'default';
  }
};

const { Title, Text, Paragraph } = Typography;
const { Option } = Select;

interface FormValues {
  name: string;
  description?: string;
  workload: string;
  run_profile: string;
  // Override settings (optional, for when custom overrides are needed)
  threads?: number;
  clients?: number;
  duration?: string;
  requests?: number;
  tags?: string;
  // ANN Benchmarks config
  ann_dataset?: string;
  ann_k?: number;
  ann_ef_construction?: number;
  ann_ef_search?: number;
  ann_m?: number;
  // FTSB config
  ftsb_use_case?: string;
  ftsb_doc_count?: number;
  ftsb_query_count?: number;
  ftsb_workers?: number;
  ftsb_pipeline?: number;
  ftsb_cluster_mode?: boolean;
  // VectorDB Bench config
  vdb_case_type?: string;
  vdb_k?: number;
  vdb_concurrency?: string;
  vdb_m?: number;
  vdb_ef_construction?: number;
  vdb_ef_search?: number;
  vdb_drop_old?: boolean;
  vdb_load?: boolean;
}

export default function NewBenchmark() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [form] = Form.useForm();
  const [workloads, setWorkloads] = useState<Workload[]>([]);
  const [runProfiles, setRunProfiles] = useState<RunProfile[]>([]);
  const [loading, setLoading] = useState(false);
  const [running, setRunning] = useState(false);
  const [runId, setRunId] = useState<string | null>(null);
  const [progress, setProgress] = useState(0);
  const [currentStep, setCurrentStep] = useState(0);
  const [showOverrides, setShowOverrides] = useState(false);
  
  // Benchmark tool selection
  const [selectedTool, setSelectedTool] = useState<BenchmarkToolType>('memtier_benchmark');
  
  // Watch form values for reactive display
  const selectedWorkloadName = Form.useWatch('workload', form);
  const selectedRunProfileName = Form.useWatch('run_profile', form);
  
  // Infrastructure selection
  const infraIdFromUrl = searchParams.get('infra');
  const [infrastructures, setInfrastructures] = useState<Infrastructure[]>([]);
  const [selectedInfraId, setSelectedInfraId] = useState<string | null>(infraIdFromUrl);
  const [infraLoading, setInfraLoading] = useState(true);
  
  // Selected infrastructure object
  const selectedInfra = infrastructures.find(i => i.id === selectedInfraId) || null;

  // WebSocket for real-time progress
  const { lastMessage } = useWebSocket(running ? runId : null);

  useEffect(() => {
    loadWorkloads();
    loadRunProfiles();
    loadInfrastructures();
  }, []);

  useEffect(() => {
    if (lastMessage?.type === 'progress') {
      setProgress(lastMessage.progress || 0);
    }
    if (lastMessage?.type === 'completed') {
      setCurrentStep(2);
      setRunning(false);
      message.success('Benchmark completed!');
      setTimeout(() => {
        navigate(`/runs/${runId}`);
      }, 1500);
    }
    if (lastMessage?.type === 'error') {
      message.error(lastMessage.error || 'Benchmark failed');
      setRunning(false);
    }
  }, [lastMessage, runId, navigate]);

  const loadWorkloads = async () => {
    try {
      const data = await api.getWorkloads();
      setWorkloads(data);
    } catch (error) {
      console.error('Failed to load workloads');
    }
  };

  const loadRunProfiles = async () => {
    try {
      const data = await api.getRunProfilesFull();
      setRunProfiles(data);
    } catch (error) {
      console.error('Failed to load run profiles');
    }
  };

  const loadInfrastructures = async () => {
    setInfraLoading(true);
    try {
      const data = await api.getInfrastructures();
      // Only show ready infrastructures
      const readyInfras = data.filter((i: Infrastructure) => i.status === 'ready');
      setInfrastructures(readyInfras);
      
      // If URL has infra param, select it
      if (infraIdFromUrl && readyInfras.some((i: Infrastructure) => i.id === infraIdFromUrl)) {
        setSelectedInfraId(infraIdFromUrl);
      }
    } catch (error) {
      console.error('Failed to load infrastructures');
    } finally {
      setInfraLoading(false);
    }
  };

  const onFinish = async (values: FormValues) => {
    if (!selectedInfra) {
      message.error('Please select an infrastructure');
      return;
    }
    
    setLoading(true);
    setCurrentStep(1);
    
    try {
      // Build tool-specific configuration
      let toolConfig: Record<string, unknown> = {};
      
      if (selectedTool === 'ann_benchmarks') {
        toolConfig = {
          dataset: values.ann_dataset,
          k: values.ann_k,
          ef_construction: values.ann_ef_construction,
          ef_search: values.ann_ef_search,
          m: values.ann_m,
        };
      } else if (selectedTool === 'ftsb') {
        toolConfig = {
          use_case: values.ftsb_use_case,
          doc_count: values.ftsb_doc_count,
          query_count: values.ftsb_query_count,
          workers: values.ftsb_workers,
          pipeline: values.ftsb_pipeline,
          cluster_mode: values.ftsb_cluster_mode,
        };
      } else if (selectedTool === 'vectordb_bench') {
        toolConfig = {
          case_type: values.vdb_case_type,
          k: values.vdb_k,
          concurrency: values.vdb_concurrency?.split(',').map(s => parseInt(s.trim())).filter(n => !isNaN(n)),
          m: values.vdb_m,
          ef_construction: values.vdb_ef_construction,
          ef_search: values.vdb_ef_search,
          drop_old: values.vdb_drop_old,
          load: values.vdb_load,
        };
      }

      // Infrastructure-based benchmark
      const result = await api.runCloudBenchmark({
        infrastructure_id: selectedInfra.id,
        name: values.name,
        description: values.description,
        tool: selectedTool,
        workload: values.workload,
        run_profile: values.run_profile,
        threads: values.threads,
        clients: values.clients,
        requests: values.requests || 100000,
        tool_config: selectedTool !== 'memtier_benchmark' ? toolConfig : undefined,
        tags: values.tags?.split(',').map(t => t.trim()).filter(Boolean),
      });
      
      setRunId(result.benchmark_id || result.id);
      setRunning(true);
      message.success(`Benchmark started on ${selectedInfra.name}!`);
      
      // Poll for completion since cloud benchmarks are async
      pollCloudBenchmarkStatus(result.benchmark_id || result.id);
    } catch (error: any) {
      message.error(error.message || 'Failed to start benchmark');
      setCurrentStep(0);
    } finally {
      setLoading(false);
    }
  };

  const pollCloudBenchmarkStatus = async (benchmarkId: string) => {
    const poll = async () => {
      try {
        const status = await api.getBenchmarkStatus(benchmarkId);
        setProgress(status.progress || 0);
        
        if (status.status === 'completed') {
          setCurrentStep(2);
          setRunning(false);
          message.success('Cloud benchmark completed!');
          setTimeout(() => {
            navigate(`/runs/${benchmarkId}`);
          }, 1500);
          return;
        }
        
        if (status.status === 'failed') {
          message.error('Cloud benchmark failed');
          setRunning(false);
          setCurrentStep(0);
          return;
        }
        
        // Continue polling
        setTimeout(poll, 2000);
      } catch (error) {
        // Keep polling on error
        setTimeout(poll, 5000);
      }
    };
    
    poll();
  };

  const selectedWorkload = workloads.find(w => w.name === selectedWorkloadName);
  const selectedRunProfile = runProfiles.find(p => p.name === selectedRunProfileName);

  // Show loading spinner while loading infrastructure
  if (infraLoading) {
    return (
      <div style={{ textAlign: 'center', padding: 100 }}>
        <Spin size="large" />
        <p style={{ marginTop: 16, color: 'rgba(255,255,255,0.65)' }}>Loading infrastructures...</p>
      </div>
    );
  }

  return (
    <div>
      <Title level={3} style={{ color: 'rgba(255,255,255,0.85)', marginBottom: 24 }}>
        <ThunderboltOutlined style={{ marginRight: 8, color: '#DC382D' }} />
        New Benchmark
      </Title>

      {/* Infrastructure Selection */}
      <Card 
        title={
          <Space>
            <CloudOutlined />
            <span>Select Infrastructure</span>
          </Space>
        }
        style={{ marginBottom: 24 }}
      >
        {infrastructures.length === 0 ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={
              <Space direction="vertical" align="center">
                <Text type="secondary">No infrastructure configured yet</Text>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  Set up an infrastructure to run benchmarks against Redis clusters
                </Text>
              </Space>
            }
          >
            <Button 
              type="primary" 
              icon={<PlusOutlined />}
              onClick={() => navigate('/infrastructure/new')}
            >
              Manage Infrastructure
            </Button>
          </Empty>
        ) : (
          <>
            <Radio.Group 
              value={selectedInfraId} 
              onChange={(e) => setSelectedInfraId(e.target.value)}
              style={{ width: '100%' }}
            >
              <Row gutter={[16, 16]}>
                {infrastructures.map((infra) => (
                  <Col xs={24} sm={12} lg={8} key={infra.id}>
                    <Card
                      hoverable
                      size="small"
                      onClick={() => setSelectedInfraId(infra.id)}
                      style={{
                        border: selectedInfraId === infra.id ? '2px solid #DC382D' : '1px solid #333333',
                        background: selectedInfraId === infra.id ? 'rgba(220, 56, 45, 0.15)' : '#1a1a1a',
                        cursor: 'pointer',
                      }}
                      bodyStyle={{ padding: 16 }}
                    >
                      <Radio value={infra.id} style={{ width: '100%' }}>
                        <Space direction="vertical" size={4} style={{ width: '100%' }}>
                          <Space>
                            <Text strong>{infra.name}</Text>
                            <Badge status="success" />
                          </Space>
                          <Space wrap size={4}>
                            <Tag color="blue">{infra.provider}</Tag>
                            <Tag><GlobalOutlined /> {infra.region}</Tag>
                            {infra.config?.amr?.sku && (
                              <Tag color="purple">{infra.config.amr.sku}</Tag>
                            )}
                          </Space>
                          {infra.outputs?.redis_hostname && (
                            <Text type="secondary" style={{ fontSize: 11 }}>
                              <SafetyCertificateOutlined style={{ marginRight: 4 }} />
                              {infra.outputs.redis_hostname}
                            </Text>
                          )}
                        </Space>
                      </Radio>
                    </Card>
                  </Col>
                ))}
              </Row>
            </Radio.Group>
            
            <Divider style={{ margin: '16px 0' }} />
            
            <Button 
              type="dashed" 
              icon={<PlusOutlined />}
              onClick={() => navigate('/infrastructure/new')}
            >
              Add Infrastructure
            </Button>
          </>
        )}
      </Card>

      {/* Selected Infrastructure Info */}
      {selectedInfra && (
        <Alert
          type="success"
          icon={<CheckCircleOutlined />}
          message={`Infrastructure: ${selectedInfra.name}`}
          description={
            <Descriptions size="small" column={2} style={{ marginTop: 8 }}>
              <Descriptions.Item label="Provider">{selectedInfra.provider}</Descriptions.Item>
              <Descriptions.Item label="Region">{selectedInfra.region}</Descriptions.Item>
              {selectedInfra.config?.amr?.sku && (
                <Descriptions.Item label="SKU">{selectedInfra.config.amr.sku}</Descriptions.Item>
              )}
              {selectedInfra.outputs?.redis_hostname && (
                <Descriptions.Item label="Host">{selectedInfra.outputs.redis_hostname}</Descriptions.Item>
              )}
              {selectedInfra.outputs?.runner_ips && (
                <Descriptions.Item label="Runners">{selectedInfra.outputs.runner_ips.length} VMs</Descriptions.Item>
              )}
            </Descriptions>
          }
          showIcon
          style={{ marginBottom: 24 }}
        />
      )}

      {/* Benchmark Tool Selection */}
      <Card 
        title={
          <Space>
            <ExperimentOutlined />
            <span>Benchmark Tool</span>
          </Space>
        }
        style={{ marginBottom: 24 }}
      >
        <Row gutter={[16, 16]}>
          {BENCHMARK_TOOLS.map((tool) => (
            <Col xs={24} sm={12} lg={8} xl={6} key={tool.id}>
              <Card
                hoverable={tool.available}
                size="small"
                onClick={() => tool.available && setSelectedTool(tool.id)}
                style={{
                  border: selectedTool === tool.id ? '2px solid #DC382D' : '1px solid #333333',
                  background: selectedTool === tool.id ? 'rgba(220, 56, 45, 0.15)' : tool.available ? '#1a1a1a' : '#0d0d0d',
                  cursor: tool.available ? 'pointer' : 'not-allowed',
                  height: '100%',
                  opacity: tool.available ? 1 : 0.7,
                }}
                bodyStyle={{ padding: 16 }}
              >
                <Space direction="vertical" size={8} style={{ width: '100%' }}>
                  <Space>
                    <span style={{ 
                      fontSize: 24, 
                      color: selectedTool === tool.id ? '#DC382D' : 'rgba(255,255,255,0.65)' 
                    }}>
                      {getToolIcon(tool)}
                    </span>
                    <Text strong style={{ 
                      color: selectedTool === tool.id ? '#DC382D' : 'inherit' 
                    }}>
                      {tool.name}
                    </Text>
                    {!tool.available && (
                      <Tag color="orange">Coming Soon</Tag>
                    )}
                  </Space>
                  <Tag color={getCategoryColor(tool.category)}>{tool.category}</Tag>
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    {tool.description}
                  </Text>
                  {tool.requires_module && (
                    <Space wrap size={4}>
                      {tool.requires_module.map(mod => (
                        <Tooltip key={mod} title={`Requires Redis ${mod} module`}>
                          <Tag color="geekblue" style={{ fontSize: 10 }}>{mod}</Tag>
                        </Tooltip>
                      ))}
                    </Space>
                  )}
                  {tool.documentation_url && (
                    <a 
                      href={tool.documentation_url} 
                      target="_blank" 
                      rel="noopener noreferrer"
                      onClick={(e) => e.stopPropagation()}
                      style={{ fontSize: 11 }}
                    >
                      Documentation →
                    </a>
                  )}
                </Space>
              </Card>
            </Col>
          ))}
        </Row>
        
        {selectedTool !== 'memtier_benchmark' && (
          <Alert
            type="info"
            message={`${BENCHMARK_TOOLS.find(t => t.id === selectedTool)?.name} Configuration`}
            description={
              <Text type="secondary" style={{ fontSize: 13 }}>
                {selectedTool === 'ann_benchmarks' && 
                  'ANN Benchmarks requires pre-configured datasets. Configure algorithm parameters and dataset selection below.'}
                {selectedTool === 'ftsb' && 
                  'FTSB will generate test data and queries based on the selected use case. Ensure RediSearch module is loaded.'}
                {selectedTool === 'vectordb_bench' && 
                  'VectorDB Bench provides comprehensive vector search testing with various dataset sizes and configurations.'}
              </Text>
            }
            showIcon
            style={{ marginTop: 16 }}
          />
        )}
      </Card>

      {/* Progress Steps */}
      <Card style={{ marginBottom: 24 }}>
        <Steps
          current={currentStep}
          items={[
            {
              title: 'Configure',
              icon: currentStep === 0 ? <SettingOutlined /> : <CheckCircleOutlined />,
            },
            {
              title: 'Running',
              icon: currentStep === 1 ? <LoadingOutlined /> : currentStep > 1 ? <CheckCircleOutlined /> : undefined,
            },
            {
              title: 'Complete',
              icon: currentStep === 2 ? <CheckCircleOutlined style={{ color: '#52c41a' }} /> : undefined,
            },
          ]}
        />
        
        {running && (
          <div style={{ marginTop: 24, textAlign: 'center' }}>
            <Progress
              type="circle"
              percent={progress}
              strokeColor="#DC382D"
              format={(pct) => `${pct}%`}
            />
            <Paragraph style={{ marginTop: 16 }}>
              <Text type="secondary">Benchmark in progress...</Text>
            </Paragraph>
          </div>
        )}
      </Card>

      {/* Configuration Form */}
      {currentStep === 0 && (
        <Form
          form={form}
          layout="vertical"
          onFinish={onFinish}
          initialValues={{
            run_profile: 'default',
            requests: 100000,
          }}
        >
          <Row gutter={24}>
            {/* Basic Info */}
            <Col xs={24} lg={12}>
              <Card 
                title="Basic Information" 
                style={{ marginBottom: 24 }}
              >
                <Form.Item
                  name="name"
                  label="Benchmark Name"
                  rules={[{ required: true, message: 'Please enter a name' }]}
                >
                  <Input placeholder="e.g., Production Baseline Test" />
                </Form.Item>

                <Form.Item
                  name="description"
                  label="Description"
                >
                  <Input.TextArea rows={2} placeholder="Optional description" />
                </Form.Item>

                <Form.Item
                  name="tags"
                  label="Tags"
                >
                  <Input placeholder="Comma-separated tags (e.g., production, baseline)" />
                </Form.Item>
              </Card>
            </Col>

            {/* Workload - Only for memtier_benchmark */}
            {selectedTool === 'memtier_benchmark' && (
            <Col xs={24} lg={12}>
              <Card 
                title="Workload" 
                style={{ marginBottom: 24 }}
              >
                <Form.Item
                  name="workload"
                  label="Workload Profile"
                  rules={[{ required: selectedTool === 'memtier_benchmark', message: 'Please select a workload' }]}
                >
                  <Select placeholder="Select workload">
                    {workloads.map((w) => (
                      <Option key={w.name} value={w.name}>
                        {w.name}
                      </Option>
                    ))}
                  </Select>
                </Form.Item>

                {selectedWorkload && (
                  <div style={{ marginTop: 16 }}>
                    <Text type="secondary">Operations:</Text>
                    <div style={{ marginTop: 8 }}>
                      {selectedWorkload.operations?.map((op) => (
                        <Tag key={op.command} style={{ marginBottom: 4 }}>
                          {op.command} ({(op.ratio * 100).toFixed(0)}%)
                        </Tag>
                      ))}
                    </div>
                  </div>
                )}
              </Card>
            </Col>
            )}

            {/* Performance Settings - Only for memtier_benchmark */}
            {selectedTool === 'memtier_benchmark' && (
            <Col xs={24} lg={12}>
              <Card 
                title="Run Profile" 
                extra={<Link to="/run-profiles"><SettingOutlined /> Manage</Link>}
                style={{ marginBottom: 24 }}
              >
                <Form.Item
                  name="run_profile"
                  label="Run Profile"
                  rules={[{ required: true, message: 'Please select a run profile' }]}
                  extra="Controls execution settings: threads, clients, duration, pipeline"
                >
                  <Select placeholder="Select run profile">
                    {runProfiles.map((p) => (
                      <Option key={p.name} value={p.name}>
                        {p.name} {p.is_builtin && <Tag>builtin</Tag>}
                      </Option>
                    ))}
                  </Select>
                </Form.Item>

                {selectedRunProfile && (
                  <div style={{ marginTop: 8, padding: 12, background: '#f5f5f5', borderRadius: 6 }}>
                    <Row gutter={16}>
                      <Col span={12}>
                        <Text type="secondary">Threads:</Text> <Text strong>{selectedRunProfile.threads}</Text>
                      </Col>
                      <Col span={12}>
                        <Text type="secondary">Clients:</Text> <Text strong>{selectedRunProfile.clients}</Text>
                      </Col>
                    </Row>
                    <Row gutter={16} style={{ marginTop: 8 }}>
                      <Col span={12}>
                        {selectedRunProfile.duration ? (
                          <><Text type="secondary">Duration:</Text> <Text strong>{selectedRunProfile.duration}</Text></>
                        ) : (
                          <><Text type="secondary">Requests:</Text> <Text strong>{selectedRunProfile.requests?.toLocaleString()}</Text></>
                        )}
                      </Col>
                      <Col span={12}>
                        <Text type="secondary">Pipeline:</Text> <Text strong>{selectedRunProfile.pipeline}</Text>
                      </Col>
                    </Row>
                    {selectedRunProfile.description && (
                      <div style={{ marginTop: 8, borderTop: '1px solid #e8e8e8', paddingTop: 8 }}>
                        <Text type="secondary" style={{ fontSize: 12 }}>{selectedRunProfile.description}</Text>
                      </div>
                    )}
                  </div>
                )}

                {/* Override Settings (Collapsible) */}
                <div style={{ marginTop: 16 }}>
                  <Button 
                    type="link" 
                    onClick={() => setShowOverrides(!showOverrides)}
                    style={{ padding: 0 }}
                    icon={showOverrides ? <DownOutlined /> : <RightOutlined />}
                  >
                    {showOverrides ? 'Hide' : 'Show'} Override Settings
                  </Button>
                  
                  {showOverrides && (
                    <div style={{ marginTop: 12, padding: 12, background: 'rgba(220, 56, 45, 0.1)', border: '1px solid rgba(220, 56, 45, 0.3)', borderRadius: 6 }}>
                      <Alert 
                        message="Override run profile settings for this benchmark only" 
                        type="warning" 
                        showIcon 
                        style={{ marginBottom: 12 }}
                      />
                      <Row gutter={16}>
                        <Col span={12}>
                          <Form.Item name="threads" label="Threads" style={{ marginBottom: 8 }}>
                            <InputNumber min={1} max={64} style={{ width: '100%' }} placeholder="Use profile" />
                          </Form.Item>
                        </Col>
                        <Col span={12}>
                          <Form.Item name="clients" label="Clients" style={{ marginBottom: 8 }}>
                            <InputNumber min={1} max={1000} style={{ width: '100%' }} placeholder="Use profile" />
                          </Form.Item>
                        </Col>
                      </Row>
                      {selectedInfra ? (
                        <Form.Item name="requests" label="Requests" style={{ marginBottom: 0 }}>
                          <InputNumber min={1000} style={{ width: '100%' }} placeholder="Use profile" />
                        </Form.Item>
                      ) : (
                        <Form.Item name="duration" label="Duration" style={{ marginBottom: 0 }}>
                          <Input placeholder="Use profile (e.g., 30s, 5m)" />
                        </Form.Item>
                      )}
                    </div>
                  )}
                </div>
              </Card>
            </Col>
            )}

            {/* ANN Benchmarks Configuration */}
            {selectedTool === 'ann_benchmarks' && (
              <Col xs={24}>
                <Card 
                  title={<><RadarChartOutlined /> ANN Benchmarks Configuration</>}
                  style={{ marginBottom: 24 }}
                >
                  <Row gutter={24}>
                    <Col xs={24} lg={12}>
                      <Form.Item
                        name="ann_dataset"
                        label="Dataset"
                        rules={[{ required: selectedTool === 'ann_benchmarks', message: 'Please select a dataset' }]}
                        extra="Pre-configured datasets with ground truth for recall measurement"
                      >
                        <Select placeholder="Select dataset">
                          <Option value="glove-100-angular">GloVe-100 (Angular, 1.2M vectors)</Option>
                          <Option value="sift-128-euclidean">SIFT-128 (Euclidean, 1M vectors)</Option>
                          <Option value="gist-960-euclidean">GIST-960 (Euclidean, 1M vectors)</Option>
                          <Option value="fashion-mnist-784-euclidean">Fashion-MNIST (Euclidean, 60K vectors)</Option>
                          <Option value="nytimes-256-angular">NYTimes-256 (Angular, 290K vectors)</Option>
                          <Option value="deep1b-96-angular">DEEP1B-96 (Angular, 10M vectors)</Option>
                        </Select>
                      </Form.Item>

                      <Form.Item
                        name="ann_k"
                        label="K (Nearest Neighbors)"
                        initialValue={10}
                        extra="Number of nearest neighbors to retrieve"
                      >
                        <InputNumber min={1} max={1000} style={{ width: '100%' }} />
                      </Form.Item>
                    </Col>

                    <Col xs={24} lg={12}>
                      <Form.Item
                        name="ann_ef_construction"
                        label="ef_construction"
                        initialValue={200}
                        extra="HNSW index build parameter (higher = better recall, slower build)"
                      >
                        <InputNumber min={50} max={1000} style={{ width: '100%' }} />
                      </Form.Item>

                      <Form.Item
                        name="ann_ef_search"
                        label="ef_search"
                        initialValue={100}
                        extra="HNSW search parameter (higher = better recall, slower search)"
                      >
                        <InputNumber min={10} max={500} style={{ width: '100%' }} />
                      </Form.Item>

                      <Form.Item
                        name="ann_m"
                        label="M (Max Connections)"
                        initialValue={16}
                        extra="HNSW parameter: max number of connections per node"
                      >
                        <InputNumber min={4} max={64} style={{ width: '100%' }} />
                      </Form.Item>
                    </Col>
                  </Row>
                </Card>
              </Col>
            )}

            {/* FTSB Configuration */}
            {selectedTool === 'ftsb' && (
              <Col xs={24}>
                <Card 
                  title={<><SearchOutlined /> FTSB Configuration</>}
                  style={{ marginBottom: 24 }}
                >
                  <Row gutter={24}>
                    <Col xs={24} lg={12}>
                      <Form.Item
                        name="ftsb_use_case"
                        label="Use Case"
                        rules={[{ required: selectedTool === 'ftsb', message: 'Please select a use case' }]}
                        extra="Pre-defined benchmark scenarios with data generation"
                      >
                        <Select placeholder="Select use case">
                          <Option value="enwiki_abstract">Wikipedia Abstracts (Full-text search)</Option>
                          <Option value="enwiki_pages">Wikipedia Pages (Large documents)</Option>
                          <Option value="nyc_taxis">NYC Taxis (Write performance)</Option>
                          <Option value="ecommerce_inventory">E-commerce Inventory (Aggregations)</Option>
                        </Select>
                      </Form.Item>

                      <Form.Item
                        name="ftsb_doc_count"
                        label="Document Count"
                        initialValue={100000}
                        extra="Number of documents to generate and index"
                      >
                        <InputNumber min={1000} max={10000000} style={{ width: '100%' }} />
                      </Form.Item>

                      <Form.Item
                        name="ftsb_query_count"
                        label="Query Count"
                        initialValue={10000}
                        extra="Number of search queries to execute"
                      >
                        <InputNumber min={100} max={1000000} style={{ width: '100%' }} />
                      </Form.Item>
                    </Col>

                    <Col xs={24} lg={12}>
                      <Form.Item
                        name="ftsb_workers"
                        label="Workers"
                        initialValue={4}
                        extra="Concurrent workers executing queries"
                      >
                        <InputNumber min={1} max={64} style={{ width: '100%' }} />
                      </Form.Item>

                      <Form.Item
                        name="ftsb_pipeline"
                        label="Pipeline"
                        initialValue={1}
                        extra="Number of commands to pipeline"
                      >
                        <InputNumber min={1} max={100} style={{ width: '100%' }} />
                      </Form.Item>

                      <Form.Item
                        name="ftsb_cluster_mode"
                        label="Cluster Mode"
                        valuePropName="checked"
                        extra="Enable for Redis Cluster deployments"
                      >
                        <Switch />
                      </Form.Item>
                    </Col>
                  </Row>
                </Card>
              </Col>
            )}

            {/* VectorDB Bench Configuration */}
            {selectedTool === 'vectordb_bench' && (
              <Col xs={24}>
                <Card 
                  title={<><ExperimentOutlined /> VectorDB Bench Configuration</>}
                  style={{ marginBottom: 24 }}
                >
                  <Row gutter={24}>
                    <Col xs={24} lg={12}>
                      <Form.Item
                        name="vdb_case_type"
                        label="Test Case"
                        rules={[{ required: selectedTool === 'vectordb_bench', message: 'Please select a test case' }]}
                        extra="Performance test configurations with varying dataset sizes"
                      >
                        <Select placeholder="Select test case">
                          <Option value="Performance768D1M">768D × 1M vectors (Medium)</Option>
                          <Option value="Performance768D10M">768D × 10M vectors (Large)</Option>
                          <Option value="Performance1536D500K">1536D × 500K vectors (OpenAI-like)</Option>
                          <Option value="Performance1536D5M">1536D × 5M vectors (Large OpenAI-like)</Option>
                          <Option value="CapacityDim128">Capacity Test - 128D</Option>
                          <Option value="CapacityDim960">Capacity Test - 960D</Option>
                        </Select>
                      </Form.Item>

                      <Form.Item
                        name="vdb_k"
                        label="K (Nearest Neighbors)"
                        initialValue={100}
                        extra="Number of nearest neighbors to retrieve"
                      >
                        <InputNumber min={1} max={1000} style={{ width: '100%' }} />
                      </Form.Item>

                      <Form.Item
                        name="vdb_concurrency"
                        label="Concurrency Levels"
                        initialValue="1,10,20"
                        extra="Comma-separated concurrency values to test"
                      >
                        <Input placeholder="1,10,20,50" />
                      </Form.Item>
                    </Col>

                    <Col xs={24} lg={12}>
                      <Form.Item
                        name="vdb_m"
                        label="M (HNSW connections)"
                        initialValue={16}
                        extra="HNSW index parameter"
                      >
                        <InputNumber min={4} max={64} style={{ width: '100%' }} />
                      </Form.Item>

                      <Form.Item
                        name="vdb_ef_construction"
                        label="ef_construction"
                        initialValue={200}
                        extra="HNSW index build parameter"
                      >
                        <InputNumber min={50} max={1000} style={{ width: '100%' }} />
                      </Form.Item>

                      <Form.Item
                        name="vdb_ef_search"
                        label="ef_search"
                        initialValue={100}
                        extra="HNSW search parameter"
                      >
                        <InputNumber min={10} max={500} style={{ width: '100%' }} />
                      </Form.Item>

                      <Row gutter={16}>
                        <Col span={12}>
                          <Form.Item
                            name="vdb_drop_old"
                            label="Drop Old Data"
                            valuePropName="checked"
                            initialValue={true}
                          >
                            <Switch />
                          </Form.Item>
                        </Col>
                        <Col span={12}>
                          <Form.Item
                            name="vdb_load"
                            label="Load Data"
                            valuePropName="checked"
                            initialValue={true}
                          >
                            <Switch />
                          </Form.Item>
                        </Col>
                      </Row>
                    </Col>
                  </Row>
                </Card>
              </Col>
            )}
          </Row>

          <Divider />

          <Row justify="end">
            <Space>
              <Button onClick={() => navigate('/runs')}>Cancel</Button>
              <Button 
                type="primary" 
                htmlType="submit" 
                loading={loading}
                disabled={!selectedInfra}
                icon={<ThunderboltOutlined />}
                size="large"
              >
                {selectedInfra 
                  ? `Run on ${selectedInfra.name}` 
                  : 'Select Infrastructure'
                }
              </Button>
            </Space>
          </Row>
        </Form>
      )}
    </div>
  );
}
