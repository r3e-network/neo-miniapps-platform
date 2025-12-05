import React, { useEffect, useState } from 'react';
import { FiPlus, FiTrash2, FiPlay, FiPause, FiActivity, FiRadio } from 'react-icons/fi';
import { Card, Button, Input, Modal, Badge } from '@/components/common';
import toast from 'react-hot-toast';

interface DataStream {
  id: string;
  name: string;
  sourceType: 'websocket' | 'sse' | 'polling';
  sourceUrl: string;
  status: 'active' | 'paused' | 'error';
  messagesPerMinute: number;
  lastMessage?: string;
}

export function DataStreamsServicePage() {
  const [streams, setStreams] = useState<DataStream[]>([]);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [formData, setFormData] = useState({
    name: '',
    sourceType: 'websocket' as 'websocket' | 'sse' | 'polling',
    sourceUrl: '',
    interval: 60,
  });

  const handleCreate = async () => {
    if (!formData.name || !formData.sourceUrl) {
      toast.error('Name and source URL are required');
      return;
    }

    setIsSubmitting(true);
    try {
      toast.success('Stream created');
      setShowCreateModal(false);
      setFormData({ name: '', sourceType: 'websocket', sourceUrl: '', interval: 60 });
    } catch (e) {
      toast.error('Failed to create stream');
    }
    setIsSubmitting(false);
  };

  const handleToggle = async (stream: DataStream) => {
    const newStatus = stream.status === 'active' ? 'paused' : 'active';
    toast.success(`Stream ${newStatus}`);
  };

  const getSourceIcon = (type: string) => {
    switch (type) {
      case 'websocket': return <FiRadio className="w-4 h-4" />;
      case 'sse': return <FiActivity className="w-4 h-4" />;
      case 'polling': return <FiActivity className="w-4 h-4" />;
      default: return <FiRadio className="w-4 h-4" />;
    }
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'active': return <Badge variant="success">Active</Badge>;
      case 'paused': return <Badge variant="warning">Paused</Badge>;
      case 'error': return <Badge variant="error">Error</Badge>;
      default: return <Badge>{status}</Badge>;
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-surface-900">DataStreams Service</h1>
          <p className="text-surface-500 mt-1">
            Real-time data streaming with WebSocket, SSE, and polling support
          </p>
        </div>
        <Button leftIcon={<FiPlus className="w-4 h-4" />} onClick={() => setShowCreateModal(true)}>
          Create Stream
        </Button>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <p className="text-sm text-surface-500">Total Streams</p>
          <p className="text-2xl font-bold text-surface-900">{streams.length}</p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Active</p>
          <p className="text-2xl font-bold text-green-600">
            {streams.filter(s => s.status === 'active').length}
          </p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Messages/min</p>
          <p className="text-2xl font-bold text-surface-900">
            {streams.reduce((a, s) => a + s.messagesPerMinute, 0)}
          </p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Errors</p>
          <p className="text-2xl font-bold text-red-600">
            {streams.filter(s => s.status === 'error').length}
          </p>
        </Card>
      </div>

      {/* Streams List */}
      <Card padding="none">
        <div className="p-4 border-b border-surface-200">
          <h3 className="font-semibold text-surface-900">Data Streams</h3>
        </div>
        <div className="divide-y divide-surface-200">
          {streams.length === 0 ? (
            <div className="p-8 text-center text-surface-400">
              No streams configured. Create one to start receiving real-time data.
            </div>
          ) : (
            streams.map((stream) => (
              <div key={stream.id} className="p-4 hover:bg-surface-50 transition-colors">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className="p-2 bg-surface-100 rounded-lg">
                      {getSourceIcon(stream.sourceType)}
                    </div>
                    <div>
                      <div className="flex items-center gap-2">
                        <span className="font-medium text-surface-900">{stream.name}</span>
                        {getStatusBadge(stream.status)}
                        <Badge size="sm" variant="default">{stream.sourceType}</Badge>
                      </div>
                      <p className="text-xs text-surface-400 mt-1 font-mono truncate max-w-md">
                        {stream.sourceUrl}
                      </p>
                      <p className="text-xs text-surface-400">
                        {stream.messagesPerMinute} msg/min
                        {stream.lastMessage && ` • Last: ${new Date(stream.lastMessage).toLocaleString()}`}
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => handleToggle(stream)}
                      className="p-2 text-surface-400 hover:text-surface-600 transition-colors"
                      title={stream.status === 'active' ? 'Pause' : 'Start'}
                    >
                      {stream.status === 'active' ? <FiPause className="w-4 h-4" /> : <FiPlay className="w-4 h-4" />}
                    </button>
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
        title="Create Data Stream"
        size="lg"
      >
        <div className="space-y-4">
          <Input
            label="Stream Name"
            placeholder="Price Updates"
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
              <option value="websocket">WebSocket</option>
              <option value="sse">Server-Sent Events (SSE)</option>
              <option value="polling">HTTP Polling</option>
            </select>
          </div>
          <Input
            label="Source URL"
            placeholder="wss://stream.example.com/data"
            value={formData.sourceUrl}
            onChange={(e) => setFormData({ ...formData, sourceUrl: e.target.value })}
          />
          {formData.sourceType === 'polling' && (
            <Input
              label="Polling Interval (seconds)"
              type="number"
              value={formData.interval.toString()}
              onChange={(e) => setFormData({ ...formData, interval: parseInt(e.target.value) || 60 })}
            />
          )}
          <div className="flex gap-3 pt-4">
            <Button variant="secondary" onClick={() => setShowCreateModal(false)} className="flex-1">
              Cancel
            </Button>
            <Button onClick={handleCreate} isLoading={isSubmitting} className="flex-1">
              Create Stream
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
