import React, { useState } from 'react';
import { useMixer } from '../hooks/useMixer';
import { useWallet } from '../hooks/useWallet';
import { parseNote } from '../utils/crypto';

export function WithdrawPage() {
  const { withdraw } = useMixer();
  const { connected, signAndInvoke } = useWallet();
  const [note, setNote] = useState('');
  const [recipient, setRecipient] = useState('');
  const [processing, setProcessing] = useState(false);

  const handleWithdraw = async () => {
    if (!connected) {
      alert('Please connect your wallet');
      return;
    }
    if (!note || !recipient) {
      alert('Please enter your note and recipient address');
      return;
    }

    try {
      setProcessing(true);

      // Parse note
      const { commitment, nullifier, secret } = parseNote(note);

      // Generate proof and execute withdrawal
      const script = await withdraw(nullifier, commitment, recipient, secret);
      await signAndInvoke(script);

      alert('Withdrawal requested! Funds will be available after the delay period.');
      setNote('');
      setRecipient('');
    } catch (error) {
      alert('Withdrawal failed: ' + (error as Error).message);
    } finally {
      setProcessing(false);
    }
  };

  return (
    <div className="max-w-xl mx-auto">
      <h1 className="text-3xl font-bold text-white mb-8 text-center">Withdraw from Mixer</h1>

      <div className="bg-white/5 backdrop-blur-md rounded-2xl p-8 border border-white/10">
        {/* Note Input */}
        <div className="mb-6">
          <label className="block text-white/80 mb-2">Your Secret Note</label>
          <textarea
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder="Paste your secret note here..."
            className="w-full bg-white/5 border border-white/20 rounded-xl p-4 text-white placeholder-white/40 focus:border-purple-500 focus:outline-none resize-none h-32"
          />
        </div>

        {/* Recipient Input */}
        <div className="mb-6">
          <label className="block text-white/80 mb-2">Recipient Address</label>
          <input
            type="text"
            value={recipient}
            onChange={(e) => setRecipient(e.target.value)}
            placeholder="Neo N3 address (N...)"
            className="w-full bg-white/5 border border-white/20 rounded-xl p-4 text-white placeholder-white/40 focus:border-purple-500 focus:outline-none"
          />
          <p className="text-white/40 text-xs mt-2">
            Use a fresh address that has never been linked to your deposit address
          </p>
        </div>

        {/* Info */}
        <div className="bg-blue-500/10 border border-blue-500/30 rounded-xl p-4 mb-6">
          <div className="flex items-start">
            <svg className="w-5 h-5 text-blue-400 mr-3 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <div>
              <p className="text-blue-400 font-semibold text-sm">Withdrawal Delay</p>
              <p className="text-blue-400/80 text-sm">
                For privacy, withdrawals have a random delay of 1-24 hours. You can complete the withdrawal after the delay period.
              </p>
            </div>
          </div>
        </div>

        {/* Withdraw Button */}
        <button
          onClick={handleWithdraw}
          disabled={!connected || processing || !note || !recipient}
          className={`w-full py-4 rounded-xl font-bold text-lg transition ${
            connected && !processing && note && recipient
              ? 'bg-gradient-to-r from-purple-500 to-pink-500 text-white hover:opacity-90'
              : 'bg-gray-600 text-gray-400 cursor-not-allowed'
          }`}
        >
          {processing ? 'Processing...' : 'Request Withdrawal'}
        </button>
      </div>
    </div>
  );
}
