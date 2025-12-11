import { useQuery } from '@tanstack/react-query';
import { Activity, Server, Shield, Wallet, TrendingUp, Clock } from 'lucide-react';
import { api } from '../api/client';
import { useAuthStore } from '../stores/auth';

export function Dashboard() {
  const { user } = useAuthStore();

  const { data: health } = useQuery({
    queryKey: ['health'],
    queryFn: () => api.getHealth(),
    refetchInterval: 30000,
  });

  // Fetch user's GAS balance
  const { data: gasAccount } = useQuery({
    queryKey: ['gasbank-account'],
    queryFn: () => api.getGasBankAccount(),
    refetchInterval: 10000,
  });

  // Fetch recent transactions
  const { data: recentTxs } = useQuery({
    queryKey: ['recent-transactions'],
    queryFn: () => api.listTransactions(),
  });

  // Fetch neovault requests
  const { data: neovaultRequests } = useQuery({
    queryKey: ['neovault-requests'],
    queryFn: () => api.getNeoVaultRequests(),
  });

  const formatGas = (amount: number) => (amount / 1e8).toFixed(4);
  
  const activeRequests = neovaultRequests?.filter(r => r.status === 0 || r.status === 1).length || 0;

  const stats = [
    { 
      name: 'GAS Balance', 
      value: gasAccount ? formatGas(gasAccount.balance - gasAccount.reserved) : '0.0000',
      icon: Wallet, 
      color: 'text-green-500',
      suffix: 'GAS'
    },
    { 
      name: 'Enclave Status', 
      value: health?.enclave ? 'Secure' : 'Simulation', 
      icon: Shield, 
      color: health?.enclave ? 'text-blue-500' : 'text-yellow-500'
    },
    { 
      name: 'Active Requests', 
      value: activeRequests.toString(), 
      icon: Activity, 
      color: 'text-purple-500' 
    },
    { 
      name: 'Services Active', 
      value: '9', 
      icon: Server, 
      color: 'text-cyan-500' 
    },
  ];

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-white">Dashboard</h1>
        <p className="text-gray-400 mt-2">
          Welcome back, {user?.address?.slice(0, 8)}...{user?.address?.slice(-6)}
        </p>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
        {stats.map((stat) => (
          <div key={stat.name} className="bg-gray-800 rounded-xl p-6 border border-gray-700 hover:border-gray-600 transition-colors">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-gray-400 text-sm">{stat.name}</p>
                <p className="text-2xl font-bold text-white mt-1">
                  {stat.value} {stat.suffix && <span className="text-lg text-gray-400">{stat.suffix}</span>}
                </p>
              </div>
              <stat.icon className={`w-10 h-10 ${stat.color}`} />
            </div>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
        {/* Recent Activity */}
        <div className="bg-gray-800 rounded-xl border border-gray-700">
          <div className="px-6 py-4 border-b border-gray-700">
            <h2 className="text-xl font-semibold text-white flex items-center gap-2">
              <Clock className="w-5 h-5" />
              Recent Activity
            </h2>
          </div>
          <div className="p-6">
            {recentTxs && recentTxs.length > 0 ? (
              <div className="space-y-3">
                {recentTxs.slice(0, 5).map((tx) => (
                  <div key={tx.id} className="flex items-center justify-between py-2 border-b border-gray-700 last:border-0">
                    <div className="flex items-center gap-3">
                      <div className={`w-2 h-2 rounded-full ${tx.amount > 0 ? 'bg-green-500' : 'bg-red-500'}`} />
                      <div>
                        <p className="text-white text-sm">{tx.tx_type}</p>
                        <p className="text-gray-500 text-xs">
                          {new Date(tx.created_at).toLocaleString('en-US', { 
                            month: 'short', 
                            day: 'numeric', 
                            hour: '2-digit', 
                            minute: '2-digit' 
                          })}
                        </p>
                      </div>
                    </div>
                    <span className={`text-sm font-medium ${tx.amount > 0 ? 'text-green-400' : 'text-red-400'}`}>
                      {tx.amount > 0 ? '+' : ''}{formatGas(tx.amount)} GAS
                    </span>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-gray-400 text-center py-8">No recent activity</p>
            )}
          </div>
        </div>

        {/* Active NeoVault Requests */}
        <div className="bg-gray-800 rounded-xl border border-gray-700">
          <div className="px-6 py-4 border-b border-gray-700">
            <h2 className="text-xl font-semibold text-white flex items-center gap-2">
              <TrendingUp className="w-5 h-5" />
              Active NeoVault Requests
            </h2>
          </div>
          <div className="p-6">
            {neovaultRequests && neovaultRequests.length > 0 ? (
              <div className="space-y-3">
                {neovaultRequests.filter(r => r.status === 0 || r.status === 1).slice(0, 5).map((req) => (
                  <div key={req.request_id} className="flex items-center justify-between py-2 border-b border-gray-700 last:border-0">
                    <div>
                      <p className="text-white text-sm font-mono">{req.request_id.slice(0, 8)}...</p>
                      <p className="text-gray-500 text-xs">
                        {new Date(req.created_at).toLocaleString('en-US', { 
                          month: 'short', 
                          day: 'numeric', 
                          hour: '2-digit', 
                          minute: '2-digit' 
                        })}
                      </p>
                    </div>
                    <div className="text-right">
                      <p className="text-white text-sm">{req.amount} GAS</p>
                      <span className={`text-xs px-2 py-0.5 rounded ${
                        req.status === 0 ? 'bg-yellow-500/10 text-yellow-400' : 'bg-blue-500/10 text-blue-400'
                      }`}>
                        {req.status === 0 ? 'Pending' : 'Processing'}
                      </span>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-gray-400 text-center py-8">No active requests</p>
            )}
          </div>
        </div>
      </div>

      {/* Services Overview */}
      <div className="bg-gray-800 rounded-xl p-6 border border-gray-700">
        <h2 className="text-xl font-semibold text-white mb-4">Services Overview</h2>
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-4">
          {['VRF', 'NeoVault', 'Oracle', 'Secrets', 'NeoFeeds', 'NeoFlow', 'NeoCompute', 'AccountPool', 'GasBank'].map((service) => (
            <div key={service} className="bg-gray-700 rounded-lg p-4 text-center hover:bg-gray-600 transition-colors cursor-pointer">
              <div className="w-3 h-3 bg-green-500 rounded-full mx-auto mb-2" />
              <p className="text-sm text-gray-300">{service}</p>
              <p className="text-xs text-gray-500 mt-1">Online</p>
            </div>
          ))}
        </div>
      </div>

      {/* TEE Attestation */}
      <div className="mt-6 bg-gray-800 rounded-xl p-6 border border-gray-700">
        <h2 className="text-xl font-semibold text-white mb-4">TEE Attestation</h2>
        <div className="flex items-center gap-4">
          <Shield className={`w-12 h-12 ${health?.enclave ? 'text-green-500' : 'text-yellow-500'}`} />
          <div>
            <p className="text-white font-medium">
              {health?.enclave ? 'Running in SGX Enclave' : 'Running in Simulation Mode'}
            </p>
            <p className="text-gray-400 text-sm">
              {health?.enclave
                ? 'All services are protected by Intel SGX hardware enclaves'
                : 'Enable SGX hardware for production security'}
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
