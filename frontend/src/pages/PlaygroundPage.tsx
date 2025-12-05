import React, { useState } from 'react';
import { Link } from 'react-router-dom';
import {
  FiPlay,
  FiShield,
  FiCode,
  FiCopy,
  FiCheck,
  FiChevronDown,
  FiLoader,
  FiAlertCircle,
  FiCheckCircle,
} from 'react-icons/fi';

type ServiceType = 'oracle' | 'vrf' | 'secrets' | 'datafeeds' | 'automation';

interface RequestTemplate {
  name: string;
  service: ServiceType;
  method: string;
  endpoint: string;
  body: string;
  description: string;
}

const templates: RequestTemplate[] = [
  {
    name: 'Oracle - Fetch Price',
    service: 'oracle',
    method: 'POST',
    endpoint: '/api/v1/oracle/request',
    description: 'Fetch cryptocurrency price from CoinGecko',
    body: JSON.stringify({
      url: 'https://api.coingecko.com/api/v3/simple/price',
      method: 'GET',
      params: {
        ids: 'neo',
        vs_currencies: 'usd'
      }
    }, null, 2),
  },
  {
    name: 'VRF - Random Number',
    service: 'vrf',
    method: 'POST',
    endpoint: '/api/v1/vrf/request',
    description: 'Generate verifiable random number',
    body: JSON.stringify({
      seed: 'unique-seed-' + Date.now(),
      numWords: 1,
      callback: {
        contract: 'NXxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx',
        method: 'onRandomReceived'
      }
    }, null, 2),
  },
  {
    name: 'Secrets - Store Secret',
    service: 'secrets',
    method: 'POST',
    endpoint: '/api/v1/secrets',
    description: 'Store an encrypted secret in TEE',
    body: JSON.stringify({
      name: 'MY_API_KEY',
      value: 'sk-xxxxxxxxxxxxx',
      description: 'OpenAI API Key'
    }, null, 2),
  },
  {
    name: 'Secrets - List Secrets',
    service: 'secrets',
    method: 'GET',
    endpoint: '/api/v1/secrets',
    description: 'List all stored secrets (values hidden)',
    body: '',
  },
  {
    name: 'DataFeeds - Get Price',
    service: 'datafeeds',
    method: 'GET',
    endpoint: '/api/v1/datafeeds/NEO-USD/latest',
    description: 'Get latest NEO/USD price feed',
    body: '',
  },
  {
    name: 'Automation - Create Job',
    service: 'automation',
    method: 'POST',
    endpoint: '/api/v1/automation/jobs',
    description: 'Create a scheduled automation job',
    body: JSON.stringify({
      name: 'Daily Price Update',
      schedule: '0 0 * * *',
      action: {
        type: 'oracle_request',
        config: {
          url: 'https://api.coingecko.com/api/v3/simple/price',
          params: { ids: 'neo', vs_currencies: 'usd' }
        }
      }
    }, null, 2),
  },
];

