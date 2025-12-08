import { useQuery } from '@tanstack/react-query';
import { Activity, Server, Shield, Zap } from 'lucide-react';
import { api } from '../api/client';

export function Dashboard() {
  const { data: health } = useQuery({
    queryKey: ['health'],
    queryFn: () => api.getHealth(),
    refetchInterval: 30000,
  });

  const stats = [
    { name: 'Services Active', value: '14', icon: Server, color: 'text-green-500' },
    { name: 'Enclave Status', value: health?.enclave ? 'Secure' : 'Simulation', icon: Shield, color: 'text-blue-500' },
    { name: 'Requests Today', value: '1,234', icon: Activity, color: 'text-purple-500' },
    { name: 'Automations', value: '12', icon: Zap, color: 'text-yellow-500' },
  ];

  return (
    <div>
      <h1 className="text-3xl font-bold text-white mb-8">Dashboard</h1>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
        {stats.map((stat) => (
          <div key={stat.name} className="bg-gray-800 rounded-xl p-6 border border-gray-700">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-gray-400 text-sm">{stat.name}</p>
                <p className="text-2xl font-bold text-white mt-1">{stat.value}</p>
              </div>
              <stat.icon className={`w-10 h-10 ${stat.color}`} />
            </div>
          </div>
        ))}
      </div>

      {/* Services Overview */}
      <div className="bg-gray-800 rounded-xl p-6 border border-gray-700">
        <h2 className="text-xl font-semibold text-white mb-4">Services Overview</h2>
        <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-7 gap-4">
          {['Oracle', 'VRF', 'Mixer', 'Secrets', 'DataFeeds', 'GasBank', 'Automation',
            'Confidential', 'Accounts', 'CCIP', 'DataLink', 'DataStreams', 'DTA', 'CRE'].map((service) => (
            <div key={service} className="bg-gray-700 rounded-lg p-3 text-center">
              <div className="w-3 h-3 bg-green-500 rounded-full mx-auto mb-2" />
              <p className="text-sm text-gray-300">{service}</p>
            </div>
          ))}
        </div>
      </div>

      {/* TEE Attestation */}
      <div className="mt-8 bg-gray-800 rounded-xl p-6 border border-gray-700">
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
