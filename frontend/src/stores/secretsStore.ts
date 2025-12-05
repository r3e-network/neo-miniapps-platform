import { create } from 'zustand';
import type { Secret } from '@/types';
import apiClient from '@/api/client';

interface SecretsState {
  secrets: Secret[];
  isLoading: boolean;

  // Actions
  fetchSecrets: () => Promise<void>;
  createSecret: (name: string, value: string, tags?: string[]) => Promise<boolean>;
  updateSecret: (name: string, value: string) => Promise<boolean>;
  deleteSecret: (name: string) => Promise<boolean>;
  getSecretValue: (name: string, version?: number) => Promise<string | null>;
}

export const useSecretsStore = create<SecretsState>((set) => ({
  secrets: [],
  isLoading: false,

  fetchSecrets: async () => {
    set({ isLoading: true });
    try {
      const response = await apiClient.get<Secret[]>('/secrets');
      if (response.success && response.data) {
        set({ secrets: response.data });
      }
    } finally {
      set({ isLoading: false });
    }
  },

  createSecret: async (name: string, value: string, tags?: string[]) => {
    try {
      const response = await apiClient.post<Secret>('/secrets', {
        name,
        value,
        tags,
      });
      if (response.success && response.data) {
        set((state) => ({
          secrets: [...state.secrets, response.data!],
        }));
        return true;
      }
      return false;
    } catch {
      return false;
    }
  },

  updateSecret: async (name: string, value: string) => {
    try {
      const response = await apiClient.put<Secret>(`/secrets/${name}`, { value });
      if (response.success && response.data) {
        set((state) => ({
          secrets: state.secrets.map((s) =>
            s.name === name ? { ...s, version: response.data!.version } : s
          ),
        }));
        return true;
      }
      return false;
    } catch {
      return false;
    }
  },

  deleteSecret: async (name: string) => {
    try {
      const response = await apiClient.delete(`/secrets/${name}`);
      if (response.success) {
        set((state) => ({
          secrets: state.secrets.filter((s) => s.name !== name),
        }));
        return true;
      }
      return false;
    } catch {
      return false;
    }
  },

  getSecretValue: async (name: string, version?: number) => {
    try {
      const url = version ? `/secrets/${name}?version=${version}` : `/secrets/${name}`;
      const response = await apiClient.get<{ value: string }>(url);
      if (response.success && response.data) {
        return response.data.value;
      }
      return null;
    } catch {
      return null;
    }
  },
}));
