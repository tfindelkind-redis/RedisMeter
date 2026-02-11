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
  Row,
  Col,
  Badge,
  message,
  Steps,
  Progress,
} from 'antd';
import {
  CloudOutlined,
  RocketOutlined,
  CheckCircleOutlined,
  LoadingOutlined,
  SettingOutlined,
} from '@ant-design/icons';
import { CloudProvider } from '@/types';
import { encryptSensitiveFields } from '@/utils/encryption';
import { 
  PROVIDER_INFO, 
  PROVIDER_COMPONENTS, 
  getAllProviders,
} from '@/components/Infrastructure/providers';
import api from '@/api/client';

const { Title, Text, Paragraph } = Typography;

// Provider icons (you can replace with actual SVG icons)
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

export default function InfrastructureNew() {
  const navigate = useNavigate();
  const [form] = Form.useForm();
  const [activeProvider, setActiveProvider] = useState<CloudProvider>('azure');
  const [loading, setLoading] = useState(false);
  const [provisioning, setProvisioning] = useState(false);
  const [currentStep, setCurrentStep] = useState(0);
  const [progress, setProgress] = useState(0);

  const providers = getAllProviders();
  const ProviderForm = PROVIDER_COMPONENTS[activeProvider];

  const onProviderChange = (key: string) => {
    setActiveProvider(key as CloudProvider);
    // Reset form when switching providers
    form.resetFields();
    form.setFieldsValue({ provider: key });
  };

  const onFinish = async (values: any) => {
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

      // Handle self-managed infrastructure differently
      if (activeProvider === 'self_managed') {
        // Build self-managed config with encrypted credentials
        const selfManagedConfig = {
          runners: {
            machines: values.runners_list || [],
            ssh: {
              auth_method: values.ssh_auth_method,
              username: values.ssh_username,
              password: values.ssh_password,
              private_key: values.ssh_private_key,
              private_key_path: values.ssh_private_key_path,
              passphrase: values.ssh_key_passphrase,
              port: values.ssh_port || 22,
              connect_timeout: values.ssh_connect_timeout || 30,
              strict_host_key_checking: values.ssh_strict_host_key || false,
            },
          },
          redis: {
            targets: values.redis_targets_list || [],
            credentials: {
              username: values.redis_username,
              password: values.redis_password,
              tls_enabled: values.redis_tls_enabled || false,
              tls_skip_verify: values.redis_tls_skip_verify || false,
              tls_cert: values.redis_tls_cert,
              tls_key: values.redis_tls_key,
              tls_ca: values.redis_tls_ca,
            },
          },
          tool_paths: {
            memtier_benchmark: values.memtier_path,
            redis_cli: values.redis_cli_path,
            ftsb: values.ftsb_path,
            ann_benchmarks: values.ann_benchmarks_path,
          },
        };

        // Encrypt sensitive fields before sending
        const { data: encryptedConfig, encryptionKey } = await encryptSensitiveFields(selfManagedConfig);

        config = {
          name: values.name,
          provider: 'self_managed',
          region: 'custom',
          tags: values.tags ? parseTags(values.tags) : undefined,
          self_managed: encryptedConfig,
          encryption_key: encryptionKey,
        };

        // Self-managed infra is immediately "ready" (no provisioning needed)
        const result = await api.createInfrastructure(config);
        
        setProgress(100);
        setCurrentStep(2);
        setProvisioning(false);
        message.success('Self-managed infrastructure registered successfully!');
        setTimeout(() => {
          navigate(`/infrastructure/${result.id}`);
        }, 1500);
        return;
      }

      // Build cloud infrastructure config from form values
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

  const tabItems = providers.map(provider => ({
    key: provider.id,
    label: (
      <Space>
        <ProviderIcon provider={provider.id} />
        <span>{provider.name}</span>
        {!provider.enabled && <Badge count="Soon" style={{ backgroundColor: '#666' }} />}
      </Space>
    ),
    children: (
      <div style={{ padding: '16px 0' }}>
        <Paragraph type="secondary" style={{ marginBottom: 24 }}>
          {provider.description}
        </Paragraph>
        {ProviderForm && <ProviderForm form={form} disabled={provisioning || !provider.enabled} />}
      </div>
    ),
    disabled: provisioning,
  }));

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
              title: 'Provisioning',
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
                Provisioning infrastructure... This may take 5-15 minutes for Azure Managed Redis.
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

          {/* Provider Tabs */}
          <Card style={{ background: '#1f1f1f', border: '1px solid #303030', marginBottom: 24 }}>
            <Tabs
              activeKey={activeProvider}
              onChange={onProviderChange}
              items={tabItems}
              type="card"
              size="large"
            />
          </Card>

          {/* Submit Button */}
          <Row justify="end">
            <Space>
              <Button onClick={() => navigate('/infrastructure')}>Cancel</Button>
              <Button 
                type="primary" 
                htmlType="submit" 
                loading={loading}
                icon={<RocketOutlined />}
                size="large"
                disabled={!PROVIDER_INFO[activeProvider].enabled}
              >
                Provision Infrastructure
              </Button>
            </Space>
          </Row>
        </Form>
      )}
    </div>
  );
}
