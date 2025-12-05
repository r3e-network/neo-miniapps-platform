import React, { useEffect, useState } from 'react';
import { FiPlus, FiTrash2, FiPlay, FiPause, FiEdit2, FiExternalLink } from 'react-icons/fi';
import { Card, CardHeader, Button, Input, Modal, Badge } from '@/components/common';
import { useServicesStore } from '@/stores/servicesStore';
import type { DataFeed } from '@/types';
import toast from 'react-hot-toast';

export function OracleServicePage() {
  const { dataFeeds, fetchDataFeeds, createDataFeed, updateDataFeed, deleteDataFeed } = useServicesStore();
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [editingFeed, setEditingFeed] = useState<DataFeed | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [formData, setFormData] = useState({
    name: '',
    url: '',
    method: 'GET' as 'GET' | 'POST',
    schedule: '',
    headers: '',
  });

  useEffect(() => {
    fetchDataFeeds();
  }, [fetchDataFeeds]);

  const resetForm = () => {
    setFormData({ name: '', url: '', method: 'GET', schedule: '', headers: '' });
    setEditingFeed(null);
  };

  const handleSubmit = async () => {
    if (!formData.name || !formData.url) {
      toast.error('Name and URL are required');
      return;
    }

    setIsSubmitting(true);
    try {
      const headers = formData.headers
        ? JSON.parse(formData.headers)
        : undefined;

      if (editingFeed) {
        const success = await updateDataFeed(editingFeed.id, {
          name: formData.name,
          url: formData.url,
          method: formData.method,
          schedule: formData.schedule || undefined,
          headers,
        });
        if (success) {
          toast.success('Data feed updated');
          setShowCreateModal(false);
          resetForm();
        }
      } else {
        const success = await createDataFeed({
          name: formData.name,
          url: formData.url,
          method: formData.method,
          schedule: formData.schedule || undefined,
          headers,
        });
        if (success) {
          toast.success('Data feed created');
          setShowCreateModal(false);
          resetForm();
        }
      }
    } catch (e) {
      toast.error('Invalid headers JSON');
    }
    setIsSubmitting(false);
  };

  const handleEdit = (feed: DataFeed) => {
    setEditingFeed(feed);
    setFormData({
      name: feed.name,
      url: feed.url,
      method: feed.method,
      schedule: feed.schedule || '',
      headers: feed.headers ? JSON.stringify(feed.headers, null, 2) : '',
    });
    setShowCreateModal(true);
  };

  const handleDelete = async (id: string, name: string) => {
    if (!confirm(`Delete "${name}"?`)) return;
    const success = await deleteDataFeed(id);
    if (success) toast.success('Data feed deleted');
    else toast.error('Failed to delete');
  };

  const handleToggle = async (feed: DataFeed) => {
    const newStatus = feed.status === 'active' ? 'paused' : 'active';
    const success = await updateDataFeed(feed.id, { status: newStatus });
    if (success) toast.success(`Feed ${newStatus}`);
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
          <h1 className="text-2xl font-bold text-surface-900">Oracle Service</h1>
          <p className="text-surface-500 mt-1">
            Fetch external data with TEE-protected HTTP requests
          </p>
        </div>
        <Button leftIcon={<FiPlus className="w-4 h-4" />} onClick={() => setShowCreateModal(true)}>
          Create Data Feed
        </Button>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <p className="text-sm text-surface-500">Total Feeds</p>
          <p className="text-2xl font-bold text-surface-900">{dataFeeds.length}</p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Active</p>
          <p className="text-2xl font-bold text-green-600">
            {dataFeeds.filter(f => f.status === 'active').length}
          </p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Paused</p>
          <p className="text-2xl font-bold text-yellow-600">
            {dataFeeds.filter(f => f.status === 'paused').length}
          </p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Errors</p>
          <p className="text-2xl font-bold text-red-600">
            {dataFeeds.filter(f => f.status === 'error').length}
          </p>
        </Card>
      </div>

      {/* Data Feeds List */}
      <Card padding="none">
        <div className="p-4 border-b border-surface-200">
          <h3 className="font-semibold text-surface-900">Data Feeds</h3>
        </div>
        <div className="divide-y divide-surface-200">
          {dataFeeds.length === 0 ? (
            <div className="p-8 text-center text-surface-400">
              No data feeds configured. Create one to get started.
            </div>
          ) : (
            dataFeeds.map((feed) => (
              <div key={feed.id} className="p-4 hover:bg-surface-50 transition-colors">
                <div className="flex items-start justify-between">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <h4 className="font-medium text-surface-900">{feed.name}</h4>
                      {getStatusBadge(feed.status)}
                      <Badge size="sm" variant="default">{feed.method}</Badge>
                    </div>
                    <p className="text-sm text-surface-500 font-mono mt-1 truncate">{feed.url}</p>
                    {feed.schedule && (
                      <p className="text-xs text-surface-400 mt-1">Schedule: {feed.schedule}</p>
                    )}
                    {feed.lastFetchAt && (
                      <p className="text-xs text-surface-400">
                        Last fetch: {new Date(feed.lastFetchAt).toLocaleString()}
                      </p>
                    )}
                    {feed.lastError && (
                      <p className="text-xs text-red-500 mt-1">{feed.lastError}</p>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => handleToggle(feed)}
                      className="p-2 text-surface-400 hover:text-surface-600 transition-colors"
                      title={feed.status === 'active' ? 'Pause' : 'Activate'}
                    >
                      {feed.status === 'active' ? <FiPause className="w-4 h-4" /> : <FiPlay className="w-4 h-4" />}
                    </button>
                    <button
                      onClick={() => handleEdit(feed)}
                      className="p-2 text-surface-400 hover:text-surface-600 transition-colors"
                      title="Edit"
                    >
                      <FiEdit2 className="w-4 h-4" />
                    </button>
                    <button
                      onClick={() => handleDelete(feed.id, feed.name)}
                      className="p-2 text-surface-400 hover:text-red-600 transition-colors"
                      title="Delete"
                    >
                      <FiTrash2 className="w-4 h-4" />
                    </button>
                  </div>
                </div>
              </div>
            ))
          )}
        </div>
      </Card>

      {/* Create/Edit Modal */}
      <Modal
        isOpen={showCreateModal}
        onClose={() => { setShowCreateModal(false); resetForm(); }}
        title={editingFeed ? 'Edit Data Feed' : 'Create Data Feed'}
        size="lg"
      >
        <div className="space-y-4">
          <Input
            label="Name"
            placeholder="Price Feed"
            value={formData.name}
            onChange={(e) => setFormData({ ...formData, name: e.target.value })}
          />
          <Input
            label="URL"
            placeholder="https://api.example.com/data"
            value={formData.url}
            onChange={(e) => setFormData({ ...formData, url: e.target.value })}
          />
          <div>
            <label className="block text-sm font-medium text-surface-700 mb-1">Method</label>
            <select
              className="w-full px-4 py-2.5 border border-surface-300 rounded-lg"
              value={formData.method}
              onChange={(e) => setFormData({ ...formData, method: e.target.value as 'GET' | 'POST' })}
            >
              <option value="GET">GET</option>
              <option value="POST">POST</option>
            </select>
          </div>
          <Input
            label="Schedule (Cron)"
            placeholder="*/5 * * * * (every 5 minutes)"
            value={formData.schedule}
            onChange={(e) => setFormData({ ...formData, schedule: e.target.value })}
            helperText="Leave empty for manual triggers only"
          />
          <div>
            <label className="block text-sm font-medium text-surface-700 mb-1">Headers (JSON)</label>
            <textarea
              className="w-full px-4 py-3 border border-surface-300 rounded-lg font-mono text-sm"
              rows={3}
              placeholder='{"Authorization": "Bearer token"}'
              value={formData.headers}
              onChange={(e) => setFormData({ ...formData, headers: e.target.value })}
            />
          </div>
          <div className="flex gap-3 pt-4">
            <Button variant="secondary" onClick={() => { setShowCreateModal(false); resetForm(); }} className="flex-1">
              Cancel
            </Button>
            <Button onClick={handleSubmit} isLoading={isSubmitting} className="flex-1">
              {editingFeed ? 'Update' : 'Create'}
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
