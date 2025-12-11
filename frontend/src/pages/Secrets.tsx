import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  Key,
  Plus,
  Trash2,
  Shield,
  Clock,
  Eye,
  EyeOff,
  AlertCircle,
  Loader2,
  Users,
  FileText,
  X
} from 'lucide-react';
import { api } from '../api/client';

interface Secret {
  id: string;
  name: string;
  version: number;
  created_at: string;
  last_accessed?: string;
}

interface Permission {
  service_name: string;
  granted_at: string;
}

interface AuditLog {
  id: string;
  secret_name: string;
  action: string;
  service_name?: string;
  timestamp: string;
  details?: string;
}

export function Secrets() {
  const queryClient = useQueryClient();

  // UI State
  const [showCreate, setShowCreate] = useState(false);
  const [showDelete, setShowDelete] = useState<string | null>(null);
  const [showPermissions, setShowPermissions] = useState<string | null>(null);
  const [showAuditLog, setShowAuditLog] = useState<string | null>(null);
  const [showValue, setShowValue] = useState(false);

  // Form State
  const [newSecret, setNewSecret] = useState({ name: '', value: '' });
  const [newPermission, setNewPermission] = useState('');

  // Queries
  const { data: secrets, isLoading } = useQuery({
    queryKey: ['neostore'],
    queryFn: () => api.listSecrets(),
  });

  const { data: permissions } = useQuery({
    queryKey: ['secret-permissions', showPermissions],
    queryFn: () => showPermissions ? api.getSecretPermissions(showPermissions) : Promise.resolve([]),
    enabled: !!showPermissions,
  });

  const { data: auditLogs } = useQuery({
    queryKey: ['secret-audit', showAuditLog],
    queryFn: () => showAuditLog ? api.getSecretAuditLog(showAuditLog) : Promise.resolve([]),
    enabled: !!showAuditLog,
  });

  // Mutations
  const createMutation = useMutation({
    mutationFn: () => api.createSecret(newSecret.name, newSecret.value),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['neostore'] });
      setShowCreate(false);
      setNewSecret({ name: '', value: '' });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (name: string) => api.deleteSecret(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['neostore'] });
      setShowDelete(null);
    },
  });

  const grantPermissionMutation = useMutation({
    mutationFn: ({ secretName, serviceName }: { secretName: string; serviceName: string }) =>
      api.grantSecretPermission(secretName, serviceName),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['secret-permissions'] });
      setNewPermission('');
    },
  });

  const revokePermissionMutation = useMutation({
    mutationFn: ({ secretName, serviceName }: { secretName: string; serviceName: string }) =>
      api.revokeSecretPermission(secretName, serviceName),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['secret-permissions'] });
    },
  });

  // Handlers
  const handleCreate = () => {
    if (!newSecret.name || !newSecret.value) return;
    createMutation.mutate();
  };

  const handleDelete = (name: string) => {
    deleteMutation.mutate(name);
  };

  const handleGrantPermission = () => {
    if (!showPermissions || !newPermission) return;
    grantPermissionMutation.mutate({
      secretName: showPermissions,
      serviceName: newPermission,
    });
  };

  const handleRevokePermission = (serviceName: string) => {
    if (!showPermissions) return;
    revokePermissionMutation.mutate({
      secretName: showPermissions,
      serviceName,
    });
  };

  // Format date
  const formatDate = (dateStr?: string) => {
    if (!dateStr) return 'Never';
    return new Date(dateStr).toLocaleString();
  };

  return (
    <div className="max-w-6xl mx-auto">
      <div className="flex items-center justify-between mb-8">
        <div className="flex items-center gap-3">
          <div className="p-3 bg-green-500/20 rounded-xl">
            <Key className="w-8 h-8 text-green-500" />
          </div>
          <div>
            <h1 className="text-3xl font-bold text-white">Secrets Management</h1>
            <p className="text-gray-400">Securely store and manage sensitive data in TEE</p>
          </div>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="flex items-center gap-2 bg-green-600 hover:bg-green-700 text-white px-4 py-2 rounded-lg transition-colors"
        >
          <Plus className="w-5 h-5" />
          Add Secret
        </button>
      </div>

      {/* Info Banner */}
      <div className="bg-green-500/10 border border-green-500/30 rounded-xl p-4 mb-8">
        <div className="flex items-start gap-3">
          <Shield className="w-5 h-5 text-green-500 mt-0.5" />
          <div>
            <h3 className="text-green-400 font-medium mb-1">TEE-Protected Storage</h3>
            <p className="text-gray-400 text-sm">
              Secrets are encrypted and stored within the Trusted Execution Environment. Only authorized
              services with explicit permissions can access them. All access is logged for audit purposes.
            </p>
          </div>
        </div>
      </div>

      {/* Create Modal */}
      {showCreate && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-gray-800 rounded-xl p-6 w-full max-w-md border border-gray-700">
            <h2 className="text-xl font-semibold text-white mb-4">Create Secret</h2>
            <div className="space-y-4">
              <div>
                <label className="block text-sm text-gray-400 mb-1">Name</label>
                <input
                  type="text"
                  value={newSecret.name}
                  onChange={(e) => setNewSecret({ ...newSecret, name: e.target.value })}
                  className="w-full bg-gray-700 border border-gray-600 rounded-lg px-3 py-2 text-white"
                  placeholder="API_KEY"
                />
                <p className="text-xs text-gray-500 mt-1">
                  Use uppercase with underscores (e.g., DATABASE_PASSWORD)
                </p>
              </div>
              <div>
                <label className="block text-sm text-gray-400 mb-1">Value</label>
                <div className="relative">
                  <input
                    type={showValue ? 'text' : 'password'}
                    value={newSecret.value}
                    onChange={(e) => setNewSecret({ ...newSecret, value: e.target.value })}
                    className="w-full bg-gray-700 border border-gray-600 rounded-lg px-3 py-2 pr-10 text-white"
                    placeholder="••••••••"
                  />
                  <button
                    type="button"
                    onClick={() => setShowValue(!showValue)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-white"
                  >
                    {showValue ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                  </button>
                </div>
                <p className="text-xs text-gray-500 mt-1">
                  Value is encrypted before transmission using TEE public key
                </p>
              </div>
            </div>

            {createMutation.isError && (
              <div className="mt-4 p-3 bg-red-500/10 border border-red-500/30 rounded-lg flex items-center gap-2 text-red-400 text-sm">
                <AlertCircle className="w-4 h-4" />
                {(createMutation.error as Error)?.message || 'Failed to create secret'}
              </div>
            )}

            <div className="flex gap-3 mt-6">
              <button
                onClick={() => setShowCreate(false)}
                className="flex-1 bg-gray-700 hover:bg-gray-600 text-white py-2 rounded-lg"
              >
                Cancel
              </button>
              <button
                onClick={handleCreate}
                disabled={createMutation.isPending || !newSecret.name || !newSecret.value}
                className="flex-1 bg-green-600 hover:bg-green-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white py-2 rounded-lg flex items-center justify-center gap-2"
              >
                {createMutation.isPending ? (
                  <>
                    <Loader2 className="w-4 h-4 animate-spin" />
                    Creating...
                  </>
                ) : (
                  'Create'
                )}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete Confirmation Modal */}
      {showDelete && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-gray-800 rounded-xl p-6 w-full max-w-md border border-gray-700">
            <div className="flex items-center gap-3 mb-4">
              <div className="p-2 bg-red-500/20 rounded-lg">
                <AlertCircle className="w-6 h-6 text-red-500" />
              </div>
              <h2 className="text-xl font-semibold text-white">Delete Secret</h2>
            </div>
            <p className="text-gray-400 mb-6">
              Are you sure you want to delete the secret <strong className="text-white">{showDelete}</strong>?
              This action cannot be undone and will revoke all service permissions.
            </p>

            {deleteMutation.isError && (
              <div className="mb-4 p-3 bg-red-500/10 border border-red-500/30 rounded-lg flex items-center gap-2 text-red-400 text-sm">
                <AlertCircle className="w-4 h-4" />
                {(deleteMutation.error as Error)?.message || 'Failed to delete secret'}
              </div>
            )}

            <div className="flex gap-3">
              <button
                onClick={() => setShowDelete(null)}
                className="flex-1 bg-gray-700 hover:bg-gray-600 text-white py-2 rounded-lg"
              >
                Cancel
              </button>
              <button
                onClick={() => handleDelete(showDelete)}
                disabled={deleteMutation.isPending}
                className="flex-1 bg-red-600 hover:bg-red-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white py-2 rounded-lg flex items-center justify-center gap-2"
              >
                {deleteMutation.isPending ? (
                  <>
                    <Loader2 className="w-4 h-4 animate-spin" />
                    Deleting...
                  </>
                ) : (
                  <>
                    <Trash2 className="w-4 h-4" />
                    Delete
                  </>
                )}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Permissions Modal */}
      {showPermissions && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-gray-800 rounded-xl p-6 w-full max-w-2xl border border-gray-700 max-h-[80vh] overflow-y-auto">
            <div className="flex items-center justify-between mb-6">
              <div className="flex items-center gap-3">
                <Users className="w-6 h-6 text-blue-500" />
                <h2 className="text-xl font-semibold text-white">
                  Permissions: {showPermissions}
                </h2>
              </div>
              <button
                onClick={() => setShowPermissions(null)}
                className="text-gray-400 hover:text-white"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {/* Grant Permission Form */}
            <div className="mb-6 p-4 bg-gray-700/50 rounded-lg">
              <label className="block text-sm text-gray-400 mb-2">Grant Access to Service</label>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={newPermission}
                  onChange={(e) => setNewPermission(e.target.value)}
                  placeholder="Service name (e.g., neoflow, neooracle)"
                  className="flex-1 bg-gray-700 border border-gray-600 rounded-lg px-3 py-2 text-white text-sm"
                />
                <button
                  onClick={handleGrantPermission}
                  disabled={grantPermissionMutation.isPending || !newPermission}
                  className="bg-blue-600 hover:bg-blue-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white px-4 py-2 rounded-lg text-sm flex items-center gap-2"
                >
                  {grantPermissionMutation.isPending ? (
                    <Loader2 className="w-4 h-4 animate-spin" />
                  ) : (
                    <Plus className="w-4 h-4" />
                  )}
                  Grant
                </button>
              </div>
            </div>

            {/* Permissions List */}
            <div>
              <h3 className="text-sm font-medium text-gray-400 mb-3">Authorized Services</h3>
              {permissions && permissions.length > 0 ? (
                <div className="space-y-2">
                  {permissions.map((perm: Permission) => (
                    <div
                      key={perm.service_name}
                      className="flex items-center justify-between p-3 bg-gray-700/50 rounded-lg"
                    >
                      <div>
                        <div className="text-white font-medium">{perm.service_name}</div>
                        <div className="text-xs text-gray-500">
                          Granted: {formatDate(perm.granted_at)}
                        </div>
                      </div>
                      <button
                        onClick={() => handleRevokePermission(perm.service_name)}
                        disabled={revokePermissionMutation.isPending}
                        className="text-red-500 hover:text-red-400 disabled:opacity-50"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="text-center py-8 text-gray-500">
                  <Users className="w-12 h-12 mx-auto mb-2 opacity-50" />
                  <p>No services have access to this secret</p>
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Audit Log Modal */}
      {showAuditLog && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-gray-800 rounded-xl p-6 w-full max-w-4xl border border-gray-700 max-h-[80vh] overflow-y-auto">
            <div className="flex items-center justify-between mb-6">
              <div className="flex items-center gap-3">
                <FileText className="w-6 h-6 text-purple-500" />
                <h2 className="text-xl font-semibold text-white">
                  Audit Log: {showAuditLog}
                </h2>
              </div>
              <button
                onClick={() => setShowAuditLog(null)}
                className="text-gray-400 hover:text-white"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {auditLogs && auditLogs.length > 0 ? (
              <div className="space-y-3">
                {auditLogs.map((log: AuditLog) => (
                  <div
                    key={log.id}
                    className="p-4 bg-gray-700/50 rounded-lg border border-gray-600"
                  >
                    <div className="flex items-start justify-between mb-2">
                      <div className="flex items-center gap-2">
                        <span className={`px-2 py-1 rounded text-xs font-medium ${
                          log.action === 'created' ? 'bg-green-500/20 text-green-400' :
                          log.action === 'accessed' ? 'bg-blue-500/20 text-blue-400' :
                          log.action === 'deleted' ? 'bg-red-500/20 text-red-400' :
                          'bg-gray-500/20 text-gray-400'
                        }`}>
                          {log.action.toUpperCase()}
                        </span>
                        {log.service_name && (
                          <span className="text-gray-400 text-sm">by {log.service_name}</span>
                        )}
                      </div>
                      <div className="flex items-center gap-1 text-gray-500 text-xs">
                        <Clock className="w-3 h-3" />
                        {formatDate(log.timestamp)}
                      </div>
                    </div>
                    {log.details && (
                      <p className="text-gray-400 text-sm">{log.details}</p>
                    )}
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-center py-12 text-gray-500">
                <FileText className="w-16 h-16 mx-auto mb-3 opacity-50" />
                <p>No audit logs available</p>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Secrets List */}
      <div className="bg-gray-800 rounded-xl border border-gray-700">
        {isLoading ? (
          <div className="p-8 text-center">
            <Loader2 className="w-8 h-8 text-gray-400 animate-spin mx-auto mb-2" />
            <p className="text-gray-400">Loading secrets...</p>
          </div>
        ) : secrets && secrets.length > 0 ? (
          <table className="w-full">
            <thead>
              <tr className="border-b border-gray-700">
                <th className="text-left text-gray-400 font-medium px-6 py-4">Name</th>
                <th className="text-left text-gray-400 font-medium px-6 py-4">Version</th>
                <th className="text-left text-gray-400 font-medium px-6 py-4">Created</th>
                <th className="text-left text-gray-400 font-medium px-6 py-4">Last Accessed</th>
                <th className="text-left text-gray-400 font-medium px-6 py-4">Actions</th>
              </tr>
            </thead>
            <tbody>
              {secrets.map((secret: Secret) => (
                <tr key={secret.id} className="border-b border-gray-700 last:border-0 hover:bg-gray-700/30">
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-3">
                      <Key className="w-5 h-5 text-green-500" />
                      <span className="text-white font-medium">{secret.name}</span>
                    </div>
                  </td>
                  <td className="px-6 py-4 text-gray-400">v{secret.version}</td>
                  <td className="px-6 py-4 text-gray-400 text-sm">
                    {formatDate(secret.created_at)}
                  </td>
                  <td className="px-6 py-4 text-gray-400 text-sm">
                    {formatDate(secret.last_accessed)}
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-2">
                      <button
                        onClick={() => setShowPermissions(secret.name)}
                        className="p-2 text-blue-500 hover:text-blue-400 hover:bg-blue-500/10 rounded transition-colors"
                        title="Manage Permissions"
                      >
                        <Users className="w-4 h-4" />
                      </button>
                      <button
                        onClick={() => setShowAuditLog(secret.name)}
                        className="p-2 text-purple-500 hover:text-purple-400 hover:bg-purple-500/10 rounded transition-colors"
                        title="View Audit Log"
                      >
                        <FileText className="w-4 h-4" />
                      </button>
                      <button
                        onClick={() => setShowDelete(secret.name)}
                        className="p-2 text-red-500 hover:text-red-400 hover:bg-red-500/10 rounded transition-colors"
                        title="Delete Secret"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : (
          <div className="p-12 text-center">
            <Key className="w-16 h-16 mx-auto mb-4 text-gray-600" />
            <p className="text-gray-400 mb-2">No secrets yet</p>
            <p className="text-gray-500 text-sm mb-4">
              Create your first secret to securely store sensitive data
            </p>
            <button
              onClick={() => setShowCreate(true)}
              className="inline-flex items-center gap-2 bg-green-600 hover:bg-green-700 text-white px-4 py-2 rounded-lg transition-colors"
            >
              <Plus className="w-4 h-4" />
              Create Secret
            </button>
          </div>
        )}
      </div>

      {/* Security Info */}
      <div className="mt-8 grid grid-cols-1 md:grid-cols-3 gap-6">
        <div className="bg-gray-800 rounded-xl border border-gray-700 p-6">
          <div className="flex items-center gap-3 mb-3">
            <Shield className="w-6 h-6 text-green-500" />
            <h3 className="text-white font-semibold">TEE Protection</h3>
          </div>
          <p className="text-gray-400 text-sm">
            All secrets are encrypted and stored within the Trusted Execution Environment,
            protected from unauthorized access.
          </p>
        </div>
        <div className="bg-gray-800 rounded-xl border border-gray-700 p-6">
          <div className="flex items-center gap-3 mb-3">
            <Users className="w-6 h-6 text-blue-500" />
            <h3 className="text-white font-semibold">Access Control</h3>
          </div>
          <p className="text-gray-400 text-sm">
            Fine-grained permissions control which services can access each secret.
            Revoke access anytime.
          </p>
        </div>
        <div className="bg-gray-800 rounded-xl border border-gray-700 p-6">
          <div className="flex items-center gap-3 mb-3">
            <FileText className="w-6 h-6 text-purple-500" />
            <h3 className="text-white font-semibold">Audit Trail</h3>
          </div>
          <p className="text-gray-400 text-sm">
            Complete audit log of all secret operations including creation, access,
            and permission changes.
          </p>
        </div>
      </div>
    </div>
  );
}
