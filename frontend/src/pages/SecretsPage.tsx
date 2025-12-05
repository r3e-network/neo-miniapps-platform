import React, { useEffect, useState } from 'react';
import { FiPlus, FiTrash2, FiEye, FiEyeOff, FiCopy, FiTag, FiSearch } from 'react-icons/fi';
import { Card, CardHeader, Button, Input, Modal, Badge } from '@/components/common';
import { useSecretsStore } from '@/stores/secretsStore';
import toast from 'react-hot-toast';

export function SecretsPage() {
  const { secrets, isLoading, fetchSecrets, createSecret, deleteSecret, getSecretValue } = useSecretsStore();
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [newSecret, setNewSecret] = useState({ name: '', value: '', tags: '' });
  const [revealedSecrets, setRevealedSecrets] = useState<Record<string, string>>({});
  const [isCreating, setIsCreating] = useState(false);

  useEffect(() => {
    fetchSecrets();
  }, [fetchSecrets]);

  const filteredSecrets = secrets.filter(
    (s) =>
      s.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      s.tags.some((t) => t.toLowerCase().includes(searchQuery.toLowerCase()))
  );

  const handleCreate = async () => {
    if (!newSecret.name || !newSecret.value) {
      toast.error('Name and value are required');
      return;
    }

    setIsCreating(true);
    const tags = newSecret.tags.split(',').map((t) => t.trim()).filter(Boolean);
    const success = await createSecret(newSecret.name, newSecret.value, tags);

    if (success) {
      toast.success('Secret created successfully');
      setShowCreateModal(false);
      setNewSecret({ name: '', value: '', tags: '' });
    } else {
      toast.error('Failed to create secret');
    }
    setIsCreating(false);
  };

  const handleDelete = async (name: string) => {
    if (!confirm(`Are you sure you want to delete "${name}"?`)) return;

    const success = await deleteSecret(name);
    if (success) {
      toast.success('Secret deleted');
    } else {
      toast.error('Failed to delete secret');
    }
  };

  const handleReveal = async (name: string) => {
    if (revealedSecrets[name]) {
      setRevealedSecrets((prev) => {
        const next = { ...prev };
        delete next[name];
        return next;
      });
      return;
    }

    const value = await getSecretValue(name);
    if (value) {
      setRevealedSecrets((prev) => ({ ...prev, [name]: value }));
      // Auto-hide after 30 seconds
      setTimeout(() => {
        setRevealedSecrets((prev) => {
          const next = { ...prev };
          delete next[name];
          return next;
        });
      }, 30000);
    } else {
      toast.error('Failed to retrieve secret value');
    }
  };

  const handleCopy = (value: string) => {
    navigator.clipboard.writeText(value);
    toast.success('Copied to clipboard');
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-surface-900">Secrets</h1>
          <p className="text-surface-500 mt-1">
            Securely store and manage sensitive data with TEE protection
          </p>
        </div>
        <Button leftIcon={<FiPlus className="w-4 h-4" />} onClick={() => setShowCreateModal(true)}>
          Add Secret
        </Button>
      </div>

      {/* Search */}
      <Card padding="sm">
        <Input
          placeholder="Search secrets by name or tag..."
          leftIcon={<FiSearch className="w-5 h-5" />}
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
        />
      </Card>

      {/* Secrets List */}
      <Card padding="none">
        <div className="divide-y divide-surface-200">
          {isLoading ? (
            <div className="p-8 text-center text-surface-400">Loading secrets...</div>
          ) : filteredSecrets.length === 0 ? (
            <div className="p-8 text-center">
              <FiLock className="w-12 h-12 mx-auto text-surface-300 mb-3" />
              <p className="text-surface-500">No secrets found</p>
              <p className="text-sm text-surface-400 mt-1">
                {searchQuery ? 'Try a different search term' : 'Create your first secret to get started'}
              </p>
            </div>
          ) : (
            filteredSecrets.map((secret) => (
              <div key={secret.id} className="p-4 hover:bg-surface-50 transition-colors">
                <div className="flex items-start justify-between">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <h3 className="font-medium text-surface-900 font-mono">{secret.name}</h3>
                      <Badge size="sm">v{secret.version}</Badge>
                    </div>

                    {/* Value display */}
                    <div className="mt-2 flex items-center gap-2">
                      <code className="flex-1 px-3 py-2 bg-surface-100 rounded-lg text-sm font-mono text-surface-600 truncate">
                        {revealedSecrets[secret.name] || '••••••••••••••••••••'}
                      </code>
                      <button
                        onClick={() => handleReveal(secret.name)}
                        className="p-2 text-surface-400 hover:text-surface-600 transition-colors"
                        title={revealedSecrets[secret.name] ? 'Hide' : 'Reveal'}
                      >
                        {revealedSecrets[secret.name] ? (
                          <FiEyeOff className="w-4 h-4" />
                        ) : (
                          <FiEye className="w-4 h-4" />
                        )}
                      </button>
                      {revealedSecrets[secret.name] && (
                        <button
                          onClick={() => handleCopy(revealedSecrets[secret.name])}
                          className="p-2 text-surface-400 hover:text-surface-600 transition-colors"
                          title="Copy"
                        >
                          <FiCopy className="w-4 h-4" />
                        </button>
                      )}
                    </div>

                    {/* Tags */}
                    {secret.tags.length > 0 && (
                      <div className="mt-2 flex items-center gap-1">
                        <FiTag className="w-3 h-3 text-surface-400" />
                        {secret.tags.map((tag) => (
                          <span
                            key={tag}
                            className="px-2 py-0.5 text-xs bg-surface-100 text-surface-600 rounded"
                          >
                            {tag}
                          </span>
                        ))}
                      </div>
                    )}

                    <p className="text-xs text-surface-400 mt-2">
                      Created {new Date(secret.createdAt).toLocaleDateString()}
                      {secret.expiresAt && ` • Expires ${new Date(secret.expiresAt).toLocaleDateString()}`}
                    </p>
                  </div>

                  <button
                    onClick={() => handleDelete(secret.name)}
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
      <Modal isOpen={showCreateModal} onClose={() => setShowCreateModal(false)} title="Add New Secret">
        <div className="space-y-4">
          <Input
            label="Secret Name"
            placeholder="API_KEY, DATABASE_URL, etc."
            value={newSecret.name}
            onChange={(e) => setNewSecret({ ...newSecret, name: e.target.value })}
            helperText="Use uppercase with underscores for consistency"
          />

          <div>
            <label className="block text-sm font-medium text-surface-700 mb-1">Secret Value</label>
            <textarea
              className="w-full px-4 py-3 border border-surface-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent font-mono text-sm"
              rows={3}
              placeholder="Enter the secret value..."
              value={newSecret.value}
              onChange={(e) => setNewSecret({ ...newSecret, value: e.target.value })}
            />
          </div>

          <Input
            label="Tags (optional)"
            placeholder="production, api, database"
            value={newSecret.tags}
            onChange={(e) => setNewSecret({ ...newSecret, tags: e.target.value })}
            helperText="Comma-separated tags for organization"
          />

          <div className="flex gap-3 pt-4">
            <Button variant="secondary" onClick={() => setShowCreateModal(false)} className="flex-1">
              Cancel
            </Button>
            <Button onClick={handleCreate} isLoading={isCreating} className="flex-1">
              Create Secret
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}

// Add missing import
import { FiLock } from 'react-icons/fi';
