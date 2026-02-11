import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Card,
  Row,
  Col,
  Typography,
  Descriptions,
  Tag,
  Button,
  Space,
  Tabs,
  Spin,
  Empty,
  message,
  Modal,
  Input,
  Divider,
} from 'antd';
import {
  ArrowLeftOutlined,
  BookOutlined,
  BarChartOutlined,
  DiffOutlined,
} from '@ant-design/icons';
import api from '@/api/client';
import { BenchmarkRun, AnalysisResult } from '@/types';
import StatusBadge from '@/components/StatusBadge';
import MetricCard from '@/components/MetricCard';
import {
  ThroughputChart,
  LatencyChart,
  LatencyDistribution,
  HistogramChart,
  OperationsPieChart,
} from '@/components/Charts';
import dayjs from 'dayjs';

const { Title, Text } = Typography;
const { TabPane } = Tabs;

export default function RunDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [run, setRun] = useState<BenchmarkRun | null>(null);
  const [analysis, setAnalysis] = useState<AnalysisResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [analysisLoading, setAnalysisLoading] = useState(false);
  const [baselineModalOpen, setBaselineModalOpen] = useState(false);
  const [baselineName, setBaselineName] = useState('');

  useEffect(() => {
    if (id) {
      loadRun(id);
    }
  }, [id]);

  const loadRun = async (runId: string) => {
    setLoading(true);
    try {
      const data = await api.getRun(runId);
      setRun(data);
    } catch (error) {
      message.error('Failed to load run');
      navigate('/runs');
    } finally {
      setLoading(false);
    }
  };

  const runAnalysis = async () => {
    if (!id) return;
    setAnalysisLoading(true);
    try {
      const result = await api.analyze(id);
      setAnalysis(result);
    } catch (error) {
      message.error('Failed to run analysis');
    } finally {
      setAnalysisLoading(false);
    }
  };

  const createBaseline = async () => {
    if (!id || !baselineName) return;
    try {
      await api.createBaseline(id, baselineName);
      message.success('Baseline created successfully');
      setBaselineModalOpen(false);
      setBaselineName('');
    } catch (error) {
      message.error('Failed to create baseline');
    }
  };

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: 100 }}>
        <Spin size="large" />
      </div>
    );
  }

  if (!run) {
    return <Empty description="Run not found" />;
  }

  const summary = run.results?.summary;

  return (
    <div>
      {/* Header */}
      <Row justify="space-between" align="middle" style={{ marginBottom: 24 }}>
        <Col>
          <Space>
            <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/runs')} />
            <Title level={3} style={{ color: 'rgba(255,255,255,0.85)', margin: 0 }}>
              {run.name || run.id.slice(0, 8)}
            </Title>
            <StatusBadge status={run.status} />
          </Space>
        </Col>
        <Col>
          <Space>
            <Button
              icon={<BookOutlined />}
              onClick={() => setBaselineModalOpen(true)}
              disabled={run.status !== 'completed'}
            >
              Save as Baseline
            </Button>
            <Button
              icon={<BarChartOutlined />}
              onClick={runAnalysis}
              loading={analysisLoading}
              disabled={run.status !== 'completed'}
            >
              Analyze
            </Button>
            <Button
              icon={<DiffOutlined />}
              onClick={() => navigate(`/compare?run=${run.id}`)}
              disabled={run.status !== 'completed'}
            >
              Compare
            </Button>
          </Space>
        </Col>
      </Row>

      {/* Metrics Summary */}
      {summary && (
        <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
          <Col xs={24} sm={12} lg={6}>
            <MetricCard
              title="Throughput"
              value={summary.ops_per_second}
              suffix="ops/sec"
              precision={0}
              valueStyle={{ color: '#52c41a' }}
            />
          </Col>
          <Col xs={24} sm={12} lg={6}>
            <MetricCard
              title="Avg Latency"
              value={summary.avg_latency_ms}
              suffix="ms"
              precision={3}
            />
          </Col>
          <Col xs={24} sm={12} lg={6}>
            <MetricCard
              title="P99 Latency"
              value={summary.p99_latency_ms}
              suffix="ms"
              precision={3}
              valueStyle={summary.p99_latency_ms > 10 ? { color: '#faad14' } : undefined}
            />
          </Col>
          <Col xs={24} sm={12} lg={6}>
            <MetricCard
              title="Error Rate"
              value={summary.error_rate * 100}
              suffix="%"
              precision={2}
              valueStyle={summary.error_rate > 0.01 ? { color: '#ff4d4f' } : { color: '#52c41a' }}
            />
          </Col>
        </Row>
      )}

      <Tabs defaultActiveKey="charts" type="card">
        <TabPane tab="Charts" key="charts">
          <Row gutter={[16, 16]}>
            {run.results?.time_series && run.results.time_series.length > 0 && (
              <>
                <Col xs={24} lg={12}>
                  <ThroughputChart data={run.results.time_series} />
                </Col>
                <Col xs={24} lg={12}>
                  <LatencyChart data={run.results.time_series} />
                </Col>
              </>
            )}
            {summary && (
              <Col xs={24} lg={12}>
                <LatencyDistribution
                  data={{
                    min: summary.min_latency_ms,
                    avg: summary.avg_latency_ms,
                    p50: summary.p50_latency_ms,
                    p90: summary.p90_latency_ms,
                    p95: summary.p95_latency_ms,
                    p99: summary.p99_latency_ms,
                    p999: summary.p999_latency_ms,
                    max: summary.max_latency_ms,
                  }}
                />
              </Col>
            )}
            {run.results?.latency_histogram && (
              <Col xs={24} lg={12}>
                <HistogramChart data={run.results.latency_histogram} />
              </Col>
            )}
            {run.results?.by_operation && Object.keys(run.results.by_operation).length > 0 && (
              <Col xs={24} lg={12}>
                <OperationsPieChart data={run.results.by_operation} />
              </Col>
            )}
          </Row>
        </TabPane>

        <TabPane tab="Details" key="details">
          <Row gutter={[16, 16]}>
            <Col xs={24} lg={12}>
              <Card title="Run Information" style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
                <Descriptions column={1} size="small">
                  <Descriptions.Item label="ID">
                    <Text code>{run.id}</Text>
                  </Descriptions.Item>
                  <Descriptions.Item label="Created">
                    {dayjs(run.created_at).format('YYYY-MM-DD HH:mm:ss')}
                  </Descriptions.Item>
                  <Descriptions.Item label="Duration">
                    {run.duration || '-'}
                  </Descriptions.Item>
                  <Descriptions.Item label="Tags">
                    <Space wrap>
                      {run.tags?.map((tag) => <Tag key={tag}>{tag}</Tag>)}
                    </Space>
                  </Descriptions.Item>
                  {run.description && (
                    <Descriptions.Item label="Description">
                      {run.description}
                    </Descriptions.Item>
                  )}
                </Descriptions>
              </Card>
            </Col>

            <Col xs={24} lg={12}>
              <Card title="Workload" style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
                <Descriptions column={1} size="small">
                  <Descriptions.Item label="Name">
                    <Tag color="blue">{run.workload?.name}</Tag>
                  </Descriptions.Item>
                  <Descriptions.Item label="Type">
                    {run.workload?.type}
                  </Descriptions.Item>
                  <Descriptions.Item label="Threads">
                    {run.workload?.threads}
                  </Descriptions.Item>
                  <Descriptions.Item label="Clients">
                    {run.workload?.clients}
                  </Descriptions.Item>
                  <Descriptions.Item label="Duration">
                    {run.workload?.duration}
                  </Descriptions.Item>
                  <Descriptions.Item label="Operations">
                    <Space wrap>
                      {run.workload?.operations?.map((op) => (
                        <Tag key={op.command}>
                          {op.command} ({(op.ratio * 100).toFixed(0)}%)
                        </Tag>
                      ))}
                    </Space>
                  </Descriptions.Item>
                </Descriptions>
              </Card>
            </Col>

            <Col xs={24} lg={12}>
              <Card title="Target" style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
                <Descriptions column={1} size="small">
                  <Descriptions.Item label="Host">
                    {run.target?.host}
                  </Descriptions.Item>
                  <Descriptions.Item label="Port">
                    {run.target?.port}
                  </Descriptions.Item>
                  <Descriptions.Item label="TLS">
                    {run.target?.tls ? <Tag color="green">Enabled</Tag> : <Tag>Disabled</Tag>}
                  </Descriptions.Item>
                  <Descriptions.Item label="Cluster">
                    {run.target?.cluster ? <Tag color="purple">Yes</Tag> : <Tag>No</Tag>}
                  </Descriptions.Item>
                </Descriptions>
              </Card>
            </Col>

            <Col xs={24} lg={12}>
              <Card title="Environment" style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
                <Space direction="vertical" style={{ width: '100%' }} size="small">
                  <Text strong>Host</Text>
                  <Descriptions column={1} size="small">
                    <Descriptions.Item label="Hostname">
                      {run.environment?.host?.hostname || '-'}
                    </Descriptions.Item>
                    <Descriptions.Item label="OS">
                      {run.environment?.host?.os || '-'} / {run.environment?.host?.arch || '-'}
                    </Descriptions.Item>
                    <Descriptions.Item label="CPUs">
                      {run.environment?.host?.cpus || '-'}
                    </Descriptions.Item>
                    <Descriptions.Item label="Memory">
                      {run.environment?.host?.memory_gb?.toFixed(1) || '-'} GB
                    </Descriptions.Item>
                    {run.environment?.host?.cpu_model && (
                      <Descriptions.Item label="CPU Model">
                        {run.environment.host.cpu_model}
                      </Descriptions.Item>
                    )}
                  </Descriptions>

                  <Divider style={{ margin: '8px 0' }} />
                  <Text strong>Redis Server</Text>
                  <Descriptions column={1} size="small">
                    <Descriptions.Item label="Version">
                      {run.environment?.redis?.version || '-'}
                    </Descriptions.Item>
                    <Descriptions.Item label="Mode">
                      <Tag color={run.environment?.redis?.cluster_enabled ? 'purple' : 'blue'}>
                        {run.environment?.redis?.mode || 'standalone'}
                      </Tag>
                    </Descriptions.Item>
                    {run.environment?.redis?.memory_used !== undefined && (
                      <Descriptions.Item label="Memory Used">
                        {((run.environment.redis.memory_used || 0) / (1024 * 1024)).toFixed(2)} MB
                        {run.environment.redis.memory_max !== undefined && run.environment.redis.memory_max > 0 &&
                          ` / ${(run.environment.redis.memory_max / (1024 * 1024)).toFixed(0)} MB`}
                      </Descriptions.Item>
                    )}
                    {run.environment?.redis?.connected_clients !== undefined && (
                      <Descriptions.Item label="Clients">
                        {run.environment.redis.connected_clients} connected
                      </Descriptions.Item>
                    )}
                    {run.environment?.redis?.total_keys !== undefined && run.environment.redis.total_keys > 0 && (
                      <Descriptions.Item label="Keys">
                        {run.environment.redis.total_keys.toLocaleString()}
                        {run.environment.redis.total_expires !== undefined && run.environment.redis.total_expires > 0 &&
                          ` (${run.environment.redis.total_expires.toLocaleString()} with TTL)`}
                      </Descriptions.Item>
                    )}
                    {run.environment?.redis?.evicted_keys !== undefined && run.environment.redis.evicted_keys > 0 && (
                      <Descriptions.Item label="Evicted Keys">
                        <Text type="warning">{run.environment.redis.evicted_keys.toLocaleString()}</Text>
                      </Descriptions.Item>
                    )}
                    {run.environment?.redis?.instantaneous_ops_per_sec !== undefined && run.environment.redis.instantaneous_ops_per_sec > 0 && (
                      <Descriptions.Item label="Baseline Load">
                        {run.environment.redis.instantaneous_ops_per_sec.toLocaleString()} ops/sec
                      </Descriptions.Item>
                    )}
                  </Descriptions>

                  <Divider style={{ margin: '8px 0' }} />
                  <Descriptions column={1} size="small">
                    {run.environment?.network_latency_ms !== undefined && run.environment.network_latency_ms > 0 && (
                      <Descriptions.Item label="Network Latency">
                        {run.environment.network_latency_ms.toFixed(2)} ms
                      </Descriptions.Item>
                    )}
                    <Descriptions.Item label="Fingerprint">
                      <Text code style={{ fontSize: 10 }}>{run.environment?.fingerprint || '-'}</Text>
                    </Descriptions.Item>
                  </Descriptions>
                </Space>
              </Card>
            </Col>
          </Row>
        </TabPane>

        {analysis && (
          <TabPane tab="Analysis" key="analysis">
            <Row gutter={[16, 16]}>
              {analysis.analyzers?.map((analyzer) => (
                <Col xs={24} lg={12} key={analyzer.name}>
                  <Card
                    title={analyzer.name}
                    extra={
                      <Tag color={analyzer.score >= 0.8 ? 'green' : analyzer.score >= 0.6 ? 'orange' : 'red'}>
                        Score: {(analyzer.score * 100).toFixed(0)}%
                      </Tag>
                    }
                    style={{ background: '#1f1f1f', border: '1px solid #303030' }}
                  >
                    {analyzer.findings?.map((finding, i) => (
                      <div key={i} style={{ marginBottom: 8 }}>
                        <Tag color={
                          finding.severity === 'critical' ? 'red' :
                          finding.severity === 'warning' ? 'orange' : 'blue'
                        }>
                          {finding.severity}
                        </Tag>
                        <Text style={{ marginLeft: 8 }}>{finding.message}</Text>
                      </div>
                    ))}
                    {analyzer.recommendations?.length > 0 && (
                      <div style={{ marginTop: 16 }}>
                        <Text strong style={{ color: 'rgba(255,255,255,0.65)' }}>
                          Recommendations:
                        </Text>
                        <ul style={{ margin: '8px 0', paddingLeft: 20 }}>
                          {analyzer.recommendations.map((rec, i) => (
                            <li key={i}>
                              <Text style={{ color: 'rgba(255,255,255,0.85)' }}>{rec}</Text>
                            </li>
                          ))}
                        </ul>
                      </div>
                    )}
                  </Card>
                </Col>
              ))}
            </Row>
          </TabPane>
        )}

        <TabPane tab="Raw Data" key="raw">
          <Card style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
            <pre style={{ 
              background: '#141414', 
              padding: 16, 
              borderRadius: 4,
              overflow: 'auto',
              maxHeight: 600,
              color: '#52c41a',
              fontSize: 12,
            }}>
              {JSON.stringify(run, null, 2)}
            </pre>
          </Card>
        </TabPane>
      </Tabs>

      {/* Baseline Modal */}
      <Modal
        title="Save as Baseline"
        open={baselineModalOpen}
        onOk={createBaseline}
        onCancel={() => setBaselineModalOpen(false)}
        okText="Create"
        okButtonProps={{ disabled: !baselineName }}
      >
        <Input
          placeholder="Baseline name"
          value={baselineName}
          onChange={(e) => setBaselineName(e.target.value)}
        />
      </Modal>
    </div>
  );
}
