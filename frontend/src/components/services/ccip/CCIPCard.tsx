import React from 'react';
import { FiSend, FiArrowRight, FiClock } from 'react-icons/fi';
import { Card, Badge } from '@/components/common';

interface CCIPCardProps {
  message: {
    id: string;
    sourceChain: string;
    destChain: string;
    receiver: string;
    status: 'pending' | 'confirmed' | 'delivered' | 'failed';
    createdAt: string;
  };
  onClick?: () => void;
}

export function CCIPCard({ message, onClick }: CCIPCardProps) {
  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'pending': return <Badge variant="warning">Pending</Badge>;
      case 'confirmed': return <Badge variant="info">Confirmed</Badge>;
      case 'delivered': return <Badge variant="success">Delivered</Badge>;
      case 'failed': return <Badge variant="error">Failed</Badge>;
      default: return <Badge>{status}</Badge>;
    }
  };

  const getChainName = (chain: string) => {
    const names: Record<string, string> = {
      neo: 'Neo N3', ethereum: 'Ethereum', polygon: 'Polygon', bsc: 'BNB Chain',
    };
    return names[chain] || chain;
  };

  return (
    <Card className="cursor-pointer hover:shadow-md transition-shadow" onClick={onClick}>
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-indigo-100 rounded-lg">
            <FiSend className="w-5 h-5 text-indigo-600" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <span className="font-medium text-surface-900">{getChainName(message.sourceChain)}</span>
              <FiArrowRight className="w-4 h-4 text-surface-400" />
              <span className="font-medium text-surface-900">{getChainName(message.destChain)}</span>
            </div>
            <p className="text-xs text-surface-400 font-mono mt-1">
              To: {message.receiver.slice(0, 16)}...
            </p>
          </div>
        </div>
        {getStatusBadge(message.status)}
      </div>
      <div className="mt-3 flex items-center gap-2 text-xs text-surface-400">
        <FiClock className="w-3 h-3" />
        {new Date(message.createdAt).toLocaleString()}
      </div>
    </Card>
  );
}
