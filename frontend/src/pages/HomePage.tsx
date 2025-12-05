import React from 'react';
import { Link } from 'react-router-dom';
import {
  FiShield,
  FiZap,
  FiLock,
  FiDatabase,
  FiCpu,
  FiGlobe,
  FiArrowRight,
  FiCheck,
  FiGithub,
  FiBook,
  FiPlay,
  FiActivity,
} from 'react-icons/fi';

const features = [
  {
    icon: FiShield,
    title: 'TEE-Protected Execution',
    description: 'All sensitive operations run inside Intel SGX enclaves. Private keys and secrets never leave the trusted execution environment.',
  },
  {
    icon: FiLock,
    title: 'Secure Secrets Management',
    description: 'Store API keys, credentials, and sensitive data with hardware-level encryption. Access secrets securely in your functions.',
  },
  {
    icon: FiDatabase,
    title: 'Oracle & Data Feeds',
    description: 'Fetch real-world data securely. Price feeds, weather data, sports scores - all verified and delivered on-chain.',
  },
  {
    icon: FiZap,
    title: 'Verifiable Random Functions',
    description: 'Generate provably fair random numbers for gaming, NFTs, and lotteries with cryptographic proofs.',
  },
  {
    icon: FiCpu,
    title: 'Automation Engine',
    description: 'Schedule and automate smart contract interactions. Cron jobs, event triggers, and conditional execution.',
  },
  {
    icon: FiGlobe,
    title: 'Cross-Chain Support',
    description: 'Native support for Neo N3 with extensible architecture for multi-chain deployments.',
  },
];

const services = [
  { name: 'Oracle', status: 'active', requests: '1.2M+', description: 'External data delivery' },
  { name: 'VRF', status: 'active', requests: '500K+', description: 'Verifiable randomness' },
  { name: 'Secrets', status: 'active', requests: '2.1M+', description: 'Secure key storage' },
  { name: 'DataFeeds', status: 'active', requests: '3.5M+', description: 'Price & market data' },
  { name: 'Automation', status: 'active', requests: '800K+', description: 'Scheduled execution' },
  { name: 'GasBank', status: 'active', requests: '1.8M+', description: 'Gas sponsorship' },
  { name: 'Mixer', status: 'active', requests: '150K+', description: 'Privacy transactions' },
  { name: 'CCIP', status: 'active', requests: '200K+', description: 'Cross-chain messaging' },
];

const stats = [
  { label: 'Total Requests', value: '10M+' },
  { label: 'Active Services', value: '14' },
  { label: 'Uptime', value: '99.99%' },
  { label: 'TEE Attestations', value: '50K+' },
];

