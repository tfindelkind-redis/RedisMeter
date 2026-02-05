import { Routes, Route } from 'react-router-dom';
import { Layout } from 'antd';
import Sidebar from '@/components/Layout/Sidebar';
import Header from '@/components/Layout/Header';
import Dashboard from '@/pages/Dashboard';
import Runs from '@/pages/Runs';
import RunDetail from '@/pages/RunDetail';
import Baselines from '@/pages/Baselines';
import Compare from '@/pages/Compare';
import NewBenchmark from '@/pages/NewBenchmark';
import Analytics from '@/pages/Analytics';
import InfrastructureList from '@/pages/InfrastructureList';
import InfrastructureNew from '@/pages/InfrastructureNew';
import InfrastructureDetail from '@/pages/InfrastructureDetail';
import { useGlobalWebSocket } from '@/hooks/useWebSocket';
import { useEffect } from 'react';
import { useStore } from '@/store';

const { Content } = Layout;

function App() {
  // Initialize WebSocket connection
  useGlobalWebSocket();

  const { fetchRuns, fetchBaselines, fetchWorkloads } = useStore();

  // Fetch initial data
  useEffect(() => {
    fetchRuns();
    fetchBaselines();
    fetchWorkloads();
  }, [fetchRuns, fetchBaselines, fetchWorkloads]);

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sidebar />
      <Layout>
        <Header />
        <Content style={{ margin: '24px', overflow: 'auto' }}>
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/runs" element={<Runs />} />
            <Route path="/runs/:id" element={<RunDetail />} />
            <Route path="/baselines" element={<Baselines />} />
            <Route path="/compare" element={<Compare />} />
            <Route path="/new" element={<NewBenchmark />} />
            <Route path="/analytics" element={<Analytics />} />
            <Route path="/infrastructure" element={<InfrastructureList />} />
            <Route path="/infrastructure/new" element={<InfrastructureNew />} />
            <Route path="/infrastructure/:id" element={<InfrastructureDetail />} />
          </Routes>
        </Content>
      </Layout>
    </Layout>
  );
}

export default App;
