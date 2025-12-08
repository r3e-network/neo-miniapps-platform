# GasBank Module

The `gasbank` module provides gas fee management for the Service Layer.

## Overview

This module handles gas fee operations including:

- User gas balance tracking
- Gas deposits and withdrawals
- Service fee deductions
- Balance queries

## Components

### GasBank (`gasbank.go`)

Main gas bank service for managing user gas balances.

```go
gb := gasbank.New(gasbank.Config{
    DB: repository,
})
```

## API

### Get Balance

```go
balance, err := gb.GetBalance(ctx, userID)
```

### Deposit

```go
err := gb.Deposit(ctx, userID, amount, txHash)
```

### Withdraw

```go
err := gb.Withdraw(ctx, userID, amount, destinationAddress)
```

### Deduct Service Fee

```go
err := gb.DeductFee(ctx, userID, serviceID, amount)
```

## Data Model

```go
type GasBankAccount struct {
    UserID       string    `json:"user_id"`
    Balance      int64     `json:"balance"`      // In smallest GAS unit (10^-8)
    TotalDeposit int64     `json:"total_deposit"`
    TotalSpent   int64     `json:"total_spent"`
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}
```

## Fee Structure

| Service | Fee Type | Amount |
|---------|----------|--------|
| VRF | Per request | 0.001 GAS |
| Mixer | Percentage | 0.5% of amount |
| Automation | Per execution | 0.0005 GAS |
| DataFeeds | Per update | 0.0001 GAS |

## Usage Example

```go
package main

import (
    "context"
    "github.com/R3E-Network/service_layer/internal/gasbank"
)

func main() {
    gb := gasbank.New(gasbank.Config{DB: repo})

    // Check balance before operation
    balance, _ := gb.GetBalance(ctx, userID)
    if balance < requiredFee {
        // Insufficient balance
        return
    }

    // Deduct fee for service
    gb.DeductFee(ctx, userID, "vrf", requiredFee)

    // Process service request...
}
```

## Testing

```bash
go test ./internal/gasbank/... -v
```

Current test coverage: **40.9%**
