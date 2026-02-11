// Provider Registry - Extensible pattern for adding new cloud providers
// To add a new provider:
// 1. Create a new component file (e.g., VMwareProvider.tsx)
// 2. Add it to the PROVIDERS array below
// 3. The provider will automatically appear in the UI

import React from 'react';
import { CloudProvider, ProviderInfo, ProviderFormProps } from '@/types';

// Import provider info from separate file to avoid circular dependencies
import { PROVIDER_INFO } from './providerInfo';

// Re-export for consumers
export { PROVIDER_INFO };
export type { ProviderFormProps };

// Import provider components AFTER importing PROVIDER_INFO
import AzureProvider from './AzureProvider';
import AWSProvider from './AWSProvider';
import GCPProvider from './GCPProvider';
import KubernetesProvider from './KubernetesProvider';
import SelfManagedProvider from './SelfManagedProvider';

// Provider form components
export const PROVIDER_COMPONENTS: Record<CloudProvider, React.ComponentType<any>> = {
  azure: AzureProvider,
  aws: AWSProvider,
  gcp: GCPProvider,
  kubernetes: KubernetesProvider,
  vmware: () => null, // Placeholder
  local: () => null, // Placeholder
  self_managed: SelfManagedProvider,
};

// Get enabled providers
export const getEnabledProviders = (): ProviderInfo[] => {
  return Object.values(PROVIDER_INFO).filter(p => p.enabled);
};

// Get all providers (for showing "coming soon" badges)
export const getAllProviders = (): ProviderInfo[] => {
  return Object.values(PROVIDER_INFO);
};

export { AzureProvider, AWSProvider, GCPProvider, KubernetesProvider, SelfManagedProvider };
