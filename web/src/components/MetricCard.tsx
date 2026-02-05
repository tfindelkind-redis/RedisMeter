import { Card, Statistic, Tooltip } from 'antd';
import { ArrowUpOutlined, ArrowDownOutlined } from '@ant-design/icons';
import { Sparkline } from '@/components/Charts';

interface MetricCardProps {
  title: string;
  value: number | string;
  suffix?: string;
  prefix?: React.ReactNode;
  precision?: number;
  trend?: 'up' | 'down' | number; // direction or percentage change
  trendValue?: number; // absolute percentage value
  sparklineData?: number[];
  tooltip?: string;
  valueStyle?: React.CSSProperties;
}

export default function MetricCard({
  title,
  value,
  suffix,
  prefix,
  precision = 2,
  trend,
  trendValue,
  sparklineData,
  tooltip,
  valueStyle,
}: MetricCardProps) {
  const getTrendColor = (t: 'up' | 'down' | number) => {
    if (t === 'up' || (typeof t === 'number' && t > 0)) return '#52c41a';
    if (t === 'down' || (typeof t === 'number' && t < 0)) return '#ff4d4f';
    return '#999';
  };

  const getTrendIcon = (t: 'up' | 'down' | number) => {
    if (t === 'up' || (typeof t === 'number' && t > 0)) return <ArrowUpOutlined />;
    if (t === 'down' || (typeof t === 'number' && t < 0)) return <ArrowDownOutlined />;
    return null;
  };

  const displayValue = typeof trend === 'number' ? Math.abs(trend) : trendValue;

  const content = (
    <Card
      style={{ background: '#1f1f1f', border: '1px solid #303030' }}
      styles={{ body: { padding: 20 } }}
    >
      <Statistic
        title={<span style={{ color: 'rgba(255,255,255,0.65)' }}>{title}</span>}
        value={typeof value === 'number' ? value : undefined}
        valueRender={typeof value === 'string' ? () => <span>{value}</span> : undefined}
        precision={typeof value === 'number' ? precision : undefined}
        suffix={suffix}
        prefix={prefix}
        valueStyle={{ color: 'rgba(255,255,255,0.85)', ...valueStyle }}
      />
      {trend !== undefined && (
        <div style={{ marginTop: 8 }}>
          <span style={{ color: getTrendColor(trend) }}>
            {getTrendIcon(trend)}
            {' '}
            {displayValue !== undefined ? `${displayValue.toFixed(1)}%` : ''}
          </span>
          <span style={{ color: 'rgba(255,255,255,0.45)', marginLeft: 8 }}>
            vs baseline
          </span>
        </div>
      )}
      {sparklineData && sparklineData.length > 0 && (
        <div className="sparkline-container" style={{ marginTop: 8 }}>
          <Sparkline data={sparklineData} />
        </div>
      )}
    </Card>
  );

  if (tooltip) {
    return <Tooltip title={tooltip}>{content}</Tooltip>;
  }

  return content;
}
