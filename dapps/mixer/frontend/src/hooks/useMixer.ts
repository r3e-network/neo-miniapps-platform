import { useState, useEffect, useCallback } from 'react';
import Neon, { sc, rpc } from '@cityofzion/neon-js';

const RPC_URL = import.meta.env.VITE_RPC_URL || 'http://localhost:50012';
const MIXER_CONTRACT = import.meta.env.VITE_MIXER_CONTRACT || '';

export interface MixerStats {
  totalDeposits: number;
  totalWithdrawals: number;
  totalVolume: string;
  pool1Deposits: number;
  pool2Deposits: number;
  pool3Deposits: number;
}

export interface MixerPool {
  poolId: number;
  denomination: string;
  totalDeposits: number;
  pendingCount: number;
  active: boolean;
}

export function useMixer() {
  const [stats, setStats] = useState<MixerStats | null>(null);
  const [pools, setPools] = useState<MixerPool[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchStats = useCallback(async () => {
    try {
      setLoading(true);
      const rpcClient = new rpc.RPCClient(RPC_URL);

      const result = await rpcClient.invokeFunction(MIXER_CONTRACT, 'getStats', []);

      if (result.state === 'HALT' && result.stack[0]) {
        const data = result.stack[0].value as any[];
        setStats({
          totalDeposits: parseInt(data[0]) || 0,
          totalWithdrawals: parseInt(data[1]) || 0,
          totalVolume: (parseInt(data[2]) / 1e8).toFixed(2),
          pool1Deposits: parseInt(data[3]) || 0,
          pool2Deposits: parseInt(data[4]) || 0,
          pool3Deposits: parseInt(data[5]) || 0,
        });
      }
    } catch (err) {
      console.error('Failed to fetch stats:', err);
    } finally {
      setLoading(false);
    }
  }, []);

  const deposit = useCallback(async (poolId: number, commitment: string): Promise<string> => {
    const script = sc.createScript({
      scriptHash: MIXER_CONTRACT,
      operation: 'deposit',
      args: [
        sc.ContractParam.integer(poolId),
        sc.ContractParam.byteArray(commitment),
      ],
    });
    return script.str;
  }, []);

  const withdraw = useCallback(async (
    nullifier: string,
    commitment: string,
    recipient: string,
    proof: string
  ): Promise<string> => {
    const script = sc.createScript({
      scriptHash: MIXER_CONTRACT,
      operation: 'requestWithdrawal',
      args: [
        sc.ContractParam.byteArray(nullifier),
        sc.ContractParam.byteArray(commitment),
        sc.ContractParam.hash160(recipient),
        sc.ContractParam.byteArray(proof),
      ],
    });
    return script.str;
  }, []);

  const completeWithdrawal = useCallback(async (nullifier: string): Promise<string> => {
    const script = sc.createScript({
      scriptHash: MIXER_CONTRACT,
      operation: 'completeWithdrawal',
      args: [sc.ContractParam.byteArray(nullifier)],
    });
    return script.str;
  }, []);

  useEffect(() => {
    fetchStats();
    const interval = setInterval(fetchStats, 60000);
    return () => clearInterval(interval);
  }, [fetchStats]);

  return {
    stats,
    pools,
    loading,
    deposit,
    withdraw,
    completeWithdrawal,
    refresh: fetchStats,
  };
}
