import { useQuery } from '@tanstack/react-query';
import { Server, Shield, Wallet, Clock, Key } from 'lucide-react';
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

  const formatGas = (amount: number) => (amount / 1e8).toFixed(4);

  const { data: secrets } = useQuery({
    queryKey: ['secrets'],
    queryFn: () => api.listSecrets(),
  });

  const serviceNames = [
    'DataFeeds',
    'Automation',
    'ConfCompute',
    'ConfOracle',
    'Account Pool',
    'Global Signer',
    'Gateway',
  ];

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
      name: 'Secrets',
      value: (secrets?.length || 0).toString(),
      icon: Key, 
      color: 'text-purple-500' 
    },
    { 
      name: 'Services Active', 
      value: serviceNames.length.toString(), 
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

        {/* Secrets */}
        <div className="bg-gray-800 rounded-xl border border-gray-700">
          <div className="px-6 py-4 border-b border-gray-700">
            <h2 className="text-xl font-semibold text-white flex items-center gap-2">
              <Key className="w-5 h-5" />
              Secrets
            </h2>
          </div>
          <div className="p-6">
            {secrets && secrets.length > 0 ? (
              <div className="space-y-3">
                {secrets.slice(0, 5).map((secret) => (
                  <div key={secret.id} className="flex items-center justify-between py-2 border-b border-gray-700 last:border-0">
                    <div>
                      <p className="text-white text-sm font-mono">{secret.name}</p>
                      <p className="text-gray-500 text-xs">
                        Updated {new Date(secret.updated_at).toLocaleString('en-US', { 
                          month: 'short', 
                          day: 'numeric', 
                          hour: '2-digit', 
                          minute: '2-digit' 
                        })}
                      </p>
                    </div>
                    <div className="text-right">
                      <span className="text-xs px-2 py-0.5 rounded bg-purple-500/10 text-purple-400">
                        v{secret.version}
                      </span>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-gray-400 text-center py-8">No secrets yet</p>
            )}
          </div>
        </div>
      </div>

      {/* Services Overview */}
      <div className="bg-gray-800 rounded-xl p-6 border border-gray-700">
        <h2 className="text-xl font-semibold text-white mb-4">Services Overview</h2>
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-4">
          {serviceNames.map((service) => (
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
