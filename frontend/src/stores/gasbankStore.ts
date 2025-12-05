import { create } from 'zustand';
import type { GasAccount, GasTransaction, DepositRequest } from '@/types';
import apiClient from '@/api/client';

interface GasBankState {
  account: GasAccount | null;
  transactions: GasTransaction[];
  isLoading: boolean;

  // Actions
  fetchAccount: () => Promise<void>;
  fetchTransactions: (page?: number, pageSize?: number) => Promise<void>;
  deposit: (request: DepositRequest) => Promise<boolean>;
  withdraw: (amount: string, toAddress: string) => Promise<boolean>;
  getDepositAddress: () => Promise<string | null>;
}

export const useGasBankStore = create<GasBankState>((set) => ({
  account: null,
  transactions: [],
  isLoading: false,

  fetchAccount: async () => {
    set({ isLoading: true });
    try {
      const response = await apiClient.get<GasAccount>('/gasbank/account');
      if (response.success && response.data) {
        set({ account: response.data });
      }
    } finally {
      set({ isLoading: false });
    }
  },

  fetchTransactions: async (page = 1, pageSize = 20) => {
    try {
      const response = await apiClient.get<GasTransaction[]>('/gasbank/transactions', {
        page,
        pageSize,
      });
      if (response.success && response.data) {
        set({ transactions: response.data });
      }
    } catch {
      // Ignore errors
    }
  },

  deposit: async (request: DepositRequest) => {
    try {
      const response = await apiClient.post<GasTransaction>('/gasbank/deposit', request);
      if (response.success && response.data) {
        set((state) => ({
          transactions: [response.data!, ...state.transactions],
        }));
        return true;
      }
      return false;
    } catch {
      return false;
    }
  },

  withdraw: async (amount: string, toAddress: string) => {
    try {
      const response = await apiClient.post<GasTransaction>('/gasbank/withdraw', {
        amount,
        toAddress,
      });
      if (response.success && response.data) {
        set((state) => ({
          transactions: [response.data!, ...state.transactions],
        }));
        return true;
      }
      return false;
    } catch {
      return false;
    }
  },

  getDepositAddress: async () => {
    try {
      const response = await apiClient.get<{ address: string }>('/gasbank/deposit-address');
      if (response.success && response.data) {
        return response.data.address;
      }
      return null;
    } catch {
      return null;
    }
  },
}));
