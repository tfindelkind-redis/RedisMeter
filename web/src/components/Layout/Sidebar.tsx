import { Layout, Menu, Badge } from 'antd';
import {
  DashboardOutlined,
  ThunderboltOutlined,
  DatabaseOutlined,
  BarChartOutlined,
  DiffOutlined,
  PlusOutlined,
  LineChartOutlined,
  CloudOutlined,
} from '@ant-design/icons';
import { useNavigate, useLocation } from 'react-router-dom';
import { useStore } from '@/store';

const { Sider } = Layout;

export default function Sidebar() {
  const navigate = useNavigate();
  const location = useLocation();
  const { activeBenchmark } = useStore();

  const menuItems = [
    {
      key: '/',
      icon: <DashboardOutlined />,
      label: 'Dashboard',
    },
    {
      key: '/new',
      icon: <PlusOutlined />,
      label: (
        <span>
          New Benchmark
          {activeBenchmark && (
            <Badge status="processing" style={{ marginLeft: 8 }} />
          )}
        </span>
      ),
    },
    {
      key: '/runs',
      icon: <ThunderboltOutlined />,
      label: 'Benchmark Runs',
    },
    {
      type: 'divider' as const,
    },
    {
      key: '/infrastructure',
      icon: <CloudOutlined />,
      label: 'Cloud Infrastructure',
    },
    {
      type: 'divider' as const,
    },
    {
      key: '/baselines',
      icon: <DatabaseOutlined />,
      label: 'Baselines',
    },
    {
      key: '/compare',
      icon: <DiffOutlined />,
      label: 'Compare',
    },
    {
      key: '/analytics',
      icon: <LineChartOutlined />,
      label: 'Analytics',
    },
  ];

  return (
    <Sider
      width={240}
      style={{
        background: '#fafafa',
        borderRight: '1px solid #e8e8e8',
      }}
    >
      <div
        style={{
          height: 64,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          borderBottom: '1px solid #e8e8e8',
          background: '#fafafa',
        }}
      >
        <BarChartOutlined
          style={{ fontSize: 24, color: '#DC382D', marginRight: 8 }}
        />
        <span
          style={{
            fontSize: 18,
            fontWeight: 600,
            color: '#1f1f1f',
          }}
        >
          RedisMeter
        </span>
      </div>
      <Menu
        mode="inline"
        theme="light"
        selectedKeys={[location.pathname]}
        style={{
          background: '#fafafa',
          borderRight: 0,
          marginTop: 8,
        }}
        items={menuItems}
        onClick={({ key }) => navigate(key)}
      />
    </Sider>
  );
}
