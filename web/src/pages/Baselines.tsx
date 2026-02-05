import { useEffect, useState } from 'react';
import {
  Card,
  Table,
  Button,
  Space,
  Tag,
  message,
  Typography,
  Row,
  Col,
  Empty,
  Spin,
  Tooltip,
  Popconfirm,
} from 'antd';
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  DeleteOutlined,
  EyeOutlined,
  DiffOutlined,
} from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import api from '@/api/client';
import { Baseline } from '@/types';
import dayjs from 'dayjs';

const { Title, Text } = Typography;

export default function Baselines() {
  const navigate = useNavigate();
  const [baselines, setBaselines] = useState<Baseline[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadBaselines();
  }, []);

  const loadBaselines = async () => {
    setLoading(true);
    try {
      const data = await api.getBaselines();
      setBaselines(data);
    } catch (error) {
      message.error('Failed to load baselines');
    } finally {
      setLoading(false);
    }
  };

  const deleteBaseline = async (id: string) => {
    try {
      await api.deleteBaseline(id);
      message.success('Baseline deleted');
      loadBaselines();
    } catch (error) {
      message.error('Failed to delete baseline');
    }
  };

  const toggleActive = async (baseline: Baseline) => {
    try {
      await api.updateBaseline(baseline.id, { active: !baseline.active });
      message.success(`Baseline ${baseline.active ? 'deactivated' : 'activated'}`);
      loadBaselines();
    } catch (error) {
      message.error('Failed to update baseline');
    }
  };

  const columns = [
    {
      title: 'Name',
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: Baseline) => (
        <Space>
          <Text strong style={{ color: 'rgba(255,255,255,0.85)' }}>{name}</Text>
          {record.active && <Tag color="green">Active</Tag>}
        </Space>
      ),
    },
    {
      title: 'Workload',
      dataIndex: ['metrics', 'workload'],
      key: 'workload',
      render: (workload: string) => <Tag color="blue">{workload || 'N/A'}</Tag>,
    },
    {
      title: 'Throughput',
      dataIndex: ['metrics', 'ops_per_second'],
      key: 'throughput',
      render: (val: number) => (
        <Text style={{ color: '#52c41a' }}>
          {val?.toLocaleString() || '-'} ops/sec
        </Text>
      ),
      sorter: (a: Baseline, b: Baseline) => 
        (a.metrics?.ops_per_second || 0) - (b.metrics?.ops_per_second || 0),
    },
    {
      title: 'Avg Latency',
      dataIndex: ['metrics', 'avg_latency_ms'],
      key: 'latency',
      render: (val: number) => `${val?.toFixed(3) || '-'} ms`,
      sorter: (a: Baseline, b: Baseline) => 
        (a.metrics?.avg_latency_ms || 0) - (b.metrics?.avg_latency_ms || 0),
    },
    {
      title: 'P99 Latency',
      dataIndex: ['metrics', 'p99_latency_ms'],
      key: 'p99',
      render: (val: number) => `${val?.toFixed(3) || '-'} ms`,
    },
    {
      title: 'Environment',
      key: 'environment',
      render: (_: any, record: Baseline) => {
        const env = record.environment;
        if (!env) return '-';
        return (
          <Tooltip title={`${env.host?.hostname} - ${env.redis?.version}`}>
            <Tag>{env.fingerprint?.slice(0, 8)}</Tag>
          </Tooltip>
        );
      },
    },
    {
      title: 'Created',
      dataIndex: 'created_at',
      key: 'created',
      render: (date: string) => dayjs(date).format('YYYY-MM-DD HH:mm'),
      sorter: (a: Baseline, b: Baseline) => 
        new Date(a.created_at).getTime() - new Date(b.created_at).getTime(),
      defaultSortOrder: 'descend' as const,
    },
    {
      title: 'Actions',
      key: 'actions',
      width: 200,
      render: (_: any, record: Baseline) => (
        <Space>
          <Tooltip title="View source run">
            <Button
              icon={<EyeOutlined />}
              size="small"
              onClick={() => navigate(`/runs/${record.run_id}`)}
            />
          </Tooltip>
          <Tooltip title="Compare">
            <Button
              icon={<DiffOutlined />}
              size="small"
              onClick={() => navigate(`/compare?baseline=${record.id}`)}
            />
          </Tooltip>
          <Tooltip title={record.active ? 'Deactivate' : 'Activate'}>
            <Button
              icon={record.active ? <CloseCircleOutlined /> : <CheckCircleOutlined />}
              size="small"
              onClick={() => toggleActive(record)}
              type={record.active ? 'default' : 'primary'}
            />
          </Tooltip>
          <Popconfirm
            title="Delete baseline?"
            description="This action cannot be undone."
            onConfirm={() => deleteBaseline(record.id)}
            okText="Delete"
            cancelText="Cancel"
            okButtonProps={{ danger: true }}
          >
            <Button icon={<DeleteOutlined />} size="small" danger />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Row justify="space-between" align="middle" style={{ marginBottom: 24 }}>
        <Col>
          <Title level={3} style={{ color: 'rgba(255,255,255,0.85)', margin: 0 }}>
            Baselines
          </Title>
          <Text type="secondary">
            Save benchmark results as baselines for comparison
          </Text>
        </Col>
      </Row>

      <Card style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
        {loading ? (
          <div style={{ textAlign: 'center', padding: 50 }}>
            <Spin size="large" />
          </div>
        ) : baselines.length === 0 ? (
          <Empty
            description="No baselines yet"
            image={Empty.PRESENTED_IMAGE_SIMPLE}
          >
            <Button type="primary" onClick={() => navigate('/runs')}>
              View Runs
            </Button>
          </Empty>
        ) : (
          <Table
            dataSource={baselines}
            columns={columns}
            rowKey="id"
            pagination={{
              pageSize: 10,
              showSizeChanger: true,
              showTotal: (total) => `${total} baselines`,
            }}
          />
        )}
      </Card>

      {/* Summary Cards */}
      {baselines.length > 0 && (
        <Row gutter={[16, 16]} style={{ marginTop: 24 }}>
          <Col xs={24} sm={8}>
            <Card style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
              <div style={{ textAlign: 'center' }}>
                <Text type="secondary">Total Baselines</Text>
                <Title level={2} style={{ color: '#1890ff', margin: '8px 0' }}>
                  {baselines.length}
                </Title>
              </div>
            </Card>
          </Col>
          <Col xs={24} sm={8}>
            <Card style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
              <div style={{ textAlign: 'center' }}>
                <Text type="secondary">Active Baselines</Text>
                <Title level={2} style={{ color: '#52c41a', margin: '8px 0' }}>
                  {baselines.filter(b => b.active).length}
                </Title>
              </div>
            </Card>
          </Col>
          <Col xs={24} sm={8}>
            <Card style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
              <div style={{ textAlign: 'center' }}>
                <Text type="secondary">Avg Throughput</Text>
                <Title level={2} style={{ color: '#DC382D', margin: '8px 0' }}>
                  {Math.round(
                    baselines.reduce((sum, b) => sum + (b.metrics?.ops_per_second || 0), 0) / 
                    baselines.length
                  ).toLocaleString()}
                </Title>
                <Text type="secondary">ops/sec</Text>
              </div>
            </Card>
          </Col>
        </Row>
      )}
    </div>
  );
}
