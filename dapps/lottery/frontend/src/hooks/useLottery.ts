import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useWallet, parseGasToInt } from './useWallet';

// Contract configuration - update with deployed contract hash
const LOTTERY_CONTRACT = import.meta.env.VITE_LOTTERY_CONTRACT || '0x0000000000000000000000000000000000000000';
const TICKET_PRICE = 2; // 2 GAS

export interface LotteryInfo {
  currentDrawId: number;
  nextDrawTime: number;
  currentPool: string;
  ticketCount: number;
  jackpot: string;
  ticketPrice: string;
  isLocked: boolean;
  isPaused: boolean;
}

export interface Draw {
  drawId: number;
  startTime: number;
  drawTime: number;
  winningNumbers: number[];
  winningMega: number;
  totalPool: string;
  ticketCount: number;
  completed: boolean;
}

export interface Ticket {
  ticketId: number;
  drawId: number;
  owner: string;
  mainNumbers: number[];
  megaNumber: number;
  purchaseTime: number;
  claimed: boolean;
  prizeTier: number;
}

// Parse Neo N3 stack item
function parseStackItem(item: any): any {
  if (!item) return null;

  switch (item.type) {
    case 'Integer':
      return parseInt(item.value, 10);
    case 'ByteString':
      // Try to decode as string or return hex
      try {
        return Buffer.from(item.value, 'base64').toString('utf8');
      } catch {
        return item.value;
      }
    case 'Array':
      return item.value.map(parseStackItem);
    case 'Map':
      const map: Record<string, any> = {};
      for (const entry of item.value) {
        const key = parseStackItem(entry.key);
        map[key] = parseStackItem(entry.value);
      }
      return map;
    case 'Boolean':
      return item.value;
    default:
      return item.value;
  }
}

export function useLotteryInfo() {
  const { invokeRead, address } = useWallet();

  return useQuery({
    queryKey: ['lottery-info'],
    queryFn: async (): Promise<LotteryInfo> => {
      try {
        const result = await invokeRead({
          scriptHash: LOTTERY_CONTRACT,
          operation: 'getLotteryInfo',
          args: [],
        });

        const data = parseStackItem(result[0]);

        return {
          currentDrawId: data.currentDrawId || 1,
          nextDrawTime: data.nextDrawTime || Date.now() + 86400000,
          currentPool: (data.currentPool / 1e8).toString() || '0',
          ticketCount: data.ticketCount || 0,
          jackpot: (data.jackpot / 1e8).toString() || '0',
          ticketPrice: (data.ticketPrice / 1e8).toString() || '2',
          isLocked: data.isLocked || false,
          isPaused: data.isPaused || false,
        };
      } catch (error) {
        // Return mock data for development
        return {
          currentDrawId: 42,
          nextDrawTime: Date.now() + 3600000, // 1 hour from now
          currentPool: '15420',
          ticketCount: 7710,
          jackpot: '125000',
          ticketPrice: '2',
          isLocked: false,
          isPaused: false,
        };
      }
    },
    refetchInterval: 30000, // Refresh every 30 seconds
    enabled: !!address,
  });
}

export function useRecentDraws(count: number = 10) {
  const { invokeRead, address } = useWallet();

  return useQuery({
    queryKey: ['recent-draws', count],
    queryFn: async (): Promise<Draw[]> => {
      try {
        const result = await invokeRead({
          scriptHash: LOTTERY_CONTRACT,
          operation: 'getRecentDraws',
          args: [{ type: 'Integer', value: count.toString() }],
        });

        const draws = parseStackItem(result[0]) as any[];

        return draws.map((d: any) => ({
          drawId: d.drawId,
          startTime: d.startTime,
          drawTime: d.drawTime,
          winningNumbers: d.winningNumbers || [],
          winningMega: d.winningMega || 0,
          totalPool: (d.totalPool / 1e8).toString(),
          ticketCount: d.ticketCount,
          completed: d.completed,
        }));
      } catch (error) {
        // Return mock data for development
        return [
          {
            drawId: 41,
            startTime: Date.now() - 86400000,
            drawTime: Date.now() - 3600000,
            winningNumbers: [7, 14, 28, 45, 62],
            winningMega: 12,
            totalPool: '12500',
            ticketCount: 6250,
            completed: true,
          },
          {
            drawId: 40,
            startTime: Date.now() - 172800000,
            drawTime: Date.now() - 90000000,
            winningNumbers: [3, 19, 33, 51, 67],
            winningMega: 8,
            totalPool: '18200',
            ticketCount: 9100,
            completed: true,
          },
        ];
      }
    },
    enabled: !!address,
  });
}

