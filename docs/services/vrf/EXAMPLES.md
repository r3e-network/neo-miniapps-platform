# VRF Service Usage Examples

## Table of Contents

1. [Basic Random Number Generation](#basic-random-number-generation)
2. [Smart Contract Integration](#smart-contract-integration)
3. [Lottery System](#lottery-system)
4. [NFT Trait Generation](#nft-trait-generation)
5. [Validator Selection](#validator-selection)
6. [Proof Verification](#proof-verification)

---

## Basic Random Number Generation

### Simple Random Number Request

```bash
curl -X POST http://localhost:8080/api/vrf/generate \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "account_id": "user123",
    "seed": "0x1234567890abcdef"
  }'
```

**Response**:
```json
{
  "success": true,
  "data": {
    "randomness": "0x9876543210fedcba1234567890abcdef9876543210fedcba1234567890abcdef",
    "proof": "0xabcdef123456789...",
    "input": "0x1234567890abcdef",
    "request_id": "vrf-1701234567890"
  }
}
```

### JavaScript/TypeScript

```typescript
import { VRFClient } from '@r3e/service-layer-sdk';

const client = new VRFClient({
  baseURL: 'http://localhost:8080/api',
  apiKey: 'your-api-key'
});

async function generateRandomNumber() {
  const output = await client.generateRandomness({
    account_id: 'user123',
    seed: '0x' + Buffer.from('my_seed_12345').toString('hex')
  });

  console.log('Randomness:', output.randomness);
  console.log('Proof:', output.proof);

  // Verify the randomness
  const valid = await client.verifyRandomness(output);
  console.log('Valid:', valid);

  return output;
}

generateRandomNumber();
```

### Go

```go
package main

import (
    "context"
    "encoding/hex"
    "fmt"

    "github.com/R3E-Network/service_layer/sdk/go/client"
)

func main() {
    client := client.NewVRFClient("http://localhost:8080/api", "your-api-key")

    // Generate randomness
    seed := []byte("my_seed_12345")
    output, err := client.GenerateRandomness(context.Background(), &client.GenerateRandomnessRequest{
        AccountID: "user123",
        Seed:      seed,
    })
    if err != nil {
        panic(err)
    }

    fmt.Printf("Randomness: %s\n", hex.EncodeToString(output.Randomness))
    fmt.Printf("Proof: %s\n", hex.EncodeToString(output.Proof))

    // Verify randomness
    valid, err := client.VerifyRandomness(context.Background(), output)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Valid: %v\n", valid)
}
```

---

## Smart Contract Integration

### Neo N3 Lottery Contract

```csharp
using Neo;
using Neo.SmartContract.Framework;
using Neo.SmartContract.Framework.Services;
using System.Numerics;

public class LotteryContract : SmartContract
{
    private static readonly UInt160 VRFService = "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq".ToScriptHash();
    private static readonly UInt160 GatewayContract = "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq".ToScriptHash();

    // Start lottery draw
    public static void DrawLottery()
    {
        // Ensure only admin can draw
        if (!Runtime.CheckWitness(GetAdmin()))
            throw new Exception("Only admin can draw");

        // Generate seed from block hash + timestamp
        ByteString seed = Runtime.GetScriptContainer().Hash + Runtime.Time;

        // Request randomness via Gateway
        var payload = StdLib.Serialize(new VRFRequest {
            Seed = seed,
            BlockHash = Ledger.CurrentHash,
            BlockNumber = (ulong)Ledger.CurrentIndex,
            NumWords = 1
        });

        var requestId = (ByteString)Contract.Call(
            GatewayContract,
            "request",
            CallFlags.All,
            new object[] { "vrf", payload, Runtime.ExecutingScriptHash, "onRandomnessReceived" }
        );

        Storage.Put(Storage.CurrentContext, "pending_draw", requestId);
        Runtime.Log($"Lottery draw requested: {requestId}");
    }

    // Callback from VRF service
    public static void OnRandomnessReceived(ByteString requestId, bool success, ByteString data, BigInteger statusCode)
    {
        // Verify caller is Gateway
        if (Runtime.CallingScriptHash != GatewayContract)
            throw new Exception("Unauthorized callback");

        if (!success)
        {
            Runtime.Log($"VRF request failed: {statusCode}");
            return;
        }

        // Deserialize VRF response
        var response = (VRFResponse)StdLib.Deserialize(data);

        // Get participant count
        BigInteger participantCount = GetParticipantCount();
        if (participantCount == 0)
        {
            Runtime.Log("No participants");
            return;
        }

        // Convert randomness to winner index
        BigInteger random = (BigInteger)response.Randomness;
        BigInteger winnerIndex = random % participantCount;

        // Select winner
        UInt160 winner = GetParticipant(winnerIndex);
        Storage.Put(Storage.CurrentContext, "winner", winner);
        Storage.Put(Storage.CurrentContext, "winner_proof", response.Proof);

        Runtime.Log($"Winner selected: {winner} (index: {winnerIndex})");

        // Distribute prize
        DistributePrize(winner);
    }

    private static BigInteger GetParticipantCount()
    {
        var stored = Storage.Get(Storage.CurrentContext, "participant_count");
        return stored != null ? (BigInteger)stored : 0;
    }

    private static UInt160 GetParticipant(BigInteger index)
    {
        return (UInt160)Storage.Get(Storage.CurrentContext, "participant_" + index);
    }

    private static void DistributePrize(UInt160 winner)
    {
        BigInteger prizeAmount = GetPrizePool();
        UInt160 gasToken = "0xd2a4cff31913016155e38e474a2c06d08be276cf".ToScriptHash();

        Contract.Call(gasToken, "transfer", CallFlags.All,
            new object[] { Runtime.ExecutingScriptHash, winner, prizeAmount, null });

        Runtime.Log($"Prize distributed: {prizeAmount} GAS to {winner}");
    }

    private static BigInteger GetPrizePool()
    {
        var stored = Storage.Get(Storage.CurrentContext, "prize_pool");
        return stored != null ? (BigInteger)stored : 0;
    }

    private static UInt160 GetAdmin()
    {
        return (UInt160)Storage.Get(Storage.CurrentContext, "admin");
    }
}

public class VRFRequest
{
    public ByteString Seed;
    public ByteString BlockHash;
    public ulong BlockNumber;
    public int NumWords;
}

public class VRFResponse
{
    public ByteString Randomness;
    public ByteString Proof;
    public ByteString PublicKey;
}
```

---

## Lottery System

### Complete Lottery Implementation

```typescript
import { VRFClient } from '@r3e/service-layer-sdk';

class LotterySystem {
  private vrfClient: VRFClient;
  private participants: string[] = [];

  constructor(apiKey: string) {
    this.vrfClient = new VRFClient({
      baseURL: 'http://localhost:8080/api',
      apiKey
    });
  }

  // Add participant
  addParticipant(address: string) {
    this.participants.push(address);
    console.log(`Participant added: ${address}`);
  }

  // Draw winner
  async drawWinner(): Promise<string> {
    if (this.participants.length === 0) {
      throw new Error('No participants');
    }

    // Generate seed from timestamp + participant count
    const seed = Buffer.from(
      `lottery_${Date.now()}_${this.participants.length}`
    ).toString('hex');

    // Request randomness
    const output = await this.vrfClient.generateRandomness({
      account_id: 'lottery_system',
      seed: '0x' + seed
    });

    // Verify randomness
    const valid = await this.vrfClient.verifyRandomness(output);
    if (!valid) {
      throw new Error('Invalid VRF proof');
    }

    // Convert randomness to winner index
    const randomBigInt = BigInt('0x' + output.randomness);
    const winnerIndex = Number(randomBigInt % BigInt(this.participants.length));

    const winner = this.participants[winnerIndex];

    console.log('Lottery Results:');
    console.log(`  Total Participants: ${this.participants.length}`);
    console.log(`  Randomness: ${output.randomness}`);
    console.log(`  Winner Index: ${winnerIndex}`);
    console.log(`  Winner: ${winner}`);
    console.log(`  Proof: ${output.proof}`);

    return winner;
  }

  // Verify winner selection
  async verifyWinner(winner: string, randomness: string, proof: string): Promise<boolean> {
    // Verify VRF proof
    const valid = await this.vrfClient.verifyRandomness({
      randomness,
      proof,
      input: '0x' + Buffer.from('lottery_seed').toString('hex')
    });

    if (!valid) {
      return false;
    }

    // Verify winner index calculation
    const randomBigInt = BigInt('0x' + randomness);
    const winnerIndex = Number(randomBigInt % BigInt(this.participants.length));
    const expectedWinner = this.participants[winnerIndex];

    return winner === expectedWinner;
  }
}

// Usage
const lottery = new LotterySystem('your-api-key');

// Add participants
lottery.addParticipant('NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq');
lottery.addParticipant('NZ8zBpRh9zLaQ5DdUz4qUb9bf5MZvELyq6');
lottery.addParticipant('NhGomBpYnKXArr55nHRQ5rzy79TwKVXZbr');

// Draw winner
const winner = await lottery.drawWinner();
console.log(`Winner: ${winner}`);
```

---

## NFT Trait Generation

### Random Trait Assignment

```typescript
interface NFTTraits {
  background: string;
  body: string;
  eyes: string;
  mouth: string;
  accessory: string;
}

class NFTMinter {
  private vrfClient: VRFClient;

  private traitOptions = {
    background: ['Blue', 'Red', 'Green', 'Purple', 'Gold'],
    body: ['Human', 'Alien', 'Robot', 'Zombie', 'Ape'],
    eyes: ['Normal', 'Laser', 'Sunglasses', 'Closed', 'Glowing'],
    mouth: ['Smile', 'Frown', 'Surprised', 'Neutral', 'Fangs'],
    accessory: ['None', 'Hat', 'Crown', 'Earring', 'Necklace']
  };

  constructor(apiKey: string) {
    this.vrfClient = new VRFClient({
      baseURL: 'http://localhost:8080/api',
      apiKey
    });
  }

  async mintNFT(owner: string, tokenId: number): Promise<NFTTraits> {
    // Generate seed from owner + tokenId
    const seed = Buffer.from(`${owner}_${tokenId}`).toString('hex');

    // Request randomness (5 words for 5 traits)
    const output = await this.vrfClient.generateRandomness({
      account_id: owner,
      seed: '0x' + seed
    });

    // Parse randomness into trait indices
    const randomBytes = Buffer.from(output.randomness.slice(2), 'hex');
    const traits: NFTTraits = {
      background: this.selectTrait('background', randomBytes.slice(0, 8)),
      body: this.selectTrait('body', randomBytes.slice(8, 16)),
      eyes: this.selectTrait('eyes', randomBytes.slice(16, 24)),
      mouth: this.selectTrait('mouth', randomBytes.slice(24, 32)),
      accessory: this.selectTrait('accessory', randomBytes.slice(32, 40))
    };

    console.log(`NFT #${tokenId} Traits:`, traits);
    console.log(`Proof: ${output.proof}`);

    return traits;
  }

  private selectTrait(category: keyof typeof this.traitOptions, randomBytes: Buffer): string {
    const options = this.traitOptions[category];
    const randomValue = BigInt('0x' + randomBytes.toString('hex'));
    const index = Number(randomValue % BigInt(options.length));
    return options[index];
  }
}

// Usage
const minter = new NFTMinter('your-api-key');

// Mint NFT with random traits
const traits = await minter.mintNFT('NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq', 1);
console.log('Generated Traits:', traits);
```

---

## Validator Selection

### Random Validator Selection

```go
package main

import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "math/big"

    "github.com/R3E-Network/service_layer/sdk/go/client"
)

type ValidatorPool struct {
    vrfClient  *client.VRFClient
    validators []string
}

func NewValidatorPool(apiKey string, validators []string) *ValidatorPool {
    return &ValidatorPool{
        vrfClient:  client.NewVRFClient("http://localhost:8080/api", apiKey),
        validators: validators,
    }
}

func (vp *ValidatorPool) SelectValidators(ctx context.Context, count int) ([]string, error) {
    if count > len(vp.validators) {
        return nil, fmt.Errorf("requested count exceeds validator pool size")
    }

    // Generate seed from current timestamp + validator count
    seed := sha256.Sum256([]byte(fmt.Sprintf("validator_selection_%d", len(vp.validators))))

    // Request randomness
    output, err := vp.vrfClient.GenerateRandomness(ctx, &client.GenerateRandomnessRequest{
        AccountID: "validator_pool",
        Seed:      seed[:],
    })
    if err != nil {
        return nil, err
    }

    // Verify randomness
    valid, err := vp.vrfClient.VerifyRandomness(ctx, output)
    if err != nil {
        return nil, err
    }
    if !valid {
        return nil, fmt.Errorf("invalid VRF proof")
    }

    // Select validators using Fisher-Yates shuffle with VRF randomness
    selected := vp.shuffleAndSelect(output.Randomness, count)

    fmt.Printf("Selected %d validators:\n", len(selected))
    for i, validator := range selected {
        fmt.Printf("  %d. %s\n", i+1, validator)
    }
    fmt.Printf("Proof: %s\n", hex.EncodeToString(output.Proof))

    return selected, nil
}

func (vp *ValidatorPool) shuffleAndSelect(randomness []byte, count int) []string {
    // Create a copy of validators
    validators := make([]string, len(vp.validators))
    copy(validators, vp.validators)

    // Use randomness to shuffle
    for i := 0; i < count; i++ {
        // Get random index from randomness
        offset := i * 8
        if offset+8 > len(randomness) {
            // Extend randomness if needed
            hash := sha256.Sum256(randomness)
            randomness = append(randomness, hash[:]...)
        }

        randomBytes := randomness[offset : offset+8]
        randomBig := new(big.Int).SetBytes(randomBytes)
        poolSize := big.NewInt(int64(len(validators) - i))
        randomIndex := new(big.Int).Mod(randomBig, poolSize).Int64()

        // Swap
        swapIndex := i + int(randomIndex)
        validators[i], validators[swapIndex] = validators[swapIndex], validators[i]
    }

    return validators[:count]
}

func main() {
    validators := []string{
        "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq",
        "NZ8zBpRh9zLaQ5DdUz4qUb9bf5MZvELyq6",
        "NhGomBpYnKXArr55nHRQ5rzy79TwKVXZbr",
        "NiHURyS5cmPbNfRMvMeVyVt8jJLfYXfF3V",
        "NZcuGiwRu1QscpmCyxj5XwQBUf6sk7dJJN",
    }

    pool := NewValidatorPool("your-api-key", validators)

    // Select 3 validators
    selected, err := pool.SelectValidators(context.Background(), 3)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Selected validators: %v\n", selected)
}
```

---

## Proof Verification

### Verify VRF Proof

```python
import requests
import json

def verify_vrf_proof(randomness, proof, seed):
    """Verify VRF proof"""
    response = requests.post(
        'http://localhost:8080/api/vrf/verify',
        headers={'Content-Type': 'application/json'},
        json={
            'randomness': randomness,
            'proof': proof,
            'input': seed
        }
    )

    result = response.json()
    return result['data']['valid']

# Example usage
randomness = '0x9876543210fedcba1234567890abcdef9876543210fedcba1234567890abcdef'
proof = '0xabcdef123456789...'
seed = '0x1234567890abcdef'

valid = verify_vrf_proof(randomness, proof, seed)
print(f"Proof valid: {valid}")
```

### Off-Chain Verification

```typescript
import { VRFClient } from '@r3e/service-layer-sdk';
import * as crypto from 'crypto';

class VRFVerifier {
  private vrfClient: VRFClient;

  constructor(apiKey: string) {
    this.vrfClient = new VRFClient({
      baseURL: 'http://localhost:8080/api',
      apiKey
    });
  }

  async verifyLotteryResult(
    winner: string,
    participants: string[],
    randomness: string,
    proof: string,
    seed: string
  ): Promise<boolean> {
    // 1. Verify VRF proof
    const proofValid = await this.vrfClient.verifyRandomness({
      randomness,
      proof,
      input: seed
    });

    if (!proofValid) {
      console.error('Invalid VRF proof');
      return false;
    }

    // 2. Verify winner calculation
    const randomBigInt = BigInt(randomness);
    const winnerIndex = Number(randomBigInt % BigInt(participants.length));
    const expectedWinner = participants[winnerIndex];

    if (winner !== expectedWinner) {
      console.error('Winner does not match expected result');
      return false;
    }

    console.log('Verification successful:');
    console.log(`  Proof: Valid`);
    console.log(`  Winner Index: ${winnerIndex}`);
    console.log(`  Winner: ${winner}`);

    return true;
  }
}

// Usage
const verifier = new VRFVerifier('your-api-key');

const participants = [
  'NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq',
  'NZ8zBpRh9zLaQ5DdUz4qUb9bf5MZvELyq6',
  'NhGomBpYnKXArr55nHRQ5rzy79TwKVXZbr'
];

const valid = await verifier.verifyLotteryResult(
  'NZ8zBpRh9zLaQ5DdUz4qUb9bf5MZvELyq6',
  participants,
  '0x9876543210fedcba...',
  '0xabcdef123456...',
  '0x1234567890abcdef'
);

console.log(`Result valid: ${valid}`);
```

---

## Testing

### Unit Test Example

```typescript
import { describe, it, expect } from '@jest/globals';
import { VRFClient } from '@r3e/service-layer-sdk';

describe('VRF Service', () => {
  const client = new VRFClient({
    baseURL: 'http://localhost:8080/api',
    apiKey: 'test-api-key'
  });

  it('should generate verifiable randomness', async () => {
    const output = await client.generateRandomness({
      account_id: 'test_user',
      seed: '0x1234567890abcdef'
    });

    expect(output.randomness).toBeDefined();
    expect(output.proof).toBeDefined();
    expect(output.randomness.length).toBe(66); // 0x + 64 hex chars

    // Verify the randomness
    const valid = await client.verifyRandomness(output);
    expect(valid).toBe(true);
  });

  it('should produce deterministic output for same seed', async () => {
    const seed = '0x1234567890abcdef';

    const output1 = await client.generateRandomness({
      account_id: 'test_user',
      seed
    });

    const output2 = await client.generateRandomness({
      account_id: 'test_user',
      seed
    });

    expect(output1.randomness).toBe(output2.randomness);
    expect(output1.proof).toBe(output2.proof);
  });
});
```

---

## Additional Resources

- [VRF Service README](./README.md)
- [API Documentation](./API.md)
- [Contract Documentation](./CONTRACT.md)
- [VRF Specification (RFC 9381)](https://datatracker.ietf.org/doc/html/rfc9381)
