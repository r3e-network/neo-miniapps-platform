import { useState, useEffect } from 'react';
import { Shuffle, Trash2, Sparkles } from 'lucide-react';
import { generateRandomNumbers } from '../hooks/useLottery';

interface NumberPickerProps {
  onNumbersSelected: (mainNumbers: number[], megaNumber: number) => void;
  disabled?: boolean;
}

export function NumberPicker({ onNumbersSelected, disabled }: NumberPickerProps) {
  const [mainNumbers, setMainNumbers] = useState<number[]>([]);
  const [megaNumber, setMegaNumber] = useState<number | null>(null);

  useEffect(() => {
    if (mainNumbers.length === 5 && megaNumber !== null) {
      onNumbersSelected(mainNumbers, megaNumber);
    }
  }, [mainNumbers, megaNumber, onNumbersSelected]);

  const toggleMainNumber = (num: number) => {
    if (disabled) return;

    if (mainNumbers.includes(num)) {
      setMainNumbers(mainNumbers.filter((n) => n !== num));
    } else if (mainNumbers.length < 5) {
      setMainNumbers([...mainNumbers, num].sort((a, b) => a - b));
    }
  };

  const selectMegaNumber = (num: number) => {
    if (disabled) return;
    setMegaNumber(megaNumber === num ? null : num);
  };

  const handleQuickPick = () => {
    if (disabled) return;
    const { main, mega } = generateRandomNumbers();
    setMainNumbers(main);
    setMegaNumber(mega);
  };

  const handleClear = () => {
    if (disabled) return;
    setMainNumbers([]);
    setMegaNumber(null);
  };

  const isComplete = mainNumbers.length === 5 && megaNumber !== null;

  return (
    <div className="space-y-6">
      {/* Selected Numbers Display */}
      <div className="glass rounded-2xl p-6">
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 mb-6">
          <div>
            <h3 className="text-lg font-bold text-white">Your Numbers</h3>
            <p className="text-sm text-gray-400 mt-1">
              {isComplete ? 'Ready to play!' : `Select ${5 - mainNumbers.length} more numbers${megaNumber === null ? ' and Mega Ball' : ''}`}
            </p>
          </div>
          <div className="flex gap-2">
            <button
              onClick={handleQuickPick}
              disabled={disabled}
              className="btn-secondary flex items-center gap-2 px-4 py-2 text-sm"
            >
              <Shuffle className="w-4 h-4" />
              Quick Pick
            </button>
            <button
              onClick={handleClear}
              disabled={disabled}
              className="flex items-center gap-2 bg-white/5 hover:bg-white/10 border border-white/10 disabled:opacity-50 text-white px-4 py-2 rounded-xl text-sm transition-all"
            >
              <Trash2 className="w-4 h-4" />
              Clear
            </button>
          </div>
        </div>

        {/* Selected Numbers Display */}
        <div className="flex items-center justify-center gap-3 flex-wrap py-4">
          {/* Main Numbers */}
          {[0, 1, 2, 3, 4].map((index) => (
            <div
              key={index}
              className={`lottery-ball ${mainNumbers[index] ? 'selected' : ''} ${!mainNumbers[index] ? 'opacity-40' : ''}`}
            >
              {mainNumbers[index] || '?'}
            </div>
          ))}

          {/* Separator */}
          <div className="w-px h-14 bg-gradient-to-b from-transparent via-gray-500 to-transparent mx-3" />

          {/* Mega Number */}
          <div
            className={`lottery-ball mega ${megaNumber ? 'selected' : ''} ${!megaNumber ? 'opacity-40' : ''}`}
          >
            {megaNumber || '?'}
          </div>
        </div>

        {/* Completion Status */}
        {isComplete && (
          <div className="flex items-center justify-center gap-2 mt-4 py-3 bg-green-500/10 border border-green-500/30 rounded-xl">
            <Sparkles className="w-5 h-5 text-green-400" />
            <span className="text-green-400 font-medium">Numbers selected! Ready to buy ticket.</span>
          </div>
        )}
      </div>

      {/* Main Numbers Grid */}
      <div className="glass rounded-2xl p-6">
        <div className="flex items-center justify-between mb-4">
          <h4 className="text-sm font-semibold text-white">
            Pick 5 Numbers
          </h4>
          <div className="flex items-center gap-2">
            <span className="text-xs text-gray-400">1-70</span>
            <div className="flex items-center gap-1 bg-yellow-500/20 px-2 py-1 rounded-full">
              <span className="text-xs font-bold text-yellow-400">{mainNumbers.length}</span>
              <span className="text-xs text-yellow-400/70">/5</span>
            </div>
          </div>
        </div>
        <div className="number-grid">
          {Array.from({ length: 70 }, (_, i) => i + 1).map((num) => {
            const isSelected = mainNumbers.includes(num);
            const isDisabled = disabled || (mainNumbers.length >= 5 && !isSelected);
            return (
              <button
                key={num}
                onClick={() => toggleMainNumber(num)}
                disabled={isDisabled}
                className={`number-cell ${isSelected ? 'selected' : ''} ${isDisabled ? 'disabled' : ''}`}
              >
                {num}
              </button>
            );
          })}
        </div>
      </div>

      {/* Mega Number Grid */}
      <div className="glass rounded-2xl p-6">
        <div className="flex items-center justify-between mb-4">
          <h4 className="text-sm font-semibold text-white">
            Pick Mega Ball
          </h4>
          <div className="flex items-center gap-2">
            <span className="text-xs text-gray-400">1-25</span>
            <div className="flex items-center gap-1 bg-red-500/20 px-2 py-1 rounded-full">
              <span className="text-xs font-bold text-red-400">{megaNumber ? '1' : '0'}</span>
              <span className="text-xs text-red-400/70">/1</span>
            </div>
          </div>
        </div>
        <div className="grid grid-cols-5 sm:grid-cols-10 md:grid-cols-13 gap-2">
          {Array.from({ length: 25 }, (_, i) => i + 1).map((num) => {
            const isSelected = megaNumber === num;
            return (
              <button
                key={num}
                onClick={() => selectMegaNumber(num)}
                disabled={disabled}
                className={`number-cell mega ${isSelected ? 'selected' : ''} ${disabled ? 'disabled' : ''}`}
              >
                {num}
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}
