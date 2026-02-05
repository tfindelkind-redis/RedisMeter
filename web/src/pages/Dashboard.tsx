import { Row, Col, Card, Typography, Table, Tag, Empty } from 'antd';
import { useNavigate } from 'react-router-dom';
import { useStore } from '@/store';
import MetricCard from '@/components/MetricCard';
import StatusBadge from '@/components/StatusBadge';
import { ThroughputChart, LatencyDistribution } from '@/components/Charts';
import { BenchmarkRun } from '@/types';
import dayjs from 'dayjs';
import relativeTime from 'dayjs/plugin/relativeTime';

dayjs.extend(relativeTime);

const { Title, Text } = Typography;

export default function Dashboard() {
  const navigate = useNavigate();
  const { runs, loadingRuns } = useStore();

  // Get recent completed runs
  const completedRuns = runs.filter((r) => r.status === 'completed');
  const recentRuns = completedRuns.slice(0, 5);
  const latestRun = completedRuns[0];

  // Calculate aggregate stats
  const avgThroughput = completedRuns.length > 0
    ? completedRuns.reduce((sum, r) => sum + (r.results?.summary?.ops_per_second || 0), 0) / completedRuns.length
    : 0;
  
  const avgLatency = completedRuns.length > 0
    ? completedRuns.reduce((sum, r) => sum + (r.results?.summary?.avg_latency_ms || 0), 0) / completedRuns.length
    : 0;

  const totalRuns = runs.length;
  const failedRuns = runs.filter((r) => r.status === 'failed').length;

  // Get throughput trend data
  const throughputTrend = completedRuns
    .slice(0, 20)
    .reverse()
    .map((r) => r.results?.summary?.ops_per_second || 0);

  const columns = [
    {
      title: 'Name',
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: BenchmarkRun) => (
        <a onClick={() => navigate(`/runs/${record.id}`)}>{name || record.id.slice(0, 8)}</a>
      ),
    },
    {
      title: 'Workload',
      dataIndex: ['workload', 'name'],
      key: 'workload',
      render: (name: string) => <Tag>{name}</Tag>,
    },
    {
      title: 'Status',
      dataIndex: 'status',
      key: 'status',
      render: (status: BenchmarkRun['status']) => <StatusBadge status={status} size="small" />,
    },
    {
      title: 'Ops/sec',
      key: 'throughput',
      render: (_: unknown, record: BenchmarkRun) => (
        <Text style={{ color: '#52c41a' }}>
          {record.results?.summary?.ops_per_second.toLocaleString(undefined, { maximumFractionDigits: 0 }) || '-'}
        </Text>
      ),
    },
    {
      title: 'P99 Latency',
      key: 'p99',
      render: (_: unknown, record: BenchmarkRun) => (
        <Text>
          {record.results?.summary?.p99_latency_ms.toFixed(2) || '-'} ms
        </Text>
      ),
    },
    {
      title: 'Time',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (date: string) => dayjs(date).fromNow(),
    },
  ];

  return (
    <div>
      <Title level={3} style={{ color: 'rgba(255,255,255,0.85)', marginBottom: 24 }}>
        Dashboard
      </Title>

      {/* Summary Stats */}
      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
        <Col xs={24} sm={12} lg={6}>
          <MetricCard
            title="Total Runs"
            value={totalRuns}
            precision={0}
          />
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <MetricCard
            title="Avg Throughput"
            value={avgThroughput}
            suffix="ops/sec"
            sparklineData={throughputTrend}
            precision={0}
            valueStyle={{ color: '#52c41a' }}
          />
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <MetricCard
            title="Avg Latency"
            value={avgLatency}
            suffix="ms"
            precision={3}
            valueStyle={{ color: '#1890ff' }}
          />
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <MetricCard
            title="Failed Runs"
            value={failedRuns}
            precision={0}
            valueStyle={failedRuns > 0 ? { color: '#ff4d4f' } : { color: '#52c41a' }}
          />
        </Col>
      </Row>

      {/* Charts Row */}
      {latestRun?.results?.time_series && latestRun.results.time_series.length > 0 ? (
        <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
          <Col xs={24} lg={14}>
            <ThroughputChart data={latestRun.results.time_series} height={280} />
          </Col>
          <Col xs={24} lg={10}>
            {latestRun.results.summary && (
              <LatencyDistribution
                data={{
                  min: latestRun.results.summary.min_latency_ms,
                  avg: latestRun.results.summary.avg_latency_ms,
                  p50: latestRun.results.summary.p50_latency_ms,
                  p90: latestRun.results.summary.p90_latency_ms,
                  p95: latestRun.results.summary.p95_latency_ms,
                  p99: latestRun.results.summary.p99_latency_ms,
                  p999: latestRun.results.summary.p999_latency_ms,
                  max: latestRun.results.summary.max_latency_ms,
                }}
                height={280}
              />
            )}
          </Col>
        </Row>
      ) : (
        <Card style={{ marginBottom: 24, background: '#1f1f1f', border: '1px solid #303030' }}>
          <Empty
            description={
              <Text style={{ color: 'rgba(255,255,255,0.45)' }}>
                No benchmark data available. Run a benchmark to see charts.
              </Text>
            }
          />
        </Card>
      )}

      {/* Recent Runs Table */}
      <Card
        title={<span style={{ color: 'rgba(255,255,255,0.85)' }}>Recent Benchmarks</span>}
        style={{ background: '#1f1f1f', border: '1px solid #303030' }}
        extra={
          <a onClick={() => navigate('/runs')}>View All</a>
        }
      >
        <Table
          columns={columns}
          dataSource={recentRuns}
          rowKey="id"
          loading={loadingRuns}
          pagination={false}
          size="small"
          style={{ background: 'transparent' }}
        />
      </Card>
    </div>
  );
}
