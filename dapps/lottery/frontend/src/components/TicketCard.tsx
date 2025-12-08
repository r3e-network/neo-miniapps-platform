import { useState } from 'react';
import { Check, Clock, Trophy, Loader2, Sparkles, XCircle } from 'lucide-react';
import { Ticket, PRIZE_TIERS, useClaimPrize, useCheckTicket } from '../hooks/useLottery';

interface TicketCardProps {
  ticket: Ticket;
  winningNumbers?: number[];
  winningMega?: number;
  drawCompleted?: boolean;
}

export function TicketCard({ ticket, winningNumbers, winningMega, drawCompleted }: TicketCardProps) {
  const [prizeTier, setPrizeTier] = useState<number | null>(ticket.prizeTier || null);
  const claimPrize = useClaimPrize();
  const checkTicket = useCheckTicket();

  const isMainMatch = (num: number) => winningNumbers?.includes(num);
  const isMegaMatch = ticket.megaNumber === winningMega;

  const matchCount = winningNumbers
    ? ticket.mainNumbers.filter((n) => winningNumbers.includes(n)).length
    : 0;

  const handleCheck = async () => {
    try {
      const tier = await checkTicket.mutateAsync(ticket.ticketId);
      setPrizeTier(tier);
    } catch (error) {
      console.error('Failed to check ticket:', error);
    }
  };

  const handleClaim = async () => {
    try {
      await claimPrize.mutateAsync(ticket.ticketId);
    } catch (error) {
      console.error('Failed to claim prize:', error);
    }
  };

  const formatDate = (timestamp: number) => {
    return new Date(timestamp).toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  const isWinner = prizeTier !== null && prizeTier > 0;

  return (
    <div className={`ticket-card rounded-2xl p-5 ${isWinner ? 'winner' : ''}`}>
      {/* Header */}
      <div className="flex items-start justify-between mb-5">
        <div>
          <div className="flex items-center gap-2">
            <span className="text-sm font-semibold text-white">Ticket #{ticket.ticketId}</span>
            {isWinner && (
              <span className="prize-badge jackpot text-xs">
                <Trophy className="w-3 h-3" />
                Winner
              </span>
            )}
          </div>
          <div className="text-xs text-gray-500 mt-1">Draw #{ticket.drawId}</div>
        </div>
        <div className="text-right">
          <div className="text-xs text-gray-500">{formatDate(ticket.purchaseTime)}</div>
          {ticket.claimed && (
            <span className="inline-flex items-center gap-1 text-xs text-green-400 mt-1 bg-green-500/10 px-2 py-0.5 rounded-full">
              <Check className="w-3 h-3" />
              Claimed
            </span>
          )}
          {!drawCompleted && (
            <span className="inline-flex items-center gap-1 text-xs text-yellow-400 mt-1 bg-yellow-500/10 px-2 py-0.5 rounded-full">
              <Clock className="w-3 h-3" />
              Pending
            </span>
          )}
        </div>
      </div>

      {/* Numbers */}
      <div className="flex items-center justify-center gap-2 flex-wrap mb-5">
        {ticket.mainNumbers.map((num, index) => (
          <div
            key={index}
            className={`lottery-ball small ${
              drawCompleted && isMainMatch(num) ? 'winning' : ''
            }`}
            style={{
              background: drawCompleted && isMainMatch(num)
                ? 'linear-gradient(145deg, #22c55e 0%, #16a34a 100%)'
                : undefined,
              color: drawCompleted && isMainMatch(num) ? 'white' : undefined,
            }}
          >
            {num}
          </div>
        ))}
        <div className="w-px h-10 bg-gradient-to-b from-transparent via-gray-600 to-transparent mx-1" />
        <div
          className={`lottery-ball small mega ${
            drawCompleted && isMegaMatch ? 'winning' : ''
          }`}
          style={{
            background: drawCompleted && isMegaMatch
              ? 'linear-gradient(145deg, #22c55e 0%, #16a34a 100%)'
              : undefined,
          }}
        >
          {ticket.megaNumber}
        </div>
      </div>

      {/* Match Summary (when draw completed) */}
      {drawCompleted && winningNumbers && (
        <div className="flex items-center justify-center gap-4 mb-4 py-2 bg-white/5 rounded-lg">
          <div className="text-center">
            <div className="text-lg font-bold text-white">{matchCount}</div>
            <div className="text-xs text-gray-400">Numbers</div>
          </div>
          <div className="w-px h-8 bg-gray-700" />
          <div className="text-center">
            <div className="text-lg font-bold text-white">{isMegaMatch ? '1' : '0'}</div>
            <div className="text-xs text-gray-400">Mega</div>
          </div>
        </div>
      )}

      {/* Prize Status */}
      {drawCompleted && (
        <div className="border-t border-white/10 pt-4">
          {prizeTier === null && !ticket.claimed ? (
            <button
              onClick={handleCheck}
              disabled={checkTicket.isPending}
              className="w-full btn-secondary py-2.5 text-sm flex items-center justify-center gap-2"
            >
              {checkTicket.isPending ? (
                <>
                  <Loader2 className="w-4 h-4 animate-spin" />
                  Checking...
                </>
              ) : (
                <>
                  <Sparkles className="w-4 h-4" />
                  Check Ticket
                </>
              )}
            </button>
          ) : prizeTier !== null && prizeTier > 0 && !ticket.claimed ? (
            <div className="space-y-3">
              <div className="text-center py-3 bg-yellow-500/10 border border-yellow-500/30 rounded-xl">
                <div className="flex items-center justify-center gap-2 text-yellow-400 mb-1">
                  <Trophy className="w-5 h-5" />
                  <span className="font-bold text-lg">{PRIZE_TIERS[prizeTier].name}</span>
                </div>
                <div className="text-sm text-yellow-400/80">{PRIZE_TIERS[prizeTier].description}</div>
              </div>
              <button
                onClick={handleClaim}
                disabled={claimPrize.isPending}
                className="w-full btn-primary py-2.5 text-sm flex items-center justify-center gap-2"
              >
                {claimPrize.isPending ? (
                  <>
                    <Loader2 className="w-4 h-4 animate-spin" />
                    Claiming...
                  </>
                ) : (
                  <>
                    <Sparkles className="w-4 h-4" />
                    Claim Prize
                  </>
                )}
              </button>
            </div>
          ) : prizeTier === 0 ? (
            <div className="flex items-center justify-center gap-2 py-3 bg-white/5 rounded-xl text-gray-400 text-sm">
              <XCircle className="w-4 h-4" />
              No prize - Better luck next time!
            </div>
          ) : ticket.claimed ? (
            <div className="flex items-center justify-center gap-2 py-3 bg-green-500/10 border border-green-500/30 rounded-xl text-green-400 text-sm">
              <Check className="w-4 h-4" />
              Prize successfully claimed!
            </div>
          ) : null}
        </div>
      )}
    </div>
  );
}
