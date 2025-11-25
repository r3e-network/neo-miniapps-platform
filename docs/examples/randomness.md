# Randomness Services Quickstart

Generate cryptographically secure random numbers using the Random and VRF services.

## Overview

The Service Layer provides two randomness services:

| Service | Use Case | Verification |
|---------|----------|--------------|
| **Random** | General purpose randomness | ED25519 signature |
| **VRF** | Provably fair randomness | Cryptographic proof |

## Prerequisites

```bash
export TOKEN=dev-token
export TENANT=tenant-a
export API=http://localhost:8080
export ACCOUNT_ID=<your-account-id>
```

---

# Random Service

## Quick Start

### Generate Random Bytes

```bash
# Generate 32 random bytes
curl -s -X POST $API/accounts/$ACCOUNT_ID/random \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"length": 32, "request_id": "lottery-001"}'
```

**Response**:
```json
{
  "request_id": "lottery-001",
  "random_bytes": "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=",
  "signature": "ed25519-signature-base64",
  "public_key": "signer-public-key-base64",
  "timestamp": "2025-01-15T10:00:00Z"
}
```

### List Random History

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  "$API/accounts/$ACCOUNT_ID/random/requests?limit=10" | jq
```

## CLI Usage

```bash
# Generate random bytes
slctl random generate --account $ACCOUNT_ID --length 64

# Generate with custom request ID
slctl random generate --account $ACCOUNT_ID --length 32 --request-id "game-round-42"

# List history
slctl random list --account $ACCOUNT_ID --limit 20
```

## Use Cases

### Lottery Numbers

```bash
# Generate 6 random bytes for lottery
RESULT=$(curl -s -X POST $API/accounts/$ACCOUNT_ID/random \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"length": 6, "request_id": "lottery-2025-01-15"}')

