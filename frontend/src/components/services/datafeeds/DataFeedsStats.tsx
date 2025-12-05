import React from 'react';
import { FiTrendingUp, FiCheckCircle, FiClock, FiDatabase } from 'react-icons/fi';
import { Card } from '@/components/common';

interface DataFeedsStatsProps {
  totalFeeds: number;
  activeFeeds: number;
  avgUpdateInterval: string;
  totalUpdates: number;
}

export function DataFeedsStats({ totalFeeds, activeFeeds, avgUpdateInterval, totalUpdates }: DataFeedsStatsProps) {
  const stats = [
    { label: 'Total Feeds', value: totalFeeds, icon: FiTrendingUp, color: 'text-green-600' },
    { label: 'Active', value: activeFeeds, icon: FiCheckCircle, color: 'text-blue-600' },
    { label: 'Avg Interval', value: avgUpdateInterval, icon: FiClock, color: 'text-purple-600' },
    { label: 'Total Updates', value: totalUpdates.toLocaleString(), icon: FiDatabase, color: 'text-surface-600' },
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
