# NeoRand Supabase Repository

Database layer for the NeoRand VRF service.

## Overview

This package provides NeoRand-specific data access for tracking VRF requests and their fulfillment status.

## File Structure

| File | Purpose |
|------|---------|
| `repository.go` | Repository interface and implementation |
| `models.go` | Data models |

## Data Models

### RequestRecord

```go
type RequestRecord struct {
    ID          string    `json:"id"`
    RequestID   string    `json:"request_id"`   // On-chain request ID
    UserContract string   `json:"user_contract"`
    Seed        string    `json:"seed"`
    NumWords    int       `json:"num_words"`
    Status      string    `json:"status"`       // pending, processing, fulfilled, failed
    RandomWords []string  `json:"random_words,omitempty"`
    Proof       string    `json:"proof,omitempty"`
    TxHash      string    `json:"tx_hash,omitempty"`
    CreatedAt   time.Time `json:"created_at"`
    FulfilledAt time.Time `json:"fulfilled_at,omitempty"`
    Error       string    `json:"error,omitempty"`
}
```

## Repository Interface

```go
type RepositoryInterface interface {
    Create(ctx context.Context, req *RequestRecord) error
    Update(ctx context.Context, req *RequestRecord) error
    GetByRequestID(ctx context.Context, requestID string) (*RequestRecord, error)
    ListByStatus(ctx context.Context, status string) ([]RequestRecord, error)
}
```

## Usage

```go
import neorandsupabase "github.com/R3E-Network/service_layer/services/neorand/supabase"

repo := neorandsupabase.NewRepository(baseRepo)

// Create request
err := repo.Create(ctx, &neorandsupabase.RequestRecord{
    RequestID:    "123",
    UserContract: "NUser...",
    Seed:         "0x...",
    NumWords:     1,
    Status:       "pending",
})

// Get pending requests
pending, err := repo.ListByStatus(ctx, "pending")
```

## Status Values

| Status | Description |
|--------|-------------|
| `pending` | Request received, awaiting processing |
| `processing` | VRF generation in progress |
| `fulfilled` | Randomness delivered |
| `failed` | Fulfillment failed |

## Related Documentation

- [Marble Service](../marble/README.md)
- [Service Overview](../README.md)
