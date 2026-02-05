import { useState } from 'react';
import {
  Table,
  Card,
  Input,
  Select,
  Button,
  Space,
  Tag,
  Typography,
  Popconfirm,
  message,
  Row,
  Col,
} from 'antd';
import {
  SearchOutlined,
  DeleteOutlined,
  EyeOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { useStore } from '@/store';
import StatusBadge from '@/components/StatusBadge';
import { BenchmarkRun } from '@/types';
import dayjs from 'dayjs';

const { Title, Text } = Typography;

export default function Runs() {
  const navigate = useNavigate();
  const { runs, loadingRuns, fetchRuns, deleteRun } = useStore();
  const [searchText, setSearchText] = useState('');
  const [statusFilter, setStatusFilter] = useState<string | undefined>();

  const handleDelete = async (id: string) => {
    try {
      await deleteRun(id);
      message.success('Run deleted successfully');
    } catch (error) {
      message.error('Failed to delete run');
    }
  };

  const filteredRuns = runs.filter((run) => {
    const matchesSearch = !searchText || 
      run.name?.toLowerCase().includes(searchText.toLowerCase()) ||
      run.id.toLowerCase().includes(searchText.toLowerCase()) ||
      run.workload?.name.toLowerCase().includes(searchText.toLowerCase());
    
    const matchesStatus = !statusFilter || run.status === statusFilter;

    return matchesSearch && matchesStatus;
  });

  const columns = [
    {
      title: 'Name',
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: BenchmarkRun) => (
        <a onClick={() => navigate(`/runs/${record.id}`)}>
          {name || record.id.slice(0, 8)}
        </a>
      ),
      sorter: (a: BenchmarkRun, b: BenchmarkRun) => 
        (a.name || a.id).localeCompare(b.name || b.id),
    },
    {
      title: 'Workload',
      dataIndex: ['workload', 'name'],
      key: 'workload',
      render: (name: string) => <Tag color="blue">{name}</Tag>,
      filters: [...new Set(runs.map(r => r.workload?.name).filter((w): w is string => Boolean(w)))].map(w => ({
        text: w,
        value: w,
      })),
      onFilter: (value: unknown, record: BenchmarkRun) => record.workload?.name === value,
    },
    {
      title: 'Target',
      key: 'target',
      render: (_: unknown, record: BenchmarkRun) => (
        <Text code style={{ fontSize: 12 }}>
          {record.target?.host}:{record.target?.port}
        </Text>
      ),
    },
    {
      title: 'Status',
      dataIndex: 'status',
      key: 'status',
      render: (status: BenchmarkRun['status']) => <StatusBadge status={status} />,
      filters: [
        { text: 'Completed', value: 'completed' },
        { text: 'Running', value: 'running' },
        { text: 'Failed', value: 'failed' },
        { text: 'Pending', value: 'pending' },
      ],
      onFilter: (value: unknown, record: BenchmarkRun) => record.status === value,
    },
    {
      title: 'Throughput',
      key: 'throughput',
      sorter: (a: BenchmarkRun, b: BenchmarkRun) => 
        (a.results?.summary?.ops_per_second || 0) - (b.results?.summary?.ops_per_second || 0),
      render: (_: unknown, record: BenchmarkRun) => {
        const ops = record.results?.summary?.ops_per_second;
        return ops ? (
          <Text strong style={{ color: '#52c41a' }}>
            {ops.toLocaleString(undefined, { maximumFractionDigits: 0 })} ops/sec
          </Text>
        ) : '-';
      },
    },
    {
      title: 'Avg Latency',
      key: 'avgLatency',
      sorter: (a: BenchmarkRun, b: BenchmarkRun) => 
        (a.results?.summary?.avg_latency_ms || 0) - (b.results?.summary?.avg_latency_ms || 0),
      render: (_: unknown, record: BenchmarkRun) => {
        const lat = record.results?.summary?.avg_latency_ms;
        return lat ? `${lat.toFixed(3)} ms` : '-';
      },
    },
    {
      title: 'P99 Latency',
      key: 'p99Latency',
      sorter: (a: BenchmarkRun, b: BenchmarkRun) => 
        (a.results?.summary?.p99_latency_ms || 0) - (b.results?.summary?.p99_latency_ms || 0),
      render: (_: unknown, record: BenchmarkRun) => {
        const lat = record.results?.summary?.p99_latency_ms;
        return lat ? `${lat.toFixed(3)} ms` : '-';
      },
    },
    {
      title: 'Tags',
      dataIndex: 'tags',
      key: 'tags',
      render: (tags: string[]) => (
        <Space size={[0, 4]} wrap>
          {tags?.map((tag) => (
            <Tag key={tag} style={{ margin: 0 }}>{tag}</Tag>
          ))}
        </Space>
      ),
    },
    {
      title: 'Created',
      dataIndex: 'created_at',
      key: 'created_at',
      sorter: (a: BenchmarkRun, b: BenchmarkRun) => 
        dayjs(a.created_at).unix() - dayjs(b.created_at).unix(),
      defaultSortOrder: 'descend' as const,
      render: (date: string) => (
        <Text style={{ color: 'rgba(255,255,255,0.65)' }}>
          {dayjs(date).format('MMM D, YYYY HH:mm')}
        </Text>
      ),
    },
    {
      title: 'Actions',
      key: 'actions',
      render: (_: unknown, record: BenchmarkRun) => (
        <Space>
          <Button
            type="text"
            icon={<EyeOutlined />}
            onClick={() => navigate(`/runs/${record.id}`)}
          />
          <Popconfirm
            title="Delete this run?"
            onConfirm={() => handleDelete(record.id)}
            okText="Yes"
            cancelText="No"
          >
            <Button type="text" danger icon={<DeleteOutlined />} />
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
            Benchmark Runs
          </Title>
        </Col>
        <Col>
          <Button icon={<ReloadOutlined />} onClick={() => fetchRuns()}>
            Refresh
          </Button>
        </Col>
      </Row>

      <Card style={{ background: '#1f1f1f', border: '1px solid #303030', marginBottom: 16 }}>
        <Space size="middle">
          <Input
            placeholder="Search runs..."
            prefix={<SearchOutlined />}
            value={searchText}
            onChange={(e) => setSearchText(e.target.value)}
            style={{ width: 250 }}
            allowClear
          />
          <Select
            placeholder="Filter by status"
            value={statusFilter}
            onChange={setStatusFilter}
            allowClear
            style={{ width: 150 }}
            options={[
              { label: 'Completed', value: 'completed' },
              { label: 'Running', value: 'running' },
              { label: 'Failed', value: 'failed' },
              { label: 'Pending', value: 'pending' },
              { label: 'Cancelled', value: 'cancelled' },
            ]}
          />
        </Space>
      </Card>

      <Card style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
        <Table
          columns={columns}
          dataSource={filteredRuns}
          rowKey="id"
          loading={loadingRuns}
          pagination={{
            pageSize: 20,
            showSizeChanger: true,
            showTotal: (total) => `${total} runs`,
          }}
        />
      </Card>
    </div>
  );
}
