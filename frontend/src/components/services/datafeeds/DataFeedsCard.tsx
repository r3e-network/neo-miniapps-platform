import React from 'react';
import { FiTrendingUp, FiClock, FiDollarSign } from 'react-icons/fi';
import { Card, Badge } from '@/components/common';

interface DataFeedsCardProps {
  feed: {
    id: string;
    name: string;
    price: string;
    change24h?: number;
    updatedAt: string;
    source: string;
  };
  onClick?: () => void;
}

export function DataFeedsCard({ feed, onClick }: DataFeedsCardProps) {
  const isPositive = (feed.change24h || 0) >= 0;

  return (
    <Card className="cursor-pointer hover:shadow-md transition-shadow" onClick={onClick}>
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-green-100 rounded-lg">
            <FiDollarSign className="w-5 h-5 text-green-600" />
          </div>
          <div>
            <h3 className="font-medium text-surface-900">{feed.name}</h3>
            <p className="text-xs text-surface-400">{feed.source}</p>
          </div>
        </div>
        {feed.change24h !== undefined && (
          <Badge variant={isPositive ? 'success' : 'error'}>
            {isPositive ? '+' : ''}{feed.change24h.toFixed(2)}%
          </Badge>
        )}
      </div>
      <div className="mt-4">
        <p className="text-2xl font-bold text-surface-900">${feed.price}</p>
        <p className="text-xs text-surface-400 mt-1 flex items-center gap-1">
          <FiClock className="w-3 h-3" />
          {new Date(feed.updatedAt).toLocaleString()}
        </p>
      </div>
    </Card>
  );
}
