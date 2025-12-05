import React from 'react';
import { FiCode, FiPlay, FiCheckCircle, FiPercent } from 'react-icons/fi';
import { Card } from '@/components/common';

interface CREStatsProps {
  deployedFunctions: number;
  totalExecutions: number;
  successRate: number;
  running: number;
}

export function CREStats({ deployedFunctions, totalExecutions, successRate, running }: CREStatsProps) {
  const stats = [
    { label: 'Functions', value: deployedFunctions, icon: FiCode, color: 'text-orange-600' },
    { label: 'Executions', value: totalExecutions, icon: FiPlay, color: 'text-blue-600' },
    { label: 'Success Rate', value: `${successRate}%`, icon: FiCheckCircle, color: 'text-green-600' },
    { label: 'Running', value: running, icon: FiPercent, color: 'text-yellow-600' },
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
