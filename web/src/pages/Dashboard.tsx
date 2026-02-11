import { Row, Col, Card, Typography, Table, Tag, Empty, Progress, Tooltip } from 'antd';
import { useNavigate } from 'react-router-dom';
import { useStore } from '@/store';
import StatusBadge from '@/components/StatusBadge';
import { BenchmarkRun } from '@/types';
import dayjs from 'dayjs';
import relativeTime from 'dayjs/plugin/relativeTime';
import {
  ArrowUpOutlined,
  ArrowDownOutlined,
  ClockCircleOutlined,
  ThunderboltOutlined,
  DashboardOutlined,
  CheckCircleOutlined,
} from '@ant-design/icons';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip as RechartsTooltip,
  ResponsiveContainer,
  Cell,
} from 'recharts';

dayjs.extend(relativeTime);

const { Title, Text } = Typography;

// Helper to format large numbers
const formatNumber = (num: number, precision = 0) => {
  if (num >= 1000000) return `${(num / 1000000).toFixed(1)}M`;
  if (num >= 1000) return `${(num / 1000).toFixed(precision > 0 ? 1 : 0)}K`;
  return num.toFixed(precision);
};

interface StatCardProps {
  icon: React.ReactNode;
  label: string;
  value: string | number;
  suffix?: string;
  subtext?: string;
  trend?: number; // percentage change
  color?: string;
  onClick?: () => void;
}

function StatCard({ icon, label, value, suffix, subtext, trend, color = '#DC382D', onClick }: StatCardProps) {
  return (
    <Card
      style={{ 
        background: '#1f1f1f', 
        border: '1px solid #303030',
        cursor: onClick ? 'pointer' : 'default',
      }}
      styles={{ body: { padding: 20 } }}
      onClick={onClick}
      hoverable={!!onClick}
    >
      <div style={{ display: 'flex', alignItems: 'flex-start', gap: 12 }}>
        <div style={{ 
          background: `${color}20`, 
          padding: 8, 
          borderRadius: 8,
          color: color,
          fontSize: 20,
        }}>
          {icon}
        </div>
        <div style={{ flex: 1 }}>
          <Text style={{ color: 'rgba(255,255,255,0.65)', fontSize: 12 }}>{label}</Text>
          <div style={{ display: 'flex', alignItems: 'baseline', gap: 4, marginTop: 4 }}>
            <Text style={{ color: 'rgba(255,255,255,0.95)', fontSize: 24, fontWeight: 600 }}>
              {value}
            </Text>
            {suffix && (
              <Text style={{ color, fontSize: 14, fontWeight: 500 }}>{suffix}</Text>
            )}
          </div>
          {(subtext || trend !== undefined) && (
            <div style={{ marginTop: 4, display: 'flex', alignItems: 'center', gap: 8 }}>
              {trend !== undefined && trend !== 0 && (
                <span style={{ 
                  color: trend > 0 ? '#52c41a' : '#ff4d4f',
                  fontSize: 12,
                  display: 'flex',
                  alignItems: 'center',
                  gap: 2,
                }}>
                  {trend > 0 ? <ArrowUpOutlined /> : <ArrowDownOutlined />}
                  {Math.abs(trend).toFixed(1)}%
                </span>
              )}
              {subtext && (
                <Text style={{ color: 'rgba(255,255,255,0.45)', fontSize: 12 }}>{subtext}</Text>
              )}
            </div>
          )}
        </div>
      </div>
    </Card>
  );
}

