import React, { useEffect, useState } from 'react';
import { FiPlus, FiTrash2, FiEdit2, FiUser, FiServer, FiFileText } from 'react-icons/fi';
import { Card, Button, Input, Modal, Badge } from '@/components/common';
import toast from 'react-hot-toast';

interface Account {
  id: string;
  name: string;
  type: 'user' | 'service' | 'contract';
  status: 'active' | 'suspended' | 'pending';
  balance: string;
  createdAt: string;
}

export function AccountsServicePage() {
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [formData, setFormData] = useState({
    name: '',
    type: 'user' as 'user' | 'service' | 'contract',
  });

  useEffect(() => {
    fetchAccounts();
  }, []);

  const fetchAccounts = async () => {
    // API call would go here
    setAccounts([]);
  };

  const handleSubmit = async () => {
    if (!formData.name) {
      toast.error('Account name is required');
      return;
    }

    setIsSubmitting(true);
    try {
      // API call to create account
      toast.success('Account created');
      setShowCreateModal(false);
      setFormData({ name: '', type: 'user' });
      fetchAccounts();
    } catch (e) {
      toast.error('Failed to create account');
    }
    setIsSubmitting(false);
  };

  const handleDelete = async (id: string, name: string) => {
    if (!confirm(`Delete account "${name}"?`)) return;
    try {
      toast.success('Account deleted');
      fetchAccounts();
    } catch (e) {
      toast.error('Failed to delete');
    }
  };

  const getTypeIcon = (type: string) => {
    switch (type) {
      case 'user': return <FiUser className="w-4 h-4" />;
      case 'service': return <FiServer className="w-4 h-4" />;
      case 'contract': return <FiFileText className="w-4 h-4" />;
      default: return <FiUser className="w-4 h-4" />;
    }
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'active': return <Badge variant="success">Active</Badge>;
      case 'suspended': return <Badge variant="error">Suspended</Badge>;
      case 'pending': return <Badge variant="warning">Pending</Badge>;
      default: return <Badge>{status}</Badge>;
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-surface-900">Accounts Service</h1>
          <p className="text-surface-500 mt-1">
            Manage user, service, and contract accounts
          </p>
        </div>
        <Button leftIcon={<FiPlus className="w-4 h-4" />} onClick={() => setShowCreateModal(true)}>
          Create Account
        </Button>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <p className="text-sm text-surface-500">Total Accounts</p>
          <p className="text-2xl font-bold text-surface-900">{accounts.length}</p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">User Accounts</p>
          <p className="text-2xl font-bold text-blue-600">
            {accounts.filter(a => a.type === 'user').length}
          </p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Service Accounts</p>
          <p className="text-2xl font-bold text-purple-600">
            {accounts.filter(a => a.type === 'service').length}
          </p>
        </Card>
        <Card>
          <p className="text-sm text-surface-500">Contract Accounts</p>
          <p className="text-2xl font-bold text-green-600">
            {accounts.filter(a => a.type === 'contract').length}
          </p>
        </Card>
      </div>

      {/* Accounts List */}
      <Card padding="none">
        <div className="p-4 border-b border-surface-200">
          <h3 className="font-semibold text-surface-900">All Accounts</h3>
        </div>
        <div className="divide-y divide-surface-200">
          {accounts.length === 0 ? (
            <div className="p-8 text-center text-surface-400">
              No accounts found. Create one to get started.
            </div>
          ) : (
            accounts.map((account) => (
              <div key={account.id} className="p-4 hover:bg-surface-50 transition-colors">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className="p-2 bg-surface-100 rounded-lg">
                      {getTypeIcon(account.type)}
                    </div>
                    <div>
                      <div className="flex items-center gap-2">
                        <span className="font-medium text-surface-900">{account.name}</span>
                        {getStatusBadge(account.status)}
                        <Badge size="sm" variant="default">{account.type}</Badge>
                      </div>
                      <p className="text-xs text-surface-400 mt-1">
                        Balance: {account.balance} • Created: {new Date(account.createdAt).toLocaleDateString()}
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      className="p-2 text-surface-400 hover:text-surface-600 transition-colors"
                      title="Edit"
                    >
                      <FiEdit2 className="w-4 h-4" />
                    </button>
                    <button
                      onClick={() => handleDelete(account.id, account.name)}
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
        onClose={() => { setShowCreateModal(false); setFormData({ name: '', type: 'user' }); }}
        title="Create Account"
        size="md"
      >
        <div className="space-y-4">
          <Input
            label="Account Name"
            placeholder="My Account"
            value={formData.name}
            onChange={(e) => setFormData({ ...formData, name: e.target.value })}
          />
          <div>
            <label className="block text-sm font-medium text-surface-700 mb-1">Account Type</label>
            <select
              className="w-full px-4 py-2.5 border border-surface-300 rounded-lg"
              value={formData.type}
              onChange={(e) => setFormData({ ...formData, type: e.target.value as any })}
            >
              <option value="user">User Account</option>
              <option value="service">Service Account</option>
              <option value="contract">Contract Account</option>
            </select>
          </div>
          <div className="flex gap-3 pt-4">
            <Button variant="secondary" onClick={() => setShowCreateModal(false)} className="flex-1">
              Cancel
            </Button>
            <Button onClick={handleSubmit} isLoading={isSubmitting} className="flex-1">
              Create
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
