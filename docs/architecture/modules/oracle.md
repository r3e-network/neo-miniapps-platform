# Oracle Module

## Responsibilities

- Manage external data sources (REST/WebSocket/etc) and associated metadata.
- Create and track oracle requests, including status transitions and results.
- Provide a lifecycle-managed dispatcher for polling pending requests and resolving callbacks.

## Key Components

- `internal/app/domain/oracle` – data source and request models.
- `internal/app/services/oracle/service.go` – business logic.
- `internal/app/services/oracle/dispatcher.go` – background runner.
- Store contract (`OracleStore`) with memory/PostgreSQL implementations.
- HTTP endpoints: `/accounts/{id}/oracle/sources`, `/accounts/{id}/oracle/requests`.

## Interactions

- Functions service delegates creation/completion of requests.
- Dispatcher runs as `system.Service`; integration with blockchain callbacks will extend it.

## Usage

```go
oracleSvc := oracle.New(accountsStore, oracleStore, log)
source, _ := oracleSvc.CreateSource(ctx, accountID, "prices", "https://api.example.com", "GET", "", nil, "")
req, _ := oracleSvc.CreateRequest(ctx, accountID, source.ID, `{"pair":"NEO/USD"}`)
_ = oracleSvc.CompleteRequest(ctx, req.ID, `{"price":10.5}`)
```

## Checklist

- [x] Create/update/list oracle data sources with uniqueness validation.
- [x] Create/list requests with status transitions.
- [x] Lifecycle dispatcher service registration.
- [x] Callback resolver with timeout handling (future: blockchain integration).
