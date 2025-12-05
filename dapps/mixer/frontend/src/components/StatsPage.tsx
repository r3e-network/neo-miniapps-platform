import React from 'react';
import { useMixer } from '../hooks/useMixer';

export function StatsPage() {
  const { stats, pools, loading } = useMixer();

  if (loading) {
    return (
      <div className="max-w-3xl mx-auto text-center">
        <div className="animate-spin w-12 h-12 border-4 border-purple-400 border-t-transparent rounded-full mx-auto"></div>
        <p className="text-white/70 mt-4">Loading statistics...</p>
      </div>
    );
  }

  return (
    <div className="max-w-3xl mx-auto">
      <h1 className="text-3xl font-bold text-white mb-8 text-center">Mixer Statistics</h1>

      {/* Global Stats */}
      <div className="grid md:grid-cols-3 gap-6 mb-8">
        <div className="bg-white/5 backdrop-blur-md rounded-xl p-6 border border-white/10 text-center">
          <p className="text-white/60 text-sm mb-1">Total Deposits</p>
          <p className="text-3xl font-bold text-white">{stats?.totalDeposits || 0}</p>
        </div>
        <div className="bg-white/5 backdrop-blur-md rounded-xl p-6 border border-white/10 text-center">
          <p className="text-white/60 text-sm mb-1">Total Withdrawals</p>
          <p className="text-3xl font-bold text-white">{stats?.totalWithdrawals || 0}</p>
        </div>
        <div className="bg-white/5 backdrop-blur-md rounded-xl p-6 border border-white/10 text-center">
          <p className="text-white/60 text-sm mb-1">Total Volume</p>
          <p className="text-3xl font-bold text-purple-400">{stats?.totalVolume || 0} <span className="text-lg">GAS</span></p>
        </div>
      </div>

      {/* Pool Details */}
      <div className="bg-white/5 backdrop-blur-md rounded-2xl p-6 border border-white/10">
        <h2 className="text-xl font-bold text-white mb-6">Pool Details</h2>
        <div className="space-y-4">
          {[
            { id: 1, amount: '1 GAS', deposits: stats?.pool1Deposits || 0 },
            { id: 2, amount: '10 GAS', deposits: stats?.pool2Deposits || 0 },
            { id: 3, amount: '100 GAS', deposits: stats?.pool3Deposits || 0 },
          ].map(pool => (
            <div key={pool.id} className="flex items-center justify-between bg-white/5 rounded-xl p-4">
              <div className="flex items-center">
                <div className="w-10 h-10 bg-purple-500/20 rounded-full flex items-center justify-center mr-4">
                  <span className="text-purple-400 font-bold">{pool.id}</span>
                </div>
                <div>
                  <p className="text-white font-semibold">{pool.amount} Pool</p>
                  <p className="text-white/60 text-sm">Fixed denomination</p>
                </div>
              </div>
              <div className="text-right">
                <p className="text-white font-semibold">{pool.deposits}</p>
                <p className="text-white/60 text-sm">deposits</p>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
