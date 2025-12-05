import { create } from 'zustand';
import type {
  ServiceConfig,
  ServiceType,
  ServiceStats,
  DataFeed,
  PriceFeed,
  AutomationTask,
  VRFRequest,
} from '@/types';
import apiClient from '@/api/client';

interface ServicesState {
  configs: ServiceConfig[];
  stats: Record<ServiceType, ServiceStats | null>;
  dataFeeds: DataFeed[];
  priceFeeds: PriceFeed[];
  automationTasks: AutomationTask[];
  vrfRequests: VRFRequest[];
  isLoading: boolean;

  // Actions
  fetchConfigs: () => Promise<void>;
  fetchStats: (serviceType: ServiceType) => Promise<void>;
  createConfig: (config: Partial<ServiceConfig>) => Promise<boolean>;
  updateConfig: (id: string, updates: Partial<ServiceConfig>) => Promise<boolean>;
  deleteConfig: (id: string) => Promise<boolean>;
  toggleService: (id: string, enabled: boolean) => Promise<boolean>;

  // Oracle
  fetchDataFeeds: () => Promise<void>;
  createDataFeed: (feed: Partial<DataFeed>) => Promise<boolean>;
  updateDataFeed: (id: string, updates: Partial<DataFeed>) => Promise<boolean>;
  deleteDataFeed: (id: string) => Promise<boolean>;

  // DataFeeds
  fetchPriceFeeds: () => Promise<void>;
  createPriceFeed: (feed: Partial<PriceFeed>) => Promise<boolean>;
  updatePriceFeed: (id: string, updates: Partial<PriceFeed>) => Promise<boolean>;

  // Automation
  fetchAutomationTasks: () => Promise<void>;
  createAutomationTask: (task: Partial<AutomationTask>) => Promise<boolean>;
  updateAutomationTask: (id: string, updates: Partial<AutomationTask>) => Promise<boolean>;
  deleteAutomationTask: (id: string) => Promise<boolean>;
  executeTask: (id: string) => Promise<boolean>;

  // VRF
  fetchVRFRequests: () => Promise<void>;
  requestRandomness: (seed: string, callbackContract?: string) => Promise<string | null>;
}

