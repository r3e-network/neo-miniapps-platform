import { Server, ExternalLink } from 'lucide-react';

const services = [
  { id: 'neooracle', name: 'NeoOracle', description: 'Allowlisted HTTP fetch with TEE proofs', status: 'active' },
  { id: 'neorand', name: 'NeoRand', description: 'Verifiable randomness (VRF) with proofs', status: 'active' },
  { id: 'neovault', name: 'NeoVault', description: 'Privacy-preserving transactions and dispute proofs', status: 'active' },
  { id: 'neostore', name: 'NeoStore', description: 'Encrypted secrets & permissioned injection', status: 'active' },
  { id: 'neofeeds', name: 'NeoFeeds', description: 'Price feed aggregation & signed updates', status: 'active' },
  { id: 'neoflow', name: 'NeoFlow', description: 'Trigger-based automation & webhooks', status: 'active' },
  { id: 'neocompute', name: 'NeoCompute', description: 'TEE-secured JavaScript execution', status: 'active' },
  { id: 'neoaccounts', name: 'NeoAccounts', description: 'HD-derived account pool & signing', status: 'active' },
  { id: 'gasbank', name: 'GasBank', description: 'Off-chain fee balance management (via Gateway)', status: 'active' },
];

export function Services() {
  return (
    <div>
      <h1 className="text-3xl font-bold text-white mb-8">Services</h1>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {services.map((service) => (
          <div key={service.id} className="bg-gray-800 rounded-xl p-6 border border-gray-700 hover:border-green-500 transition-colors">
            <div className="flex items-start justify-between">
              <div className="flex items-center gap-3">
                <div className="p-2 bg-gray-700 rounded-lg">
                  <Server className="w-6 h-6 text-green-500" />
                </div>
                <div>
                  <h3 className="text-lg font-semibold text-white">{service.name}</h3>
                  <p className="text-gray-400 text-sm">{service.description}</p>
                </div>
              </div>
              <div className={`px-2 py-1 rounded text-xs font-medium ${
                service.status === 'active' ? 'bg-green-500/20 text-green-500' : 'bg-gray-500/20 text-gray-500'
              }`}>
                {service.status}
              </div>
            </div>

            <div className="mt-4 pt-4 border-t border-gray-700">
              <button className="flex items-center gap-2 text-green-500 hover:text-green-400 text-sm font-medium">
                <ExternalLink className="w-4 h-4" />
                View Documentation
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
