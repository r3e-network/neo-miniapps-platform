import React from 'react';
import { FiShuffle, FiUsers, FiDollarSign } from 'react-icons/fi';
import { Card, Badge } from '@/components/common';

interface MixerCardProps {
  pool: {
    id: string;
    asset: string;
    denomination: string;
    totalDeposits: number;
    anonymitySet: number;
    status: 'active' | 'paused';
  };
  onMix?: () => void;
}

export function MixerCard({ pool, onMix }: MixerCardProps) {
  return (
    <Card className="hover:shadow-md transition-shadow">
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-purple-100 rounded-lg">
            <FiShuffle className="w-5 h-5 text-purple-600" />
          </div>
          <div>
            <h3 className="font-medium text-surface-900">
              {pool.denomination} {pool.asset}
            </h3>
            <p className="text-xs text-surface-400">Pool ID: {pool.id}</p>
          </div>
        </div>
        <Badge variant={pool.status === 'active' ? 'success' : 'warning'}>
          {pool.status}
        </Badge>
      </div>
      <div className="mt-4 grid grid-cols-2 gap-4">
        <div className="flex items-center gap-2 text-sm">
          <FiDollarSign className="w-4 h-4 text-surface-400" />
          <span className="text-surface-500">Deposits:</span>
          <span className="font-medium">{pool.totalDeposits}</span>
        </div>
        <div className="flex items-center gap-2 text-sm">
          <FiUsers className="w-4 h-4 text-surface-400" />
          <span className="text-surface-500">Anonymity:</span>
          <span className="font-medium">{pool.anonymitySet}</span>
        </div>
      </div>
    </Card>
  );
}
