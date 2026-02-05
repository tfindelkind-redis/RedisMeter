import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Cell,
} from 'recharts';

interface LatencyDistributionProps {
  data: {
    p50: number;
    p90: number;
    p95: number;
    p99: number;
    p999?: number;
    avg: number;
    min: number;
    max: number;
  };
  height?: number;
}

export default function LatencyDistribution({ data, height = 300 }: LatencyDistributionProps) {
  const chartData = [
    { name: 'Min', value: data.min, color: '#52c41a' },
    { name: 'Avg', value: data.avg, color: '#1890ff' },
    { name: 'P50', value: data.p50, color: '#13c2c2' },
    { name: 'P90', value: data.p90, color: '#faad14' },
    { name: 'P95', value: data.p95, color: '#fa8c16' },
    { name: 'P99', value: data.p99, color: '#f5222d' },
    ...(data.p999 ? [{ name: 'P99.9', value: data.p999, color: '#722ed1' }] : []),
    { name: 'Max', value: data.max, color: '#eb2f96' },
  ];

  return (
    <div className="chart-container">
      <h4 style={{ margin: '0 0 16px 0', color: 'rgba(255,255,255,0.85)' }}>
        Latency Distribution
      </h4>
      <ResponsiveContainer width="100%" height={height}>
        <BarChart data={chartData} layout="vertical">
          <CartesianGrid strokeDasharray="3 3" stroke="#303030" horizontal={false} />
          <XAxis
            type="number"
            stroke="#666"
            tick={{ fill: '#999' }}
            tickFormatter={(value) => `${value.toFixed(2)}ms`}
          />
          <YAxis
            type="category"
            dataKey="name"
            stroke="#666"
            tick={{ fill: '#999' }}
            width={50}
          />
          <Tooltip
            contentStyle={{
              background: '#2d2d2d',
              border: '1px solid #404040',
              borderRadius: 4,
            }}
            formatter={(value: number) => [`${value.toFixed(3)}ms`, 'Latency']}
          />
          <Bar dataKey="value" radius={[0, 4, 4, 0]}>
            {chartData.map((entry, index) => (
              <Cell key={`cell-${index}`} fill={entry.color} />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
