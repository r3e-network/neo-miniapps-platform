import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { User, Key, Bell, Plus, Trash2, Copy, Check, AlertCircle, Wallet, Link2 } from 'lucide-react';
import { useAuthStore } from '../stores/auth';
import { api, API_BASE, APIKey, Wallet as WalletType, OAuthProvider } from '../api/client';

export function Settings() {
  const { user } = useAuthStore();
  const queryClient = useQueryClient();
  const [showCreateKey, setShowCreateKey] = useState(false);
  const [newKeyName, setNewKeyName] = useState('');
  const [newKeyScopes, setNewKeyScopes] = useState<string[]>([]);
  const [createdKey, setCreatedKey] = useState<string | null>(null);
  const [copiedKey, setCopiedKey] = useState(false);

  // Fetch API keys
  const { data: apiKeys, isLoading: keysLoading } = useQuery({
    queryKey: ['api-keys'],
    queryFn: () => api.listAPIKeys(),
  });

  // Fetch wallets
  const { data: wallets, isLoading: walletsLoading } = useQuery({
    queryKey: ['wallets'],
    queryFn: () => api.listWallets(),
  });

  // Fetch OAuth providers
  const { data: oauthProviders, isLoading: oauthLoading } = useQuery({
    queryKey: ['oauth-providers'],
    queryFn: () => api.listOAuthProviders(),
  });

  // Create API key mutation
  const createKeyMutation = useMutation({
    mutationFn: () => api.createAPIKey(newKeyName, newKeyScopes),
    onSuccess: (data) => {
      setCreatedKey(data.key);
      setNewKeyName('');
      setNewKeyScopes([]);
      queryClient.invalidateQueries({ queryKey: ['api-keys'] });
    },
  });

  // Revoke API key mutation
  const revokeKeyMutation = useMutation({
    mutationFn: (keyId: string) => api.revokeAPIKey(keyId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['api-keys'] });
    },
  });

  // Set primary wallet mutation
  const setPrimaryMutation = useMutation({
    mutationFn: (walletId: string) => api.setPrimaryWallet(walletId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['wallets'] });
    },
  });

  // Delete wallet mutation
  const deleteWalletMutation = useMutation({
    mutationFn: (walletId: string) => api.deleteWallet(walletId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['wallets'] });
    },
  });

  // Unlink OAuth provider mutation
  const unlinkOAuthMutation = useMutation({
    mutationFn: (providerId: string) => api.unlinkOAuthProvider(providerId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['oauth-providers'] });
    },
  });

  const handleOAuthLink = (provider: 'google' | 'github') => {
    window.location.href = `${API_BASE}/auth/${provider}`;
  };

  const handleCopyKey = async () => {
    if (createdKey) {
      await navigator.clipboard.writeText(createdKey);
      setCopiedKey(true);
      setTimeout(() => setCopiedKey(false), 2000);
    }
  };

  const handleCreateKey = () => {
    if (newKeyName.trim()) {
      createKeyMutation.mutate();
    }
  };

  const availableScopes = [
    { id: 'neooracle', label: 'NeoOracle Service' },
    { id: 'neorand', label: 'NeoRand Service' },
    { id: 'neostore', label: 'NeoStore (Secrets)' },
    { id: 'neoflow', label: 'NeoFlow' },
    { id: 'neofeeds', label: 'Data Feeds' },
    { id: 'gasbank', label: 'Gas Bank' },
  ];

  const toggleScope = (scope: string) => {
    setNewKeyScopes((prev) =>
      prev.includes(scope) ? prev.filter((s) => s !== scope) : [...prev, scope]
    );
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  };

  return (
    <div>
      <h1 className="text-3xl font-bold text-white mb-8">Settings</h1>

      <div className="space-y-6">
        {/* Profile */}
        <div className="bg-gray-800 rounded-xl p-6 border border-gray-700">
          <div className="flex items-center gap-3 mb-4">
            <User className="w-5 h-5 text-green-500" />
            <h2 className="text-lg font-semibold text-white">Profile</h2>
          </div>
          <div className="space-y-4">
            <div>
              <label className="block text-sm text-gray-400 mb-1">User ID</label>
              <input
                type="text"
                value={user?.id || ''}
                readOnly
                className="w-full bg-gray-700 border border-gray-600 rounded-lg px-3 py-2 text-white font-mono text-sm"
              />
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Primary Wallet Address</label>
              <input
                type="text"
                value={user?.address || ''}
                readOnly
                className="w-full bg-gray-700 border border-gray-600 rounded-lg px-3 py-2 text-white font-mono"
              />
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Email (optional)</label>
              <input
                type="email"
                placeholder="your@email.com"
                className="w-full bg-gray-700 border border-gray-600 rounded-lg px-3 py-2 text-white"
              />
            </div>
          </div>
        </div>

        {/* Linked Accounts */}
        <div className="bg-gray-800 rounded-xl p-6 border border-gray-700">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-3">
              <Link2 className="w-5 h-5 text-green-500" />
              <h2 className="text-lg font-semibold text-white">Linked Accounts</h2>
            </div>
          </div>

          <p className="text-gray-400 mb-4 text-sm">
            Link your Google or GitHub account for easy login.
          </p>

          {oauthLoading ? (
            <div className="text-gray-400">Loading linked accounts...</div>
          ) : (
            <div className="space-y-3">
              {/* Google */}
              {oauthProviders?.find((p: OAuthProvider) => p.provider === 'google') ? (
                <div className="flex items-center justify-between bg-gray-700/50 rounded-lg p-3">
                  <div className="flex items-center gap-3">
                    <svg className="w-5 h-5" viewBox="0 0 24 24">
                      <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
                      <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
                      <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/>
                      <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
                    </svg>
                    <div>
                      <span className="text-white">Google</span>
                      <span className="text-gray-400 text-sm ml-2">
                        {oauthProviders.find((p: OAuthProvider) => p.provider === 'google')?.email}
                      </span>
                    </div>
                  </div>
                  <button
                    onClick={() => {
                      const provider = oauthProviders.find((p: OAuthProvider) => p.provider === 'google');
                      if (provider && confirm('Unlink your Google account?')) {
                        unlinkOAuthMutation.mutate(provider.id);
                      }
                    }}
                    className="text-red-400 hover:text-red-300 p-2"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              ) : (
                <button
                  onClick={() => handleOAuthLink('google')}
                  className="w-full flex items-center gap-3 bg-white hover:bg-gray-100 text-gray-800 font-medium py-3 px-4 rounded-lg transition-colors"
                >
                  <svg className="w-5 h-5" viewBox="0 0 24 24">
                    <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
                    <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
                    <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"/>
                    <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
                  </svg>
                  <span>Link Google Account</span>
                </button>
              )}

              {/* GitHub */}
              {oauthProviders?.find((p: OAuthProvider) => p.provider === 'github') ? (
                <div className="flex items-center justify-between bg-gray-700/50 rounded-lg p-3">
                  <div className="flex items-center gap-3">
                    <svg className="w-5 h-5 text-white" viewBox="0 0 24 24" fill="currentColor">
                      <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
                    </svg>
                    <div>
                      <span className="text-white">GitHub</span>
                      <span className="text-gray-400 text-sm ml-2">
                        {oauthProviders.find((p: OAuthProvider) => p.provider === 'github')?.display_name ||
                         oauthProviders.find((p: OAuthProvider) => p.provider === 'github')?.email}
                      </span>
                    </div>
                  </div>
                  <button
                    onClick={() => {
                      const provider = oauthProviders.find((p: OAuthProvider) => p.provider === 'github');
                      if (provider && confirm('Unlink your GitHub account?')) {
                        unlinkOAuthMutation.mutate(provider.id);
                      }
                    }}
                    className="text-red-400 hover:text-red-300 p-2"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              ) : (
                <button
                  onClick={() => handleOAuthLink('github')}
                  className="w-full flex items-center gap-3 bg-gray-900 border border-gray-600 hover:bg-gray-800 text-white font-medium py-3 px-4 rounded-lg transition-colors"
                >
                  <svg className="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
                  </svg>
                  <span>Link GitHub Account</span>
                </button>
              )}
            </div>
          )}
        </div>

        {/* Connected Wallets */}
        <div className="bg-gray-800 rounded-xl p-6 border border-gray-700">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-3">
              <Wallet className="w-5 h-5 text-green-500" />
              <h2 className="text-lg font-semibold text-white">Connected Wallets</h2>
            </div>
          </div>

          {walletsLoading ? (
            <div className="text-gray-400">Loading wallets...</div>
          ) : wallets && wallets.length > 0 ? (
            <div className="space-y-3">
              {wallets.map((wallet: WalletType) => (
                <div
                  key={wallet.id}
                  className="flex items-center justify-between bg-gray-700/50 rounded-lg p-3"
                >
                  <div className="flex items-center gap-3">
                    <div className="font-mono text-sm text-white">{wallet.address}</div>
                    {wallet.is_primary && (
                      <span className="bg-green-500/20 text-green-400 text-xs px-2 py-0.5 rounded">
                        Primary
                      </span>
                    )}
                    {wallet.label && (
                      <span className="text-gray-400 text-sm">{wallet.label}</span>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    {!wallet.is_primary && (
                      <>
                        <button
                          onClick={() => setPrimaryMutation.mutate(wallet.id)}
                          className="text-gray-400 hover:text-white text-sm"
                        >
                          Set Primary
                        </button>
                        <button
                          onClick={() => deleteWalletMutation.mutate(wallet.id)}
                          className="text-red-400 hover:text-red-300 p-1"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </>
                    )}
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="text-gray-400">No wallets connected</div>
          )}
        </div>

        {/* API Keys */}
        <div className="bg-gray-800 rounded-xl p-6 border border-gray-700">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-3">
              <Key className="w-5 h-5 text-green-500" />
              <h2 className="text-lg font-semibold text-white">API Keys</h2>
            </div>
            <button
              onClick={() => {
                setShowCreateKey(true);
                setCreatedKey(null);
              }}
              className="flex items-center gap-2 bg-green-600 hover:bg-green-700 text-white px-3 py-1.5 rounded-lg text-sm"
            >
              <Plus className="w-4 h-4" />
              New Key
            </button>
          </div>

          <p className="text-gray-400 mb-4 text-sm">
            API keys allow programmatic access to the Service Layer. Keep them secure!
          </p>

          {/* Created Key Alert */}
          {createdKey && (
            <div className="bg-yellow-500/10 border border-yellow-500 rounded-lg p-4 mb-4">
              <div className="flex items-start gap-3">
                <AlertCircle className="w-5 h-5 text-yellow-500 flex-shrink-0 mt-0.5" />
                <div className="flex-1">
                  <p className="text-yellow-500 font-medium mb-2">
                    Save your API key now! It won't be shown again.
                  </p>
                  <div className="flex items-center gap-2 bg-gray-900 rounded p-2">
                    <code className="text-green-400 text-sm flex-1 break-all">{createdKey}</code>
                    <button
                      onClick={handleCopyKey}
                      className="text-gray-400 hover:text-white p-1"
                    >
                      {copiedKey ? (
                        <Check className="w-4 h-4 text-green-500" />
                      ) : (
                        <Copy className="w-4 h-4" />
                      )}
                    </button>
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* Create Key Form */}
          {showCreateKey && !createdKey && (
            <div className="bg-gray-700/50 rounded-lg p-4 mb-4">
              <div className="space-y-4">
                <div>
                  <label className="block text-sm text-gray-400 mb-1">Key Name</label>
                  <input
                    type="text"
                    value={newKeyName}
                    onChange={(e) => setNewKeyName(e.target.value)}
                    placeholder="e.g., Production Server"
                    className="w-full bg-gray-700 border border-gray-600 rounded-lg px-3 py-2 text-white"
                  />
                </div>
                <div>
                  <label className="block text-sm text-gray-400 mb-2">Scopes (optional)</label>
                  <div className="flex flex-wrap gap-2">
                    {availableScopes.map((scope) => (
                      <button
                        key={scope.id}
                        onClick={() => toggleScope(scope.id)}
                        className={`px-3 py-1 rounded-full text-sm ${
                          newKeyScopes.includes(scope.id)
                            ? 'bg-green-600 text-white'
                            : 'bg-gray-600 text-gray-300 hover:bg-gray-500'
                        }`}
                      >
                        {scope.label}
                      </button>
                    ))}
                  </div>
                  <p className="text-gray-500 text-xs mt-2">
                    Leave empty for full access
                  </p>
                </div>
                <div className="flex gap-2">
                  <button
                    onClick={handleCreateKey}
                    disabled={!newKeyName.trim() || createKeyMutation.isPending}
                    className="bg-green-600 hover:bg-green-700 disabled:bg-gray-600 text-white px-4 py-2 rounded-lg"
                  >
                    {createKeyMutation.isPending ? 'Creating...' : 'Create Key'}
                  </button>
                  <button
                    onClick={() => setShowCreateKey(false)}
                    className="bg-gray-600 hover:bg-gray-500 text-white px-4 py-2 rounded-lg"
                  >
                    Cancel
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* API Keys List */}
          {keysLoading ? (
            <div className="text-gray-400">Loading API keys...</div>
          ) : apiKeys && apiKeys.length > 0 ? (
            <div className="space-y-3">
              {apiKeys.map((key: APIKey) => (
                <div
                  key={key.id}
                  className="flex items-center justify-between bg-gray-700/50 rounded-lg p-3"
                >
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="text-white font-medium">{key.name}</span>
                      <code className="text-gray-400 text-sm">{key.prefix}...</code>
                    </div>
                    <div className="text-gray-500 text-sm mt-1">
                      Created {formatDate(key.created_at)}
                      {key.last_used && ` • Last used ${formatDate(key.last_used)}`}
                    </div>
                    {key.scopes && key.scopes.length > 0 && (
                      <div className="flex gap-1 mt-1">
                        {key.scopes.map((scope) => (
                          <span
                            key={scope}
                            className="bg-gray-600 text-gray-300 text-xs px-2 py-0.5 rounded"
                          >
                            {scope}
                          </span>
                        ))}
                      </div>
                    )}
                  </div>
                  <button
                    onClick={() => {
                      if (confirm('Are you sure you want to revoke this API key?')) {
                        revokeKeyMutation.mutate(key.id);
                      }
                    }}
                    className="text-red-400 hover:text-red-300 p-2"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              ))}
            </div>
          ) : (
            <div className="text-gray-400 text-center py-4">
              No API keys yet. Create one to get started.
            </div>
          )}
        </div>

        {/* Notifications */}
        <div className="bg-gray-800 rounded-xl p-6 border border-gray-700">
          <div className="flex items-center gap-3 mb-4">
            <Bell className="w-5 h-5 text-green-500" />
            <h2 className="text-lg font-semibold text-white">Notifications</h2>
          </div>
          <div className="space-y-3">
            <label className="flex items-center gap-3">
              <input type="checkbox" className="w-4 h-4 rounded" defaultChecked />
              <span className="text-gray-300">Email notifications for neoflow triggers</span>
            </label>
            <label className="flex items-center gap-3">
              <input type="checkbox" className="w-4 h-4 rounded" defaultChecked />
              <span className="text-gray-300">Low gas balance alerts</span>
            </label>
            <label className="flex items-center gap-3">
              <input type="checkbox" className="w-4 h-4 rounded" />
              <span className="text-gray-300">Weekly usage reports</span>
            </label>
          </div>
        </div>

        {/* CLI Usage */}
        <div className="bg-gray-800 rounded-xl p-6 border border-gray-700">
          <div className="flex items-center gap-3 mb-4">
            <Key className="w-5 h-5 text-green-500" />
            <h2 className="text-lg font-semibold text-white">CLI Usage</h2>
          </div>
          <p className="text-gray-400 mb-4 text-sm">
            Use your API key with the Service Layer CLI:
          </p>
          <div className="bg-gray-900 rounded-lg p-4">
            <code className="text-green-400 text-sm">
              # Set your API key{'\n'}
              export SERVICE_LAYER_API_KEY="sl_your_api_key_here"{'\n\n'}
              # Or pass it directly{'\n'}
              service-layer --api-key sl_your_api_key_here oracle fetch https://api.example.com
            </code>
          </div>
        </div>
      </div>
    </div>
  );
}
