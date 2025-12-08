import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Key, Plus, Trash2 } from 'lucide-react';
import { api } from '../api/client';

export function Secrets() {
  const queryClient = useQueryClient();
  const [showCreate, setShowCreate] = useState(false);
  const [newSecret, setNewSecret] = useState({ name: '', value: '' });

  const { data: secrets, isLoading } = useQuery({
    queryKey: ['secrets'],
    queryFn: () => api.listSecrets(),
  });

  const createMutation = useMutation({
    mutationFn: () => api.createSecret(newSecret.name, newSecret.value),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['secrets'] });
      setShowCreate(false);
      setNewSecret({ name: '', value: '' });
    },
  });

  return (
    <div>
      <div className="flex items-center justify-between mb-8">
        <h1 className="text-3xl font-bold text-white">Secrets</h1>
        <button
          onClick={() => setShowCreate(true)}
          className="flex items-center gap-2 bg-green-600 hover:bg-green-700 text-white px-4 py-2 rounded-lg transition-colors"
        >
          <Plus className="w-5 h-5" />
          Add Secret
        </button>
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
              </div>
              <div>
                <label className="block text-sm text-gray-400 mb-1">Value</label>
                <input
                  type="password"
                  value={newSecret.value}
                  onChange={(e) => setNewSecret({ ...newSecret, value: e.target.value })}
                  className="w-full bg-gray-700 border border-gray-600 rounded-lg px-3 py-2 text-white"
                  placeholder="••••••••"
                />
              </div>
            </div>
            <div className="flex gap-3 mt-6">
              <button
                onClick={() => setShowCreate(false)}
                className="flex-1 bg-gray-700 hover:bg-gray-600 text-white py-2 rounded-lg"
              >
                Cancel
              </button>
              <button
                onClick={() => createMutation.mutate()}
                disabled={createMutation.isPending}
                className="flex-1 bg-green-600 hover:bg-green-700 text-white py-2 rounded-lg"
              >
                {createMutation.isPending ? 'Creating...' : 'Create'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Secrets List */}
      <div className="bg-gray-800 rounded-xl border border-gray-700">
        {isLoading ? (
          <div className="p-8 text-center text-gray-400">Loading...</div>
        ) : secrets?.length === 0 ? (
          <div className="p-8 text-center text-gray-400">
            <Key className="w-12 h-12 mx-auto mb-4 opacity-50" />
            <p>No secrets yet. Create your first secret to get started.</p>
          </div>
        ) : (
          <table className="w-full">
            <thead>
              <tr className="border-b border-gray-700">
                <th className="text-left text-gray-400 font-medium px-6 py-4">Name</th>
                <th className="text-left text-gray-400 font-medium px-6 py-4">Version</th>
                <th className="text-left text-gray-400 font-medium px-6 py-4">Actions</th>
              </tr>
            </thead>
            <tbody>
              {secrets?.map((secret) => (
                <tr key={secret.id} className="border-b border-gray-700 last:border-0">
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-3">
                      <Key className="w-5 h-5 text-green-500" />
                      <span className="text-white font-medium">{secret.name}</span>
                    </div>
                  </td>
                  <td className="px-6 py-4 text-gray-400">v{secret.version}</td>
                  <td className="px-6 py-4">
                    <button className="text-red-500 hover:text-red-400">
                      <Trash2 className="w-5 h-5" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
