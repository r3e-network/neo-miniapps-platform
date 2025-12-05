import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useLottery } from '../hooks/useLottery';
import { useWallet } from '../hooks/useWallet';

export function BuyTicketPage() {
  const { currentRound, buyTicket } = useLottery();
  const { connected, signAndInvoke } = useWallet();
  const navigate = useNavigate();

  const [selectedNumbers, setSelectedNumbers] = useState<number[]>([]);
  const [purchasing, setPurchasing] = useState(false);

  const toggleNumber = (num: number) => {
    if (selectedNumbers.includes(num)) {
      setSelectedNumbers(selectedNumbers.filter(n => n !== num));
    } else if (selectedNumbers.length < 6) {
      setSelectedNumbers([...selectedNumbers, num]);
    }
  };

  const quickPick = () => {
    const numbers: number[] = [];
    while (numbers.length < 6) {
      const num = Math.floor(Math.random() * 49) + 1;
      if (!numbers.includes(num)) numbers.push(num);
    }
    setSelectedNumbers(numbers.sort((a, b) => a - b));
  };

  const handlePurchase = async () => {
    if (!connected) {
      alert('Please connect your wallet first');
      return;
    }
    if (selectedNumbers.length !== 6) {
      alert('Please select 6 numbers');
      return;
    }

    try {
      setPurchasing(true);
      const script = await buyTicket(selectedNumbers);
      await signAndInvoke(script);
      alert('Ticket purchased successfully!');
      navigate('/tickets');
    } catch (error) {
      alert('Failed to purchase ticket: ' + (error as Error).message);
    } finally {
      setPurchasing(false);
    }
  };

  return (
    <div className="max-w-2xl mx-auto">
      <h1 className="text-3xl font-bold text-white mb-8 text-center">Buy Lottery Ticket</h1>

      {currentRound && currentRound.status === 0 ? (
        <div className="bg-white/10 backdrop-blur-md rounded-2xl p-8 border border-white/20">
          <div className="text-center mb-6">
            <p className="text-white/60">Round #{currentRound.roundId}</p>
            <p className="text-2xl font-bold text-yellow-400">{currentRound.ticketPrice} GAS per ticket</p>
          </div>

          <div className="mb-6">
            <div className="flex justify-between items-center mb-4">
              <p className="text-white">Select 6 numbers (1-49)</p>
              <button
                onClick={quickPick}
                className="text-yellow-400 hover:text-yellow-300 text-sm"
              >
                Quick Pick
              </button>
            </div>

            <div className="grid grid-cols-7 gap-2">
              {Array.from({ length: 49 }, (_, i) => i + 1).map(num => (
                <button
                  key={num}
                  onClick={() => toggleNumber(num)}
                  className={`w-10 h-10 rounded-full font-semibold transition ${
                    selectedNumbers.includes(num)
                      ? 'bg-gradient-to-br from-yellow-400 to-orange-500 text-black'
                      : 'bg-white/10 text-white hover:bg-white/20'
                  }`}
                >
                  {num}
                </button>
              ))}
            </div>
          </div>

          <div className="bg-white/5 rounded-xl p-4 mb-6">
            <p className="text-white/60 text-sm mb-2">Your Numbers</p>
            <div className="flex space-x-3">
              {selectedNumbers.length > 0 ? (
                selectedNumbers.sort((a, b) => a - b).map((num, i) => (
                  <div
                    key={i}
                    className="w-10 h-10 rounded-full bg-gradient-to-br from-yellow-400 to-orange-500 flex items-center justify-center text-black font-bold"
                  >
                    {num}
                  </div>
                ))
              ) : (
                <p className="text-white/40">No numbers selected</p>
              )}
            </div>
          </div>

          <button
            onClick={handlePurchase}
            disabled={selectedNumbers.length !== 6 || purchasing || !connected}
            className={`w-full py-4 rounded-xl font-bold text-lg transition ${
              selectedNumbers.length === 6 && connected && !purchasing
                ? 'bg-gradient-to-r from-yellow-400 to-orange-500 text-black hover:opacity-90'
                : 'bg-gray-600 text-gray-400 cursor-not-allowed'
            }`}
          >
            {purchasing ? 'Processing...' : connected ? 'Purchase Ticket' : 'Connect Wallet to Purchase'}
          </button>
        </div>
      ) : (
        <div className="bg-white/10 backdrop-blur-md rounded-2xl p-8 text-center">
          <p className="text-white/70">No active lottery round available for ticket purchase</p>
        </div>
      )}
    </div>
  );
}
