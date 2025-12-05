import React, { useEffect } from 'react';
import { Link } from 'react-router-dom';
import {
  FiCloud,
  FiShuffle,
  FiActivity,
  FiZap,
  FiLock,
  FiDollarSign,
  FiLink,
  FiDatabase,
  FiShield,
  FiCpu,
  FiArrowRight,
  FiCheckCircle,
  FiXCircle,
} from 'react-icons/fi';
import { Card, Badge } from '@/components/common';
import { useServicesStore } from '@/stores/servicesStore';

interface ServiceInfo {
  id: string;
  name: string;
  description: string;
  icon: React.ElementType;
  href: string;
  color: string;
  bgColor: string;
  features: string[];
}

const SERVICES: ServiceInfo[] = [
  {
    id: 'oracle',
    name: 'Oracle',
    description: 'Fetch external data with TEE-protected HTTP requests',
    icon: FiCloud,
    href: '/services/oracle',
    color: 'text-blue-600',
    bgColor: 'bg-blue-100',
    features: ['HTTP/HTTPS requests', 'JSON parsing', 'Scheduled fetches', 'Signed responses'],
  },
  {
    id: 'vrf',
    name: 'VRF',
    description: 'Verifiable random function for provably fair randomness',
    icon: FiShuffle,
    href: '/services/vrf',
    color: 'text-purple-600',
    bgColor: 'bg-purple-100',
    features: ['Cryptographic proofs', 'On-chain verification', 'Callback support', 'Seed customization'],
  },
  {
    id: 'datafeeds',
    name: 'DataFeeds',
    description: 'Real-time price feeds with multi-source aggregation',
    icon: FiActivity,
    href: '/services/datafeeds',
    color: 'text-green-600',
    bgColor: 'bg-green-100',
    features: ['Price aggregation', 'Deviation triggers', 'Historical data', 'Contract push'],
  },
  {
    id: 'automation',
    name: 'Automation',
    description: 'Schedule and automate on-chain operations',
    icon: FiZap,
    href: '/services/automation',
    color: 'text-yellow-600',
    bgColor: 'bg-yellow-100',
    features: ['Cron scheduling', 'Event triggers', 'Condition-based', 'Gas optimization'],
  },
  {
    id: 'secrets',
    name: 'Secrets',
    description: 'Secure secret storage with TEE encryption',
    icon: FiLock,
    href: '/secrets',
    color: 'text-red-600',
    bgColor: 'bg-red-100',
    features: ['AES-256 encryption', 'Version control', 'Access control', 'Audit logging'],
  },
  {
    id: 'gasbank',
    name: 'GasBank',
    description: 'Manage gas fees and sponsor transactions',
    icon: FiDollarSign,
    href: '/gasbank',
    color: 'text-emerald-600',
    bgColor: 'bg-emerald-100',
    features: ['Balance management', 'Gas sponsorship', 'Usage tracking', 'Auto top-up'],
  },
  {
    id: 'ccip',
    name: 'CCIP',
    description: 'Cross-chain interoperability protocol',
    icon: FiLink,
    href: '/services/ccip',
    color: 'text-indigo-600',
    bgColor: 'bg-indigo-100',
    features: ['Cross-chain messaging', 'Token transfers', 'Multi-chain support', 'Proof verification'],
  },
  {
    id: 'confidential',
    name: 'Confidential',
    description: 'Execute confidential computations in TEE',
    icon: FiShield,
    href: '/services/confidential',
    color: 'text-pink-600',
    bgColor: 'bg-pink-100',
    features: ['Secure execution', 'Data privacy', 'Attestation', 'Sealed outputs'],
  },
  {
    id: 'datalink',
    name: 'DataLink',
    description: 'Sync data between on-chain and off-chain',
    icon: FiDatabase,
    href: '/services/datalink',
    color: 'text-cyan-600',
    bgColor: 'bg-cyan-100',
    features: ['Bi-directional sync', 'Schema mapping', 'Conflict resolution', 'Real-time updates'],
  },
  {
    id: 'cre',
    name: 'CRE',
    description: 'Chainlink Runtime Environment for workflows',
    icon: FiCpu,
    href: '/services/cre',
    color: 'text-orange-600',
    bgColor: 'bg-orange-100',
    features: ['Workflow engine', 'Step orchestration', 'Error handling', 'Retry logic'],
  },
];

export function ServicesPage() {
  const { configs, fetchConfigs } = useServicesStore();

  useEffect(() => {
    fetchConfigs();
  }, [fetchConfigs]);

  const getServiceStatus = (serviceId: string) => {
    const config = configs.find((c) => c.serviceType === serviceId);
    return config?.enabled ?? false;
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-surface-900">Services</h1>
        <p className="text-surface-500 mt-1">
          Configure and manage TEE-powered blockchain services
        </p>
      </div>

      {/* Services Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {SERVICES.map((service) => {
          const isActive = getServiceStatus(service.id);

          return (
            <Link key={service.id} to={service.href}>
              <Card hover className="h-full group">
                <div className="flex items-start justify-between mb-4">
                  <div className={`p-3 rounded-xl ${service.bgColor}`}>
                    <service.icon className={`w-6 h-6 ${service.color}`} />
                  </div>
                  <div className="flex items-center gap-2">
                    {isActive ? (
                      <Badge variant="success" size="sm">
                        <FiCheckCircle className="w-3 h-3 mr-1" />
                        Active
                      </Badge>
                    ) : (
                      <Badge variant="default" size="sm">
                        <FiXCircle className="w-3 h-3 mr-1" />
                        Inactive
                      </Badge>
                    )}
                  </div>
                </div>

                <h3 className="text-lg font-semibold text-surface-900 group-hover:text-primary-600 transition-colors">
                  {service.name}
                </h3>
                <p className="text-sm text-surface-500 mt-1">{service.description}</p>

                <div className="mt-4 flex flex-wrap gap-1">
                  {service.features.slice(0, 3).map((feature) => (
                    <span
                      key={feature}
                      className="px-2 py-0.5 text-xs bg-surface-100 text-surface-600 rounded"
                    >
                      {feature}
                    </span>
                  ))}
                </div>

                <div className="mt-4 flex items-center text-sm text-primary-600 opacity-0 group-hover:opacity-100 transition-opacity">
                  Configure
                  <FiArrowRight className="w-4 h-4 ml-1" />
                </div>
              </Card>
            </Link>
          );
        })}
      </div>

      {/* Quick Stats */}
      <Card>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <div className="text-center p-4">
            <p className="text-3xl font-bold text-surface-900">
              {configs.filter((c) => c.enabled).length}
            </p>
            <p className="text-sm text-surface-500">Active Services</p>
          </div>
          <div className="text-center p-4">
            <p className="text-3xl font-bold text-surface-900">{SERVICES.length}</p>
            <p className="text-sm text-surface-500">Available Services</p>
          </div>
          <div className="text-center p-4">
            <p className="text-3xl font-bold text-green-600">99.9%</p>
            <p className="text-sm text-surface-500">Uptime</p>
          </div>
          <div className="text-center p-4">
            <p className="text-3xl font-bold text-surface-900">TEE</p>
            <p className="text-sm text-surface-500">Protected</p>
          </div>
        </div>
      </Card>
    </div>
  );
}
