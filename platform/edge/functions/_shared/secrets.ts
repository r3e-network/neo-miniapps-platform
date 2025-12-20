import { mustGetEnv } from "./env.ts";

const textEncoder = new TextEncoder();
const textDecoder = new TextDecoder();

let cachedKey: CryptoKey | null | undefined;

function decodeHex(hex: string): Uint8Array {
  const trimmed = hex.trim();
  if (trimmed.length % 2 !== 0) throw new Error("invalid hex length");
  const out = new Uint8Array(trimmed.length / 2);
  for (let i = 0; i < out.length; i++) {
    const byte = Number.parseInt(trimmed.slice(i * 2, i * 2 + 2), 16);
    if (Number.isNaN(byte)) throw new Error("invalid hex");
    out[i] = byte;
  }
  return out;
}

function base64ToBytes(b64: string): Uint8Array {
  let s = b64.trim();
  s = s.replace(/-/g, "+").replace(/_/g, "/");
  while (s.length % 4 !== 0) s += "=";
  const bin = atob(s);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
}

function bytesToBase64(bytes: Uint8Array): string {
  let bin = "";
  for (let i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i]);
  return btoa(bin);
}

function normalizeMasterKey(raw: string): Uint8Array {
  const trimmed = raw.trim().replace(/^0x/i, "");
  if (!trimmed) throw new Error("SECRETS_MASTER_KEY is required");

  // Prefer hex when it looks like hex.
  if (/^[0-9a-fA-F]+$/.test(trimmed)) {
    const decoded = decodeHex(trimmed);
    if (decoded.length === 32) return decoded;
  }

  // Backward-compatible: allow 32-char plaintext keys for dev.
  if (trimmed.length === 32) return textEncoder.encode(trimmed);

  throw new Error("SECRETS_MASTER_KEY must be 32 bytes (or 64 hex chars)");
}

async function getAESKey(): Promise<CryptoKey> {
  if (cachedKey !== undefined) {
    if (cachedKey === null) throw new Error("SECRETS_MASTER_KEY not configured");
    return cachedKey;
  }

  const raw = mustGetEnv("SECRETS_MASTER_KEY");
  const keyBytes = normalizeMasterKey(raw);
  const keyData = keyBytes.buffer as ArrayBuffer;

  cachedKey = await crypto.subtle.importKey("raw", keyData, "AES-GCM", false, ["encrypt", "decrypt"]);
  return cachedKey;
}

export async function encryptSecretValue(plaintext: string): Promise<string> {
  const key = await getAESKey();
  const nonce = crypto.getRandomValues(new Uint8Array(12));
  const data = textEncoder.encode(plaintext);

  const encrypted = new Uint8Array(
    await crypto.subtle.encrypt({ name: "AES-GCM", iv: nonce }, key, data),
  );

  const out = new Uint8Array(nonce.length + encrypted.length);
  out.set(nonce, 0);
  out.set(encrypted, nonce.length);
  return bytesToBase64(out);
}

export async function decryptSecretValue(ciphertextBase64: string): Promise<string> {
  const key = await getAESKey();
  const raw = base64ToBytes(ciphertextBase64);
  if (raw.length < 13) throw new Error("ciphertext too short");
  const nonce = raw.slice(0, 12);
  const encrypted = raw.slice(12);

  const decrypted = new Uint8Array(
    await crypto.subtle.decrypt({ name: "AES-GCM", iv: nonce }, key, encrypted),
  );
  return textDecoder.decode(decrypted);
}

export function encodeBytesToBase64(bytes: Uint8Array): string {
  return bytesToBase64(bytes);
}

export function decodeBase64ToBytes(b64: string): Uint8Array {
  return base64ToBytes(b64);
}
