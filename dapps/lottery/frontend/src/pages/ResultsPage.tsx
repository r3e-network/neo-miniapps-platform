import { Trophy } from 'lucide-react';
import { useRecentDraws } from '../hooks/useLottery';
import { formatGas } from '../hooks/useWallet';

export function ResultsPage() {
  const { data: draws, isLoading } = useRecentDraws(20);

  const formatDate = (timestamp: number) => {
    return new Date(timestamp).toLocaleDateString('en-US', {
      weekday: 'short',
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    });
  };

  return (
    <div className="max-w-4xl mx-auto">
      <div className="text-center mb-8">
        <h1 className="text-3xl font-bold text-white mb-2">Draw Results</h1>
        <p className="text-gray-400">View past lottery draw results and winning numbers</p>
      </div>

      {isLoading ? (
        <div className="text-center py-12">
          <div className="w-12 h-12 border-4 border-yellow-500 border-t-transparent rounded-full animate-spin mx-auto mb-4" />
          <p className="text-gray-400">Loading results...</p>
        </div>
      ) : draws && draws.length > 0 ? (
        <div className="space-y-4">
          {draws.map((draw) => (
            <div
              key={draw.drawId}
              className={`glass rounded-xl p-6 ${
                draw.completed ? '' : 'border-2 border-yellow-500/50'
              }`}
            >
              <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
                <div>
                  <div className="flex items-center gap-3 mb-2">
                    <span className="text-lg font-semibold text-white">
                      Draw #{draw.drawId}
                    </span>
                    {!draw.completed && (
                      <span className="bg-yellow-500/20 text-yellow-400 text-xs px-2 py-1 rounded">
                        In Progress
                      </span>
                    )}
                  </div>
                  <div className="text-sm text-gray-400">
                    {formatDate(draw.drawTime)}
                  </div>
                </div>

                {draw.completed ? (
                  <div className="flex items-center gap-2">
                    {draw.winningNumbers.map((num, i) => (
                      <div
                        key={i}
                        className="lottery-ball winning"
                        style={{ animationDelay: `${i * 0.1}s` }}
                      >
                        {num}
                      </div>
                    ))}
                    <div className="w-px h-14 bg-gray-600 mx-2" />
                    <div className="lottery-ball mega winning">
                      {draw.winningMega}
                    </div>
                  </div>
                ) : (
                  <div className="text-yellow-400 font-medium">
                    🎰 Drawing in progress...
                  </div>
                )}

                <div className="text-right">
                  <div className="text-sm text-gray-400">Prize Pool</div>
                  <div className="text-lg font-semibold text-white">
                    {formatGas(draw.totalPool)} GAS
                  </div>
                  <div className="text-xs text-gray-500">
                    {draw.ticketCount.toLocaleString()} tickets
                  </div>
                </div>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="text-center py-12">
          <div className="w-20 h-20 bg-gray-800 rounded-full flex items-center justify-center mx-auto mb-6">
            <Trophy className="w-10 h-10 text-gray-400" />
          </div>
          <h2 className="text-xl font-semibold text-white mb-2">No Results Yet</h2>
          <p className="text-gray-400">
            Draw results will appear here after the first draw is completed.
          </p>
        </div>
      )}
    </div>
  );
}
