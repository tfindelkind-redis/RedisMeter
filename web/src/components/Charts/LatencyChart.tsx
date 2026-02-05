import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
} from 'recharts';
import { TimeSeriesPoint } from '@/types';

interface LatencyChartProps {
  data: TimeSeriesPoint[];
  height?: number;
}

export default function LatencyChart({ data, height = 300 }: LatencyChartProps) {
  const chartData = data.map((point, index) => ({
    time: index,
    avg: point.avg_latency_ms,
    p99: point.p99_latency_ms,
  }));

  return (
    <div className="chart-container">
      <h4 style={{ margin: '0 0 16px 0', color: 'rgba(255,255,255,0.85)' }}>
        Latency Over Time
      </h4>
      <ResponsiveContainer width="100%" height={height}>
        <LineChart data={chartData}>
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
            tickFormatter={(value) => `${value.toFixed(2)}ms`}
          />
          <Tooltip
            contentStyle={{
              background: '#2d2d2d',
              border: '1px solid #404040',
              borderRadius: 4,
            }}
            formatter={(value: number) => [`${value.toFixed(3)}ms`]}
            labelFormatter={(label) => `Time: ${label}s`}
          />
          <Legend />
          <Line
            type="monotone"
            dataKey="avg"
            name="Avg Latency"
            stroke="#52c41a"
            strokeWidth={2}
            dot={false}
          />
          <Line
            type="monotone"
            dataKey="p99"
            name="P99 Latency"
            stroke="#faad14"
            strokeWidth={2}
            dot={false}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
