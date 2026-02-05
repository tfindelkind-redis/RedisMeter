import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Card,
  Typography,
  Space,
  Tag,
  Descriptions,
  Button,
  Row,
  Col,
  Statistic,
  Popconfirm,
  message,
  Spin,
  Alert,
  Divider,
  List,
  Tooltip,
  Badge,
} from 'antd';
import {
  CloudOutlined,
  ArrowLeftOutlined,
  DeleteOutlined,
  PlayCircleOutlined,
  ReloadOutlined,
  CheckCircleOutlined,
  LoadingOutlined,
  CloseCircleOutlined,
  ClockCircleOutlined,
  CopyOutlined,
  GlobalOutlined,
  DatabaseOutlined,
  DesktopOutlined,
  KeyOutlined,
} from '@ant-design/icons';
import { Infrastructure, InfraStatus, CloudProvider } from '@/types';
import { PROVIDER_INFO } from '@/components/Infrastructure/providers';
import api from '@/api/client';

const { Title, Text, Paragraph } = Typography;

const statusConfig: Record<InfraStatus, { color: string; icon: React.ReactNode; label: string }> = {
  pending: { color: 'default', icon: <ClockCircleOutlined />, label: 'Pending' },
  provisioning: { color: 'processing', icon: <LoadingOutlined />, label: 'Provisioning' },
  ready: { color: 'success', icon: <CheckCircleOutlined />, label: 'Ready' },
  failed: { color: 'error', icon: <CloseCircleOutlined />, label: 'Failed' },
  destroying: { color: 'warning', icon: <LoadingOutlined />, label: 'Destroying' },
  destroyed: { color: 'default', icon: <DeleteOutlined />, label: 'Destroyed' },
};

const providerColors: Record<CloudProvider, string> = {
  azure: '#0078d4',
  aws: '#ff9900',
  gcp: '#4285f4',
  kubernetes: '#326ce5',
  vmware: '#607078',
  local: '#52c41a',
};

