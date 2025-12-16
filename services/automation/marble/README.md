# NeoFlow Marble Service

TEE-secured task automation service running inside MarbleRun enclave.

## Overview

The NeoFlow Marble service implements trigger-based task automation:
1. Users register triggers with conditions via API
2. TEE monitors conditions continuously
3. When conditions are met, TEE executes callbacks

## Architecture

```
┌───────────────────────────────────────────────────────────────┐
│                    MarbleRun Enclave (TEE)                    │
│                                                               │
│  ┌─────────────┐    ┌─────────────┐     ┌─────────────┐       │
│  │  Scheduler  │    │  Condition  │     │  Executor   │       │
│  │             │───>│  Evaluator  │────>│ (Callback)  │       │
│  └─────────────┘    └─────────────┘     └─────┬───────┘       │
│         │                  │                  │               │
│  ┌──────▼──────┐    ┌──────▼──────┐           │               │
│  │  Supabase   │    │  NeoFeeds   │           │               │
│  │ Repository  │    │  (Prices)   │           │               │
│  └─────────────┘    └─────────────┘           │               │
└───────────────────────────────────────────────┼───────────────┘
                                                │
                              ┌─────────────────┼───────────────┐
                              ▼                 ▼               │
                       ┌─────────────┐   ┌─────────────┐        │
                       │NeoFlow Svc  │   │User Contract│        │
                       │ (On-Chain)  │   │ (Callback)  │        │
                       └─────────────┘   └─────────────┘        │
```

## File Structure

| File | Purpose |
|------|---------|
| `service.go` | Service initialization and configuration |
| `triggers.go` | Trigger evaluation logic |
| `handlers.go` | HTTP request handlers |
| `api.go` | Route registration |
| `types.go` | Data structures |

Lifecycle is handled by the shared `commonservice.BaseService` (start/stop hooks, workers, standard routes).

## Key Components

### Service Struct

```go
type Service struct {
    *commonservice.BaseService
    mu        sync.RWMutex
    scheduler *Scheduler

    repo neoflowsupabase.RepositoryInterface

    chainClient       *chain.Client
    teeFulfiller      *chain.TEEFulfiller
    neoflowHash       string
    neoFeedsContract  *chain.NeoFeedsContract
    eventListener     *chain.EventListener
    enableChainExec   bool
}
```

### Scheduler

```go
type Scheduler struct {
    mu            sync.RWMutex
    triggers      map[string]*neoflowsupabase.Trigger
    chainTriggers map[uint64]*chain.Trigger
    stopCh        chan struct{}
}
```

## Trigger Types

| Type | ID | Description |
|------|-----|-------------|
| Time | 1 | Cron expressions |
| Price | 2 | Price thresholds |
| Event | 3 | On-chain events |
| Threshold | 4 | Balance thresholds |

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

## Configuration

```go
type Config struct {
    Marble           *marble.Marble
    DB               database.RepositoryInterface
    NeoFlowRepo      neoflowsupabase.RepositoryInterface
    ChainClient      *chain.Client
    TEEFulfiller     *chain.TEEFulfiller
    NeoFlowHash      string
    NeoFeedsContract *chain.NeoFeedsContract
    EventListener    *chain.EventListener
    EnableChainExec  bool
}
```

## Constants

| Constant | Value | Description |
|----------|-------|-------------|
| `SchedulerInterval` | 1 second | Trigger check frequency |
| `ChainTriggerInterval` | 5 seconds | Chain trigger sync |
| `ServiceFeePerExecution` | 0.0005 GAS | Per execution fee |

## Dependencies

### Infrastructure Packages

| Package | Purpose |
|---------|---------|
| `infrastructure/chain` | Neo N3 blockchain interaction + price feed reads (`chain.NeoFeedsContract`) |
| `infrastructure/marble` | MarbleRun TEE utilities |
| `infrastructure/service` | Base service |
| `services/automation/supabase` | Repository |

## Related Documentation

- [NeoFlow Service Overview](../README.md)
- [Chain Integration](../../../infrastructure/chain/README.md)
- [Smart Contract](../contract/README.md)
- [Database Layer](../supabase/README.md)
