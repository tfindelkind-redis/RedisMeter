import { useState, useEffect } from 'react';
import {
  Card,
  Table,
  Button,
  Space,
  Typography,
  Tag,
  Modal,
  Form,
  Input,
  InputNumber,
  Select,
  Switch,
  Row,
  Col,
  Divider,
  message,
  Popconfirm,
  Tooltip,
  Empty,
  Collapse,
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  CopyOutlined,
  SettingOutlined,
  LockOutlined,
  RocketOutlined,
  ThunderboltOutlined,
  ClockCircleOutlined,
} from '@ant-design/icons';
import { ColumnsType } from 'antd/es/table';
import api from '@/api/client';

const { Title, Text } = Typography;
const { Option } = Select;
const { TextArea } = Input;

interface RunProfile {
  name: string;
  description: string;
  is_builtin?: boolean;
  // Parallelism
  threads: number;
  clients: number;
  // Duration/Requests
  duration?: string;
  requests?: number;
  // Pipelining
  pipeline: number;
  // Rate limiting
  rate_limit?: number;
  // Iterations
  run_count?: number;
  // Connection
  reconnect_interval?: number;
  protocol?: string;
  select_db?: number;
  // Advanced
  distinct_client_seed?: boolean;
  randomize_seed?: boolean;
  multi_key_get?: number;
  // Output
  hide_histogram?: boolean;
}

const PROTOCOLS = ['redis', 'resp2', 'resp3'];

