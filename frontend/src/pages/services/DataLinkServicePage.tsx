import React, { useEffect, useState } from 'react';
import { FiPlus, FiTrash2, FiRefreshCw, FiDatabase, FiGlobe, FiLink } from 'react-icons/fi';
import { Card, Button, Input, Modal, Badge } from '@/components/common';
import toast from 'react-hot-toast';

interface Connection {
  id: string;
  name: string;
  sourceType: 'api' | 'database' | 'blockchain';
  sourceUrl: string;
  status: 'active' | 'error' | 'syncing';
  lastSync?: string;
  recordCount: number;
}

export function DataLinkServicePage() {
  const [connections, setConnections] = useState<Connection[]>([]);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [formData, setFormData] = useState({
    name: '',
    sourceType: 'api' as 'api' | 'database' | 'blockchain',
    sourceUrl: '',
  });

  const handleCreate = async () => {
    if (!formData.name || !formData.sourceUrl) {
      toast.error('Name and source URL are required');
      return;
    }

    setIsSubmitting(true);
    try {
      toast.success('Connection created');
      setShowCreateModal(false);
      setFormData({ name: '', sourceType: 'api', sourceUrl: '' });
    } catch (e) {
      toast.error('Failed to create connection');
    }
    setIsSubmitting(false);
  };

  const handleSync = async (id: string) => {
    toast.success('Sync started');
  };

  const getSourceIcon = (type: string) => {
    switch (type) {
      case 'api': return <FiGlobe className="w-4 h-4" />;
      case 'database': return <FiDatabase className="w-4 h-4" />;
      case 'blockchain': return <FiLink className="w-4 h-4" />;
      default: return <FiGlobe className="w-4 h-4" />;
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
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-surface-900">DataLink Service</h1>
          <p className="text-surface-500 mt-1">
            Connect and synchronize data from external sources
          </p>
        </div>
        <Button leftIcon={<FiPlus className="w-4 h-4" />} onClick={() => setShowCreateModal(true)}>
          Add Connection
        </Button>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <p className="text-sm text-surface-500">Total Connections</p>
          <p className="text-2xl font-bold text-surface-900">{connections.length}</p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Active</p>
          <p className="text-2xl font-bold text-green-600">
            {connections.filter(c => c.status === 'active').length}
          </p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Total Records</p>
          <p className="text-2xl font-bold text-surface-900">
            {connections.reduce((a, c) => a + c.recordCount, 0).toLocaleString()}
          </p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Errors</p>
          <p className="text-2xl font-bold text-red-600">
            {connections.filter(c => c.status === 'error').length}
          </p>
        </Card>
      </div>

      {/* Connections List */}
      <Card padding="none">
        <div className="p-4 border-b border-surface-200">
          <h3 className="font-semibold text-surface-900">Data Connections</h3>
        </div>
        <div className="divide-y divide-surface-200">
          {connections.length === 0 ? (
            <div className="p-8 text-center text-surface-400">
              No connections configured. Add one to start syncing data.
            </div>
          ) : (
            connections.map((conn) => (
              <div key={conn.id} className="p-4 hover:bg-surface-50 transition-colors">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className="p-2 bg-surface-100 rounded-lg">
                      {getSourceIcon(conn.sourceType)}
                    </div>
                    <div>
                      <div className="flex items-center gap-2">
                        <span className="font-medium text-surface-900">{conn.name}</span>
                        {getStatusBadge(conn.status)}
                        <Badge size="sm" variant="default">{conn.sourceType}</Badge>
                      </div>
                      <p className="text-xs text-surface-400 mt-1 font-mono truncate max-w-md">
                        {conn.sourceUrl}
                      </p>
                      <p className="text-xs text-surface-400">
                        Records: {conn.recordCount.toLocaleString()}
                        {conn.lastSync && ` • Last sync: ${new Date(conn.lastSync).toLocaleString()}`}
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() => handleSync(conn.id)}
                      leftIcon={<FiRefreshCw className="w-4 h-4" />}
                    >
                      Sync
                    </Button>
                    <button className="p-2 text-surface-400 hover:text-red-600 transition-colors">
                      <FiTrash2 className="w-4 h-4" />
                    </button>
                  </div>
                </div>
              </div>
            ))
          )}
        </div>
      </Card>

      {/* Create Modal */}
      <Modal
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        title="Add Data Connection"
        size="lg"
      >
        <div className="space-y-4">
          <Input
            label="Connection Name"
            placeholder="My Data Source"
            value={formData.name}
            onChange={(e) => setFormData({ ...formData, name: e.target.value })}
          />
          <div>
            <label className="block text-sm font-medium text-surface-700 mb-1">Source Type</label>
            <select
              className="w-full px-4 py-2.5 border border-surface-300 rounded-lg"
              value={formData.sourceType}
              onChange={(e) => setFormData({ ...formData, sourceType: e.target.value as any })}
            >
              <option value="api">REST API</option>
              <option value="database">Database</option>
              <option value="blockchain">Blockchain</option>
            </select>
          </div>
          <Input
            label="Source URL"
            placeholder="https://api.example.com/data"
            value={formData.sourceUrl}
            onChange={(e) => setFormData({ ...formData, sourceUrl: e.target.value })}
          />
          <div className="flex gap-3 pt-4">
            <Button variant="secondary" onClick={() => setShowCreateModal(false)} className="flex-1">
              Cancel
            </Button>
            <Button onClick={handleCreate} isLoading={isSubmitting} className="flex-1">
              Create Connection
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
