import React from 'react';
import { FiZap, FiClock, FiPlay, FiPause } from 'react-icons/fi';
import { Card, Badge } from '@/components/common';

interface AutomationCardProps {
  trigger: {
    id: string;
    name: string;
    type: 'cron' | 'interval' | 'event';
    schedule?: string;
    status: 'active' | 'paused' | 'error';
    lastRun?: string;
    nextRun?: string;
  };
  onToggle?: () => void;
}

export function AutomationCard({ trigger, onToggle }: AutomationCardProps) {
  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'active': return <Badge variant="success">Active</Badge>;
      case 'paused': return <Badge variant="warning">Paused</Badge>;
      case 'error': return <Badge variant="error">Error</Badge>;
      default: return <Badge>{status}</Badge>;
    }
  };

  return (
    <Card className="hover:shadow-md transition-shadow">
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-yellow-100 rounded-lg">
            <FiZap className="w-5 h-5 text-yellow-600" />
          </div>
          <div>
            <h3 className="font-medium text-surface-900">{trigger.name}</h3>
            <p className="text-xs text-surface-400">
              {trigger.type === 'cron' && trigger.schedule}
              {trigger.type === 'interval' && `Every ${trigger.schedule}`}
              {trigger.type === 'event' && 'Event-based'}
            </p>
          </div>
        </div>
        {getStatusBadge(trigger.status)}
      </div>
      <div className="mt-4 flex items-center justify-between text-sm text-surface-500">
        <div className="flex items-center gap-4">
          {trigger.lastRun && (
            <span className="flex items-center gap-1">
              <FiClock className="w-4 h-4" />
              Last: {new Date(trigger.lastRun).toLocaleTimeString()}
            </span>
          )}
        </div>
        <button
          onClick={onToggle}
          className="p-1 text-surface-400 hover:text-surface-600 transition-colors"
        >
          {trigger.status === 'active' ? <FiPause className="w-4 h-4" /> : <FiPlay className="w-4 h-4" />}
        </button>
      </div>
    </Card>
  );
}
