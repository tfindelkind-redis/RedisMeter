import { useState } from 'react';
import {
  Form,
  Input,
  InputNumber,
  Switch,
  Select,
  Card,
  Typography,
  Space,
  Button,
  Row,
  Col,
  Alert,
  Divider,
  Collapse,
  Tag,
  Tooltip,
  Table,
} from 'antd';
import {
  ClusterOutlined,
  DatabaseOutlined,
  LockOutlined,
  KeyOutlined,
  PlusOutlined,
  DeleteOutlined,
  SafetyCertificateOutlined,
  InfoCircleOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
} from '@ant-design/icons';
import { ProviderFormProps } from '@/types';

const { Paragraph } = Typography;
const { Option } = Select;
const { TextArea } = Input;
const { Panel } = Collapse;

interface RunnerEntry {
  id: string;
  name: string;
  host: string;
  port: number;
}

interface RedisTargetEntry {
  id: string;
  name: string;
  host: string;
  port: number;
  is_cluster: boolean;
}

export default function SelfManagedProvider({ form, disabled }: ProviderFormProps) {
  const sshAuthMethod = Form.useWatch('ssh_auth_method', form);
  const tlsEnabled = Form.useWatch('redis_tls_enabled', form);
  
  const [runners, setRunners] = useState<RunnerEntry[]>([
    { id: '1', name: 'runner-1', host: '', port: 22 }
  ]);
  
  const [redisTargets, setRedisTargets] = useState<RedisTargetEntry[]>([
    { id: '1', name: 'redis-primary', host: '', port: 6379, is_cluster: false }
  ]);

  // Add a new runner
  const addRunner = () => {
    const newId = String(runners.length + 1);
    setRunners([...runners, { 
      id: newId, 
      name: `runner-${newId}`, 
      host: '', 
      port: 22 
    }]);
  };

  // Remove a runner
  const removeRunner = (id: string) => {
    if (runners.length > 1) {
      setRunners(runners.filter(r => r.id !== id));
    }
  };

  // Update runner field
  const updateRunner = (id: string, field: keyof RunnerEntry, value: any) => {
    setRunners(runners.map(r => 
      r.id === id ? { ...r, [field]: value } : r
    ));
    // Update form field for submission
    form.setFieldValue('runners_list', runners.map(r => 
      r.id === id ? { ...r, [field]: value } : r
    ));
  };

  // Add a new Redis target
  const addRedisTarget = () => {
    const newId = String(redisTargets.length + 1);
    setRedisTargets([...redisTargets, { 
      id: newId, 
      name: `redis-${newId}`, 
      host: '', 
      port: 6379,
      is_cluster: false
    }]);
  };

  // Remove a Redis target
  const removeRedisTarget = (id: string) => {
    if (redisTargets.length > 1) {
      setRedisTargets(redisTargets.filter(t => t.id !== id));
    }
  };

  // Update Redis target field
  const updateRedisTarget = (id: string, field: keyof RedisTargetEntry, value: any) => {
    setRedisTargets(redisTargets.map(t => 
      t.id === id ? { ...t, [field]: value } : t
    ));
    form.setFieldValue('redis_targets_list', redisTargets.map(t => 
      t.id === id ? { ...t, [field]: value } : t
    ));
  };

  // Initialize form values
  useState(() => {
    form.setFieldValue('runners_list', runners);
    form.setFieldValue('redis_targets_list', redisTargets);
  });

  const runnerColumns = [
    {
      title: 'Name',
      dataIndex: 'name',
      key: 'name',
      width: 150,
      render: (_: any, record: RunnerEntry) => (
        <Input
          size="small"
          value={record.name}
          onChange={(e) => updateRunner(record.id, 'name', e.target.value)}
          disabled={disabled}
          placeholder="runner-1"
        />
      ),
    },
    {
      title: 'Host (IP/Hostname)',
      dataIndex: 'host',
      key: 'host',
      render: (_: any, record: RunnerEntry) => (
        <Input
          size="small"
          value={record.host}
          onChange={(e) => updateRunner(record.id, 'host', e.target.value)}
          disabled={disabled}
          placeholder="192.168.1.100"
        />
      ),
    },
    {
      title: 'SSH Port',
      dataIndex: 'port',
      key: 'port',
      width: 100,
      render: (_: any, record: RunnerEntry) => (
        <InputNumber
          size="small"
          min={1}
          max={65535}
          value={record.port}
          onChange={(v) => updateRunner(record.id, 'port', v || 22)}
          disabled={disabled}
        />
      ),
    },
    {
      title: '',
      key: 'action',
      width: 50,
      render: (_: any, record: RunnerEntry) => (
        runners.length > 1 && (
          <Button
            type="text"
            danger
            size="small"
            icon={<DeleteOutlined />}
            onClick={() => removeRunner(record.id)}
            disabled={disabled}
          />
        )
      ),
    },
  ];

  const redisColumns = [
    {
      title: 'Name',
      dataIndex: 'name',
      key: 'name',
      width: 150,
      render: (_: any, record: RedisTargetEntry) => (
        <Input
          size="small"
          value={record.name}
          onChange={(e) => updateRedisTarget(record.id, 'name', e.target.value)}
          disabled={disabled}
          placeholder="redis-primary"
        />
      ),
    },
    {
      title: 'Host (IP/Hostname)',
      dataIndex: 'host',
      key: 'host',
      render: (_: any, record: RedisTargetEntry) => (
        <Input
          size="small"
          value={record.host}
          onChange={(e) => updateRedisTarget(record.id, 'host', e.target.value)}
          disabled={disabled}
          placeholder="redis.example.com"
        />
      ),
    },
    {
      title: 'Port',
      dataIndex: 'port',
      key: 'port',
      width: 100,
      render: (_: any, record: RedisTargetEntry) => (
        <InputNumber
          size="small"
          min={1}
          max={65535}
          value={record.port}
          onChange={(v) => updateRedisTarget(record.id, 'port', v || 6379)}
          disabled={disabled}
        />
      ),
    },
    {
      title: 'Cluster',
      dataIndex: 'is_cluster',
      key: 'is_cluster',
      width: 80,
      render: (_: any, record: RedisTargetEntry) => (
        <Switch
          size="small"
          checked={record.is_cluster}
          onChange={(v) => updateRedisTarget(record.id, 'is_cluster', v)}
          disabled={disabled}
        />
      ),
    },
    {
      title: '',
      key: 'action',
      width: 50,
      render: (_: any, record: RedisTargetEntry) => (
        redisTargets.length > 1 && (
          <Button
            type="text"
            danger
            size="small"
            icon={<DeleteOutlined />}
            onClick={() => removeRedisTarget(record.id)}
            disabled={disabled}
          />
        )
      ),
    },
  ];

  return (
    <div>
      <Alert
        message="Self-Managed Infrastructure"
        description="Configure your own machines as benchmark runners and connect to any Redis deployment. All credentials are encrypted before storage."
        type="info"
        showIcon
        icon={<SafetyCertificateOutlined />}
        style={{ marginBottom: 24 }}
      />

      {/* Hidden form fields for data */}
      <Form.Item name="runners_list" hidden>
        <Input />
      </Form.Item>
      <Form.Item name="redis_targets_list" hidden>
        <Input />
      </Form.Item>

      {/* Runner Machines Section */}
      <Card
        title={
          <Space>
            <ClusterOutlined style={{ color: '#1890ff' }} />
            <span>Runner Machines</span>
            <Tag color="blue">{runners.length} machine(s)</Tag>
          </Space>
        }
        style={{ background: '#141414', border: '1px solid #303030', marginBottom: 24 }}
        extra={
          <Button
            type="dashed"
            size="small"
            icon={<PlusOutlined />}
            onClick={addRunner}
            disabled={disabled}
          >
            Add Runner
          </Button>
        }
      >
        <Paragraph type="secondary" style={{ marginBottom: 16 }}>
          Machines that will execute benchmark tools (memtier_benchmark, ftsb, etc.)
        </Paragraph>

        <Table
          dataSource={runners}
          columns={runnerColumns}
          rowKey="id"
          pagination={false}
          size="small"
          style={{ marginBottom: 24 }}
        />

        <Divider orientation="left">
          <Space>
            <KeyOutlined style={{ color: '#faad14' }} />
            SSH Authentication
          </Space>
        </Divider>

        <Alert
          message="Password Security"
          description="Passwords and private keys are encrypted using AES-256-GCM before storage. They are never stored in plain text."
          type="warning"
          showIcon
          icon={<LockOutlined />}
          style={{ marginBottom: 16 }}
        />

        <Row gutter={24}>
          <Col span={8}>
            <Form.Item
              name="ssh_auth_method"
              label="Authentication Method"
              initialValue="key"
              rules={[{ required: true }]}
            >
              <Select disabled={disabled}>
                <Option value="key">SSH Private Key (Paste)</Option>
                <Option value="key_file">SSH Key File (Path on Server)</Option>
                <Option value="password">Password</Option>
              </Select>
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item
              name="ssh_username"
              label="SSH Username"
              rules={[{ required: true, message: 'Username is required' }]}
            >
              <Input 
                placeholder="ubuntu" 
                disabled={disabled}
                prefix={<SafetyCertificateOutlined />}
              />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item
              name="ssh_port"
              label="Default SSH Port"
              initialValue={22}
            >
              <InputNumber 
                min={1} 
                max={65535} 
                style={{ width: '100%' }}
                disabled={disabled}
              />
            </Form.Item>
          </Col>
        </Row>

        {sshAuthMethod === 'password' && (
          <Form.Item
            name="ssh_password"
            label={
              <Space>
                SSH Password
                <Tooltip title="Password will be encrypted before storage">
                  <LockOutlined style={{ color: '#52c41a' }} />
                </Tooltip>
              </Space>
            }
            rules={[{ required: true, message: 'Password is required' }]}
          >
            <Input.Password 
              placeholder="Enter SSH password" 
              disabled={disabled}
              iconRender={visible => visible ? <CheckCircleOutlined /> : <CloseCircleOutlined />}
            />
          </Form.Item>
        )}

        {sshAuthMethod === 'key' && (
          <>
            <Form.Item
              name="ssh_private_key"
              label={
                <Space>
                  SSH Private Key
                  <Tooltip title="Private key will be encrypted before storage">
                    <LockOutlined style={{ color: '#52c41a' }} />
                  </Tooltip>
                </Space>
              }
              rules={[{ required: true, message: 'Private key is required' }]}
              extra="Paste your private key (PEM format). It will be encrypted."
            >
              <TextArea 
                rows={6}
                placeholder="-----BEGIN OPENSSH PRIVATE KEY-----&#10;...&#10;-----END OPENSSH PRIVATE KEY-----"
                disabled={disabled}
                style={{ fontFamily: 'monospace', fontSize: 12 }}
              />
            </Form.Item>
            <Form.Item
              name="ssh_key_passphrase"
              label={
                <Space>
                  Key Passphrase (if encrypted)
                  <Tooltip title="Passphrase will be encrypted before storage">
                    <LockOutlined style={{ color: '#52c41a' }} />
                  </Tooltip>
                </Space>
              }
            >
              <Input.Password 
                placeholder="Leave empty if key is not password-protected" 
                disabled={disabled}
              />
            </Form.Item>
          </>
        )}

        {sshAuthMethod === 'key_file' && (
          <Form.Item
            name="ssh_private_key_path"
            label="Private Key Path (on RedisMeter server)"
            rules={[{ required: true, message: 'Key path is required' }]}
            extra="Path to the private key file on the server running RedisMeter"
          >
            <Input 
              placeholder="/home/user/.ssh/id_rsa" 
              disabled={disabled}
              style={{ fontFamily: 'monospace' }}
            />
          </Form.Item>
        )}

        <Collapse ghost style={{ marginTop: 16 }}>
          <Panel header="Advanced SSH Settings" key="advanced-ssh">
            <Row gutter={24}>
              <Col span={8}>
                <Form.Item
                  name="ssh_connect_timeout"
                  label="Connection Timeout (seconds)"
                  initialValue={30}
                >
                  <InputNumber 
                    min={5} 
                    max={300} 
                    style={{ width: '100%' }}
                    disabled={disabled}
                  />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item
                  name="ssh_strict_host_key"
                  label="Strict Host Key Checking"
                  valuePropName="checked"
                  initialValue={false}
                >
                  <Switch disabled={disabled} />
                </Form.Item>
              </Col>
            </Row>
          </Panel>
        </Collapse>
      </Card>

      {/* Redis Target Section */}
      <Card
        title={
          <Space>
            <DatabaseOutlined style={{ color: '#52c41a' }} />
            <span>Redis Target(s)</span>
            <Tag color="green">{redisTargets.length} target(s)</Tag>
          </Space>
        }
        style={{ background: '#141414', border: '1px solid #303030', marginBottom: 24 }}
        extra={
          <Button
            type="dashed"
            size="small"
            icon={<PlusOutlined />}
            onClick={addRedisTarget}
            disabled={disabled}
          >
            Add Target
          </Button>
        }
      >
        <Paragraph type="secondary" style={{ marginBottom: 16 }}>
          Redis instances to run benchmarks against. Enable "Cluster" for Redis Cluster deployments.
        </Paragraph>

        <Table
          dataSource={redisTargets}
          columns={redisColumns}
          rowKey="id"
          pagination={false}
          size="small"
          style={{ marginBottom: 24 }}
        />

        <Divider orientation="left">
          <Space>
            <LockOutlined style={{ color: '#faad14' }} />
            Redis Authentication
          </Space>
        </Divider>

        <Row gutter={24}>
          <Col span={12}>
            <Form.Item
              name="redis_username"
              label="Redis Username"
              extra="Leave empty for default user, or specify ACL username"
            >
              <Input 
                placeholder="default" 
                disabled={disabled}
              />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="redis_password"
              label={
                <Space>
                  Redis Password
                  <Tooltip title="Password will be encrypted before storage">
                    <LockOutlined style={{ color: '#52c41a' }} />
                  </Tooltip>
                </Space>
              }
            >
              <Input.Password 
                placeholder="Redis AUTH password" 
                disabled={disabled}
              />
            </Form.Item>
          </Col>
        </Row>

        <Divider orientation="left">
          <Space>
            <SafetyCertificateOutlined style={{ color: '#722ed1' }} />
            TLS Configuration
          </Space>
        </Divider>

        <Row gutter={24}>
          <Col span={8}>
            <Form.Item
              name="redis_tls_enabled"
              label="Enable TLS"
              valuePropName="checked"
              initialValue={false}
            >
              <Switch disabled={disabled} />
            </Form.Item>
          </Col>
          {tlsEnabled && (
            <Col span={8}>
              <Form.Item
                name="redis_tls_skip_verify"
                label="Skip Certificate Verification"
                valuePropName="checked"
                initialValue={false}
                extra="⚠️ Not recommended for production"
              >
                <Switch disabled={disabled} />
              </Form.Item>
            </Col>
          )}
        </Row>

        {tlsEnabled && (
          <Collapse ghost>
            <Panel header="TLS Certificates (Optional - for mTLS)" key="tls-certs">
              <Row gutter={24}>
                <Col span={12}>
                  <Form.Item
                    name="redis_tls_cert"
                    label="Client Certificate (PEM)"
                    extra="For mutual TLS authentication"
                  >
                    <TextArea
                      rows={4}
                      placeholder="-----BEGIN CERTIFICATE-----&#10;...&#10;-----END CERTIFICATE-----"
                      disabled={disabled}
                      style={{ fontFamily: 'monospace', fontSize: 11 }}
                    />
                  </Form.Item>
                </Col>
                <Col span={12}>
                  <Form.Item
                    name="redis_tls_key"
                    label={
                      <Space>
                        Client Private Key (PEM)
                        <Tooltip title="Key will be encrypted before storage">
                          <LockOutlined style={{ color: '#52c41a' }} />
                        </Tooltip>
                      </Space>
                    }
                  >
                    <TextArea
                      rows={4}
                      placeholder="-----BEGIN PRIVATE KEY-----&#10;...&#10;-----END PRIVATE KEY-----"
                      disabled={disabled}
                      style={{ fontFamily: 'monospace', fontSize: 11 }}
                    />
                  </Form.Item>
                </Col>
              </Row>
              <Form.Item
                name="redis_tls_ca"
                label="CA Certificate (PEM)"
                extra="Custom CA certificate for server verification"
              >
                <TextArea
                  rows={4}
                  placeholder="-----BEGIN CERTIFICATE-----&#10;...&#10;-----END CERTIFICATE-----"
                  disabled={disabled}
                  style={{ fontFamily: 'monospace', fontSize: 11 }}
                />
              </Form.Item>
            </Panel>
          </Collapse>
        )}
      </Card>

      {/* Tool Paths Section */}
      <Card
        title={
          <Space>
            <InfoCircleOutlined style={{ color: '#1890ff' }} />
            <span>Benchmark Tool Paths (Optional)</span>
          </Space>
        }
        style={{ background: '#141414', border: '1px solid #303030' }}
      >
        <Paragraph type="secondary" style={{ marginBottom: 16 }}>
          Specify custom paths if benchmark tools are not in the system PATH on runner machines.
        </Paragraph>

        <Row gutter={24}>
          <Col span={12}>
            <Form.Item
              name="memtier_path"
              label="memtier_benchmark Path"
              extra="Default: memtier_benchmark (from PATH)"
            >
              <Input 
                placeholder="/usr/local/bin/memtier_benchmark" 
                disabled={disabled}
                style={{ fontFamily: 'monospace' }}
              />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="redis_cli_path"
              label="redis-cli Path"
              extra="Default: redis-cli (from PATH)"
            >
              <Input 
                placeholder="/usr/local/bin/redis-cli" 
                disabled={disabled}
                style={{ fontFamily: 'monospace' }}
              />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={24}>
          <Col span={12}>
            <Form.Item
              name="ftsb_path"
              label="ftsb Path (RediSearch benchmark)"
              extra="Default: ftsb_redisearch (from PATH)"
            >
              <Input 
                placeholder="/opt/ftsb/ftsb_redisearch" 
                disabled={disabled}
                style={{ fontFamily: 'monospace' }}
              />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="ann_benchmarks_path"
              label="ann-benchmarks Path"
              extra="Default: python (from PATH with ann_benchmarks module)"
            >
              <Input 
                placeholder="/opt/ann-benchmarks/run.py" 
                disabled={disabled}
                style={{ fontFamily: 'monospace' }}
              />
            </Form.Item>
          </Col>
        </Row>
      </Card>
    </div>
  );
}
