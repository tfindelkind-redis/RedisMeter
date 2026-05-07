import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Tabs,
  Form,
  Button,
  Card,
  Typography,
  Space,
  Input,
  InputNumber,
  Row,
  Col,
  Badge,
  message,
  Steps,
  Progress,
  Switch,
  Alert,
  Tag,
} from 'antd';
import {
  CloudOutlined,
  RocketOutlined,
  CheckCircleOutlined,
  LoadingOutlined,
  SettingOutlined,
  LinkOutlined,
  PlusOutlined,
  DesktopOutlined,
  LockOutlined,
  DatabaseOutlined,
} from '@ant-design/icons';
import { CloudProvider } from '@/types';
import { 
  PROVIDER_INFO, 
  PROVIDER_COMPONENTS, 
} from '@/components/Infrastructure/providers';
import api from '@/api/client';

const { Title, Text, Paragraph } = Typography;

// Mode for infrastructure setup
type SetupMode = 'existing' | 'deploy';

// Provider icons
const ProviderIcon = ({ provider }: { provider: CloudProvider }) => {
  const iconStyles: Record<CloudProvider, { color: string; label: string }> = {
    azure: { color: '#0078d4', label: 'Azure' },
    aws: { color: '#ff9900', label: 'AWS' },
    gcp: { color: '#4285f4', label: 'GCP' },
    kubernetes: { color: '#326ce5', label: 'K8s' },
    vmware: { color: '#607078', label: 'VMw' },
    local: { color: '#52c41a', label: 'Local' },
    self_managed: { color: '#722ed1', label: 'BYOI' },
  };
  const style = iconStyles[provider];
  return (
    <div style={{ 
      width: 32, 
      height: 32, 
      borderRadius: 4, 
      background: style.color,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      color: 'white',
      fontWeight: 'bold',
      fontSize: 10,
    }}>
      {style.label}
    </div>
  );
};

// Providers that can deploy new infrastructure  
const DEPLOY_PROVIDERS: CloudProvider[] = ['azure', 'local'];

