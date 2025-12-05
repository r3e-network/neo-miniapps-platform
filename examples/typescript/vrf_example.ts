/**
 * VRF (Verifiable Random Function) Service Example
 *
 * This example demonstrates how to generate verifiable random numbers
 * for fair selection, lotteries, and gaming applications.
 *
 * Run: npx ts-node vrf_example.ts
 */

import { createClient, SupabaseClient } from '@supabase/supabase-js';

// =============================================================================
// Configuration
// =============================================================================

const SUPABASE_URL = process.env.VITE_SUPABASE_URL || 'http://localhost:54321';
const SUPABASE_KEY = process.env.VITE_SUPABASE_ANON_KEY || '';

// =============================================================================
// Types
// =============================================================================

interface ServiceRequest {
  id?: string;
  service_type: string;
  operation: string;
  payload: Record<string, unknown>;
  status?: string;
  result?: Record<string, unknown>;
  error_message?: string;
}

interface VRFResult {
  randomness: string;
  proof: string;
  seed: string;
}

// =============================================================================
// VRF Service Client
// =============================================================================

class VRFService {
  private supabase: SupabaseClient;

  constructor(url: string, key: string) {
    this.supabase = createClient(url, key);
  }

  /**
   * Generate a verifiable random number
   */
  async generateRandom(seed: string, numValues: number = 1): Promise<VRFResult> {
    // Submit request
    const { data: request, error } = await this.supabase
      .from('service_requests')
      .insert({
        service_type: 'vrf',
        operation: 'random',
        payload: { seed, num_values: numValues },
      })
      .select()
      .single();

    if (error) throw new Error(`Submit failed: ${error.message}`);

    // Wait for completion
    const result = await this.waitForCompletion(request.id);

    if (result.status === 'failed') {
      throw new Error(result.error_message || 'VRF generation failed');
    }

    return result.result as VRFResult;
  }

  /**
   * Verify a VRF proof
   */
  async verifyProof(randomness: string, proof: string, seed: string): Promise<boolean> {
    const { data: request, error } = await this.supabase
      .from('service_requests')
      .insert({
        service_type: 'vrf',
        operation: 'verify',
        payload: { randomness, proof, seed },
      })
      .select()
      .single();

    if (error) throw new Error(`Submit failed: ${error.message}`);

    const result = await this.waitForCompletion(request.id);
    return result.result?.valid === true;
  }

  private async waitForCompletion(id: string, timeoutMs: number = 30000): Promise<ServiceRequest> {
    const startTime = Date.now();

    while (Date.now() - startTime < timeoutMs) {
      const { data } = await this.supabase
        .from('service_requests')
        .select('*')
        .eq('id', id)
        .single();

      if (data?.status === 'completed' || data?.status === 'failed') {
        return data;
      }

      await new Promise(resolve => setTimeout(resolve, 1000));
    }

    throw new Error('Timeout');
  }
}

// =============================================================================
// Utility Functions
// =============================================================================

/**
 * Convert hex randomness to a number in range [0, max)
 */
function randomToRange(randomnessHex: string, max: number): number {
  const randomBigInt = BigInt('0x' + randomnessHex);
  return Number(randomBigInt % BigInt(max));
}

/**
 * Shuffle array using VRF randomness (Fisher-Yates)
 */
function shuffleWithRandomness(array: string[], randomnessHex: string): string[] {
  const result = [...array];
  const randomBigInt = BigInt('0x' + randomnessHex);

  for (let i = result.length - 1; i > 0; i--) {
    const j = Number((randomBigInt >> BigInt(i * 8)) % BigInt(i + 1));
    [result[i], result[j]] = [result[j], result[i]];
  }

  return result;
}

// =============================================================================
// Examples
// =============================================================================

async function main() {
  console.log('=== VRF Service Example ===\n');

  if (!SUPABASE_KEY) {
    console.error('Error: VITE_SUPABASE_ANON_KEY environment variable required');
    process.exit(1);
  }

  const vrf = new VRFService(SUPABASE_URL, SUPABASE_KEY);

  // Example 1: Simple random number
  console.log('Example 1: Generate verifiable random number');
  console.log('-'.repeat(50));

  try {
    const seed = `lottery-${Date.now()}`;
    console.log(`Seed: ${seed}`);

    const result = await vrf.generateRandom(seed);

    console.log('✓ Random number generated!');
    console.log(`  Randomness: ${result.randomness.substring(0, 32)}...`);
    console.log(`  Proof: ${result.proof.substring(0, 32)}...`);
    console.log();
  } catch (error) {
    console.error('Error:', error);
  }

  // Example 2: Fair lottery winner selection
  console.log('Example 2: Fair lottery winner selection');
  console.log('-'.repeat(50));

  try {
    const participants = ['Alice', 'Bob', 'Charlie', 'Diana', 'Eve', 'Frank'];
    console.log(`Participants: ${participants.join(', ')}`);

    const seed = `lottery-winner-${Date.now()}`;
    const result = await vrf.generateRandom(seed);

    const winnerIndex = randomToRange(result.randomness, participants.length);
    const winner = participants[winnerIndex];

    console.log('✓ Winner selected!');
    console.log(`  Winner: ${winner} (index ${winnerIndex})`);
    console.log(`  Seed: ${seed}`);
    console.log(`  Proof available for verification`);
    console.log();
  } catch (error) {
    console.error('Error:', error);
  }

  // Example 3: NFT trait randomization
  console.log('Example 3: NFT trait randomization');
  console.log('-'.repeat(50));

  try {
    const traits = {
      background: ['Blue', 'Red', 'Green', 'Purple', 'Gold'],
      body: ['Robot', 'Human', 'Alien', 'Zombie'],
      accessory: ['Hat', 'Glasses', 'Earring', 'None'],
    };

    const tokenId = 12345;
    const seed = `nft-traits-${tokenId}`;
    const result = await vrf.generateRandom(seed);

    const randomBigInt = BigInt('0x' + result.randomness);

    const selectedTraits = {
      background: traits.background[Number(randomBigInt % BigInt(traits.background.length))],
      body: traits.body[Number((randomBigInt >> 8n) % BigInt(traits.body.length))],
      accessory: traits.accessory[Number((randomBigInt >> 16n) % BigInt(traits.accessory.length))],
    };

    console.log(`✓ NFT #${tokenId} traits generated!`);
    console.log(`  Background: ${selectedTraits.background}`);
    console.log(`  Body: ${selectedTraits.body}`);
    console.log(`  Accessory: ${selectedTraits.accessory}`);
    console.log();
  } catch (error) {
    console.error('Error:', error);
  }

  // Example 4: Dice roll for gaming
  console.log('Example 4: Provably fair dice roll');
  console.log('-'.repeat(50));

  try {
    const gameId = 'game-' + Date.now();
    const seed = `dice-roll-${gameId}`;
    const result = await vrf.generateRandom(seed);

    const diceValue = randomToRange(result.randomness, 6) + 1; // 1-6

    console.log(`✓ Dice rolled!`);
    console.log(`  Value: ${diceValue}`);
    console.log(`  Game ID: ${gameId}`);
    console.log(`  Verifiable: Anyone can verify with seed + proof`);
    console.log();
  } catch (error) {
    console.error('Error:', error);
  }

  console.log('=== Example Complete ===');
}

// Run
main().catch(console.error);

export { VRFService, randomToRange, shuffleWithRandomness };
