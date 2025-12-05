import React, { useState } from 'react';
import { useMixer } from '../hooks/useMixer';
import { useWallet } from '../hooks/useWallet';
import { generateCommitment, downloadNote } from '../utils/crypto';

export function DepositPage() {
  const { deposit, pools } = useMixer();
  const { connected, signAndInvoke } = useWallet();
  const [selectedPool, setSelectedPool] = useState<number>(1);
  const [processing, setProcessing] = useState(false);
  const [note, setNote] = useState<string | null>(null);

  const poolOptions = [
    { id: 1, amount: '1 GAS', value: 1 },
    { id: 2, amount: '10 GAS', value: 10 },
    { id: 3, amount: '100 GAS', value: 100 },
  ];

  const handleDeposit = async () => {
    if (!connected) {
      alert('Please connect your wallet');
      return;
    }

    try {
      setProcessing(true);

      // Generate commitment and note
      const { commitment, secret, nullifier, note: noteString } = generateCommitment();

      // Execute deposit
      const script = await deposit(selectedPool, commitment);
      await signAndInvoke(script);

      // Save note for user
      setNote(noteString);

      alert('Deposit successful! Save your note to withdraw later.');
    } catch (error) {
      alert('Deposit failed: ' + (error as Error).message);
    } finally {
      setProcessing(false);
    }
  };

  const handleDownloadNote = () => {
    if (note) {
      downloadNote(note, `mixer-note-${Date.now()}.txt`);
    }
  };

  return (
    <div className="max-w-xl mx-auto">
      <h1 className="text-3xl font-bold text-white mb-8 text-center">Deposit to Mixer</h1>

      <div className="bg-white/5 backdrop-blur-md rounded-2xl p-8 border border-white/10">
        {/* Pool Selection */}
        <div className="mb-6">
          <label className="block text-white/80 mb-3">Select Pool</label>
          <div className="grid grid-cols-3 gap-3">
            {poolOptions.map(pool => (
              <button
                key={pool.id}
                onClick={() => setSelectedPool(pool.id)}
                className={`p-4 rounded-xl border-2 transition ${
                  selectedPool === pool.id
                    ? 'border-purple-500 bg-purple-500/20'
                    : 'border-white/10 bg-white/5 hover:border-white/30'
                }`}
              >
                <p className="text-2xl font-bold text-white">{pool.value}</p>
                <p className="text-white/60 text-sm">GAS</p>
              </button>
            ))}
          </div>
        </div>

        {/* Info */}
        <div className="bg-yellow-500/10 border border-yellow-500/30 rounded-xl p-4 mb-6">
          <div className="flex items-start">
            <svg className="w-5 h-5 text-yellow-400 mr-3 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
            <div>
              <p className="text-yellow-400 font-semibold text-sm">Important</p>
              <p className="text-yellow-400/80 text-sm">
                After depositing, you will receive a secret note. Save it securely - it's the only way to withdraw your funds!
              </p>
            </div>
          </div>
        </div>

        {/* Note Display */}
        {note && (
          <div className="bg-green-500/10 border border-green-500/30 rounded-xl p-4 mb-6">
            <p className="text-green-400 font-semibold text-sm mb-2">Your Secret Note</p>
            <div className="bg-black/30 rounded-lg p-3 mb-3">
              <code className="text-green-300 text-xs break-all">{note}</code>
            </div>
            <button
              onClick={handleDownloadNote}
              className="w-full bg-green-500/20 text-green-400 py-2 rounded-lg hover:bg-green-500/30 transition"
            >
              Download Note
            </button>
          </div>
        )}

        {/* Deposit Button */}
        <button
          onClick={handleDeposit}
          disabled={!connected || processing}
          className={`w-full py-4 rounded-xl font-bold text-lg transition ${
            connected && !processing
              ? 'bg-gradient-to-r from-purple-500 to-pink-500 text-white hover:opacity-90'
              : 'bg-gray-600 text-gray-400 cursor-not-allowed'
          }`}
        >
          {processing ? 'Processing...' : connected ? `Deposit ${poolOptions.find(p => p.id === selectedPool)?.amount}` : 'Connect Wallet'}
        </button>
      </div>
    </div>
  );
}
