import { sc } from '@cityofzion/neon-js';

const LOTTERY_CONTRACT = import.meta.env.VITE_LOTTERY_CONTRACT || '';
const GATEWAY_CONTRACT = import.meta.env.VITE_GATEWAY_CONTRACT || '';

/**
 * Creates a script for buying lottery tickets.
 */
export function createBuyTicketScript(
  roundId: number,
  numbers: number[],
  ticketCount: number
): string {
  const script = sc.createScript({
    scriptHash: LOTTERY_CONTRACT,
    operation: 'buyTicket',
    args: [
      sc.ContractParam.integer(roundId),
      sc.ContractParam.array(...numbers.map(n => sc.ContractParam.integer(n))),
      sc.ContractParam.integer(ticketCount),
    ],
  });
  return script.str;
}

/**
 * Creates a script for claiming prizes.
 */
export function createClaimPrizeScript(ticketId: string): string {
  const script = sc.createScript({
    scriptHash: LOTTERY_CONTRACT,
    operation: 'claimPrize',
    args: [sc.ContractParam.byteArray(ticketId)],
  });
  return script.str;
}

/**
 * Creates a script for requesting VRF randomness via Gateway.
 */
export function createRequestRandomnessScript(roundId: number): string {
  const script = sc.createScript({
    scriptHash: GATEWAY_CONTRACT,
    operation: 'requestRandomness',
    args: [
      sc.ContractParam.hash160(LOTTERY_CONTRACT),
      sc.ContractParam.integer(roundId),
    ],
  });
  return script.str;
}

/**
 * Parses lottery ticket data from contract response.
 */
export interface TicketData {
  id: string;
  roundId: number;
  numbers: number[];
  owner: string;
  claimed: boolean;
  prize: number;
}

export function parseTicketData(data: any[]): TicketData {
  return {
    id: data[0] as string,
    roundId: parseInt(data[1] as string),
    numbers: (data[2] as any[]).map(n => parseInt(n as string)),
    owner: data[3] as string,
    claimed: data[4] as boolean,
    prize: parseInt(data[5] as string) / 1e8,
  };
}

/**
 * Parses round data from contract response.
 */
export interface RoundData {
  id: number;
  startTime: number;
  endTime: number;
  ticketPrice: number;
  prizePool: number;
  winningNumbers: number[];
  status: 'active' | 'drawing' | 'completed';
  totalTickets: number;
}

export function parseRoundData(data: any[]): RoundData {
  const statusMap: Record<number, RoundData['status']> = {
    0: 'active',
    1: 'drawing',
    2: 'completed',
  };

  return {
    id: parseInt(data[0] as string),
    startTime: parseInt(data[1] as string) * 1000,
    endTime: parseInt(data[2] as string) * 1000,
    ticketPrice: parseInt(data[3] as string) / 1e8,
    prizePool: parseInt(data[4] as string) / 1e8,
    winningNumbers: (data[5] as any[]).map(n => parseInt(n as string)),
    status: statusMap[parseInt(data[6] as string)] || 'active',
    totalTickets: parseInt(data[7] as string),
  };
}
