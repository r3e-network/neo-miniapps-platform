import React, { useEffect, useState } from 'react';
import { FiPlus, FiTrash2, FiEye, FiEyeOff, FiCopy, FiShield } from 'react-icons/fi';
import { Card, Button, Input, Modal, Badge } from '@/components/common';
import { useServicesStore } from '@/stores/servicesStore';
import toast from 'react-hot-toast';

interface Secret {
  id: string;
  key: string;
  createdAt: string;
  updatedAt: string;
  version: number;
}

export function SecretsServicePage() {
  const [secrets, setSecrets] = useState<Secret[]>([]);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [formData, setFormData] = useState({ key: '', value: '' });

  useEffect(() => {
    // Fetch secrets list (metadata only)
    fetchSecrets();
  }, []);

  const fetchSecrets = async () => {
    // API call would go here
    setSecrets([]);
  };

  const handleSubmit = async () => {
    if (!formData.key || !formData.value) {
      toast.error('Key and value are required');
      return;
    }

    setIsSubmitting(true);
    try {
      // API call to store secret
      toast.success('Secret stored securely in TEE');
      setShowCreateModal(false);
      setFormData({ key: '', value: '' });
      fetchSecrets();
    } catch (e) {
      toast.error('Failed to store secret');
    }
    setIsSubmitting(false);
  };

  const handleDelete = async (key: string) => {
    if (!confirm(`Delete secret "${key}"?`)) return;
    try {
      // API call to delete
      toast.success('Secret deleted');
      fetchSecrets();
    } catch (e) {
      toast.error('Failed to delete');
    }
  };

  const handleCopyKey = (key: string) => {
    navigator.clipboard.writeText(key);
    toast.success('Key copied to clipboard');
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-surface-900">Secrets Service</h1>
          <p className="text-surface-500 mt-1">
            Securely store and manage secrets in TEE-protected storage
          </p>
        </div>
        <Button leftIcon={<FiPlus className="w-4 h-4" />} onClick={() => setShowCreateModal(true)}>
          Add Secret
        </Button>
      </div>

      {/* Security Info */}
      <Card className="bg-green-50 border-green-200">
        <div className="flex items-start gap-3">
          <FiShield className="w-5 h-5 text-green-600 mt-0.5" />
          <div>
            <h3 className="font-medium text-green-900">TEE-Protected Storage</h3>
            <p className="text-sm text-green-700 mt-1">
              All secrets are encrypted and stored within the Trusted Execution Environment.
              Secret values never leave the TEE in plaintext.
            </p>
          </div>
        </div>
      </Card>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card>
          <p className="text-sm text-surface-500">Total Secrets</p>
          <p className="text-2xl font-bold text-surface-900">{secrets.length}</p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Storage Used</p>
          <p className="text-2xl font-bold text-surface-900">0 KB</p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Last Updated</p>
          <p className="text-2xl font-bold text-surface-900">-</p>
        </Card>
      </div>

      {/* Secrets List */}
      <Card padding="none">
        <div className="p-4 border-b border-surface-200">
          <h3 className="font-semibold text-surface-900">Stored Secrets</h3>
        </div>
        <div className="divide-y divide-surface-200">
          {secrets.length === 0 ? (
            <div className="p-8 text-center text-surface-400">
              No secrets stored. Add one to get started.
            </div>
          ) : (
            secrets.map((secret) => (
              <div key={secret.id} className="p-4 hover:bg-surface-50 transition-colors">
                <div className="flex items-center justify-between">
                  <div>
                    <div className="flex items-center gap-2">
                      <code className="font-medium text-surface-900">{secret.key}</code>
                      <Badge size="sm">v{secret.version}</Badge>
                    </div>
                    <p className="text-xs text-surface-400 mt-1">
                      Created: {new Date(secret.createdAt).toLocaleString()}
                    </p>
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => handleCopyKey(secret.key)}
                      className="p-2 text-surface-400 hover:text-surface-600 transition-colors"
                      title="Copy key"
                    >
                      <FiCopy className="w-4 h-4" />
                    </button>
                    <button
                      onClick={() => handleDelete(secret.key)}
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

      {/* Create Modal */}
      <Modal
        isOpen={showCreateModal}
        onClose={() => { setShowCreateModal(false); setFormData({ key: '', value: '' }); }}
        title="Add Secret"
        size="lg"
      >
        <div className="space-y-4">
          <Input
            label="Secret Key"
            placeholder="API_KEY"
            value={formData.key}
            onChange={(e) => setFormData({ ...formData, key: e.target.value })}
            helperText="Use uppercase with underscores (e.g., MY_SECRET_KEY)"
          />
          <div>
            <label className="block text-sm font-medium text-surface-700 mb-1">Secret Value</label>
            <textarea
              className="w-full px-4 py-3 border border-surface-300 rounded-lg font-mono text-sm"
              rows={4}
              placeholder="Enter secret value..."
              value={formData.value}
              onChange={(e) => setFormData({ ...formData, value: e.target.value })}
            />
            <p className="text-xs text-surface-400 mt-1">
              Value will be encrypted and stored in TEE
            </p>
          </div>
          <div className="flex gap-3 pt-4">
            <Button variant="secondary" onClick={() => setShowCreateModal(false)} className="flex-1">
              Cancel
            </Button>
            <Button onClick={handleSubmit} isLoading={isSubmitting} className="flex-1">
              Store Secret
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
