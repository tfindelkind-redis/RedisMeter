import { Tag } from 'antd';
import { RunStatus } from '@/types';
import {
  ClockCircleOutlined,
  SyncOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  StopOutlined,
} from '@ant-design/icons';

interface StatusBadgeProps {
  status: RunStatus;
  size?: 'small' | 'default';
}

export default function StatusBadge({ status, size = 'default' }: StatusBadgeProps) {
  const config = {
    pending: {
      color: 'default',
      icon: <ClockCircleOutlined />,
      text: 'Pending',
    },
    running: {
      color: 'processing',
      icon: <SyncOutlined spin />,
      text: 'Running',
    },
    completed: {
      color: 'success',
      icon: <CheckCircleOutlined />,
      text: 'Completed',
    },
    failed: {
      color: 'error',
      icon: <CloseCircleOutlined />,
      text: 'Failed',
    },
    cancelled: {
      color: 'warning',
      icon: <StopOutlined />,
      text: 'Cancelled',
    },
  };

  const { color, icon, text } = config[status] || config.pending;

  return (
    <Tag color={color} icon={icon} style={size === 'small' ? { fontSize: 12 } : undefined}>
      {text}
    </Tag>
  );
}
