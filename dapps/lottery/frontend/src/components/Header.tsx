import React from 'react';
import { Link } from 'react-router-dom';
import { useWallet } from '../hooks/useWallet';

export function Header() {
  const { connected, address, balance, connect, disconnect } = useWallet();

  return (
    <header className="bg-black/30 backdrop-blur-md border-b border-white/10">
      <div className="container mx-auto px-4 py-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center space-x-8">
            <Link to="/" className="text-2xl font-bold text-white">
              <span className="text-yellow-400">Mega</span>Lottery
            </Link>
            <nav className="hidden md:flex space-x-6">
              <Link to="/" className="text-white/80 hover:text-white transition">
                Home
              </Link>
              <Link to="/buy" className="text-white/80 hover:text-white transition">
                Buy Tickets
              </Link>
              <Link to="/tickets" className="text-white/80 hover:text-white transition">
                My Tickets
              </Link>
              <Link to="/results" className="text-white/80 hover:text-white transition">
                Results
              </Link>
            </nav>
          </div>

          <div className="flex items-center space-x-4">
            {connected ? (
              <>
                <div className="text-white/80 text-sm">
                  <span className="text-yellow-400">{balance}</span> GAS
                </div>
                <div className="bg-white/10 rounded-lg px-4 py-2">
                  <span className="text-white text-sm">
                    {address?.slice(0, 6)}...{address?.slice(-4)}
                  </span>
                </div>
                <button
                  onClick={disconnect}
                  className="bg-red-500/20 text-red-400 px-4 py-2 rounded-lg hover:bg-red-500/30 transition"
                >
                  Disconnect
                </button>
              </>
            ) : (
              <button
                onClick={connect}
                className="bg-gradient-to-r from-yellow-400 to-orange-500 text-black font-semibold px-6 py-2 rounded-lg hover:opacity-90 transition"
              >
                Connect Wallet
              </button>
            )}
          </div>
        </div>
      </div>
    </header>
  );
}
