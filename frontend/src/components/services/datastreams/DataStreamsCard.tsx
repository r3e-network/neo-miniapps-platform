import React from 'react';
import { FiRadio, FiActivity, FiClock } from 'react-icons/fi';
import { Card, Badge } from '@/components/common';

interface DataStreamsCardProps {
  stream: {
    id: string;
    name: string;
    sourceType: 'websocket' | 'sse' | 'polling';
    sourceUrl: string;
    status: 'active' | 'paused' | 'error';
    messagesPerMinute: number;
    lastMessage?: string;
  };
  onClick?: () => void;
}

export function DataStreamsCard({ stream, onClick }: DataStreamsCardProps) {
  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'active': return <Badge variant="success">Active</Badge>;
      case 'paused': return <Badge variant="warning">Paused</Badge>;
      case 'error': return <Badge variant="error">Error</Badge>;
      default: return <Badge>{status}</Badge>;
    }
  };

  return (
    <Card className="cursor-pointer hover:shadow-md transition-shadow" onClick={onClick}>
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-cyan-100 rounded-lg">
            <FiRadio className="w-5 h-5 text-cyan-600" />
          </div>
          <div>
            <h3 className="font-medium text-surface-900">{stream.name}</h3>
            <p className="text-xs text-surface-400 font-mono truncate max-w-[200px]">
              {stream.sourceUrl}
            </p>
          </div>
        </div>
        {getStatusBadge(stream.status)}
      </div>
      <div className="mt-4 flex items-center gap-4 text-sm text-surface-500">
        <span className="flex items-center gap-1">
          <FiActivity className="w-4 h-4" />
          {stream.messagesPerMinute} msg/min
        </span>
        <Badge size="sm" variant="default">{stream.sourceType}</Badge>
      </div>
    </Card>
  );
}
