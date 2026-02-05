import { 
  Form, 
  Select, 
  InputNumber, 
  Switch, 
  Row, 
  Col, 
  Card, 
  Space,
  Alert,
} from 'antd';
import { 
  CloudServerOutlined, 
  DatabaseOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import { ProviderFormProps } from '@/types';
import { PROVIDER_INFO } from './providerInfo';

const { Option, OptGroup } = Select;

const awsInfo = PROVIDER_INFO.aws;

// Group instance types by category
const groupedInstances = awsInfo.instance_types.reduce((acc, inst) => {
  if (!acc[inst.category]) acc[inst.category] = [];
  acc[inst.category].push(inst);
  return acc;
}, {} as Record<string, typeof awsInfo.instance_types>);

const categoryLabels: Record<string, string> = {
  general: 'General Purpose (M5)',
  compute: 'Compute Optimized (C5)',
  memory: 'Memory Optimized (R5)',
};

export default function AWSProvider({ disabled }: ProviderFormProps) {

  return (
    <div>
      <Alert
        message="AWS Support Coming Soon"
        description="AWS ElastiCache and EC2 runner support is under development. Check back soon!"
        type="info"
        showIcon
        style={{ marginBottom: 24 }}
      />

      {/* Region Selection */}
      <Card
        title={
          <Space>
            <CloudServerOutlined style={{ color: '#ff9900' }} />
            <span>AWS Region</span>
          </Space>
        }
        style={{ background: '#1f1f1f', border: '1px solid #303030', marginBottom: 24 }}
        size="small"
      >
        <Form.Item
          name="region"
          label="Region"
          rules={[{ required: true, message: 'Please select a region' }]}
        >
          <Select 
            placeholder="Select AWS region"
            disabled={disabled || true} // Always disabled for now
            showSearch
            optionFilterProp="children"
          >
            {awsInfo.regions.map(region => (
              <Option key={region.id} value={region.id}>
                {region.name}
              </Option>
            ))}
          </Select>
        </Form.Item>
      </Card>

      {/* ElastiCache Configuration */}
      <Card
        title={
          <Space>
            <DatabaseOutlined style={{ color: '#DC382D' }} />
            <span>Amazon ElastiCache</span>
            <Form.Item name="elasticache_enabled" valuePropName="checked" noStyle>
              <Switch size="small" disabled={true} />
            </Form.Item>
          </Space>
        }
        style={{ background: '#1f1f1f', border: '1px solid #303030', marginBottom: 24 }}
        size="small"
      >
        <Form.Item
          name="elasticache_node_type"
          label="Node Type"
        >
          <Select 
            placeholder="Select node type"
            disabled={true}
          >
            {awsInfo.redis_skus?.map(sku => (
              <Option key={sku.id} value={sku.id}>
                {sku.name} - {sku.description}
              </Option>
            ))}
          </Select>
        </Form.Item>

        <Row gutter={16}>
          <Col span={12}>
            <Form.Item
              name="elasticache_cluster_mode"
              label="Cluster Mode"
              valuePropName="checked"
            >
              <Switch disabled={true} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="elasticache_replicas"
              label="Read Replicas"
            >
              <InputNumber min={0} max={5} disabled={true} style={{ width: '100%' }} />
            </Form.Item>
          </Col>
        </Row>
      </Card>

      {/* Runner VMs Configuration */}
      <Card
        title={
          <Space>
            <ThunderboltOutlined style={{ color: '#52c41a' }} />
            <span>EC2 Runner Instances</span>
          </Space>
        }
        style={{ background: '#1f1f1f', border: '1px solid #303030' }}
        size="small"
      >
        <Form.Item
          name="runner_count"
          label="Number of Runners"
        >
          <InputNumber 
            min={1} 
            max={20} 
            disabled={true}
            style={{ width: '100%' }}
          />
        </Form.Item>

        <Form.Item
          name="runner_type"
          label="Instance Type"
        >
          <Select 
            placeholder="Select instance type"
            disabled={true}
          >
            {Object.entries(groupedInstances).map(([category, instances]) => (
              <OptGroup key={category} label={categoryLabels[category] || category}>
                {instances.map(inst => (
                  <Option key={inst.id} value={inst.id}>
                    {inst.name} ({inst.vcpus} vCPU, {inst.memory_gb}GB)
                  </Option>
                ))}
              </OptGroup>
            ))}
          </Select>
        </Form.Item>

        <Form.Item
          name="runners_spot"
          label="Use Spot Instances"
          valuePropName="checked"
        >
          <Switch disabled={true} />
        </Form.Item>
      </Card>
    </div>
  );
}
