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
  ExperimentOutlined,
  SettingOutlined,
  AppstoreOutlined,
  BookOutlined,
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
      label: 'Infrastructure',
    },
    {
      key: '/infra-profiles',
      icon: <AppstoreOutlined />,
      label: 'Infrastructure Profiles',
    },
    {
      key: '/workloads',
      icon: <ExperimentOutlined />,
      label: 'Workload Profiles',
    },
    {
      key: '/run-profiles',
      icon: <SettingOutlined />,
      label: 'Run Profiles',
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
    {
      key: '/docs',
      icon: <BookOutlined />,
      label: 'Documentation',
    },
  ];

  return (
    <Sider
      width={240}
      style={{
        background: '#141414',
        borderRight: '1px solid #333333',
      }}
    >
      <div
        style={{
          height: 64,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          borderBottom: '1px solid #333333',
          background: '#141414',
        }}
      >
        <BarChartOutlined
          style={{ fontSize: 24, color: '#DC382D', marginRight: 8 }}
        />
        <span
          style={{
            fontSize: 18,
            fontWeight: 600,
            color: '#ffffff',
          }}
        >
          RedisMeter
        </span>
      </div>
      <Menu
        mode="inline"
        theme="dark"
        selectedKeys={[location.pathname]}
        style={{
          background: '#141414',
          borderRight: 0,
          marginTop: 8,
        }}
        items={menuItems}
        onClick={({ key }) => navigate(key)}
      />
    </Sider>
  );
}
