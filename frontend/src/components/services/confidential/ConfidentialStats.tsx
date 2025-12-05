import React from 'react';
import { FiCpu, FiCheckCircle, FiActivity, FiZap } from 'react-icons/fi';
import { Card } from '@/components/common';

interface ConfidentialStatsProps {
  totalComputations: number;
  completed: number;
  running: number;
  totalGasUsed: number;
}

export function ConfidentialStats({ totalComputations, completed, running, totalGasUsed }: ConfidentialStatsProps) {
  const stats = [
    { label: 'Total', value: totalComputations, icon: FiCpu, color: 'text-blue-600' },
    { label: 'Completed', value: completed, icon: FiCheckCircle, color: 'text-green-600' },
    { label: 'Running', value: running, icon: FiActivity, color: 'text-yellow-600' },
    { label: 'Gas Used', value: totalGasUsed, icon: FiZap, color: 'text-purple-600' },
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
