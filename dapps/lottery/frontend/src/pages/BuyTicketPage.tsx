import { useState, useCallback } from 'react';
import { Link } from 'react-router-dom';
import { Ticket, AlertCircle, CheckCircle, Loader2, Wallet, Info, Clock, Users, Coins } from 'lucide-react';
import { NumberPicker } from '../components/NumberPicker';
import { Countdown } from '../components/Countdown';
import { useLotteryInfo, useBuyTicket, useQuickPick } from '../hooks/useLottery';
import { useWallet, formatGas } from '../hooks/useWallet';

export function BuyTicketPage() {
  const { address, balance } = useWallet();
  const { data: lotteryInfo, isLoading: infoLoading } = useLotteryInfo();
  const buyTicket = useBuyTicket();
  const quickPick = useQuickPick();

  const [selectedNumbers, setSelectedNumbers] = useState<{
    main: number[];
    mega: number | null;
  }>({ main: [], mega: null });
  const [txHash, setTxHash] = useState<string | null>(null);

  const ticketPrice = parseFloat(lotteryInfo?.ticketPrice || '2');
  const hasEnoughBalance = parseFloat(balance) >= ticketPrice;
  const isComplete = selectedNumbers.main.length === 5 && selectedNumbers.mega !== null;
  const isLocked = lotteryInfo?.isLocked || false;
  const isPaused = lotteryInfo?.isPaused || false;

  const handleNumbersSelected = useCallback((main: number[], mega: number) => {
    setSelectedNumbers({ main, mega });
  }, []);

  const handleBuyTicket = async () => {
    if (!isComplete || !selectedNumbers.mega) return;

    try {
      const hash = await buyTicket.mutateAsync({
        mainNumbers: selectedNumbers.main,
        megaNumber: selectedNumbers.mega,
      });
      setTxHash(hash);
      setSelectedNumbers({ main: [], mega: null });
    } catch (error) {
      console.error('Failed to buy ticket:', error);
    }
  };

  const handleQuickPick = async () => {
    try {
      const hash = await quickPick.mutateAsync();
      setTxHash(hash);
    } catch (error) {
      console.error('Failed to quick pick:', error);
    }
  };

  if (!address) {
    return (
      <div className="max-w-2xl mx-auto text-center py-20">
        <div className="w-24 h-24 glass rounded-3xl flex items-center justify-center mx-auto mb-8">
          <Wallet className="w-12 h-12 text-gray-400" />
        </div>
        <h1 className="text-3xl md:text-4xl font-bold text-white mb-4">Connect Your Wallet</h1>
        <p className="text-gray-400 text-lg mb-8 max-w-md mx-auto">
          Connect your Neo N3 wallet to buy lottery tickets and participate in the draw.
        </p>
        <div className="flex items-center justify-center gap-6 text-sm text-gray-500">
          <span className="flex items-center gap-2">
            <span className="w-2 h-2 rounded-full bg-green-400" />
            NeoLine
          </span>
          <span className="flex items-center gap-2">
            <span className="w-2 h-2 rounded-full bg-blue-400" />
            OneGate
          </span>
          <span className="flex items-center gap-2">
            <span className="w-2 h-2 rounded-full bg-purple-400" />
            O3 Wallet
          </span>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-6xl mx-auto">
      {/* Page Header */}
      <div className="text-center mb-10">
        <h1 className="text-3xl md:text-4xl font-bold text-white mb-3">Buy Lottery Ticket</h1>
        <p className="text-gray-400 text-lg">
          Pick 5 numbers (1-70) and 1 Mega Ball (1-25) to enter the draw
        </p>
      </div>

      {/* Status Alerts */}
      <div className="space-y-4 mb-8">
        {isLocked && (
          <div className="bg-red-500/10 border border-red-500/30 rounded-2xl p-5 flex items-start gap-4 animate-slide-up">
            <div className="w-10 h-10 rounded-xl bg-red-500/20 flex items-center justify-center flex-shrink-0">
              <AlertCircle className="w-5 h-5 text-red-400" />
            </div>
            <div>
              <div className="text-red-400 font-semibold mb-1">Ticket Sales Locked</div>
              <div className="text-red-400/70 text-sm">
                Sales are locked 1 minute before the draw. Please wait for the next draw to purchase tickets.
              </div>
            </div>
          </div>
        )}

        {isPaused && (
          <div className="bg-yellow-500/10 border border-yellow-500/30 rounded-2xl p-5 flex items-start gap-4 animate-slide-up">
            <div className="w-10 h-10 rounded-xl bg-yellow-500/20 flex items-center justify-center flex-shrink-0">
              <AlertCircle className="w-5 h-5 text-yellow-400" />
            </div>
            <div>
              <div className="text-yellow-400 font-semibold mb-1">Lottery Paused</div>
              <div className="text-yellow-400/70 text-sm">
                The lottery is temporarily paused for maintenance. Please check back later.
              </div>
            </div>
          </div>
        )}

        {txHash && (
          <div className="bg-green-500/10 border border-green-500/30 rounded-2xl p-5 flex items-start gap-4 animate-slide-up">
            <div className="w-10 h-10 rounded-xl bg-green-500/20 flex items-center justify-center flex-shrink-0">
              <CheckCircle className="w-5 h-5 text-green-400" />
            </div>
            <div className="flex-1">
              <div className="text-green-400 font-semibold mb-1">Ticket Purchased Successfully!</div>
              <div className="text-green-400/70 text-sm mb-2">
                Transaction: <code className="bg-green-500/20 px-2 py-0.5 rounded">{txHash.slice(0, 24)}...</code>
              </div>
              <Link
                to="/tickets"
                className="inline-flex items-center gap-2 text-green-400 text-sm font-medium hover:text-green-300 transition-colors"
              >
                View My Tickets →
              </Link>
            </div>
          </div>
        )}
      </div>

      <div className="grid lg:grid-cols-3 gap-8">
        {/* Number Picker - Main Content */}
        <div className="lg:col-span-2">
          <NumberPicker
            onNumbersSelected={handleNumbersSelected}
            disabled={isLocked || isPaused || buyTicket.isPending}
          />
        </div>

        {/* Sidebar */}
        <div className="space-y-6">
          {/* Draw Info Card */}
          <div className="glass rounded-2xl p-6">
            <h3 className="text-lg font-bold text-white mb-5 flex items-center gap-2">
              <Clock className="w-5 h-5 text-yellow-400" />
              Next Draw
            </h3>
            {lotteryInfo ? (
              <div className="space-y-5">
                <div className="text-center py-2">
                  <Countdown targetTime={lotteryInfo.nextDrawTime} />
                </div>
                <div className="space-y-3 pt-4 border-t border-white/10">
                  <div className="flex items-center justify-between">
                    <span className="text-gray-400 text-sm flex items-center gap-2">
                      <span className="w-1.5 h-1.5 rounded-full bg-gray-500" />
                      Draw #
                    </span>
                    <span className="text-white font-semibold">{lotteryInfo.currentDrawId}</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-gray-400 text-sm flex items-center gap-2">
                      <Coins className="w-3.5 h-3.5" />
                      Current Pool
                    </span>
                    <span className="text-yellow-400 font-semibold">{formatGas(lotteryInfo.currentPool)} GAS</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-gray-400 text-sm flex items-center gap-2">
                      <Users className="w-3.5 h-3.5" />
                      Tickets Sold
                    </span>
                    <span className="text-white font-semibold">{lotteryInfo.ticketCount.toLocaleString()}</span>
                  </div>
                </div>
              </div>
            ) : (
              <div className="space-y-3">
                <div className="skeleton h-20 rounded-xl" />
                <div className="skeleton h-4 rounded w-3/4" />
                <div className="skeleton h-4 rounded w-1/2" />
              </div>
            )}
          </div>

          {/* Purchase Summary Card */}
          <div className="glass rounded-2xl p-6">
            <h3 className="text-lg font-bold text-white mb-5 flex items-center gap-2">
              <Ticket className="w-5 h-5 text-purple-400" />
              Purchase Summary
            </h3>
            <div className="space-y-4">
              <div className="flex items-center justify-between py-3 border-b border-white/10">
                <span className="text-gray-400">Ticket Price</span>
                <span className="text-white font-bold text-lg">{ticketPrice} GAS</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-gray-400">Your Balance</span>
                <span className={`font-bold text-lg ${hasEnoughBalance ? 'text-green-400' : 'text-red-400'}`}>
                  {formatGas(balance)} GAS
                </span>
              </div>
              {!hasEnoughBalance && (
                <div className="bg-red-500/10 border border-red-500/30 rounded-xl p-3 text-red-400 text-sm">
                  Insufficient balance. Please deposit more GAS to your wallet.
                </div>
              )}
            </div>

            <div className="space-y-3 mt-6 pt-6 border-t border-white/10">
              <button
                onClick={handleBuyTicket}
                disabled={!isComplete || !hasEnoughBalance || isLocked || isPaused || buyTicket.isPending}
                className="w-full btn-primary py-3.5 flex items-center justify-center gap-2 text-base"
              >
                {buyTicket.isPending ? (
                  <>
                    <Loader2 className="w-5 h-5 animate-spin" />
                    Processing...
                  </>
                ) : (
                  <>
                    <Ticket className="w-5 h-5" />
                    Buy Ticket
                  </>
                )}
              </button>

              <button
                onClick={handleQuickPick}
                disabled={!hasEnoughBalance || isLocked || isPaused || quickPick.isPending}
                className="w-full btn-secondary py-3.5 flex items-center justify-center gap-2 text-base"
              >
                {quickPick.isPending ? (
                  <>
                    <Loader2 className="w-5 h-5 animate-spin" />
                    Processing...
                  </>
                ) : (
                  <>
                    <span className="text-lg">🎲</span>
                    Quick Pick (Random)
                  </>
                )}
              </button>
            </div>
          </div>

          {/* Tips Card */}
          <div className="bg-blue-500/5 border border-blue-500/20 rounded-2xl p-5">
            <h4 className="text-blue-400 font-semibold mb-3 flex items-center gap-2">
              <Info className="w-4 h-4" />
              Tips
            </h4>
            <ul className="text-blue-400/80 text-sm space-y-2">
              <li className="flex items-start gap-2">
                <span className="w-1 h-1 rounded-full bg-blue-400 mt-2 flex-shrink-0" />
                Sales lock 1 minute before each draw
              </li>
              <li className="flex items-start gap-2">
                <span className="w-1 h-1 rounded-full bg-blue-400 mt-2 flex-shrink-0" />
                Quick Pick generates random numbers using blockchain entropy
              </li>
              <li className="flex items-start gap-2">
                <span className="w-1 h-1 rounded-full bg-blue-400 mt-2 flex-shrink-0" />
                Check results after midnight UTC daily
              </li>
              <li className="flex items-start gap-2">
                <span className="w-1 h-1 rounded-full bg-blue-400 mt-2 flex-shrink-0" />
                Claim prizes within 30 days of the draw
              </li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  );
}