# Parse bytes into numbers 1-49
echo $RESULT | jq -r '.random_bytes' | base64 -d | xxd -p | \
  fold -w2 | while read byte; do
    echo $(( (16#$byte % 49) + 1 ))
  done | sort -n | uniq | head -6
```

### Session Token Generation

```bash
# Generate 64-byte session token
curl -s -X POST $API/accounts/$ACCOUNT_ID/random \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"length": 64, "request_id": "session-'"$(date +%s)"'"}'
```

### Shuffle Algorithm Seed

```javascript
// In Devpack function
export default function(params, secrets) {
  const rand = Devpack.random.generate({
    length: 32,
    requestId: `shuffle-${params.gameId}`
  });

  return Devpack.respond.success({
    seed: rand.randomBytes,
    signature: rand.signature,
    publicKey: rand.publicKey
  });
}
```

---

# VRF Service

## Overview

VRF (Verifiable Random Function) provides randomness with cryptographic proofs, ensuring:
- **Unpredictability**: Output cannot be predicted before computation
- **Verifiability**: Anyone can verify the output was computed correctly
- **Uniqueness**: Each input produces exactly one output

## Quick Start

### 1. Create VRF Key

```bash
KEY_ID=$(curl -s -X POST $API/accounts/$ACCOUNT_ID/vrf/keys \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "game-vrf-key",
    "algorithm": "secp256k1"
  }' | jq -r .ID)

echo "VRF Key ID: $KEY_ID"
```

### 2. Submit VRF Request

```bash
REQ_ID=$(curl -s -X POST $API/accounts/$ACCOUNT_ID/vrf/requests \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "key_id": "'"$KEY_ID"'",
    "seed": "game-round-42-block-12345",
    "callback_url": "https://app.example.com/vrf-result"
  }' | jq -r .ID)

echo "VRF Request ID: $REQ_ID"
```

### 3. Check Request Status

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  $API/accounts/$ACCOUNT_ID/vrf/requests/$REQ_ID | jq
```

**Response**:
```json
{
  "ID": "req-uuid",
  "KeyID": "key-uuid",
  "Seed": "game-round-42-block-12345",
  "Status": "fulfilled",
  "Output": "vrf-output-bytes",
  "Proof": "vrf-proof-bytes",
  "CallbackURL": "https://app.example.com/vrf-result",
  "CreatedAt": "2025-01-15T10:00:00Z",
  "FulfilledAt": "2025-01-15T10:00:01Z"
}
```

## CLI Usage

```bash
# Create VRF key
slctl vrf keys create --account $ACCOUNT_ID \
  --name "lottery-key" \
  --algorithm secp256k1

# List keys
slctl vrf keys list --account $ACCOUNT_ID

# Submit request
slctl vrf requests create --account $ACCOUNT_ID \
  --key $KEY_ID \
  --seed "lottery-round-123"

# List requests
slctl vrf requests list --account $ACCOUNT_ID --limit 10
```

## VRF vs Random Comparison

| Feature | Random | VRF |
|---------|--------|-----|
| **Speed** | Instant | Requires computation |
| **Verification** | Signature check | Full cryptographic proof |
| **Use Case** | General randomness | Gaming, lotteries, NFTs |
| **Predictability** | Unpredictable | Unpredictable + provable |
| **Key Management** | Server-managed | User-managed keys |

## VRF Use Cases

### Provably Fair Gaming

```bash
# Create game-specific VRF key
KEY_ID=$(curl -s -X POST $API/accounts/$ACCOUNT_ID/vrf/keys \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "poker-game", "algorithm": "secp256k1"}' | jq -r .ID)

# Generate VRF for card shuffle
# Seed includes block hash for additional randomness
curl -s -X POST $API/accounts/$ACCOUNT_ID/vrf/requests \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "key_id": "'"$KEY_ID"'",
    "seed": "game-123-block-0xabc123-round-1",
    "callback_url": "https://game.example.com/shuffle-callback"
  }'
```

### NFT Trait Generation

```javascript
// Devpack function for NFT minting
export default async function(params, secrets) {
  // Request VRF for trait generation
  const vrfResult = Devpack.vrf.request({
    keyId: params.vrfKeyId,
    seed: `nft-${params.tokenId}-${params.blockHash}`
  });

  // Use VRF output to determine traits
  const traits = generateTraits(vrfResult.output);

  return Devpack.respond.success({
    tokenId: params.tokenId,
    traits: traits,
    vrfProof: vrfResult.proof
  });
}
```

### Lottery Draw

```bash
# Weekly lottery draw with VRF
LOTTERY_SEED="lottery-2025-week-03-block-$(curl -s $NEO_RPC -d '{"jsonrpc":"2.0","method":"getblockcount","params":[],"id":1}' | jq -r .result)"

curl -s -X POST $API/accounts/$ACCOUNT_ID/vrf/requests \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "key_id": "'"$KEY_ID"'",
    "seed": "'"$LOTTERY_SEED"'",
    "callback_url": "https://lottery.example.com/draw-result"
  }'
```

## Verification

### Verify VRF Proof

```javascript
// Client-side verification
const crypto = require('crypto');

function verifyVRF(publicKey, seed, output, proof) {
  // Reconstruct expected output from proof
  // Implementation depends on VRF algorithm
  const verified = vrfVerify(publicKey, seed, output, proof);
  return verified;
}

// Example usage
const result = await fetch(`${API}/accounts/${accountId}/vrf/requests/${reqId}`);
const { Output, Proof, Seed } = await result.json();

const key = await fetch(`${API}/accounts/${accountId}/vrf/keys/${keyId}`);
const { PublicKey } = await key.json();

const isValid = verifyVRF(PublicKey, Seed, Output, Proof);
console.log(`VRF verification: ${isValid ? 'PASS' : 'FAIL'}`);
```

## Best Practices

### Random Service

1. **Use Unique Request IDs**: Include timestamp or unique identifier
2. **Verify Signatures**: Always verify ED25519 signatures client-side
3. **Appropriate Length**: Use minimum bytes needed (typically 32)

### VRF Service

1. **Include Block Hash in Seed**: Prevents prediction before block finality
2. **Store Proofs**: Keep VRF proofs for audit/dispute resolution
3. **Key Rotation**: Rotate VRF keys periodically
4. **Verify On-Chain**: For critical applications, verify VRF on-chain

## Error Handling

| HTTP Status | Error | Resolution |
|-------------|-------|------------|
| 400 | "length must be positive" | Specify length > 0 |
| 400 | "seed is required" | Provide VRF seed |
| 404 | "key not found" | Check VRF key ID |
| 500 | "vrf computation failed" | Retry or contact support |

## Related Documentation

- [Service Catalog](../service-catalog.md)
- [Functions Service](../service-catalog.md#2-functions-service)
- [Security Hardening](../security-hardening.md)