export default function InfrastructureDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [infra, setInfra] = useState<Infrastructure | null>(null);
  const [loading, setLoading] = useState(true);
  const [destroying, setDestroying] = useState(false);

  useEffect(() => {
    if (id) {
      loadInfrastructure();
      // Poll for updates if provisioning
      const interval = setInterval(() => {
        if (infra?.status === 'provisioning' || infra?.status === 'destroying') {
          loadInfrastructure();
        }
      }, 5000);
      return () => clearInterval(interval);
    }
  }, [id, infra?.status]);

  const loadInfrastructure = async () => {
    try {
      const data = await api.getInfrastructure(id!);
      setInfra(data);
    } catch (error) {
      message.error('Failed to load infrastructure');
      navigate('/infrastructure');
    } finally {
      setLoading(false);
    }
  };

  const handleDestroy = async () => {
    if (!infra) return;
    setDestroying(true);
    try {
      await api.destroyInfrastructure(infra.id);
      message.success('Infrastructure destruction initiated');
      loadInfrastructure();
    } catch (error: any) {
      message.error(error.message || 'Failed to destroy infrastructure');
    } finally {
      setDestroying(false);
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    message.success('Copied to clipboard');
  };

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: 100 }}>
        <Spin size="large" />
      </div>
    );
  }

  if (!infra) {
    return (
      <Alert
        type="error"
        message="Infrastructure not found"
        description="The requested infrastructure could not be found."
        showIcon
      />
    );
  }

  const status = statusConfig[infra.status];
  const providerInfo = PROVIDER_INFO[infra.provider];

  return (
    <div>
      {/* Header */}
      <Row justify="space-between" align="middle" style={{ marginBottom: 24 }}>
        <Col>
          <Space>
            <Button 
              icon={<ArrowLeftOutlined />} 
              onClick={() => navigate('/infrastructure')}
            >
              Back
            </Button>
            <Title level={3} style={{ color: 'rgba(255,255,255,0.85)', margin: 0 }}>
              <CloudOutlined style={{ marginRight: 8, color: providerColors[infra.provider] }} />
              {infra.name}
            </Title>
            <Tag color={providerColors[infra.provider]}>{providerInfo?.name}</Tag>
            <Badge status={status.color as any} text={
              <Space>
                {status.icon}
                <span>{status.label}</span>
              </Space>
            } />
          </Space>
        </Col>
        <Col>
          <Space>
            <Button icon={<ReloadOutlined />} onClick={loadInfrastructure}>
              Refresh
            </Button>
            {infra.status === 'ready' && (
              <Button 
                type="primary"
                icon={<PlayCircleOutlined />}
                onClick={() => navigate(`/new?infra=${infra.id}`)}
              >
                Run Benchmark
              </Button>
            )}
            {['ready', 'failed'].includes(infra.status) && (
              <Popconfirm
                title="Destroy Infrastructure"
                description="This will permanently delete all resources. Continue?"
                onConfirm={handleDestroy}
                okText="Destroy"
                okButtonProps={{ danger: true }}
              >
                <Button danger icon={<DeleteOutlined />} loading={destroying}>
                  Destroy
                </Button>
              </Popconfirm>
            )}
          </Space>
        </Col>
      </Row>

      {/* Error Alert */}
      {infra.error && (
        <Alert
          type="error"
          message="Provisioning Failed"
          description={infra.error}
          showIcon
          style={{ marginBottom: 24 }}
        />
      )}

      {/* Provisioning Progress */}
      {infra.status === 'provisioning' && (
        <Alert
          type="info"
          message="Provisioning in Progress"
          description="Infrastructure is being provisioned. This may take 5-15 minutes for Azure Managed Redis."
          showIcon
          icon={<LoadingOutlined />}
          style={{ marginBottom: 24 }}
        />
      )}

      <Row gutter={24}>
        {/* Left Column - Details */}
        <Col span={16}>
          {/* Basic Information */}
          <Card 
            title="Infrastructure Details"
            style={{ background: '#1f1f1f', border: '1px solid #303030', marginBottom: 24 }}
          >
            <Descriptions column={2} size="small">
              <Descriptions.Item label="ID">
                <Space>
                  <Text copyable={{ text: infra.id }}>{infra.id}</Text>
                </Space>
              </Descriptions.Item>
              <Descriptions.Item label="Name">{infra.name}</Descriptions.Item>
              <Descriptions.Item label="Provider">
                <Tag color={providerColors[infra.provider]}>{providerInfo?.name}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="Region">{infra.region}</Descriptions.Item>
              <Descriptions.Item label="Created">
                {new Date(infra.created_at).toLocaleString()}
              </Descriptions.Item>
              <Descriptions.Item label="Updated">
                {new Date(infra.updated_at).toLocaleString()}
              </Descriptions.Item>
              {infra.expires_at && infra.expires_at !== '0001-01-01T00:00:00Z' && (
                <Descriptions.Item label="Expires">
                  {new Date(infra.expires_at).toLocaleString()}
                </Descriptions.Item>
              )}
            </Descriptions>
          </Card>

          {/* Connection Details */}
          {infra.status === 'ready' && infra.outputs && (
            <>
              {/* Redis Connection */}
              {infra.outputs.redis_hostname && (
                <Card 
                  title={
                    <Space>
                      <DatabaseOutlined style={{ color: '#DC382D' }} />
                      <span>Redis Connection</span>
                    </Space>
                  }
                  style={{ background: '#1f1f1f', border: '1px solid #303030', marginBottom: 24 }}
                >
                  <Descriptions column={1} size="small">
                    <Descriptions.Item label="Host">
                      <Space>
                        <Text code>{infra.outputs.redis_hostname}</Text>
                        <Tooltip title="Copy">
                          <Button 
                            type="text" 
                            size="small"
                            icon={<CopyOutlined />}
                            onClick={() => copyToClipboard(infra.outputs?.redis_hostname || '')}
                          />
                        </Tooltip>
                      </Space>
                    </Descriptions.Item>
                    <Descriptions.Item label="Port">
                      <Text code>{infra.outputs.redis_port || 10000}</Text>
                    </Descriptions.Item>
                    <Descriptions.Item label="Connection String">
                      <Space>
                        <Text code style={{ wordBreak: 'break-all' }}>
                          redis://{infra.outputs.redis_hostname}:{infra.outputs.redis_port || 10000}
                        </Text>
                        <Tooltip title="Copy">
                          <Button 
                            type="text" 
                            size="small"
                            icon={<CopyOutlined />}
                            onClick={() => copyToClipboard(
                              `redis://${infra.outputs?.redis_hostname}:${infra.outputs?.redis_port || 10000}`
                            )}
                          />
                        </Tooltip>
                      </Space>
                    </Descriptions.Item>
                    {infra.outputs.redis_password && (
                      <Descriptions.Item label="Password">
                        <Space>
                          <Text type="secondary">********</Text>
                          <Tooltip title="Copy Password">
                            <Button 
                              type="text" 
                              size="small"
                              icon={<KeyOutlined />}
                              onClick={() => copyToClipboard(infra.outputs?.redis_password || '')}
                            />
                          </Tooltip>
                        </Space>
                      </Descriptions.Item>
                    )}
                  </Descriptions>
                </Card>
              )}

              {/* Runner VMs */}
              {infra.outputs.runner_ips && infra.outputs.runner_ips.length > 0 && (
                <Card 
                  title={
                    <Space>
                      <DesktopOutlined style={{ color: '#52c41a' }} />
                      <span>Runner VMs ({infra.outputs.runner_ips.length})</span>
                    </Space>
                  }
                  style={{ background: '#1f1f1f', border: '1px solid #303030', marginBottom: 24 }}
                >
                  <List
                    dataSource={infra.outputs.runner_ips.map((ip, i) => ({
                      ip,
                      privateIp: infra.outputs?.runner_private_ips?.[i],
                      index: i,
                    }))}
                    renderItem={(item) => (
                      <List.Item>
                        <List.Item.Meta
                          avatar={
                            <div style={{
                              width: 32,
                              height: 32,
                              borderRadius: '50%',
                              background: '#52c41a',
                              display: 'flex',
                              alignItems: 'center',
                              justifyContent: 'center',
                              color: 'white',
                              fontWeight: 'bold',
                            }}>
                              {item.index + 1}
                            </div>
                          }
                          title={
                            <Space>
                              <Text>Runner {item.index + 1}</Text>
                              <Text code>{item.ip}</Text>
                              <Tooltip title="Copy">
                                <Button 
                                  type="text" 
                                  size="small"
                                  icon={<CopyOutlined />}
                                  onClick={() => copyToClipboard(item.ip)}
                                />
                              </Tooltip>
                            </Space>
                          }
                          description={
                            <Space>
                              <GlobalOutlined />
                              <Text type="secondary">Private: {item.privateIp}</Text>
                            </Space>
                          }
                        />
                        <Space>
                          <Tooltip title="SSH Command">
                            <Button 
                              size="small"
                              onClick={() => copyToClipboard(
                                `ssh ${infra.config?.runners?.ssh_user || 'azureuser'}@${item.ip}`
                              )}
                            >
                              Copy SSH
                            </Button>
                          </Tooltip>
                        </Space>
                      </List.Item>
                    )}
                  />
                </Card>
              )}
            </>
          )}

          {/* Configuration */}
          <Card 
            title="Configuration"
            style={{ background: '#1f1f1f', border: '1px solid #303030' }}
          >
            <Descriptions column={2} size="small">
              {infra.config?.runners && (
                <>
                  <Descriptions.Item label="Runner Count">
                    {infra.config.runners.count}
                  </Descriptions.Item>
                  <Descriptions.Item label="Instance Type">
                    {infra.config.runners.instance_type}
                  </Descriptions.Item>
                  <Descriptions.Item label="Spot Instances">
                    {infra.config.runners.spot_instances ? 'Yes' : 'No'}
                  </Descriptions.Item>
                  <Descriptions.Item label="SSH User">
                    {infra.config.runners.ssh_user}
                  </Descriptions.Item>
                </>
              )}
              {infra.config?.amr && (
                <>
                  <Descriptions.Item label="AMR SKU">
                    <Tag color="blue">{infra.config.amr.sku}</Tag>
                  </Descriptions.Item>
                  <Descriptions.Item label="High Availability">
                    {infra.config.amr.high_availability ? 'Enabled' : 'Disabled'}
                  </Descriptions.Item>
                  <Descriptions.Item label="Clustering">
                    {infra.config.amr.clustering_policy}
                  </Descriptions.Item>
                  <Descriptions.Item label="Modules">
                    {infra.config.amr.modules?.join(', ') || 'None'}
                  </Descriptions.Item>
                </>
              )}
            </Descriptions>
          </Card>
        </Col>

        {/* Right Column - Quick Actions */}
        <Col span={8}>
          {/* Quick Stats */}
          <Card 
            title="Quick Stats"
            style={{ background: '#1f1f1f', border: '1px solid #303030', marginBottom: 24 }}
          >
            <Row gutter={[16, 16]}>
              <Col span={12}>
                <Statistic
                  title="Runners"
                  value={infra.outputs?.runner_ips?.length || infra.config?.runners?.count || 0}
                  prefix={<DesktopOutlined />}
                  valueStyle={{ color: 'rgba(255,255,255,0.85)' }}
                />
              </Col>
              <Col span={12}>
                <Statistic
                  title="Status"
                  value={status.label}
                  valueStyle={{ color: status.color === 'success' ? '#52c41a' : 'rgba(255,255,255,0.85)' }}
                />
              </Col>
            </Row>
          </Card>

          {/* Quick Commands */}
          {infra.status === 'ready' && (
            <Card 
              title="Quick Commands"
              style={{ background: '#1f1f1f', border: '1px solid #303030' }}
            >
              <Space direction="vertical" style={{ width: '100%' }}>
                <Button 
                  block 
                  icon={<PlayCircleOutlined />}
                  onClick={() => navigate(`/new?infra=${infra.id}`)}
                >
                  Run Benchmark
                </Button>
                <Divider style={{ margin: '12px 0' }} />
                <Paragraph type="secondary" style={{ fontSize: 12 }}>
                  CLI Commands:
                </Paragraph>
                <Button 
                  block 
                  size="small"
                  onClick={() => copyToClipboard(
                    `redismeter cloud run cache --infra ${infra.id}`
                  )}
                >
                  Copy: Run Benchmark
                </Button>
                <Button 
                  block 
                  size="small"
                  onClick={() => copyToClipboard(
                    `redismeter infra status ${infra.id}`
                  )}
                >
                  Copy: Check Status
                </Button>
                <Button 
                  block 
                  size="small"
                  danger
                  onClick={() => copyToClipboard(
                    `redismeter infra down ${infra.id}`
                  )}
                >
                  Copy: Destroy
                </Button>
              </Space>
            </Card>
          )}
        </Col>
      </Row>
    </div>
  );
}
