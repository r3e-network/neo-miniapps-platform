# NeoFlow Service

Task neoflow service for the Neo Service Layer.

## Overview

The NeoFlow service provides trigger-based task neoflow for smart contracts. Users can register triggers with conditions, and the TEE monitors these conditions continuously, executing callbacks when conditions are met.

## Architecture

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│    User      │     │ NeoFlow      │     │ User Contract│
│              │     │ Service (TEE)│     │              │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                    │
       │ Register Trigger   │                    │
       │───────────────────>│                    │
       │                    │                    │
       │                    │ Monitor Condition  │
       │                    │                    │
       │                    │ Condition Met!     │
       │                    │                    │
       │                    │ Execute Callback   │
       │                    │───────────────────>│
```

## Trigger Types

| Type | ID | Description | Example |
|------|-----|-------------|---------|
| Time | 1 | Cron expressions | "Every Friday 00:00 UTC" |
| Price | 2 | Price thresholds | "When BTC > $100,000" |
| Event | 3 | On-chain events | "When contract X emits event Y" |
| Threshold | 4 | Balance thresholds | "When balance < 10 GAS" |

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Service health check |
| `/info` | GET | Service status |
| `/triggers` | GET | List user's triggers |
| `/triggers` | POST | Create trigger |
| `/triggers/{id}` | GET | Get trigger details |
| `/triggers/{id}` | PUT | Update trigger |
| `/triggers/{id}` | DELETE | Delete trigger |
| `/triggers/{id}/enable` | POST | Enable trigger |
| `/triggers/{id}/disable` | POST | Disable trigger |
| `/triggers/{id}/executions` | GET | List executions |
| `/triggers/{id}/resume` | POST | Resume trigger |

## Request/Response Types

### Create Trigger (Cron)

```json
POST /triggers
{
    "name": "Daily Report",
    "trigger_type": "cron",
    "schedule": "0 9 * * *",
    "action": {
        "type": "webhook",
        "url": "https://example.com/callback",
        "method": "POST"
    }
}
```

### Create Trigger (Price)

```json
POST /triggers
{
    "name": "BTC Alert",
    "trigger_type": "price",
    "condition": {
        "feed_id": "BTC/USD",
        "operator": ">",
        "threshold": 10000000000000
    },
    "action": {
        "type": "contract_call",
        "contract": "0x...",
        "method": "onPriceAlert"
    }
}
```

### Create Trigger (Threshold)

```json
POST /triggers
{
    "name": "Low Balance Alert",
    "trigger_type": "threshold",
    "condition": {
        "address": "NAddr...",
        "asset": "GAS",
        "operator": "<",
        "threshold": 1000000000
    },
    "action": {
        "type": "webhook",
        "url": "https://example.com/alert"
    }
}
```

### Trigger Response

```json
{
    "id": "trigger-123",
    "name": "Daily Report",
    "trigger_type": "cron",
    "schedule": "0 9 * * *",
    "enabled": true,
    "last_execution": "2025-12-07T09:00:00Z",
    "next_execution": "2025-12-08T09:00:00Z",
    "created_at": "2025-12-01T00:00:00Z"
}
```

## Cron Expression Format

Standard 5-field cron format:

```
┌───────────── minute (0 - 59)
│ ┌───────────── hour (0 - 23)
│ │ ┌───────────── day of month (1 - 31)
│ │ │ ┌───────────── month (1 - 12)
│ │ │ │ ┌───────────── day of week (0 - 6) (Sunday = 0)
│ │ │ │ │
* * * * *
```

Examples:
- `0 9 * * *` - Every day at 9:00 AM
- `*/15 * * * *` - Every 15 minutes
- `0 0 * * 1` - Every Monday at midnight
- `0 0 1 * *` - First day of every month

## Fee Structure

| Operation | Fee |
|-----------|-----|
| Per execution | 0.0005 GAS |

## Data Layer

The NeoFlow service uses a service-specific Supabase repository for database operations.

### Package Structure

```
services/neoflow/
├── supabase/
│   ├── repository.go    # NeoFlow-specific repository interface
│   └── models.go        # NeoFlow data models (Trigger, Execution)
├── neoflow_service.go    # Service implementation
├── neoflow_handlers.go   # HTTP handlers
├── neoflow_triggers.go   # Trigger logic
├── neoflow_types.go      # Type definitions
└── README.md
```

### Repository Interface

```go
import neoflowsupabase "github.com/R3E-Network/service_layer/services/neoflow/supabase"

// Create repository
neoflowRepo := neoflowsupabase.NewRepository(baseRepo)

// Operations
err := neoflowRepo.CreateTrigger(ctx, &neoflowsupabase.Trigger{...})
triggers, err := neoflowRepo.GetTriggers(ctx, userID)
err := neoflowRepo.UpdateTrigger(ctx, trigger)
err := neoflowRepo.CreateExecution(ctx, &neoflowsupabase.Execution{...})
executions, err := neoflowRepo.GetExecutions(ctx, triggerID, limit)
```

### Data Models

| Model | Description |
|-------|-------------|
| `Trigger` | NeoFlow trigger with type, condition, action |
| `Execution` | Execution record with status, result, timestamps |

## Testing

```bash
go test ./services/neoflow/... -v -cover
```

Current test coverage: **11.1%**

## Version

- Service ID: `neoflow`
- Version: `2.0.0`
