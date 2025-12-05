# VRF Service (Verifiable Random Function)

## Overview

The VRF Service provides cryptographically secure, verifiable random number generation for smart contracts and applications on the Neo N3 blockchain. All randomness is generated within a Trusted Execution Environment (TEE) with cryptographic proofs of correctness.

## Features

### Core Capabilities
- **Verifiable Randomness**: Cryptographic proofs ensure randomness cannot be manipulated
- **TEE Protection**: All random generation occurs within secure enclave
- **Deterministic Verification**: Anyone can verify the randomness using public key
- **Blockchain Integration**: Seamless integration with Neo N3 smart contracts
- **Request Tracking**: Complete audit trail of all randomness requests
- **Proof Generation**: VRF proofs for each random output

### Security Features
- **Unpredictability**: Random values cannot be predicted before generation
- **Uniqueness**: Each seed produces unique, deterministic output
- **Non-Malleability**: Proofs cannot be forged or modified
- **Public Verifiability**: Anyone can verify randomness with public key
- **TEE Attestation**: Remote attestation proves execution in secure environment

## Architecture

### Component Structure

```
VRF Service
├── Service Layer (service.go)
│   ├── Request Management
│   ├── Randomness Generation
│   ├── Proof Verification
│   └── Contract Integration
├── Enclave Layer (enclave.go)
│   ├── VRF Key Management
│   ├── Random Generation (VRF)
│   ├── Proof Generation
│   └── Signature Creation
└── Storage Layer (store.go)
    ├── Request History
    ├── Generated Randomness
    └── Statistics
```

### VRF Algorithm

The service uses ECVRF (Elliptic Curve VRF) based on secp256r1:

1. **Key Generation**: TEE generates VRF key pair (private key never leaves enclave)
2. **Random Generation**: `VRF_prove(sk, seed) → (randomness, proof)`
3. **Verification**: `VRF_verify(pk, seed, randomness, proof) → valid/invalid`

## Service Manifest

```yaml
Service ID: vrf
Version: 1.0.0
Description: Verifiable Random Function service with TEE protection

Required Capabilities:
  - CapKeys: Key management
  - CapKeysSign: Signature generation
  - CapStorage: Data persistence

Optional Capabilities:
  - CapNetwork: Network access
  - CapAttestation: Remote attestation
  - CapDatabase: Database storage
  - CapMetrics: Performance monitoring
  - CapCache: Result caching
  - CapNeo: Neo blockchain integration
  - CapContract: Contract callbacks

Resource Limits:
  - Max Memory: 64 MB
  - Max CPU Time: 10 seconds
```

## Use Cases

### 1. Gaming & Lotteries
Generate provably fair random numbers for games, lotteries, and prize draws.

### 2. NFT Minting
Randomly assign traits, rarities, or token IDs during NFT minting.

### 3. Governance
Random selection of validators, committee members, or proposal reviewers.

### 4. Sampling
Random sampling for audits, surveys, or statistical analysis.

### 5. Cryptographic Protocols
Secure randomness for key generation, nonces, and protocol initialization.

## Request Flow

1. **Request Creation**: User/contract submits randomness request with seed
2. **TEE Generation**: VRF algorithm generates randomness + proof in enclave
3. **Storage**: Request and output stored with proof
4. **Callback**: Result delivered to requesting contract
5. **Verification**: Anyone can verify randomness using public key + proof

## Data Types

### VRF Request

```go
type VRFRequest struct {
    ID              string        // Unique request ID
    AccountID       string        // Requester account
    Status          RequestStatus // pending, fulfilled, failed
    Seed            []byte        // Input seed
    BlockHash       []byte        // Optional block hash
    BlockNumber     int64         // Block number
    Randomness      []byte        // Generated randomness
    Proof           []byte        // VRF proof
    CallbackAddress string        // Contract callback address
    FulfilledAt     time.Time     // Fulfillment timestamp
}
```

### VRF Output

```go
type VRFOutput struct {
    Randomness []byte // 32-byte random value
    Proof      []byte // VRF proof
    Input      []byte // Original seed
}
```

## Metrics

The service exposes the following metrics:

- `vrf_requests_total`: Total number of VRF requests
- `vrf_requests_fulfilled`: Total fulfilled requests
- `vrf_requests_failed`: Total failed requests
- `vrf_generation_duration_seconds`: Generation time histogram

## Security Considerations

### Trust Model

- VRF private key generated and stored in TEE
- Key never leaves secure enclave
- All randomness generation occurs in TEE
- Proofs are publicly verifiable

### Threat Mitigation

- **Prediction**: Impossible without private key
- **Manipulation**: Proofs ensure correctness
- **Replay**: Each seed produces unique output
- **Key Extraction**: TEE prevents key access

## Performance Characteristics

- **Latency**: 10-50ms per request
- **Throughput**: 100+ requests/second
- **Proof Size**: ~96 bytes
- **Randomness Size**: 32 bytes

## Integration Points

### Contract Integration

```go
type VRFContractRequest struct {
    Seed        []byte // Input seed
    BlockHash   []byte // Optional block hash
    BlockNumber uint64 // Block number
    NumWords    int    // Number of random words
}

type VRFContractResponse struct {
    Randomness []byte // Generated randomness
    Proof      []byte // VRF proof
    PublicKey  []byte // VRF public key
}
```

## Verification

### Verify Randomness

Anyone can verify VRF output:

```go
valid := service.VerifyRandomness(ctx, &VRFOutput{
    Randomness: randomness,
    Proof:      proof,
    Input:      seed,
})
```

### Get Public Key

```go
publicKey := service.GetPublicKey(ctx)
```

## Troubleshooting

### Common Issues

**Issue**: Request not fulfilled
- **Solution**: Check service status, verify seed format

**Issue**: Verification fails
- **Solution**: Ensure correct public key, check proof format

**Issue**: Slow generation
- **Solution**: Check TEE availability, monitor metrics

## Related Services

- **Oracle Service**: External randomness sources
- **Secrets Service**: Key management
- **Automation Service**: Scheduled random generation

## References

- [API Documentation](./API.md)
- [Contract Documentation](./CONTRACT.md)
- [Usage Examples](./EXAMPLES.md)
- [VRF Specification (RFC 9381)](https://datatracker.ietf.org/doc/html/rfc9381)
