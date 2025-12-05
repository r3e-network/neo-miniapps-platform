import React from 'react';
import { FiCode, FiPlay, FiClock } from 'react-icons/fi';
import { Card, Badge } from '@/components/common';

interface CRECardProps {
  func: {
    id: string;
    name: string;
    status: 'deployed' | 'paused' | 'error';
    executions: number;
    lastExecution?: string;
  };
  onExecute?: () => void;
}

export function CRECard({ func, onExecute }: CRECardProps) {
  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'deployed': return <Badge variant="success">Deployed</Badge>;
      case 'paused': return <Badge variant="warning">Paused</Badge>;
      case 'error': return <Badge variant="error">Error</Badge>;
      default: return <Badge>{status}</Badge>;
    }
  };

  return (
    <Card className="hover:shadow-md transition-shadow">
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-orange-100 rounded-lg">
            <FiCode className="w-5 h-5 text-orange-600" />
          </div>
          <div>
            <h3 className="font-medium text-surface-900">{func.name}</h3>
            <p className="text-xs text-surface-400 font-mono">{func.id.slice(0, 16)}...</p>
          </div>
        </div>
        {getStatusBadge(func.status)}
      </div>
      <div className="mt-4 flex items-center justify-between">
        <div className="flex items-center gap-4 text-sm text-surface-500">
          <span className="flex items-center gap-1">
            <FiPlay className="w-4 h-4" />
            {func.executions} runs
          </span>
          {func.lastExecution && (
            <span className="flex items-center gap-1">
              <FiClock className="w-4 h-4" />
              {new Date(func.lastExecution).toLocaleDateString()}
            </span>
          )}
        </div>
      </div>
    </Card>
  );
}
