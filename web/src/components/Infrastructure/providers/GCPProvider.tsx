import { 
  Form, 
  Select, 
  InputNumber, 
  Switch, 
  Card, 
  Space,
  Alert,
  Row,
  Col,
} from 'antd';
import { 
  CloudServerOutlined, 
  DatabaseOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import { ProviderFormProps } from '@/types';
import { PROVIDER_INFO } from './providerInfo';

const { Option, OptGroup } = Select;

const gcpInfo = PROVIDER_INFO.gcp;

// Group instance types by category
const groupedInstances = gcpInfo.instance_types.reduce((acc, inst) => {
  if (!acc[inst.category]) acc[inst.category] = [];
  acc[inst.category].push(inst);
  return acc;
}, {} as Record<string, typeof gcpInfo.instance_types>);

const categoryLabels: Record<string, string> = {
  general: 'General Purpose (E2)',
  compute: 'Compute Optimized (C2)',
  memory: 'Memory Optimized (N2)',
};

export default function GCPProvider(_props: ProviderFormProps) {
  return (
    <div>
      <Alert
        message="Google Cloud Support Coming Soon"
        description="Memorystore for Redis and Compute Engine runner support is under development."
        type="info"
        showIcon
        style={{ marginBottom: 24 }}
      />

      {/* Region Selection */}
      <Card
        title={
          <Space>
            <CloudServerOutlined style={{ color: '#4285f4' }} />
            <span>GCP Region</span>
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
            placeholder="Select GCP region"
            disabled={true}
            showSearch
          >
            {gcpInfo.regions.map(region => (
              <Option key={region.id} value={region.id}>
                {region.name}
              </Option>
            ))}
          </Select>
        </Form.Item>

        <Form.Item
          name="project_id"
          label="Project ID"
        >
          <Select placeholder="Select project" disabled={true} />
        </Form.Item>
      </Card>

      {/* Memorystore Configuration */}
      <Card
        title={
          <Space>
            <DatabaseOutlined style={{ color: '#DC382D' }} />
            <span>Memorystore for Redis</span>
            <Form.Item name="memorystore_enabled" valuePropName="checked" noStyle>
              <Switch size="small" disabled={true} />
            </Form.Item>
          </Space>
        }
        style={{ background: '#1f1f1f', border: '1px solid #303030', marginBottom: 24 }}
        size="small"
      >
        <Form.Item
          name="memorystore_tier"
          label="Tier"
        >
          <Select placeholder="Select tier" disabled={true}>
            {gcpInfo.redis_skus?.map(sku => (
              <Option key={sku.id} value={sku.id}>
                {sku.name} - {sku.description}
              </Option>
            ))}
          </Select>
        </Form.Item>

        <Row gutter={16}>
          <Col span={12}>
            <Form.Item
              name="memorystore_size_gb"
              label="Memory Size (GB)"
            >
              <InputNumber min={1} max={300} disabled={true} style={{ width: '100%' }} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              name="memorystore_replicas"
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
            <span>Compute Engine Runners</span>
          </Space>
        }
        style={{ background: '#1f1f1f', border: '1px solid #303030' }}
        size="small"
      >
        <Form.Item
          name="runner_count"
          label="Number of Runners"
        >
          <InputNumber min={1} max={20} disabled={true} style={{ width: '100%' }} />
        </Form.Item>

        <Form.Item
          name="runner_type"
          label="Machine Type"
        >
          <Select placeholder="Select machine type" disabled={true}>
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
          name="runners_preemptible"
          label="Use Preemptible VMs"
          valuePropName="checked"
        >
          <Switch disabled={true} />
        </Form.Item>
      </Card>
    </div>
  );
}
