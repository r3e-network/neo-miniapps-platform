# NeoFlow Marble Service

TEE-secured task automation service running inside MarbleRun enclave.

## Overview

The NeoFlow Marble service implements trigger-based task automation:
1. Users register triggers with conditions via API
2. TEE monitors conditions continuously
3. When conditions are met, TEE executes callbacks (off-chain webhooks or on-chain invocations)

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
│  │  Supabase   │    │  Neo N3     │           │               │
│  │ Repository  │    │  Contracts  │           │               │
│  └─────────────┘    └─────────────┘           │               │
└───────────────────────────────────────────────┼───────────────┘
                                                │
                              ┌─────────────────┼───────────────┐
                              ▼                 ▼               │
                       ┌─────────────┐   ┌─────────────┐        │
                       │Automation   │   │User Contract│        │
                       │Anchor       │   │ (Callback)  │        │
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

    chainClient          *chain.Client
    priceFeedHash        string
    priceFeed            *chain.PriceFeedContract
    automationAnchorHash string
    automationAnchor     *chain.AutomationAnchorContract
    txProxy              txproxytypes.Invoker
    eventListener        *chain.EventListener
    enableChainExec      bool
}
```

### Scheduler

```go
type Scheduler struct {
    mu            sync.RWMutex
    triggers      map[string]*neoflowsupabase.Trigger
    anchoredTasks map[string]*anchoredTaskState
}
```

## Trigger Types (Current)

- Supabase triggers: `cron` (webhooks today)
- Anchored tasks (AutomationAnchor): `cron`, `price` (uses on-chain `PriceFeed`)

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
    ChainClient          *chain.Client
    PriceFeedHash        string
    AutomationAnchorHash string
    TxProxy              txproxytypes.Invoker
    EventListener        *chain.EventListener
    EnableChainExec      bool
}
```

## Constants

| Constant | Value | Description |
|----------|-------|-------------|
| `SchedulerInterval` | 1 second | Trigger check frequency |
| `AnchoredTaskInterval` | 5 seconds | Anchored task evaluation frequency |
| `ServiceFeePerExecution` | 0.0005 GAS | Per execution fee |

## Dependencies

### Infrastructure Packages

| Package | Purpose |
|---------|---------|
| `infrastructure/chain` | Neo N3 blockchain interaction + platform contract reads (`PriceFeed`, `AutomationAnchor`) |
| `infrastructure/marble` | MarbleRun TEE utilities |
| `infrastructure/service` | Base service |
| `services/automation/supabase` | Repository |

## Related Documentation

- [NeoFlow Service Overview](../README.md)
- [Chain Integration](../../../infrastructure/chain/README.md)
- [Smart Contract](../contract/README.md)
- [Database Layer](../supabase/README.md)
