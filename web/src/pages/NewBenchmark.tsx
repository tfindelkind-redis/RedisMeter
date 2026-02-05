import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
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
} from 'antd';
import {
  PlayCircleOutlined,
  ThunderboltOutlined,
  SettingOutlined,
  CheckCircleOutlined,
  LoadingOutlined,
  CloudOutlined,
} from '@ant-design/icons';
import api from '@/api/client';
import { Workload, BenchmarkConfig, Infrastructure } from '@/types';
import useWebSocket from '@/hooks/useWebSocket';

const { Title, Text, Paragraph } = Typography;
const { Option } = Select;

interface FormValues {
  name: string;
  description?: string;
  host: string;
  port: number;
  password?: string;
  tls: boolean;
  cluster: boolean;
  workload: string;
  threads: number;
  clients: number;
  duration: string;
  requests?: number;
  tags?: string;
}

export default function NewBenchmark() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [form] = Form.useForm();
  const [workloads, setWorkloads] = useState<Workload[]>([]);
  const [loading, setLoading] = useState(false);
  const [running, setRunning] = useState(false);
  const [runId, setRunId] = useState<string | null>(null);
  const [progress, setProgress] = useState(0);
  const [currentStep, setCurrentStep] = useState(0);
  
  // Cloud infrastructure mode
  const infraId = searchParams.get('infra');
  const [infrastructure, setInfrastructure] = useState<Infrastructure | null>(null);
  const [infraLoading, setInfraLoading] = useState(!!infraId);
  const isCloudMode = !!infraId && !!infrastructure;

  // WebSocket for real-time progress
  const { lastMessage } = useWebSocket(running ? runId : null);

  useEffect(() => {
    loadWorkloads();
    if (infraId) {
      loadInfrastructure(infraId);
    }
  }, [infraId]);

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

  const loadInfrastructure = async (id: string) => {
    setInfraLoading(true);
    try {
      const data = await api.getInfrastructure(id);
      setInfrastructure(data);
      
      // Pre-fill form with infrastructure data
      if (data.outputs) {
        form.setFieldsValue({
          host: data.outputs.redis_hostname,
          port: data.outputs.redis_port,
          password: data.outputs.redis_primary_key,
          tls: true, // AMR always uses TLS
          cluster: data.config?.amr?.clustering_policy === 'OSSCluster',
        });
      }
    } catch (error) {
      message.error('Failed to load infrastructure');
      navigate('/infrastructure');
    } finally {
      setInfraLoading(false);
    }
  };

  const onFinish = async (values: FormValues) => {
    setLoading(true);
    setCurrentStep(1);
    
    try {
      if (isCloudMode) {
        // Cloud benchmark mode
        const result = await api.runCloudBenchmark({
          infrastructure_id: infrastructure.id,
          workload: values.workload,
          threads: values.threads,
          clients: values.clients,
          requests: values.requests || 100000,
        });
        setRunId(result.benchmark_id || result.id);
        setRunning(true);
        message.success('Cloud benchmark started!');
        
        // Poll for completion since cloud benchmarks are async
        pollCloudBenchmarkStatus(result.benchmark_id || result.id);
      } else {
        // Local benchmark mode
        const config: BenchmarkConfig = {
          name: values.name,
          description: values.description,
          target: {
            host: values.host,
            port: values.port,
            password: values.password,
            tls: values.tls,
            cluster: values.cluster,
          },
          workload: values.workload,
          threads: values.threads,
          clients: values.clients,
          duration: values.duration,
          tags: values.tags?.split(',').map(t => t.trim()).filter(Boolean),
        };

        const result = await api.startBenchmark(config);
        setRunId(result.id);
        setRunning(true);
        message.success('Benchmark started!');
      }
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

  const selectedWorkload = workloads.find(w => w.name === form.getFieldValue('workload'));

  // Show loading spinner while loading infrastructure
  if (infraLoading) {
    return (
      <div style={{ textAlign: 'center', padding: 100 }}>
        <Spin size="large" />
        <p style={{ marginTop: 16, color: 'rgba(0,0,0,0.65)' }}>Loading infrastructure...</p>
      </div>
    );
  }

  return (
    <div>
      <Title level={3} style={{ color: 'rgba(0,0,0,0.85)', marginBottom: 24 }}>
        {isCloudMode ? (
          <>
            <CloudOutlined style={{ marginRight: 8, color: '#0078d4' }} />
            Cloud Benchmark
          </>
        ) : (
          <>
            <ThunderboltOutlined style={{ marginRight: 8, color: '#DC382D' }} />
            New Benchmark
          </>
        )}
      </Title>

      {/* Infrastructure Info - Cloud Mode */}
      {isCloudMode && infrastructure && (
        <Alert
          type="info"
          icon={<CloudOutlined />}
          message="Cloud Infrastructure Benchmark"
          description={
            <Descriptions size="small" column={2} style={{ marginTop: 8 }}>
              <Descriptions.Item label="Infrastructure">{infrastructure.name}</Descriptions.Item>
              <Descriptions.Item label="Provider">{infrastructure.provider}</Descriptions.Item>
              <Descriptions.Item label="Region">{infrastructure.region}</Descriptions.Item>
              {infrastructure.config?.amr?.sku && (
                <Descriptions.Item label="AMR SKU">{infrastructure.config.amr.sku}</Descriptions.Item>
              )}
              {infrastructure.outputs?.runner_ips && (
                <Descriptions.Item label="Runners">{infrastructure.outputs.runner_ips.length} VMs</Descriptions.Item>
              )}
            </Descriptions>
          }
          showIcon
          style={{ marginBottom: 24 }}
        />
      )}

      {/* Progress Steps */}
      <Card style={{ background: '#ffffff', border: '1px solid #d9d9d9', marginBottom: 24 }}>
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
            host: 'localhost',
            port: 6379,
            tls: false,
            cluster: false,
            threads: 4,
            clients: 50,
            duration: '30s',
            requests: 100000,
          }}
        >
          <Row gutter={24}>
            {/* Basic Info */}
            <Col xs={24} lg={12}>
              <Card 
                title="Basic Information" 
                style={{ background: '#ffffff', border: '1px solid #d9d9d9', marginBottom: 24 }}
              >
                <Form.Item
                  name="name"
                  label="Benchmark Name"
                  rules={[{ required: !isCloudMode, message: 'Please enter a name' }]}
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

            {/* Target Configuration - Hidden in cloud mode */}
            {!isCloudMode && (
              <Col xs={24} lg={12}>
                <Card 
                  title="Redis Target" 
                  style={{ background: '#ffffff', border: '1px solid #d9d9d9', marginBottom: 24 }}
                >
                  <Row gutter={16}>
                    <Col span={16}>
                      <Form.Item
                        name="host"
                        label="Host"
                        rules={[{ required: true }]}
                      >
                        <Input placeholder="localhost" />
                      </Form.Item>
                    </Col>
                    <Col span={8}>
                      <Form.Item
                        name="port"
                        label="Port"
                        rules={[{ required: true }]}
                      >
                        <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                      </Form.Item>
                    </Col>
                  </Row>

                  <Form.Item
                    name="password"
                    label="Password"
                  >
                    <Input.Password placeholder="Optional password" />
                  </Form.Item>

                  <Row gutter={16}>
                    <Col span={12}>
                      <Form.Item name="tls" label="TLS" valuePropName="checked">
                        <Switch />
                      </Form.Item>
                    </Col>
                    <Col span={12}>
                      <Form.Item name="cluster" label="Cluster Mode" valuePropName="checked">
                        <Switch />
                      </Form.Item>
                    </Col>
                  </Row>
                </Card>
              </Col>
            )}

            {/* Workload */}
            <Col xs={24} lg={12}>
              <Card 
                title="Workload" 
                style={{ background: '#ffffff', border: '1px solid #d9d9d9', marginBottom: 24 }}
              >
                <Form.Item
                  name="workload"
                  label="Workload Profile"
                  rules={[{ required: true, message: 'Please select a workload' }]}
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

            {/* Performance Settings */}
            <Col xs={24} lg={12}>
              <Card 
                title="Performance Settings" 
                style={{ background: '#ffffff', border: '1px solid #d9d9d9', marginBottom: 24 }}
              >
                <Form.Item
                  name="threads"
                  label="Threads"
                  rules={[{ required: true }]}
                  extra="Number of worker threads per runner"
                >
                  <InputNumber min={1} max={64} style={{ width: '100%' }} />
                </Form.Item>

                <Form.Item
                  name="clients"
                  label="Clients"
                  rules={[{ required: true }]}
                  extra="Number of concurrent connections per runner"
                >
                  <InputNumber min={1} max={1000} style={{ width: '100%' }} />
                </Form.Item>

                {isCloudMode ? (
                  <Form.Item
                    name="requests"
                    label="Requests"
                    rules={[{ required: true }]}
                    extra="Total number of requests per runner"
                  >
                    <InputNumber min={1000} style={{ width: '100%' }} />
                  </Form.Item>
                ) : (
                  <Form.Item
                    name="duration"
                    label="Duration"
                    rules={[{ required: true }]}
                    extra="e.g., 30s, 5m, 1h"
                  >
                    <Input placeholder="30s" />
                  </Form.Item>
                )}
              </Card>
            </Col>
          </Row>

          <Divider />

          <Row justify="end">
            <Space>
              <Button onClick={() => navigate(isCloudMode ? '/infrastructure' : '/runs')}>Cancel</Button>
              <Button 
                type="primary" 
                htmlType="submit" 
                loading={loading}
                icon={isCloudMode ? <CloudOutlined /> : <PlayCircleOutlined />}
                size="large"
              >
                {isCloudMode ? 'Start Cloud Benchmark' : 'Start Benchmark'}
              </Button>
            </Space>
          </Row>
        </Form>
      )}
    </div>
  );
}
