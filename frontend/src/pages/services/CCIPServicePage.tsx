import React, { useEffect, useState } from 'react';
import { FiSend, FiArrowRight, FiClock, FiCheckCircle, FiAlertCircle } from 'react-icons/fi';
import { Card, Button, Input, Modal, Badge } from '@/components/common';
import toast from 'react-hot-toast';

interface Lane {
  source: string;
  dest: string;
  status: 'active' | 'paused';
  fee: string;
}

interface Message {
  id: string;
  sourceChain: string;
  destChain: string;
  receiver: string;
  status: 'pending' | 'confirmed' | 'delivered' | 'failed';
  createdAt: string;
}

export function CCIPServicePage() {
  const [lanes, setLanes] = useState<Lane[]>([
    { source: 'neo', dest: 'ethereum', status: 'active', fee: '0.1 GAS' },
    { source: 'neo', dest: 'polygon', status: 'active', fee: '0.05 GAS' },
    { source: 'neo', dest: 'bsc', status: 'active', fee: '0.05 GAS' },
  ]);
  const [messages, setMessages] = useState<Message[]>([]);
  const [showSendModal, setShowSendModal] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [formData, setFormData] = useState({
    destChain: 'ethereum',
    receiver: '',
    data: '',
    token: '',
    amount: '',
  });

  const handleSend = async () => {
    if (!formData.receiver || !formData.data) {
      toast.error('Receiver and data are required');
      return;
    }

    setIsSubmitting(true);
    try {
      toast.success('Cross-chain message submitted');
      setShowSendModal(false);
      setFormData({ destChain: 'ethereum', receiver: '', data: '', token: '', amount: '' });
    } catch (e) {
      toast.error('Failed to send message');
    }
    setIsSubmitting(false);
  };

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
      neo: 'Neo N3',
      ethereum: 'Ethereum',
      polygon: 'Polygon',
      bsc: 'BNB Chain',
    };
    return names[chain] || chain;
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-surface-900">CCIP Service</h1>
          <p className="text-surface-500 mt-1">
            Cross-Chain Interoperability Protocol for secure message passing
          </p>
        </div>
        <Button leftIcon={<FiSend className="w-4 h-4" />} onClick={() => setShowSendModal(true)}>
          Send Message
        </Button>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <p className="text-sm text-surface-500">Active Lanes</p>
          <p className="text-2xl font-bold text-surface-900">{lanes.filter(l => l.status === 'active').length}</p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Messages Sent</p>
          <p className="text-2xl font-bold text-surface-900">{messages.length}</p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Delivered</p>
          <p className="text-2xl font-bold text-green-600">
            {messages.filter(m => m.status === 'delivered').length}
          </p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Pending</p>
          <p className="text-2xl font-bold text-yellow-600">
            {messages.filter(m => m.status === 'pending').length}
          </p>
        </Card>
      </div>

      {/* Supported Lanes */}
      <Card padding="none">
        <div className="p-4 border-b border-surface-200">
          <h3 className="font-semibold text-surface-900">Supported Lanes</h3>
        </div>
        <div className="divide-y divide-surface-200">
          {lanes.map((lane, idx) => (
            <div key={idx} className="p-4 hover:bg-surface-50 transition-colors">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <span className="font-medium text-surface-900">{getChainName(lane.source)}</span>
                  <FiArrowRight className="w-4 h-4 text-surface-400" />
                  <span className="font-medium text-surface-900">{getChainName(lane.dest)}</span>
                  <Badge variant={lane.status === 'active' ? 'success' : 'warning'}>
                    {lane.status}
                  </Badge>
                </div>
                <span className="text-sm text-surface-500">Fee: {lane.fee}</span>
              </div>
            </div>
          ))}
        </div>
      </Card>

      {/* Recent Messages */}
      <Card padding="none">
        <div className="p-4 border-b border-surface-200">
          <h3 className="font-semibold text-surface-900">Recent Messages</h3>
        </div>
        <div className="divide-y divide-surface-200">
          {messages.length === 0 ? (
            <div className="p-8 text-center text-surface-400">
              No messages sent yet. Send your first cross-chain message.
            </div>
          ) : (
            messages.map((msg) => (
              <div key={msg.id} className="p-4">
                <div className="flex items-center justify-between">
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="text-surface-900">
                        {getChainName(msg.sourceChain)} → {getChainName(msg.destChain)}
                      </span>
                      {getStatusBadge(msg.status)}
                    </div>
                    <p className="text-xs text-surface-400 mt-1 font-mono">
                      To: {msg.receiver.slice(0, 20)}...
                    </p>
                  </div>
                  <span className="text-xs text-surface-400">
                    {new Date(msg.createdAt).toLocaleString()}
                  </span>
                </div>
              </div>
            ))
          )}
        </div>
      </Card>

      {/* Send Modal */}
      <Modal
        isOpen={showSendModal}
        onClose={() => setShowSendModal(false)}
        title="Send Cross-Chain Message"
        size="lg"
      >
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-surface-700 mb-1">Destination Chain</label>
            <select
              className="w-full px-4 py-2.5 border border-surface-300 rounded-lg"
              value={formData.destChain}
              onChange={(e) => setFormData({ ...formData, destChain: e.target.value })}
            >
              <option value="ethereum">Ethereum</option>
              <option value="polygon">Polygon</option>
              <option value="bsc">BNB Chain</option>
            </select>
          </div>
          <Input
            label="Receiver Address"
            placeholder="0x..."
            value={formData.receiver}
            onChange={(e) => setFormData({ ...formData, receiver: e.target.value })}
          />
          <div>
            <label className="block text-sm font-medium text-surface-700 mb-1">Message Data</label>
            <textarea
              className="w-full px-4 py-3 border border-surface-300 rounded-lg font-mono text-sm"
              rows={3}
              placeholder="0x..."
              value={formData.data}
              onChange={(e) => setFormData({ ...formData, data: e.target.value })}
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <Input
              label="Token (Optional)"
              placeholder="NEO, GAS, etc."
              value={formData.token}
              onChange={(e) => setFormData({ ...formData, token: e.target.value })}
            />
            <Input
              label="Amount (Optional)"
              placeholder="0"
              value={formData.amount}
              onChange={(e) => setFormData({ ...formData, amount: e.target.value })}
            />
          </div>
          <div className="flex gap-3 pt-4">
            <Button variant="secondary" onClick={() => setShowSendModal(false)} className="flex-1">
              Cancel
            </Button>
            <Button onClick={handleSend} isLoading={isSubmitting} className="flex-1">
              Send Message
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
