import React from 'react';
import { useLottery } from '../hooks/useLottery';
import { useWallet } from '../hooks/useWallet';

export function MyTicketsPage() {
  const { currentRound, claimPrize } = useLottery();
  const { connected, address, signAndInvoke } = useWallet();

  // Mock tickets for demo
  const tickets = [
    { ticketId: 1, roundId: 1, numbers: [5, 12, 23, 34, 41, 49], claimed: false },
    { ticketId: 2, roundId: 1, numbers: [3, 17, 25, 33, 42, 48], claimed: true },
  ];

  const handleClaim = async (roundId: number, ticketId: number) => {
    try {
      const script = await claimPrize(roundId, ticketId);
      await signAndInvoke(script);
      alert('Prize claimed successfully!');
    } catch (error) {
      alert('Failed to claim: ' + (error as Error).message);
    }
  };

  if (!connected) {
    return (
      <div className="max-w-2xl mx-auto text-center">
        <div className="bg-white/10 backdrop-blur-md rounded-2xl p-8">
          <p className="text-white/70 mb-4">Connect your wallet to view your tickets</p>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-2xl mx-auto">
      <h1 className="text-3xl font-bold text-white mb-8 text-center">My Tickets</h1>

      <div className="space-y-4">
        {tickets.map(ticket => (
          <div
            key={ticket.ticketId}
            className="bg-white/10 backdrop-blur-md rounded-xl p-6 border border-white/20"
          >
            <div className="flex justify-between items-start mb-4">
              <div>
                <p className="text-white/60 text-sm">Ticket #{ticket.ticketId}</p>
                <p className="text-white/60 text-sm">Round #{ticket.roundId}</p>
              </div>
              <span className={`px-3 py-1 rounded-full text-xs font-semibold ${
                ticket.claimed ? 'bg-green-500/20 text-green-400' : 'bg-yellow-500/20 text-yellow-400'
              }`}>
                {ticket.claimed ? 'Claimed' : 'Active'}
              </span>
            </div>

            <div className="flex space-x-2 mb-4">
              {ticket.numbers.map((num, i) => (
                <div
                  key={i}
                  className="w-10 h-10 rounded-full bg-white/10 flex items-center justify-center text-white font-semibold"
                >
                  {num}
                </div>
              ))}
            </div>

            {!ticket.claimed && currentRound?.status === 2 && (
              <button
                onClick={() => handleClaim(ticket.roundId, ticket.ticketId)}
                className="w-full bg-gradient-to-r from-green-400 to-emerald-500 text-black font-semibold py-2 rounded-lg hover:opacity-90 transition"
              >
                Claim Prize
              </button>
            )}
          </div>
        ))}

        {tickets.length === 0 && (
          <div className="bg-white/10 backdrop-blur-md rounded-xl p-8 text-center">
            <p className="text-white/70">You haven't purchased any tickets yet</p>
          </div>
        )}
      </div>
    </div>
  );
}
