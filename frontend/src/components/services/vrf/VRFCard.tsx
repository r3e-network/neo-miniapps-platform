import React from 'react';
import { FiHash, FiClock, FiCheckCircle, FiLoader } from 'react-icons/fi';
import { Card, Badge } from '@/components/common';

interface VRFCardProps {
  request: {
    id: string;
    seed: string;
    status: 'pending' | 'fulfilled' | 'failed';
    randomValue?: string;
    createdAt: string;
    fulfilledAt?: string;
  };
  onClick?: () => void;
}

export function VRFCard({ request, onClick }: VRFCardProps) {
  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'pending': return <Badge variant="warning">Pending</Badge>;
      case 'fulfilled': return <Badge variant="success">Fulfilled</Badge>;
      case 'failed': return <Badge variant="error">Failed</Badge>;
      default: return <Badge>{status}</Badge>;
    }
  };

  return (
    <Card
      className="cursor-pointer hover:shadow-md transition-shadow"
      onClick={onClick}
    >
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-purple-100 rounded-lg">
            <FiHash className="w-5 h-5 text-purple-600" />
          </div>
          <div>
            <h3 className="font-medium text-surface-900 font-mono text-sm">
              {request.id.slice(0, 16)}...
            </h3>
            <p className="text-xs text-surface-400">
              Seed: {request.seed.slice(0, 20)}...
            </p>
          </div>
        </div>
        {getStatusBadge(request.status)}
      </div>
      {request.randomValue && (
        <div className="mt-3 p-2 bg-surface-50 rounded font-mono text-xs text-surface-600 truncate">
          {request.randomValue}
        </div>
      )}
      <div className="mt-3 flex items-center gap-4 text-xs text-surface-400">
        <span className="flex items-center gap-1">
          <FiClock className="w-3 h-3" />
          {new Date(request.createdAt).toLocaleString()}
        </span>
      </div>
    </Card>
  );
}
