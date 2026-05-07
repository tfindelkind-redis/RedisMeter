import {
  PieChart,
  Pie,
  Cell,
  ResponsiveContainer,
  Tooltip,
  Legend,
} from 'recharts';
import { Empty } from 'antd';
import { OperationMetrics } from '@/types';

interface OperationsPieChartProps {
  data: Record<string, OperationMetrics>;
  height?: number;
}

const COLORS = ['#DC382D', '#1890ff', '#52c41a', '#faad14', '#722ed1', '#13c2c2', '#eb2f96'];

export default function OperationsPieChart({ data, height = 300 }: OperationsPieChartProps) {
  const chartData = Object.entries(data).map(([name, metrics]) => ({
    name,
    // Cloud aggregation can provide ops/sec without request count.
    // Fall back to ops/sec to avoid rendering an empty chart.
    value: metrics.count > 0 ? metrics.count : metrics.ops_per_second,
    unit: metrics.count > 0 ? 'requests' : 'ops/sec',
    opsPerSec: metrics.ops_per_second,
  }));

  const total = chartData.reduce((sum, item) => sum + item.value, 0);

  if (chartData.length === 0 || total <= 0) {
    return (
      <div className="chart-container">
        <h4 style={{ margin: '0 0 16px 0', color: 'rgba(255,255,255,0.85)' }}>
          Operations Breakdown
        </h4>
        <Empty description="No operation breakdown data available" image={Empty.PRESENTED_IMAGE_SIMPLE} />
      </div>
    );
  }

  return (
    <div className="chart-container">
      <h4 style={{ margin: '0 0 16px 0', color: 'rgba(255,255,255,0.85)' }}>
        Operations Breakdown
      </h4>
      <ResponsiveContainer width="100%" height={height}>
        <PieChart>
          <Pie
            data={chartData}
            cx="50%"
            cy="50%"
            innerRadius={60}
            outerRadius={100}
            paddingAngle={2}
            dataKey="value"
            label={({ name, percent }) => `${name} ${(percent * 100).toFixed(0)}%`}
            labelLine={false}
          >
            {chartData.map((_, index) => (
              <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
            ))}
          </Pie>
          <Tooltip
            contentStyle={{
              background: '#2d2d2d',
              border: '1px solid #404040',
              borderRadius: 4,
            }}
            formatter={(value: number, name: string, item: any) => {
              const unit = item?.payload?.unit || 'requests';
              return [`${value.toLocaleString()} ${unit} (${((value / total) * 100).toFixed(1)}%)`, name];
            }}
          />
          <Legend />
        </PieChart>
      </ResponsiveContainer>
    </div>
  );
}
