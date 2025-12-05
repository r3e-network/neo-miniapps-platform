import React, { useEffect, useState } from 'react';
import { FiShuffle, FiLock, FiClock, FiCheckCircle, FiAlertCircle } from 'react-icons/fi';
import { Card, Button, Input, Modal, Badge } from '@/components/common';
import toast from 'react-hot-toast';

interface MixPool {
  id: string;
  asset: string;
  denomination: string;
  totalDeposits: number;
  anonymitySet: number;
  status: 'active' | 'paused';
}

interface MixRequest {
  id: string;
  poolId: string;
  amount: string;
  status: 'pending' | 'mixing' | 'completed' | 'failed';
  createdAt: string;
  completedAt?: string;
}

export function MixerServicePage() {
  const [pools, setPools] = useState<MixPool[]>([
    { id: 'neo-10', asset: 'NEO', denomination: '10', totalDeposits: 150, anonymitySet: 45, status: 'active' },
    { id: 'neo-100', asset: 'NEO', denomination: '100', totalDeposits: 80, anonymitySet: 28, status: 'active' },
    { id: 'gas-100', asset: 'GAS', denomination: '100', totalDeposits: 200, anonymitySet: 62, status: 'active' },
  ]);
  const [requests, setRequests] = useState<MixRequest[]>([]);
  const [showMixModal, setShowMixModal] = useState(false);
  const [selectedPool, setSelectedPool] = useState<MixPool | null>(null);
  const [destination, setDestination] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleMix = async () => {
    if (!selectedPool || !destination) {
      toast.error('Please select a pool and enter destination');
      return;
    }

    setIsSubmitting(true);
    try {
      // API call to submit mix request
      toast.success('Mix request submitted');
      setShowMixModal(false);
      setDestination('');
      setSelectedPool(null);
    } catch (e) {
      toast.error('Failed to submit mix request');
    }
    setIsSubmitting(false);
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'pending': return <Badge variant="warning">Pending</Badge>;
      case 'mixing': return <Badge variant="info">Mixing</Badge>;
      case 'completed': return <Badge variant="success">Completed</Badge>;
      case 'failed': return <Badge variant="error">Failed</Badge>;
      default: return <Badge>{status}</Badge>;
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-surface-900">Mixer Service</h1>
          <p className="text-surface-500 mt-1">
            Privacy-preserving token mixing with TEE-protected operations
          </p>
        </div>
      </div>

      {/* Privacy Info */}
      <Card className="bg-purple-50 border-purple-200">
        <div className="flex items-start gap-3">
          <FiLock className="w-5 h-5 text-purple-600 mt-0.5" />
          <div>
            <h3 className="font-medium text-purple-900">Privacy Protection</h3>
            <p className="text-sm text-purple-700 mt-1">
              All mixing operations are performed within the TEE. Your transaction links
              are cryptographically broken while maintaining full auditability.
            </p>
          </div>
        </div>
      </Card>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <p className="text-sm text-surface-500">Active Pools</p>
          <p className="text-2xl font-bold text-surface-900">{pools.filter(p => p.status === 'active').length}</p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Total Deposits</p>
          <p className="text-2xl font-bold text-surface-900">{pools.reduce((a, p) => a + p.totalDeposits, 0)}</p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Avg Anonymity Set</p>
          <p className="text-2xl font-bold text-surface-900">
            {Math.round(pools.reduce((a, p) => a + p.anonymitySet, 0) / pools.length)}
          </p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Your Requests</p>
          <p className="text-2xl font-bold text-surface-900">{requests.length}</p>
        </Card>
      </div>

      {/* Mix Pools */}
      <Card padding="none">
        <div className="p-4 border-b border-surface-200">
          <h3 className="font-semibold text-surface-900">Available Pools</h3>
        </div>
        <div className="divide-y divide-surface-200">
          {pools.map((pool) => (
            <div key={pool.id} className="p-4 hover:bg-surface-50 transition-colors">
              <div className="flex items-center justify-between">
                <div>
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-surface-900">
                      {pool.denomination} {pool.asset}
                    </span>
                    <Badge variant={pool.status === 'active' ? 'success' : 'warning'}>
                      {pool.status}
                    </Badge>
                  </div>
                  <div className="flex items-center gap-4 mt-1 text-sm text-surface-500">
                    <span>Deposits: {pool.totalDeposits}</span>
                    <span>Anonymity Set: {pool.anonymitySet}</span>
                  </div>
                </div>
                <Button
                  size="sm"
                  onClick={() => { setSelectedPool(pool); setShowMixModal(true); }}
                  disabled={pool.status !== 'active'}
                  leftIcon={<FiShuffle className="w-4 h-4" />}
                >
                  Mix
                </Button>
              </div>
            </div>
          ))}
        </div>
      </Card>

      {/* Recent Requests */}
      <Card padding="none">
        <div className="p-4 border-b border-surface-200">
          <h3 className="font-semibold text-surface-900">Your Mix Requests</h3>
        </div>
        <div className="divide-y divide-surface-200">
          {requests.length === 0 ? (
            <div className="p-8 text-center text-surface-400">
              No mix requests yet. Select a pool to start mixing.
            </div>
          ) : (
            requests.map((req) => (
              <div key={req.id} className="p-4">
                <div className="flex items-center justify-between">
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-surface-900">{req.amount}</span>
                      {getStatusBadge(req.status)}
                    </div>
                    <p className="text-xs text-surface-400 mt-1">
                      {new Date(req.createdAt).toLocaleString()}
                    </p>
                  </div>
                </div>
              </div>
            ))
          )}
        </div>
      </Card>

      {/* Mix Modal */}
      <Modal
        isOpen={showMixModal}
        onClose={() => { setShowMixModal(false); setSelectedPool(null); setDestination(''); }}
        title="Create Mix Request"
        size="lg"
      >
        <div className="space-y-4">
          {selectedPool && (
            <div className="p-4 bg-surface-50 rounded-lg">
              <p className="text-sm text-surface-500">Selected Pool</p>
              <p className="font-medium text-surface-900">
                {selectedPool.denomination} {selectedPool.asset}
              </p>
              <p className="text-xs text-surface-400 mt-1">
                Anonymity Set: {selectedPool.anonymitySet} participants
              </p>
            </div>
          )}
          <Input
            label="Destination Address"
            placeholder="Enter Neo N3 address"
            value={destination}
            onChange={(e) => setDestination(e.target.value)}
            helperText="The address where mixed tokens will be sent"
          />
          <div className="p-4 bg-yellow-50 rounded-lg">
            <p className="text-sm text-yellow-800">
              <strong>Note:</strong> Mixing may take several hours depending on pool activity.
              You will receive your tokens at the destination address once mixing is complete.
            </p>
          </div>
          <div className="flex gap-3 pt-4">
            <Button variant="secondary" onClick={() => setShowMixModal(false)} className="flex-1">
              Cancel
            </Button>
            <Button onClick={handleMix} isLoading={isSubmitting} className="flex-1">
              Submit Mix Request
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
