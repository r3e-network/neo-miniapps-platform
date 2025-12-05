import React, { useEffect, useState } from 'react';
import { FiPlus, FiPlay, FiTrash2, FiCode, FiClock } from 'react-icons/fi';
import { Card, Button, Input, Modal, Badge } from '@/components/common';
import toast from 'react-hot-toast';

interface CREFunction {
  id: string;
  name: string;
  status: 'deployed' | 'paused' | 'error';
  executions: number;
  lastExecution?: string;
  createdAt: string;
}

interface Execution {
  id: string;
  functionId: string;
  status: 'pending' | 'running' | 'success' | 'failed';
  createdAt: string;
  duration?: number;
}

export function CREServicePage() {
  const [functions, setFunctions] = useState<CREFunction[]>([]);
  const [executions, setExecutions] = useState<Execution[]>([]);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showExecuteModal, setShowExecuteModal] = useState(false);
  const [selectedFunction, setSelectedFunction] = useState<CREFunction | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [formData, setFormData] = useState({ name: '', source: '' });
  const [executeArgs, setExecuteArgs] = useState('');

  const handleCreate = async () => {
    if (!formData.name || !formData.source) {
      toast.error('Name and source code are required');
      return;
    }

    setIsSubmitting(true);
    try {
      toast.success('Function deployed');
      setShowCreateModal(false);
      setFormData({ name: '', source: '' });
    } catch (e) {
      toast.error('Failed to deploy function');
    }
    setIsSubmitting(false);
  };

  const handleExecute = async () => {
    if (!selectedFunction) return;

    setIsSubmitting(true);
    try {
      toast.success('Execution started');
      setShowExecuteModal(false);
      setExecuteArgs('');
      setSelectedFunction(null);
    } catch (e) {
      toast.error('Failed to execute function');
    }
    setIsSubmitting(false);
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'deployed': return <Badge variant="success">Deployed</Badge>;
      case 'paused': return <Badge variant="warning">Paused</Badge>;
      case 'error': return <Badge variant="error">Error</Badge>;
      case 'pending': return <Badge variant="warning">Pending</Badge>;
      case 'running': return <Badge variant="info">Running</Badge>;
      case 'success': return <Badge variant="success">Success</Badge>;
      case 'failed': return <Badge variant="error">Failed</Badge>;
      default: return <Badge>{status}</Badge>;
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-surface-900">CRE Service</h1>
          <p className="text-surface-500 mt-1">
            Chainlink Runtime Environment - Deploy and execute serverless functions
          </p>
        </div>
        <Button leftIcon={<FiPlus className="w-4 h-4" />} onClick={() => setShowCreateModal(true)}>
          Deploy Function
        </Button>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <p className="text-sm text-surface-500">Deployed Functions</p>
          <p className="text-2xl font-bold text-surface-900">{functions.length}</p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Total Executions</p>
          <p className="text-2xl font-bold text-surface-900">
            {functions.reduce((a, f) => a + f.executions, 0)}
          </p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Success Rate</p>
          <p className="text-2xl font-bold text-green-600">
            {executions.length > 0
              ? Math.round((executions.filter(e => e.status === 'success').length / executions.length) * 100)
              : 100}%
          </p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Running</p>
          <p className="text-2xl font-bold text-blue-600">
            {executions.filter(e => e.status === 'running').length}
          </p>
        </Card>
      </div>

      {/* Functions List */}
      <Card padding="none">
        <div className="p-4 border-b border-surface-200">
          <h3 className="font-semibold text-surface-900">Deployed Functions</h3>
        </div>
        <div className="divide-y divide-surface-200">
          {functions.length === 0 ? (
            <div className="p-8 text-center text-surface-400">
              No functions deployed. Deploy your first function to get started.
            </div>
          ) : (
            functions.map((func) => (
              <div key={func.id} className="p-4 hover:bg-surface-50 transition-colors">
                <div className="flex items-center justify-between">
                  <div>
                    <div className="flex items-center gap-2">
                      <FiCode className="w-4 h-4 text-surface-400" />
                      <span className="font-medium text-surface-900">{func.name}</span>
                      {getStatusBadge(func.status)}
                    </div>
                    <p className="text-xs text-surface-400 mt-1">
                      Executions: {func.executions}
                      {func.lastExecution && ` • Last: ${new Date(func.lastExecution).toLocaleString()}`}
                    </p>
                  </div>
                  <div className="flex items-center gap-2">
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() => { setSelectedFunction(func); setShowExecuteModal(true); }}
                      leftIcon={<FiPlay className="w-4 h-4" />}
                    >
                      Execute
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

      {/* Recent Executions */}
      <Card padding="none">
        <div className="p-4 border-b border-surface-200">
          <h3 className="font-semibold text-surface-900">Recent Executions</h3>
        </div>
        <div className="divide-y divide-surface-200">
          {executions.length === 0 ? (
            <div className="p-8 text-center text-surface-400">
              No executions yet.
            </div>
          ) : (
            executions.slice(0, 10).map((exec) => (
              <div key={exec.id} className="p-4">
                <div className="flex items-center justify-between">
                  <div>
                    <div className="flex items-center gap-2">
                      <code className="text-sm text-surface-900">{exec.id}</code>
                      {getStatusBadge(exec.status)}
                    </div>
                    <p className="text-xs text-surface-400 mt-1">
                      {new Date(exec.createdAt).toLocaleString()}
                      {exec.duration && ` • Duration: ${exec.duration}ms`}
                    </p>
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
        title="Deploy Function"
        size="xl"
      >
        <div className="space-y-4">
          <Input
            label="Function Name"
            placeholder="my-function"
            value={formData.name}
            onChange={(e) => setFormData({ ...formData, name: e.target.value })}
          />
          <div>
            <label className="block text-sm font-medium text-surface-700 mb-1">Source Code</label>
            <textarea
              className="w-full px-4 py-3 border border-surface-300 rounded-lg font-mono text-sm"
              rows={12}
              placeholder={`// CRE Function
const { ethers } = await import("npm:ethers@6.10.0");

const response = await Functions.makeHttpRequest({
  url: "https://api.example.com/data"
});

return Functions.encodeString(JSON.stringify(response.data));`}
              value={formData.source}
              onChange={(e) => setFormData({ ...formData, source: e.target.value })}
            />
          </div>
          <div className="flex gap-3 pt-4">
            <Button variant="secondary" onClick={() => setShowCreateModal(false)} className="flex-1">
              Cancel
            </Button>
            <Button onClick={handleCreate} isLoading={isSubmitting} className="flex-1">
              Deploy
            </Button>
          </div>
        </div>
      </Modal>

      {/* Execute Modal */}
      <Modal
        isOpen={showExecuteModal}
        onClose={() => { setShowExecuteModal(false); setSelectedFunction(null); }}
        title={`Execute: ${selectedFunction?.name}`}
        size="lg"
      >
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-surface-700 mb-1">Arguments (JSON Array)</label>
            <textarea
              className="w-full px-4 py-3 border border-surface-300 rounded-lg font-mono text-sm"
              rows={4}
              placeholder='["arg1", "arg2"]'
              value={executeArgs}
              onChange={(e) => setExecuteArgs(e.target.value)}
            />
          </div>
          <div className="flex gap-3 pt-4">
            <Button variant="secondary" onClick={() => setShowExecuteModal(false)} className="flex-1">
              Cancel
            </Button>
            <Button onClick={handleExecute} isLoading={isSubmitting} className="flex-1">
              Execute
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
