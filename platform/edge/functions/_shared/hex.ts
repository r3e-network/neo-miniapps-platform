export function normalizeHexBytes(value: string, expectedBytes: number, label: string): string {
  let s = String(value ?? "").trim();
  s = s.replace(/^0x/i, "");
  if (!s) throw new Error(`${label} required`);
  if (!/^[0-9a-fA-F]+$/.test(s)) throw new Error(`${label} must be hex`);
  if (s.length !== expectedBytes * 2) throw new Error(`${label} must be ${expectedBytes} bytes`);
  return s.toLowerCase();
}