export default function Dashboard() {
  const navigate = useNavigate();
  const { runs, loadingRuns } = useStore();

  // Get completed runs sorted by date (newest first)
  const completedRuns = runs
    .filter((r) => r.status === 'completed')
    .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
  
  const recentRuns = completedRuns.slice(0, 5);
  const latestRun = completedRuns[0];
  const previousRun = completedRuns[1];

  // Calculate meaningful stats
  const totalRuns = runs.length;
  const successRate = totalRuns > 0 
    ? ((completedRuns.length / totalRuns) * 100)
    : 0;

  // Latest run metrics
  const latestThroughput = latestRun?.results?.summary?.ops_per_second || 0;
  const latestP99 = latestRun?.results?.summary?.p99_latency_ms || 0;
  
  // Previous run for trend calculation
  const previousThroughput = previousRun?.results?.summary?.ops_per_second || 0;
  const previousP99 = previousRun?.results?.summary?.p99_latency_ms || 0;
  
  // Trend calculations (positive = improvement)
  const throughputTrend = previousThroughput > 0 
    ? ((latestThroughput - previousThroughput) / previousThroughput) * 100 
    : 0;
  // For latency, lower is better, so invert the trend
  const latencyTrend = previousP99 > 0 
    ? ((previousP99 - latestP99) / previousP99) * 100 
    : 0;

  // Best performance ever achieved
  const bestThroughput = completedRuns.reduce((best, r) => {
    const ops = r.results?.summary?.ops_per_second || 0;
    return ops > best ? ops : best;
  }, 0);

  // Prepare chart data - last 10 runs with throughput
  const chartData = completedRuns
    .slice(0, 10)
    .reverse()
    .map((r, idx) => ({
      name: dayjs(r.created_at).format('MM/DD HH:mm'),
      shortName: `#${10 - idx}`,
      throughput: r.results?.summary?.ops_per_second || 0,
      p99: r.results?.summary?.p99_latency_ms || 0,
      workload: r.workload?.name || 'unknown',
      id: r.id,
      isLatest: r.id === latestRun?.id,
    }));

  const columns = [
    {
      title: 'Name',
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: BenchmarkRun) => (
        <a onClick={() => navigate(`/runs/${record.id}`)}>
          {name || record.id.split('-')[0]}
        </a>
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
          {record.results?.summary?.ops_per_second
            ? formatNumber(record.results.summary.ops_per_second)
            : '-'}
        </Text>
      ),
    },
    {
      title: 'P99 Latency',
      key: 'p99',
      render: (_: unknown, record: BenchmarkRun) => (
        <Text>
          {record.results?.summary?.p99_latency_ms?.toFixed(2) || '-'} ms
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
          <StatCard
            icon={<DashboardOutlined />}
            label="Total Benchmark Runs"
            value={totalRuns}
            color="#1890ff"
            subtext={completedRuns.length > 0 ? `${completedRuns.length} completed` : undefined}
            onClick={() => navigate('/runs')}
          />
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Tooltip title={latestRun ? `From: ${latestRun.id.split('-')[0]} (${dayjs(latestRun.created_at).fromNow()})` : ''}>
            <div>
              <StatCard
                icon={<ThunderboltOutlined />}
                label="Latest Throughput"
                value={latestThroughput > 0 ? formatNumber(latestThroughput) : '-'}
                suffix={latestThroughput > 0 ? 'ops/sec' : undefined}
                color="#52c41a"
                trend={latestThroughput > 0 ? throughputTrend : undefined}
                subtext={throughputTrend !== 0 ? 'vs previous run' : latestRun ? latestRun.workload?.name : undefined}
                onClick={latestRun ? () => navigate(`/runs/${latestRun.id}`) : undefined}
              />
            </div>
          </Tooltip>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Tooltip title={latestRun ? `From: ${latestRun.id.split('-')[0]} (${dayjs(latestRun.created_at).fromNow()})` : ''}>
            <div>
              <StatCard
                icon={<ClockCircleOutlined />}
                label="Latest P99 Latency"
                value={latestP99 > 0 ? latestP99.toFixed(2) : '-'}
                suffix={latestP99 > 0 ? 'ms' : undefined}
                color="#fa8c16"
                trend={latestP99 > 0 ? latencyTrend : undefined}
                subtext={latencyTrend !== 0 ? 'vs previous (↑ = improved)' : undefined}
                onClick={latestRun ? () => navigate(`/runs/${latestRun.id}`) : undefined}
              />
            </div>
          </Tooltip>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <StatCard
            icon={<CheckCircleOutlined />}
            label="Success Rate"
            value={totalRuns > 0 ? `${successRate.toFixed(0)}%` : '-'}
            color={successRate >= 90 ? '#52c41a' : successRate >= 70 ? '#fa8c16' : '#ff4d4f'}
            subtext={totalRuns > 0 ? `${completedRuns.length} of ${totalRuns} runs` : 'No runs yet'}
          />
        </Col>
      </Row>

      {/* Charts Row */}
      {chartData.length > 0 ? (
        <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
          <Col xs={24} lg={16}>
            <Card 
              title={<span style={{ color: 'rgba(255,255,255,0.85)' }}>Recent Throughput Comparison</span>}
              style={{ background: '#1f1f1f', border: '1px solid #303030' }}
              extra={
                bestThroughput > 0 && (
                  <Text style={{ color: 'rgba(255,255,255,0.45)', fontSize: 12 }}>
                    Best: {formatNumber(bestThroughput)} ops/sec
                  </Text>
                )
              }
            >
              <ResponsiveContainer width="100%" height={250}>
                <BarChart data={chartData} margin={{ top: 10, right: 10, left: 10, bottom: 20 }}>
                  <XAxis 
                    dataKey="name" 
                    tick={{ fill: 'rgba(255,255,255,0.45)', fontSize: 10 }}
                    angle={-45}
                    textAnchor="end"
                    height={60}
                  />
                  <YAxis 
                    tick={{ fill: 'rgba(255,255,255,0.45)', fontSize: 10 }}
                    tickFormatter={(v) => formatNumber(v)}
                  />
                  <RechartsTooltip
                    contentStyle={{ 
                      background: '#2a2a2a', 
                      border: '1px solid #404040',
                      borderRadius: 4,
                    }}
                    labelStyle={{ color: 'rgba(255,255,255,0.85)' }}
                    formatter={(value: number) => [
                      `${formatNumber(value)} ops/sec`,
                      'Throughput'
                    ]}
                  />
                  <Bar dataKey="throughput" radius={[4, 4, 0, 0]}>
                    {chartData.map((entry, index) => (
                      <Cell 
                        key={`cell-${index}`} 
                        fill={entry.isLatest ? '#DC382D' : '#52c41a'} 
                        fillOpacity={entry.isLatest ? 1 : 0.7}
                      />
                    ))}
                  </Bar>
                </BarChart>
              </ResponsiveContainer>
              <div style={{ textAlign: 'center', marginTop: 8 }}>
                <Text style={{ color: 'rgba(255,255,255,0.45)', fontSize: 11 }}>
                  <span style={{ display: 'inline-block', width: 12, height: 12, background: '#DC382D', marginRight: 4, borderRadius: 2 }} />
                  Latest Run
                  <span style={{ display: 'inline-block', width: 12, height: 12, background: '#52c41a', marginLeft: 16, marginRight: 4, borderRadius: 2, opacity: 0.7 }} />
                  Previous Runs
                </Text>
              </div>
            </Card>
          </Col>
          <Col xs={24} lg={8}>
            <Card 
              title={<span style={{ color: 'rgba(255,255,255,0.85)' }}>Latest Run Summary</span>}
              style={{ background: '#1f1f1f', border: '1px solid #303030', height: '100%' }}
              extra={
                latestRun && (
                  <a onClick={() => navigate(`/runs/${latestRun.id}`)}>Details →</a>
                )
              }
            >
              {latestRun ? (
                <div>
                  <div style={{ marginBottom: 16 }}>
                    <Text style={{ color: 'rgba(255,255,255,0.45)', fontSize: 12 }}>Workload</Text>
                    <div><Tag color="blue">{latestRun.workload?.name || 'N/A'}</Tag></div>
                  </div>
                  <div style={{ marginBottom: 16 }}>
                    <Text style={{ color: 'rgba(255,255,255,0.45)', fontSize: 12 }}>Target</Text>
                    <div>
                      <Text style={{ color: 'rgba(255,255,255,0.85)' }}>
                        {latestRun.target?.host || 'N/A'}:{latestRun.target?.port || ''}
                      </Text>
                    </div>
                  </div>
                  <div style={{ marginBottom: 16 }}>
                    <Text style={{ color: 'rgba(255,255,255,0.45)', fontSize: 12 }}>Throughput</Text>
                    <Progress 
                      percent={bestThroughput > 0 ? (latestThroughput / bestThroughput) * 100 : 0}
                      strokeColor="#52c41a"
                      trailColor="#303030"
                      format={() => `${formatNumber(latestThroughput)} ops/sec`}
                    />
                  </div>
                  <div style={{ marginBottom: 16 }}>
                    <Text style={{ color: 'rgba(255,255,255,0.45)', fontSize: 12 }}>Latency Percentiles</Text>
                    <div style={{ display: 'flex', justifyContent: 'space-between', marginTop: 8 }}>
                      <div style={{ textAlign: 'center' }}>
                        <Text style={{ color: '#52c41a', fontSize: 16, fontWeight: 500 }}>
                          {latestRun.results?.summary?.p50_latency_ms?.toFixed(2) || '-'}
                        </Text>
                        <div><Text style={{ color: 'rgba(255,255,255,0.45)', fontSize: 11 }}>P50 ms</Text></div>
                      </div>
                      <div style={{ textAlign: 'center' }}>
                        <Text style={{ color: '#fa8c16', fontSize: 16, fontWeight: 500 }}>
                          {latestRun.results?.summary?.p99_latency_ms?.toFixed(2) || '-'}
                        </Text>
                        <div><Text style={{ color: 'rgba(255,255,255,0.45)', fontSize: 11 }}>P99 ms</Text></div>
                      </div>
                      <div style={{ textAlign: 'center' }}>
                        <Text style={{ color: '#ff4d4f', fontSize: 16, fontWeight: 500 }}>
                          {latestRun.results?.summary?.p999_latency_ms?.toFixed(2) || '-'}
                        </Text>
                        <div><Text style={{ color: 'rgba(255,255,255,0.45)', fontSize: 11 }}>P99.9 ms</Text></div>
                      </div>
                    </div>
                  </div>
                  <div>
                    <Text style={{ color: 'rgba(255,255,255,0.45)', fontSize: 12 }}>
                      Ran {dayjs(latestRun.created_at).fromNow()}
                    </Text>
                  </div>
                </div>
              ) : (
                <Empty 
                  description={
                    <Text style={{ color: 'rgba(255,255,255,0.45)' }}>
                      No completed runs yet
                    </Text>
                  }
                />
              )}
            </Card>
          </Col>
        </Row>
      ) : (
        <Card style={{ marginBottom: 24, background: '#1f1f1f', border: '1px solid #303030' }}>
          <Empty
            description={
              <Text style={{ color: 'rgba(255,255,255,0.45)' }}>
                No benchmark data available. Run a benchmark to see performance charts.
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
