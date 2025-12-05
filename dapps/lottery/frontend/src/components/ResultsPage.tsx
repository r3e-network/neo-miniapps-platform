import React from 'react';

export function ResultsPage() {
  // Mock past results
  const pastRounds = [
    { roundId: 5, winningNumbers: [7, 14, 23, 31, 42, 49], jackpot: '150.00', winners: 3, date: '2024-01-15' },
    { roundId: 4, winningNumbers: [3, 11, 25, 33, 40, 47], jackpot: '120.50', winners: 2, date: '2024-01-14' },
    { roundId: 3, winningNumbers: [5, 18, 22, 35, 41, 48], jackpot: '95.00', winners: 1, date: '2024-01-13' },
    { roundId: 2, winningNumbers: [2, 9, 19, 28, 38, 45], jackpot: '80.25', winners: 4, date: '2024-01-12' },
    { roundId: 1, winningNumbers: [1, 12, 24, 36, 44, 49], jackpot: '50.00', winners: 2, date: '2024-01-11' },
  ];

  return (
    <div className="max-w-3xl mx-auto">
      <h1 className="text-3xl font-bold text-white mb-8 text-center">Past Results</h1>

      <div className="space-y-4">
        {pastRounds.map(round => (
          <div
            key={round.roundId}
            className="bg-white/10 backdrop-blur-md rounded-xl p-6 border border-white/20"
          >
            <div className="flex justify-between items-start mb-4">
              <div>
                <p className="text-xl font-semibold text-white">Round #{round.roundId}</p>
                <p className="text-white/60 text-sm">{round.date}</p>
              </div>
              <div className="text-right">
                <p className="text-yellow-400 font-bold">{round.jackpot} GAS</p>
                <p className="text-white/60 text-sm">{round.winners} winner(s)</p>
              </div>
            </div>

            <div className="flex justify-center space-x-3">
              {round.winningNumbers.map((num, i) => (
                <div
                  key={i}
                  className="w-12 h-12 rounded-full bg-gradient-to-br from-yellow-400 to-orange-500 flex items-center justify-center text-black font-bold text-lg"
                >
                  {num}
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
