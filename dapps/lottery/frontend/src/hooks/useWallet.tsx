import React, { createContext, useContext, useState, useCallback, ReactNode } from 'react';
import Neon, { wallet, sc, rpc } from '@cityofzion/neon-js';

interface WalletState {
  connected: boolean;
  address: string | null;
  balance: string;
  connect: () => Promise<void>;
  disconnect: () => void;
  signAndInvoke: (script: string) => Promise<string>;
}

const WalletContext = createContext<WalletState | null>(null);

const RPC_URL = import.meta.env.VITE_RPC_URL || 'http://localhost:50012';
const LOTTERY_CONTRACT = import.meta.env.VITE_LOTTERY_CONTRACT || '';

export function WalletProvider({ children }: { children: ReactNode }) {
  const [connected, setConnected] = useState(false);
  const [address, setAddress] = useState<string | null>(null);
  const [balance, setBalance] = useState('0');

  const connect = useCallback(async () => {
    try {
      // For demo, create a new wallet or use WalletConnect
      // In production, integrate with NeoLine, O3, or WalletConnect
      const account = new wallet.Account();
      setAddress(account.address);
      setConnected(true);

      // Fetch balance
      const rpcClient = new rpc.RPCClient(RPC_URL);
      const gasBalance = await rpcClient.invokeFunction(
        Neon.CONST.NATIVE_CONTRACT_HASH.GasToken,
        'balanceOf',
        [sc.ContractParam.hash160(account.address)]
      );

      if (gasBalance.state === 'HALT' && gasBalance.stack[0]) {
        const bal = parseInt(gasBalance.stack[0].value as string) / 1e8;
        setBalance(bal.toFixed(4));
      }
    } catch (error) {
      console.error('Failed to connect wallet:', error);
    }
  }, []);

  const disconnect = useCallback(() => {
    setConnected(false);
    setAddress(null);
    setBalance('0');
  }, []);

  const signAndInvoke = useCallback(async (script: string): Promise<string> => {
    if (!connected || !address) {
      throw new Error('Wallet not connected');
    }
    // In production, use WalletConnect or browser extension
    return 'tx-hash-placeholder';
  }, [connected, address]);

  return (
    <WalletContext.Provider value={{ connected, address, balance, connect, disconnect, signAndInvoke }}>
      {children}
    </WalletContext.Provider>
  );
}

export function useWallet() {
  const context = useContext(WalletContext);
  if (!context) {
    throw new Error('useWallet must be used within WalletProvider');
  }
  return context;
}
