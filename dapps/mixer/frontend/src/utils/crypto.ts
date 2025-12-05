import { u } from '@cityofzion/neon-js';

/**
 * Generates a random commitment for mixer deposit.
 * commitment = hash(secret || nullifier)
 */
export function generateCommitment() {
  // Generate random secret and nullifier
  const secret = generateRandomBytes(32);
  const nullifier = generateRandomBytes(32);

  // Create commitment hash
  const combined = secret + nullifier;
  const commitment = u.sha256(combined);

  // Create note string for user to save
  const note = `mixer-note:${secret}:${nullifier}:${commitment}`;

  return {
    commitment,
    secret,
    nullifier,
    note,
  };
}

/**
 * Parses a mixer note string.
 */
export function parseNote(note: string) {
  const parts = note.split(':');
  if (parts.length !== 4 || parts[0] !== 'mixer-note') {
    throw new Error('Invalid note format');
  }

  return {
    secret: parts[1],
    nullifier: parts[2],
    commitment: parts[3],
  };
}

/**
 * Generates a proof for withdrawal.
 * In production, this would generate a ZK proof.
 */
export function generateProof(secret: string, nullifier: string, commitment: string): string {
  // Simplified proof - in production use ZK-SNARKs
  const proofData = u.sha256(secret + nullifier + commitment);
  return proofData;
}

/**
 * Generates random bytes as hex string.
 */
function generateRandomBytes(length: number): string {
  const bytes = new Uint8Array(length);
  crypto.getRandomValues(bytes);
  return Array.from(bytes)
    .map(b => b.toString(16).padStart(2, '0'))
    .join('');
}

/**
 * Downloads note as a text file.
 */
export function downloadNote(note: string, filename: string) {
  const blob = new Blob([note], { type: 'text/plain' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}
