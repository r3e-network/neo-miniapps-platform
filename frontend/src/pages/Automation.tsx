import { useQuery } from '@tanstack/react-query';
import { Zap, Plus, Play, Pause } from 'lucide-react';
import { api } from '../api/client';

export function Automation() {
  const { data: triggers, isLoading } = useQuery({
    queryKey: ['triggers'],
    queryFn: () => api.listTriggers(),
  });

  return (
    <div>
      <div className="flex items-center justify-between mb-8">
        <h1 className="text-3xl font-bold text-white">Automation</h1>
        <button className="flex items-center gap-2 bg-green-600 hover:bg-green-700 text-white px-4 py-2 rounded-lg transition-colors">
          <Plus className="w-5 h-5" />
          Create Trigger
        </button>
      </div>

      {/* Triggers List */}
      <div className="bg-gray-800 rounded-xl border border-gray-700">
        {isLoading ? (
          <div className="p-8 text-center text-gray-400">Loading...</div>
        ) : triggers?.length === 0 ? (
          <div className="p-8 text-center text-gray-400">
            <Zap className="w-12 h-12 mx-auto mb-4 opacity-50" />
            <p>No automation triggers yet.</p>
            <p className="text-sm mt-2">Create triggers to automate your workflows.</p>
          </div>
        ) : (
          <div className="divide-y divide-gray-700">
            {triggers?.map((trigger) => (
              <div key={trigger.id} className="p-6 flex items-center justify-between">
                <div className="flex items-center gap-4">
                  <div className={`p-2 rounded-lg ${trigger.enabled ? 'bg-green-500/20' : 'bg-gray-700'}`}>
                    <Zap className={`w-5 h-5 ${trigger.enabled ? 'text-green-500' : 'text-gray-500'}`} />
                  </div>
                  <div>
                    <h3 className="text-white font-medium">{trigger.name}</h3>
                    <p className="text-gray-400 text-sm">ID: {trigger.id}</p>
                  </div>
                </div>
                <button className={`p-2 rounded-lg ${trigger.enabled ? 'bg-red-500/20 text-red-500' : 'bg-green-500/20 text-green-500'}`}>
                  {trigger.enabled ? <Pause className="w-5 h-5" /> : <Play className="w-5 h-5" />}
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
