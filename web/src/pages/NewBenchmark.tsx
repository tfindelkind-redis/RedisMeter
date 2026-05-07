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
  ClockCircleOutlined,
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
  QuestionCircleOutlined,
} from '@ant-design/icons';
import api from '@/api/client';
import { 
  Workload, 
  Infrastructure, 
  BenchmarkToolType, 
  BenchmarkTool,
  BenchmarkStatus,
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
    id: 'vector_db_benchmark',
    name: 'Vector DB Benchmark',
    description: 'Redis vector search benchmarking using redis/vector-db-benchmark. Tests RediSearch HNSW and VectorSets with standard datasets.',
    icon: 'radar',
    category: 'vector',
    supported_workloads: ['vector-search', 'hnsw', 'similarity', 'filtered-search'],
    requires_module: ['search'],
    available: true,
    documentation_url: 'https://github.com/redis/vector-db-benchmark',
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
  // Vector DB Benchmark (redis/vector-db-benchmark) config
  vbm_dataset?: string;
  vbm_engine?: string;
  vbm_k?: number;
  vbm_ef_runtime?: number;
  vbm_parallelism?: number;
  vbm_upload_only?: boolean;
  vbm_search_only?: boolean;
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
  const [benchmarkStatus, setBenchmarkStatus] = useState<BenchmarkStatus | null>(null);
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
      
      if (selectedTool === 'vector_db_benchmark') {
        toolConfig = {
          dataset: values.vbm_dataset,
          engine: values.vbm_engine,
          k: values.vbm_k,
          ef_runtime: values.vbm_ef_runtime,
          parallelism: values.vbm_parallelism,
          upload_only: values.vbm_upload_only,
          search_only: values.vbm_search_only,
        };
      }

      const profile = runProfiles.find((p) => p.name === values.run_profile);
      const effectiveDuration = values.duration || profile?.duration;
      const effectiveRequests = values.requests ?? profile?.requests;

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
        duration: effectiveDuration,
        requests: effectiveDuration ? undefined : (effectiveRequests || 100000),
        tool_config: selectedTool !== 'memtier_benchmark' ? toolConfig : undefined,
        tags: values.tags?.split(',').map(t => t.trim()).filter(Boolean),
      });
      
      setRunId(result.benchmark_id || result.id);
      setBenchmarkStatus(null);
      setProgress(0);
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
        setBenchmarkStatus(status);
        
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

  const formatSeconds = (seconds?: number) => {
    if (!seconds || seconds <= 0) {
      return '0s';
    }

    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    if (mins === 0) {
      return `${secs}s`;
    }

    return `${mins}m ${secs}s`;
  };

  const detailedStages = [
    { key: 'starting', title: 'Preparing' },
    { key: 'connecting', title: 'Connecting' },
    { key: 'executing', title: 'Executing' },
    { key: 'collecting_results', title: 'Collecting Results' },
    { key: 'saving', title: 'Saving Run' },
    { key: 'completed', title: 'Complete' },
  ];

  const currentDetailedStage = Math.max(0, (benchmarkStatus?.stage_index ?? 1) - 1);

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
              <Badge.Ribbon 
                text={selectedTool === tool.id ? "Selected" : ""} 
                color="#DC382D"
                style={{ display: selectedTool === tool.id ? 'block' : 'none' }}
              >
                <Card
                  hoverable={tool.available}
                  size="small"
                  onClick={() => tool.available && setSelectedTool(tool.id)}
                  style={{
                    border: selectedTool === tool.id ? '2px solid #DC382D' : '1px solid #333333',
                    background: selectedTool === tool.id ? 'rgba(220, 56, 45, 0.25)' : tool.available ? '#1a1a1a' : '#0d0d0d',
                    cursor: tool.available ? 'pointer' : 'not-allowed',
                    height: '100%',
                    opacity: tool.available ? 1 : 0.7,
                    boxShadow: selectedTool === tool.id ? '0 0 12px rgba(220, 56, 45, 0.4)' : 'none',
                    transition: 'all 0.2s ease',
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
                        color: selectedTool === tool.id ? '#ffffff' : 'inherit',
                        fontSize: selectedTool === tool.id ? 14 : 13,
                      }}>
                        {tool.name}
                      </Text>
                      {selectedTool === tool.id && (
                        <CheckCircleOutlined style={{ color: '#DC382D', fontSize: 16 }} />
                      )}
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
              </Badge.Ribbon>
            </Col>
          ))}
        </Row>
        
        {selectedTool !== 'memtier_benchmark' && (
          <Alert
            type="info"
            message={`${BENCHMARK_TOOLS.find(t => t.id === selectedTool)?.name} Configuration`}
            description={
              <Text type="secondary" style={{ fontSize: 13 }}>
                {selectedTool === 'vector_db_benchmark' && 
                  'Vector DB Benchmark runs via Docker using redis/vector-db-benchmark. Select dataset and engine configuration below.'}
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
          <div style={{ marginTop: 24 }}>
            <div style={{ textAlign: 'center' }}>
              <Progress
                type="circle"
                percent={progress}
                strokeColor="#DC382D"
                format={(pct) => `${pct}%`}
              />
              <Paragraph style={{ marginTop: 16, marginBottom: 8 }}>
                <Text strong>{benchmarkStatus?.stage_label || 'Benchmark in progress...'}</Text>
              </Paragraph>
              <Space size="large" wrap>
                <Text type="secondary">
                  <ClockCircleOutlined style={{ marginRight: 6 }} />
                  Elapsed: {formatSeconds(benchmarkStatus?.elapsed_seconds)}
                </Text>
                <Text type="secondary">
                  ETA: {formatSeconds(benchmarkStatus?.estimated_remaining_seconds)}
                </Text>
              </Space>
            </div>

            <div style={{ marginTop: 24 }}>
              <Steps
                size="small"
                current={currentDetailedStage}
                items={detailedStages.map((stage) => ({ title: stage.title }))}
              />
            </div>
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
                title={
                  <Space>
                    Workload
                    <Tooltip title="A Workload defines WHAT operations to run against Redis: which commands (GET, SET, etc.), their mix ratio, key patterns, and data sizes.">
                      <QuestionCircleOutlined style={{ color: '#8c8c8c', fontSize: 14 }} />
                    </Tooltip>
                  </Space>
                }
                extra={<Link to="/workloads"><SettingOutlined /> Manage</Link>}
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
                  <div style={{ marginTop: 16, padding: 12, background: 'rgba(0, 0, 0, 0.06)', borderRadius: 6 }}>
                    <Text type="secondary">Operations:</Text>
                    <div style={{ marginTop: 8 }}>
                      {selectedWorkload.operations && selectedWorkload.operations.length > 0 ? (
                        selectedWorkload.operations.map((op) => (
                          <Tag key={op.command} style={{ marginBottom: 4 }}>
                            {op.command} ({(op.ratio * 100).toFixed(0)}%)
                          </Tag>
                        ))
                      ) : (
                        <Text type="secondary" italic>Custom workload configuration</Text>
                      )}
                    </div>
                    {selectedWorkload.description && (
                      <div style={{ marginTop: 8, borderTop: '1px solid rgba(255, 255, 255, 0.1)', paddingTop: 8 }}>
                        <Text type="secondary" style={{ fontSize: 12 }}>{selectedWorkload.description}</Text>
                      </div>
                    )}
                  </div>
                )}
              </Card>
            </Col>
            )}

            {/* Performance Settings - Only for memtier_benchmark */}
            {selectedTool === 'memtier_benchmark' && (
            <Col xs={24} lg={12}>
              <Card 
                title={
                  <Space>
                    Run Profile
                    <Tooltip title="A Run Profile defines HOW to execute the benchmark: number of threads, clients per thread, test duration or request count, and pipeline depth.">
                      <QuestionCircleOutlined style={{ color: '#8c8c8c', fontSize: 14 }} />
                    </Tooltip>
                  </Space>
                }
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
                  <div style={{ marginTop: 8, padding: 12, background: 'rgba(0, 0, 0, 0.06)', borderRadius: 6 }}>
                    <Row gutter={16}>
                      <Col span={12}>
                        <Text type="secondary">Threads:</Text> <Text>{selectedRunProfile.threads}</Text>
                      </Col>
                      <Col span={12}>
                        <Text type="secondary">Clients:</Text> <Text>{selectedRunProfile.clients}</Text>
                      </Col>
                    </Row>
                    <Row gutter={16} style={{ marginTop: 8 }}>
                      <Col span={12}>
                        {selectedRunProfile.duration ? (
                          <><Text type="secondary">Duration:</Text> <Text>{selectedRunProfile.duration}</Text></>
                        ) : (
                          <><Text type="secondary">Requests:</Text> <Text>{selectedRunProfile.requests?.toLocaleString()}</Text></>
                        )}
                      </Col>
                      <Col span={12}>
                        <Text type="secondary">Pipeline:</Text> <Text>{selectedRunProfile.pipeline}</Text>
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

            {/* Vector DB Benchmark Configuration */}
            {selectedTool === 'vector_db_benchmark' && (
              <Col xs={24}>
                <Card 
                  title={<><RadarChartOutlined /> Vector DB Benchmark Configuration</>}
                  style={{ marginBottom: 24 }}
                >
                  <Row gutter={24}>
                    <Col xs={24} lg={12}>
                      <Form.Item
                        name="vbm_dataset"
                        label="Dataset"
                        rules={[{ required: selectedTool === 'vector_db_benchmark', message: 'Please select a dataset' }]}
                        extra="Standard benchmark datasets with ground truth"
                      >
                        <Select placeholder="Select dataset">
                          <Option value="random-100">Random-100 (100 vectors, testing ~1 min)</Option>
                          <Option value="glove-25-angular">GloVe-25 (1.2M vectors, ~5 min)</Option>
                          <Option value="glove-100-angular">GloVe-100 (1.2M vectors, ~15 min)</Option>
                          <Option value="gist-960-euclidean">GIST-960 (1M vectors, ~30 min)</Option>
                          <Option value="deep-image-96-angular">Deep Image (10M vectors, ~45 min)</Option>
                          <Option value="laion-small-clip">LAION Small (100K vectors, ~10 min)</Option>
                          <Option value="dbpedia-openai-1m">DBpedia OpenAI (1M, 1536D, ~2 hours)</Option>
                          <Option value="h-and-m-2048-filtered">H&M Fashion Filtered (105K, ~20 min)</Option>
                          <Option value="arxiv-384-filtered">ArXiv Papers Filtered (2.2M, ~1 hour)</Option>
                        </Select>
                      </Form.Item>

                      <Form.Item
                        name="vbm_engine"
                        label="Redis Engine"
                        initialValue="redis-default-simple"
                        extra="Redis vector search engine configuration"
                      >
                        <Select>
                          <Option value="redis-default-simple">Redis RediSearch (default)</Option>
                          <Option value="redis-hnsw-m-16-ef-200">Redis HNSW (M=16, ef=200)</Option>
                          <Option value="redis-hnsw-m-32-ef-400">Redis HNSW (M=32, ef=400)</Option>
                          <Option value="vectorsets-fp32-default">Redis VectorSets (FP32)</Option>
                          <Option value="vectorsets-q8-default">Redis VectorSets (INT8)</Option>
                        </Select>
                      </Form.Item>

                      <Form.Item
                        name="vbm_k"
                        label="K (Nearest Neighbors)"
                        initialValue={10}
                        extra="Number of nearest neighbors to retrieve"
                      >
                        <InputNumber min={1} max={1000} style={{ width: '100%' }} />
                      </Form.Item>
                    </Col>

                    <Col xs={24} lg={12}>
                      <Form.Item
                        name="vbm_ef_runtime"
                        label="ef_runtime"
                        initialValue={10}
                        extra="HNSW search parameter (higher = better recall, slower search)"
                      >
                        <InputNumber min={1} max={500} style={{ width: '100%' }} />
                      </Form.Item>

                      <Form.Item
                        name="vbm_parallelism"
                        label="Parallelism"
                        initialValue={1}
                        extra="Number of parallel search threads"
                      >
                        <InputNumber min={1} max={64} style={{ width: '100%' }} />
                      </Form.Item>

                      <Form.Item
                        name="vbm_upload_only"
                        label="Upload Only"
                        valuePropName="checked"
                        extra="Only upload data, skip search benchmark"
                      >
                        <Switch />
                      </Form.Item>

                      <Form.Item
                        name="vbm_search_only"
                        label="Search Only"
                        valuePropName="checked"
                        extra="Only run search benchmark (assumes data exists)"
                      >
                        <Switch />
                      </Form.Item>
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
