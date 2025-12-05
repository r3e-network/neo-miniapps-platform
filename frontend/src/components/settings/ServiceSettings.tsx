import React, { useState } from 'react';
import {
  FiGlobe, FiHash, FiKey, FiDollarSign, FiShuffle, FiTrendingUp,
  FiZap, FiSend, FiLock, FiCode, FiLink, FiRadio, FiShield,
  FiChevronDown, FiChevronUp, FiSettings
} from 'react-icons/fi';
import { Card, CardHeader, Badge, Input } from '@/components/common';

interface ServiceConfig {
  id: string;
  name: string;
  icon: React.ReactNode;
  enabled: boolean;
  settings: {
    key: string;
    label: string;
    type: 'toggle' | 'number' | 'text' | 'select';
    value: any;
    options?: { label: string; value: string }[];
    description?: string;
  }[];
}

const defaultServiceConfigs: ServiceConfig[] = [
  {
    id: 'oracle',
    name: 'Oracle Service',
    icon: <FiGlobe className="w-5 h-5 text-blue-600" />,
    enabled: true,
    settings: [
      { key: 'cache_ttl', label: 'Cache TTL (seconds)', type: 'number', value: 300, description: 'How long to cache responses' },
      { key: 'max_requests', label: 'Max Requests/min', type: 'number', value: 100, description: 'Rate limit per minute' },
      { key: 'timeout', label: 'Request Timeout (ms)', type: 'number', value: 30000 },
    ],
  },
  {
    id: 'vrf',
    name: 'VRF Service',
    icon: <FiHash className="w-5 h-5 text-purple-600" />,
    enabled: true,
    settings: [
      { key: 'callback_gas', label: 'Callback Gas Limit', type: 'number', value: 200000 },
      { key: 'confirmations', label: 'Required Confirmations', type: 'number', value: 1 },
    ],
  },
  {
    id: 'secrets',
    name: 'Secrets Service',
    icon: <FiKey className="w-5 h-5 text-green-600" />,
    enabled: true,
    settings: [
      { key: 'max_secrets', label: 'Max Secrets', type: 'number', value: 50 },
      { key: 'max_size', label: 'Max Secret Size (KB)', type: 'number', value: 64 },
      { key: 'auto_rotate', label: 'Auto-rotate Keys', type: 'toggle', value: false },
    ],
  },
  {
    id: 'gasbank',
    name: 'GasBank Service',
    icon: <FiDollarSign className="w-5 h-5 text-yellow-600" />,
    enabled: true,
    settings: [
      { key: 'auto_topup', label: 'Auto Top-up', type: 'toggle', value: false, description: 'Automatically top up when balance is low' },
      { key: 'min_balance', label: 'Min Balance Alert', type: 'number', value: 10 },
      { key: 'max_tx_amount', label: 'Max Transaction Amount', type: 'number', value: 1000 },
    ],
  },
  {
    id: 'mixer',
    name: 'Mixer Service',
    icon: <FiShuffle className="w-5 h-5 text-purple-600" />,
    enabled: false,
    settings: [
      { key: 'default_pool', label: 'Default Pool', type: 'select', value: 'neo-10', options: [
        { label: '10 NEO', value: 'neo-10' },
        { label: '100 NEO', value: 'neo-100' },
        { label: '100 GAS', value: 'gas-100' },
      ]},
      { key: 'auto_withdraw', label: 'Auto Withdraw', type: 'toggle', value: false },
    ],
  },
  {
    id: 'datafeeds',
    name: 'DataFeeds Service',
    icon: <FiTrendingUp className="w-5 h-5 text-green-600" />,
    enabled: true,
    settings: [
      { key: 'update_interval', label: 'Update Interval (sec)', type: 'number', value: 60 },
      { key: 'deviation_threshold', label: 'Deviation Threshold (%)', type: 'number', value: 1 },
    ],
  },
  {
    id: 'automation',
    name: 'Automation Service',
    icon: <FiZap className="w-5 h-5 text-yellow-600" />,
    enabled: true,
    settings: [
      { key: 'max_triggers', label: 'Max Active Triggers', type: 'number', value: 10 },
      { key: 'retry_count', label: 'Retry Count', type: 'number', value: 3 },
      { key: 'retry_delay', label: 'Retry Delay (sec)', type: 'number', value: 60 },
    ],
  },
  {
    id: 'ccip',
    name: 'CCIP Service',
    icon: <FiSend className="w-5 h-5 text-indigo-600" />,
    enabled: false,
    settings: [
      { key: 'default_dest', label: 'Default Destination', type: 'select', value: 'ethereum', options: [
        { label: 'Ethereum', value: 'ethereum' },
        { label: 'Polygon', value: 'polygon' },
        { label: 'BNB Chain', value: 'bsc' },
      ]},
      { key: 'gas_limit', label: 'Gas Limit', type: 'number', value: 200000 },
    ],
  },
  {
    id: 'confidential',
    name: 'Confidential Computing',
    icon: <FiLock className="w-5 h-5 text-blue-600" />,
    enabled: false,
    settings: [
      { key: 'max_compute_time', label: 'Max Compute Time (sec)', type: 'number', value: 60 },
      { key: 'max_memory', label: 'Max Memory (MB)', type: 'number', value: 128 },
    ],
  },
  {
    id: 'cre',
    name: 'CRE Service',
    icon: <FiCode className="w-5 h-5 text-orange-600" />,
    enabled: false,
    settings: [
      { key: 'max_functions', label: 'Max Functions', type: 'number', value: 5 },
      { key: 'execution_timeout', label: 'Execution Timeout (sec)', type: 'number', value: 30 },
    ],
  },
  {
    id: 'datalink',
    name: 'DataLink Service',
    icon: <FiLink className="w-5 h-5 text-blue-600" />,
    enabled: false,
    settings: [
      { key: 'max_connections', label: 'Max Connections', type: 'number', value: 5 },
      { key: 'sync_interval', label: 'Sync Interval (min)', type: 'number', value: 15 },
    ],
  },
  {
    id: 'datastreams',
    name: 'DataStreams Service',
    icon: <FiRadio className="w-5 h-5 text-cyan-600" />,
    enabled: false,
    settings: [
      { key: 'max_streams', label: 'Max Streams', type: 'number', value: 3 },
      { key: 'buffer_size', label: 'Buffer Size', type: 'number', value: 100 },
    ],
  },
  {
    id: 'dta',
    name: 'DTA Service',
    icon: <FiShield className="w-5 h-5 text-green-600" />,
    enabled: false,
    settings: [
      { key: 'default_validity', label: 'Default Validity (days)', type: 'number', value: 365 },
      { key: 'auto_renew', label: 'Auto Renew', type: 'toggle', value: false },
    ],
  },
];

