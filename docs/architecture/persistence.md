# Persistence Architecture

This document outlines how the refactored API skeleton selects storage
implementations and how to configure each option.

## Storage Abstraction

Both the `functions` and `secrets` services expose a simple `Store`
interface. Service logic is agnostic of the underlying persistence layer and
can therefore switch between implementations at runtime.

```go
type Store interface {
    List(ctx context.Context) ([]Item, error)
    Get(ctx context.Context, id string) (Item, error)
    Create(ctx context.Context, item Item) (Item, error)
    Delete(ctx context.Context, id string) error
}
```

Two concrete stores are provided:

| Store       | Description                                                                 |
|-------------|-----------------------------------------------------------------------------|
| `memory`    | In-memory map (default). Useful for tests or ephemeral environments.        |
| `sql`       | Relational database store. Uses `database/sql` and bootstraps table schema. |

## Selecting a Store

The application reads the storage mode from the configuration file. Both
functions and secrets can be configured independently.

```yaml
services:
  functions:
    storage: memory # or sql
  secrets:
    storage: memory # or sql
```

If the storage value is omitted, `memory` is assumed.

### SQL Configuration

When any service is configured for `sql`, the runtime opens a shared
`*sql.DB` using the `database` block:

```yaml
database:
  driver: postgres
  dsn: "postgres://user:pass@localhost:5432/service_layer?sslmode=disable"
  max_open_conns: 10      # optional
  max_idle_conns: 5       # optional
  conn_max_lifetime: 300  # optional, seconds
```

The application calls `db.PingContext` during start-up and shuts the
connection down gracefully on exit. A SQL driver must be imported in
`go.mod` (e.g. `_ "github.com/lib/pq"` for PostgreSQL).

### Table Schema

Each SQL-backed module uses the shared migrations under
`internal/platform/migrations`. The core schema is split across:

- `0001_app_core.sql` – `app_accounts`, `app_functions`, `app_triggers`
- `0002_app_domain_tables.sql` – `app_automation_jobs`,
  `app_price_feeds`, `app_price_feed_snapshots`, `app_oracle_sources`,
  `app_oracle_requests`

The new domain stores (automation, price feed, oracle) rely on these
tables; run the migrations before starting the application in SQL mode.

## Example Configurations

#### In-Memory (default)

```yaml
services:
  functions:
    storage: memory
  secrets:
    storage: memory
```

#### PostgreSQL Backed

```yaml
database:
  driver: postgres
  dsn: "postgres://service_layer:service_layer@localhost:5432/service_layer?sslmode=disable"

services:
  functions:
    storage: sql
  secrets:
    storage: sql
```

## Testing

The SQL stores are covered with sqlmock-based unit tests. The high-level
API tests (`scripts/test-api.sh`) continue to run against the in-memory
stores by default.

To validate SQL connectivity manually, set the storage mode to `sql` and
provide a running database instance. The application logs will surface
any connection or schema issues during start-up.
