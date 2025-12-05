import React from 'react';
import { FiTrendingUp, FiCheckCircle, FiAlertCircle, FiClock } from 'react-icons/fi';
import { Card } from '@/components/common';

interface OracleStatsProps {
  totalFeeds: number;
  activeFeeds: number;
  errorFeeds: number;
  avgResponseTime?: number;
}

export function OracleStats({ totalFeeds, activeFeeds, errorFeeds, avgResponseTime }: OracleStatsProps) {
  const stats = [
    { label: 'Total Feeds', value: totalFeeds, icon: FiTrendingUp, color: 'text-blue-600' },
    { label: 'Active', value: activeFeeds, icon: FiCheckCircle, color: 'text-green-600' },
    { label: 'Errors', value: errorFeeds, icon: FiAlertCircle, color: 'text-red-600' },
    { label: 'Avg Response', value: avgResponseTime ? `${avgResponseTime}ms` : '-', icon: FiClock, color: 'text-purple-600' },
  ];

  return (
    <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
      {stats.map((stat) => (
        <Card key={stat.label}>
          <div className="flex items-center gap-3">
            <div className={`p-2 bg-surface-100 rounded-lg ${stat.color}`}>
              <stat.icon className="w-5 h-5" />
            </div>
            <div>
              <p className="text-sm text-surface-500">{stat.label}</p>
              <p className="text-xl font-bold text-surface-900">{stat.value}</p>
            </div>
          </div>
        </Card>
      ))}
    </div>
  );
}
