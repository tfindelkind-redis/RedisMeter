// Provider information - separated to avoid circular dependencies
import { CloudProvider, ProviderInfo } from '@/types';

// Provider metadata for the UI
export const PROVIDER_INFO: Record<CloudProvider, ProviderInfo> = {
  azure: {
    id: 'azure',
    name: 'Azure',
    icon: 'azure',
    description: 'Azure Managed Redis (AMR) with runner VMs',
    enabled: true,
    regions: [
      { id: 'eastus', name: 'East US', available: true },
      { id: 'eastus2', name: 'East US 2', available: true },
      { id: 'westus', name: 'West US', available: true },
      { id: 'westus2', name: 'West US 2', available: true },
      { id: 'westus3', name: 'West US 3', available: true },
      { id: 'centralus', name: 'Central US', available: true },
      { id: 'northeurope', name: 'North Europe', available: true },
      { id: 'westeurope', name: 'West Europe', available: true },
      { id: 'uksouth', name: 'UK South', available: true },
      { id: 'southeastasia', name: 'Southeast Asia', available: true },
      { id: 'australiaeast', name: 'Australia East', available: true },
    ],
    instance_types: [
      { id: 'Standard_D2s_v3', name: 'D2s v3', vcpus: 2, memory_gb: 8, category: 'general' },
      { id: 'Standard_D4s_v3', name: 'D4s v3', vcpus: 4, memory_gb: 16, category: 'general' },
      { id: 'Standard_D8s_v3', name: 'D8s v3', vcpus: 8, memory_gb: 32, category: 'general' },
      { id: 'Standard_D16s_v3', name: 'D16s v3', vcpus: 16, memory_gb: 64, category: 'general' },
      { id: 'Standard_F4s_v2', name: 'F4s v2', vcpus: 4, memory_gb: 8, category: 'compute' },
      { id: 'Standard_F8s_v2', name: 'F8s v2', vcpus: 8, memory_gb: 16, category: 'compute' },
      { id: 'Standard_F16s_v2', name: 'F16s v2', vcpus: 16, memory_gb: 32, category: 'compute' },
      { id: 'Standard_E4s_v3', name: 'E4s v3', vcpus: 4, memory_gb: 32, category: 'memory' },
      { id: 'Standard_E8s_v3', name: 'E8s v3', vcpus: 8, memory_gb: 64, category: 'memory' },
    ],
    redis_skus: [
      { id: 'Balanced_B0', name: 'Balanced B0', tier: 'Balanced', description: '250MB, Basic', memory_gb: 0.25, throughput: '1K ops/s' },
      { id: 'Balanced_B1', name: 'Balanced B1', tier: 'Balanced', description: '1GB, Standard', memory_gb: 1, throughput: '10K ops/s' },
      { id: 'Balanced_B3', name: 'Balanced B3', tier: 'Balanced', description: '3GB, Standard', memory_gb: 3, throughput: '30K ops/s' },
      { id: 'Balanced_B5', name: 'Balanced B5', tier: 'Balanced', description: '6GB, Standard', memory_gb: 6, throughput: '60K ops/s' },
      { id: 'Balanced_B10', name: 'Balanced B10', tier: 'Balanced', description: '12GB, Standard', memory_gb: 12, throughput: '120K ops/s' },
      { id: 'Balanced_B20', name: 'Balanced B20', tier: 'Balanced', description: '24GB, Standard', memory_gb: 24, throughput: '240K ops/s' },
      { id: 'Balanced_B50', name: 'Balanced B50', tier: 'Balanced', description: '50GB, Standard', memory_gb: 50, throughput: '500K ops/s' },
      { id: 'Balanced_B100', name: 'Balanced B100', tier: 'Balanced', description: '100GB, Standard', memory_gb: 100, throughput: '1M ops/s' },
      { id: 'MemoryOptimized_M10', name: 'Memory M10', tier: 'Memory Optimized', description: '12GB Memory Optimized', memory_gb: 12 },
      { id: 'MemoryOptimized_M20', name: 'Memory M20', tier: 'Memory Optimized', description: '24GB Memory Optimized', memory_gb: 24 },
      { id: 'MemoryOptimized_M50', name: 'Memory M50', tier: 'Memory Optimized', description: '50GB Memory Optimized', memory_gb: 50 },
      { id: 'MemoryOptimized_M100', name: 'Memory M100', tier: 'Memory Optimized', description: '100GB Memory Optimized', memory_gb: 100 },
      { id: 'ComputeOptimized_X3', name: 'Compute X3', tier: 'Compute Optimized', description: '3GB Compute Optimized', memory_gb: 3 },
      { id: 'ComputeOptimized_X5', name: 'Compute X5', tier: 'Compute Optimized', description: '6GB Compute Optimized', memory_gb: 6 },
      { id: 'ComputeOptimized_X10', name: 'Compute X10', tier: 'Compute Optimized', description: '12GB Compute Optimized', memory_gb: 12 },
      { id: 'FlashOptimized_A250', name: 'Flash A250', tier: 'Flash Optimized', description: '250GB Flash', memory_gb: 250 },
      { id: 'FlashOptimized_A500', name: 'Flash A500', tier: 'Flash Optimized', description: '500GB Flash', memory_gb: 500 },
      { id: 'FlashOptimized_A700', name: 'Flash A700', tier: 'Flash Optimized', description: '700GB Flash', memory_gb: 700 },
      { id: 'FlashOptimized_A1000', name: 'Flash A1000', tier: 'Flash Optimized', description: '1TB Flash', memory_gb: 1000 },
    ],
  },
  aws: {
    id: 'aws',
    name: 'AWS',
    icon: 'aws',
    description: 'Amazon ElastiCache with EC2 runners',
    enabled: false, // Coming soon
    regions: [
      { id: 'us-east-1', name: 'US East (N. Virginia)', available: true },
      { id: 'us-east-2', name: 'US East (Ohio)', available: true },
      { id: 'us-west-1', name: 'US West (N. California)', available: true },
      { id: 'us-west-2', name: 'US West (Oregon)', available: true },
      { id: 'eu-west-1', name: 'EU (Ireland)', available: true },
      { id: 'eu-central-1', name: 'EU (Frankfurt)', available: true },
      { id: 'ap-southeast-1', name: 'Asia Pacific (Singapore)', available: true },
    ],
    instance_types: [
      { id: 't3.medium', name: 't3.medium', vcpus: 2, memory_gb: 4, category: 'general' },
      { id: 't3.large', name: 't3.large', vcpus: 2, memory_gb: 8, category: 'general' },
      { id: 'c5.large', name: 'c5.large', vcpus: 2, memory_gb: 4, category: 'compute' },
      { id: 'c5.xlarge', name: 'c5.xlarge', vcpus: 4, memory_gb: 8, category: 'compute' },
    ],
  },
  gcp: {
    id: 'gcp',
    name: 'Google Cloud',
    icon: 'gcp',
    description: 'Memorystore for Redis with Compute Engine runners',
    enabled: false, // Coming soon
    regions: [
      { id: 'us-central1', name: 'US Central (Iowa)', available: true },
      { id: 'us-east1', name: 'US East (S. Carolina)', available: true },
      { id: 'europe-west1', name: 'Europe West (Belgium)', available: true },
      { id: 'asia-east1', name: 'Asia East (Taiwan)', available: true },
    ],
    instance_types: [
      { id: 'n1-standard-2', name: 'n1-standard-2', vcpus: 2, memory_gb: 7.5, category: 'general' },
      { id: 'n1-standard-4', name: 'n1-standard-4', vcpus: 4, memory_gb: 15, category: 'general' },
      { id: 'c2-standard-4', name: 'c2-standard-4', vcpus: 4, memory_gb: 16, category: 'compute' },
    ],
  },
  kubernetes: {
    id: 'kubernetes',
    name: 'Kubernetes',
    icon: 'kubernetes',
    description: 'Redis on Kubernetes with pod runners',
    enabled: false, // Coming soon
    regions: [], // N/A for K8s
    instance_types: [], // N/A for K8s
  },
  vmware: {
    id: 'vmware',
    name: 'VMware',
    icon: 'vmware',
    description: 'VMware vSphere VMs for on-premises benchmarking',
    enabled: false, // Coming soon
    regions: [],
    instance_types: [],
  },
  local: {
    id: 'local',
    name: 'Local',
    icon: 'desktop',
    description: 'Run benchmarks on local machine',
    enabled: true,
    regions: [{ id: 'local', name: 'Local Machine', available: true }],
    instance_types: [],
  },
};
