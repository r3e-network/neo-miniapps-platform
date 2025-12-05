import React from 'react';
import { FiShield, FiFileText, FiClock } from 'react-icons/fi';
import { Card, Badge } from '@/components/common';

interface DTACardProps {
  certificate: {
    id: string;
    dataHash: string;
    dataType: string;
    source: string;
    status: 'valid' | 'expired' | 'revoked';
    issuedAt: string;
    validUntil?: string;
  };
  onClick?: () => void;
}

export function DTACard({ certificate, onClick }: DTACardProps) {
  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'valid': return <Badge variant="success">Valid</Badge>;
      case 'expired': return <Badge variant="warning">Expired</Badge>;
      case 'revoked': return <Badge variant="error">Revoked</Badge>;
      default: return <Badge>{status}</Badge>;
    }
  };

  return (
    <Card className="cursor-pointer hover:shadow-md transition-shadow" onClick={onClick}>
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-green-100 rounded-lg">
            <FiShield className="w-5 h-5 text-green-600" />
          </div>
          <div>
            <h3 className="font-medium text-surface-900 font-mono text-sm">
              {certificate.id.slice(0, 16)}...
            </h3>
            <p className="text-xs text-surface-400">{certificate.dataType}</p>
          </div>
        </div>
        {getStatusBadge(certificate.status)}
      </div>
      <div className="mt-3 p-2 bg-surface-50 rounded font-mono text-xs text-surface-600 truncate">
        Hash: {certificate.dataHash.slice(0, 32)}...
      </div>
      <div className="mt-3 flex items-center gap-4 text-xs text-surface-400">
        <span className="flex items-center gap-1">
          <FiFileText className="w-3 h-3" />
          {certificate.source}
        </span>
        <span className="flex items-center gap-1">
          <FiClock className="w-3 h-3" />
          {new Date(certificate.issuedAt).toLocaleDateString()}
        </span>
      </div>
    </Card>
  );
}
