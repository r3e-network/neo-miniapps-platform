import React from 'react';
import { FiShield, FiCheckCircle, FiClock, FiXCircle } from 'react-icons/fi';
import { Card } from '@/components/common';

interface DTAStatsProps {
  totalCertificates: number;
  valid: number;
  expired: number;
  revoked: number;
}

export function DTAStats({ totalCertificates, valid, expired, revoked }: DTAStatsProps) {
  const stats = [
    { label: 'Total Certs', value: totalCertificates, icon: FiShield, color: 'text-green-600' },
    { label: 'Valid', value: valid, icon: FiCheckCircle, color: 'text-green-600' },
    { label: 'Expired', value: expired, icon: FiClock, color: 'text-yellow-600' },
    { label: 'Revoked', value: revoked, icon: FiXCircle, color: 'text-red-600' },
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
