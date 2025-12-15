import { Link, Outlet, useLocation } from 'react-router-dom';
import { Home, Server, Key, Wallet, Zap, Settings, LogOut, Shield, Shuffle, Moon, Sun, Dices } from 'lucide-react';
import { useAuthStore } from '../stores/auth';
import { useThemeStore } from '../stores/theme';
import { api } from '../api/client';
import { useEffect } from 'react';

const navigation = [
  { name: 'Dashboard', href: '/', icon: Home },
  { name: 'Services', href: '/services', icon: Server },
  { name: 'NeoVault', href: '/neovault', icon: Shuffle },
  { name: 'NeoRand', href: '/neorand', icon: Dices },
  { name: 'NeoStore', href: '/neostore', icon: Key },
  { name: 'Gas Bank', href: '/gasbank', icon: Wallet },
  { name: 'NeoFlow', href: '/neoflow', icon: Zap },
  { name: 'Settings', href: '/settings', icon: Settings },
];

export function Layout() {
  const location = useLocation();
  const { user, logout } = useAuthStore();
  const { theme, toggleTheme } = useThemeStore();

  // Apply theme to document root
  useEffect(() => {
    if (theme === 'light') {
      document.documentElement.classList.remove('dark');
      document.documentElement.classList.add('light');
    } else {
      document.documentElement.classList.remove('light');
      document.documentElement.classList.add('dark');
    }
  }, [theme]);

  const isDark = theme === 'dark';

  const handleLogout = async () => {
    try {
      await api.logout();
    } catch (err) {
      console.error('Logout error:', err);
    } finally {
      logout();
    }
  };

  return (
    <div className={`min-h-screen ${isDark ? 'bg-gray-900' : 'bg-gray-50'}`}>
      {/* Sidebar */}
      <div className={`fixed inset-y-0 left-0 w-64 ${isDark ? 'bg-gray-800 border-gray-700' : 'bg-white border-gray-200'} border-r`}>
        <div className={`flex items-center gap-2 px-6 py-4 border-b ${isDark ? 'border-gray-700' : 'border-gray-200'}`}>
          <Shield className="w-8 h-8 text-green-500" />
          <span className={`text-xl font-bold ${isDark ? 'text-white' : 'text-gray-900'}`}>Neo Service Layer</span>
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
                    : isDark
                    ? 'text-gray-300 hover:bg-gray-700 hover:text-white'
                    : 'text-gray-700 hover:bg-gray-100 hover:text-gray-900'
                }`}
              >
                <item.icon className="w-5 h-5" />
                {item.name}
              </Link>
            );
          })}
        </nav>

        <div className={`absolute bottom-0 left-0 right-0 p-4 border-t ${isDark ? 'border-gray-700' : 'border-gray-200'}`}>
          <div className="flex items-center justify-between mb-3">
            <div className={`text-sm ${isDark ? 'text-gray-400' : 'text-gray-600'} truncate`}>
              {user?.address?.slice(0, 8)}...{user?.address?.slice(-6)}
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={toggleTheme}
                className={`p-2 ${isDark ? 'text-gray-400 hover:text-white' : 'text-gray-600 hover:text-gray-900'} transition-colors`}
                title={`Switch to ${isDark ? 'light' : 'dark'} mode`}
              >
                {isDark ? <Sun className="w-5 h-5" /> : <Moon className="w-5 h-5" />}
              </button>
              <button
                onClick={handleLogout}
                className={`p-2 ${isDark ? 'text-gray-400 hover:text-white' : 'text-gray-600 hover:text-gray-900'} transition-colors`}
                title="Logout"
              >
                <LogOut className="w-5 h-5" />
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Main content */}
      <div className="pl-64">
        <main className="p-8">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
