import { useEffect, useState } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import {
  Card,
  Row,
  Col,
  Typography,
  Select,
  Button,
  Space,
  Spin,
  Empty,
  message,
  Descriptions,
  Tag,
  Statistic,
  Progress,
} from 'antd';
import {
  ArrowLeftOutlined,
  SwapOutlined,
  ArrowUpOutlined,
  ArrowDownOutlined,
  CheckCircleFilled,
  WarningFilled,
  CloseCircleFilled,
} from '@ant-design/icons';
import api from '@/api/client';
import { BenchmarkRun, Baseline, ComparisonResult } from '@/types';
import { ComparisonRadar } from '@/components/Charts';

const { Title, Text } = Typography;

export default function Compare() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  
  const [runs, setRuns] = useState<BenchmarkRun[]>([]);
  const [baselines, setBaselines] = useState<Baseline[]>([]);
  const [selectedRunId, setSelectedRunId] = useState<string>(searchParams.get('run') || '');
  const [selectedBaselineId, setSelectedBaselineId] = useState<string>(searchParams.get('baseline') || '');
  const [comparison, setComparison] = useState<ComparisonResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [comparing, setComparing] = useState(false);

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    setLoading(true);
    try {
      const [runsData, baselinesData] = await Promise.all([
        api.getRuns(),
        api.getBaselines(),
      ]);
      setRuns(runsData.filter(r => r.status === 'completed'));
      setBaselines(baselinesData);
      
      // Auto-select from URL params
      if (searchParams.get('baseline')) {
        setSelectedBaselineId(searchParams.get('baseline')!);
      }
    } catch (error) {
      message.error('Failed to load data');
    } finally {
      setLoading(false);
    }
  };

  const runComparison = async () => {
    if (!selectedRunId || !selectedBaselineId) {
      message.warning('Please select both a run and a baseline');
      return;
    }

    const selectedBaselineForCompare = baselines.find(b => b.id === selectedBaselineId);
    if (!selectedBaselineForCompare?.run_id) {
      message.error('Selected baseline is missing an associated run');
      return;
    }
    
    setComparing(true);
    try {
      const result = await api.compare(selectedRunId, selectedBaselineForCompare.run_id);
      setComparison(result);
    } catch (error: any) {
      const apiError = error?.response?.data?.error || error?.message || 'Unknown error';
      message.error(`Failed to compare: ${apiError}`);
    } finally {
      setComparing(false);
    }
  };

  const getChangeColor = (pct: number, inverse: boolean = false) => {
    if (inverse) pct = -pct;
    if (pct > 5) return '#52c41a';
    if (pct < -5) return '#ff4d4f';
    return '#faad14';
  };

  const getChangeIcon = (pct: number, inverse: boolean = false) => {
    if (inverse) pct = -pct;
    if (pct > 0) return <ArrowUpOutlined />;
    if (pct < 0) return <ArrowDownOutlined />;
    return null;
  };

  const renderThresholdStatus = (status: 'pass' | 'warn' | 'fail') => {
    switch (status) {
      case 'pass':
        return <Tag icon={<CheckCircleFilled />} color="success">Pass</Tag>;
      case 'warn':
        return <Tag icon={<WarningFilled />} color="warning">Warning</Tag>;
      case 'fail':
        return <Tag icon={<CloseCircleFilled />} color="error">Failed</Tag>;
    }
  };

  const formatDetailValue = (value: unknown): string => {
    if (value === null || value === undefined) return '-';
    if (typeof value === 'boolean') return value ? 'true' : 'false';
    if (typeof value === 'number') return Number.isFinite(value) ? String(value) : '-';
    if (typeof value === 'string') return value || '-';
    if (Array.isArray(value)) return value.length ? value.join(', ') : '-';
    try {
      return JSON.stringify(value);
    } catch {
      return String(value);
    }
  };

  const categoryLabel: Record<string, string> = {
    workload: 'Workload',
    execution_profile: 'Execution Profile',
    target_infra: 'Target / Infrastructure',
    environment: 'Environment / Hardware',
  };

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: 100 }}>
        <Spin size="large" />
      </div>
    );
  }

  const selectedRun = runs.find(r => r.id === selectedRunId);
  const selectedBaseline = baselines.find(b => b.id === selectedBaselineId);

  return (
    <div>
      {/* Header */}
      <Row justify="space-between" align="middle" style={{ marginBottom: 24 }}>
        <Col>
          <Space>
            <Button icon={<ArrowLeftOutlined />} onClick={() => navigate(-1)} />
            <Title level={3} style={{ color: 'rgba(255,255,255,0.85)', margin: 0 }}>
              Compare Results
            </Title>
          </Space>
        </Col>
      </Row>

      {/* Selection */}
      <Card style={{ background: '#1f1f1f', border: '1px solid #303030', marginBottom: 24 }}>
        <Row gutter={[16, 16]} align="middle">
          <Col xs={24} md={10}>
            <Text strong style={{ display: 'block', marginBottom: 8 }}>Benchmark Run</Text>
            <Select
              style={{ width: '100%' }}
              placeholder="Select a completed run"
              value={selectedRunId || undefined}
              onChange={setSelectedRunId}
              showSearch
              optionFilterProp="label"
              options={runs.map(r => ({
                value: r.id,
                label: `${r.id}${r.name ? ` (${r.name})` : ''} - ${r.workload?.name || 'Unknown'}`,
              }))}
            />
          </Col>
          <Col xs={24} md={4} style={{ textAlign: 'center' }}>
            <SwapOutlined style={{ fontSize: 24, color: '#DC382D' }} />
          </Col>
          <Col xs={24} md={10}>
            <Text strong style={{ display: 'block', marginBottom: 8 }}>Baseline</Text>
            <Select
              style={{ width: '100%' }}
              placeholder="Select a baseline"
              value={selectedBaselineId || undefined}
              onChange={setSelectedBaselineId}
              showSearch
              optionFilterProp="label"
              options={baselines.map(b => ({
                value: b.id,
                label: `${b.run_id || b.id}${b.name ? ` (${b.name})` : ''}${b.active ? ' (Active)' : ''}`,
              }))}
            />
          </Col>
        </Row>
        <Row style={{ marginTop: 16 }} justify="center">
          <Button 
            type="primary" 
            onClick={runComparison}
            loading={comparing}
            disabled={!selectedRunId || !selectedBaselineId}
            size="large"
          >
            Compare
          </Button>
        </Row>
      </Card>

      {/* Comparison Results */}
      {comparison && (
        <>
          {/* Overall Status */}
          <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
            <Col xs={24}>
              <Card 
                style={{ 
                  background: comparison.passed ? '#162312' : '#2a1215',
                  border: `1px solid ${comparison.passed ? '#274916' : '#58181c'}`,
                }}
              >
                <Row align="middle" justify="center">
                  <Col>
                    <Space size="large">
                      {comparison.passed ? (
                        <CheckCircleFilled style={{ fontSize: 48, color: '#52c41a' }} />
                      ) : (
                        <CloseCircleFilled style={{ fontSize: 48, color: '#ff4d4f' }} />
                      )}
                      <div>
                        <Title level={3} style={{ color: 'rgba(255,255,255,0.85)', margin: 0 }}>
                          {comparison.passed ? 'Comparison Passed' : 'Comparison Failed'}
                        </Title>
                        <Text type="secondary">
                          {comparison.violations?.length || 0} threshold violations
                        </Text>
                      </div>
                    </Space>
                  </Col>
                </Row>
              </Card>
            </Col>
          </Row>

          {/* Metric Changes */}
          <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
            <Col xs={24} sm={12} lg={6}>
              <Card style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
                <Statistic
                  title="Throughput Change"
                  value={comparison.changes?.throughput_pct || 0}
                  precision={2}
                  suffix="%"
                  valueStyle={{ color: getChangeColor(comparison.changes?.throughput_pct || 0) }}
                  prefix={getChangeIcon(comparison.changes?.throughput_pct || 0)}
                />
                <Progress 
                  percent={Math.min(Math.abs(comparison.changes?.throughput_pct || 0), 100)} 
                  showInfo={false}
                  strokeColor={getChangeColor(comparison.changes?.throughput_pct || 0)}
                  size="small"
                />
              </Card>
            </Col>
            <Col xs={24} sm={12} lg={6}>
              <Card style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
                <Statistic
                  title="Avg Latency Change"
                  value={comparison.changes?.avg_latency_pct || 0}
                  precision={2}
                  suffix="%"
                  valueStyle={{ color: getChangeColor(comparison.changes?.avg_latency_pct || 0, true) }}
                  prefix={getChangeIcon(comparison.changes?.avg_latency_pct || 0, true)}
                />
                <Progress 
                  percent={Math.min(Math.abs(comparison.changes?.avg_latency_pct || 0), 100)} 
                  showInfo={false}
                  strokeColor={getChangeColor(comparison.changes?.avg_latency_pct || 0, true)}
                  size="small"
                />
              </Card>
            </Col>
            <Col xs={24} sm={12} lg={6}>
              <Card style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
                <Statistic
                  title="P99 Latency Change"
                  value={comparison.changes?.p99_latency_pct || 0}
                  precision={2}
                  suffix="%"
                  valueStyle={{ color: getChangeColor(comparison.changes?.p99_latency_pct || 0, true) }}
                  prefix={getChangeIcon(comparison.changes?.p99_latency_pct || 0, true)}
                />
                <Progress 
                  percent={Math.min(Math.abs(comparison.changes?.p99_latency_pct || 0), 100)} 
                  showInfo={false}
                  strokeColor={getChangeColor(comparison.changes?.p99_latency_pct || 0, true)}
                  size="small"
                />
              </Card>
            </Col>
            <Col xs={24} sm={12} lg={6}>
              <Card style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
                <Statistic
                  title="Error Rate Change"
                  value={comparison.changes?.error_rate_pct || 0}
                  precision={2}
                  suffix="%"
                  valueStyle={{ color: getChangeColor(comparison.changes?.error_rate_pct || 0, true) }}
                  prefix={getChangeIcon(comparison.changes?.error_rate_pct || 0, true)}
                />
                <Progress 
                  percent={Math.min(Math.abs(comparison.changes?.error_rate_pct || 0), 100)} 
                  showInfo={false}
                  strokeColor={getChangeColor(comparison.changes?.error_rate_pct || 0, true)}
                  size="small"
                />
              </Card>
            </Col>
          </Row>

          {/* Visualization */}
          <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
            <Col xs={24} lg={12}>
              <ComparisonRadar
                baseline={selectedBaseline?.metrics}
                current={selectedRun?.results?.summary}
              />
            </Col>
            <Col xs={24} lg={12}>
              <Card title="Side-by-Side Metrics" style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
                <Descriptions column={2} size="small">
                  <Descriptions.Item label="Baseline Throughput">
                    <Text strong>{selectedBaseline?.metrics?.ops_per_second?.toLocaleString()} ops/sec</Text>
                  </Descriptions.Item>
                  <Descriptions.Item label="Current Throughput">
                    <Text strong style={{ color: '#52c41a' }}>
                      {selectedRun?.results?.summary?.ops_per_second?.toLocaleString()} ops/sec
                    </Text>
                  </Descriptions.Item>
                  <Descriptions.Item label="Baseline Avg Latency">
                    <Text strong>{selectedBaseline?.metrics?.avg_latency_ms?.toFixed(3)} ms</Text>
                  </Descriptions.Item>
                  <Descriptions.Item label="Current Avg Latency">
                    <Text strong>{selectedRun?.results?.summary?.avg_latency_ms?.toFixed(3)} ms</Text>
                  </Descriptions.Item>
                  <Descriptions.Item label="Baseline P99">
                    <Text strong>{selectedBaseline?.metrics?.p99_latency_ms?.toFixed(3)} ms</Text>
                  </Descriptions.Item>
                  <Descriptions.Item label="Current P99">
                    <Text strong>{selectedRun?.results?.summary?.p99_latency_ms?.toFixed(3)} ms</Text>
                  </Descriptions.Item>
                </Descriptions>
              </Card>
            </Col>
          </Row>

          {/* Threshold Violations */}
          {comparison.violations && comparison.violations.length > 0 && (
            <Card 
              title="Threshold Violations" 
              style={{ background: '#1f1f1f', border: '1px solid #303030', marginBottom: 24 }}
            >
              {comparison.violations.map((v, i) => (
                <Row key={i} style={{ padding: '8px 0', borderBottom: '1px solid #303030' }}>
                  <Col span={8}>
                    <Text strong>{v.metric}</Text>
                  </Col>
                  <Col span={4}>
                    {v.status ? renderThresholdStatus(v.status) : <Tag color="red">Violation</Tag>}
                  </Col>
                  <Col span={6}>
                    <Text>Expected: {v.threshold}</Text>
                  </Col>
                  <Col span={6}>
                    <Text style={{ color: '#ff4d4f' }}>Actual: {v.actual}</Text>
                  </Col>
                </Row>
              ))}
            </Card>
          )}

          <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
            <Col xs={24}>
              <Card title="Comparison Context" style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
                <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
                  <Col xs={24} sm={12} md={6}>
                    <Text type="secondary">Comparable</Text>
                    <div>
                      <Tag color={comparison.comparable ? 'green' : 'red'}>
                        {comparison.comparable ? 'Yes' : 'No'}
                      </Tag>
                    </div>
                  </Col>
                  <Col xs={24} sm={12} md={6}>
                    <Text type="secondary">Quality</Text>
                    <div>
                      <Tag color={comparison.comparison_quality === 'valid' ? 'green' : comparison.comparison_quality === 'questionable' ? 'orange' : 'red'}>
                        {comparison.comparison_quality || 'unknown'}
                      </Tag>
                    </div>
                  </Col>
                  <Col xs={24} sm={12} md={6}>
                    <Text type="secondary">Blocking Differences</Text>
                    <div><Text strong>{comparison.blocking_differences?.length || 0}</Text></div>
                  </Col>
                  <Col xs={24} sm={12} md={6}>
                    <Text type="secondary">Warnings</Text>
                    <div><Text strong>{comparison.warnings?.length || 0}</Text></div>
                  </Col>
                </Row>

                {comparison.blocking_differences && comparison.blocking_differences.length > 0 && (
                  <Card size="small" title="Blocking Differences" style={{ marginBottom: 12, background: '#141414', border: '1px solid #58181c' }}>
                    {comparison.blocking_differences.map((item, idx) => (
                      <div key={idx}><Text style={{ color: '#ff7875' }}>- {item}</Text></div>
                    ))}
                  </Card>
                )}

                {comparison.warnings && comparison.warnings.length > 0 && (
                  <Card size="small" title="Warnings" style={{ marginBottom: 12, background: '#141414', border: '1px solid #614700' }}>
                    {comparison.warnings.map((item, idx) => (
                      <div key={idx}><Text style={{ color: '#ffd666' }}>- {item}</Text></div>
                    ))}
                  </Card>
                )}

                {comparison.compatibility && (
                  <Row gutter={[12, 12]}>
                    {Object.entries(comparison.compatibility).map(([key, details]) => (
                      <Col xs={24} lg={12} key={key}>
                        <Card
                          size="small"
                          title={categoryLabel[key] || key}
                          style={{ background: '#141414', border: '1px solid #303030' }}
                        >
                          <Descriptions column={1} size="small">
                            {Object.entries(details || {}).map(([detailKey, detailValue]) => (
                              <Descriptions.Item key={detailKey} label={detailKey.replace(/_/g, ' ')}>
                                <Text>{formatDetailValue(detailValue)}</Text>
                              </Descriptions.Item>
                            ))}
                          </Descriptions>
                        </Card>
                      </Col>
                    ))}
                  </Row>
                )}
              </Card>
            </Col>
          </Row>
        </>
      )}

      {!comparison && !comparing && (
        <Empty
          description="Select a run and baseline to compare"
          image={Empty.PRESENTED_IMAGE_SIMPLE}
        />
      )}
    </div>
  );
}
