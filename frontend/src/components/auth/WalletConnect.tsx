import React, { useState, useCallback } from 'react';
import { wallet, rpc, sc } from '@cityofzion/neon-js';
import { FiLink, FiCheck, FiX } from 'react-icons/fi';
import { useAuthStore } from '@/stores/authStore';
import { Button } from '@/components/common';
import toast from 'react-hot-toast';

const RPC_URL = import.meta.env.VITE_RPC_URL || 'https://testnet1.neo.coz.io:443';
const GAS_CONTRACT = '0xd2a4cff31913016155e38e474a2c06d08be276cf';

interface WalletConnectProps {
  onConnect?: (address: string) => void;
  showLinkOption?: boolean;
}

export function WalletConnect({ onConnect, showLinkOption = false }: WalletConnectProps) {
  const { wallet: walletState, connectWallet, disconnectWallet, linkWallet, loginWithWallet } = useAuthStore();
  const [isConnecting, setIsConnecting] = useState(false);
  const [isLinking, setIsLinking] = useState(false);

  const fetchBalance = useCallback(async (address: string): Promise<string> => {
    try {
      const rpcClient = new rpc.RPCClient(RPC_URL);
      const result = await rpcClient.invokeFunction(
        GAS_CONTRACT,
        'balanceOf',
        [sc.ContractParam.hash160(address)]
      );

      if (result.state === 'HALT' && result.stack[0]) {
        const balance = parseInt(result.stack[0].value as string) / 1e8;
        return balance.toFixed(4);
      }
      return '0';
    } catch (error) {
      console.error('Failed to fetch balance:', error);
      return '0';
    }
  }, []);

  const handleConnect = async () => {
    setIsConnecting(true);
    try {
      // Check for NeoLine wallet
      if (typeof window !== 'undefined' && (window as any).NEOLine) {
        const neoline = new (window as any).NEOLine.Init();
        const account = await neoline.getAccount();
        const balance = await fetchBalance(account.address);

        connectWallet(account.address, balance);
        onConnect?.(account.address);
        toast.success('Wallet connected successfully!');
      }
      // Check for O3 wallet
      else if (typeof window !== 'undefined' && (window as any).neo3Dapi) {
        const neo3 = (window as any).neo3Dapi;
        const account = await neo3.getAccount();
        const balance = await fetchBalance(account.address);

        connectWallet(account.address, balance);
        onConnect?.(account.address);
        toast.success('Wallet connected successfully!');
      }
      // Fallback: Create demo account for testing
      else {
        const demoAccount = new wallet.Account();
        const balance = await fetchBalance(demoAccount.address);

        connectWallet(demoAccount.address, balance);
        onConnect?.(demoAccount.address);
        toast.success('Demo wallet connected (install NeoLine or O3 for production)');
      }
    } catch (error) {
      console.error('Failed to connect wallet:', error);
      toast.error('Failed to connect wallet. Please try again.');
    } finally {
      setIsConnecting(false);
    }
  };

  const handleDisconnect = () => {
    disconnectWallet();
    toast.success('Wallet disconnected');
  };

  const handleLinkWallet = async () => {
    if (!walletState.address) return;

    setIsLinking(true);
    try {
      // In production, this would sign a message to prove ownership
      const message = `Link wallet ${walletState.address} to Service Layer account`;
      const signature = 'demo-signature'; // Would be actual signature from wallet

      const success = await linkWallet(walletState.address, signature);
      if (success) {
        toast.success('Wallet linked to your account!');
      } else {
        toast.error('Failed to link wallet');
      }
    } catch (error) {
      toast.error('Failed to link wallet');
    } finally {
      setIsLinking(false);
    }
  };

  const handleWalletLogin = async () => {
    if (!walletState.address) {
      await handleConnect();
      return;
    }

    setIsConnecting(true);
    try {
      const message = `Sign in to Service Layer with ${walletState.address}`;
      const signature = 'demo-signature'; // Would be actual signature

      const success = await loginWithWallet(walletState.address, signature);
      if (success) {
        toast.success('Signed in with wallet!');
      } else {
        toast.error('Failed to sign in with wallet');
      }
    } catch (error) {
      toast.error('Failed to sign in with wallet');
    } finally {
      setIsConnecting(false);
    }
  };

  if (walletState.connected) {
    return (
      <div className="space-y-4">
        <div className="flex items-center justify-between p-4 bg-green-50 rounded-lg border border-green-200">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 bg-green-100 rounded-full flex items-center justify-center">
              <FiCheck className="w-5 h-5 text-green-600" />
            </div>
            <div>
              <p className="text-sm font-medium text-green-800">Wallet Connected</p>
              <p className="text-xs font-mono text-green-600">
                {walletState.address?.slice(0, 10)}...{walletState.address?.slice(-8)}
              </p>
            </div>
          </div>
          <div className="text-right">
            <p className="text-lg font-semibold text-green-800">{walletState.balance}</p>
            <p className="text-xs text-green-600">GAS</p>
          </div>
        </div>

        <div className="flex gap-2">
          {showLinkOption && (
            <Button
              variant="secondary"
              onClick={handleLinkWallet}
              isLoading={isLinking}
              leftIcon={<FiLink className="w-4 h-4" />}
              className="flex-1"
            >
              Link to Account
            </Button>
          )}
          <Button
            variant="ghost"
            onClick={handleDisconnect}
            leftIcon={<FiX className="w-4 h-4" />}
            className={showLinkOption ? '' : 'flex-1'}
          >
            Disconnect
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      <Button
        variant="secondary"
        onClick={handleConnect}
        isLoading={isConnecting}
        leftIcon={<FiLink className="w-4 h-4" />}
        className="w-full"
        size="lg"
      >
        Connect Neo N3 Wallet
      </Button>

      <p className="text-xs text-center text-surface-500">
        Supports NeoLine, O3, and WalletConnect
      </p>
    </div>
  );
}
