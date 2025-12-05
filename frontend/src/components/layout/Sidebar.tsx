import React from 'react';
import { NavLink, useLocation } from 'react-router-dom';
import clsx from 'clsx';
import {
  FiHome,
  FiDatabase,
  FiKey,
  FiLock,
  FiDollarSign,
  FiSettings,
  FiActivity,
  FiCpu,
  FiCloud,
  FiZap,
  FiShuffle,
  FiLink,
} from 'react-icons/fi';

interface NavItem {
  name: string;
  href: string;
  icon: React.ElementType;
  badge?: string;
}

const navigation: NavItem[] = [
  { name: 'Dashboard', href: '/dashboard', icon: FiHome },
  { name: 'Services', href: '/services', icon: FiCpu },
  { name: 'Oracle', href: '/services/oracle', icon: FiCloud },
  { name: 'VRF', href: '/services/vrf', icon: FiShuffle },
  { name: 'DataFeeds', href: '/services/datafeeds', icon: FiActivity },
  { name: 'Automation', href: '/services/automation', icon: FiZap },
  { name: 'Secrets', href: '/secrets', icon: FiLock },
  { name: 'GasBank', href: '/gasbank', icon: FiDollarSign },
  { name: 'API Keys', href: '/apikeys', icon: FiKey },
  { name: 'Integrations', href: '/integrations', icon: FiLink },
  { name: 'Settings', href: '/settings', icon: FiSettings },
];

interface SidebarProps {
  collapsed?: boolean;
}

export function Sidebar({ collapsed = false }: SidebarProps) {
  const location = useLocation();

  return (
    <aside
      className={clsx(
        'fixed left-0 top-16 bottom-0 bg-white border-r border-surface-200 transition-all duration-300 z-40',
        collapsed ? 'w-16' : 'w-64'
      )}
    >
      <nav className="h-full overflow-y-auto py-4">
        <ul className="space-y-1 px-3">
          {navigation.map((item) => {
            const isActive = location.pathname === item.href ||
              (item.href !== '/dashboard' && location.pathname.startsWith(item.href));

            return (
              <li key={item.name}>
                <NavLink
                  to={item.href}
                  className={clsx(
                    'flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-colors',
                    isActive
                      ? 'bg-primary-50 text-primary-700'
                      : 'text-surface-600 hover:bg-surface-50 hover:text-surface-900'
                  )}
                >
                  <item.icon className={clsx('w-5 h-5 flex-shrink-0', isActive && 'text-primary-600')} />
                  {!collapsed && (
                    <>
                      <span className="flex-1">{item.name}</span>
                      {item.badge && (
                        <span className="px-2 py-0.5 text-xs font-medium bg-primary-100 text-primary-700 rounded-full">
                          {item.badge}
                        </span>
                      )}
                    </>
                  )}
                </NavLink>
              </li>
            );
          })}
        </ul>
      </nav>
    </aside>
  );
}
