import { Layout, Badge, Space, Typography, Progress } from 'antd';
import { WifiOutlined, DisconnectOutlined, SyncOutlined } from '@ant-design/icons';
import { useStore } from '@/store';

const { Header: AntHeader } = Layout;
const { Text } = Typography;

export default function Header() {
  const { wsConnected, activeBenchmark } = useStore();

  return (
    <AntHeader
      style={{
        background: '#141414',
        padding: '0 24px',
        borderBottom: '1px solid #333333',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
      }}
    >
      <div>
        {activeBenchmark && (
          <Space>
            <SyncOutlined spin style={{ color: '#1890ff' }} />
            <Text style={{ color: 'rgba(255,255,255,0.85)' }}>
              Benchmark running...
            </Text>
            <Progress
              percent={Math.round(activeBenchmark.progress * 100)}
              size="small"
              style={{ width: 200 }}
            />
          </Space>
        )}
      </div>
      <Space>
        <Badge
          status={wsConnected ? 'success' : 'error'}
          text={
            <Text style={{ color: 'rgba(255,255,255,0.65)' }}>
              {wsConnected ? (
                <>
                  <WifiOutlined /> Connected
                </>
              ) : (
                <>
                  <DisconnectOutlined /> Disconnected
                </>
              )}
            </Text>
          }
        />
      </Space>
    </AntHeader>
  );
}
