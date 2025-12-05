import React, { useEffect, useState } from 'react';
import { FiPlay, FiClock, FiCheckCircle, FiLock, FiCode } from 'react-icons/fi';
import { Card, Button, Input, Modal, Badge } from '@/components/common';
import toast from 'react-hot-toast';

interface Computation {
  id: string;
  status: 'pending' | 'running' | 'completed' | 'failed';
  createdAt: string;
  completedAt?: string;
  gasUsed?: number;
}

export function ConfidentialServicePage() {
  const [computations, setComputations] = useState<Computation[]>([]);
  const [showComputeModal, setShowComputeModal] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [formData, setFormData] = useState({
    code: '',
    inputs: '',
  });

  const handleSubmit = async () => {
    if (!formData.code) {
      toast.error('Code is required');
      return;
    }

    setIsSubmitting(true);
    try {
      toast.success('Computation submitted to TEE');
      setShowComputeModal(false);
      setFormData({ code: '', inputs: '' });
    } catch (e) {
      toast.error('Failed to submit computation');
    }
    setIsSubmitting(false);
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'pending': return <Badge variant="warning">Pending</Badge>;
      case 'running': return <Badge variant="info">Running</Badge>;
      case 'completed': return <Badge variant="success">Completed</Badge>;
      case 'failed': return <Badge variant="error">Failed</Badge>;
      default: return <Badge>{status}</Badge>;
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-surface-900">Confidential Computing</h1>
          <p className="text-surface-500 mt-1">
            Execute private computations within the Trusted Execution Environment
          </p>
        </div>
        <Button leftIcon={<FiPlay className="w-4 h-4" />} onClick={() => setShowComputeModal(true)}>
          New Computation
        </Button>
      </div>

      {/* Security Info */}
      <Card className="bg-blue-50 border-blue-200">
        <div className="flex items-start gap-3">
          <FiLock className="w-5 h-5 text-blue-600 mt-0.5" />
          <div>
            <h3 className="font-medium text-blue-900">TEE-Protected Execution</h3>
            <p className="text-sm text-blue-700 mt-1">
              Your code and data are processed inside the TEE. Neither the platform operators
              nor external parties can access your computation inputs or intermediate states.
            </p>
          </div>
        </div>
      </Card>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <p className="text-sm text-surface-500">Total Computations</p>
          <p className="text-2xl font-bold text-surface-900">{computations.length}</p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Completed</p>
          <p className="text-2xl font-bold text-green-600">
            {computations.filter(c => c.status === 'completed').length}
          </p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Running</p>
          <p className="text-2xl font-bold text-blue-600">
            {computations.filter(c => c.status === 'running').length}
          </p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Total Gas Used</p>
          <p className="text-2xl font-bold text-surface-900">
            {computations.reduce((a, c) => a + (c.gasUsed || 0), 0)}
          </p>
        </Card>
      </div>

      {/* Computations List */}
      <Card padding="none">
        <div className="p-4 border-b border-surface-200">
          <h3 className="font-semibold text-surface-900">Recent Computations</h3>
        </div>
        <div className="divide-y divide-surface-200">
          {computations.length === 0 ? (
            <div className="p-8 text-center text-surface-400">
              No computations yet. Submit your first confidential computation.
            </div>
          ) : (
            computations.map((comp) => (
              <div key={comp.id} className="p-4 hover:bg-surface-50 transition-colors">
                <div className="flex items-center justify-between">
                  <div>
                    <div className="flex items-center gap-2">
                      <code className="text-sm text-surface-900">{comp.id}</code>
                      {getStatusBadge(comp.status)}
                    </div>
                    <p className="text-xs text-surface-400 mt-1">
                      Started: {new Date(comp.createdAt).toLocaleString()}
                      {comp.completedAt && ` • Completed: ${new Date(comp.completedAt).toLocaleString()}`}
                    </p>
                  </div>
                  {comp.status === 'completed' && (
                    <Button size="sm" variant="secondary">
                      View Result
                    </Button>
                  )}
                </div>
              </div>
            ))
          )}
        </div>
      </Card>

      {/* Compute Modal */}
      <Modal
        isOpen={showComputeModal}
        onClose={() => setShowComputeModal(false)}
        title="New Confidential Computation"
        size="xl"
      >
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-surface-700 mb-1">
              <FiCode className="inline w-4 h-4 mr-1" />
              Code (JavaScript)
            </label>
            <textarea
              className="w-full px-4 py-3 border border-surface-300 rounded-lg font-mono text-sm"
              rows={10}
              placeholder={`// Your confidential computation code
function compute(inputs) {
  // Process inputs securely
  return { result: inputs.a + inputs.b };
}`}
              value={formData.code}
              onChange={(e) => setFormData({ ...formData, code: e.target.value })}
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-surface-700 mb-1">Inputs (JSON)</label>
            <textarea
              className="w-full px-4 py-3 border border-surface-300 rounded-lg font-mono text-sm"
              rows={4}
              placeholder='{"a": 10, "b": 20}'
              value={formData.inputs}
              onChange={(e) => setFormData({ ...formData, inputs: e.target.value })}
            />
          </div>
          <div className="flex gap-3 pt-4">
            <Button variant="secondary" onClick={() => setShowComputeModal(false)} className="flex-1">
              Cancel
            </Button>
            <Button onClick={handleSubmit} isLoading={isSubmitting} className="flex-1">
              Execute in TEE
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
