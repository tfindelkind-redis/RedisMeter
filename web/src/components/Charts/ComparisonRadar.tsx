import {
  RadarChart,
  PolarGrid,
  PolarAngleAxis,
  PolarRadiusAxis,
  Radar,
  Legend,
  ResponsiveContainer,
  Tooltip,
} from 'recharts';
import { SummaryMetrics } from '@/types';

interface ComparisonRadarProps {
  baseline?: SummaryMetrics;
  current?: SummaryMetrics;
  baselineName?: string;
  currentName?: string;
  height?: number;
}

export default function ComparisonRadar({
  baseline,
  current,
  baselineName = 'Baseline',
  currentName = 'Current',
  height = 350,
}: ComparisonRadarProps) {
  // Return early if no data
  if (!baseline || !current) {
    return (
      <div style={{ height, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <span style={{ color: 'rgba(255,255,255,0.45)' }}>No data available for comparison</span>
      </div>
    );
  }

  // Normalize metrics for radar chart (0-100 scale)
  const normalizeMetric = (value: number, max: number, invert = false) => {
    const normalized = Math.min((value / max) * 100, 100);
    return invert ? 100 - normalized : normalized;
  };

  const maxThroughput = Math.max(baseline.ops_per_second, current.ops_per_second) * 1.2;
  const maxLatency = Math.max(baseline.p99_latency_ms, current.p99_latency_ms) * 1.2;
  const maxErrors = Math.max(baseline.error_rate, current.error_rate, 1) * 1.2;

  const data = [
    {
      metric: 'Throughput',
      [baselineName]: normalizeMetric(baseline.ops_per_second, maxThroughput),
      [currentName]: normalizeMetric(current.ops_per_second, maxThroughput),
      fullMark: 100,
    },
    {
      metric: 'Avg Latency',
      [baselineName]: normalizeMetric(baseline.avg_latency_ms, maxLatency, true),
      [currentName]: normalizeMetric(current.avg_latency_ms, maxLatency, true),
      fullMark: 100,
    },
    {
      metric: 'P99 Latency',
      [baselineName]: normalizeMetric(baseline.p99_latency_ms, maxLatency, true),
      [currentName]: normalizeMetric(current.p99_latency_ms, maxLatency, true),
      fullMark: 100,
    },
    {
      metric: 'P95 Latency',
      [baselineName]: normalizeMetric(baseline.p95_latency_ms, maxLatency, true),
      [currentName]: normalizeMetric(current.p95_latency_ms, maxLatency, true),
      fullMark: 100,
    },
    {
      metric: 'Error Rate',
      [baselineName]: normalizeMetric(baseline.error_rate, maxErrors, true),
      [currentName]: normalizeMetric(current.error_rate, maxErrors, true),
      fullMark: 100,
    },
  ];

  return (
    <div className="chart-container">
      <h4 style={{ margin: '0 0 16px 0', color: 'rgba(255,255,255,0.85)' }}>
        Performance Comparison
      </h4>
      <ResponsiveContainer width="100%" height={height}>
        <RadarChart data={data}>
          <PolarGrid stroke="#404040" />
          <PolarAngleAxis dataKey="metric" tick={{ fill: '#999', fontSize: 12 }} />
          <PolarRadiusAxis angle={30} domain={[0, 100]} tick={{ fill: '#666' }} />
          <Radar
            name={baselineName}
            dataKey={baselineName}
            stroke="#1890ff"
            fill="#1890ff"
            fillOpacity={0.3}
          />
          <Radar
            name={currentName}
            dataKey={currentName}
            stroke="#DC382D"
            fill="#DC382D"
            fillOpacity={0.3}
          />
          <Tooltip
            contentStyle={{
              background: '#2d2d2d',
              border: '1px solid #404040',
              borderRadius: 4,
            }}
          />
          <Legend />
        </RadarChart>
      </ResponsiveContainer>
    </div>
  );
}
