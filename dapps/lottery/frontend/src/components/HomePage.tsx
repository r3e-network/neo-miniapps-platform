import React from 'react';
import { Link } from 'react-router-dom';
import { useLottery } from '../hooks/useLottery';

export function HomePage() {
  const { currentRound, loading } = useLottery();

  const formatTime = (timestamp: number) => {
    const date = new Date(timestamp);
    return date.toLocaleString();
  };

  const getTimeRemaining = (endTime: number) => {
    const now = Date.now();
    const diff = endTime - now;
    if (diff <= 0) return 'Draw in progress...';

    const hours = Math.floor(diff / (1000 * 60 * 60));
    const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60));
    return `${hours}h ${minutes}m remaining`;
  };

  const statusText = (status: number) => {
    switch (status) {
      case 0: return 'Open';
      case 1: return 'Drawing';
      case 2: return 'Completed';
      case 3: return 'Cancelled';
      default: return 'Unknown';
    }
  };

  return (
    <div className="max-w-4xl mx-auto">
      {/* Hero Section */}
      <div className="text-center mb-12">
        <h1 className="text-5xl font-bold text-white mb-4">
          Win Big with <span className="text-yellow-400">MegaLottery</span>
        </h1>
        <p className="text-xl text-white/70 mb-8">
          Powered by Service Layer VRF for provably fair draws
        </p>
      </div>

      {/* Current Round Card */}
      {loading ? (
        <div className="bg-white/10 backdrop-blur-md rounded-2xl p-8 text-center">
          <div className="animate-spin w-12 h-12 border-4 border-yellow-400 border-t-transparent rounded-full mx-auto"></div>
          <p className="text-white/70 mt-4">Loading lottery data...</p>
        </div>
      ) : currentRound ? (
        <div className="bg-white/10 backdrop-blur-md rounded-2xl p-8 border border-white/20">
          <div className="flex justify-between items-start mb-6">
            <div>
              <span className="text-white/60 text-sm">Round #{currentRound.roundId}</span>
              <span className={`ml-3 px-3 py-1 rounded-full text-xs font-semibold ${
                currentRound.status === 0 ? 'bg-green-500/20 text-green-400' :
                currentRound.status === 1 ? 'bg-yellow-500/20 text-yellow-400' :
                'bg-gray-500/20 text-gray-400'
              }`}>
                {statusText(currentRound.status)}
              </span>
            </div>
            <div className="text-right">
              <p className="text-white/60 text-sm">Draw Time</p>
              <p className="text-white font-semibold">{formatTime(currentRound.endTime)}</p>
            </div>
          </div>

          {/* Jackpot Display */}
          <div className="text-center py-8 bg-gradient-to-r from-yellow-500/20 to-orange-500/20 rounded-xl mb-6">
            <p className="text-white/60 text-sm mb-2">Current Jackpot</p>
            <p className="text-5xl font-bold text-yellow-400">
              {currentRound.jackpot} <span className="text-2xl">GAS</span>
            </p>
            <p className="text-white/60 mt-2">{getTimeRemaining(currentRound.endTime)}</p>
          </div>

          {/* Stats */}
          <div className="grid grid-cols-3 gap-4 mb-6">
            <div className="bg-white/5 rounded-lg p-4 text-center">
              <p className="text-white/60 text-sm">Ticket Price</p>
              <p className="text-xl font-semibold text-white">{currentRound.ticketPrice} GAS</p>
            </div>
            <div className="bg-white/5 rounded-lg p-4 text-center">
              <p className="text-white/60 text-sm">Tickets Sold</p>
              <p className="text-xl font-semibold text-white">{currentRound.ticketCount}</p>
            </div>
            <div className="bg-white/5 rounded-lg p-4 text-center">
              <p className="text-white/60 text-sm">Pick Numbers</p>
              <p className="text-xl font-semibold text-white">6 of 49</p>
            </div>
          </div>

          {/* Winning Numbers (if completed) */}
          {currentRound.winningNumbers && (
            <div className="mb-6">
              <p className="text-white/60 text-sm mb-3 text-center">Winning Numbers</p>
              <div className="flex justify-center space-x-3">
                {currentRound.winningNumbers.map((num, i) => (
                  <div
                    key={i}
                    className="w-12 h-12 rounded-full bg-gradient-to-br from-yellow-400 to-orange-500 flex items-center justify-center text-black font-bold text-lg"
                  >
                    {num}
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* CTA Button */}
          {currentRound.status === 0 && (
            <Link
              to="/buy"
              className="block w-full bg-gradient-to-r from-yellow-400 to-orange-500 text-black font-bold text-lg py-4 rounded-xl text-center hover:opacity-90 transition"
            >
              Buy Tickets Now
            </Link>
          )}
        </div>
      ) : (
        <div className="bg-white/10 backdrop-blur-md rounded-2xl p-8 text-center">
          <p className="text-white/70">No active lottery round</p>
        </div>
      )}

      {/* Features */}
      <div className="grid md:grid-cols-3 gap-6 mt-12">
        <div className="bg-white/5 backdrop-blur-md rounded-xl p-6 border border-white/10">
          <div className="w-12 h-12 bg-blue-500/20 rounded-lg flex items-center justify-center mb-4">
            <svg className="w-6 h-6 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
            </svg>
          </div>
          <h3 className="text-lg font-semibold text-white mb-2">Provably Fair</h3>
          <p className="text-white/60 text-sm">
            VRF (Verifiable Random Function) ensures transparent and verifiable random number generation.
          </p>
        </div>

        <div className="bg-white/5 backdrop-blur-md rounded-xl p-6 border border-white/10">
          <div className="w-12 h-12 bg-green-500/20 rounded-lg flex items-center justify-center mb-4">
            <svg className="w-6 h-6 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          </div>
          <h3 className="text-lg font-semibold text-white mb-2">Automated Draws</h3>
          <p className="text-white/60 text-sm">
            Service Layer Automation ensures draws happen on schedule without manual intervention.
          </p>
        </div>

        <div className="bg-white/5 backdrop-blur-md rounded-xl p-6 border border-white/10">
          <div className="w-12 h-12 bg-purple-500/20 rounded-lg flex items-center justify-center mb-4">
            <svg className="w-6 h-6 text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 9V7a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2m2 4h10a2 2 0 002-2v-6a2 2 0 00-2-2H9a2 2 0 00-2 2v6a2 2 0 002 2zm7-5a2 2 0 11-4 0 2 2 0 014 0z" />
            </svg>
          </div>
          <h3 className="text-lg font-semibold text-white mb-2">Instant Payouts</h3>
          <p className="text-white/60 text-sm">
            Winners can claim their prizes instantly through smart contract execution.
          </p>
        </div>
      </div>
    </div>
  );
}