export function ServiceSettings() {
  const [configs, setConfigs] = useState<ServiceConfig[]>(defaultServiceConfigs);
  const [expandedService, setExpandedService] = useState<string | null>(null);

  const toggleService = (serviceId: string) => {
    setConfigs(configs.map(c =>
      c.id === serviceId ? { ...c, enabled: !c.enabled } : c
    ));
  };

  const updateSetting = (serviceId: string, key: string, value: any) => {
    setConfigs(configs.map(c =>
      c.id === serviceId
        ? { ...c, settings: c.settings.map(s => s.key === key ? { ...s, value } : s) }
        : c
    ));
  };

  const toggleExpand = (serviceId: string) => {
    setExpandedService(expandedService === serviceId ? null : serviceId);
  };

  return (
    <Card>
      <CardHeader
        title="Service Settings"
        description="Configure individual service preferences and limits"
        action={<FiSettings className="w-5 h-5 text-surface-400" />}
      />
      <div className="space-y-2">
        {configs.map((service) => (
          <div key={service.id} className="border border-surface-200 rounded-lg overflow-hidden">
            {/* Service Header */}
            <div
              className="flex items-center justify-between p-4 bg-surface-50 cursor-pointer hover:bg-surface-100 transition-colors"
              onClick={() => toggleExpand(service.id)}
            >
              <div className="flex items-center gap-3">
                <div className="p-2 bg-white rounded-lg shadow-sm">
                  {service.icon}
                </div>
                <div>
                  <p className="font-medium text-surface-900">{service.name}</p>
                  <p className="text-xs text-surface-500">
                    {service.settings.length} settings
                  </p>
                </div>
              </div>
              <div className="flex items-center gap-3">
                <label className="relative inline-flex items-center cursor-pointer" onClick={(e) => e.stopPropagation()}>
                  <input
                    type="checkbox"
                    checked={service.enabled}
                    onChange={() => toggleService(service.id)}
                    className="sr-only peer"
                  />
                  <div className="w-11 h-6 bg-surface-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-primary-300 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-surface-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-primary-600"></div>
                </label>
                {expandedService === service.id ? (
                  <FiChevronUp className="w-5 h-5 text-surface-400" />
                ) : (
                  <FiChevronDown className="w-5 h-5 text-surface-400" />
                )}
              </div>
            </div>

            {/* Service Settings */}
            {expandedService === service.id && (
              <div className="p-4 border-t border-surface-200 space-y-4">
                {service.settings.map((setting) => (
                  <div key={setting.key} className="flex items-center justify-between">
                    <div className="flex-1">
                      <p className="text-sm font-medium text-surface-700">{setting.label}</p>
                      {setting.description && (
                        <p className="text-xs text-surface-400">{setting.description}</p>
                      )}
                    </div>
                    <div className="w-40">
                      {setting.type === 'toggle' && (
                        <label className="relative inline-flex items-center cursor-pointer">
                          <input
                            type="checkbox"
                            checked={setting.value}
                            onChange={(e) => updateSetting(service.id, setting.key, e.target.checked)}
                            className="sr-only peer"
                            disabled={!service.enabled}
                          />
                          <div className="w-11 h-6 bg-surface-200 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-primary-300 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-surface-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-primary-600 peer-disabled:opacity-50"></div>
                        </label>
                      )}
                      {setting.type === 'number' && (
                        <input
                          type="number"
                          value={setting.value}
                          onChange={(e) => updateSetting(service.id, setting.key, parseInt(e.target.value))}
                          disabled={!service.enabled}
                          className="w-full px-3 py-1.5 text-sm border border-surface-300 rounded-lg disabled:opacity-50 disabled:bg-surface-100"
                        />
                      )}
                      {setting.type === 'text' && (
                        <input
                          type="text"
                          value={setting.value}
                          onChange={(e) => updateSetting(service.id, setting.key, e.target.value)}
                          disabled={!service.enabled}
                          className="w-full px-3 py-1.5 text-sm border border-surface-300 rounded-lg disabled:opacity-50 disabled:bg-surface-100"
                        />
                      )}
                      {setting.type === 'select' && setting.options && (
                        <select
                          value={setting.value}
                          onChange={(e) => updateSetting(service.id, setting.key, e.target.value)}
                          disabled={!service.enabled}
                          className="w-full px-3 py-1.5 text-sm border border-surface-300 rounded-lg disabled:opacity-50 disabled:bg-surface-100"
                        >
                          {setting.options.map((opt) => (
                            <option key={opt.value} value={opt.value}>{opt.label}</option>
                          ))}
                        </select>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        ))}
      </div>
    </Card>
  );
}
