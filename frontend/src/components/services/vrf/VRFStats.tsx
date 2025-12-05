import React from 'react';
import { FiHash, FiCheckCircle, FiClock, FiPercent } from 'react-icons/fi';
import { Card } from '@/components/common';

interface VRFStatsProps {
  totalRequests: number;
  fulfilledRequests: number;
  pendingRequests: number;
  avgFulfillTime?: number;
}

export function VRFStats({ totalRequests, fulfilledRequests, pendingRequests, avgFulfillTime }: VRFStatsProps) {
  const successRate = totalRequests > 0
    ? Math.round((fulfilledRequests / totalRequests) * 100)
    : 100;

  const stats = [
    { label: 'Total Requests', value: totalRequests, icon: FiHash, color: 'text-purple-600' },
    { label: 'Fulfilled', value: fulfilledRequests, icon: FiCheckCircle, color: 'text-green-600' },
    { label: 'Pending', value: pendingRequests, icon: FiClock, color: 'text-yellow-600' },
    { label: 'Success Rate', value: `${successRate}%`, icon: FiPercent, color: 'text-blue-600' },
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
