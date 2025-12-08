# MegaLottery - Decentralized Lottery on Neo N3

A fully decentralized lottery dApp powered by Neo N3 blockchain and Service Layer infrastructure.

## Features

- **Provably Fair**: Uses Service Layer VRF (Verifiable Random Function) for cryptographically secure random number generation
- **Automated Draws**: Daily draws triggered automatically by Service Layer Automation
- **Transparent**: All operations recorded on-chain for full transparency
- **Multiple Prize Tiers**: MegaMillions-style prize structure with jackpot rollover
- **1-Minute Lockout**: Ticket sales lock 1 minute before draw for fairness

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        MegaLottery dApp                         │
├─────────────────────────────────────────────────────────────────┤
│  Frontend (React + Vite)                                        │
│  - Wallet Connection (NeoLine, OneGate, O3)                     │
│  - Number Picker UI                                             │
│  - Ticket Management                                            │
│  - Results Display                                              │
├─────────────────────────────────────────────────────────────────┤
│  Smart Contract (Neo N3 C#)                                     │
│  - Ticket Purchase & Storage                                    │
│  - Prize Distribution                                           │
│  - VRF Callback Handler                                         │
│  - Automation Trigger Handler                                   │
├─────────────────────────────────────────────────────────────────┤
│  Service Layer Integration                                      │
│  ┌─────────────┐  ┌─────────────────┐                           │
│  │ VRF Service │  │ Automation Svc  │                           │
│  │ (Random #s) │  │ (Daily Draws)   │                           │
│  └─────────────┘  └─────────────────┘                           │
└─────────────────────────────────────────────────────────────────┘
```

## Game Rules

### How to Play
1. Pick 5 main numbers (1-70)
2. Pick 1 Mega Ball (1-25)
3. Pay 2 GAS per ticket
4. Wait for the daily draw at 00:00 UTC

### Prize Tiers

| Match | Prize | Odds |
|-------|-------|------|
| 5 + Mega Ball | JACKPOT (50% of pool) | 1 in 302,575,350 |
| 5 Numbers | 20% of pool | 1 in 12,607,306 |
| 4 + Mega Ball | 10% of pool | 1 in 931,001 |
| 4 or 3 + Mega | 10% of pool | 1 in 38,792 |
| 3 or Mega Ball | 10% of pool | 1 in 606 |

*10% goes to operations fund*

### Important Rules
- Ticket sales lock **1 minute before** each draw
- Prizes must be claimed within **30 days**
- Unclaimed jackpots roll over to the next draw

## Project Structure

```
dapps/lottery/
├── contract/
│   ├── MegaLottery.cs      # Neo N3 smart contract
│   └── MegaLottery.csproj  # Project file
├── frontend/
│   ├── src/
│   │   ├── components/     # React components
│   │   ├── hooks/          # Custom hooks (wallet, lottery)
│   │   ├── pages/          # Page components
│   │   └── utils/          # Utility functions
│   ├── public/             # Static assets
│   └── package.json        # Dependencies
└── README.md               # This file
```

## Setup & Deployment

### Prerequisites
- Node.js 18+
- .NET SDK 7.0+ (for contract compilation)
- Neo N3 wallet with GAS

### Contract Deployment

1. **Compile the contract**:
```bash
cd contract
dotnet build
```

2. **Deploy to Neo N3**:
```bash
# Using neo-express or neo-cli
neo-express contract deploy MegaLottery.nef
```

3. **Initialize the contract**:
```bash
# Set VRF contract address
neo-express contract invoke MegaLottery setVRFContract <VRF_CONTRACT_HASH>

# Set Automation contract address
neo-express contract invoke MegaLottery setAutomationContract <AUTOMATION_CONTRACT_HASH>

# Register daily trigger
neo-express contract invoke MegaLottery registerDailyTrigger
```

### Frontend Setup

1. **Install dependencies**:
```bash
cd frontend
npm install
```

2. **Configure environment**:
```bash
cp .env.example .env
# Edit .env with your contract hash
```

3. **Run development server**:
```bash
npm run dev
```

4. **Build for production**:
```bash
npm run build
```

## Service Layer Integration

### VRF Integration

The contract requests random numbers from Service Layer VRF:

```csharp
// Request 6 random numbers for the draw
ByteString requestId = Contract.Call(vrfContract, "requestRandomness", CallFlags.All, new object[] {
    seed,           // Unique seed
    6,              // Number of random words
    contractHash,   // Callback contract
    200000          // Callback gas limit
});
```

The VRF service calls back with verifiable random numbers:

```csharp
public static void FulfillRandomness(ByteString requestId, BigInteger[] randomWords)
{
    // Verify caller is VRF contract
    // Convert random words to winning numbers
    // Complete the draw
}
```

### Automation Integration

Daily draws are triggered by Service Layer Automation:

```csharp
// Register time-based trigger for daily draws
Contract.Call(automationContract, "registerTrigger", CallFlags.All, new object[] {
    contractHash,       // Callback contract
    1,                  // Trigger type: Time
    "0 0 * * *",        // Cron: Daily at midnight UTC
    "unveilWinner",     // Callback method
    0                   // Max executions (unlimited)
});
```

## Security Considerations

1. **VRF Verification**: All random numbers are cryptographically verified
2. **Lockout Period**: 1-minute lockout prevents last-second manipulation
3. **On-Chain Storage**: All tickets and results stored on blockchain
4. **Access Control**: Admin functions protected by witness checks
5. **Reentrancy Protection**: State changes before external calls

## Testing

### Contract Tests
```bash
cd contract
dotnet test
```

### Frontend Tests
```bash
cd frontend
npm test
```

## License

MIT License - See LICENSE file for details.

## Links

- [Service Layer Documentation](https://docs.servicelayer.io)
- [Neo N3 Documentation](https://docs.neo.org)
- [NeoLine Wallet](https://neoline.io)
