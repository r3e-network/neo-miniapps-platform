import React, { useState } from 'react';
import { Link } from 'react-router-dom';
import {
  FiBook,
  FiCode,
  FiZap,
  FiShield,
  FiDatabase,
  FiLock,
  FiCpu,
  FiGlobe,
  FiChevronRight,
  FiSearch,
  FiExternalLink,
  FiCopy,
  FiCheck,
} from 'react-icons/fi';

const docSections = [
  {
    id: 'getting-started',
    title: 'Getting Started',
    icon: FiZap,
    items: [
      { title: 'Introduction', slug: 'introduction' },
      { title: 'Quick Start', slug: 'quickstart' },
      { title: 'Authentication', slug: 'authentication' },
      { title: 'API Keys', slug: 'api-keys' },
    ],
  },
  {
    id: 'architecture',
    title: 'Architecture',
    icon: FiCpu,
    items: [
      { title: 'TEE Trust Root', slug: 'tee-trust-root' },
      { title: 'ServiceOS Layer', slug: 'serviceos' },
      { title: 'Capabilities System', slug: 'capabilities' },
      { title: 'Security Model', slug: 'security' },
    ],
  },
  {
    id: 'services',
    title: 'Services',
    icon: FiDatabase,
    items: [
      { title: 'Oracle Service', slug: 'oracle' },
      { title: 'VRF Service', slug: 'vrf' },
      { title: 'Secrets Service', slug: 'secrets' },
      { title: 'DataFeeds Service', slug: 'datafeeds' },
      { title: 'Automation Service', slug: 'automation' },
      { title: 'GasBank Service', slug: 'gasbank' },
      { title: 'Mixer Service', slug: 'mixer' },
      { title: 'CCIP Service', slug: 'ccip' },
    ],
  },
  {
    id: 'api-reference',
    title: 'API Reference',
    icon: FiCode,
    items: [
      { title: 'REST API', slug: 'rest-api' },
      { title: 'WebSocket API', slug: 'websocket' },
      { title: 'Error Codes', slug: 'errors' },
      { title: 'Rate Limits', slug: 'rate-limits' },
    ],
  },
  {
    id: 'sdks',
    title: 'SDKs & Tools',
    icon: FiGlobe,
    items: [
      { title: 'JavaScript SDK', slug: 'sdk-js' },
      { title: 'Go SDK', slug: 'sdk-go' },
      { title: 'Python SDK', slug: 'sdk-python' },
      { title: 'CLI Tool', slug: 'cli' },
    ],
  },
];

const codeExamples: Record<string, { title: string; code: string; language: string }> = {
  introduction: {
    title: 'Quick Example',
    language: 'javascript',
    code: `import { ServiceLayerClient } from '@r3e/service-layer-sdk';

const client = new ServiceLayerClient({
  apiKey: 'your-api-key',
  endpoint: 'https://api.servicelayer.neo'
});

// Request oracle data
const result = await client.oracle.request({
  url: 'https://api.coingecko.com/api/v3/simple/price',
  params: { ids: 'neo', vs_currencies: 'usd' }
});

console.log('NEO Price:', result.data.neo.usd);`,
  },
  quickstart: {
    title: 'Installation',
    language: 'bash',
    code: `# Install the SDK
npm install @r3e/service-layer-sdk

# Or using yarn
yarn add @r3e/service-layer-sdk

# Or using pnpm
pnpm add @r3e/service-layer-sdk`,
  },
  oracle: {
    title: 'Oracle Request',
    language: 'javascript',
    code: `// Make an oracle request
const response = await client.oracle.request({
  url: 'https://api.example.com/data',
  method: 'GET',
  headers: {
    'Authorization': 'Bearer {secrets.API_KEY}'
  }
});

// The response is signed by TEE
console.log('Data:', response.data);
console.log('TEE Signature:', response.signature);
console.log('Attestation:', response.attestation);`,
  },
  vrf: {
    title: 'VRF Random Number',
    language: 'javascript',
    code: `// Request verifiable random number
const vrf = await client.vrf.requestRandomness({
  seed: 'unique-seed-value',
  numWords: 1
});

// Verify the proof on-chain
console.log('Random Value:', vrf.randomValue);
console.log('Proof:', vrf.proof);
console.log('Can verify on-chain:', vrf.verifiable);`,
  },
  secrets: {
    title: 'Secrets Management',
    language: 'javascript',
    code: `// Store a secret (encrypted in TEE)
await client.secrets.set('API_KEY', 'sk-xxx-secret-key');

// Use secret in oracle request (never exposed)
const result = await client.oracle.request({
  url: 'https://api.openai.com/v1/chat/completions',
  headers: {
    'Authorization': 'Bearer {secrets.API_KEY}'
  },
  body: { model: 'gpt-4', messages: [...] }
});`,
  },
};

