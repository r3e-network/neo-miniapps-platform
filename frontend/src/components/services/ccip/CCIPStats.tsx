import React from 'react';
import { FiSend, FiCheckCircle, FiClock, FiLink } from 'react-icons/fi';
import { Card } from '@/components/common';

interface CCIPStatsProps {
  activeLanes: number;
  messagesSent: number;
  delivered: number;
  pending: number;
}

export function CCIPStats({ activeLanes, messagesSent, delivered, pending }: CCIPStatsProps) {
  const stats = [
    { label: 'Active Lanes', value: activeLanes, icon: FiLink, color: 'text-indigo-600' },
    { label: 'Messages Sent', value: messagesSent, icon: FiSend, color: 'text-blue-600' },
    { label: 'Delivered', value: delivered, icon: FiCheckCircle, color: 'text-green-600' },
    { label: 'Pending', value: pending, icon: FiClock, color: 'text-yellow-600' },
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
