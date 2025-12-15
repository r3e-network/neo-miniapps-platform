import { useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import {
  Shuffle,
  Clock,
  Shield,
  AlertCircle,
  CheckCircle,
  Loader2,
  Plus,
  Trash2,
  Eye,
  EyeOff,
  Info
} from 'lucide-react';
import { api } from '../api/client';

// Mix duration options (in milliseconds)
const MIX_OPTIONS = [
  { value: 30 * 60 * 1000, label: '30 Minutes', description: 'Quick mix for small amounts' },
  { value: 60 * 60 * 1000, label: '1 Hour', description: 'Standard privacy level' },
  { value: 24 * 60 * 60 * 1000, label: '24 Hours', description: 'Enhanced privacy' },
  { value: 7 * 24 * 60 * 60 * 1000, label: '7 Days', description: 'Maximum privacy' },
];

// Request status mapping
const STATUS_MAP: Record<number, { label: string; color: string }> = {
  0: { label: 'Pending', color: 'text-yellow-500' },
  1: { label: 'Claimed', color: 'text-blue-500' },
  2: { label: 'Completed', color: 'text-green-500' },
  3: { label: 'Refunded', color: 'text-gray-500' },
};

interface TargetAddress {
  address: string;
  amount: string;
}

interface MixRequest {
  request_id: string;
  amount: string;
  status: number;
  mix_option: number;
  created_at: string;
  deadline: string;
  can_refund: boolean;
}

export function NeoVault() {
  // Form state
  const [targets, setTargets] = useState<TargetAddress[]>([{ address: '', amount: '' }]);
  const [mixOption, setMixOption] = useState(MIX_OPTIONS[1].value);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [agreedToTerms, setAgreedToTerms] = useState(false);

  // Query for user's mix requests
  const { data: requests, isLoading: loadingRequests, refetch } = useQuery({
    queryKey: ['neovault-requests'],
    queryFn: () => api.getNeoVaultRequests(),
  });

  // Query for service info
  const { data: serviceInfo } = useQuery({
    queryKey: ['neovault-info'],
    queryFn: () => api.getNeoVaultInfo(),
  });

  // Create mix request mutation
  const createMutation = useMutation({
    mutationFn: (data: { targets: TargetAddress[]; mixOption: number }) =>
      api.createMixRequest(data.targets, data.mixOption),
    onSuccess: () => {
      refetch();
      setTargets([{ address: '', amount: '' }]);
      setAgreedToTerms(false);
    },
  });

  // Claim refund mutation
  const refundMutation = useMutation({
    mutationFn: (requestId: string) => api.claimNeoVaultRefund(requestId),
    onSuccess: () => refetch(),
  });

  // Calculate total amount
  const totalAmount = targets.reduce((sum, t) => {
    const amount = parseFloat(t.amount) || 0;
    return sum + amount;
  }, 0);

  // Add target address
  const addTarget = () => {
    if (targets.length < 5) {
      setTargets([...targets, { address: '', amount: '' }]);
    }
  };

  // Remove target address
  const removeTarget = (index: number) => {
    if (targets.length > 1) {
      setTargets(targets.filter((_, i) => i !== index));
    }
  };

  // Update target
  const updateTarget = (index: number, field: keyof TargetAddress, value: string) => {
    const newTargets = [...targets];
    newTargets[index][field] = value;
    setTargets(newTargets);
  };

  // Validate form
  const isValid = targets.every(t => t.address.length > 0 && parseFloat(t.amount) > 0) &&
                  agreedToTerms &&
                  totalAmount > 0;

  // Handle submit
  const handleSubmit = () => {
    if (!isValid) return;
    createMutation.mutate({ targets, mixOption });
  };

  // Format date
  const formatDate = (dateStr: string) => {
    return new Date(dateStr).toLocaleString();
  };

  // Format duration
  const formatDuration = (ms: number) => {
    const option = MIX_OPTIONS.find(o => o.value === ms);
    return option?.label || `${ms / 1000 / 60} minutes`;
  };

  return (
    <div className="max-w-6xl mx-auto">
      <div className="flex items-center gap-3 mb-8">
        <div className="p-3 bg-purple-500/20 rounded-xl">
          <Shuffle className="w-8 h-8 text-purple-500" />
        </div>
        <div>
          <h1 className="text-3xl font-bold text-white">Privacy NeoVault</h1>
          <p className="text-gray-400">Double-Blind HD Multi-sig Privacy Transactions</p>
        </div>
      </div>

      {/* Info Banner */}
      <div className="bg-purple-500/10 border border-purple-500/30 rounded-xl p-4 mb-8">
        <div className="flex items-start gap-3">
          <Shield className="w-5 h-5 text-purple-500 mt-0.5" />
          <div>
            <h3 className="text-purple-400 font-medium mb-1">How it works</h3>
            <p className="text-gray-400 text-sm">
              Your funds are split and sent to multiple HD-derived pool accounts that appear as ordinary users.
              After the mixing period, funds are sent to your target addresses from different pool accounts.
              No one can trace the connection between sender and receiver.
            </p>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
        {/* Create Mix Request */}
        <div className="lg:col-span-2">
          <div className="bg-gray-800 rounded-xl border border-gray-700 p-6">
            <h2 className="text-xl font-semibold text-white mb-6">Create Mix Request</h2>

            {/* Target Addresses */}
            <div className="mb-6">
              <div className="flex items-center justify-between mb-3">
                <label className="text-sm font-medium text-gray-300">Target Addresses</label>
                <button
                  onClick={addTarget}
                  disabled={targets.length >= 5}
                  className="flex items-center gap-1 text-sm text-purple-400 hover:text-purple-300 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  <Plus className="w-4 h-4" />
                  Add Address
                </button>
              </div>

              <div className="space-y-3">
                {targets.map((target, index) => (
                  <div key={index} className="flex gap-3">
                    <input
                      type="text"
                      placeholder="Neo N3 Address (e.g., NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq)"
                      value={target.address}
                      onChange={(e) => updateTarget(index, 'address', e.target.value)}
                      className="flex-1 bg-gray-700 border border-gray-600 rounded-lg px-4 py-3 text-white placeholder-gray-500 focus:outline-none focus:border-purple-500"
                    />
                    <input
                      type="number"
                      placeholder="Amount (GAS)"
                      value={target.amount}
                      onChange={(e) => updateTarget(index, 'amount', e.target.value)}
                      className="w-32 bg-gray-700 border border-gray-600 rounded-lg px-4 py-3 text-white placeholder-gray-500 focus:outline-none focus:border-purple-500"
                    />
                    {targets.length > 1 && (
                      <button
                        onClick={() => removeTarget(index)}
                        className="p-3 text-gray-400 hover:text-red-400 transition-colors"
                      >
                        <Trash2 className="w-5 h-5" />
                      </button>
                    )}
                  </div>
                ))}
              </div>

              <p className="text-xs text-gray-500 mt-2">
                Target addresses are encrypted with TEE public key. Only the TEE can decrypt them.
              </p>
            </div>

            {/* Mix Duration */}
            <div className="mb-6">
              <label className="text-sm font-medium text-gray-300 mb-3 block">Mix Duration</label>
              <div className="grid grid-cols-2 gap-3">
                {MIX_OPTIONS.map((option) => (
                  <button
                    key={option.value}
                    onClick={() => setMixOption(option.value)}
                    className={`p-4 rounded-lg border text-left transition-colors ${
                      mixOption === option.value
                        ? 'border-purple-500 bg-purple-500/10'
                        : 'border-gray-600 bg-gray-700 hover:border-gray-500'
                    }`}
                  >
                    <div className="flex items-center gap-2 mb-1">
                      <Clock className={`w-4 h-4 ${mixOption === option.value ? 'text-purple-400' : 'text-gray-400'}`} />
                      <span className={`font-medium ${mixOption === option.value ? 'text-purple-400' : 'text-white'}`}>
                        {option.label}
                      </span>
                    </div>
                    <p className="text-xs text-gray-500">{option.description}</p>
                  </button>
                ))}
              </div>
            </div>

            {/* Advanced Options */}
            <div className="mb-6">
              <button
                onClick={() => setShowAdvanced(!showAdvanced)}
                className="flex items-center gap-2 text-sm text-gray-400 hover:text-white"
              >
                {showAdvanced ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                {showAdvanced ? 'Hide' : 'Show'} Advanced Options
              </button>

              {showAdvanced && (
                <div className="mt-4 p-4 bg-gray-700/50 rounded-lg">
                  <div className="flex items-start gap-2 text-sm text-gray-400">
                    <Info className="w-4 h-4 mt-0.5" />
                    <div>
                      <p className="mb-2">
                        <strong className="text-gray-300">Safety Buffer:</strong> 7 days added to deadline for user protection
                      </p>
                      <p className="mb-2">
                        <strong className="text-gray-300">Bond Protection:</strong> Service bond covers your funds if mixing fails
                      </p>
                      <p>
                        <strong className="text-gray-300">Refund:</strong> Claim refund from bond after deadline if not completed
                      </p>
                    </div>
                  </div>
                </div>
              )}
            </div>

            {/* Summary */}
            <div className="bg-gray-700/50 rounded-lg p-4 mb-6">
              <div className="flex justify-between items-center mb-2">
                <span className="text-gray-400">Total Amount</span>
                <span className="text-xl font-bold text-white">{totalAmount.toFixed(8)} GAS</span>
              </div>
              <div className="flex justify-between items-center mb-2">
                <span className="text-gray-400">Mix Duration</span>
                <span className="text-white">{formatDuration(mixOption)}</span>
              </div>
              <div className="flex justify-between items-center mb-2">
                <span className="text-gray-400">Target Addresses</span>
                <span className="text-white">{targets.filter(t => t.address).length}</span>
              </div>
              <div className="flex justify-between items-center">
                <span className="text-gray-400">Service Fee</span>
                <span className="text-white">0.5 GAS</span>
              </div>
            </div>

            {/* Terms */}
            <div className="mb-6">
              <label className="flex items-start gap-3 cursor-pointer">
                <input
                  type="checkbox"
                  checked={agreedToTerms}
                  onChange={(e) => setAgreedToTerms(e.target.checked)}
                  className="mt-1 w-4 h-4 rounded border-gray-600 bg-gray-700 text-purple-500 focus:ring-purple-500"
                />
                <span className="text-sm text-gray-400">
                  I understand that this is a privacy service. I confirm that I am not using this service for
                  illegal purposes and that I am responsible for complying with all applicable laws.
                </span>
              </label>
            </div>

            {/* Submit Button */}
            <button
              onClick={handleSubmit}
              disabled={!isValid || createMutation.isPending}
              className="w-full py-4 bg-purple-600 hover:bg-purple-500 disabled:bg-gray-600 disabled:cursor-not-allowed text-white font-medium rounded-lg transition-colors flex items-center justify-center gap-2"
            >
              {createMutation.isPending ? (
                <>
                  <Loader2 className="w-5 h-5 animate-spin" />
                  Creating Request...
                </>
              ) : (
                <>
                  <Shuffle className="w-5 h-5" />
                  Create Mix Request
                </>
              )}
            </button>

            {createMutation.isError && (
              <div className="mt-4 p-3 bg-red-500/10 border border-red-500/30 rounded-lg flex items-center gap-2 text-red-400">
                <AlertCircle className="w-5 h-5" />
                {createMutation.error?.message || 'Failed to create mix request'}
              </div>
            )}

            {createMutation.isSuccess && (
              <div className="mt-4 p-3 bg-green-500/10 border border-green-500/30 rounded-lg flex items-center gap-2 text-green-400">
                <CheckCircle className="w-5 h-5" />
                Mix request created successfully!
              </div>
            )}
          </div>
        </div>

        {/* Service Info & Stats */}
        <div className="space-y-6">
          {/* Service Status */}
          <div className="bg-gray-800 rounded-xl border border-gray-700 p-6">
            <h3 className="text-lg font-semibold text-white mb-4">Service Status</h3>
            <div className="space-y-3">
              <div className="flex justify-between">
                <span className="text-gray-400">Status</span>
                <span className="text-green-400 flex items-center gap-1">
                  <span className="w-2 h-2 bg-green-400 rounded-full"></span>
                  Active
                </span>
              </div>
              <div className="flex justify-between">
                <span className="text-gray-400">Bond Amount</span>
                <span className="text-white">{serviceInfo?.bond_amount || '100'} GAS</span>
              </div>
              <div className="flex justify-between">
                <span className="text-gray-400">Available Capacity</span>
                <span className="text-white">{serviceInfo?.available_capacity || '90'} GAS</span>
              </div>
              <div className="flex justify-between">
                <span className="text-gray-400">Total Mixed</span>
                <span className="text-white">{serviceInfo?.total_mixed || '1,234'} GAS</span>
              </div>
            </div>
          </div>

          {/* Privacy Features */}
          <div className="bg-gray-800 rounded-xl border border-gray-700 p-6">
            <h3 className="text-lg font-semibold text-white mb-4">Privacy Features</h3>
            <ul className="space-y-3 text-sm">
              <li className="flex items-start gap-2">
                <CheckCircle className="w-4 h-4 text-green-400 mt-0.5" />
                <span className="text-gray-300">HD-derived pool accounts (no on-chain registration)</span>
              </li>
              <li className="flex items-start gap-2">
                <CheckCircle className="w-4 h-4 text-green-400 mt-0.5" />
                <span className="text-gray-300">1-of-2 multisig for TEE + Master recovery</span>
              </li>
              <li className="flex items-start gap-2">
                <CheckCircle className="w-4 h-4 text-green-400 mt-0.5" />
                <span className="text-gray-300">Random split amounts prevent correlation</span>
              </li>
              <li className="flex items-start gap-2">
                <CheckCircle className="w-4 h-4 text-green-400 mt-0.5" />
                <span className="text-gray-300">Continuous noise transactions</span>
              </li>
              <li className="flex items-start gap-2">
                <CheckCircle className="w-4 h-4 text-green-400 mt-0.5" />
                <span className="text-gray-300">ECIES encrypted target addresses</span>
              </li>
            </ul>
          </div>
        </div>
      </div>

      {/* My Requests */}
      <div className="mt-8">
        <h2 className="text-xl font-semibold text-white mb-4">My Mix Requests</h2>

        {loadingRequests ? (
          <div className="bg-gray-800 rounded-xl border border-gray-700 p-8 flex items-center justify-center">
            <Loader2 className="w-6 h-6 text-gray-400 animate-spin" />
          </div>
        ) : requests && requests.length > 0 ? (
          <div className="bg-gray-800 rounded-xl border border-gray-700 overflow-hidden">
            <table className="w-full">
              <thead>
                <tr className="border-b border-gray-700">
                  <th className="text-left text-gray-400 font-medium px-6 py-4">Request ID</th>
                  <th className="text-left text-gray-400 font-medium px-6 py-4">Amount</th>
                  <th className="text-left text-gray-400 font-medium px-6 py-4">Duration</th>
                  <th className="text-left text-gray-400 font-medium px-6 py-4">Status</th>
                  <th className="text-left text-gray-400 font-medium px-6 py-4">Created</th>
                  <th className="text-left text-gray-400 font-medium px-6 py-4">Deadline</th>
                  <th className="text-left text-gray-400 font-medium px-6 py-4">Action</th>
                </tr>
              </thead>
              <tbody>
                {requests.map((request: MixRequest) => (
                  <tr key={request.request_id} className="border-b border-gray-700/50 hover:bg-gray-700/30">
                    <td className="px-6 py-4">
                      <span className="text-white font-mono text-sm">#{request.request_id}</span>
                    </td>
                    <td className="px-6 py-4">
                      <span className="text-white">{request.amount} GAS</span>
                    </td>
                    <td className="px-6 py-4">
                      <span className="text-gray-300">{formatDuration(request.mix_option)}</span>
                    </td>
                    <td className="px-6 py-4">
                      <span className={`${STATUS_MAP[request.status]?.color || 'text-gray-400'}`}>
                        {STATUS_MAP[request.status]?.label || 'Unknown'}
                      </span>
                    </td>
                    <td className="px-6 py-4">
                      <span className="text-gray-400 text-sm">{formatDate(request.created_at)}</span>
                    </td>
                    <td className="px-6 py-4">
                      <span className="text-gray-400 text-sm">{formatDate(request.deadline)}</span>
                    </td>
                    <td className="px-6 py-4">
                      {request.can_refund && request.status !== 2 && request.status !== 3 && (
                        <button
                          onClick={() => refundMutation.mutate(request.request_id)}
                          disabled={refundMutation.isPending}
                          className="px-3 py-1 bg-red-500/20 text-red-400 hover:bg-red-500/30 rounded text-sm font-medium transition-colors disabled:opacity-50"
                        >
                          {refundMutation.isPending ? 'Claiming...' : 'Claim Refund'}
                        </button>
                      )}
                      {request.status === 2 && (
                        <span className="text-green-400 text-sm flex items-center gap-1">
                          <CheckCircle className="w-4 h-4" />
                          Completed
                        </span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <div className="bg-gray-800 rounded-xl border border-gray-700 p-8 text-center">
            <Shuffle className="w-12 h-12 text-gray-600 mx-auto mb-3" />
            <p className="text-gray-400">No mix requests yet</p>
            <p className="text-gray-500 text-sm">Create your first privacy transaction above</p>
          </div>
        )}
      </div>

      {/* How It Works */}
      <div className="mt-8 bg-gray-800 rounded-xl border border-gray-700 p-6">
        <h2 className="text-xl font-semibold text-white mb-6">How Privacy Mixing Works</h2>
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          <div className="text-center">
            <div className="w-12 h-12 bg-purple-500/20 rounded-full flex items-center justify-center mx-auto mb-3">
              <span className="text-purple-400 font-bold">1</span>
            </div>
            <h4 className="text-white font-medium mb-1">Create Request</h4>
            <p className="text-gray-400 text-sm">Enter target addresses and amount. Addresses are encrypted.</p>
          </div>
          <div className="text-center">
            <div className="w-12 h-12 bg-purple-500/20 rounded-full flex items-center justify-center mx-auto mb-3">
              <span className="text-purple-400 font-bold">2</span>
            </div>
            <h4 className="text-white font-medium mb-1">Funds Split</h4>
            <p className="text-gray-400 text-sm">TEE splits your funds to random HD pool accounts.</p>
          </div>
          <div className="text-center">
            <div className="w-12 h-12 bg-purple-500/20 rounded-full flex items-center justify-center mx-auto mb-3">
              <span className="text-purple-400 font-bold">3</span>
            </div>
            <h4 className="text-white font-medium mb-1">Mixing Period</h4>
            <p className="text-gray-400 text-sm">Funds transfer randomly within pool with noise transactions.</p>
          </div>
          <div className="text-center">
            <div className="w-12 h-12 bg-purple-500/20 rounded-full flex items-center justify-center mx-auto mb-3">
              <span className="text-purple-400 font-bold">4</span>
            </div>
            <h4 className="text-white font-medium mb-1">Delivery</h4>
            <p className="text-gray-400 text-sm">Funds sent to targets from different pool accounts.</p>
          </div>
        </div>
      </div>
    </div>
  );
}
