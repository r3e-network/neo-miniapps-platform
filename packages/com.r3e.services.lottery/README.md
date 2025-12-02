# MegaLottery Service

A decentralized lottery service similar to Mega Millions, featuring:
- **VRF Integration**: Verifiable random number generation for fair draws
- **Automation Service**: Periodic automated draws via cron scheduling
- **Smart Contract**: Neo N3 blockchain contract for on-chain transparency

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      MegaLottery Service                        │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐ │
│  │   Lottery   │  │     VRF     │  │      Automation         │ │
│  │   Service   │──│   Service   │──│       Service           │ │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘ │
│         │                │                     │                │
│         ▼                ▼                     ▼                │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                  Neo N3 Smart Contract                      ││
│  │                   (MegaLottery.cs)                          ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

## Game Rules (Mega Millions Style)

### Number Selection
- **Main Numbers**: Pick 5 numbers from 1-70
- **Mega Number**: Pick 1 number from 1-25
- **Quick Pick**: Auto-generate random numbers

### Prize Tiers

| Tier | Match | Prize Type | Prize |
|------|-------|------------|-------|
| 1 | 5 + Mega | Jackpot | 50% of pool |
| 2 | 5 | Second | 15% of pool |
| 3 | 4 + Mega | Third | 10% of pool |
| 4 | 4 | Fixed | 0.5 GAS |
| 5 | 3 + Mega | Fixed | 0.2 GAS |
| 6 | 3 | Fixed | 0.1 GAS |
| 7 | 2 + Mega | Fixed | 0.05 GAS |
| 8 | 1 + Mega | Fixed | 0.02 GAS |
| 9 | Mega only | Fixed | 0.01 GAS |

## Usage

### 1. Create Lottery Configuration

```go
config, err := lotteryService.CreateConfig(ctx, accountID, lottery.LotteryConfig{
    Name:         "My Lottery",
    Description:  "Weekly lottery draw",
    VRFKeyID:     "vrf-key-id",
    DrawSchedule: "0 0 * * 3,6", // Wed & Sat at midnight
})
```

### 2. Start a Round

```go
round, err := lotteryService.StartRound(ctx, accountID)
```

### 3. Buy Tickets

```go
// Manual selection
ticket, err := lotteryService.BuyTicket(ctx, playerAccountID, roundID, 
    []int{5, 12, 23, 45, 67}, // Main numbers
    15,                        // Mega number
)

// Quick pick (random)
ticket, err := lotteryService.QuickPick(ctx, playerAccountID, roundID)
```

### 4. Execute Draw

```go
// Automated draw with VRF
result, err := lotteryService.ExecuteAutomatedDraw(ctx, roundID)

// Result contains:
// - WinningNumbers: [5]int
// - MegaNumber: int
// - Winners: []Winner
// - TotalPrizePool: int64
```

### 5. Claim Prize

```go
ticket, err := lotteryService.ClaimPrize(ctx, playerAccountID, ticketID)
```

## HTTP API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | /rounds | List lottery rounds |
| POST | /rounds | Start new round |
| GET | /rounds/{id} | Get round details |
| GET | /rounds/active | Get active round |
| POST | /rounds/{id}/draw | Execute draw |
| GET | /tickets | List my tickets |
| POST | /tickets | Buy ticket |
| GET | /tickets/{id} | Get ticket details |
| POST | /tickets/{id}/claim | Claim prize |
| GET | /stats | Get lottery statistics |

## Smart Contract Integration

The `MegaLottery.cs` contract provides:

1. **Ticket Purchase**: On-chain ticket storage with GAS payment
2. **VRF Callback**: Receives random numbers from RandomnessHub
3. **Prize Distribution**: Automatic prize calculation and payout
4. **Automation Hook**: Called by AutomationScheduler for periodic draws

### Contract Methods

```csharp
// Buy a ticket
ByteString BuyTicket(byte[] numbers, byte megaNumber)

// Quick pick
ByteString QuickPick()

// Initiate draw (operator/automation only)
void InitiateDraw()

// Claim prize
BigInteger ClaimPrize(ByteString ticketId)
```

## Configuration

### Default Values

```go
const (
    DefaultTicketPrice       = 10_000_000 // 0.1 GAS
    DefaultMainNumberMin     = 1
    DefaultMainNumberMax     = 70
    DefaultMainNumberCount   = 5
    DefaultMegaNumberMin     = 1
    DefaultMegaNumberMax     = 25
    DefaultMinTicketsForDraw = 10
    DefaultDrawSchedule      = "0 0 * * 3,6" // Wed & Sat
)
```

## Testing

Run the comprehensive test suite:

```bash
go test -v ./packages/com.r3e.services.lottery/...
```

Tests cover:
- Full lottery lifecycle
- Prize tier calculations
- Number validation
- Random number generation
- Winner detection and prize claiming

## Security Considerations

1. **VRF Randomness**: Uses verifiable random functions for provably fair draws
2. **On-chain Transparency**: All tickets and results stored on Neo N3
3. **TEE Attestation**: VRF keys protected by Trusted Execution Environment
4. **Access Control**: Only authorized operators can initiate draws
