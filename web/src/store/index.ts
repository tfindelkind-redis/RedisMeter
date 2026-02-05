import { create } from 'zustand';
import { BenchmarkRun, Baseline, Workload } from '@/types';
import api from '@/api/client';

interface AppState {
  // Data
  runs: BenchmarkRun[];
  baselines: Baseline[];
  workloads: Workload[];
  selectedRun: BenchmarkRun | null;
  
  // Loading states
  loadingRuns: boolean;
  loadingBaselines: boolean;
  loadingWorkloads: boolean;
  
  // Active benchmark
  activeBenchmark: {
    id: string;
    status: string;
    progress: number;
  } | null;
  
  // WebSocket connection
  wsConnected: boolean;
  
  // Actions
  fetchRuns: (params?: { limit?: number }) => Promise<void>;
  fetchBaselines: () => Promise<void>;
  fetchWorkloads: () => Promise<void>;
  selectRun: (run: BenchmarkRun | null) => void;
  deleteRun: (id: string) => Promise<void>;
  startBenchmark: (config: {
    target: { host: string; port: number; password?: string };
    workload: string;
    duration?: string;
    threads?: number;
    clients?: number;
  }) => Promise<string>;
  setActiveBenchmark: (benchmark: { id: string; status: string; progress: number } | null) => void;
  setWsConnected: (connected: boolean) => void;
  refreshRun: (id: string) => Promise<void>;
}

export const useStore = create<AppState>((set, get) => ({
  // Initial state
  runs: [],
  baselines: [],
  workloads: [],
  selectedRun: null,
  loadingRuns: false,
  loadingBaselines: false,
  loadingWorkloads: false,
  activeBenchmark: null,
  wsConnected: false,

  // Actions
  fetchRuns: async (params) => {
    set({ loadingRuns: true });
    try {
      const runs = await api.getRuns({ limit: params?.limit || 50 });
      set({ runs, loadingRuns: false });
    } catch (error) {
      console.error('Failed to fetch runs:', error);
      set({ loadingRuns: false });
    }
  },

  fetchBaselines: async () => {
    set({ loadingBaselines: true });
    try {
      const baselines = await api.getBaselines();
      set({ baselines, loadingBaselines: false });
    } catch (error) {
      console.error('Failed to fetch baselines:', error);
      set({ loadingBaselines: false });
    }
  },

  fetchWorkloads: async () => {
    set({ loadingWorkloads: true });
    try {
      const workloads = await api.getWorkloads();
      set({ workloads, loadingWorkloads: false });
    } catch (error) {
      console.error('Failed to fetch workloads:', error);
      set({ loadingWorkloads: false });
    }
  },

  selectRun: (run) => {
    set({ selectedRun: run });
  },

  deleteRun: async (id) => {
    await api.deleteRun(id);
    const { runs, selectedRun } = get();
    set({
      runs: runs.filter((r) => r.id !== id),
      selectedRun: selectedRun?.id === id ? null : selectedRun,
    });
  },

  startBenchmark: async (config) => {
    const result = await api.startBenchmark(config);
    const benchmarkId = result.id || result.benchmark_id || '';
    set({
      activeBenchmark: {
        id: benchmarkId,
        status: 'running',
        progress: 0,
      },
    });
    return benchmarkId;
  },

  setActiveBenchmark: (benchmark) => {
    set({ activeBenchmark: benchmark });
  },

  setWsConnected: (connected) => {
    set({ wsConnected: connected });
  },

  refreshRun: async (id) => {
    try {
      const run = await api.getRun(id);
      const { runs } = get();
      const index = runs.findIndex((r) => r.id === id);
      if (index >= 0) {
        const newRuns = [...runs];
        newRuns[index] = run;
        set({ runs: newRuns });
      } else {
        set({ runs: [run, ...runs] });
      }
    } catch (error) {
      console.error('Failed to refresh run:', error);
    }
  },
}));
