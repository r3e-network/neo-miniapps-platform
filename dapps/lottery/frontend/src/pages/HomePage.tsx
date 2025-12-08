import { Link } from 'react-router-dom';
import { Ticket, Trophy, Shield, Zap, ArrowRight, Star } from 'lucide-react';
import { Countdown } from '../components/Countdown';
import { useLotteryInfo, useRecentDraws } from '../hooks/useLottery';
import { useWallet, formatGas } from '../hooks/useWallet';

export function HomePage() {
  const { address } = useWallet();
  const { data: lotteryInfo, isLoading } = useLotteryInfo();
  const { data: recentDraws } = useRecentDraws(3);

  const formatJackpot = (amount: string) => {
    const num = parseFloat(amount);
    if (num >= 1000000) {
      return `${(num / 1000000).toFixed(1)}M`;
    } else if (num >= 1000) {
      return `${(num / 1000).toFixed(0)}K`;
    }
    return formatGas(amount);
  };

  return (
    <div className="space-y-16">
      {/* Hero Section */}
      <section className="text-center py-12 md:py-20">
        <div className="animate-float mb-8">
          <div className="inline-flex items-center justify-center w-24 h-24 rounded-3xl bg-gradient-to-br from-yellow-400 via-yellow-500 to-orange-500 shadow-2xl shadow-yellow-500/30">
            <span className="text-5xl">🎰</span>
          </div>
        </div>

        <h1 className="text-4xl sm:text-5xl md:text-6xl lg:text-7xl font-extrabold text-white mb-6 leading-tight">
          Win Big with{' '}
          <span className="text-gradient-gold">MegaLottery</span>
        </h1>

        <p className="text-lg md:text-xl text-gray-400 mb-10 max-w-2xl mx-auto leading-relaxed">
          The first decentralized lottery on Neo N3. Powered by Service Layer VRF for
          provably fair and transparent draws.
        </p>

        {/* Jackpot Display */}
        <div className="jackpot-display rounded-3xl p-8 md:p-10 max-w-2xl mx-auto mb-10">
          <div className="flex items-center justify-center gap-2 text-sm text-yellow-400 uppercase tracking-widest mb-3">
            <Star className="w-4 h-4" />
            <span>Current Jackpot</span>
            <Star className="w-4 h-4" />
          </div>
          <div className="jackpot-amount text-5xl sm:text-6xl md:text-7xl font-black mb-3">
            {isLoading ? (
              <span className="skeleton inline-block w-48 h-16 rounded-lg" />
            ) : (
              `${formatJackpot(lotteryInfo?.jackpot || '0')} GAS`
            )}
          </div>
          <div className="flex items-center justify-center gap-4 text-gray-400 text-sm">
            <span>Pool: {isLoading ? '...' : formatGas(lotteryInfo?.currentPool || '0')} GAS</span>
            <span className="w-1 h-1 rounded-full bg-gray-600" />
            <span>{isLoading ? '...' : lotteryInfo?.ticketCount.toLocaleString()} tickets sold</span>
          </div>
        </div>

        {/* Countdown */}
        <div className="mb-10">
          <div className="text-sm text-gray-400 uppercase tracking-widest mb-6">
            Next Draw In
          </div>
          {lotteryInfo ? (
            <Countdown targetTime={lotteryInfo.nextDrawTime} />
          ) : (
            <div className="skeleton w-80 h-20 mx-auto rounded-xl" />
          )}
        </div>

        {/* CTA Buttons */}
        <div className="flex flex-col sm:flex-row items-center justify-center gap-4">
          <Link
            to="/buy"
            className="btn-primary flex items-center gap-3 px-8 py-4 text-lg"
          >
            <Ticket className="w-5 h-5" />
            Buy Ticket - {lotteryInfo?.ticketPrice || '2'} GAS
          </Link>
          <Link
            to="/how-to-play"
            className="flex items-center gap-2 bg-white/5 hover:bg-white/10 border border-white/10 text-white font-medium px-8 py-4 rounded-xl text-lg transition-all"
          >
            How to Play
            <ArrowRight className="w-5 h-5" />
          </Link>
        </div>

        {/* Lockout Warning */}
        {lotteryInfo?.isLocked && (
          <div className="mt-8 bg-red-500/20 border border-red-500/40 rounded-2xl p-5 max-w-md mx-auto animate-pulse-soft">
            <div className="text-red-400 font-semibold flex items-center justify-center gap-2">
              <span className="text-xl">🔒</span>
              Ticket sales locked - Draw in progress
            </div>
          </div>
        )}
      </section>

      {/* Features */}
      <section className="grid md:grid-cols-3 gap-6">
        {[
          {
            icon: Shield,
            color: 'purple',
            title: 'Provably Fair',
            description: 'Powered by Service Layer VRF. Every draw uses verifiable random numbers that can be independently verified on-chain.',
          },
          {
            icon: Zap,
            color: 'yellow',
            title: 'Automated Draws',
            description: 'Daily draws at midnight UTC, automatically triggered by Service Layer Automation. No human intervention required.',
          },
          {
            icon: Trophy,
            color: 'green',
            title: 'Instant Payouts',
            description: 'Winners can claim prizes immediately after the draw. All payouts are processed on-chain with full transparency.',
          },
        ].map((feature) => {
          const Icon = feature.icon;
          return (
            <div key={feature.title} className="glass rounded-2xl p-8 text-center group hover:bg-white/5 transition-all duration-300">
              <div className={`w-16 h-16 bg-${feature.color}-500/20 rounded-2xl flex items-center justify-center mx-auto mb-5 group-hover:scale-110 transition-transform`}>
                <Icon className={`w-8 h-8 text-${feature.color}-400`} />
              </div>
              <h3 className="text-xl font-bold text-white mb-3">{feature.title}</h3>
              <p className="text-gray-400 leading-relaxed">{feature.description}</p>
            </div>
          );
        })}
      </section>

      {/* Recent Draws */}
      {recentDraws && recentDraws.length > 0 && (
        <section>
          <div className="flex items-center justify-between mb-8">
            <h2 className="text-2xl md:text-3xl font-bold text-white">Recent Draws</h2>
            <Link
              to="/results"
              className="text-yellow-400 hover:text-yellow-300 font-medium flex items-center gap-2 group"
            >
              View All
              <ArrowRight className="w-4 h-4 group-hover:translate-x-1 transition-transform" />
            </Link>
          </div>

          <div className="grid md:grid-cols-3 gap-6">
            {recentDraws.map((draw) => (
              <div key={draw.drawId} className="glass rounded-2xl p-6 hover:bg-white/5 transition-all">
                <div className="flex items-center justify-between mb-5">
                  <span className="text-sm font-semibold text-white">Draw #{draw.drawId}</span>
                  <span className="text-xs text-gray-500 bg-white/5 px-2 py-1 rounded-full">
                    {new Date(draw.drawTime).toLocaleDateString()}
                  </span>
                </div>

                {draw.completed ? (
                  <>
                    <div className="flex items-center justify-center gap-2 mb-4">
                      {draw.winningNumbers.map((num, i) => (
                        <div
                          key={i}
                          className="lottery-ball small"
                        >
                          {num}
                        </div>
                      ))}
                      <div className="w-px h-10 bg-gradient-to-b from-transparent via-gray-600 to-transparent mx-1" />
                      <div className="lottery-ball small mega">
                        {draw.winningMega}
                      </div>
                    </div>
                    <div className="text-center text-sm text-gray-400 bg-white/5 py-2 rounded-lg">
                      Pool: <span className="text-white font-medium">{formatGas(draw.totalPool)} GAS</span>
                    </div>
                  </>
                ) : (
                  <div className="text-center py-8">
                    <div className="inline-flex items-center gap-2 text-yellow-400 animate-pulse">
                      <div className="w-2 h-2 rounded-full bg-yellow-400 animate-ping" />
                      Draw in progress...
                    </div>
                  </div>
                )}
              </div>
            ))}
          </div>
        </section>
      )}

      {/* How It Works */}
      <section className="glass rounded-3xl p-8 md:p-12">
        <h2 className="text-2xl md:text-3xl font-bold text-white text-center mb-10">How It Works</h2>
        <div className="grid sm:grid-cols-2 lg:grid-cols-4 gap-8">
          {[
            { step: 1, title: 'Connect Wallet', desc: 'Connect your Neo N3 wallet (NeoLine, OneGate, or O3)', icon: '🔗' },
            { step: 2, title: 'Pick Numbers', desc: 'Choose 5 numbers (1-70) and 1 Mega Ball (1-25)', icon: '🎯' },
            { step: 3, title: 'Buy Ticket', desc: 'Pay 2 GAS per ticket. Quick Pick available!', icon: '🎫' },
            { step: 4, title: 'Win Prizes', desc: 'Match numbers to win. Jackpot for 5+Mega!', icon: '🏆' },
          ].map((item, index) => (
            <div key={item.step} className="text-center relative">
              {index < 3 && (
                <div className="hidden lg:block absolute top-8 left-[60%] w-[80%] h-px bg-gradient-to-r from-yellow-500/50 to-transparent" />
              )}
              <div className="w-16 h-16 rounded-2xl bg-gradient-to-br from-yellow-400 to-orange-500 flex items-center justify-center text-2xl mx-auto mb-4 shadow-lg shadow-yellow-500/20">
                {item.icon}
              </div>
              <div className="text-xs text-yellow-400 font-semibold mb-2">STEP {item.step}</div>
              <h4 className="text-white font-bold mb-2">{item.title}</h4>
              <p className="text-gray-400 text-sm leading-relaxed">{item.desc}</p>
            </div>
          ))}
        </div>
      </section>

      {/* Prize Tiers */}
      <section>
        <h2 className="text-2xl md:text-3xl font-bold text-white text-center mb-8">Prize Tiers</h2>
        <div className="glass rounded-2xl overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="border-b border-white/10 bg-white/5">
                  <th className="text-left text-gray-400 font-semibold py-4 px-6">Match</th>
                  <th className="text-left text-gray-400 font-semibold py-4 px-6">Prize</th>
                  <th className="text-left text-gray-400 font-semibold py-4 px-6">Odds</th>
                </tr>
              </thead>
              <tbody>
                {[
                  { match: '5 + Mega Ball', prize: 'JACKPOT (50%)', odds: '1 in 302,575,350', highlight: true, badge: 'jackpot' },
                  { match: '5 Numbers', prize: '20% of Pool', odds: '1 in 12,607,306', badge: 'second' },
                  { match: '4 + Mega Ball', prize: '10% of Pool', odds: '1 in 931,001', badge: 'third' },
                  { match: '4 or 3 + Mega', prize: '10% of Pool', odds: '1 in 38,792', badge: 'other' },
                  { match: '3 or Mega Ball', prize: '10% of Pool', odds: '1 in 606', badge: 'other' },
                ].map((tier, i) => (
                  <tr
                    key={i}
                    className={`border-b border-white/5 transition-colors hover:bg-white/5 ${tier.highlight ? 'bg-yellow-500/5' : ''}`}
                  >
                    <td className="py-4 px-6">
                      <div className="flex items-center gap-3">
                        <span className={`prize-badge ${tier.badge}`}>{tier.match}</span>
                      </div>
                    </td>
                    <td className={`py-4 px-6 font-semibold ${tier.highlight ? 'text-yellow-400' : 'text-white'}`}>
                      {tier.prize}
                    </td>
                    <td className="py-4 px-6 text-gray-400 text-sm">{tier.odds}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="px-6 py-4 bg-white/5 border-t border-white/10">
            <p className="text-gray-400 text-sm text-center">
              * 10% of the pool goes to operations fund for platform maintenance
            </p>
          </div>
        </div>
      </section>
    </div>
  );
}
