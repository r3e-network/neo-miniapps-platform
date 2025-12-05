import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  FiActivity,
  FiDollarSign,
  FiKey,
  FiLock,
  FiTrendingUp,
  FiArrowRight,
  FiCheckCircle,
  FiAlertCircle,
  FiClock,
} from 'react-icons/fi';
import { Card, CardHeader, Badge } from '@/components/common';
import { useAuthStore } from '@/stores/authStore';
import { useGasBankStore } from '@/stores/gasbankStore';
import { useSecretsStore } from '@/stores/secretsStore';
import { useAPIKeysStore } from '@/stores/apiKeysStore';
import type { DashboardStats, ActivityItem } from '@/types';
import apiClient from '@/api/client';

export function DashboardPage() {
  const { account, wallet } = useAuthStore();
  const { account: gasAccount, fetchAccount } = useGasBankStore();
  const { secrets, fetchSecrets } = useSecretsStore();
  const { apiKeys, fetchAPIKeys } = useAPIKeysStore();
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [recentActivity, setRecentActivity] = useState<ActivityItem[]>([]);

  useEffect(() => {
    fetchAccount();
    fetchSecrets();
    fetchAPIKeys();
    fetchDashboardStats();
  }, []);

  const fetchDashboardStats = async () => {
    try {
      const response = await apiClient.get<DashboardStats>('/dashboard/stats');
      if (response.success && response.data) {
        setStats(response.data);
        setRecentActivity(response.data.recentActivity || []);
      }
    } catch (error) {
      // Use mock data for demo
      setStats({
        totalRequests: 12847,
        successRate: 99.2,
        activeServices: 5,
        gasBalance: gasAccount?.balance || '0',
        secretsCount: secrets.length,
        apiKeysCount: apiKeys.length,
        recentActivity: [],
      });
    }
  };

  const statCards = [
    {
      title: 'Total Requests',
      value: stats?.totalRequests?.toLocaleString() || '0',
      change: '+12.5%',
      icon: FiActivity,
      color: 'text-blue-600',
      bgColor: 'bg-blue-100',
    },
    {
      title: 'Success Rate',
      value: `${stats?.successRate || 0}%`,
      change: '+0.3%',
      icon: FiTrendingUp,
      color: 'text-green-600',
      bgColor: 'bg-green-100',
    },
    {
      title: 'GAS Balance',
      value: gasAccount?.balance || wallet.balance || '0',
      change: 'GAS',
      icon: FiDollarSign,
      color: 'text-yellow-600',
      bgColor: 'bg-yellow-100',
    },
    {
      title: 'Active Secrets',
      value: secrets.length.toString(),
      change: `${apiKeys.length} API Keys`,
      icon: FiLock,
      color: 'text-purple-600',
      bgColor: 'bg-purple-100',
    },
  ];

  const quickActions = [
    { name: 'Create API Key', href: '/apikeys', icon: FiKey },
    { name: 'Add Secret', href: '/secrets', icon: FiLock },
    { name: 'Deposit GAS', href: '/gasbank', icon: FiDollarSign },
    { name: 'Configure Service', href: '/services', icon: FiActivity },
  ];

  const getActivityIcon = (status: string) => {
    switch (status) {
      case 'success':
        return <FiCheckCircle className="w-4 h-4 text-green-500" />;
      case 'error':
        return <FiAlertCircle className="w-4 h-4 text-red-500" />;
      default:
        return <FiClock className="w-4 h-4 text-yellow-500" />;
    }
  };

  return (
    <div className="space-y-6">
      {/* Welcome Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-surface-900">
            Welcome back, {account?.name || 'User'}
          </h1>
          <p className="text-surface-500 mt-1">
            Here's what's happening with your services today.
          </p>
        </div>
        <Badge variant={account?.tier === 'pro' ? 'info' : 'default'}>
          {account?.tier || 'Free'} Plan
        </Badge>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {statCards.map((stat) => (
          <Card key={stat.title} className="relative overflow-hidden">
            <div className="flex items-start justify-between">
              <div>
                <p className="text-sm text-surface-500">{stat.title}</p>
                <p className="text-2xl font-bold text-surface-900 mt-1">{stat.value}</p>
                <p className="text-xs text-surface-400 mt-1">{stat.change}</p>
              </div>
              <div className={`p-3 rounded-lg ${stat.bgColor}`}>
                <stat.icon className={`w-5 h-5 ${stat.color}`} />
              </div>
            </div>
          </Card>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Quick Actions */}
        <Card>
          <CardHeader title="Quick Actions" description="Common tasks at your fingertips" />
          <div className="space-y-2">
            {quickActions.map((action) => (
              <Link
                key={action.name}
                to={action.href}
                className="flex items-center justify-between p-3 rounded-lg hover:bg-surface-50 transition-colors group"
              >
                <div className="flex items-center gap-3">
                  <div className="p-2 bg-primary-100 rounded-lg">
                    <action.icon className="w-4 h-4 text-primary-600" />
                  </div>
                  <span className="text-sm font-medium text-surface-700">{action.name}</span>
                </div>
                <FiArrowRight className="w-4 h-4 text-surface-400 group-hover:text-primary-600 transition-colors" />
              </Link>
            ))}
          </div>
        </Card>

        {/* Recent Activity */}
        <Card className="lg:col-span-2">
          <CardHeader
            title="Recent Activity"
            description="Latest events across your services"
            action={
              <Link to="/activity" className="text-sm text-primary-600 hover:text-primary-700">
                View all
              </Link>
            }
          />
          <div className="space-y-3">
            {recentActivity.length > 0 ? (
              recentActivity.slice(0, 5).map((activity) => (
                <div
                  key={activity.id}
                  className="flex items-center gap-3 p-3 rounded-lg bg-surface-50"
                >
                  {getActivityIcon(activity.status)}
                  <div className="flex-1 min-w-0">
                    <p className="text-sm text-surface-700 truncate">{activity.description}</p>
                    <p className="text-xs text-surface-400">{activity.timestamp}</p>
                  </div>
                  {activity.serviceType && (
                    <Badge size="sm">{activity.serviceType}</Badge>
                  )}
                </div>
              ))
            ) : (
              <div className="text-center py-8 text-surface-400">
                <FiActivity className="w-8 h-8 mx-auto mb-2 opacity-50" />
                <p className="text-sm">No recent activity</p>
                <p className="text-xs mt-1">Start using services to see activity here</p>
              </div>
            )}
          </div>
        </Card>
      </div>

      {/* Services Overview */}
      <Card>
        <CardHeader
          title="Services Overview"
          description="Status of your configured services"
          action={
            <Link to="/services" className="text-sm text-primary-600 hover:text-primary-700">
              Manage services
            </Link>
          }
        />
        <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-4">
          {['Oracle', 'VRF', 'DataFeeds', 'Automation', 'Secrets', 'GasBank'].map((service) => (
            <Link
              key={service}
              to={`/services/${service.toLowerCase()}`}
              className="p-4 rounded-lg border border-surface-200 hover:border-primary-300 hover:bg-primary-50 transition-all text-center group"
            >
              <div className="w-10 h-10 mx-auto mb-2 rounded-lg bg-surface-100 group-hover:bg-primary-100 flex items-center justify-center transition-colors">
                <FiActivity className="w-5 h-5 text-surface-500 group-hover:text-primary-600" />
              </div>
              <p className="text-sm font-medium text-surface-700">{service}</p>
              <p className="text-xs text-green-600 mt-1">Active</p>
            </Link>
          ))}
        </div>
      </Card>
    </div>
  );
}
