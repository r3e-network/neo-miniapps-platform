# Mixer Service

Privacy-preserving transaction mixing service using **Double-Blind HD 1/2 Multi-sig** architecture for Neo N3 blockchain.

## Architecture Overview

```
                    ┌─────────────────────────────────────────┐
                    │           Mixer Service                 │
                    ├─────────────────────────────────────────┤
                    │                                         │
    ┌───────────────┼───────────────┐   ┌────────────────────┼───────────────┐
    │               │               │   │                    │               │
    ▼               ▼               ▼   ▼                    ▼               ▼
┌─────────┐   ┌─────────┐   ┌─────────────────┐   ┌─────────────┐   ┌─────────────┐
│   TEE   │   │ Master  │   │   Chain Client  │   │   Executor  │   │    Store    │
│ Manager │   │   Key   │   │   (Neo N3 RPC)  │   │  (Background│   │ (PostgreSQL)│
│         │   │Provider │   │                 │   │   Worker)   │   │             │
└────┬────┘   └────┬────┘   └────────┬────────┘   └──────┬──────┘   └─────────────┘
     │             │                 │                   │
     │             │                 │                   │
     ▼             ▼                 ▼                   ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                         HD Key Derivation Path                                  │
│                      m/44'/888'/0'/0/{pool_index}                               │
│                                                                                 │
│   TEE Key (online)  ────┐                                                       │
│                         ├──► Neo N3 1-of-2 Multi-sig Address                    │
│   Master Key (offline) ─┘                                                       │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## Key Components

### 1. TEE Manager (`tee_manager.go`)

Manages the TEE-side of the Double-Blind architecture:

- **Sealed Root Seed**: Encrypted within TEE enclave, never exposed
- **HD Key Derivation**: Derives keys at `m/44'/888'/0'/0/{index}`
- **Transaction Signing**: Signs transactions using TEE-derived keys
- **ZK Proof Generation**: Creates privacy-preserving proofs
- **TEE Attestation**: Provides cryptographic proof of TEE execution

```go
type TEEManager interface {
    DerivePoolKeys(ctx context.Context, index uint32, masterPubKey []byte) (*PoolKeyPair, error)
    SignTransaction(ctx context.Context, hdIndex uint32, txData []byte) ([]byte, error)
    GetTEEPublicKey(ctx context.Context, hdIndex uint32) ([]byte, error)
    GenerateZKProof(ctx context.Context, req MixRequest) (string, error)
    SignAttestation(ctx context.Context, data []byte) (string, error)
}
```

### 2. Master Key Provider (`master_key_provider.go`)

Manages the offline Master key side:

- **Public-Key-Only Mode**: Production mode with only public keys
- **Extended Public Key Mode**: Can derive child public keys
- **Full-Key Mode**: Testing/recovery with private keys available

```go
type MasterKeyProvider interface {
    GetMasterPublicKey(ctx context.Context, hdIndex uint32) ([]byte, error)
    VerifyMasterSignature(ctx context.Context, hdIndex uint32, data, signature []byte) (bool, error)
}
```

### 3. Chain Client (`chain_client.go`)

Neo N3 blockchain integration:

- **RPC Communication**: JSON-RPC calls to Neo N3 nodes
- **Balance Queries**: NEP-17 token balance lookups
- **Transaction Building**: Constructs unsigned transactions
- **Transaction Submission**: Sends signed transactions
- **Proof Submission**: Submits ZK proofs to mixer contract

```go
type ChainClient interface {
    GetBalance(ctx context.Context, address, tokenAddress string) (string, error)
    SendTransaction(ctx context.Context, signedTx []byte) (string, error)
    GetTransactionStatus(ctx context.Context, txHash string) (bool, int64, error)
    BuildTransferTx(ctx context.Context, from, to, amount, tokenAddress string) ([]byte, error)
    SubmitMixProof(ctx context.Context, requestID, proofHash, teeSignature string) (string, error)
}
```

### 4. Mixing Executor (`executor.go`)

Background worker for transaction execution:

- **Scheduled Processing**: Polls for due transactions
- **Internal Transfers**: Pool-to-pool obfuscation
- **Delivery Execution**: Sends funds to target addresses
- **Retry Handling**: Automatic retry with exponential backoff
- **Confirmation Tracking**: Waits for blockchain confirmations

## Security Model

### Double-Blind Architecture

Neither party knows the other's private key:

