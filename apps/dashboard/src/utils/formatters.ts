const amountFormatter = new Intl.NumberFormat(undefined, { maximumFractionDigits: 3 });
const timeFormatter = new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" });

export function formatAmount(value: number | undefined): string {
  if (typeof value !== "number" || Number.isNaN(value)) {
    return "0";
  }
  return amountFormatter.format(value);
}

export function formatTimestamp(value?: string): string {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return timeFormatter.format(date);
}

export function formatDuration(ms?: number): string {
  if (typeof ms !== "number" || !Number.isFinite(ms) || ms < 0) {
    return "n/a";
  }
  const seconds = Math.floor(ms / 1000);
  const minutes = Math.floor(seconds / 60);
  if (minutes > 0) {
    const rem = seconds % 60;
    return `${minutes}m${rem > 0 ? ` ${rem}s` : ""}`;
  }
  return `${seconds}s`;
}

export function formatSnippet(value: string, limit = 32) {
  if (!value) {
    return "";
  }
  return value.length > limit ? `${value.slice(0, limit)}…` : value;
}