export default function InfrastructureNew() {
  const navigate = useNavigate();
  const [form] = Form.useForm();
  const [setupMode, setSetupMode] = useState<SetupMode>('deploy');
  const [activeProvider, setActiveProvider] = useState<CloudProvider>('azure');
  const [loading, setLoading] = useState(false);
  const [provisioning, setProvisioning] = useState(false);
  const [currentStep, setCurrentStep] = useState(0);
  const [progress, setProgress] = useState(0);
  
  const ProviderForm = PROVIDER_COMPONENTS[activeProvider];

  const onModeChange = (mode: SetupMode) => {
    setSetupMode(mode);
    // Set default provider for each mode
    if (mode === 'existing') {
      setActiveProvider('local');
    } else {
      setActiveProvider('azure');
    }
    form.resetFields();
  };

  const onProviderChange = (key: string) => {
    setActiveProvider(key as CloudProvider);
    // Reset form when switching providers
    form.resetFields();
    form.setFieldsValue({ provider: key });
  };

  const onFinish = async (values: any) => {
    // Check if self-managed (disabled)
    if (activeProvider === 'self_managed') {
      message.warning('Self-Managed infrastructure is coming soon!');
      return;
    }

    const providerInfo = PROVIDER_INFO[activeProvider];
    if (!providerInfo.enabled) {
      message.warning(`${providerInfo.name} support is coming soon!`);
      return;
    }

    setLoading(true);
    setCurrentStep(1);
    setProvisioning(true);

    try {
      let config: any;

      // Handle existing local infrastructure (just credentials)
      if (setupMode === 'existing' && activeProvider === 'local') {
        config = {
          name: values.name,
          provider: 'local',
          region: 'local',
          local: {
            mode: 'existing',
            redis: {
              host: values.redis_host || 'localhost',
              port: values.redis_port || 6379,
              password: values.redis_password,
              tls_enabled: values.redis_tls_enabled || false,
              database: values.redis_database || 0,
            },
          },
        };

        // Local existing infra is immediately "ready"
        const result = await api.createInfrastructure(config);
        
        setProgress(100);
        setCurrentStep(2);
        setProvisioning(false);
        message.success('Local infrastructure connected successfully!');
        setTimeout(() => {
          navigate(`/infrastructure/${result.id}`);
        }, 1500);
        return;
      }

      // Handle local Docker deployment
      if (setupMode === 'deploy' && activeProvider === 'local') {
        config = {
          name: values.name,
          provider: 'local',
          region: 'local',
          local: {
            mode: 'docker',
            docker: {
              image: values.docker_image || 'redis/redis-stack:latest',
              port: values.docker_port || 6379,
              memory_limit: values.docker_memory || '2g',
              persistence: values.docker_persistence || false,
            },
          },
        };

        const result = await api.createInfrastructure(config);
        pollInfraStatus(result.id);
        return;
      }

      // Build cloud infrastructure config from form values (Azure)
      config = {
        name: values.name,
        provider: activeProvider,
        region: values.region,
        ttl: values.ttl,
        tags: values.tags ? parseTags(values.tags) : undefined,
        amr: values.amr_enabled ? {
          sku: values.amr_sku,
          modules: values.amr_modules,
          high_availability: values.amr_ha,
          clustering_policy: values.amr_clustering,
          eviction_policy: values.amr_eviction,
        } : undefined,
        runners: {
          count: values.runner_count,
          instance_type: values.runner_type,
          spot_instances: values.runners_spot,
          ssh_public_key: values.ssh_key,
          ssh_user: values.ssh_user,
        },
      };

      const result = await api.createInfrastructure(config);

      // Start polling for status
      pollInfraStatus(result.id);

    } catch (error: any) {
      message.error(error.message || 'Failed to create infrastructure');
      setCurrentStep(0);
      setProvisioning(false);
    } finally {
      setLoading(false);
    }
  };

  const pollInfraStatus = async (id: string) => {
    const poll = async () => {
      try {
        const infra = await api.getInfrastructure(id);
        
        if (infra.status === 'ready') {
          setProgress(100);
          setCurrentStep(2);
          setProvisioning(false);
          message.success('Infrastructure provisioned successfully!');
          setTimeout(() => {
            navigate(`/infrastructure/${id}`);
          }, 1500);
          return;
        }
        
        if (infra.status === 'failed') {
          message.error(infra.error || 'Infrastructure provisioning failed');
          setCurrentStep(0);
          setProvisioning(false);
          return;
        }

        // Update progress (estimate based on typical provisioning time)
        setProgress(prev => Math.min(prev + 5, 95));
        
        // Continue polling
        setTimeout(poll, 5000);
      } catch (error) {
        console.error('Failed to poll infrastructure status', error);
        setTimeout(poll, 5000);
      }
    };
    
    poll();
  };

  const parseTags = (tagString: string): Record<string, string> => {
    const tags: Record<string, string> = {};
    tagString.split(',').forEach(pair => {
      const [key, value] = pair.split('=').map(s => s.trim());
      if (key && value) tags[key] = value;
    });
    return tags;
  };

  // Build tab items for deploy mode
  const deployTabItems = DEPLOY_PROVIDERS.map(providerId => {
    const provider = PROVIDER_INFO[providerId];
    const isDisabled = providerId === 'self_managed';
    return {
      key: providerId,
      label: (
        <Space>
          <ProviderIcon provider={providerId} />
          <span>{provider.name}</span>
          {isDisabled && <Badge count="Soon" style={{ backgroundColor: '#666' }} />}
        </Space>
      ),
      children: (
        <div style={{ padding: '16px 0' }}>
          <Paragraph type="secondary" style={{ marginBottom: 24 }}>
            {provider.description}
          </Paragraph>
          {providerId === 'local' ? (
            // Local Docker deployment form
            <Space direction="vertical" style={{ width: '100%' }} size="large">
              <Alert
                type="info"
                message="Deploy Redis as Docker Container"
                description="This will start a Redis container on your local machine for benchmarking."
                showIcon
              />
              <Row gutter={24}>
                <Col span={12}>
                  <Form.Item
                    name="docker_image"
                    label="Docker Image"
                    initialValue="redis/redis-stack:latest"
                    extra="Redis image to use"
                  >
                    <Input placeholder="redis/redis-stack:latest" />
                  </Form.Item>
                </Col>
                <Col span={12}>
                  <Form.Item
                    name="docker_port"
                    label="Port"
                    initialValue={6379}
                    extra="Redis port mapping"
                  >
                    <InputNumber min={1024} max={65535} style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
              </Row>
              <Row gutter={24}>
                <Col span={12}>
                  <Form.Item
                    name="docker_memory"
                    label="Memory Limit"
                    initialValue="2g"
                    extra="Container memory limit"
                  >
                    <Input placeholder="2g" />
                  </Form.Item>
                </Col>
                <Col span={12}>
                  <Form.Item
                    name="docker_persistence"
                    label="Enable Persistence"
                    valuePropName="checked"
                    extra="Persist data between restarts"
                  >
                    <Switch />
                  </Form.Item>
                </Col>
              </Row>
            </Space>
          ) : (
            ProviderForm && <ProviderForm form={form} disabled={provisioning || isDisabled} />
          )}
        </div>
      ),
      disabled: provisioning || isDisabled,
    };
  });

  return (
    <div>
      <Title level={3} style={{ color: 'rgba(255,255,255,0.85)', marginBottom: 24 }}>
        <CloudOutlined style={{ marginRight: 8, color: '#0078d4' }} />
        New Infrastructure
      </Title>

      {/* Progress Steps */}
      <Card style={{ background: '#1f1f1f', border: '1px solid #303030', marginBottom: 24 }}>
        <Steps
          current={currentStep}
          items={[
            {
              title: 'Configure',
              icon: currentStep === 0 ? <SettingOutlined /> : <CheckCircleOutlined />,
            },
            {
              title: setupMode === 'existing' ? 'Connecting' : 'Provisioning',
              icon: currentStep === 1 ? <LoadingOutlined /> : currentStep > 1 ? <CheckCircleOutlined /> : undefined,
            },
            {
              title: 'Ready',
              icon: currentStep === 2 ? <CheckCircleOutlined style={{ color: '#52c41a' }} /> : undefined,
            },
          ]}
        />
        
        {provisioning && (
          <div style={{ marginTop: 24, textAlign: 'center' }}>
            <Progress
              type="circle"
              percent={progress}
              strokeColor="#0078d4"
              format={(pct) => `${pct}%`}
            />
            <Paragraph style={{ marginTop: 16 }}>
              <Text type="secondary">
                {setupMode === 'existing' 
                  ? 'Connecting to infrastructure...'
                  : activeProvider === 'local'
                    ? 'Starting Docker container...'
                    : 'Provisioning infrastructure... This may take 5-15 minutes for Azure Managed Redis.'
                }
              </Text>
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
            provider: 'azure',
            runner_count: 2,
            runner_type: 'Standard_D4s_v3',
            ssh_user: 'azureuser',
            amr_enabled: true,
            amr_sku: 'Balanced_B0',
            amr_clustering: 'OSSCluster',
            amr_eviction: 'VolatileLRU',
          }}
        >
          {/* Infrastructure Name */}
          <Card style={{ background: '#1f1f1f', border: '1px solid #303030', marginBottom: 24 }}>
            <Row gutter={24}>
              <Col span={12}>
                <Form.Item
                  name="name"
                  label="Infrastructure Name"
                  rules={[
                    { required: true, message: 'Please enter a name' },
                    { pattern: /^[a-z0-9-]+$/, message: 'Only lowercase letters, numbers, and hyphens' },
                  ]}
                  extra="Used for resource naming and identification"
                >
                  <Input placeholder="my-benchmark-env" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item name="provider" hidden>
                  <Input />
                </Form.Item>
              </Col>
            </Row>
          </Card>

          {/* Setup Mode Selection */}
          <Card style={{ background: '#1f1f1f', border: '1px solid #303030', marginBottom: 24 }}>
            <Row gutter={24}>
              <Col span={12}>
                <Card
                  hoverable
                  onClick={() => onModeChange('existing')}
                  style={{
                    border: setupMode === 'existing' ? '2px solid #DC382D' : '1px solid #333333',
                    background: setupMode === 'existing' ? 'rgba(220, 56, 45, 0.15)' : '#1a1a1a',
                    cursor: 'pointer',
                  }}
                  bodyStyle={{ padding: 20 }}
                >
                  <Space direction="vertical" size={8}>
                    <Space>
                      <LinkOutlined style={{ fontSize: 24, color: setupMode === 'existing' ? '#DC382D' : '#888' }} />
                      <Text strong style={{ fontSize: 16 }}>Connect to Existing</Text>
                    </Space>
                    <Text type="secondary">
                      Connect to an existing Redis deployment by providing connection credentials
                    </Text>
                    <Tag color="green">Local Redis</Tag>
                  </Space>
                </Card>
              </Col>
              <Col span={12}>
                <Card
                  hoverable
                  onClick={() => onModeChange('deploy')}
                  style={{
                    border: setupMode === 'deploy' ? '2px solid #DC382D' : '1px solid #333333',
                    background: setupMode === 'deploy' ? 'rgba(220, 56, 45, 0.15)' : '#1a1a1a',
                    cursor: 'pointer',
                  }}
                  bodyStyle={{ padding: 20 }}
                >
                  <Space direction="vertical" size={8}>
                    <Space>
                      <PlusOutlined style={{ fontSize: 24, color: setupMode === 'deploy' ? '#DC382D' : '#888' }} />
                      <Text strong style={{ fontSize: 16 }}>Deploy New</Text>
                    </Space>
                    <Text type="secondary">
                      Provision new Redis infrastructure on Azure or deploy locally via Docker
                    </Text>
                    <Space size={4}>
                      <Tag color="blue">Azure</Tag>
                      <Tag color="green">Docker</Tag>
                    </Space>
                  </Space>
                </Card>
              </Col>
            </Row>
          </Card>

          {/* Existing Infrastructure - Local Connection */}
          {setupMode === 'existing' && (
            <Card 
              title={
                <Space>
                  <DatabaseOutlined />
                  <span>Connect to Local Redis</span>
                </Space>
              }
              style={{ background: '#1f1f1f', border: '1px solid #303030', marginBottom: 24 }}
            >
              <Alert
                type="info"
                message="Connect to Existing Redis"
                description="Enter the connection details for your local Redis instance. This can be a Redis server running natively or in a container."
                showIcon
                style={{ marginBottom: 24 }}
              />
              <Row gutter={24}>
                <Col span={12}>
                  <Form.Item
                    name="redis_host"
                    label="Redis Host"
                    initialValue="localhost"
                    rules={[{ required: true, message: 'Please enter the host' }]}
                    extra="Hostname or IP address"
                  >
                    <Input placeholder="localhost" prefix={<DesktopOutlined />} />
                  </Form.Item>
                </Col>
                <Col span={12}>
                  <Form.Item
                    name="redis_port"
                    label="Redis Port"
                    initialValue={6379}
                    rules={[{ required: true, message: 'Please enter the port' }]}
                    extra="Default: 6379"
                  >
                    <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
              </Row>
              <Row gutter={24}>
                <Col span={12}>
                  <Form.Item
                    name="redis_password"
                    label="Password"
                    extra="Leave empty if no authentication"
                  >
                    <Input.Password placeholder="Optional" prefix={<LockOutlined />} />
                  </Form.Item>
                </Col>
                <Col span={12}>
                  <Form.Item
                    name="redis_database"
                    label="Database"
                    initialValue={0}
                    extra="Redis database index (0-15)"
                  >
                    <InputNumber min={0} max={15} style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
              </Row>
              <Row gutter={24}>
                <Col span={12}>
                  <Form.Item
                    name="redis_tls_enabled"
                    label="TLS Enabled"
                    valuePropName="checked"
                    extra="Enable TLS/SSL connection"
                  >
                    <Switch />
                  </Form.Item>
                </Col>
              </Row>
            </Card>
          )}

          {/* Deploy New Infrastructure */}
          {setupMode === 'deploy' && (
            <Card style={{ background: '#1f1f1f', border: '1px solid #303030', marginBottom: 24 }}>
              <Tabs
                activeKey={activeProvider}
                onChange={onProviderChange}
                items={deployTabItems}
                type="card"
                size="large"
              />
            </Card>
          )}

          {/* Self-Managed (Disabled) indicator */}
          {setupMode === 'deploy' && (
            <Card style={{ background: '#0d0d0d', border: '1px solid #333333', marginBottom: 24, opacity: 0.6 }}>
              <Space>
                <ProviderIcon provider="self_managed" />
                <div>
                  <Space>
                    <Text strong style={{ color: 'rgba(255,255,255,0.45)' }}>Self-Managed Infrastructure</Text>
                    <Tag color="orange">Coming Soon</Tag>
                  </Space>
                  <br />
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    Connect to your own machines via SSH and benchmark any Redis deployment
                  </Text>
                </div>
              </Space>
            </Card>
          )}

          {/* Submit Button */}
          <Row justify="end">
            <Space>
              <Button onClick={() => navigate('/infrastructure')}>Cancel</Button>
              <Button 
                type="primary" 
                htmlType="submit" 
                loading={loading}
                icon={setupMode === 'existing' ? <LinkOutlined /> : <RocketOutlined />}
                size="large"
              >
                {setupMode === 'existing' ? 'Connect' : 'Provision Infrastructure'}
              </Button>
            </Space>
          </Row>
        </Form>
      )}
    </div>
  );
}
