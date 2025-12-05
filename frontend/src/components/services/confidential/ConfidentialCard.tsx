import React from 'react';
import { FiLock, FiClock, FiCpu } from 'react-icons/fi';
import { Card, Badge } from '@/components/common';

interface ConfidentialCardProps {
  computation: {
    id: string;
    status: 'pending' | 'running' | 'completed' | 'failed';
    createdAt: string;
    completedAt?: string;
    gasUsed?: number;
  };
  onClick?: () => void;
}

export function ConfidentialCard({ computation, onClick }: ConfidentialCardProps) {
  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'pending': return <Badge variant="warning">Pending</Badge>;
      case 'running': return <Badge variant="info">Running</Badge>;
      case 'completed': return <Badge variant="success">Completed</Badge>;
      case 'failed': return <Badge variant="error">Failed</Badge>;
      default: return <Badge>{status}</Badge>;
    }
  };

  return (
    <Card className="cursor-pointer hover:shadow-md transition-shadow" onClick={onClick}>
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-blue-100 rounded-lg">
            <FiLock className="w-5 h-5 text-blue-600" />
          </div>
          <div>
            <h3 className="font-medium text-surface-900 font-mono text-sm">
              {computation.id.slice(0, 16)}...
            </h3>
            <p className="text-xs text-surface-400">
              {new Date(computation.createdAt).toLocaleString()}
            </p>
          </div>
        </div>
        {getStatusBadge(computation.status)}
      </div>
      {computation.gasUsed && (
        <div className="mt-3 flex items-center gap-2 text-xs text-surface-500">
          <FiCpu className="w-3 h-3" />
          Gas Used: {computation.gasUsed}
        </div>
      )}
    </Card>
  );
}
