import React, { useEffect, useState } from 'react';
import { FiShuffle, FiCopy, FiCheckCircle, FiClock, FiXCircle } from 'react-icons/fi';
import { Card, CardHeader, Button, Input, Badge } from '@/components/common';
import { useServicesStore } from '@/stores/servicesStore';
import toast from 'react-hot-toast';

export function VRFServicePage() {
  const { vrfRequests, fetchVRFRequests, requestRandomness } = useServicesStore();
  const [seed, setSeed] = useState('');
  const [callbackContract, setCallbackContract] = useState('');
  const [isRequesting, setIsRequesting] = useState(false);

  useEffect(() => {
    fetchVRFRequests();
  }, [fetchVRFRequests]);

  const handleRequest = async () => {
    if (!seed) {
      toast.error('Please enter a seed value');
      return;
    }

    setIsRequesting(true);
    const requestId = await requestRandomness(seed, callbackContract || undefined);
    if (requestId) {
      toast.success(`Randomness requested! ID: ${requestId.slice(0, 8)}...`);
      setSeed('');
      setCallbackContract('');
    } else {
      toast.error('Failed to request randomness');
    }
    setIsRequesting(false);
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    toast.success('Copied to clipboard');
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'fulfilled':
        return <FiCheckCircle className="w-5 h-5 text-green-500" />;
      case 'pending':
        return <FiClock className="w-5 h-5 text-yellow-500" />;
      case 'failed':
        return <FiXCircle className="w-5 h-5 text-red-500" />;
      default:
        return null;
    }
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'fulfilled':
        return <Badge variant="success">Fulfilled</Badge>;
      case 'pending':
        return <Badge variant="warning">Pending</Badge>;
      case 'failed':
        return <Badge variant="error">Failed</Badge>;
      default:
        return <Badge>{status}</Badge>;
    }
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-surface-900">VRF Service</h1>
        <p className="text-surface-500 mt-1">
          Verifiable Random Function for provably fair randomness
        </p>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <p className="text-sm text-surface-500">Total Requests</p>
          <p className="text-2xl font-bold text-surface-900">{vrfRequests.length}</p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Fulfilled</p>
          <p className="text-2xl font-bold text-green-600">
            {vrfRequests.filter(r => r.status === 'fulfilled').length}
          </p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Pending</p>
          <p className="text-2xl font-bold text-yellow-600">
            {vrfRequests.filter(r => r.status === 'pending').length}
          </p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Failed</p>
          <p className="text-2xl font-bold text-red-600">
            {vrfRequests.filter(r => r.status === 'failed').length}
          </p>
        </Card>
      </div>

      {/* Request Form */}
      <Card>
        <CardHeader
          title="Request Randomness"
          description="Generate verifiable random numbers with cryptographic proof"
        />
        <div className="space-y-4">
          <Input
            label="Seed"
            placeholder="Enter a unique seed value"
            value={seed}
            onChange={(e) => setSeed(e.target.value)}
            helperText="A unique value to derive randomness from"
          />
          <Input
            label="Callback Contract (Optional)"
            placeholder="NXV7ZhHiyM1aHXwpVsRZC6BEDrmrLAW3sM"
            value={callbackContract}
            onChange={(e) => setCallbackContract(e.target.value)}
            helperText="Contract to receive the randomness callback"
          />
          <Button
            onClick={handleRequest}
            isLoading={isRequesting}
            leftIcon={<FiShuffle className="w-4 h-4" />}
          >
            Request Randomness
          </Button>
        </div>
      </Card>

      {/* Requests History */}
      <Card padding="none">
        <div className="p-4 border-b border-surface-200">
          <h3 className="font-semibold text-surface-900">Request History</h3>
        </div>
        <div className="divide-y divide-surface-200">
          {vrfRequests.length === 0 ? (
            <div className="p-8 text-center text-surface-400">
              No VRF requests yet. Request randomness to get started.
            </div>
          ) : (
            vrfRequests.map((request) => (
              <div key={request.id} className="p-4 hover:bg-surface-50 transition-colors">
                <div className="flex items-start gap-4">
                  <div className="p-2 bg-surface-100 rounded-lg">
                    {getStatusIcon(request.status)}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-sm text-surface-900">
                        {request.id.slice(0, 16)}...
                      </span>
                      {getStatusBadge(request.status)}
                    </div>
                    <p className="text-sm text-surface-500 mt-1">
                      Seed: <span className="font-mono">{request.seed}</span>
                    </p>
                    {request.randomness && (
                      <div className="mt-2 p-3 bg-green-50 rounded-lg">
                        <p className="text-xs text-green-700 mb-1">Randomness:</p>
                        <div className="flex items-center gap-2">
                          <code className="flex-1 text-xs font-mono text-green-800 break-all">
                            {request.randomness}
                          </code>
                          <button
                            onClick={() => copyToClipboard(request.randomness!)}
                            className="p-1 text-green-600 hover:text-green-800"
                          >
                            <FiCopy className="w-4 h-4" />
                          </button>
                        </div>
                      </div>
                    )}
                    {request.proof && (
                      <div className="mt-2 p-3 bg-blue-50 rounded-lg">
                        <p className="text-xs text-blue-700 mb-1">Proof:</p>
                        <code className="text-xs font-mono text-blue-800 break-all">
                          {request.proof.slice(0, 64)}...
                        </code>
                      </div>
                    )}
                    <p className="text-xs text-surface-400 mt-2">
                      {new Date(request.createdAt).toLocaleString()}
                      {request.fulfilledAt && ` • Fulfilled ${new Date(request.fulfilledAt).toLocaleString()}`}
                    </p>
                  </div>
                </div>
              </div>
            ))
          )}
        </div>
      </Card>
    </div>
  );
}
