import { useState, useEffect, useCallback } from 'react';
import Neon, { sc, rpc } from '@cityofzion/neon-js';

const RPC_URL = import.meta.env.VITE_RPC_URL || 'http://localhost:50012';
const LOTTERY_CONTRACT = import.meta.env.VITE_LOTTERY_CONTRACT || '';

export interface LotteryRound {
  roundId: number;
  ticketPrice: string;
  startTime: number;
  endTime: number;
  jackpot: string;
  ticketCount: number;
  status: number;
  winningNumbers: number[] | null;
}

export interface LotteryTicket {
  ticketId: number;
  roundId: number;
  numbers: number[];
  purchaseTime: number;
  claimed: boolean;
}

export function useLottery() {
  const [currentRound, setCurrentRound] = useState<LotteryRound | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchCurrentRound = useCallback(async () => {
    try {
      setLoading(true);
      const rpcClient = new rpc.RPCClient(RPC_URL);

      // Get current round ID
      const roundIdResult = await rpcClient.invokeFunction(
        LOTTERY_CONTRACT,
        'getCurrentRoundId',
        []
      );

      if (roundIdResult.state !== 'HALT') {
        throw new Error('Failed to get current round');
      }

      const roundId = parseInt(roundIdResult.stack[0].value as string);

      // Get round details
      const roundResult = await rpcClient.invokeFunction(
        LOTTERY_CONTRACT,
        'getRound',
        [sc.ContractParam.integer(roundId)]
      );

      if (roundResult.state === 'HALT' && roundResult.stack[0]) {
        const data = roundResult.stack[0].value as any[];
        setCurrentRound({
          roundId,
          ticketPrice: (parseInt(data[1]) / 1e8).toFixed(2),
          startTime: parseInt(data[2]),
          endTime: parseInt(data[3]),
          jackpot: (parseInt(data[4]) / 1e8).toFixed(2),
          ticketCount: parseInt(data[5]),
          status: parseInt(data[6]),
          winningNumbers: data[7] ? data[7].map((n: any) => parseInt(n)) : null,
        });
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  }, []);

  const buyTicket = useCallback(async (numbers: number[]): Promise<string> => {
    if (!currentRound) throw new Error('No active round');

    // Build transaction script
    const script = sc.createScript({
      scriptHash: LOTTERY_CONTRACT,
      operation: 'buyTicket',
      args: [
        sc.ContractParam.integer(currentRound.roundId),
        sc.ContractParam.array(...numbers.map(n => sc.ContractParam.integer(n))),
      ],
    });

    // Return script for wallet to sign
    return script.str;
  }, [currentRound]);

  const getTicket = useCallback(async (roundId: number, ticketId: number): Promise<LotteryTicket | null> => {
    try {
      const rpcClient = new rpc.RPCClient(RPC_URL);
      const result = await rpcClient.invokeFunction(
        LOTTERY_CONTRACT,
        'getTicket',
        [
          sc.ContractParam.integer(roundId),
          sc.ContractParam.integer(ticketId),
        ]
      );

      if (result.state === 'HALT' && result.stack[0]) {
        const data = result.stack[0].value as any[];
        return {
          ticketId: parseInt(data[0]),
          roundId: parseInt(data[1]),
          numbers: data[3].map((n: any) => parseInt(n)),
          purchaseTime: parseInt(data[4]),
          claimed: data[5] === true,
        };
      }
      return null;
    } catch {
      return null;
    }
  }, []);

  const claimPrize = useCallback(async (roundId: number, ticketId: number): Promise<string> => {
    const script = sc.createScript({
      scriptHash: LOTTERY_CONTRACT,
      operation: 'claimPrize',
      args: [
        sc.ContractParam.integer(roundId),
        sc.ContractParam.integer(ticketId),
      ],
    });
    return script.str;
  }, []);

  useEffect(() => {
    fetchCurrentRound();
    const interval = setInterval(fetchCurrentRound, 30000); // Refresh every 30s
    return () => clearInterval(interval);
  }, [fetchCurrentRound]);

  return {
    currentRound,
    loading,
    error,
    buyTicket,
    getTicket,
    claimPrize,
    refresh: fetchCurrentRound,
  };
}
