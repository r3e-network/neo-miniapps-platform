import React from 'react';
import { Link } from 'react-router-dom';
import { useMixer } from '../hooks/useMixer';

export function HomePage() {
  const { stats, loading } = useMixer();

  return (
    <div className="max-w-4xl mx-auto">
      {/* Hero */}
      <div className="text-center mb-12">
        <h1 className="text-5xl font-bold text-white mb-4">
          Private Transactions on <span className="text-purple-400">Neo N3</span>
        </h1>
        <p className="text-xl text-white/70 mb-8">
          Break the link between your addresses with TEE-powered mixing
        </p>
        <div className="flex justify-center space-x-4">
          <Link to="/deposit" className="bg-gradient-to-r from-purple-500 to-pink-500 text-white font-bold px-8 py-3 rounded-xl hover:opacity-90 transition">
            Start Mixing
          </Link>
          <Link to="/stats" className="bg-white/10 text-white font-bold px-8 py-3 rounded-xl hover:bg-white/20 transition">
            View Stats
          </Link>
        </div>
      </div>

      {/* Pool Cards */}
      <div className="grid md:grid-cols-3 gap-6 mb-12">
        {[
          { id: 1, amount: '1', deposits: stats?.pool1Deposits || 0 },
          { id: 2, amount: '10', deposits: stats?.pool2Deposits || 0 },
          { id: 3, amount: '100', deposits: stats?.pool3Deposits || 0 },
        ].map(pool => (
          <div key={pool.id} className="bg-white/5 backdrop-blur-md rounded-2xl p-6 border border-white/10 hover:border-purple-500/50 transition">
            <div className="text-center">
              <p className="text-white/60 text-sm mb-2">Pool #{pool.id}</p>
              <p className="text-4xl font-bold text-white mb-1">{pool.amount}</p>
              <p className="text-purple-400 font-semibold mb-4">GAS</p>
              <div className="bg-white/5 rounded-lg p-3">
                <p className="text-white/60 text-xs">Anonymity Set</p>
                <p className="text-white font-semibold">{pool.deposits} deposits</p>
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* How It Works */}
      <div className="bg-white/5 backdrop-blur-md rounded-2xl p-8 border border-white/10 mb-12">
        <h2 className="text-2xl font-bold text-white mb-6 text-center">How It Works</h2>
        <div className="grid md:grid-cols-4 gap-6">
          {[
            { step: 1, title: 'Deposit', desc: 'Send a fixed amount to the mixer with a secret commitment' },
            { step: 2, title: 'Wait', desc: 'Your deposit joins the anonymity pool with others' },
            { step: 3, title: 'Withdraw', desc: 'Withdraw to a new address using your secret proof' },
            { step: 4, title: 'Private', desc: 'No link between deposit and withdrawal addresses' },
          ].map(item => (
            <div key={item.step} className="text-center">
              <div className="w-12 h-12 bg-purple-500/20 rounded-full flex items-center justify-center mx-auto mb-3">
                <span className="text-purple-400 font-bold">{item.step}</span>
              </div>
              <h3 className="text-white font-semibold mb-2">{item.title}</h3>
              <p className="text-white/60 text-sm">{item.desc}</p>
            </div>
          ))}
        </div>
      </div>

      {/* Security Features */}
      <div className="grid md:grid-cols-3 gap-6">
        <div className="bg-white/5 backdrop-blur-md rounded-xl p-6 border border-white/10">
          <div className="w-12 h-12 bg-green-500/20 rounded-lg flex items-center justify-center mb-4">
            <svg className="w-6 h-6 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
            </svg>
          </div>
          <h3 className="text-lg font-semibold text-white mb-2">TEE Protected</h3>
          <p className="text-white/60 text-sm">All mixing operations happen inside a Trusted Execution Environment</p>
        </div>

        <div className="bg-white/5 backdrop-blur-md rounded-xl p-6 border border-white/10">
          <div className="w-12 h-12 bg-blue-500/20 rounded-lg flex items-center justify-center mb-4">
            <svg className="w-6 h-6 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
            </svg>
          </div>
          <h3 className="text-lg font-semibold text-white mb-2">Non-Custodial</h3>
          <p className="text-white/60 text-sm">Only you control your funds with cryptographic proofs</p>
        </div>

        <div className="bg-white/5 backdrop-blur-md rounded-xl p-6 border border-white/10">
          <div className="w-12 h-12 bg-purple-500/20 rounded-lg flex items-center justify-center mb-4">
            <svg className="w-6 h-6 text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21" />
            </svg>
          </div>
          <h3 className="text-lg font-semibold text-white mb-2">Unlinkable</h3>
          <p className="text-white/60 text-sm">Deposits and withdrawals cannot be linked on-chain</p>
        </div>
      </div>
    </div>
  );
}
