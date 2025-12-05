import React, { useState } from 'react';
import { FiUser, FiMail, FiShield, FiLink, FiBell, FiGlobe, FiSave } from 'react-icons/fi';
import { Card, CardHeader, Button, Input, Badge } from '@/components/common';
import { WalletConnect } from '@/components/auth/WalletConnect';
import { ServiceSettings } from '@/components/settings';
import { useAuthStore } from '@/stores/authStore';
import toast from 'react-hot-toast';

export function SettingsPage() {
  const { user, account, wallet, updateAccount } = useAuthStore();
  const [isUpdating, setIsUpdating] = useState(false);
  const [formData, setFormData] = useState({
    name: account?.name || '',
    email: user?.email || '',
  });

  const handleSave = async () => {
    setIsUpdating(true);
    const success = await updateAccount({ name: formData.name });
    if (success) {
      toast.success('Settings saved successfully');
    } else {
      toast.error('Failed to save settings');
    }
    setIsUpdating(false);
  };

  return (
    <div className="space-y-6 max-w-4xl">
      <div>
        <h1 className="text-2xl font-bold text-surface-900">Settings</h1>
        <p className="text-surface-500 mt-1">Manage your account settings and preferences</p>
      </div>

      {/* Profile Settings */}
      <Card>
        <CardHeader
          title="Profile"
          description="Your personal information"
          action={<FiUser className="w-5 h-5 text-surface-400" />}
        />
        <div className="space-y-4">
          <Input
            label="Full Name"
            value={formData.name}
            onChange={(e) => setFormData({ ...formData, name: e.target.value })}
            leftIcon={<FiUser className="w-5 h-5" />}
          />
          <Input
            label="Email"
            type="email"
            value={formData.email}
            disabled
            leftIcon={<FiMail className="w-5 h-5" />}
            helperText="Email cannot be changed"
          />
          <div className="flex justify-end">
            <Button onClick={handleSave} isLoading={isUpdating} leftIcon={<FiSave className="w-4 h-4" />}>
              Save Changes
            </Button>
          </div>
        </div>
      </Card>

      {/* Account Info */}
      <Card>
        <CardHeader
          title="Account"
          description="Your account details and subscription"
          action={<FiShield className="w-5 h-5 text-surface-400" />}
        />
        <div className="space-y-4">
          <div className="flex items-center justify-between p-4 bg-surface-50 rounded-lg">
            <div>
              <p className="text-sm text-surface-500">Account ID</p>
              <p className="font-mono text-surface-900">{account?.id || 'N/A'}</p>
            </div>
            <Badge variant={account?.status === 'active' ? 'success' : 'warning'}>
              {account?.status || 'Unknown'}
            </Badge>
          </div>

          <div className="flex items-center justify-between p-4 bg-surface-50 rounded-lg">
            <div>
              <p className="text-sm text-surface-500">Subscription Plan</p>
              <p className="font-medium text-surface-900 capitalize">{account?.tier || 'Free'}</p>
            </div>
            <Button variant="secondary" size="sm">
              Upgrade
            </Button>
          </div>

          <div className="flex items-center justify-between p-4 bg-surface-50 rounded-lg">
            <div>
              <p className="text-sm text-surface-500">Member Since</p>
              <p className="text-surface-900">
                {account?.createdAt
                  ? new Date(account.createdAt).toLocaleDateString()
                  : 'N/A'}
              </p>
            </div>
          </div>
        </div>
      </Card>

      {/* Wallet Connection */}
      <Card>
        <CardHeader
          title="Wallet"
          description="Connect your Neo N3 wallet for deposits and transactions"
          action={<FiLink className="w-5 h-5 text-surface-400" />}
        />
        <WalletConnect showLinkOption />
        {user?.walletAddress && (
          <div className="mt-4 p-4 bg-green-50 rounded-lg">
            <p className="text-sm text-green-700">
              <strong>Linked Wallet:</strong>{' '}
              <span className="font-mono">{user.walletAddress}</span>
            </p>
          </div>
        )}
      </Card>

      {/* Service Settings */}
      <ServiceSettings />

      {/* Notifications */}
      <Card>
        <CardHeader
          title="Notifications"
          description="Configure how you receive notifications"
          action={<FiBell className="w-5 h-5 text-surface-400" />}
        />
        <div className="space-y-3">
          {[
            { id: 'email_alerts', label: 'Email Alerts', description: 'Receive alerts via email' },
            { id: 'service_updates', label: 'Service Updates', description: 'Notifications about service status' },
            { id: 'security_alerts', label: 'Security Alerts', description: 'Important security notifications' },
            { id: 'billing', label: 'Billing', description: 'Payment and billing notifications' },
          ].map((item) => (
            <label
              key={item.id}
              className="flex items-center justify-between p-4 bg-surface-50 rounded-lg cursor-pointer hover:bg-surface-100 transition-colors"
            >
              <div>
                <p className="font-medium text-surface-900">{item.label}</p>
                <p className="text-sm text-surface-500">{item.description}</p>
              </div>
              <input
                type="checkbox"
                defaultChecked={item.id === 'security_alerts'}
                className="w-5 h-5 rounded border-surface-300 text-primary-600 focus:ring-primary-500"
              />
            </label>
          ))}
        </div>
      </Card>

      {/* Danger Zone */}
      <Card className="border-red-200">
        <CardHeader
          title="Danger Zone"
          description="Irreversible actions"
        />
        <div className="space-y-3">
          <div className="flex items-center justify-between p-4 bg-red-50 rounded-lg">
            <div>
              <p className="font-medium text-red-900">Delete Account</p>
              <p className="text-sm text-red-700">
                Permanently delete your account and all associated data
              </p>
            </div>
            <Button variant="danger" size="sm">
              Delete Account
            </Button>
          </div>
        </div>
      </Card>
    </div>
  );
}