export function useUserTickets() {
  const { invokeRead, address } = useWallet();

  return useQuery({
    queryKey: ['user-tickets', address],
    queryFn: async (): Promise<Ticket[]> => {
      if (!address) return [];

      try {
        // Get ticket IDs
        const ticketIdsResult = await invokeRead({
          scriptHash: LOTTERY_CONTRACT,
          operation: 'getUserTickets',
          args: [{ type: 'Hash160', value: address }],
        });

        const ticketIds = parseStackItem(ticketIdsResult[0]) as number[];

        // Fetch each ticket
        const tickets: Ticket[] = [];
        for (const id of ticketIds.slice(-50)) { // Last 50 tickets
          const ticketResult = await invokeRead({
            scriptHash: LOTTERY_CONTRACT,
            operation: 'getTicket',
            args: [{ type: 'Integer', value: id.toString() }],
          });

          const t = parseStackItem(ticketResult[0]);
          if (t) {
            tickets.push({
              ticketId: t.ticketId,
              drawId: t.drawId,
              owner: t.owner,
              mainNumbers: t.mainNumbers || [],
              megaNumber: t.megaNumber,
              purchaseTime: t.purchaseTime,
              claimed: t.claimed,
              prizeTier: t.prizeTier,
            });
          }
        }

        return tickets.reverse();
      } catch (error) {
        return [];
      }
    },
    enabled: !!address,
  });
}

export function useBuyTicket() {
  const { invoke, address } = useWallet();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async ({
      mainNumbers,
      megaNumber,
    }: {
      mainNumbers: number[];
      megaNumber: number;
    }) => {
      if (!address) throw new Error('Wallet not connected');

      // Sort numbers before sending
      const sortedNumbers = [...mainNumbers].sort((a, b) => a - b);

      const txid = await invoke({
        scriptHash: LOTTERY_CONTRACT,
        operation: 'buyTicket',
        args: [
          { type: 'Array', value: sortedNumbers.map(n => ({ type: 'Integer', value: n.toString() })) },
          { type: 'Integer', value: megaNumber.toString() },
        ],
        signers: [
          {
            account: address,
            scopes: 17, // CalledByEntry | CustomContracts
          },
        ],
      });

      return txid;
    },
    onSuccess: () => {
      // Invalidate queries to refresh data
      queryClient.invalidateQueries({ queryKey: ['lottery-info'] });
      queryClient.invalidateQueries({ queryKey: ['user-tickets'] });
    },
  });
}

export function useQuickPick() {
  const { invoke, address } = useWallet();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async () => {
      if (!address) throw new Error('Wallet not connected');

      const txid = await invoke({
        scriptHash: LOTTERY_CONTRACT,
        operation: 'quickPick',
        args: [],
        signers: [
          {
            account: address,
            scopes: 17,
          },
        ],
      });

      return txid;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['lottery-info'] });
      queryClient.invalidateQueries({ queryKey: ['user-tickets'] });
    },
  });
}

export function useClaimPrize() {
  const { invoke, address } = useWallet();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (ticketId: number) => {
      if (!address) throw new Error('Wallet not connected');

      const txid = await invoke({
        scriptHash: LOTTERY_CONTRACT,
        operation: 'claimPrize',
        args: [{ type: 'Integer', value: ticketId.toString() }],
      });

      return txid;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['user-tickets'] });
    },
  });
}

export function useCheckTicket() {
  const { invokeRead } = useWallet();

  return useMutation({
    mutationFn: async (ticketId: number): Promise<number> => {
      const result = await invokeRead({
        scriptHash: LOTTERY_CONTRACT,
        operation: 'checkTicket',
        args: [{ type: 'Integer', value: ticketId.toString() }],
      });

      return parseStackItem(result[0]) as number;
    },
  });
}

// Prize tier names
export const PRIZE_TIERS: Record<number, { name: string; description: string }> = {
  0: { name: 'No Prize', description: 'Better luck next time!' },
  1: { name: 'JACKPOT!', description: '5 + Mega Ball' },
  2: { name: 'Second Prize', description: '5 Numbers' },
  3: { name: 'Third Prize', description: '4 + Mega Ball' },
  4: { name: 'Fourth Prize', description: '4 Numbers or 3 + Mega Ball' },
  5: { name: 'Fifth Prize', description: '3 Numbers or Mega Ball Match' },
};

// Generate random numbers for quick pick preview
export function generateRandomNumbers(): { main: number[]; mega: number } {
  const main: number[] = [];
  while (main.length < 5) {
    const num = Math.floor(Math.random() * 70) + 1;
    if (!main.includes(num)) {
      main.push(num);
    }
  }
  main.sort((a, b) => a - b);

  const mega = Math.floor(Math.random() * 25) + 1;

  return { main, mega };
}
