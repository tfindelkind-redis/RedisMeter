import { useEffect, useState, useMemo } from 'react';
import {
  Card,
  Row,
  Col,
  Typography,
  DatePicker,
  Select,
  Space,
  Spin,
  message,
  Empty,
} from 'antd';
import {
  DashboardOutlined,
  FieldTimeOutlined,
  ApartmentOutlined,
  LineChartOutlined,
} from '@ant-design/icons';
import api from '@/api/client';
import { BenchmarkRun } from '@/types';
import MetricCard from '@/components/MetricCard';
import {
  Area,
  AreaChart,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
  Line,
  LineChart,
  BarChart,
  Bar,
  PieChart,
  Pie,
  Cell,
} from 'recharts';
import dayjs from 'dayjs';

const { Title, Text } = Typography;
const { RangePicker } = DatePicker;

const COLORS = ['#DC382D', '#1890ff', '#52c41a', '#faad14', '#722ed1', '#13c2c2'];


export default function Analytics() {
  const [runs, setRuns] = useState<BenchmarkRun[]>([]);
  const [loading, setLoading] = useState(true);
  const [dateRange, setDateRange] = useState<[dayjs.Dayjs, dayjs.Dayjs] | null>(null);
  const [selectedWorkload, setSelectedWorkload] = useState<string>('all');

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    setLoading(true);
    try {
      const data = await api.getRuns();
      setRuns(data.filter(r => r.status === 'completed'));
    } catch (error) {
      message.error('Failed to load analytics data');
    } finally {
      setLoading(false);
    }
  };

  // Filtered runs based on date range and workload
  const filteredRuns = useMemo(() => {
    let filtered = runs;
    
    if (dateRange) {
      filtered = filtered.filter(r => {
        const date = dayjs(r.created_at);
        return date.isAfter(dateRange[0]) && date.isBefore(dateRange[1]);
      });
    }
    
    if (selectedWorkload !== 'all') {
      filtered = filtered.filter(r => r.workload?.name === selectedWorkload);
    }
    
    return filtered.sort((a, b) => 
      new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
    );
  }, [runs, dateRange, selectedWorkload]);

  // Get unique workloads
  const workloads = useMemo(() => {
    const unique = new Set(runs.map(r => r.workload?.name).filter(Boolean));
    return Array.from(unique);
  }, [runs]);

  // Time series data for throughput trends
  const throughputTrend = useMemo(() => {
    return filteredRuns.map(r => ({
      date: dayjs(r.created_at).format('MM/DD HH:mm'),
      throughput: r.results?.summary?.ops_per_second || 0,
      name: r.name || r.id.slice(0, 8),
    }));
  }, [filteredRuns]);

  // Time series data for latency trends
  const latencyTrend = useMemo(() => {
    return filteredRuns.map(r => ({
      date: dayjs(r.created_at).format('MM/DD HH:mm'),
      avg: r.results?.summary?.avg_latency_ms || 0,
      p99: r.results?.summary?.p99_latency_ms || 0,
      name: r.name || r.id.slice(0, 8),
    }));
  }, [filteredRuns]);

  // Workload distribution
  const workloadDistribution = useMemo(() => {
    const counts: Record<string, number> = {};
    filteredRuns.forEach(r => {
      const name = r.workload?.name || 'Unknown';
      counts[name] = (counts[name] || 0) + 1;
    });
    return Object.entries(counts).map(([name, value]) => ({ name, value }));
  }, [filteredRuns]);

  // Environment distribution (hosts)
  const envDistribution = useMemo(() => {
    const counts: Record<string, { count: number; avgThroughput: number }> = {};
    filteredRuns.forEach(r => {
      const hostname = r.environment?.host?.hostname || 'Unknown';
      if (!counts[hostname]) {
        counts[hostname] = { count: 0, avgThroughput: 0 };
      }
      counts[hostname].count++;
      counts[hostname].avgThroughput += r.results?.summary?.ops_per_second || 0;
    });
    return Object.entries(counts).map(([name, data]) => ({
      name,
      count: data.count,
      avgThroughput: Math.round(data.avgThroughput / data.count),
    }));
  }, [filteredRuns]);

  // Summary statistics
  const stats = useMemo(() => {
    if (filteredRuns.length === 0) return null;
    
    const throughputs = filteredRuns.map(r => r.results?.summary?.ops_per_second || 0);
    const latencies = filteredRuns.map(r => r.results?.summary?.avg_latency_ms || 0);
    
    const avgThroughput = throughputs.reduce((a, b) => a + b, 0) / throughputs.length;
    const avgLatency = latencies.reduce((a, b) => a + b, 0) / latencies.length;
    const maxThroughput = Math.max(...throughputs);
    const minLatency = Math.min(...latencies);
    
    // Calculate trends (compare last 5 vs previous 5)
    const recent = throughputs.slice(-5);
    const previous = throughputs.slice(-10, -5);
    let throughputTrend = 0;
    if (previous.length > 0 && recent.length > 0) {
      const recentAvg = recent.reduce((a, b) => a + b, 0) / recent.length;
      const prevAvg = previous.reduce((a, b) => a + b, 0) / previous.length;
      throughputTrend = ((recentAvg - prevAvg) / prevAvg) * 100;
    }

    return {
      avgThroughput,
      avgLatency,
      maxThroughput,
      minLatency,
      throughputTrend,
      totalRuns: filteredRuns.length,
    };
  }, [filteredRuns]);

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: 100 }}>
        <Spin size="large" />
      </div>
    );
  }

  return (
    <div>
      <Row justify="space-between" align="middle" style={{ marginBottom: 24 }}>
        <Col>
          <Title level={3} style={{ color: 'rgba(255,255,255,0.85)', margin: 0 }}>
            <DashboardOutlined style={{ marginRight: 8, color: '#DC382D' }} />
            Analytics
          </Title>
          <Text type="secondary">
            Performance trends and insights across {filteredRuns.length} runs
          </Text>
        </Col>
        <Col>
          <Space>
            <Select
              style={{ width: 200 }}
              value={selectedWorkload}
              onChange={setSelectedWorkload}
              options={[
                { value: 'all', label: 'All Workloads' },
                ...workloads.map(w => ({ value: w, label: w })),
              ]}
            />
            <RangePicker
              onChange={(dates) => setDateRange(dates as [dayjs.Dayjs, dayjs.Dayjs] | null)}
              style={{ width: 280 }}
            />
          </Space>
        </Col>
      </Row>

      {filteredRuns.length === 0 ? (
        <Empty description="No completed runs found for the selected filters" />
      ) : (
        <>
          {/* Summary Stats */}
          {stats && (
            <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
              <Col xs={24} sm={12} lg={6}>
                <MetricCard
                  title="Avg Throughput"
                  value={stats.avgThroughput}
                  suffix="ops/sec"
                  precision={0}
                  trend={stats.throughputTrend > 0 ? 'up' : stats.throughputTrend < 0 ? 'down' : undefined}
                  trendValue={Math.abs(stats.throughputTrend)}
                  valueStyle={{ color: '#52c41a' }}
                />
              </Col>
              <Col xs={24} sm={12} lg={6}>
                <MetricCard
                  title="Avg Latency"
                  value={stats.avgLatency}
                  suffix="ms"
                  precision={3}
                />
              </Col>
              <Col xs={24} sm={12} lg={6}>
                <MetricCard
                  title="Peak Throughput"
                  value={stats.maxThroughput}
                  suffix="ops/sec"
                  precision={0}
                  valueStyle={{ color: '#DC382D' }}
                />
              </Col>
              <Col xs={24} sm={12} lg={6}>
                <MetricCard
                  title="Best Latency"
                  value={stats.minLatency}
                  suffix="ms"
                  precision={3}
                  valueStyle={{ color: '#1890ff' }}
                />
              </Col>
            </Row>
          )}

          {/* Throughput Trend */}
          <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
            <Col xs={24}>
              <Card 
                title={
                  <Space>
                    <LineChartOutlined style={{ color: '#52c41a' }} />
                    <span>Throughput Over Time</span>
                  </Space>
                }
                style={{ background: '#1f1f1f', border: '1px solid #303030' }}
              >
                <ResponsiveContainer width="100%" height={300}>
                  <AreaChart data={throughputTrend}>
                    <defs>
                      <linearGradient id="throughputGradient" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="5%" stopColor="#52c41a" stopOpacity={0.8} />
                        <stop offset="95%" stopColor="#52c41a" stopOpacity={0} />
                      </linearGradient>
                    </defs>
                    <CartesianGrid strokeDasharray="3 3" stroke="#303030" />
                    <XAxis dataKey="date" stroke="#666" tick={{ fill: '#999' }} />
                    <YAxis stroke="#666" tick={{ fill: '#999' }} />
                    <Tooltip
                      contentStyle={{ 
                        backgroundColor: '#1f1f1f', 
                        border: '1px solid #303030',
                        borderRadius: 4,
                      }}
                      labelStyle={{ color: '#fff' }}
                    />
                    <Area
                      type="monotone"
                      dataKey="throughput"
                      stroke="#52c41a"
                      fill="url(#throughputGradient)"
                      name="Throughput (ops/sec)"
                    />
                  </AreaChart>
                </ResponsiveContainer>
              </Card>
            </Col>
          </Row>

          {/* Latency Trend */}
          <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
            <Col xs={24}>
              <Card 
                title={
                  <Space>
                    <FieldTimeOutlined style={{ color: '#1890ff' }} />
                    <span>Latency Trend</span>
                  </Space>
                }
                style={{ background: '#1f1f1f', border: '1px solid #303030' }}
              >
                <ResponsiveContainer width="100%" height={300}>
                  <LineChart data={latencyTrend}>
                    <CartesianGrid strokeDasharray="3 3" stroke="#303030" />
                    <XAxis dataKey="date" stroke="#666" tick={{ fill: '#999' }} />
                    <YAxis stroke="#666" tick={{ fill: '#999' }} />
                    <Tooltip
                      contentStyle={{ 
                        backgroundColor: '#1f1f1f', 
                        border: '1px solid #303030',
                        borderRadius: 4,
                      }}
                      labelStyle={{ color: '#fff' }}
                    />
                    <Legend />
                    <Line
                      type="monotone"
                      dataKey="avg"
                      stroke="#1890ff"
                      strokeWidth={2}
                      dot={{ fill: '#1890ff', r: 3 }}
                      name="Avg Latency (ms)"
                    />
                    <Line
                      type="monotone"
                      dataKey="p99"
                      stroke="#faad14"
                      strokeWidth={2}
                      dot={{ fill: '#faad14', r: 3 }}
                      name="P99 Latency (ms)"
                    />
                  </LineChart>
                </ResponsiveContainer>
              </Card>
            </Col>
          </Row>

          {/* Distribution Charts */}
          <Row gutter={[16, 16]}>
            <Col xs={24} lg={12}>
              <Card 
                title={
                  <Space>
                    <ApartmentOutlined style={{ color: '#722ed1' }} />
                    <span>Workload Distribution</span>
                  </Space>
                }
                style={{ background: '#1f1f1f', border: '1px solid #303030' }}
              >
                <ResponsiveContainer width="100%" height={300}>
                  <PieChart>
                    <Pie
                      data={workloadDistribution}
                      cx="50%"
                      cy="50%"
                      innerRadius={60}
                      outerRadius={100}
                      paddingAngle={5}
                      dataKey="value"
                      label={({ name, percent }) => `${name} (${(percent * 100).toFixed(0)}%)`}
                    >
                      {workloadDistribution.map((_, index) => (
                        <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                      ))}
                    </Pie>
                    <Tooltip
                      contentStyle={{ 
                        backgroundColor: '#1f1f1f', 
                        border: '1px solid #303030',
                        borderRadius: 4,
                      }}
                    />
                  </PieChart>
                </ResponsiveContainer>
              </Card>
            </Col>
            <Col xs={24} lg={12}>
              <Card 
                title="Performance by Environment"
                style={{ background: '#1f1f1f', border: '1px solid #303030' }}
              >
                <ResponsiveContainer width="100%" height={300}>
                  <BarChart data={envDistribution} layout="vertical">
                    <CartesianGrid strokeDasharray="3 3" stroke="#303030" />
                    <XAxis type="number" stroke="#666" tick={{ fill: '#999' }} />
                    <YAxis dataKey="name" type="category" stroke="#666" tick={{ fill: '#999' }} width={100} />
                    <Tooltip
                      contentStyle={{ 
                        backgroundColor: '#1f1f1f', 
                        border: '1px solid #303030',
                        borderRadius: 4,
                      }}
                      formatter={(value: number, name: string) => [
                        name === 'avgThroughput' ? `${value.toLocaleString()} ops/sec` : value,
                        name === 'avgThroughput' ? 'Avg Throughput' : 'Runs'
                      ]}
                    />
                    <Bar dataKey="avgThroughput" fill="#DC382D" name="Avg Throughput" />
                  </BarChart>
                </ResponsiveContainer>
              </Card>
            </Col>
          </Row>
        </>
      )}
    </div>
  );
}
