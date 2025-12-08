import { useState, useEffect } from 'react';

interface CountdownProps {
  targetTime: number;
  onComplete?: () => void;
}

interface TimeLeft {
  days: number;
  hours: number;
  minutes: number;
  seconds: number;
}

export function Countdown({ targetTime, onComplete }: CountdownProps) {
  const [timeLeft, setTimeLeft] = useState<TimeLeft>({ days: 0, hours: 0, minutes: 0, seconds: 0 });
  const [isComplete, setIsComplete] = useState(false);

  useEffect(() => {
    const calculateTimeLeft = () => {
      const now = Date.now();
      const difference = targetTime - now;

      if (difference <= 0) {
        setIsComplete(true);
        onComplete?.();
        return { days: 0, hours: 0, minutes: 0, seconds: 0 };
      }

      return {
        days: Math.floor(difference / (1000 * 60 * 60 * 24)),
        hours: Math.floor((difference / (1000 * 60 * 60)) % 24),
        minutes: Math.floor((difference / 1000 / 60) % 60),
        seconds: Math.floor((difference / 1000) % 60),
      };
    };

    setTimeLeft(calculateTimeLeft());

    const timer = setInterval(() => {
      setTimeLeft(calculateTimeLeft());
    }, 1000);

    return () => clearInterval(timer);
  }, [targetTime, onComplete]);

  if (isComplete) {
    return (
      <div className="text-center py-4">
        <div className="inline-flex items-center gap-3 px-6 py-3 rounded-2xl bg-yellow-500/20 border border-yellow-500/30">
          <span className="text-3xl animate-bounce">🎰</span>
          <span className="text-xl font-bold text-yellow-400 animate-pulse">
            Drawing in Progress...
          </span>
          <span className="text-3xl animate-bounce" style={{ animationDelay: '0.1s' }}>🎰</span>
        </div>
      </div>
    );
  }

  const segments = [
    { value: timeLeft.days, label: 'Days' },
    { value: timeLeft.hours, label: 'Hours' },
    { value: timeLeft.minutes, label: 'Mins' },
    { value: timeLeft.seconds, label: 'Secs' },
  ];

  return (
    <div className="countdown-container">
      {segments.map((segment, index) => (
        <div key={segment.label} className="flex items-center">
          <div className="countdown-segment">
            <div className="glass rounded-xl px-4 py-3 min-w-[80px]">
              <div className="countdown-value">
                {segment.value.toString().padStart(2, '0')}
              </div>
            </div>
            <div className="countdown-label mt-2">{segment.label}</div>
          </div>
          {index < segments.length - 1 && (
            <span className="countdown-separator mx-1 mb-6">:</span>
          )}
        </div>
      ))}
    </div>
  );
}
