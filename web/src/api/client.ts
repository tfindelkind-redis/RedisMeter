import axios, { AxiosInstance } from 'axios';
import { BenchmarkRun, Baseline, ComparisonResult, AnalysisResult, Workload, Infrastructure } from '@/types';

const API_BASE = '/api/v1';

class ApiClient {
  private client: AxiosInstance;

  constructor() {
    this.client = axios.create({
      baseURL: API_BASE,
      timeout: 30000,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    // Add response interceptor for error handling
    this.client.interceptors.response.use(
      (response) => response,
      (error) => {
        console.error('API Error:', error.response?.data || error.message);
        return Promise.reject(error);
      }
    );
  }

  // Set API key for authenticated requests
  setApiKey(apiKey: string) {
    this.client.defaults.headers.common['X-API-Key'] = apiKey;
  }

  // Benchmark Runs
  async getRuns(params?: {
    status?: string;
    workload?: string;
    tags?: string[];
    limit?: number;
    offset?: number;
  }): Promise<BenchmarkRun[]> {
    const response = await this.client.get('/runs', { params });
    return response.data?.runs || [];
  }

  async getRun(id: string): Promise<BenchmarkRun> {
    const response = await this.client.get(`/runs/${id}`);
    return response.data;
  }

  async deleteRun(id: string): Promise<void> {
    await this.client.delete(`/runs/${id}`);
  }

  // Baselines
  async getBaselines(): Promise<Baseline[]> {
    const response = await this.client.get('/baselines');
    return response.data?.baselines || [];
  }

  async getBaseline(id: string): Promise<Baseline> {
    const response = await this.client.get(`/baselines/${id}`);
    return response.data;
  }

  async createBaseline(runId: string, name: string, description?: string): Promise<Baseline> {
    const response = await this.client.post('/baselines', {
      run_id: runId,
      name,
      description,
    });
    return response.data;
  }

  async deleteBaseline(id: string): Promise<void> {
    await this.client.delete(`/baselines/${id}`);
  }

  async updateBaseline(id: string, updates: { name?: string; active?: boolean }): Promise<Baseline> {
    const response = await this.client.patch(`/baselines/${id}`, updates);
    return response.data;
  }

  async setActiveBaseline(id: string): Promise<void> {
    await this.client.post(`/baselines/${id}/activate`);
  }

  // Workloads
  async getWorkloads(): Promise<Workload[]> {
    const response = await this.client.get('/workloads');
    return response.data?.workloads || [];
  }

  async getWorkloadsFull(): Promise<any[]> {
    const response = await this.client.get('/workloads?full=true');
    return response.data?.workloads || [];
  }

  async getWorkload(name: string): Promise<any> {
    const response = await this.client.get(`/workloads/${name}`);
    return response.data;
  }

  async createWorkload(workload: any): Promise<any> {
    const response = await this.client.post('/workloads', workload);
    return response.data;
  }

  async updateWorkload(name: string, workload: any): Promise<any> {
    const response = await this.client.put(`/workloads/${name}`, workload);
    return response.data;
  }

  async deleteWorkload(name: string): Promise<void> {
    await this.client.delete(`/workloads/${name}`);
  }

  // Run Profiles
  async getRunProfiles(): Promise<any[]> {
    const response = await this.client.get('/run-profiles');
    return response.data?.run_profiles || [];
  }

  async getRunProfilesFull(): Promise<any[]> {
    const response = await this.client.get('/run-profiles?full=true');
    return response.data?.run_profiles || [];
  }

  async getRunProfile(name: string): Promise<any> {
    const response = await this.client.get(`/run-profiles/${name}`);
    return response.data;
  }

  async createRunProfile(profile: any): Promise<any> {
    const response = await this.client.post('/run-profiles', profile);
    return response.data;
  }

  async updateRunProfile(name: string, profile: any): Promise<any> {
    const response = await this.client.put(`/run-profiles/${name}`, profile);
    return response.data;
  }

  async deleteRunProfile(name: string): Promise<void> {
    await this.client.delete(`/run-profiles/${name}`);
  }

  // Infrastructure Profiles
  async getInfraProfiles(params?: {
    provider?: string;
    tag?: string;
  }): Promise<{ profiles: any[]; count: number }> {
    const response = await this.client.get('/infrastructure-profiles', { params });
    return {
      profiles: response.data?.profiles || [],
      count: response.data?.count || 0,
    };
  }

  async getInfraProfile(id: string): Promise<any> {
    const response = await this.client.get(`/infrastructure-profiles/${id}`);
    return response.data;
  }

  async createInfraProfile(profile: any): Promise<any> {
    const response = await this.client.post('/infrastructure-profiles', profile);
    return response.data;
  }

  async updateInfraProfile(id: string, profile: any): Promise<any> {
    const response = await this.client.put(`/infrastructure-profiles/${id}`, profile);
    return response.data;
  }

  async deleteInfraProfile(id: string): Promise<void> {
    await this.client.delete(`/infrastructure-profiles/${id}`);
  }

  async getInfraProfileStats(): Promise<{
    total_profiles: number;
    by_provider: Record<string, number>;
    most_used?: any;
    recently_created?: any;
    recently_used?: any;
  }> {
    const response = await this.client.get('/infrastructure-profiles/stats');
    return response.data;
  }

  async exportInfraProfiles(): Promise<Blob> {
    const response = await this.client.get('/infrastructure-profiles/export', {
      responseType: 'blob',
    });
    return response.data;
  }

  async importInfraProfiles(file: File, overwrite: boolean = false): Promise<{
    imported: number;
    message: string;
  }> {
    const formData = new FormData();
    formData.append('file', file);
    formData.append('overwrite', String(overwrite));
    const response = await this.client.post('/infrastructure-profiles/import', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    });
    return response.data;
  }

  // Benchmark execution
  async startBenchmark(config: {
    name?: string;
    description?: string;
    target: { host: string; port: number; password?: string; tls?: boolean; cluster?: boolean };
    workload: string;
    duration?: string;
    threads?: number;
    clients?: number;
    tags?: string[];
  }): Promise<{ id: string; benchmark_id?: string }> {
    const response = await this.client.post('/benchmark', config);
    return response.data;
  }

  async getBenchmarkStatus(id: string): Promise<{
    id: string;
    status: string;
    progress: number;
  }> {
    const response = await this.client.get(`/benchmark/${id}`);
    return response.data;
  }

  async cancelBenchmark(id: string): Promise<void> {
    await this.client.delete(`/benchmark/${id}`);
  }

  // Comparison
  async compare(
    runId: string,
    baselineId?: string,
    otherRunId?: string
  ): Promise<ComparisonResult> {
    const params: Record<string, string> = { run_id: runId };
    if (baselineId) params.baseline_id = baselineId;
    if (otherRunId) params.other_run_id = otherRunId;
    
    const response = await this.client.get('/compare', { params });
    return response.data;
  }

  // Analysis
  async analyze(runId: string, analyzers?: string[]): Promise<AnalysisResult> {
    const params: Record<string, unknown> = { run_id: runId };
    if (analyzers?.length) params.analyzers = analyzers.join(',');
    
    const response = await this.client.get('/analyze', { params });
    return response.data;
  }

  // Health check
  async health(): Promise<{
    status: string;
    storage?: { healthy: boolean; message: string };
  }> {
    const response = await this.client.get('/health');
    return response.data;
  }

  // ==========================================
  // Infrastructure Management
  // ==========================================

  // List all infrastructures
  async getInfrastructures(params?: {
    status?: string;
    provider?: string;
    limit?: number;
  }): Promise<Infrastructure[]> {
    const response = await this.client.get('/infrastructures', { params });
    return response.data?.infrastructures || [];
  }

  // Get single infrastructure
  async getInfrastructure(id: string): Promise<Infrastructure> {
    const response = await this.client.get(`/infrastructures/${id}`);
    return response.data;
  }

  // Create new infrastructure
  async createInfrastructure(config: {
    name: string;
    provider: string;
    region: string;
    ttl?: string;
    tags?: Record<string, string>;
    amr?: {
      sku: string;
      modules?: string[];
      high_availability?: boolean;
      clustering_policy?: string;
      eviction_policy?: string;
    };
    runners: {
      count: number;
      instance_type: string;
      spot_instances?: boolean;
      ssh_public_key?: string;
      ssh_user?: string;
    };
  }): Promise<Infrastructure> {
    const response = await this.client.post('/infrastructures', config);
    return response.data;
  }

  // Destroy infrastructure
  async destroyInfrastructure(id: string): Promise<void> {
    await this.client.delete(`/infrastructures/${id}`);
  }

  // Get infrastructure status
  async getInfrastructureStatus(id: string): Promise<{
    id: string;
    status: string;
    progress?: number;
    error?: string;
  }> {
    const response = await this.client.get(`/infrastructures/${id}/status`);
    return response.data;
  }

  // Run benchmark on infrastructure
  async runCloudBenchmark(config: {
    infrastructure_id: string;
    workload?: string;
    run_profile?: string;
    requests?: number;
    clients?: number;
    threads?: number;
    duration?: string;
    pipeline?: number;
    key_pattern?: string;
    data_size?: number;
    ratio?: string;
  }): Promise<{ id: string; benchmark_id?: string; status: string; message?: string }> {
    const response = await this.client.post('/cloud/benchmark', config);
    return response.data;
  }
}

export const api = new ApiClient();
export default api;
