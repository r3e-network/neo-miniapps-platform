import { Link } from 'react-router-dom';
import { Ticket, Shield, Clock, Trophy, Wallet, HelpCircle } from 'lucide-react';

export function HowToPlayPage() {
  return (
    <div className="max-w-4xl mx-auto">
      <div className="text-center mb-12">
        <h1 className="text-3xl font-bold text-white mb-2">How to Play MegaLottery</h1>
        <p className="text-gray-400">Everything you need to know about playing and winning</p>
      </div>

      {/* Getting Started */}
      <section className="mb-12">
        <h2 className="text-2xl font-bold text-white mb-6 flex items-center gap-3">
          <Wallet className="w-6 h-6 text-yellow-400" />
          Getting Started
        </h2>
        <div className="glass rounded-xl p-6 space-y-4">
          <div className="flex gap-4">
            <div className="w-8 h-8 rounded-full bg-yellow-500 text-black flex items-center justify-center font-bold flex-shrink-0">
              1
            </div>
            <div>
              <h3 className="text-white font-semibold mb-1">Install a Neo N3 Wallet</h3>
              <p className="text-gray-400 text-sm">
                Download and install one of the supported wallets: NeoLine (browser extension),
                OneGate, or O3 Wallet. Create a new wallet or import an existing one.
              </p>
            </div>
          </div>
          <div className="flex gap-4">
            <div className="w-8 h-8 rounded-full bg-yellow-500 text-black flex items-center justify-center font-bold flex-shrink-0">
              2
            </div>
            <div>
              <h3 className="text-white font-semibold mb-1">Get Some GAS</h3>
              <p className="text-gray-400 text-sm">
                You'll need GAS tokens to buy tickets. Each ticket costs 2 GAS. You can get GAS
                from exchanges or swap NEO for GAS.
              </p>
            </div>
          </div>
          <div className="flex gap-4">
            <div className="w-8 h-8 rounded-full bg-yellow-500 text-black flex items-center justify-center font-bold flex-shrink-0">
              3
            </div>
            <div>
              <h3 className="text-white font-semibold mb-1">Connect Your Wallet</h3>
              <p className="text-gray-400 text-sm">
                Click "Connect Wallet" in the header and select your wallet. Approve the
                connection request in your wallet.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* How to Play */}
      <section className="mb-12">
        <h2 className="text-2xl font-bold text-white mb-6 flex items-center gap-3">
          <Ticket className="w-6 h-6 text-yellow-400" />
          Buying Tickets
        </h2>
        <div className="glass rounded-xl p-6 space-y-4">
          <div>
            <h3 className="text-white font-semibold mb-2">Pick Your Numbers</h3>
            <p className="text-gray-400 text-sm mb-3">
              Choose <strong className="text-yellow-400">5 main numbers</strong> from 1-70 and{' '}
              <strong className="text-red-400">1 Mega Ball</strong> from 1-25.
            </p>
            <div className="flex items-center gap-2 flex-wrap">
              <div className="lottery-ball">7</div>
              <div className="lottery-ball">14</div>
              <div className="lottery-ball">28</div>
              <div className="lottery-ball">45</div>
              <div className="lottery-ball">62</div>
              <div className="w-px h-14 bg-gray-600 mx-2" />
              <div className="lottery-ball mega">12</div>
            </div>
          </div>
          <div>
            <h3 className="text-white font-semibold mb-2">Quick Pick Option</h3>
            <p className="text-gray-400 text-sm">
              Don't want to choose? Use Quick Pick to have the system randomly generate numbers
              for you. It's just as likely to win!
            </p>
          </div>
          <div>
            <h3 className="text-white font-semibold mb-2">Confirm Purchase</h3>
            <p className="text-gray-400 text-sm">
              Review your numbers and click "Buy Ticket". Approve the transaction in your wallet.
              The ticket price (2 GAS) will be deducted from your balance.
            </p>
          </div>
        </div>
      </section>

      {/* Prize Tiers */}
      <section className="mb-12">
        <h2 className="text-2xl font-bold text-white mb-6 flex items-center gap-3">
          <Trophy className="w-6 h-6 text-yellow-400" />
          Prize Tiers
        </h2>
        <div className="glass rounded-xl overflow-hidden">
          <table className="w-full">
            <thead className="bg-gray-800/50">
              <tr>
                <th className="text-left text-gray-400 font-medium py-3 px-4">Match</th>
                <th className="text-left text-gray-400 font-medium py-3 px-4">Prize</th>
                <th className="text-left text-gray-400 font-medium py-3 px-4">Example</th>
              </tr>
            </thead>
            <tbody>
              <tr className="border-t border-gray-700 bg-yellow-500/10">
                <td className="py-3 px-4 text-yellow-400 font-semibold">5 + Mega Ball</td>
                <td className="py-3 px-4 text-yellow-400 font-semibold">JACKPOT (50%)</td>
                <td className="py-3 px-4 text-gray-400">All numbers match!</td>
              </tr>
              <tr className="border-t border-gray-700">
                <td className="py-3 px-4 text-white">5 Numbers</td>
                <td className="py-3 px-4 text-white">20% of Pool</td>
                <td className="py-3 px-4 text-gray-400">All main numbers, no Mega</td>
              </tr>
              <tr className="border-t border-gray-700">
                <td className="py-3 px-4 text-white">4 + Mega Ball</td>
                <td className="py-3 px-4 text-white">10% of Pool</td>
                <td className="py-3 px-4 text-gray-400">4 main + Mega Ball</td>
              </tr>
              <tr className="border-t border-gray-700">
                <td className="py-3 px-4 text-white">4 or 3 + Mega</td>
                <td className="py-3 px-4 text-white">10% of Pool</td>
                <td className="py-3 px-4 text-gray-400">4 main OR 3 main + Mega</td>
              </tr>
              <tr className="border-t border-gray-700">
                <td className="py-3 px-4 text-white">3 or Mega Ball</td>
                <td className="py-3 px-4 text-white">10% of Pool</td>
                <td className="py-3 px-4 text-gray-400">3 main OR just Mega Ball</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p className="text-gray-500 text-sm mt-3">
          * 10% of the pool goes to operations and development
        </p>
      </section>

      {/* Draw Schedule */}
      <section className="mb-12">
        <h2 className="text-2xl font-bold text-white mb-6 flex items-center gap-3">
          <Clock className="w-6 h-6 text-yellow-400" />
          Draw Schedule
        </h2>
        <div className="glass rounded-xl p-6 space-y-4">
          <div>
            <h3 className="text-white font-semibold mb-2">Daily Draws</h3>
            <p className="text-gray-400 text-sm">
              Draws happen every day at <strong className="text-white">00:00 UTC</strong> (midnight).
              The draw is automatically triggered by Service Layer NeoFlow.
            </p>
          </div>
          <div>
            <h3 className="text-white font-semibold mb-2">Sales Lockout</h3>
            <p className="text-gray-400 text-sm">
              Ticket sales are <strong className="text-red-400">locked 1 minute before</strong> each
              draw to ensure fairness. Plan your purchases accordingly!
            </p>
          </div>
          <div>
            <h3 className="text-white font-semibold mb-2">Results</h3>
            <p className="text-gray-400 text-sm">
              Winning numbers are generated using Service Layer VRF (Verifiable Random Function)
              and posted immediately after the draw. Check the Results page or your tickets.
            </p>
          </div>
        </div>
      </section>

      {/* Security */}
      <section className="mb-12">
        <h2 className="text-2xl font-bold text-white mb-6 flex items-center gap-3">
          <Shield className="w-6 h-6 text-yellow-400" />
          Security & Fairness
        </h2>
        <div className="glass rounded-xl p-6 space-y-4">
          <div>
            <h3 className="text-white font-semibold mb-2">Verifiable Random Numbers</h3>
            <p className="text-gray-400 text-sm">
              All winning numbers are generated using Service Layer's VRF (Verifiable Random
              Function). This cryptographic proof ensures that numbers are truly random and
              cannot be manipulated by anyone, including the lottery operators.
            </p>
          </div>
          <div>
            <h3 className="text-white font-semibold mb-2">On-Chain Transparency</h3>
            <p className="text-gray-400 text-sm">
              Every ticket purchase, draw result, and prize payout is recorded on the Neo N3
              blockchain. Anyone can verify the fairness of the lottery by checking the
              smart contract.
            </p>
          </div>
          <div>
            <h3 className="text-white font-semibold mb-2">Automated Operations</h3>
            <p className="text-gray-400 text-sm">
              Draws are triggered automatically by Service Layer NeoFlow. No human
              intervention is required, eliminating the possibility of manipulation.
            </p>
          </div>
        </div>
      </section>

      {/* FAQ */}
      <section className="mb-12">
        <h2 className="text-2xl font-bold text-white mb-6 flex items-center gap-3">
          <HelpCircle className="w-6 h-6 text-yellow-400" />
          Frequently Asked Questions
        </h2>
        <div className="space-y-4">
          {[
            {
              q: 'How much does a ticket cost?',
              a: 'Each ticket costs 2 GAS. You can buy multiple tickets for the same draw.',
            },
            {
              q: 'When can I claim my prize?',
              a: 'Prizes can be claimed immediately after the draw is completed. Go to "My Tickets" and click "Claim Prize" on any winning ticket.',
            },
            {
              q: 'What happens if no one wins the jackpot?',
              a: 'The jackpot rolls over to the next draw, making it even bigger!',
            },
            {
              q: 'How long do I have to claim my prize?',
              a: 'Prizes must be claimed within 30 days of the draw. Unclaimed prizes are added to the operations fund.',
            },
            {
              q: 'Can I buy tickets from any country?',
              a: 'MegaLottery is a decentralized application. Anyone with a Neo N3 wallet can participate. Please check your local laws regarding online lotteries.',
            },
          ].map((faq, i) => (
            <div key={i} className="glass rounded-xl p-5">
              <h3 className="text-white font-semibold mb-2">{faq.q}</h3>
              <p className="text-gray-400 text-sm">{faq.a}</p>
            </div>
          ))}
        </div>
      </section>

      {/* CTA */}
      <div className="text-center">
        <Link
          to="/buy"
          className="inline-flex items-center gap-2 bg-yellow-500 hover:bg-yellow-400 text-black font-bold px-8 py-4 rounded-xl text-lg transition-colors"
        >
          <Ticket className="w-5 h-5" />
          Buy Your First Ticket
        </Link>
      </div>
    </div>
  );
}
