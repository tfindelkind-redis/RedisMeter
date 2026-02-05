import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
} from 'recharts';
import { TimeSeriesPoint } from '@/types';

interface ThroughputChartProps {
  data: TimeSeriesPoint[];
  height?: number;
}

export default function ThroughputChart({ data, height = 300 }: ThroughputChartProps) {
  const chartData = data.map((point, index) => ({
    time: index,
    ops: Math.round(point.ops_per_second),
  }));

  return (
    <div className="chart-container">
      <h4 style={{ margin: '0 0 16px 0', color: 'rgba(255,255,255,0.85)' }}>
        Throughput Over Time
      </h4>
      <ResponsiveContainer width="100%" height={height}>
        <AreaChart data={chartData}>
          <defs>
            <linearGradient id="throughputGradient" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="#DC382D" stopOpacity={0.3} />
              <stop offset="95%" stopColor="#DC382D" stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid strokeDasharray="3 3" stroke="#303030" />
          <XAxis
            dataKey="time"
            stroke="#666"
            tick={{ fill: '#999' }}
            tickFormatter={(value) => `${value}s`}
          />
          <YAxis
            stroke="#666"
            tick={{ fill: '#999' }}
            tickFormatter={(value) => `${(value / 1000).toFixed(0)}K`}
          />
          <Tooltip
            contentStyle={{
              background: '#2d2d2d',
              border: '1px solid #404040',
              borderRadius: 4,
            }}
            formatter={(value: number) => [`${value.toLocaleString()} ops/sec`, 'Throughput']}
            labelFormatter={(label) => `Time: ${label}s`}
          />
          <Legend />
          <Area
            type="monotone"
            dataKey="ops"
            name="Ops/sec"
            stroke="#DC382D"
            strokeWidth={2}
            fillOpacity={1}
            fill="url(#throughputGradient)"
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
