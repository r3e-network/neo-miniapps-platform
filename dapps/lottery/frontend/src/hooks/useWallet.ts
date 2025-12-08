import { create } from 'zustand';
import { persist } from 'zustand/middleware';

// Neo N3 wallet types
interface NeoLineN3 {
  getAccount(): Promise<{ address: string; label?: string }>;
  getNetworks(): Promise<{ networks: string[]; defaultNetwork: string }>;
  getBalance(params: { address: string; contracts?: string[] }): Promise<{ [key: string]: { amount: string; symbol: string } }>;
  invoke(params: InvokeParams): Promise<{ txid: string; nodeUrl?: string }>;
  invokeRead(params: InvokeReadParams): Promise<{ stack: any[] }>;
  signMessage(params: { message: string }): Promise<{ publicKey: string; data: string; salt: string; message: string }>;
}

interface InvokeParams {
  scriptHash: string;
  operation: string;
  args: any[];
  fee?: string;
  broadcastOverride?: boolean;
  signers?: { account: string; scopes: number }[];
}

interface InvokeReadParams {
  scriptHash: string;
  operation: string;
  args: any[];
  signers?: { account: string; scopes: number }[];
}

declare global {
  interface Window {
    NEOLineN3?: {
      Init: () => Promise<NeoLineN3>;
    };
    OneGate?: NeoLineN3;
    neo3Dapi?: NeoLineN3;
  }
}

export type WalletType = 'neoline' | 'onegate' | 'o3' | null;

interface WalletState {
  address: string | null;
  walletType: WalletType;
  balance: string;
  isConnecting: boolean;
  error: string | null;

  connect: (type: WalletType) => Promise<void>;
  disconnect: () => void;
  refreshBalance: () => Promise<void>;
  invoke: (params: InvokeParams) => Promise<string>;
  invokeRead: (params: InvokeReadParams) => Promise<any[]>;
}

const GAS_CONTRACT = '0xd2a4cff31913016155e38e474a2c06d08be276cf';

export const useWallet = create<WalletState>()(
  persist(
    (set, get) => ({
      address: null,
      walletType: null,
      balance: '0',
      isConnecting: false,
      error: null,

      connect: async (type: WalletType) => {
        set({ isConnecting: true, error: null });

        try {
          let wallet: NeoLineN3 | undefined;

          switch (type) {
            case 'neoline':
              if (!window.NEOLineN3) {
                throw new Error('NeoLine extension not installed');
              }
              wallet = await window.NEOLineN3.Init();
              break;

            case 'onegate':
              if (!window.OneGate) {
                throw new Error('OneGate wallet not available');
              }
              wallet = window.OneGate;
              break;

            case 'o3':
              if (!window.neo3Dapi) {
                throw new Error('O3 wallet not available');
              }
              wallet = window.neo3Dapi;
              break;

            default:
              throw new Error('Unknown wallet type');
          }

          const account = await wallet.getAccount();

          // Get GAS balance
          const balances = await wallet.getBalance({
            address: account.address,
            contracts: [GAS_CONTRACT],
          });

          const gasBalance = balances[GAS_CONTRACT]?.amount || '0';

          set({
            address: account.address,
            walletType: type,
            balance: gasBalance,
            isConnecting: false,
          });
        } catch (error: any) {
          set({
            error: error.message || 'Failed to connect wallet',
            isConnecting: false,
          });
          throw error;
        }
      },

      disconnect: () => {
        set({
          address: null,
          walletType: null,
          balance: '0',
          error: null,
        });
      },

      refreshBalance: async () => {
        const { address, walletType } = get();
        if (!address || !walletType) return;

        try {
          let wallet: NeoLineN3 | undefined;

          switch (walletType) {
            case 'neoline':
              wallet = await window.NEOLineN3?.Init();
              break;
            case 'onegate':
              wallet = window.OneGate;
              break;
            case 'o3':
              wallet = window.neo3Dapi;
              break;
          }

          if (!wallet) return;

          const balances = await wallet.getBalance({
            address,
            contracts: [GAS_CONTRACT],
          });

          const gasBalance = balances[GAS_CONTRACT]?.amount || '0';
          set({ balance: gasBalance });
        } catch (error) {
          console.error('Failed to refresh balance:', error);
        }
      },

      invoke: async (params: InvokeParams): Promise<string> => {
        const { address, walletType } = get();
        if (!address || !walletType) {
          throw new Error('Wallet not connected');
        }

        let wallet: NeoLineN3 | undefined;

        switch (walletType) {
          case 'neoline':
            wallet = await window.NEOLineN3?.Init();
            break;
          case 'onegate':
            wallet = window.OneGate;
            break;
          case 'o3':
            wallet = window.neo3Dapi;
            break;
        }

        if (!wallet) {
          throw new Error('Wallet not available');
        }

        const result = await wallet.invoke({
          ...params,
          signers: params.signers || [
            {
              account: address,
              scopes: 1, // CalledByEntry
            },
          ],
        });

        // Refresh balance after transaction
        setTimeout(() => get().refreshBalance(), 5000);

        return result.txid;
      },

      invokeRead: async (params: InvokeReadParams): Promise<any[]> => {
        const { walletType } = get();

        let wallet: NeoLineN3 | undefined;

        switch (walletType) {
          case 'neoline':
            wallet = await window.NEOLineN3?.Init();
            break;
          case 'onegate':
            wallet = window.OneGate;
            break;
          case 'o3':
            wallet = window.neo3Dapi;
            break;
        }

        if (!wallet) {
          throw new Error('Wallet not available');
        }

        const result = await wallet.invokeRead(params);
        return result.stack;
      },
    }),
    {
      name: 'mega-lottery-wallet',
      partialize: (state) => ({
        walletType: state.walletType,
      }),
    }
  )
);

// Helper to format GAS amount
export function formatGas(amount: string | number): string {
  const num = typeof amount === 'string' ? parseFloat(amount) : amount;
  return num.toLocaleString('en-US', {
    minimumFractionDigits: 0,
    maximumFractionDigits: 4,
  });
}

// Helper to parse GAS to smallest unit
export function parseGasToInt(amount: number): string {
  return Math.floor(amount * 1e8).toString();
}
