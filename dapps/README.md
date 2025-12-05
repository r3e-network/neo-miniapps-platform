# Service Layer Example DApps

This directory contains example decentralized applications (DApps) built on the Service Layer platform, demonstrating how to integrate with various services.

## DApps

### 1. MegaLottery

A decentralized lottery application powered by **VRF** (Verifiable Random Function) and **Automation** services.

**Features:**
- Fair winner selection using cryptographically secure VRF
- Automated draws triggered by the Automation service
- Multiple prize tiers based on number matches
- Transparent and verifiable on-chain

**Services Used:**
- **VRF Service**: Generates provably fair random numbers for winner selection
- **Automation Service**: Schedules and triggers lottery draws automatically

**Directory Structure:**
```
lottery/
├── contract/
│   └── MegaLottery.cs      # Neo N3 smart contract
└── frontend/
    ├── src/
    │   ├── components/     # React components
    │   ├── hooks/          # Custom React hooks
    │   └── utils/          # Utility functions
    └── package.json
```

### 2. PrivacyMixer

A privacy-preserving transaction mixer powered by the **Mixer** service with TEE protection.

**Features:**
- Fixed denomination pools for anonymity (1, 10, 100 GAS)
- Commitment-nullifier scheme for unlinkable transactions
- TEE-based mixing for enhanced privacy
- Time-delayed withdrawals for additional security

**Services Used:**
- **Mixer Service**: Handles deposit verification and withdrawal proofs inside TEE

**Directory Structure:**
```
mixer/
├── contract/
│   └── PrivacyMixer.cs     # Neo N3 smart contract
└── frontend/
    ├── src/
    │   ├── components/     # React components
    │   ├── hooks/          # Custom React hooks
    │   └── utils/          # Crypto utilities
    └── package.json
```

## Getting Started

### Prerequisites

- Node.js 18+
- Neo N3 wallet (NeoLine, O3, or WalletConnect compatible)
- Access to Neo N3 network (TestNet or MainNet)

### Running the Frontend

```bash
# Lottery DApp
cd lottery/frontend
npm install
npm run dev

# Mixer DApp
cd mixer/frontend
npm install
npm run dev
```

### Deploying Contracts

1. Compile the C# contracts using Neo DevPack
2. Deploy to Neo N3 network
3. Configure the Gateway contract address
4. Update frontend environment variables

### Environment Variables

Create a `.env` file in each frontend directory:

```env
VITE_RPC_URL=https://testnet.neo.org:443
VITE_LOTTERY_CONTRACT=0x...
VITE_MIXER_CONTRACT=0x...
VITE_GATEWAY_CONTRACT=0x...
```

## Architecture

### Lottery Flow

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   User      │────▶│  Gateway    │────▶│  Lottery    │
│   Wallet    │     │  Contract   │     │  Contract   │
└─────────────┘     └─────────────┘     └─────────────┘
                           │
                           ▼
                    ┌─────────────┐
                    │  Service    │
                    │   Layer     │
                    │   (TEE)     │
                    └─────────────┘
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
        ┌─────────┐  ┌─────────┐  ┌─────────┐
        │   VRF   │  │  Auto-  │  │ Oracle  │
        │ Service │  │ mation  │  │ Service │
        └─────────┘  └─────────┘  └─────────┘
```

### Mixer Flow

```
┌─────────────┐                    ┌─────────────┐
│  Deposit    │───commitment──────▶│   Mixer     │
│  Address    │                    │  Contract   │
└─────────────┘                    └─────────────┘
                                          │
                                          ▼
                                   ┌─────────────┐
                                   │   Mixer     │
                                   │  Service    │
                                   │   (TEE)     │
                                   └─────────────┘
                                          │
                                          ▼
┌─────────────┐                    ┌─────────────┐
│  Withdraw   │◀───nullifier+proof─│   Mixer     │
│  Address    │                    │  Contract   │
└─────────────┘                    └─────────────┘
```

## Security Considerations

### Lottery
- VRF ensures randomness cannot be predicted or manipulated
- All random number generation happens inside TEE
- Results are verifiable on-chain

### Mixer
- Deposits and withdrawals are cryptographically unlinkable
- Secret notes must be stored securely by users
- Time delays add additional privacy protection
- TEE ensures mixing logic cannot be observed

## License

MIT License - See LICENSE file for details.
