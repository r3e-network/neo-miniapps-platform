import React from 'react';
import { FiLink, FiDatabase, FiGlobe, FiRefreshCw } from 'react-icons/fi';
import { Card, Badge } from '@/components/common';

interface DataLinkCardProps {
  connection: {
    id: string;
    name: string;
    sourceType: 'api' | 'database' | 'blockchain';
    sourceUrl: string;
    status: 'active' | 'error' | 'syncing';
    recordCount: number;
    lastSync?: string;
  };
  onSync?: () => void;
}

export function DataLinkCard({ connection, onSync }: DataLinkCardProps) {
  const getSourceIcon = (type: string) => {
    switch (type) {
      case 'api': return <FiGlobe className="w-5 h-5 text-blue-600" />;
      case 'database': return <FiDatabase className="w-5 h-5 text-purple-600" />;
      case 'blockchain': return <FiLink className="w-5 h-5 text-green-600" />;
      default: return <FiGlobe className="w-5 h-5 text-surface-600" />;
    }
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'active': return <Badge variant="success">Active</Badge>;
      case 'error': return <Badge variant="error">Error</Badge>;
      case 'syncing': return <Badge variant="info">Syncing</Badge>;
      default: return <Badge>{status}</Badge>;
    }
  };

  return (
    <Card className="hover:shadow-md transition-shadow">
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-surface-100 rounded-lg">
            {getSourceIcon(connection.sourceType)}
          </div>
          <div>
            <h3 className="font-medium text-surface-900">{connection.name}</h3>
            <p className="text-xs text-surface-400 font-mono truncate max-w-[200px]">
              {connection.sourceUrl}
            </p>
          </div>
        </div>
        {getStatusBadge(connection.status)}
      </div>
      <div className="mt-4 flex items-center justify-between text-sm text-surface-500">
        <span>{connection.recordCount.toLocaleString()} records</span>
        {connection.lastSync && (
          <span className="flex items-center gap-1">
            <FiRefreshCw className="w-3 h-3" />
            {new Date(connection.lastSync).toLocaleDateString()}
          </span>
        )}
      </div>
    </Card>
  );
}
