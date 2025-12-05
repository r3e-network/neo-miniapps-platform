/**
 * Formats a number as GAS with proper decimals.
 */
export function formatGAS(value: number | string, decimals: number = 4): string {
  const num = typeof value === 'string' ? parseFloat(value) : value;
  if (isNaN(num)) return '0';
  return num.toLocaleString('en-US', {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  });
}

/**
 * Formats a large number with K, M, B suffixes.
 */
export function formatCompact(value: number): string {
  if (value >= 1_000_000_000) {
    return (value / 1_000_000_000).toFixed(1) + 'B';
  }
  if (value >= 1_000_000) {
    return (value / 1_000_000).toFixed(1) + 'M';
  }
  if (value >= 1_000) {
    return (value / 1_000).toFixed(1) + 'K';
  }
  return value.toString();
}

/**
 * Shortens a Neo N3 address for display.
 */
export function shortenAddress(address: string, chars: number = 4): string {
  if (!address) return '';
  return `${address.slice(0, chars + 2)}...${address.slice(-chars)}`;
}

/**
 * Formats a timestamp as a readable date string.
 */
export function formatDate(timestamp: number): string {
  return new Date(timestamp).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

/**
 * Formats time remaining until a date.
 */
export function formatTimeRemaining(targetTime: number): string {
  const now = Date.now();
  const diff = targetTime - now;

  if (diff <= 0) return 'Ended';

  const days = Math.floor(diff / (1000 * 60 * 60 * 24));
  const hours = Math.floor((diff % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));
  const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60));

  if (days > 0) return `${days}d ${hours}h`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
}

/**
 * Generates an array of random lottery numbers.
 */
export function generateRandomNumbers(count: number, max: number): number[] {
  const numbers = new Set<number>();
  while (numbers.size < count) {
    numbers.add(Math.floor(Math.random() * max) + 1);
  }
  return Array.from(numbers).sort((a, b) => a - b);
}

/**
 * Validates lottery number selection.
 */
export function validateNumbers(numbers: number[], count: number, max: number): boolean {
  if (numbers.length !== count) return false;
  if (new Set(numbers).size !== count) return false;
  return numbers.every(n => n >= 1 && n <= max);
}
