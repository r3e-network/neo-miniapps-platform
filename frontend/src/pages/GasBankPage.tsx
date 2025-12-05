import React, { useEffect, useState } from 'react';
import {
  FiDollarSign,
  FiArrowDownCircle,
  FiArrowUpCircle,
  FiCopy,
  FiRefreshCw,
  FiExternalLink,
} from 'react-icons/fi';
import { Card, CardHeader, Button, Input, Modal, Badge } from '@/components/common';
import { WalletConnect } from '@/components/auth/WalletConnect';
import { useGasBankStore } from '@/stores/gasbankStore';
import { useAuthStore } from '@/stores/authStore';
import toast from 'react-hot-toast';

export function GasBankPage() {
  const { account, transactions, isLoading, fetchAccount, fetchTransactions, deposit, withdraw, getDepositAddress } = useGasBankStore();
  const { wallet } = useAuthStore();
  const [showDepositModal, setShowDepositModal] = useState(false);
  const [showWithdrawModal, setShowWithdrawModal] = useState(false);
  const [depositAddress, setDepositAddress] = useState<string | null>(null);
  const [depositForm, setDepositForm] = useState({ amount: '', txHash: '' });
  const [withdrawForm, setWithdrawForm] = useState({ amount: '', toAddress: '' });
  const [isProcessing, setIsProcessing] = useState(false);

  useEffect(() => {
    fetchAccount();
    fetchTransactions();
  }, [fetchAccount, fetchTransactions]);

  const handleOpenDeposit = async () => {
    setShowDepositModal(true);
    const address = await getDepositAddress();
    setDepositAddress(address);
  };

  const handleDeposit = async () => {
    if (!depositForm.amount || !depositForm.txHash) {
      toast.error('Please fill in all fields');
      return;
    }

    setIsProcessing(true);
    const success = await deposit({
      amount: depositForm.amount,
      txHash: depositForm.txHash,
    });

    if (success) {
      toast.success('Deposit submitted successfully');
      setShowDepositModal(false);
      setDepositForm({ amount: '', txHash: '' });
      fetchAccount();
    } else {
      toast.error('Failed to submit deposit');
    }
    setIsProcessing(false);
  };

  const handleWithdraw = async () => {
    if (!withdrawForm.amount || !withdrawForm.toAddress) {
      toast.error('Please fill in all fields');
      return;
    }

    setIsProcessing(true);
    const success = await withdraw(withdrawForm.amount, withdrawForm.toAddress);

    if (success) {
      toast.success('Withdrawal initiated');
      setShowWithdrawModal(false);
      setWithdrawForm({ amount: '', toAddress: '' });
      fetchAccount();
    } else {
      toast.error('Failed to initiate withdrawal');
    }
    setIsProcessing(false);
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    toast.success('Copied to clipboard');
  };

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'confirmed':
        return <Badge variant="success">Confirmed</Badge>;
      case 'pending':
        return <Badge variant="warning">Pending</Badge>;
      case 'failed':
        return <Badge variant="error">Failed</Badge>;
      default:
        return <Badge>{status}</Badge>;
    }
  };

  const getTypeIcon = (type: string) => {
    switch (type) {
      case 'deposit':
        return <FiArrowDownCircle className="w-5 h-5 text-green-500" />;
      case 'withdraw':
      case 'sponsorship':
        return <FiArrowUpCircle className="w-5 h-5 text-red-500" />;
      default:
        return <FiDollarSign className="w-5 h-5 text-surface-400" />;
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-surface-900">GasBank</h1>
          <p className="text-surface-500 mt-1">
            Manage your GAS balance for service operations
          </p>
        </div>
        <Button
          variant="secondary"
          leftIcon={<FiRefreshCw className="w-4 h-4" />}
          onClick={() => {
            fetchAccount();
            fetchTransactions();
          }}
        >
          Refresh
        </Button>
      </div>

      {/* Balance Card */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card className="md:col-span-2 bg-gradient-to-br from-primary-600 to-primary-800 text-white">
          <div className="flex items-start justify-between">
            <div>
              <p className="text-primary-200 text-sm">Available Balance</p>
              <p className="text-4xl font-bold mt-2">{account?.balance || '0.0000'}</p>
              <p className="text-primary-200 mt-1">GAS</p>
            </div>
            <div className="p-3 bg-white/10 rounded-xl">
              <FiDollarSign className="w-8 h-8" />
            </div>
          </div>

          <div className="flex gap-3 mt-6">
            <Button
              variant="secondary"
              className="flex-1 bg-white text-primary-700 hover:bg-primary-50"
              leftIcon={<FiArrowDownCircle className="w-4 h-4" />}
              onClick={handleOpenDeposit}
            >
              Deposit
            </Button>
            <Button
              variant="secondary"
              className="flex-1 bg-white/10 text-white border-white/20 hover:bg-white/20"
              leftIcon={<FiArrowUpCircle className="w-4 h-4" />}
              onClick={() => setShowWithdrawModal(true)}
            >
              Withdraw
            </Button>
          </div>
        </Card>

        <Card>
          <CardHeader title="Statistics" />
          <div className="space-y-4">
            <div className="flex justify-between">
              <span className="text-surface-500">Total Deposited</span>
              <span className="font-medium">{account?.totalDeposited || '0'} GAS</span>
            </div>
            <div className="flex justify-between">
              <span className="text-surface-500">Total Spent</span>
              <span className="font-medium">{account?.totalSpent || '0'} GAS</span>
            </div>
            <div className="flex justify-between">
              <span className="text-surface-500">Transactions</span>
              <span className="font-medium">{transactions.length}</span>
            </div>
          </div>
        </Card>
      </div>

      {/* Wallet Connection */}
      {!wallet.connected && (
        <Card>
          <CardHeader
            title="Connect Wallet"
            description="Connect your Neo N3 wallet to deposit GAS directly"
          />
          <WalletConnect />
        </Card>
      )}

      {/* Transaction History */}
      <Card padding="none">
        <div className="p-4 border-b border-surface-200">
          <h3 className="font-semibold text-surface-900">Transaction History</h3>
        </div>
        <div className="divide-y divide-surface-200">
          {isLoading ? (
            <div className="p-8 text-center text-surface-400">Loading transactions...</div>
          ) : transactions.length === 0 ? (
            <div className="p-8 text-center">
              <FiDollarSign className="w-12 h-12 mx-auto text-surface-300 mb-3" />
              <p className="text-surface-500">No transactions yet</p>
              <p className="text-sm text-surface-400 mt-1">
                Deposit GAS to start using services
              </p>
            </div>
          ) : (
            transactions.map((tx) => (
              <div key={tx.id} className="p-4 hover:bg-surface-50 transition-colors">
                <div className="flex items-center gap-4">
                  <div className="p-2 bg-surface-100 rounded-lg">
                    {getTypeIcon(tx.type)}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-surface-900 capitalize">{tx.type}</span>
                      {getStatusBadge(tx.status)}
                    </div>
                    {tx.txHash && (
                      <p className="text-xs text-surface-400 font-mono mt-1 truncate">
                        {tx.txHash}
                      </p>
                    )}
                    <p className="text-xs text-surface-400 mt-1">
                      {new Date(tx.createdAt).toLocaleString()}
                    </p>
                  </div>
                  <div className="text-right">
                    <p
                      className={`font-semibold ${
                        tx.type === 'deposit' || tx.type === 'refund'
                          ? 'text-green-600'
                          : 'text-red-600'
                      }`}
                    >
                      {tx.type === 'deposit' || tx.type === 'refund' ? '+' : '-'}
                      {tx.amount} GAS
                    </p>
                  </div>
                </div>
              </div>
            ))
          )}
        </div>
      </Card>

      {/* Deposit Modal */}
      <Modal isOpen={showDepositModal} onClose={() => setShowDepositModal(false)} title="Deposit GAS">
        <div className="space-y-4">
          {depositAddress && (
            <div className="p-4 bg-surface-50 rounded-lg">
              <p className="text-sm text-surface-500 mb-2">Send GAS to this address:</p>
              <div className="flex items-center gap-2">
                <code className="flex-1 text-sm font-mono text-surface-900 break-all">
                  {depositAddress}
                </code>
                <button
                  onClick={() => copyToClipboard(depositAddress)}
                  className="p-2 text-surface-400 hover:text-surface-600"
                >
                  <FiCopy className="w-4 h-4" />
                </button>
              </div>
            </div>
          )}

          <Input
            label="Amount (GAS)"
            type="number"
            placeholder="10.0"
            value={depositForm.amount}
            onChange={(e) => setDepositForm({ ...depositForm, amount: e.target.value })}
          />

          <Input
            label="Transaction Hash"
            placeholder="0x..."
            value={depositForm.txHash}
            onChange={(e) => setDepositForm({ ...depositForm, txHash: e.target.value })}
            helperText="Enter the transaction hash after sending GAS"
          />

          <div className="flex gap-3 pt-4">
            <Button variant="secondary" onClick={() => setShowDepositModal(false)} className="flex-1">
              Cancel
            </Button>
            <Button onClick={handleDeposit} isLoading={isProcessing} className="flex-1">
              Confirm Deposit
            </Button>
          </div>
        </div>
      </Modal>

      {/* Withdraw Modal */}
      <Modal isOpen={showWithdrawModal} onClose={() => setShowWithdrawModal(false)} title="Withdraw GAS">
        <div className="space-y-4">
          <div className="p-4 bg-yellow-50 rounded-lg border border-yellow-200">
            <p className="text-sm text-yellow-800">
              Available balance: <strong>{account?.balance || '0'} GAS</strong>
            </p>
          </div>

          <Input
            label="Amount (GAS)"
            type="number"
            placeholder="10.0"
            value={withdrawForm.amount}
            onChange={(e) => setWithdrawForm({ ...withdrawForm, amount: e.target.value })}
          />

          <Input
            label="Destination Address"
            placeholder="NXV7ZhHiyM1aHXwpVsRZC6BEDrmrLAW3sM"
            value={withdrawForm.toAddress}
            onChange={(e) => setWithdrawForm({ ...withdrawForm, toAddress: e.target.value })}
          />

          <div className="flex gap-3 pt-4">
            <Button variant="secondary" onClick={() => setShowWithdrawModal(false)} className="flex-1">
              Cancel
            </Button>
            <Button onClick={handleWithdraw} isLoading={isProcessing} className="flex-1">
              Withdraw
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