export default function RunProfiles() {
  const [profiles, setProfiles] = useState<RunProfile[]>([]);
  const [loading, setLoading] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [editingProfile, setEditingProfile] = useState<RunProfile | null>(null);
  const [form] = Form.useForm();
  const [durationMode, setDurationMode] = useState<'duration' | 'requests'>('duration');

  useEffect(() => {
    loadProfiles();
  }, []);

  const loadProfiles = async () => {
    try {
      const data = await api.getRunProfilesFull();
      setProfiles(data);
    } catch (error) {
      console.error('Failed to load run profiles', error);
      message.error('Failed to load run profiles');
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = () => {
    setEditingProfile(null);
    setDurationMode('duration');
    form.resetFields();
    form.setFieldsValue({
      threads: 4,
      clients: 50,
      duration: '30s',
      pipeline: 1,
      run_count: 1,
      protocol: 'redis',
      select_db: 0,
    });
    setModalOpen(true);
  };

  const handleEdit = (profile: RunProfile) => {
    if (profile.is_builtin) {
      message.warning('Cannot edit built-in profiles. Use duplicate instead.');
      return;
    }
    setEditingProfile(profile);
    setDurationMode(profile.requests ? 'requests' : 'duration');
    form.setFieldsValue({
      ...profile,
      duration_mode: profile.requests ? 'requests' : 'duration',
    });
    setModalOpen(true);
  };

  const handleDuplicate = (profile: RunProfile) => {
    setEditingProfile(null);
    setDurationMode(profile.requests ? 'requests' : 'duration');
    form.setFieldsValue({
      ...profile,
      name: `${profile.name}-copy`,
      duration_mode: profile.requests ? 'requests' : 'duration',
    });
    setModalOpen(true);
  };

  const handleDelete = async (name: string) => {
    try {
      await api.deleteRunProfile(name);
      message.success('Run profile deleted');
      loadProfiles();
    } catch (error: unknown) {
      const err = error as { message?: string };
      message.error(err.message || 'Failed to delete run profile');
    }
  };

  const handleSave = async () => {
    try {
      const values = await form.validateFields();

      const profile: RunProfile = {
        name: values.name,
        description: values.description || '',
        threads: values.threads,
        clients: values.clients,
        pipeline: values.pipeline,
        run_count: values.run_count || 1,
        protocol: values.protocol || 'redis',
        select_db: values.select_db || 0,
        rate_limit: values.rate_limit || 0,
        reconnect_interval: values.reconnect_interval || 0,
        distinct_client_seed: values.distinct_client_seed || false,
        randomize_seed: values.randomize_seed || false,
        multi_key_get: values.multi_key_get || 0,
        hide_histogram: values.hide_histogram || false,
      };

      // Set duration or requests based on mode
      if (durationMode === 'requests') {
        profile.requests = values.requests;
      } else {
        profile.duration = values.duration;
      }

      if (editingProfile) {
        await api.updateRunProfile(editingProfile.name, profile);
        message.success('Run profile updated');
      } else {
        await api.createRunProfile(profile);
        message.success('Run profile created');
      }

      setModalOpen(false);
      loadProfiles();
    } catch (error: unknown) {
      const err = error as { message?: string };
      message.error(err.message || 'Failed to save run profile');
    }
  };

  const getProfileIcon = (profile: RunProfile) => {
    if (profile.name.includes('stress') || profile.name.includes('high-load')) {
      return <ThunderboltOutlined style={{ color: '#f5222d' }} />;
    }
    if (profile.name.includes('quick') || profile.name.includes('low-latency')) {
      return <ClockCircleOutlined style={{ color: '#52c41a' }} />;
    }
    if (profile.name.includes('throughput')) {
      return <RocketOutlined style={{ color: '#1890ff' }} />;
    }
    return <SettingOutlined style={{ color: '#faad14' }} />;
  };

  const columns: ColumnsType<RunProfile> = [
    {
      title: 'Name',
      dataIndex: 'name',
      key: 'name',
      render: (name: string, record: RunProfile) => (
        <Space>
          {getProfileIcon(record)}
          <Text strong>{name}</Text>
          {record.is_builtin && (
            <Tooltip title="Built-in profile (read-only)">
              <LockOutlined style={{ color: '#999' }} />
            </Tooltip>
          )}
        </Space>
      ),
    },
    {
      title: 'Description',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: 'Parallelism',
      key: 'parallelism',
      width: 150,
      render: (_, record: RunProfile) => (
        <Space direction="vertical" size={0}>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {record.threads} threads × {record.clients} clients
          </Text>
          <Text type="secondary" style={{ fontSize: 12 }}>
            = {record.threads * record.clients} connections
          </Text>
        </Space>
      ),
    },
    {
      title: 'Duration',
      key: 'duration',
      width: 120,
      render: (_, record: RunProfile) => {
        if (record.requests) {
          return <Tag color="purple">{record.requests.toLocaleString()} reqs</Tag>;
        }
        return <Tag color="blue">{record.duration || '30s'}</Tag>;
      },
    },
    {
      title: 'Pipeline',
      dataIndex: 'pipeline',
      key: 'pipeline',
      width: 80,
      render: (pipeline: number) => (
        <Tag color={pipeline > 1 ? 'orange' : 'default'}>{pipeline}</Tag>
      ),
    },
    {
      title: 'Rate Limit',
      key: 'rate_limit',
      width: 100,
      render: (_, record: RunProfile) => {
        if (record.rate_limit && record.rate_limit > 0) {
          return <Tag color="volcano">{record.rate_limit.toLocaleString()}/s</Tag>;
        }
        return <Text type="secondary">-</Text>;
      },
    },
    {
      title: 'Actions',
      key: 'actions',
      width: 150,
      render: (_, record: RunProfile) => (
        <Space>
          <Tooltip title="Duplicate">
            <Button type="text" icon={<CopyOutlined />} onClick={() => handleDuplicate(record)} />
          </Tooltip>
          {!record.is_builtin && (
            <>
              <Tooltip title="Edit">
                <Button type="text" icon={<EditOutlined />} onClick={() => handleEdit(record)} />
              </Tooltip>
              <Popconfirm
                title="Delete Run Profile"
                description="Are you sure you want to delete this profile?"
                onConfirm={() => handleDelete(record.name)}
                okText="Delete"
                okButtonProps={{ danger: true }}
              >
                <Tooltip title="Delete">
                  <Button type="text" danger icon={<DeleteOutlined />} />
                </Tooltip>
              </Popconfirm>
            </>
          )}
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Row justify="space-between" align="middle" style={{ marginBottom: 24 }}>
        <Col>
          <Title level={3} style={{ color: 'rgba(255,255,255,0.85)', margin: 0 }}>
            <SettingOutlined style={{ marginRight: 8, color: '#1890ff' }} />
            Run Profiles
          </Title>
          <Text type="secondary">
            Configure how benchmarks execute (threads, clients, duration, pipelining)
          </Text>
        </Col>
        <Col>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
            Create Profile
          </Button>
        </Col>
      </Row>

      <Card style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
        <Table
          columns={columns}
          dataSource={profiles}
          rowKey="name"
          loading={loading}
          pagination={false}
          locale={{
            emptyText: (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description="No run profiles found"
              >
                <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
                  Create Your First Profile
                </Button>
              </Empty>
            ),
          }}
        />
      </Card>

      {/* Run Profile Editor Modal */}
      <Modal
        title={editingProfile ? 'Edit Run Profile' : 'Create Run Profile'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={handleSave}
        width={800}
        okText={editingProfile ? 'Update' : 'Create'}
      >
        <Form form={form} layout="vertical">
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="name"
                label="Profile Name"
                rules={[
                  { required: true, message: 'Please enter a name' },
                  { pattern: /^[a-z0-9-]+$/, message: 'Only lowercase letters, numbers, and hyphens' }
                ]}
              >
                <Input placeholder="my-run-profile" disabled={!!editingProfile} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="protocol" label="Protocol">
                <Select>
                  {PROTOCOLS.map(p => (
                    <Option key={p} value={p}>{p.toUpperCase()}</Option>
                  ))}
                </Select>
              </Form.Item>
            </Col>
          </Row>

          <Form.Item name="description" label="Description">
            <TextArea rows={2} placeholder="Describe what this profile is optimized for..." />
          </Form.Item>

          <Divider>Parallelism</Divider>

          <Row gutter={16}>
            <Col span={8}>
              <Form.Item name="threads" label="Threads" rules={[{ required: true }]}>
                <InputNumber min={1} max={64} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="clients" label="Clients per Thread" rules={[{ required: true }]}>
                <InputNumber min={1} max={1000} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item label="Total Connections">
                <Form.Item noStyle shouldUpdate>
                  {({ getFieldValue }) => (
                    <InputNumber
                      value={(getFieldValue('threads') || 4) * (getFieldValue('clients') || 50)}
                      disabled
                      style={{ width: '100%' }}
                    />
                  )}
                </Form.Item>
              </Form.Item>
            </Col>
          </Row>

          <Divider>Duration / Requests</Divider>

          <Row gutter={16}>
            <Col span={8}>
              <Form.Item label="Mode">
                <Select
                  value={durationMode}
                  onChange={(v) => setDurationMode(v)}
                >
                  <Option value="duration">Duration-based</Option>
                  <Option value="requests">Request count</Option>
                </Select>
              </Form.Item>
            </Col>
            {durationMode === 'duration' ? (
              <Col span={8}>
                <Form.Item name="duration" label="Duration" rules={[{ required: true }]}>
                  <Input placeholder="30s, 5m, 1h" />
                </Form.Item>
              </Col>
            ) : (
              <Col span={8}>
                <Form.Item name="requests" label="Requests per Client" rules={[{ required: true }]}>
                  <InputNumber min={1} max={100000000} style={{ width: '100%' }} />
                </Form.Item>
              </Col>
            )}
            <Col span={8}>
              <Form.Item name="run_count" label="Iterations">
                <InputNumber min={1} max={100} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>

          <Divider>Pipelining & Rate Limiting</Divider>

          <Row gutter={16}>
            <Col span={8}>
              <Form.Item name="pipeline" label="Pipeline Depth">
                <InputNumber min={1} max={1000} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="rate_limit" label="Rate Limit (req/s/conn)">
                <InputNumber min={0} max={10000000} style={{ width: '100%' }} placeholder="0 = unlimited" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="reconnect_interval" label="Reconnect Every N Requests">
                <InputNumber min={0} style={{ width: '100%' }} placeholder="0 = never" />
              </Form.Item>
            </Col>
          </Row>

          <Collapse ghost style={{ marginTop: 16 }}>
            <Collapse.Panel header="Advanced Settings" key="advanced">
              <Row gutter={16}>
                <Col span={8}>
                  <Form.Item name="select_db" label="Redis DB Number">
                    <InputNumber min={0} max={15} style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
                <Col span={8}>
                  <Form.Item name="multi_key_get" label="Multi-Key GET (keys)">
                    <InputNumber min={0} max={1000} style={{ width: '100%' }} placeholder="0 = disabled" />
                  </Form.Item>
                </Col>
              </Row>
              <Row gutter={16}>
                <Col span={8}>
                  <Form.Item name="distinct_client_seed" valuePropName="checked" label="Distinct Client Seed">
                    <Switch />
                  </Form.Item>
                </Col>
                <Col span={8}>
                  <Form.Item name="randomize_seed" valuePropName="checked" label="Randomize Seed">
                    <Switch />
                  </Form.Item>
                </Col>
                <Col span={8}>
                  <Form.Item name="hide_histogram" valuePropName="checked" label="Hide Histogram">
                    <Switch />
                  </Form.Item>
                </Col>
              </Row>
            </Collapse.Panel>
          </Collapse>
        </Form>
      </Modal>
    </div>
  );
}
