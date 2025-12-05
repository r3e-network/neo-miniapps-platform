import React, { useState } from 'react';
import { Link } from 'react-router-dom';
import { FiMenu, FiBell, FiUser, FiLogOut, FiSettings, FiChevronDown } from 'react-icons/fi';
import { useAuthStore } from '@/stores/authStore';
import { Button } from '@/components/common';

interface HeaderProps {
  onMenuClick?: () => void;
}

export function Header({ onMenuClick }: HeaderProps) {
  const { user, account, wallet, logout, disconnectWallet } = useAuthStore();
  const [showUserMenu, setShowUserMenu] = useState(false);

  const handleLogout = () => {
    disconnectWallet();
    logout();
  };

  return (
    <header className="fixed top-0 left-0 right-0 h-16 bg-white border-b border-surface-200 z-50">
      <div className="h-full px-4 flex items-center justify-between">
        {/* Left section */}
        <div className="flex items-center gap-4">
          <button
            onClick={onMenuClick}
            className="p-2 rounded-lg text-surface-500 hover:bg-surface-100 lg:hidden"
          >
            <FiMenu className="w-5 h-5" />
          </button>

          <Link to="/dashboard" className="flex items-center gap-2">
            <div className="w-8 h-8 bg-gradient-to-br from-primary-500 to-primary-700 rounded-lg flex items-center justify-center">
              <span className="text-white font-bold text-sm">SL</span>
            </div>
            <span className="font-semibold text-surface-900 hidden sm:block">Service Layer</span>
          </Link>
        </div>

        {/* Right section */}
        <div className="flex items-center gap-3">
          {/* Wallet Status */}
          {wallet.connected && (
            <div className="hidden md:flex items-center gap-2 px-3 py-1.5 bg-green-50 text-green-700 rounded-lg text-sm">
              <div className="w-2 h-2 bg-green-500 rounded-full animate-pulse" />
              <span className="font-mono">
                {wallet.address?.slice(0, 6)}...{wallet.address?.slice(-4)}
              </span>
              <span className="text-green-600">({wallet.balance} GAS)</span>
            </div>
          )}

          {/* Notifications */}
          <button className="p-2 rounded-lg text-surface-500 hover:bg-surface-100 relative">
            <FiBell className="w-5 h-5" />
            <span className="absolute top-1 right-1 w-2 h-2 bg-red-500 rounded-full" />
          </button>

          {/* User Menu */}
          <div className="relative">
            <button
              onClick={() => setShowUserMenu(!showUserMenu)}
              className="flex items-center gap-2 p-2 rounded-lg hover:bg-surface-100"
            >
              <div className="w-8 h-8 bg-primary-100 rounded-full flex items-center justify-center">
                <FiUser className="w-4 h-4 text-primary-600" />
              </div>
              <div className="hidden md:block text-left">
                <p className="text-sm font-medium text-surface-900">{user?.name || 'User'}</p>
                <p className="text-xs text-surface-500">{account?.tier || 'Free'} Plan</p>
              </div>
              <FiChevronDown className="w-4 h-4 text-surface-400" />
            </button>

            {showUserMenu && (
              <>
                <div
                  className="fixed inset-0 z-10"
                  onClick={() => setShowUserMenu(false)}
                />
                <div className="absolute right-0 mt-2 w-56 bg-white rounded-xl shadow-lg border border-surface-200 py-2 z-20 animate-slide-down">
                  <div className="px-4 py-2 border-b border-surface-100">
                    <p className="text-sm font-medium text-surface-900">{user?.email}</p>
                    <p className="text-xs text-surface-500">Account ID: {account?.id?.slice(0, 8)}...</p>
                  </div>

                  <Link
                    to="/settings"
                    className="flex items-center gap-2 px-4 py-2 text-sm text-surface-700 hover:bg-surface-50"
                    onClick={() => setShowUserMenu(false)}
                  >
                    <FiSettings className="w-4 h-4" />
                    Settings
                  </Link>

                  <button
                    onClick={handleLogout}
                    className="flex items-center gap-2 px-4 py-2 text-sm text-red-600 hover:bg-red-50 w-full"
                  >
                    <FiLogOut className="w-4 h-4" />
                    Sign Out
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      </div>
    </header>
  );
}
