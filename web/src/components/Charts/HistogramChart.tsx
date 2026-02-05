import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  ReferenceLine,
} from 'recharts';
import { Histogram as HistogramData } from '@/types';

interface HistogramChartProps {
  data: HistogramData;
  height?: number;
}

export default function HistogramChart({ data, height = 300 }: HistogramChartProps) {
  const chartData = data.buckets.map((bucket) => ({
    range: `≤${bucket.upper_bound_ms.toFixed(2)}ms`,
    count: bucket.count,
    cumulative: bucket.cumulative,
    upperBound: bucket.upper_bound_ms,
  }));

  // Calculate total for percentages
  const total = chartData.length > 0 ? chartData[chartData.length - 1].cumulative : 0;

  // Find P99 bucket
  const p99Threshold = total * 0.99;
  const p99Bucket = chartData.find((b) => b.cumulative >= p99Threshold);

  return (
    <div className="chart-container">
      <h4 style={{ margin: '0 0 16px 0', color: 'rgba(255,255,255,0.85)' }}>
        Latency Histogram
      </h4>
      <ResponsiveContainer width="100%" height={height}>
        <BarChart data={chartData}>
          <CartesianGrid strokeDasharray="3 3" stroke="#303030" />
          <XAxis
            dataKey="range"
            stroke="#666"
            tick={{ fill: '#999', fontSize: 10 }}
            angle={-45}
            textAnchor="end"
            height={60}
          />
          <YAxis
            stroke="#666"
            tick={{ fill: '#999' }}
            tickFormatter={(value) => value.toLocaleString()}
          />
          <Tooltip
            contentStyle={{
              background: '#2d2d2d',
              border: '1px solid #404040',
              borderRadius: 4,
            }}
            formatter={(value: number, name: string) => {
              if (name === 'count') {
                return [value.toLocaleString(), 'Requests'];
              }
              return [((value / total) * 100).toFixed(2) + '%', 'Cumulative'];
            }}
          />
          {p99Bucket && (
            <ReferenceLine
              x={p99Bucket.range}
              stroke="#f5222d"
              strokeDasharray="3 3"
              label={{ value: 'P99', fill: '#f5222d', position: 'top' }}
            />
          )}
          <Bar dataKey="count" fill="#1890ff" radius={[4, 4, 0, 0]} />
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