| Component | TEE | Master |
|-----------|-----|--------|
| Private Key Location | Sealed in enclave | Cold storage |
| Public Key | Derived online | Pre-derived offline |
| Signing Capability | Daily operations | Emergency recovery |
| Compromise Impact | Limited to TEE | Limited to Master |

### 1-of-2 Multi-sig

Each pool account is a Neo N3 1-of-2 multi-sig address:

```
Verification Script:
  PUSH 1                    # Threshold
  PUSH <TEE_PubKey>         # TEE public key (33 bytes, compressed)
  PUSH <Master_PubKey>      # Master public key (33 bytes, compressed)
  PUSH 2                    # Total keys
  SYSCALL CheckMultiSig
```

**Benefits:**
- Either key can sign independently
- No single point of failure
- Recovery possible if TEE is compromised
- Daily operations don't require Master key

### HD Key Derivation

Path: `m/44'/888'/0'/0/{index}`

- `44'`: BIP-44 purpose
- `888'`: Neo N3 coin type
- `0'`: Account (hardened)
- `0`: External chain
- `{index}`: Pool account index

Each pool account uses a unique index, ensuring:
- No on-chain linkability between pools
- Independent key compromise isolation
- Deterministic key recovery

## Mix Request Lifecycle

```
┌─────────┐     ┌──────────┐     ┌─────────┐     ┌───────────┐     ┌───────────┐
│ Pending │────►│Deposited │────►│ Mixing  │────►│ Completed │     │Withdrawable│
└─────────┘     └──────────┘     └─────────┘     └───────────┘     └───────────┘
     │                                                                    ▲
     │                                                                    │
     └────────────────────────────────────────────────────────────────────┘
                            (7 days after mix_end)
```

1. **Pending**: Request created, awaiting user deposit
2. **Deposited**: User funds received in pool accounts
3. **Mixing**: Internal obfuscation transactions in progress
4. **Completed**: All funds delivered to target addresses
5. **Withdrawable**: Service unavailable, user can force withdraw

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/requests` | List mix requests for account |
| POST | `/requests` | Create new mix request |
| GET | `/requests/{id}` | Get specific mix request |
| POST | `/requests/{id}/deposit` | Confirm deposit |
| POST | `/requests/{id}/claim` | Create withdrawal claim |
| GET | `/stats` | Get mixer statistics |

## Configuration

### TEE Manager

```go
TEEManagerConfig{
    SeedSize:           32,              // Root seed size in bytes
    CacheSize:          1000,            // Max cached derived keys
    AttestationTimeout: 30 * time.Second,
}
```

### Chain Client

```go
ChainClientConfig{
    RPCURL:             "http://localhost:10332",
    MixerContractHash:  "0x...",         // Mixer contract script hash
    RequestTimeout:     30 * time.Second,
    ConfirmationBlocks: 1,
}
```

### Executor

```go
ExecutorConfig{
    PollInterval:        30 * time.Second,
    BatchSize:           10,
    RetryAttempts:       3,
    RetryDelay:          5 * time.Second,
    ConfirmationTimeout: 5 * time.Minute,
}
```

## Testing

### Smoke Tests

```bash
SGX_MODE=SIM go test -v -tags=smoke -timeout=2m ./tests/smoke/...
```

### Integration Tests

```bash
SGX_MODE=SIM go test -v -tags=integration -timeout=3m ./tests/integration/mixer/...
```

## File Structure

```
packages/com.r3e.services.mixer/
├── README.md              # This file
├── doc.go                 # Package documentation
├── domain.go              # Type definitions
├── store.go               # Store interface
├── store_postgres.go      # PostgreSQL implementation
├── service.go             # Core business logic
├── tee_manager.go         # TEE key management
├── master_key_provider.go # Offline key management
├── chain_client.go        # Neo N3 blockchain client
├── executor.go            # Background transaction executor
├── hd_key.go              # HD key derivation utilities
├── multisig.go            # Neo N3 multi-sig utilities
└── http.go                # HTTP API handlers
```

## Security Considerations

1. **TEE Root Seed**: Must be sealed using actual TEE sealing in production
2. **Master Private Key**: Must be kept in cold storage, never online
3. **Pool Account Rotation**: Retire pool accounts after 30 days
4. **7-Day Withdrawal Lock**: Prevents immediate fund extraction on service failure
5. **ZK Proofs**: Provide privacy without revealing transaction details

## Dependencies

- `system/tee`: TEE engine provider
- `pkg/logger`: Structured logging
- `system/framework`: Service engine framework
