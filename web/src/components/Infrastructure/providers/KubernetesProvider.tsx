import { 
  Form, 
  Select, 
  InputNumber, 
  Input, 
  Card, 
  Space,
  Alert,
  Row,
  Col,
} from 'antd';
import { 
  ClusterOutlined, 
  DatabaseOutlined,
  ContainerOutlined,
} from '@ant-design/icons';
import { ProviderFormProps } from '@/types';

const { Option } = Select;

export default function KubernetesProvider(_props: ProviderFormProps) {
  return (
    <div>
      <Alert
        message="Kubernetes Support Coming Soon"
        description="Deploy benchmark pods on your existing Kubernetes cluster. Support for AKS, EKS, GKE, and on-premises clusters."
        type="info"
        showIcon
        style={{ marginBottom: 24 }}
      />

      {/* Cluster Configuration */}
      <Card
        title={
          <Space>
            <ClusterOutlined style={{ color: '#326ce5' }} />
            <span>Kubernetes Cluster</span>
          </Space>
        }
        style={{ background: '#1f1f1f', border: '1px solid #303030', marginBottom: 24 }}
        size="small"
      >
        <Form.Item
          name="k8s_context"
          label="Kubeconfig Context"
          extra="Leave empty to use current context"
        >
          <Select placeholder="Select context" disabled={true} allowClear>
            <Option value="default">default</Option>
          </Select>
        </Form.Item>

        <Form.Item
          name="k8s_namespace"
          label="Namespace"
          initialValue="redismeter"
        >
          <Input placeholder="redismeter" disabled={true} />
        </Form.Item>
      </Card>

      {/* Redis Target */}
      <Card
        title={
          <Space>
            <DatabaseOutlined style={{ color: '#DC382D' }} />
            <span>Redis Target</span>
          </Space>
        }
        style={{ background: '#1f1f1f', border: '1px solid #303030', marginBottom: 24 }}
        size="small"
      >
        <Form.Item
          name="redis_service"
          label="Redis Service"
          extra="Kubernetes service name or external endpoint"
        >
          <Input placeholder="redis-master.default.svc.cluster.local" disabled={true} />
        </Form.Item>

        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="redis_port" label="Port" initialValue={6379}>
              <InputNumber min={1} max={65535} disabled={true} style={{ width: '100%' }} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="redis_secret" label="Password Secret">
              <Input placeholder="redis-password" disabled={true} />
            </Form.Item>
          </Col>
        </Row>
      </Card>

      {/* Runner Pods Configuration */}
      <Card
        title={
          <Space>
            <ContainerOutlined style={{ color: '#52c41a' }} />
            <span>Benchmark Pods</span>
          </Space>
        }
        style={{ background: '#1f1f1f', border: '1px solid #303030' }}
        size="small"
      >
        <Form.Item
          name="pod_replicas"
          label="Number of Pods"
          extra="Each pod runs memtier_benchmark"
        >
          <InputNumber min={1} max={50} disabled={true} style={{ width: '100%' }} />
        </Form.Item>

        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="pod_cpu" label="CPU Request" initialValue="1">
              <Input placeholder="1" disabled={true} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="pod_memory" label="Memory Request" initialValue="2Gi">
              <Input placeholder="2Gi" disabled={true} />
            </Form.Item>
          </Col>
        </Row>

        <Form.Item
          name="node_selector"
          label="Node Selector"
          extra="Key=Value pairs for node selection"
        >
          <Input placeholder="workload=benchmark" disabled={true} />
        </Form.Item>

        <Form.Item
          name="storage_class"
          label="Storage Class"
          extra="For persistent results storage"
        >
          <Input placeholder="standard" disabled={true} />
        </Form.Item>
      </Card>
    </div>
  );
}
