import React, { useEffect, useState } from 'react';
import { FiPlus, FiTrash2, FiCopy, FiKey, FiShield, FiClock } from 'react-icons/fi';
import { Card, CardHeader, Button, Input, Modal, Badge } from '@/components/common';
import { useAPIKeysStore } from '@/stores/apiKeysStore';
import type { Permission } from '@/types';
import toast from 'react-hot-toast';

const PERMISSION_GROUPS = [
  {
    name: 'Account',
    permissions: ['account:read', 'account:write'] as Permission[],
  },
  {
    name: 'Services',
    permissions: ['services:read', 'services:write'] as Permission[],
  },
  {
    name: 'Secrets',
    permissions: ['secrets:read', 'secrets:write'] as Permission[],
  },
  {
    name: 'GasBank',
    permissions: ['gasbank:read', 'gasbank:write'] as Permission[],
  },
  {
    name: 'Oracle',
    permissions: ['oracle:read', 'oracle:write'] as Permission[],
  },
  {
    name: 'VRF',
    permissions: ['vrf:read', 'vrf:write'] as Permission[],
  },
];

export function APIKeysPage() {
  const { apiKeys, isLoading, fetchAPIKeys, createAPIKey, deleteAPIKey } = useAPIKeysStore();
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showSecretModal, setShowSecretModal] = useState(false);
  const [newSecretKey, setNewSecretKey] = useState('');
  const [newKey, setNewKey] = useState({
    name: '',
    permissions: [] as Permission[],
    expiresIn: '90',
  });
  const [isCreating, setIsCreating] = useState(false);

  useEffect(() => {
    fetchAPIKeys();
  }, [fetchAPIKeys]);

  const handleCreate = async () => {
    if (!newKey.name) {
      toast.error('Please enter a name for the API key');
      return;
    }

    if (newKey.permissions.length === 0) {
      toast.error('Please select at least one permission');
      return;
    }

    setIsCreating(true);
    const expiresAt = newKey.expiresIn
      ? new Date(Date.now() + parseInt(newKey.expiresIn) * 24 * 60 * 60 * 1000).toISOString()
      : undefined;

    const result = await createAPIKey({
      name: newKey.name,
      permissions: newKey.permissions,
      expiresAt,
    });

    if (result) {
      setNewSecretKey(result.secretKey);
      setShowCreateModal(false);
      setShowSecretModal(true);
      setNewKey({ name: '', permissions: [], expiresIn: '90' });
    } else {
      toast.error('Failed to create API key');
    }
    setIsCreating(false);
  };

  const handleDelete = async (id: string, name: string) => {
    if (!confirm(`Are you sure you want to delete "${name}"? This action cannot be undone.`)) {
      return;
    }

    const success = await deleteAPIKey(id);
    if (success) {
      toast.success('API key deleted');
    } else {
      toast.error('Failed to delete API key');
    }
  };

  const togglePermission = (permission: Permission) => {
    setNewKey((prev) => ({
      ...prev,
      permissions: prev.permissions.includes(permission)
        ? prev.permissions.filter((p) => p !== permission)
        : [...prev.permissions, permission],
    }));
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    toast.success('Copied to clipboard');
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-surface-900">API Keys</h1>
          <p className="text-surface-500 mt-1">
            Manage API keys for programmatic access to Service Layer
          </p>
        </div>
        <Button leftIcon={<FiPlus className="w-4 h-4" />} onClick={() => setShowCreateModal(true)}>
          Create API Key
        </Button>
      </div>

      {/* API Keys List */}
      <Card padding="none">
        <div className="divide-y divide-surface-200">
          {isLoading ? (
            <div className="p-8 text-center text-surface-400">Loading API keys...</div>
          ) : apiKeys.length === 0 ? (
            <div className="p-8 text-center">
              <FiKey className="w-12 h-12 mx-auto text-surface-300 mb-3" />
              <p className="text-surface-500">No API keys yet</p>
              <p className="text-sm text-surface-400 mt-1">
                Create an API key to access Service Layer programmatically
              </p>
            </div>
          ) : (
            apiKeys.map((key) => (
              <div key={key.id} className="p-4 hover:bg-surface-50 transition-colors">
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <div className="flex items-center gap-3">
                      <div className="p-2 bg-primary-100 rounded-lg">
                        <FiKey className="w-5 h-5 text-primary-600" />
                      </div>
                      <div>
                        <h3 className="font-medium text-surface-900">{key.name}</h3>
                        <p className="text-sm text-surface-500 font-mono">{key.keyPrefix}...</p>
                      </div>
                    </div>

                    {/* Permissions */}
                    <div className="mt-3 flex flex-wrap gap-1">
                      {key.permissions.slice(0, 5).map((perm) => (
                        <Badge key={perm} size="sm" variant="default">
                          {perm}
                        </Badge>
                      ))}
                      {key.permissions.length > 5 && (
                        <Badge size="sm" variant="info">
                          +{key.permissions.length - 5} more
                        </Badge>
                      )}
                    </div>

                    {/* Meta info */}
                    <div className="mt-3 flex items-center gap-4 text-xs text-surface-400">
                      <span className="flex items-center gap-1">
                        <FiClock className="w-3 h-3" />
                        Created {new Date(key.createdAt).toLocaleDateString()}
                      </span>
                      {key.lastUsedAt && (
                        <span>Last used {new Date(key.lastUsedAt).toLocaleDateString()}</span>
                      )}
                      <span>{key.usageCount} requests</span>
                      {key.expiresAt && (
                        <span className="text-yellow-600">
                          Expires {new Date(key.expiresAt).toLocaleDateString()}
                        </span>
                      )}
                    </div>
                  </div>

                  <button
                    onClick={() => handleDelete(key.id, key.name)}
                    className="p-2 text-surface-400 hover:text-red-600 transition-colors"
                    title="Delete"
                  >
                    <FiTrash2 className="w-4 h-4" />
                  </button>
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
        title="Create API Key"
        size="lg"
      >
        <div className="space-y-4">
          <Input
            label="Key Name"
            placeholder="Production API Key"
            value={newKey.name}
            onChange={(e) => setNewKey({ ...newKey, name: e.target.value })}
            helperText="A descriptive name to identify this key"
          />

          <div>
            <label className="block text-sm font-medium text-surface-700 mb-2">
              Expiration
            </label>
            <select
              className="w-full px-4 py-2.5 border border-surface-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent"
              value={newKey.expiresIn}
              onChange={(e) => setNewKey({ ...newKey, expiresIn: e.target.value })}
            >
              <option value="30">30 days</option>
              <option value="90">90 days</option>
              <option value="180">180 days</option>
              <option value="365">1 year</option>
              <option value="">Never</option>
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium text-surface-700 mb-2">
              Permissions
            </label>
            <div className="space-y-3 max-h-64 overflow-y-auto">
              {PERMISSION_GROUPS.map((group) => (
                <div key={group.name} className="p-3 bg-surface-50 rounded-lg">
                  <p className="text-sm font-medium text-surface-700 mb-2">{group.name}</p>
                  <div className="flex flex-wrap gap-2">
                    {group.permissions.map((perm) => (
                      <label
                        key={perm}
                        className={`flex items-center gap-2 px-3 py-1.5 rounded-lg cursor-pointer transition-colors ${
                          newKey.permissions.includes(perm)
                            ? 'bg-primary-100 text-primary-700 border border-primary-300'
                            : 'bg-white border border-surface-200 text-surface-600 hover:border-surface-300'
                        }`}
                      >
                        <input
                          type="checkbox"
                          className="sr-only"
                          checked={newKey.permissions.includes(perm)}
                          onChange={() => togglePermission(perm)}
                        />
                        <FiShield className="w-3 h-3" />
                        <span className="text-sm">{perm}</span>
                      </label>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </div>

          <div className="flex gap-3 pt-4">
            <Button variant="secondary" onClick={() => setShowCreateModal(false)} className="flex-1">
              Cancel
            </Button>
            <Button onClick={handleCreate} isLoading={isCreating} className="flex-1">
              Create Key
            </Button>
          </div>
        </div>
      </Modal>

      {/* Secret Key Modal */}
      <Modal
        isOpen={showSecretModal}
        onClose={() => {
          setShowSecretModal(false);
          setNewSecretKey('');
        }}
        title="API Key Created"
      >
        <div className="space-y-4">
          <div className="p-4 bg-yellow-50 border border-yellow-200 rounded-lg">
            <p className="text-sm text-yellow-800">
              <strong>Important:</strong> This is the only time you'll see this secret key. Copy it
              now and store it securely.
            </p>
          </div>

          <div>
            <label className="block text-sm font-medium text-surface-700 mb-1">Secret Key</label>
            <div className="flex items-center gap-2">
              <code className="flex-1 px-4 py-3 bg-surface-100 rounded-lg text-sm font-mono text-surface-900 break-all">
                {newSecretKey}
              </code>
              <button
                onClick={() => copyToClipboard(newSecretKey)}
                className="p-3 bg-primary-100 text-primary-600 rounded-lg hover:bg-primary-200 transition-colors"
              >
                <FiCopy className="w-5 h-5" />
              </button>
            </div>
          </div>

          <Button
            className="w-full"
            onClick={() => {
              setShowSecretModal(false);
              setNewSecretKey('');
            }}
          >
            I've Saved My Key
          </Button>
        </div>
      </Modal>
    </div>
  );
}
