# FlashLoan Service

Decentralized flash loan protocol for Neo N3 blockchain.

## Contract

- **Address**: `0xb765bf77e9bf443e65132387ed41c9a3fd90169b`
- **Network**: Neo N3 Testnet
- **Fee**: 0.09% (9 basis points)

## Features

- Uncollateralized flash loans
- Automatic repayment verification
- Pool liquidity management
- Transaction statistics tracking

## Methods

| Method                        | Description                  |
| ----------------------------- | ---------------------------- |
| `flashLoan(borrower, amount)` | Execute a flash loan         |
| `getPoolBalance()`            | Get current pool liquidity   |
| `getStats()`                  | Get protocol statistics      |
| `calculateRepayment(amount)`  | Calculate required repayment |

## Simulation

The `simulation/` directory contains a professional account pool management system for testing and validating the flash loan protocol.

### Features

- Automatic account balance management
- Transaction recording with full metadata
- Account rotation for empty accounts
- Real-time statistics tracking
- Supabase database integration

### Usage

```bash
cd simulation

# Initialize database schema (run SQL in Supabase)
cat schema.sql

# Run with pool management
go run . run

# Run in simple mode (single account)
go run . simple

# Check status
go run . status
```

### Database Schema

See `simulation/schema.sql` for the complete database schema including:

- `flashloan_accounts` - Managed account pool
- `flashloan_transactions` - Transaction history
- `flashloan_rotation_history` - Account rotation tracking
- `flashloan_funding_queue` - Pending deposits
- `flashloan_pool_stats` - Aggregate statistics

## MiniApp

The FlashLoan MiniApp is located at `miniapps/builtin/flashloan/` and provides a web interface for:

- Viewing protocol statistics
- Executing flash loans
- Monitoring recent activity
