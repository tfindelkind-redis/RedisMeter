import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Card,
  Table,
  Button,
  Space,
  Typography,
  Tag,
  Tooltip,
  Popconfirm,
  message,
  Empty,
  Row,
  Col,
  Statistic,
  Badge,
} from 'antd';
import {
  CloudOutlined,
  PlusOutlined,
  DeleteOutlined,
  PlayCircleOutlined,
  EyeOutlined,
  ReloadOutlined,
  ClockCircleOutlined,
  CheckCircleOutlined,
  LoadingOutlined,
  CloseCircleOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import { ColumnsType } from 'antd/es/table';
import { Infrastructure, InfraStatus, CloudProvider } from '@/types';
import { PROVIDER_INFO } from '@/components/Infrastructure/providers';
import api from '@/api/client';

const { Title, Text } = Typography;

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

export default function InfrastructureList() {
  const navigate = useNavigate();
  const [infrastructures, setInfrastructures] = useState<Infrastructure[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadInfrastructures();
    // Poll for updates every 10 seconds
    const interval = setInterval(loadInfrastructures, 10000);
    return () => clearInterval(interval);
  }, []);

  const loadInfrastructures = async () => {
    try {
      const data = await api.getInfrastructures();
      setInfrastructures(data);
    } catch (error) {
      console.error('Failed to load infrastructures', error);
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await api.destroyInfrastructure(id);
      message.success('Infrastructure destruction initiated');
      loadInfrastructures();
    } catch (error: any) {
      message.error(error.message || 'Failed to destroy infrastructure');
    }
  };

  const handleRunBenchmark = (infra: Infrastructure) => {
    navigate(`/new?infra=${infra.id}`);
  };

  // Summary statistics
  const stats = {
    total: infrastructures.length,
    ready: infrastructures.filter(i => i.status === 'ready').length,
    provisioning: infrastructures.filter(i => i.status === 'provisioning').length,
    failed: infrastructures.filter(i => i.status === 'failed').length,
  };

  const columns: ColumnsType<Infrastructure> = [
    {
      title: 'Name',
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: Infrastructure) => (
        <Space direction="vertical" size={0}>
          <Text strong style={{ color: 'rgba(255,255,255,0.85)' }}>{name}</Text>
          <Text type="secondary" style={{ fontSize: 12 }}>{record.id}</Text>
        </Space>
      ),
    },
    {
      title: 'Provider',
      dataIndex: 'provider',
      key: 'provider',
      width: 120,
      render: (provider: CloudProvider) => {
        const info = PROVIDER_INFO[provider];
        return (
          <Tag color={providerColors[provider]}>
            {info?.name || provider}
          </Tag>
        );
      },
    },
    {
      title: 'Region',
      dataIndex: 'region',
      key: 'region',
      width: 140,
    },
    {
      title: 'Status',
      dataIndex: 'status',
      key: 'status',
      width: 140,
      render: (status: InfraStatus, record: Infrastructure) => {
        const config = statusConfig[status];
        return (
          <Tooltip title={record.error}>
            <Badge status={config.color as any} text={
              <Space>
                {config.icon}
                <span>{config.label}</span>
              </Space>
            } />
          </Tooltip>
        );
      },
    },
    {
      title: 'Resources',
      key: 'resources',
      width: 200,
      render: (_: any, record: Infrastructure) => (
        <Space direction="vertical" size={0}>
          {record.outputs?.redis_hostname && (
            <Text type="secondary" style={{ fontSize: 12 }}>
              Redis: {record.outputs.redis_hostname.split('.')[0]}
            </Text>
          )}
          {record.outputs?.runner_ips && (
            <Text type="secondary" style={{ fontSize: 12 }}>
              Runners: {record.outputs.runner_ips.length}
            </Text>
          )}
        </Space>
      ),
    },
    {
      title: 'Created',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 160,
      render: (date: string) => {
        const d = new Date(date);
        return (
          <Tooltip title={d.toLocaleString()}>
            <Text type="secondary">{formatRelativeTime(d)}</Text>
          </Tooltip>
        );
      },
    },
    {
      title: 'Expires',
      dataIndex: 'expires_at',
      key: 'expires_at',
      width: 120,
      render: (date: string) => {
        if (!date || date === '0001-01-01T00:00:00Z') return <Text type="secondary">-</Text>;
        const d = new Date(date);
        const isExpired = d < new Date();
        return (
          <Tooltip title={d.toLocaleString()}>
            <Text type={isExpired ? 'danger' : 'secondary'}>
              {isExpired ? 'Expired' : formatRelativeTime(d)}
            </Text>
          </Tooltip>
        );
      },
    },
    {
      title: 'Actions',
      key: 'actions',
      width: 180,
      render: (_: any, record: Infrastructure) => (
        <Space>
          <Tooltip title="View Details">
            <Button 
              type="text" 
              icon={<EyeOutlined />} 
              onClick={() => navigate(`/infrastructure/${record.id}`)}
            />
          </Tooltip>
          {record.status === 'ready' && (
            <Tooltip title="Run Benchmark">
              <Button 
                type="text" 
                icon={<PlayCircleOutlined style={{ color: '#52c41a' }} />}
                onClick={() => handleRunBenchmark(record)}
              />
            </Tooltip>
          )}
          {['ready', 'failed'].includes(record.status) && (
            <Popconfirm
              title="Destroy Infrastructure"
              description="This will permanently delete all resources. Continue?"
              onConfirm={() => handleDelete(record.id)}
              okText="Destroy"
              okButtonProps={{ danger: true }}
            >
              <Tooltip title="Destroy">
                <Button type="text" danger icon={<DeleteOutlined />} />
              </Tooltip>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Row justify="space-between" align="middle" style={{ marginBottom: 24 }}>
        <Col>
          <Title level={3} style={{ color: 'rgba(255,255,255,0.85)', margin: 0 }}>
            <CloudOutlined style={{ marginRight: 8, color: '#0078d4' }} />
            Cloud Infrastructure
          </Title>
        </Col>
        <Col>
          <Space>
            <Button icon={<ReloadOutlined />} onClick={loadInfrastructures}>
              Refresh
            </Button>
            <Button 
              type="primary" 
              icon={<PlusOutlined />}
              onClick={() => navigate('/infrastructure/new')}
            >
              New Infrastructure
            </Button>
          </Space>
        </Col>
      </Row>

      {/* Statistics */}
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={6}>
          <Card style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
            <Statistic 
              title="Total" 
              value={stats.total} 
              prefix={<CloudOutlined />}
              valueStyle={{ color: 'rgba(255,255,255,0.85)' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
            <Statistic 
              title="Ready" 
              value={stats.ready} 
              prefix={<CheckCircleOutlined style={{ color: '#52c41a' }} />}
              valueStyle={{ color: '#52c41a' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
            <Statistic 
              title="Provisioning" 
              value={stats.provisioning} 
              prefix={stats.provisioning > 0 ? <LoadingOutlined style={{ color: '#1890ff' }} /> : <ClockCircleOutlined style={{ color: 'rgba(255,255,255,0.45)' }} />}
              valueStyle={{ color: stats.provisioning > 0 ? '#1890ff' : 'rgba(255,255,255,0.45)' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
            <Statistic 
              title="Failed" 
              value={stats.failed} 
              prefix={<WarningOutlined style={{ color: '#ff4d4f' }} />}
              valueStyle={{ color: stats.failed > 0 ? '#ff4d4f' : 'rgba(255,255,255,0.45)' }}
            />
          </Card>
        </Col>
      </Row>

      {/* Infrastructure Table */}
      <Card style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
        <Table
          columns={columns}
          dataSource={infrastructures}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 10 }}
          locale={{
            emptyText: (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description="No infrastructure deployed"
              >
                <Button 
                  type="primary" 
                  icon={<PlusOutlined />}
                  onClick={() => navigate('/infrastructure/new')}
                >
                  Create Infrastructure
                </Button>
              </Empty>
            ),
          }}
        />
      </Card>
    </div>
  );
}

function formatRelativeTime(date: Date): string {
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMins / 60);
  const diffDays = Math.floor(diffHours / 24);

  if (diffMins < 1) return 'Just now';
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays < 7) return `${diffDays}d ago`;
  return date.toLocaleDateString();
}
