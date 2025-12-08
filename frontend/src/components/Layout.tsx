import { Link, useLocation } from 'react-router-dom';
import { Home, Server, Key, Wallet, Zap, Settings, LogOut, Shield, Shuffle } from 'lucide-react';
import { useAuthStore } from '../stores/auth';

const navigation = [
  { name: 'Dashboard', href: '/', icon: Home },
  { name: 'Services', href: '/services', icon: Server },
  { name: 'Mixer', href: '/mixer', icon: Shuffle },
  { name: 'Secrets', href: '/secrets', icon: Key },
  { name: 'Gas Bank', href: '/gasbank', icon: Wallet },
  { name: 'Automation', href: '/automation', icon: Zap },
  { name: 'Settings', href: '/settings', icon: Settings },
];

export function Layout({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  const { user, logout } = useAuthStore();

  return (
    <div className="min-h-screen bg-gray-900">
      {/* Sidebar */}
      <div className="fixed inset-y-0 left-0 w-64 bg-gray-800 border-r border-gray-700">
        <div className="flex items-center gap-2 px-6 py-4 border-b border-gray-700">
          <Shield className="w-8 h-8 text-green-500" />
          <span className="text-xl font-bold text-white">Neo Service Layer</span>
        </div>

        <nav className="mt-6 px-3">
          {navigation.map((item) => {
            const isActive = location.pathname === item.href;
            return (
              <Link
                key={item.name}
                to={item.href}
                className={`flex items-center gap-3 px-3 py-2 rounded-lg mb-1 transition-colors ${
                  isActive
                    ? 'bg-green-600 text-white'
                    : 'text-gray-300 hover:bg-gray-700 hover:text-white'
                }`}
              >
                <item.icon className="w-5 h-5" />
                {item.name}
              </Link>
            );
          })}
        </nav>

        <div className="absolute bottom-0 left-0 right-0 p-4 border-t border-gray-700">
          <div className="flex items-center justify-between">
            <div className="text-sm text-gray-400 truncate">
              {user?.address?.slice(0, 8)}...{user?.address?.slice(-6)}
            </div>
            <button
              onClick={logout}
              className="p-2 text-gray-400 hover:text-white transition-colors"
            >
              <LogOut className="w-5 h-5" />
            </button>
          </div>
        </div>
      </div>

      {/* Main content */}
      <div className="pl-64">
        <main className="p-8">{children}</main>
      </div>
    </div>
  );
}
