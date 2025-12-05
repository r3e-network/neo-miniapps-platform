import React from 'react';
import { FiGlobe, FiActivity, FiClock } from 'react-icons/fi';
import { Card, Badge } from '@/components/common';

interface OracleCardProps {
  feed: {
    id: string;
    name: string;
    url: string;
    method: string;
    status: 'active' | 'paused' | 'error';
    lastFetchAt?: string;
  };
  onClick?: () => void;
}

export function OracleCard({ feed, onClick }: OracleCardProps) {
  const getStatusColor = (status: string) => {
    switch (status) {
      case 'active': return 'success';
      case 'paused': return 'warning';
      case 'error': return 'error';
      default: return 'default';
    }
  };

  return (
    <Card
      className="cursor-pointer hover:shadow-md transition-shadow"
      onClick={onClick}
    >
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-blue-100 rounded-lg">
            <FiGlobe className="w-5 h-5 text-blue-600" />
          </div>
          <div>
            <h3 className="font-medium text-surface-900">{feed.name}</h3>
            <p className="text-xs text-surface-400 font-mono truncate max-w-[200px]">
              {feed.url}
            </p>
          </div>
        </div>
        <Badge variant={getStatusColor(feed.status)}>{feed.status}</Badge>
      </div>
      <div className="mt-4 flex items-center gap-4 text-sm text-surface-500">
        <span className="flex items-center gap-1">
          <FiActivity className="w-4 h-4" />
          {feed.method}
        </span>
        {feed.lastFetchAt && (
          <span className="flex items-center gap-1">
            <FiClock className="w-4 h-4" />
            {new Date(feed.lastFetchAt).toLocaleTimeString()}
          </span>
        )}
      </div>
    </Card>
  );
}
