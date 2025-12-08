import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Wallet, ArrowUpRight, ArrowDownLeft, Clock, CheckCircle, XCircle, AlertCircle, Copy, Check, ExternalLink } from 'lucide-react';
import { api, DepositRequest, GasBankTransaction } from '../api/client';
import { useAuthStore } from '../stores/auth';

// Service Layer deposit address (would be configured per environment)
const DEPOSIT_ADDRESS = 'NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq';

export function GasBank() {
  const { user } = useAuthStore();
  const queryClient = useQueryClient();
  const [showDeposit, setShowDeposit] = useState(false);
  const [depositAmount, setDepositAmount] = useState('');
  const [depositTxHash, setDepositTxHash] = useState('');
  const [copiedAddress, setCopiedAddress] = useState(false);
  const [activeTab, setActiveTab] = useState<'deposits' | 'transactions'>('deposits');

  // Fetch balance
  const { data: account, isLoading: accountLoading } = useQuery({
    queryKey: ['gasbank-account'],
    queryFn: () => api.getGasBankAccount(),
  });

  // Fetch deposits
  const { data: deposits, isLoading: depositsLoading } = useQuery({
    queryKey: ['gasbank-deposits'],
    queryFn: () => api.listDeposits(),
  });

  // Fetch transactions
  const { data: transactions, isLoading: transactionsLoading } = useQuery({
    queryKey: ['gasbank-transactions'],
    queryFn: () => api.listTransactions(),
  });

  // Create deposit mutation
  const createDepositMutation = useMutation({
    mutationFn: () => api.createDeposit(
      parseFloat(depositAmount) * 1e8, // Convert to smallest unit
      user?.address || '',
      depositTxHash
    ),
    onSuccess: () => {
      setShowDeposit(false);
      setDepositAmount('');
      setDepositTxHash('');
      queryClient.invalidateQueries({ queryKey: ['gasbank-deposits'] });
      queryClient.invalidateQueries({ queryKey: ['gasbank-account'] });
    },
  });

  const formatGas = (amount: number) => (amount / 1e8).toFixed(4);

  const handleCopyAddress = async () => {
    await navigator.clipboard.writeText(DEPOSIT_ADDRESS);
    setCopiedAddress(true);
    setTimeout(() => setCopiedAddress(false), 2000);
  };

  const handleSubmitDeposit = () => {
    if (depositAmount && parseFloat(depositAmount) > 0) {
      createDepositMutation.mutate();
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'confirmed':
      case 'completed':
        return <CheckCircle className="w-4 h-4 text-green-500" />;
      case 'pending':
      case 'confirming':
        return <Clock className="w-4 h-4 text-yellow-500" />;
      case 'failed':
      case 'expired':
        return <XCircle className="w-4 h-4 text-red-500" />;
      default:
        return <Clock className="w-4 h-4 text-gray-500" />;
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'confirmed':
      case 'completed':
        return 'text-green-400 bg-green-500/10';
      case 'pending':
      case 'confirming':
        return 'text-yellow-400 bg-yellow-500/10';
      case 'failed':
      case 'expired':
        return 'text-red-400 bg-red-500/10';
      default:
        return 'text-gray-400 bg-gray-500/10';
    }
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString('en-US', {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  return (
    <div>
      <h1 className="text-3xl font-bold text-white mb-8">Gas Bank</h1>

      {/* Balance Card */}
      <div className="bg-gradient-to-r from-green-600 to-green-700 rounded-xl p-8 mb-8">
        <div className="flex items-center gap-4 mb-4">
          <Wallet className="w-10 h-10 text-white" />
          <div>
            <p className="text-green-100">Available Balance</p>
            <p className="text-4xl font-bold text-white">
              {accountLoading ? '...' : account ? formatGas(account.balance - account.reserved) : '0.0000'} GAS
            </p>
          </div>
        </div>
        <div className="flex gap-8 mt-6">
          <div>
            <p className="text-green-100 text-sm">Total Balance</p>
            <p className="text-xl font-semibold text-white">
              {accountLoading ? '...' : account ? formatGas(account.balance) : '0.0000'} GAS
            </p>
          </div>
          <div>
            <p className="text-green-100 text-sm">Reserved</p>
            <p className="text-xl font-semibold text-white">
              {accountLoading ? '...' : account ? formatGas(account.reserved) : '0.0000'} GAS
            </p>
          </div>
        </div>
      </div>

      {/* Actions */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
        <button
          onClick={() => setShowDeposit(true)}
          className="flex items-center justify-center gap-3 bg-gray-800 hover:bg-gray-700 border border-gray-700 rounded-xl p-6 transition-colors"
        >
          <ArrowDownLeft className="w-6 h-6 text-green-500" />
          <span className="text-lg font-medium text-white">Deposit</span>
        </button>
        <button
          disabled
          className="flex items-center justify-center gap-3 bg-gray-800 border border-gray-700 rounded-xl p-6 opacity-50 cursor-not-allowed"
        >
          <ArrowUpRight className="w-6 h-6 text-red-500" />
          <span className="text-lg font-medium text-white">Withdraw (Coming Soon)</span>
        </button>
      </div>

      {/* Deposit Modal */}
      {showDeposit && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <div className="bg-gray-800 rounded-xl p-6 max-w-md w-full border border-gray-700">
            <h2 className="text-xl font-semibold text-white mb-4">Deposit GAS</h2>

            <div className="space-y-4">
              {/* Step 1: Send GAS */}
              <div className="bg-gray-700/50 rounded-lg p-4">
                <p className="text-sm text-gray-400 mb-2">Step 1: Send GAS to this address</p>
                <div className="flex items-center gap-2 bg-gray-900 rounded p-2">
                  <code className="text-green-400 text-sm flex-1 break-all">{DEPOSIT_ADDRESS}</code>
                  <button
                    onClick={handleCopyAddress}
                    className="text-gray-400 hover:text-white p-1"
                  >
                    {copiedAddress ? (
                      <Check className="w-4 h-4 text-green-500" />
                    ) : (
                      <Copy className="w-4 h-4" />
                    )}
                  </button>
                </div>
              </div>

              {/* Step 2: Enter amount */}
              <div>
                <label className="block text-sm text-gray-400 mb-1">Step 2: Enter deposit amount</label>
                <div className="relative">
                  <input
                    type="number"
                    value={depositAmount}
                    onChange={(e) => setDepositAmount(e.target.value)}
                    placeholder="0.0000"
                    step="0.0001"
                    min="0"
                    className="w-full bg-gray-700 border border-gray-600 rounded-lg px-3 py-2 text-white pr-16"
                  />
                  <span className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400">GAS</span>
                </div>
              </div>

              {/* Step 3: Enter TX hash (optional) */}
              <div>
                <label className="block text-sm text-gray-400 mb-1">
                  Step 3: Enter transaction hash (optional)
                </label>
                <input
                  type="text"
                  value={depositTxHash}
                  onChange={(e) => setDepositTxHash(e.target.value)}
                  placeholder="0x..."
                  className="w-full bg-gray-700 border border-gray-600 rounded-lg px-3 py-2 text-white font-mono text-sm"
                />
                <p className="text-xs text-gray-500 mt-1">
                  Providing the TX hash speeds up confirmation
                </p>
              </div>

              {/* Info */}
              <div className="bg-blue-500/10 border border-blue-500/30 rounded-lg p-3 flex items-start gap-2">
                <AlertCircle className="w-4 h-4 text-blue-400 flex-shrink-0 mt-0.5" />
                <p className="text-blue-400 text-sm">
                  Deposits require 1 block confirmation. Your balance will be updated automatically.
                </p>
              </div>

              {/* Actions */}
              <div className="flex gap-2 pt-2">
                <button
                  onClick={handleSubmitDeposit}
                  disabled={!depositAmount || parseFloat(depositAmount) <= 0 || createDepositMutation.isPending}
                  className="flex-1 bg-green-600 hover:bg-green-700 disabled:bg-gray-600 text-white py-2 rounded-lg"
                >
                  {createDepositMutation.isPending ? 'Submitting...' : 'Submit Deposit'}
                </button>
                <button
                  onClick={() => setShowDeposit(false)}
                  className="bg-gray-600 hover:bg-gray-500 text-white px-4 py-2 rounded-lg"
                >
                  Cancel
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Tabs */}
      <div className="flex gap-4 mb-4">
        <button
          onClick={() => setActiveTab('deposits')}
          className={`px-4 py-2 rounded-lg font-medium ${
            activeTab === 'deposits'
              ? 'bg-green-600 text-white'
              : 'bg-gray-800 text-gray-400 hover:text-white'
          }`}
        >
          Deposits
        </button>
        <button
          onClick={() => setActiveTab('transactions')}
          className={`px-4 py-2 rounded-lg font-medium ${
            activeTab === 'transactions'
              ? 'bg-green-600 text-white'
              : 'bg-gray-800 text-gray-400 hover:text-white'
          }`}
        >
          Transactions
        </button>
      </div>

      {/* Deposits List */}
      {activeTab === 'deposits' && (
        <div className="bg-gray-800 rounded-xl border border-gray-700">
          <div className="px-6 py-4 border-b border-gray-700">
            <h2 className="text-lg font-semibold text-white">Deposit History</h2>
          </div>
          {depositsLoading ? (
            <div className="p-8 text-center text-gray-400">Loading...</div>
          ) : deposits && deposits.length > 0 ? (
            <div className="divide-y divide-gray-700">
              {deposits.map((deposit: DepositRequest) => (
                <div key={deposit.id} className="px-6 py-4 flex items-center justify-between">
                  <div className="flex items-center gap-4">
                    <div className="w-10 h-10 rounded-full bg-green-500/10 flex items-center justify-center">
                      <ArrowDownLeft className="w-5 h-5 text-green-500" />
                    </div>
                    <div>
                      <div className="flex items-center gap-2">
                        <span className="text-white font-medium">
                          +{formatGas(deposit.amount)} GAS
                        </span>
                        <span className={`text-xs px-2 py-0.5 rounded ${getStatusColor(deposit.status)}`}>
                          {deposit.status}
                        </span>
                      </div>
                      <div className="text-gray-500 text-sm">
                        {formatDate(deposit.created_at)}
                        {deposit.tx_hash && (
                          <a
                            href={`https://neotube.io/transaction/${deposit.tx_hash}`}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="ml-2 text-green-400 hover:text-green-300 inline-flex items-center gap-1"
                          >
                            View TX <ExternalLink className="w-3 h-3" />
                          </a>
                        )}
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    {getStatusIcon(deposit.status)}
                    {deposit.status === 'confirming' && (
                      <span className="text-gray-400 text-sm">
                        {deposit.confirmations}/{deposit.required_confirmations}
                      </span>
                    )}
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="p-8 text-center text-gray-400">
              No deposits yet. Make your first deposit to get started.
            </div>
          )}
        </div>
      )}

      {/* Transactions List */}
      {activeTab === 'transactions' && (
        <div className="bg-gray-800 rounded-xl border border-gray-700">
          <div className="px-6 py-4 border-b border-gray-700">
            <h2 className="text-lg font-semibold text-white">Transaction History</h2>
          </div>
          {transactionsLoading ? (
            <div className="p-8 text-center text-gray-400">Loading...</div>
          ) : transactions && transactions.length > 0 ? (
            <div className="divide-y divide-gray-700">
              {transactions.map((tx: GasBankTransaction) => (
                <div key={tx.id} className="px-6 py-4 flex items-center justify-between">
                  <div className="flex items-center gap-4">
                    <div className={`w-10 h-10 rounded-full flex items-center justify-center ${
                      tx.amount > 0 ? 'bg-green-500/10' : 'bg-red-500/10'
                    }`}>
                      {tx.amount > 0 ? (
                        <ArrowDownLeft className="w-5 h-5 text-green-500" />
                      ) : (
                        <ArrowUpRight className="w-5 h-5 text-red-500" />
                      )}
                    </div>
                    <div>
                      <div className="flex items-center gap-2">
                        <span className={`font-medium ${tx.amount > 0 ? 'text-green-400' : 'text-red-400'}`}>
                          {tx.amount > 0 ? '+' : ''}{formatGas(tx.amount)} GAS
                        </span>
                        <span className="text-gray-500 text-sm">{tx.tx_type}</span>
                      </div>
                      <div className="text-gray-500 text-sm">
                        {formatDate(tx.created_at)}
                        {tx.reference_id && (
                          <span className="ml-2 text-gray-600">Ref: {tx.reference_id.slice(0, 8)}...</span>
                        )}
                      </div>
                    </div>
                  </div>
                  <div className="text-right">
                    <div className="text-white">{formatGas(tx.balance_after)} GAS</div>
                    <div className="text-gray-500 text-sm">Balance after</div>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="p-8 text-center text-gray-400">
              No transactions yet
            </div>
          )}
        </div>
      )}
    </div>
  );
}
