import { create } from 'zustand';
import type { APIKey, Permission } from '@/types';
import apiClient from '@/api/client';

interface CreateAPIKeyRequest {
  name: string;
  permissions: Permission[];
  expiresAt?: string;
}

interface CreateAPIKeyResponse {
  apiKey: APIKey;
  secretKey: string; // Only returned once on creation
}

interface APIKeysState {
  apiKeys: APIKey[];
  isLoading: boolean;

  // Actions
  fetchAPIKeys: () => Promise<void>;
  createAPIKey: (request: CreateAPIKeyRequest) => Promise<CreateAPIKeyResponse | null>;
  updateAPIKey: (id: string, updates: Partial<CreateAPIKeyRequest>) => Promise<boolean>;
  deleteAPIKey: (id: string) => Promise<boolean>;
  revokeAPIKey: (id: string) => Promise<boolean>;
}

export const useAPIKeysStore = create<APIKeysState>((set) => ({
  apiKeys: [],
  isLoading: false,

  fetchAPIKeys: async () => {
    set({ isLoading: true });
    try {
      const response = await apiClient.get<APIKey[]>('/apikeys');
      if (response.success && response.data) {
        set({ apiKeys: response.data });
      }
    } finally {
      set({ isLoading: false });
    }
  },

  createAPIKey: async (request: CreateAPIKeyRequest) => {
    try {
      const response = await apiClient.post<CreateAPIKeyResponse>('/apikeys', request);
      if (response.success && response.data) {
        set((state) => ({
          apiKeys: [...state.apiKeys, response.data!.apiKey],
        }));
        return response.data;
      }
      return null;
    } catch {
      return null;
    }
  },

  updateAPIKey: async (id: string, updates: Partial<CreateAPIKeyRequest>) => {
    try {
      const response = await apiClient.patch<APIKey>(`/apikeys/${id}`, updates);
      if (response.success && response.data) {
        set((state) => ({
          apiKeys: state.apiKeys.map((k) => (k.id === id ? response.data! : k)),
        }));
        return true;
      }
      return false;
    } catch {
      return false;
    }
  },

  deleteAPIKey: async (id: string) => {
    try {
      const response = await apiClient.delete(`/apikeys/${id}`);
      if (response.success) {
        set((state) => ({
          apiKeys: state.apiKeys.filter((k) => k.id !== id),
        }));
        return true;
      }
      return false;
    } catch {
      return false;
    }
  },

  revokeAPIKey: async (id: string) => {
    try {
      const response = await apiClient.post(`/apikeys/${id}/revoke`);
      if (response.success) {
        set((state) => ({
          apiKeys: state.apiKeys.filter((k) => k.id !== id),
        }));
        return true;
      }
      return false;
    } catch {
      return false;
    }
  },
}));
