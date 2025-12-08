import { useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { Ticket, Trophy, Home, HelpCircle, Wallet, ChevronDown, LogOut, X, Menu } from 'lucide-react';
import { useWallet, formatGas, WalletType } from '../hooks/useWallet';

export function Header() {
  const location = useLocation();
  const { address, balance, walletType, connect, disconnect, isConnecting } = useWallet();
  const [showWalletModal, setShowWalletModal] = useState(false);
  const [showAccountMenu, setShowAccountMenu] = useState(false);
  const [showMobileMenu, setShowMobileMenu] = useState(false);

  const navItems = [
    { path: '/', label: 'Home', icon: Home },
    { path: '/buy', label: 'Buy Ticket', icon: Ticket },
    { path: '/tickets', label: 'My Tickets', icon: Ticket },
    { path: '/results', label: 'Results', icon: Trophy },
    { path: '/how-to-play', label: 'How to Play', icon: HelpCircle },
  ];

  const wallets: { type: WalletType; name: string; icon: string; description: string }[] = [
    { type: 'neoline', name: 'NeoLine', icon: '🔗', description: 'Browser extension wallet' },
    { type: 'onegate', name: 'OneGate', icon: '🚪', description: 'Multi-chain wallet' },
    { type: 'o3', name: 'O3 Wallet', icon: '⭕', description: 'Mobile & desktop wallet' },
  ];

  const handleConnect = async (type: WalletType) => {
    try {
      await connect(type);
      setShowWalletModal(false);
    } catch (error) {
      console.error('Failed to connect:', error);
    }
  };

  const shortenAddress = (addr: string) => {
    return `${addr.slice(0, 6)}...${addr.slice(-4)}`;
  };

  return (
    <header className="border-b border-white/10 bg-black/40 backdrop-blur-xl sticky top-0 z-50">
      <div className="container mx-auto px-4">
        <div className="flex items-center justify-between h-16">
          {/* Logo */}
          <Link to="/" className="flex items-center gap-3 group">
            <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-yellow-400 via-yellow-500 to-orange-500 flex items-center justify-center shadow-lg shadow-yellow-500/20 group-hover:shadow-yellow-500/40 transition-shadow">
              <span className="text-xl">🎰</span>
            </div>
            <div className="hidden sm:block">
              <span className="text-xl font-bold text-white">Mega</span>
              <span className="text-xl font-bold text-gradient-gold">Lottery</span>
            </div>
          </Link>

          {/* Desktop Navigation */}
          <nav className="hidden lg:flex items-center gap-1">
            {navItems.map((item) => {
              const Icon = item.icon;
              const isActive = location.pathname === item.path;
              return (
                <Link
                  key={item.path}
                  to={item.path}
                  className={`flex items-center gap-2 px-4 py-2 rounded-xl transition-all duration-200 ${
                    isActive
                      ? 'bg-yellow-500/20 text-yellow-400 shadow-inner'
                      : 'text-gray-400 hover:text-white hover:bg-white/5'
                  }`}
                >
                  <Icon className="w-4 h-4" />
                  <span className="text-sm font-medium">{item.label}</span>
                </Link>
              );
            })}
          </nav>

          {/* Right Section */}
          <div className="flex items-center gap-3">
            {/* Wallet */}
            <div className="relative">
              {address ? (
                <div className="relative">
                  <button
                    onClick={() => setShowAccountMenu(!showAccountMenu)}
                    className="flex items-center gap-3 glass hover:bg-white/10 rounded-xl px-4 py-2 transition-all duration-200"
                  >
                    <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-purple-500 to-blue-500 flex items-center justify-center">
                      <span className="text-xs font-bold text-white">
                        {address.slice(0, 2)}
                      </span>
                    </div>
                    <div className="text-right hidden sm:block">
                      <div className="text-sm font-medium text-white">{shortenAddress(address)}</div>
                      <div className="text-xs text-yellow-400">{formatGas(balance)} GAS</div>
                    </div>
                    <ChevronDown className={`w-4 h-4 text-gray-400 transition-transform ${showAccountMenu ? 'rotate-180' : ''}`} />
                  </button>

                  {showAccountMenu && (
                    <>
                      <div
                        className="fixed inset-0 z-40"
                        onClick={() => setShowAccountMenu(false)}
                      />
                      <div className="absolute right-0 mt-2 w-56 glass rounded-xl shadow-2xl border border-white/10 py-2 z-50 animate-slide-up">
                        <div className="px-4 py-3 border-b border-white/10">
                          <div className="text-xs text-gray-400 mb-1">Connected with</div>
                          <div className="text-sm text-white font-medium capitalize flex items-center gap-2">
                            <span className="w-2 h-2 rounded-full bg-green-400 animate-pulse" />
                            {walletType}
                          </div>
                        </div>
                        <div className="px-4 py-3 border-b border-white/10">
                          <div className="text-xs text-gray-400 mb-1">Balance</div>
                          <div className="text-lg font-bold text-yellow-400">{formatGas(balance)} GAS</div>
                        </div>
                        <button
                          onClick={() => {
                            disconnect();
                            setShowAccountMenu(false);
                          }}
                          className="w-full flex items-center gap-3 px-4 py-3 text-red-400 hover:bg-red-500/10 transition-colors"
                        >
                          <LogOut className="w-4 h-4" />
                          <span className="font-medium">Disconnect</span>
                        </button>
                      </div>
                    </>
                  )}
                </div>
              ) : (
                <button
                  onClick={() => setShowWalletModal(true)}
                  className="btn-primary flex items-center gap-2 px-5 py-2.5"
                >
                  <Wallet className="w-4 h-4" />
                  <span className="hidden sm:inline">Connect Wallet</span>
                  <span className="sm:hidden">Connect</span>
                </button>
              )}
            </div>

            {/* Mobile Menu Button */}
            <button
              onClick={() => setShowMobileMenu(!showMobileMenu)}
              className="lg:hidden p-2 text-gray-400 hover:text-white transition-colors"
            >
              <Menu className="w-6 h-6" />
            </button>
          </div>
        </div>
      </div>

      {/* Mobile Navigation */}
      {showMobileMenu && (
        <>
          <div
            className="fixed inset-0 bg-black/60 z-40 lg:hidden"
            onClick={() => setShowMobileMenu(false)}
          />
          <div className="absolute top-full left-0 right-0 glass border-t border-white/10 py-4 z-50 lg:hidden animate-slide-up">
            <nav className="container mx-auto px-4 space-y-1">
              {navItems.map((item) => {
                const Icon = item.icon;
                const isActive = location.pathname === item.path;
                return (
                  <Link
                    key={item.path}
                    to={item.path}
                    onClick={() => setShowMobileMenu(false)}
                    className={`flex items-center gap-3 px-4 py-3 rounded-xl transition-colors ${
                      isActive
                        ? 'bg-yellow-500/20 text-yellow-400'
                        : 'text-gray-400 hover:text-white hover:bg-white/5'
                    }`}
                  >
                    <Icon className="w-5 h-5" />
                    <span className="font-medium">{item.label}</span>
                  </Link>
                );
              })}
            </nav>
          </div>
        </>
      )}

      {/* Wallet Modal */}
      {showWalletModal && (
        <div className="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center z-50 p-4">
          <div
            className="absolute inset-0"
            onClick={() => setShowWalletModal(false)}
          />
          <div className="glass rounded-2xl p-6 max-w-sm w-full border border-white/10 relative z-10 animate-slide-up">
            <div className="flex items-center justify-between mb-6">
              <div>
                <h2 className="text-xl font-bold text-white">Connect Wallet</h2>
                <p className="text-sm text-gray-400 mt-1">Choose your Neo N3 wallet</p>
              </div>
              <button
                onClick={() => setShowWalletModal(false)}
                className="p-2 text-gray-400 hover:text-white hover:bg-white/10 rounded-lg transition-colors"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <div className="space-y-3">
              {wallets.map((wallet) => (
                <button
                  key={wallet.type}
                  onClick={() => handleConnect(wallet.type)}
                  disabled={isConnecting}
                  className="w-full flex items-center gap-4 bg-white/5 hover:bg-white/10 border border-white/10 hover:border-yellow-500/30 rounded-xl p-4 transition-all duration-200 disabled:opacity-50 group"
                >
                  <div className="w-12 h-12 rounded-xl bg-gradient-to-br from-gray-700 to-gray-800 flex items-center justify-center text-2xl group-hover:scale-110 transition-transform">
                    {wallet.icon}
                  </div>
                  <div className="text-left flex-1">
                    <div className="text-white font-semibold">{wallet.name}</div>
                    <div className="text-gray-400 text-sm">{wallet.description}</div>
                  </div>
                  {isConnecting && (
                    <div className="w-5 h-5 border-2 border-yellow-400 border-t-transparent rounded-full animate-spin" />
                  )}
                </button>
              ))}
            </div>

            <div className="mt-6 pt-4 border-t border-white/10">
              <p className="text-gray-500 text-xs text-center">
                By connecting, you agree to the Terms of Service and Privacy Policy
              </p>
            </div>
          </div>
        </div>
      )}
    </header>
  );
}
