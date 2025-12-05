import React from 'react';
import { Link } from 'react-router-dom';
import { useWallet } from '../hooks/useWallet';

export function Header() {
  const { connected, address, balance, connect, disconnect } = useWallet();

  return (
    <header className="bg-black/40 backdrop-blur-md border-b border-white/10">
      <div className="container mx-auto px-4 py-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center space-x-8">
            <Link to="/" className="text-2xl font-bold text-white flex items-center">
              <svg className="w-8 h-8 mr-2 text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" />
              </svg>
              <span className="text-purple-400">Privacy</span>Mixer
            </Link>
            <nav className="hidden md:flex space-x-6">
              <Link to="/" className="text-white/80 hover:text-white transition">Home</Link>
              <Link to="/deposit" className="text-white/80 hover:text-white transition">Deposit</Link>
              <Link to="/withdraw" className="text-white/80 hover:text-white transition">Withdraw</Link>
              <Link to="/stats" className="text-white/80 hover:text-white transition">Stats</Link>
            </nav>
          </div>

          <div className="flex items-center space-x-4">
            {connected ? (
              <>
                <div className="text-white/80 text-sm">
                  <span className="text-purple-400">{balance}</span> GAS
                </div>
                <div className="bg-white/10 rounded-lg px-4 py-2">
                  <span className="text-white text-sm">{address?.slice(0, 6)}...{address?.slice(-4)}</span>
                </div>
                <button onClick={disconnect} className="bg-red-500/20 text-red-400 px-4 py-2 rounded-lg hover:bg-red-500/30 transition">
                  Disconnect
                </button>
              </>
            ) : (
              <button onClick={connect} className="bg-gradient-to-r from-purple-500 to-pink-500 text-white font-semibold px-6 py-2 rounded-lg hover:opacity-90 transition">
                Connect Wallet
              </button>
            )}
          </div>
        </div>
      </div>
    </header>
  );
}