export function PlaygroundPage() {
  const [selectedTemplate, setSelectedTemplate] = useState<RequestTemplate>(templates[0]);
  const [method, setMethod] = useState(templates[0].method);
  const [endpoint, setEndpoint] = useState(templates[0].endpoint);
  const [requestBody, setRequestBody] = useState(templates[0].body);
  const [response, setResponse] = useState<string>('');
  const [isLoading, setIsLoading] = useState(false);
  const [responseStatus, setResponseStatus] = useState<'success' | 'error' | null>(null);
  const [copiedRequest, setCopiedRequest] = useState(false);
  const [copiedResponse, setCopiedResponse] = useState(false);

  const handleTemplateChange = (template: RequestTemplate) => {
    setSelectedTemplate(template);
    setMethod(template.method);
    setEndpoint(template.endpoint);
    setRequestBody(template.body);
    setResponse('');
    setResponseStatus(null);
  };

  const executeRequest = async () => {
    setIsLoading(true);
    setResponse('');
    setResponseStatus(null);

    try {
      // Simulate API call
      await new Promise(resolve => setTimeout(resolve, 1000 + Math.random() * 1000));

      // Mock responses based on service type
      let mockResponse: object;

      switch (selectedTemplate.service) {
        case 'oracle':
          mockResponse = {
            success: true,
            data: {
              neo: { usd: 12.45 }
            },
            metadata: {
              requestId: 'req_' + Math.random().toString(36).substr(2, 9),
              timestamp: new Date().toISOString(),
              teeAttestation: '0x' + Array(64).fill(0).map(() => Math.floor(Math.random() * 16).toString(16)).join(''),
              signature: '0x' + Array(128).fill(0).map(() => Math.floor(Math.random() * 16).toString(16)).join('')
            }
          };
          break;
        case 'vrf':
          mockResponse = {
            success: true,
            data: {
              requestId: 'vrf_' + Math.random().toString(36).substr(2, 9),
              randomValue: '0x' + Array(64).fill(0).map(() => Math.floor(Math.random() * 16).toString(16)).join(''),
              proof: {
                gamma: '0x' + Array(64).fill(0).map(() => Math.floor(Math.random() * 16).toString(16)).join(''),
                c: '0x' + Array(32).fill(0).map(() => Math.floor(Math.random() * 16).toString(16)).join(''),
                s: '0x' + Array(32).fill(0).map(() => Math.floor(Math.random() * 16).toString(16)).join('')
              },
              verifiable: true
            }
          };
          break;
        case 'secrets':
          if (method === 'GET') {
            mockResponse = {
              success: true,
              data: [
                { name: 'MY_API_KEY', createdAt: '2024-01-15T10:30:00Z', description: 'OpenAI API Key' },
                { name: 'DB_PASSWORD', createdAt: '2024-01-10T08:00:00Z', description: 'Database password' }
              ]
            };
          } else {
            mockResponse = {
              success: true,
              data: {
                name: 'MY_API_KEY',
                createdAt: new Date().toISOString(),
                encrypted: true,
                teeSealed: true
              }
            };
          }
          break;
        case 'datafeeds':
          mockResponse = {
            success: true,
            data: {
              pair: 'NEO-USD',
              price: '12.45',
              timestamp: new Date().toISOString(),
              sources: ['binance', 'huobi', 'gate'],
              confidence: 0.99,
              signature: '0x' + Array(128).fill(0).map(() => Math.floor(Math.random() * 16).toString(16)).join('')
            }
          };
          break;
        case 'automation':
          mockResponse = {
            success: true,
            data: {
              jobId: 'job_' + Math.random().toString(36).substr(2, 9),
              name: 'Daily Price Update',
              schedule: '0 0 * * *',
              nextRun: new Date(Date.now() + 86400000).toISOString(),
              status: 'active'
            }
          };
          break;
        default:
          mockResponse = { success: true, message: 'Request completed' };
      }

      setResponse(JSON.stringify(mockResponse, null, 2));
      setResponseStatus('success');
    } catch (error) {
      setResponse(JSON.stringify({
        success: false,
        error: {
          code: 'REQUEST_FAILED',
          message: 'Failed to execute request'
        }
      }, null, 2));
      setResponseStatus('error');
    } finally {
      setIsLoading(false);
    }
  };

  const copyToClipboard = (text: string, type: 'request' | 'response') => {
    navigator.clipboard.writeText(text);
    if (type === 'request') {
      setCopiedRequest(true);
      setTimeout(() => setCopiedRequest(false), 2000);
    } else {
      setCopiedResponse(true);
      setTimeout(() => setCopiedResponse(false), 2000);
    }
  };

  const generateCurlCommand = () => {
    let curl = `curl -X ${method} '${window.location.origin}${endpoint}'`;
    curl += ` \\\n  -H 'Content-Type: application/json'`;
    curl += ` \\\n  -H 'Authorization: Bearer YOUR_API_KEY'`;
    if (requestBody && method !== 'GET') {
      curl += ` \\\n  -d '${requestBody.replace(/\n/g, '').replace(/\s+/g, ' ')}'`;
    }
    return curl;
  };

  return (
    <div className="min-h-screen bg-surface-900">
      {/* Header */}
      <header className="bg-surface-800 border-b border-surface-700 sticky top-0 z-40">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between h-16">
            <div className="flex items-center gap-4">
              <Link to="/" className="flex items-center gap-2">
                <div className="w-8 h-8 bg-gradient-to-br from-primary-500 to-accent-500 rounded-lg flex items-center justify-center">
                  <FiShield className="w-5 h-5 text-white" />
                </div>
                <span className="text-xl font-bold text-white">Playground</span>
              </Link>
            </div>
            <div className="flex items-center gap-4">
              <Link to="/docs" className="text-surface-300 hover:text-white text-sm font-medium transition-colors">
                Documentation
              </Link>
              <Link to="/dashboard" className="px-4 py-2 bg-primary-600 hover:bg-primary-500 text-white rounded-lg text-sm font-medium transition-colors">
                Dashboard
              </Link>
            </div>
          </div>
        </div>
      </header>

      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="grid lg:grid-cols-3 gap-6">
          {/* Templates Sidebar */}
          <div className="lg:col-span-1">
            <div className="bg-surface-800 rounded-xl border border-surface-700 p-4">
              <h2 className="text-lg font-semibold text-white mb-4">Request Templates</h2>
              <div className="space-y-2">
                {templates.map((template) => (
                  <button
                    key={template.name}
                    onClick={() => handleTemplateChange(template)}
                    className={`w-full text-left p-3 rounded-lg transition-colors ${
                      selectedTemplate.name === template.name
                        ? 'bg-primary-500/20 border border-primary-500/50'
                        : 'bg-surface-700/50 hover:bg-surface-700 border border-transparent'
                    }`}
                  >
                    <div className="flex items-center justify-between mb-1">
                      <span className="text-sm font-medium text-white">{template.name}</span>
                      <span className={`text-xs px-2 py-0.5 rounded ${
                        template.method === 'GET' ? 'bg-green-500/20 text-green-400' : 'bg-blue-500/20 text-blue-400'
                      }`}>
                        {template.method}
                      </span>
                    </div>
                    <p className="text-xs text-surface-400">{template.description}</p>
                  </button>
                ))}
              </div>
            </div>
          </div>

          {/* Request/Response Panel */}
          <div className="lg:col-span-2 space-y-6">
            {/* Request Builder */}
            <div className="bg-surface-800 rounded-xl border border-surface-700 overflow-hidden">
              <div className="p-4 border-b border-surface-700">
                <h2 className="text-lg font-semibold text-white mb-4">Request</h2>
                <div className="flex gap-2">
                  <select
                    value={method}
                    onChange={(e) => setMethod(e.target.value)}
                    className="px-3 py-2 bg-surface-700 border border-surface-600 rounded-lg text-white text-sm focus:outline-none focus:ring-2 focus:ring-primary-500"
                  >
                    <option value="GET">GET</option>
                    <option value="POST">POST</option>
                    <option value="PUT">PUT</option>
                    <option value="DELETE">DELETE</option>
                  </select>
                  <input
                    type="text"
                    value={endpoint}
                    onChange={(e) => setEndpoint(e.target.value)}
                    className="flex-1 px-3 py-2 bg-surface-700 border border-surface-600 rounded-lg text-white text-sm focus:outline-none focus:ring-2 focus:ring-primary-500"
                    placeholder="/api/v1/..."
                  />
                  <button
                    onClick={executeRequest}
                    disabled={isLoading}
                    className="px-6 py-2 bg-primary-600 hover:bg-primary-500 disabled:bg-primary-600/50 text-white rounded-lg text-sm font-medium transition-colors flex items-center gap-2"
                  >
                    {isLoading ? (
                      <FiLoader className="w-4 h-4 animate-spin" />
                    ) : (
                      <FiPlay className="w-4 h-4" />
                    )}
                    Send
                  </button>
                </div>
              </div>

              {/* Request Body */}
              {method !== 'GET' && (
                <div className="p-4 border-b border-surface-700">
                  <div className="flex items-center justify-between mb-2">
                    <span className="text-sm font-medium text-surface-300">Body</span>
                    <button
                      onClick={() => copyToClipboard(requestBody, 'request')}
                      className="text-surface-400 hover:text-white text-sm flex items-center gap-1"
                    >
                      {copiedRequest ? <FiCheck className="w-4 h-4" /> : <FiCopy className="w-4 h-4" />}
                      {copiedRequest ? 'Copied!' : 'Copy'}
                    </button>
                  </div>
                  <textarea
                    value={requestBody}
                    onChange={(e) => setRequestBody(e.target.value)}
                    rows={10}
                    className="w-full px-3 py-2 bg-surface-900 border border-surface-600 rounded-lg text-surface-100 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-primary-500 resize-none"
                    placeholder="Request body (JSON)"
                  />
                </div>
              )}

              {/* cURL Command */}
              <div className="p-4 bg-surface-900/50">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm font-medium text-surface-300">cURL</span>
                  <button
                    onClick={() => copyToClipboard(generateCurlCommand(), 'request')}
                    className="text-surface-400 hover:text-white text-sm flex items-center gap-1"
                  >
                    <FiCopy className="w-4 h-4" />
                    Copy
                  </button>
                </div>
                <pre className="text-xs text-surface-400 font-mono overflow-x-auto">
                  {generateCurlCommand()}
                </pre>
              </div>
            </div>

            {/* Response */}
            <div className="bg-surface-800 rounded-xl border border-surface-700 overflow-hidden">
              <div className="p-4 border-b border-surface-700 flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <h2 className="text-lg font-semibold text-white">Response</h2>
                  {responseStatus && (
                    <span className={`flex items-center gap-1 text-sm ${
                      responseStatus === 'success' ? 'text-green-400' : 'text-red-400'
                    }`}>
                      {responseStatus === 'success' ? (
                        <FiCheckCircle className="w-4 h-4" />
                      ) : (
                        <FiAlertCircle className="w-4 h-4" />
                      )}
                      {responseStatus === 'success' ? '200 OK' : 'Error'}
                    </span>
                  )}
                </div>
                {response && (
                  <button
                    onClick={() => copyToClipboard(response, 'response')}
                    className="text-surface-400 hover:text-white text-sm flex items-center gap-1"
                  >
                    {copiedResponse ? <FiCheck className="w-4 h-4" /> : <FiCopy className="w-4 h-4" />}
                    {copiedResponse ? 'Copied!' : 'Copy'}
                  </button>
                )}
              </div>
              <div className="p-4 bg-surface-900/50 min-h-[300px]">
                {isLoading ? (
                  <div className="flex items-center justify-center h-64">
                    <FiLoader className="w-8 h-8 text-primary-500 animate-spin" />
                  </div>
                ) : response ? (
                  <pre className={`text-sm font-mono overflow-x-auto ${
                    responseStatus === 'success' ? 'text-green-400' : 'text-red-400'
                  }`}>
                    {response}
                  </pre>
                ) : (
                  <div className="flex flex-col items-center justify-center h-64 text-surface-500">
                    <FiCode className="w-12 h-12 mb-4 opacity-50" />
                    <p>Send a request to see the response</p>
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
