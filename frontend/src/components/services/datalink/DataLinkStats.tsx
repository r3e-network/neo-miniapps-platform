import React from 'react';
import { FiLink, FiCheckCircle, FiDatabase, FiAlertCircle } from 'react-icons/fi';
import { Card } from '@/components/common';

interface DataLinkStatsProps {
  totalConnections: number;
  active: number;
  totalRecords: number;
  errors: number;
}

export function DataLinkStats({ totalConnections, active, totalRecords, errors }: DataLinkStatsProps) {
  const stats = [
    { label: 'Connections', value: totalConnections, icon: FiLink, color: 'text-blue-600' },
    { label: 'Active', value: active, icon: FiCheckCircle, color: 'text-green-600' },
    { label: 'Total Records', value: totalRecords.toLocaleString(), icon: FiDatabase, color: 'text-purple-600' },
    { label: 'Errors', value: errors, icon: FiAlertCircle, color: 'text-red-600' },
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