export const useServicesStore = create<ServicesState>((set, get) => ({
  configs: [],
  stats: {} as Record<ServiceType, ServiceStats | null>,
  dataFeeds: [],
  priceFeeds: [],
  automationTasks: [],
  vrfRequests: [],
  isLoading: false,

  fetchConfigs: async () => {
    set({ isLoading: true });
    try {
      const response = await apiClient.get<ServiceConfig[]>('/services/configs');
      if (response.success && response.data) {
        set({ configs: response.data });
      }
    } finally {
      set({ isLoading: false });
    }
  },

  fetchStats: async (serviceType: ServiceType) => {
    try {
      const response = await apiClient.get<ServiceStats>(`/services/${serviceType}/stats`);
      if (response.success && response.data) {
        set((state) => ({
          stats: { ...state.stats, [serviceType]: response.data },
        }));
      }
    } catch {
      // Ignore errors
    }
  },

  createConfig: async (config: Partial<ServiceConfig>) => {
    try {
      const response = await apiClient.post<ServiceConfig>('/services/configs', config);
      if (response.success && response.data) {
        set((state) => ({
          configs: [...state.configs, response.data!],
        }));
        return true;
      }
      return false;
    } catch {
      return false;
    }
  },

  updateConfig: async (id: string, updates: Partial<ServiceConfig>) => {
    try {
      const response = await apiClient.patch<ServiceConfig>(`/services/configs/${id}`, updates);
      if (response.success && response.data) {
        set((state) => ({
          configs: state.configs.map((c) => (c.id === id ? response.data! : c)),
        }));
        return true;
      }
      return false;
    } catch {
      return false;
    }
  },

  deleteConfig: async (id: string) => {
    try {
      const response = await apiClient.delete(`/services/configs/${id}`);
      if (response.success) {
        set((state) => ({
          configs: state.configs.filter((c) => c.id !== id),
        }));
        return true;
      }
      return false;
    } catch {
      return false;
    }
  },

  toggleService: async (id: string, enabled: boolean) => {
    return get().updateConfig(id, { enabled });
  },

  // Oracle Data Feeds
  fetchDataFeeds: async () => {
    try {
      const response = await apiClient.get<DataFeed[]>('/services/oracle/feeds');
      if (response.success && response.data) {
        set({ dataFeeds: response.data });
      }
    } catch {
      // Ignore errors
    }
  },

  createDataFeed: async (feed: Partial<DataFeed>) => {
    try {
      const response = await apiClient.post<DataFeed>('/services/oracle/feeds', feed);
      if (response.success && response.data) {
        set((state) => ({
          dataFeeds: [...state.dataFeeds, response.data!],
        }));
        return true;
      }
      return false;
    } catch {
      return false;
    }
  },

  updateDataFeed: async (id: string, updates: Partial<DataFeed>) => {
    try {
      const response = await apiClient.patch<DataFeed>(`/services/oracle/feeds/${id}`, updates);
      if (response.success && response.data) {
        set((state) => ({
          dataFeeds: state.dataFeeds.map((f) => (f.id === id ? response.data! : f)),
        }));
        return true;
      }
      return false;
    } catch {
      return false;
    }
  },

  deleteDataFeed: async (id: string) => {
    try {
      const response = await apiClient.delete(`/services/oracle/feeds/${id}`);
      if (response.success) {
        set((state) => ({
          dataFeeds: state.dataFeeds.filter((f) => f.id !== id),
        }));
        return true;
      }
      return false;
    } catch {
      return false;
    }
  },

  // Price Feeds
  fetchPriceFeeds: async () => {
    try {
      const response = await apiClient.get<PriceFeed[]>('/services/datafeeds/feeds');
      if (response.success && response.data) {
        set({ priceFeeds: response.data });
      }
    } catch {
      // Ignore errors
    }
  },

  createPriceFeed: async (feed: Partial<PriceFeed>) => {
    try {
      const response = await apiClient.post<PriceFeed>('/services/datafeeds/feeds', feed);
      if (response.success && response.data) {
        set((state) => ({
          priceFeeds: [...state.priceFeeds, response.data!],
        }));
        return true;
      }
      return false;
    } catch {
      return false;
    }
  },

  updatePriceFeed: async (id: string, updates: Partial<PriceFeed>) => {
    try {
      const response = await apiClient.patch<PriceFeed>(`/services/datafeeds/feeds/${id}`, updates);
      if (response.success && response.data) {
        set((state) => ({
          priceFeeds: state.priceFeeds.map((f) => (f.id === id ? response.data! : f)),
        }));
        return true;
      }
      return false;
    } catch {
      return false;
    }
  },

  // Automation Tasks
  fetchAutomationTasks: async () => {
    try {
      const response = await apiClient.get<AutomationTask[]>('/services/automation/tasks');
      if (response.success && response.data) {
        set({ automationTasks: response.data });
      }
    } catch {
      // Ignore errors
    }
  },

  createAutomationTask: async (task: Partial<AutomationTask>) => {
    try {
      const response = await apiClient.post<AutomationTask>('/services/automation/tasks', task);
      if (response.success && response.data) {
        set((state) => ({
          automationTasks: [...state.automationTasks, response.data!],
        }));
        return true;
      }
      return false;
    } catch {
      return false;
    }
  },

  updateAutomationTask: async (id: string, updates: Partial<AutomationTask>) => {
    try {
      const response = await apiClient.patch<AutomationTask>(
        `/services/automation/tasks/${id}`,
        updates
      );
      if (response.success && response.data) {
        set((state) => ({
          automationTasks: state.automationTasks.map((t) => (t.id === id ? response.data! : t)),
        }));
        return true;
      }
      return false;
    } catch {
      return false;
    }
  },

  deleteAutomationTask: async (id: string) => {
    try {
      const response = await apiClient.delete(`/services/automation/tasks/${id}`);
      if (response.success) {
        set((state) => ({
          automationTasks: state.automationTasks.filter((t) => t.id !== id),
        }));
        return true;
      }
      return false;
    } catch {
      return false;
    }
  },

  executeTask: async (id: string) => {
    try {
      const response = await apiClient.post(`/services/automation/tasks/${id}/execute`);
      return response.success;
    } catch {
      return false;
    }
  },

  // VRF
  fetchVRFRequests: async () => {
    try {
      const response = await apiClient.get<VRFRequest[]>('/services/vrf/requests');
      if (response.success && response.data) {
        set({ vrfRequests: response.data });
      }
    } catch {
      // Ignore errors
    }
  },

  requestRandomness: async (seed: string, callbackContract?: string) => {
    try {
      const response = await apiClient.post<{ requestId: string }>('/services/vrf/request', {
        seed,
        callbackContract,
      });
      if (response.success && response.data) {
        get().fetchVRFRequests();
        return response.data.requestId;
      }
      return null;
    } catch {
      return null;
    }
  },
}));