export function DocsPage() {
  const [activeSection, setActiveSection] = useState('introduction');
  const [searchQuery, setSearchQuery] = useState('');
  const [copiedCode, setCopiedCode] = useState(false);

  const currentExample = codeExamples[activeSection] || codeExamples.introduction;

  const copyCode = () => {
    navigator.clipboard.writeText(currentExample.code);
    setCopiedCode(true);
    setTimeout(() => setCopiedCode(false), 2000);
  };

  return (
    <div className="min-h-screen bg-surface-50">
      {/* Header */}
      <header className="bg-white border-b border-surface-200 sticky top-0 z-40">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between h-16">
            <div className="flex items-center gap-4">
              <Link to="/" className="flex items-center gap-2">
                <div className="w-8 h-8 bg-gradient-to-br from-primary-500 to-accent-500 rounded-lg flex items-center justify-center">
                  <FiShield className="w-5 h-5 text-white" />
                </div>
                <span className="text-xl font-bold text-surface-900">Docs</span>
              </Link>
            </div>
            <div className="flex-1 max-w-md mx-8">
              <div className="relative">
                <FiSearch className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-surface-400" />
                <input
                  type="text"
                  placeholder="Search documentation..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="w-full pl-10 pr-4 py-2 bg-surface-100 border border-surface-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
                />
              </div>
            </div>
            <div className="flex items-center gap-4">
              <Link to="/playground" className="text-surface-600 hover:text-surface-900 text-sm font-medium">
                Playground
              </Link>
              <Link to="/dashboard" className="px-4 py-2 bg-primary-600 hover:bg-primary-500 text-white rounded-lg text-sm font-medium transition-colors">
                Dashboard
              </Link>
            </div>
          </div>
        </div>
      </header>

      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="flex gap-8">
          {/* Sidebar */}
          <aside className="w-64 flex-shrink-0">
            <nav className="sticky top-24 space-y-6">
              {docSections.map((section) => (
                <div key={section.id}>
                  <div className="flex items-center gap-2 text-sm font-semibold text-surface-900 mb-2">
                    <section.icon className="w-4 h-4" />
                    {section.title}
                  </div>
                  <ul className="space-y-1 ml-6">
                    {section.items.map((item) => (
                      <li key={item.slug}>
                        <button
                          onClick={() => setActiveSection(item.slug)}
                          className={`w-full text-left px-3 py-1.5 rounded-lg text-sm transition-colors ${
                            activeSection === item.slug
                              ? 'bg-primary-100 text-primary-700 font-medium'
                              : 'text-surface-600 hover:bg-surface-100 hover:text-surface-900'
                          }`}
                        >
                          {item.title}
                        </button>
                      </li>
                    ))}
                  </ul>
                </div>
              ))}
            </nav>
          </aside>

          {/* Main Content */}
          <main className="flex-1 min-w-0">
            <div className="bg-white rounded-xl border border-surface-200 p-8">
              {/* Breadcrumb */}
              <div className="flex items-center gap-2 text-sm text-surface-500 mb-6">
                <Link to="/docs" className="hover:text-surface-700">Docs</Link>
                <FiChevronRight className="w-4 h-4" />
                <span className="text-surface-900 font-medium capitalize">{activeSection.replace('-', ' ')}</span>
              </div>

              {/* Content */}
              <article className="prose prose-surface max-w-none">
                <h1 className="text-3xl font-bold text-surface-900 mb-4 capitalize">
                  {activeSection.replace('-', ' ')}
                </h1>

                {activeSection === 'introduction' && (
                  <>
                    <p className="text-lg text-surface-600 mb-6">
                      Neo Service Layer provides TEE-protected blockchain services including oracle, VRF, secrets management, and automation. All sensitive operations run inside Intel SGX enclaves.
                    </p>
                    <h2 className="text-xl font-semibold text-surface-900 mt-8 mb-4">Key Features</h2>
                    <ul className="space-y-2 text-surface-600">
                      <li className="flex items-start gap-2">
                        <FiCheck className="w-5 h-5 text-green-500 flex-shrink-0 mt-0.5" />
                        <span><strong>TEE Protection:</strong> All cryptographic operations happen inside Intel SGX enclaves</span>
                      </li>
                      <li className="flex items-start gap-2">
                        <FiCheck className="w-5 h-5 text-green-500 flex-shrink-0 mt-0.5" />
                        <span><strong>Secrets Management:</strong> Store API keys and credentials securely with hardware encryption</span>
                      </li>
                      <li className="flex items-start gap-2">
                        <FiCheck className="w-5 h-5 text-green-500 flex-shrink-0 mt-0.5" />
                        <span><strong>Verifiable Randomness:</strong> Generate provably fair random numbers with VRF</span>
                      </li>
                      <li className="flex items-start gap-2">
                        <FiCheck className="w-5 h-5 text-green-500 flex-shrink-0 mt-0.5" />
                        <span><strong>Oracle Services:</strong> Fetch external data with TEE attestation</span>
                      </li>
                    </ul>
                  </>
                )}

                {activeSection === 'quickstart' && (
                  <>
                    <p className="text-lg text-surface-600 mb-6">
                      Get started with Neo Service Layer in just a few minutes.
                    </p>
                    <h2 className="text-xl font-semibold text-surface-900 mt-8 mb-4">Step 1: Create an Account</h2>
                    <p className="text-surface-600 mb-4">
                      Sign up at <Link to="/register" className="text-primary-600 hover:underline">servicelayer.neo/register</Link> to create your account.
                    </p>
                    <h2 className="text-xl font-semibold text-surface-900 mt-8 mb-4">Step 2: Get API Keys</h2>
                    <p className="text-surface-600 mb-4">
                      Navigate to the API Keys section in your dashboard to generate your API credentials.
                    </p>
                    <h2 className="text-xl font-semibold text-surface-900 mt-8 mb-4">Step 3: Install SDK</h2>
                    <p className="text-surface-600 mb-4">
                      Install our SDK using your preferred package manager.
                    </p>
                  </>
                )}

                {activeSection === 'tee-trust-root' && (
                  <>
                    <p className="text-lg text-surface-600 mb-6">
                      The TEE Trust Root is the foundation of our security architecture, providing hardware-backed isolation for all sensitive operations.
                    </p>
                    <h2 className="text-xl font-semibold text-surface-900 mt-8 mb-4">Architecture Overview</h2>
                    <div className="bg-surface-50 rounded-lg p-6 mb-6">
                      <pre className="text-sm text-surface-700">
{`┌─────────────────────────────────────────┐
│           TEE Trust Root                │
│  ┌─────────────────────────────────┐   │
│  │  Intel SGX Enclave              │   │
│  │  • Master Key Derivation        │   │
│  │  • Sealing/Unsealing            │   │
│  │  • Remote Attestation           │   │
│  └─────────────────────────────────┘   │
├─────────────────────────────────────────┤
│           Platform / ServiceOS          │
│  • Capability-based Access Control      │
│  • Resource Limits & Quotas             │
│  • Secrets & Keys Management            │
├─────────────────────────────────────────┤
│           Services Layer                │
│  Oracle │ VRF │ Secrets │ DataFeeds │...│
└─────────────────────────────────────────┘`}
                      </pre>
                    </div>
                  </>
                )}

                {/* Code Example */}
                <div className="mt-8">
                  <div className="flex items-center justify-between mb-2">
                    <h3 className="text-lg font-semibold text-surface-900">{currentExample.title}</h3>
                    <button
                      onClick={copyCode}
                      className="flex items-center gap-1 px-3 py-1 text-sm text-surface-600 hover:text-surface-900 bg-surface-100 hover:bg-surface-200 rounded-lg transition-colors"
                    >
                      {copiedCode ? <FiCheck className="w-4 h-4" /> : <FiCopy className="w-4 h-4" />}
                      {copiedCode ? 'Copied!' : 'Copy'}
                    </button>
                  </div>
                  <div className="bg-surface-900 rounded-lg p-4 overflow-x-auto">
                    <pre className="text-sm text-surface-100">
                      <code>{currentExample.code}</code>
                    </pre>
                  </div>
                </div>

                {/* Next Steps */}
                <div className="mt-12 pt-8 border-t border-surface-200">
                  <h3 className="text-lg font-semibold text-surface-900 mb-4">Next Steps</h3>
                  <div className="grid sm:grid-cols-2 gap-4">
                    <Link to="/playground" className="flex items-center gap-3 p-4 bg-surface-50 hover:bg-surface-100 rounded-lg transition-colors group">
                      <div className="w-10 h-10 bg-primary-100 rounded-lg flex items-center justify-center">
                        <FiCode className="w-5 h-5 text-primary-600" />
                      </div>
                      <div>
                        <div className="font-medium text-surface-900 group-hover:text-primary-600">Try Playground</div>
                        <div className="text-sm text-surface-500">Test APIs interactively</div>
                      </div>
                      <FiExternalLink className="w-4 h-4 text-surface-400 ml-auto" />
                    </Link>
                    <Link to="/docs/api" className="flex items-center gap-3 p-4 bg-surface-50 hover:bg-surface-100 rounded-lg transition-colors group">
                      <div className="w-10 h-10 bg-primary-100 rounded-lg flex items-center justify-center">
                        <FiBook className="w-5 h-5 text-primary-600" />
                      </div>
                      <div>
                        <div className="font-medium text-surface-900 group-hover:text-primary-600">API Reference</div>
                        <div className="text-sm text-surface-500">Complete API documentation</div>
                      </div>
                      <FiExternalLink className="w-4 h-4 text-surface-400 ml-auto" />
                    </Link>
                  </div>
                </div>
              </article>
            </div>
          </main>
        </div>
      </div>
    </div>
  );
}