export function HomePage() {
  return (
    <div className="min-h-screen bg-gradient-to-b from-surface-900 via-surface-800 to-surface-900">
      {/* Navigation */}
      <nav className="fixed top-0 left-0 right-0 z-50 bg-surface-900/80 backdrop-blur-lg border-b border-surface-700">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between h-16">
            <div className="flex items-center gap-2">
              <div className="w-8 h-8 bg-gradient-to-br from-primary-500 to-accent-500 rounded-lg flex items-center justify-center">
                <FiShield className="w-5 h-5 text-white" />
              </div>
              <span className="text-xl font-bold text-white">Neo Service Layer</span>
            </div>
            <div className="hidden md:flex items-center gap-8">
              <Link to="/docs" className="text-surface-300 hover:text-white transition-colors">Docs</Link>
              <Link to="/playground" className="text-surface-300 hover:text-white transition-colors">Playground</Link>
              <Link to="/services" className="text-surface-300 hover:text-white transition-colors">Services</Link>
              <a href="https://github.com/R3E-Network/service_layer" target="_blank" rel="noopener noreferrer" className="text-surface-300 hover:text-white transition-colors">
                <FiGithub className="w-5 h-5" />
              </a>
            </div>
            <div className="flex items-center gap-4">
              <Link to="/login" className="text-surface-300 hover:text-white transition-colors">Sign In</Link>
              <Link to="/register" className="px-4 py-2 bg-primary-600 hover:bg-primary-500 text-white rounded-lg transition-colors">
                Get Started
              </Link>
            </div>
          </div>
        </div>
      </nav>

      {/* Hero Section */}
      <section className="pt-32 pb-20 px-4">
        <div className="max-w-7xl mx-auto text-center">
          <div className="inline-flex items-center gap-2 px-4 py-2 bg-primary-500/10 border border-primary-500/20 rounded-full mb-8">
            <span className="w-2 h-2 bg-green-500 rounded-full animate-pulse" />
            <span className="text-primary-400 text-sm font-medium">All 14 Services Running</span>
          </div>
          <h1 className="text-5xl md:text-7xl font-bold text-white mb-6 leading-tight">
            TEE-Powered<br />
            <span className="bg-gradient-to-r from-primary-400 to-accent-400 bg-clip-text text-transparent">
              Blockchain Services
            </span>
          </h1>
          <p className="text-xl text-surface-300 max-w-3xl mx-auto mb-10">
            Secure oracle, VRF, secrets management, and automation services for Neo N3.
            All operations protected by Intel SGX Trusted Execution Environment.
          </p>
          <div className="flex flex-col sm:flex-row items-center justify-center gap-4">
            <Link to="/register" className="w-full sm:w-auto px-8 py-4 bg-primary-600 hover:bg-primary-500 text-white rounded-xl font-semibold transition-all hover:scale-105 flex items-center justify-center gap-2">
              Start Building <FiArrowRight />
            </Link>
            <Link to="/docs" className="w-full sm:w-auto px-8 py-4 bg-surface-700 hover:bg-surface-600 text-white rounded-xl font-semibold transition-all flex items-center justify-center gap-2">
              <FiBook className="w-5 h-5" /> Read Docs
            </Link>
            <Link to="/playground" className="w-full sm:w-auto px-8 py-4 border border-surface-600 hover:border-surface-500 text-white rounded-xl font-semibold transition-all flex items-center justify-center gap-2">
              <FiPlay className="w-5 h-5" /> Try Playground
            </Link>
          </div>
        </div>
      </section>

      {/* Stats Section */}
      <section className="py-12 border-y border-surface-700 bg-surface-800/50">
        <div className="max-w-7xl mx-auto px-4">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-8">
            {stats.map((stat) => (
              <div key={stat.label} className="text-center">
                <div className="text-3xl md:text-4xl font-bold text-white mb-2">{stat.value}</div>
                <div className="text-surface-400">{stat.label}</div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Features Section */}
      <section className="py-20 px-4">
        <div className="max-w-7xl mx-auto">
          <div className="text-center mb-16">
            <h2 className="text-3xl md:text-4xl font-bold text-white mb-4">
              Enterprise-Grade Security
            </h2>
            <p className="text-surface-400 max-w-2xl mx-auto">
              Built on TEE technology with hardware-level isolation. Your secrets and operations are protected by the most advanced security available.
            </p>
          </div>
          <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-6">
            {features.map((feature) => (
              <div key={feature.title} className="p-6 bg-surface-800/50 border border-surface-700 rounded-2xl hover:border-primary-500/50 transition-all group">
                <div className="w-12 h-12 bg-primary-500/10 rounded-xl flex items-center justify-center mb-4 group-hover:bg-primary-500/20 transition-colors">
                  <feature.icon className="w-6 h-6 text-primary-400" />
                </div>
                <h3 className="text-xl font-semibold text-white mb-2">{feature.title}</h3>
                <p className="text-surface-400">{feature.description}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Services Grid */}
      <section className="py-20 px-4 bg-surface-800/30">
        <div className="max-w-7xl mx-auto">
          <div className="text-center mb-16">
            <h2 className="text-3xl md:text-4xl font-bold text-white mb-4">
              Available Services
            </h2>
            <p className="text-surface-400 max-w-2xl mx-auto">
              14 production-ready services covering all your blockchain infrastructure needs.
            </p>
          </div>
          <div className="grid sm:grid-cols-2 lg:grid-cols-4 gap-4">
            {services.map((service) => (
              <Link
                key={service.name}
                to={`/services/${service.name.toLowerCase()}`}
                className="p-5 bg-surface-800 border border-surface-700 rounded-xl hover:border-primary-500/50 hover:bg-surface-700/50 transition-all group"
              >
                <div className="flex items-center justify-between mb-3">
                  <h3 className="text-lg font-semibold text-white">{service.name}</h3>
                  <span className="flex items-center gap-1 text-xs text-green-400">
                    <span className="w-1.5 h-1.5 bg-green-500 rounded-full" />
                    Active
                  </span>
                </div>
                <p className="text-sm text-surface-400 mb-3">{service.description}</p>
                <div className="flex items-center justify-between">
                  <span className="text-xs text-surface-500">{service.requests} requests</span>
                  <FiArrowRight className="w-4 h-4 text-surface-500 group-hover:text-primary-400 transition-colors" />
                </div>
              </Link>
            ))}
          </div>
        </div>
      </section>

      {/* Architecture Section */}
      <section className="py-20 px-4">
        <div className="max-w-7xl mx-auto">
          <div className="grid lg:grid-cols-2 gap-12 items-center">
            <div>
              <h2 className="text-3xl md:text-4xl font-bold text-white mb-6">
                Three-Layer TEE Architecture
              </h2>
              <p className="text-surface-400 mb-8">
                Our architecture ensures maximum security through hardware isolation while maintaining flexibility and performance.
              </p>
              <div className="space-y-4">
                {[
                  { layer: 'TEE Trust Root', desc: 'Hardware-backed key management and attestation' },
                  { layer: 'Platform/ServiceOS', desc: 'Capability-based access control and resource management' },
                  { layer: 'Services Layer', desc: '14 specialized services with isolated execution' },
                ].map((item) => (
                  <div key={item.layer} className="flex items-start gap-3">
                    <div className="w-6 h-6 bg-primary-500/20 rounded-full flex items-center justify-center flex-shrink-0 mt-0.5">
                      <FiCheck className="w-4 h-4 text-primary-400" />
                    </div>
                    <div>
                      <h4 className="text-white font-medium">{item.layer}</h4>
                      <p className="text-surface-400 text-sm">{item.desc}</p>
                    </div>
                  </div>
                ))}
              </div>
            </div>
            <div className="bg-surface-800 border border-surface-700 rounded-2xl p-6">
              <div className="space-y-4">
                <div className="p-4 bg-gradient-to-r from-red-500/10 to-orange-500/10 border border-red-500/20 rounded-xl">
                  <div className="flex items-center gap-2 mb-2">
                    <FiCpu className="w-5 h-5 text-red-400" />
                    <span className="text-white font-medium">TEE Trust Root</span>
                  </div>
                  <p className="text-xs text-surface-400">Intel SGX Enclave • Sealing • Attestation</p>
                </div>
                <div className="p-4 bg-gradient-to-r from-blue-500/10 to-cyan-500/10 border border-blue-500/20 rounded-xl">
                  <div className="flex items-center gap-2 mb-2">
                    <FiShield className="w-5 h-5 text-blue-400" />
                    <span className="text-white font-medium">Platform / ServiceOS</span>
                  </div>
                  <p className="text-xs text-surface-400">Capabilities • Secrets • Keys • Storage • Network</p>
                </div>
                <div className="p-4 bg-gradient-to-r from-green-500/10 to-emerald-500/10 border border-green-500/20 rounded-xl">
                  <div className="flex items-center gap-2 mb-2">
                    <FiActivity className="w-5 h-5 text-green-400" />
                    <span className="text-white font-medium">Services Layer</span>
                  </div>
                  <p className="text-xs text-surface-400">Oracle • VRF • Secrets • DataFeeds • Automation • Mixer • ...</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* CTA Section */}
      <section className="py-20 px-4">
        <div className="max-w-4xl mx-auto text-center">
          <h2 className="text-3xl md:text-4xl font-bold text-white mb-6">
            Ready to Build?
          </h2>
          <p className="text-surface-400 mb-8 max-w-2xl mx-auto">
            Get started with Neo Service Layer in minutes. Create an account, get your API keys, and start integrating secure blockchain services.
          </p>
          <div className="flex flex-col sm:flex-row items-center justify-center gap-4">
            <Link to="/register" className="w-full sm:w-auto px-8 py-4 bg-primary-600 hover:bg-primary-500 text-white rounded-xl font-semibold transition-all hover:scale-105">
              Create Free Account
            </Link>
            <Link to="/docs/quickstart" className="w-full sm:w-auto px-8 py-4 border border-surface-600 hover:border-surface-500 text-white rounded-xl font-semibold transition-all">
              View Quickstart Guide
            </Link>
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="py-12 px-4 border-t border-surface-700">
        <div className="max-w-7xl mx-auto">
          <div className="grid md:grid-cols-4 gap-8 mb-8">
            <div>
              <div className="flex items-center gap-2 mb-4">
                <div className="w-8 h-8 bg-gradient-to-br from-primary-500 to-accent-500 rounded-lg flex items-center justify-center">
                  <FiShield className="w-5 h-5 text-white" />
                </div>
                <span className="text-lg font-bold text-white">Neo Service Layer</span>
              </div>
              <p className="text-surface-400 text-sm">
                TEE-powered blockchain services for the Neo ecosystem.
              </p>
            </div>
            <div>
              <h4 className="text-white font-semibold mb-4">Services</h4>
              <ul className="space-y-2 text-surface-400 text-sm">
                <li><Link to="/services/oracle" className="hover:text-white transition-colors">Oracle</Link></li>
                <li><Link to="/services/vrf" className="hover:text-white transition-colors">VRF</Link></li>
                <li><Link to="/services/secrets" className="hover:text-white transition-colors">Secrets</Link></li>
                <li><Link to="/services/datafeeds" className="hover:text-white transition-colors">DataFeeds</Link></li>
              </ul>
            </div>
            <div>
              <h4 className="text-white font-semibold mb-4">Resources</h4>
              <ul className="space-y-2 text-surface-400 text-sm">
                <li><Link to="/docs" className="hover:text-white transition-colors">Documentation</Link></li>
                <li><Link to="/playground" className="hover:text-white transition-colors">Playground</Link></li>
                <li><Link to="/docs/api" className="hover:text-white transition-colors">API Reference</Link></li>
                <li><a href="https://github.com/R3E-Network/service_layer" className="hover:text-white transition-colors">GitHub</a></li>
              </ul>
            </div>
            <div>
              <h4 className="text-white font-semibold mb-4">Monitoring</h4>
              <ul className="space-y-2 text-surface-400 text-sm">
                <li><a href="http://localhost:3001" className="hover:text-white transition-colors">Grafana Dashboard</a></li>
                <li><a href="http://localhost:9091" className="hover:text-white transition-colors">Prometheus</a></li>
                <li><Link to="/status" className="hover:text-white transition-colors">Service Status</Link></li>
              </ul>
            </div>
          </div>
          <div className="pt-8 border-t border-surface-700 flex flex-col md:flex-row items-center justify-between gap-4">
            <p className="text-surface-500 text-sm">
              © 2024 R3E Network. All rights reserved.
            </p>
            <div className="flex items-center gap-6 text-surface-400 text-sm">
              <a href="#" className="hover:text-white transition-colors">Privacy</a>
              <a href="#" className="hover:text-white transition-colors">Terms</a>
              <a href="https://github.com/R3E-Network/service_layer" className="hover:text-white transition-colors">
                <FiGithub className="w-5 h-5" />
              </a>
            </div>
          </div>
        </div>
      </footer>
    </div>
  );
}
