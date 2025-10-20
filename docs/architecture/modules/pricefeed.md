# Price Feed Module

## Responsibilities

- Manage price feed configurations (base/quote pair, intervals, deviation thresholds).
- Record historical price snapshots for auditing.
- Provide lifecycle-managed refresher service for polling/scheduling.
- Fetch external prices and persist snapshots automatically.

## Key Components

- `internal/app/domain/pricefeed` – feed & snapshot models.
- `internal/app/services/pricefeed/service.go` – business logic.
- `internal/app/services/pricefeed/refresher.go` – periodic runner.
- Store contract (`PriceFeedStore`) with memory/PostgreSQL implementations.
- HTTP endpoints: `/accounts/{id}/pricefeeds`, `/accounts/{id}/pricefeeds/{feed_id}`, `/accounts/{id}/pricefeeds/{feed_id}/snapshots`.

## Interactions

- Functions service can create feeds and record snapshots for downstream analytics.
- Future runtime integrations can use the refresher to trigger updates based on deviation thresholds.

## Usage

```go
pfSvc := pricefeed.New(accountsStore, priceFeedStore, log)
feed, _ := pfSvc.CreateFeed(ctx, accountID, "NEO", "USD", "@every 1m", "@every 1h", 0.5)
_, _ = pfSvc.RecordSnapshot(ctx, feed.ID, 12.34, "oracle", time.Now())
```

## Checklist

- [x] Create/update/list price feed definitions.
- [x] Record/list price snapshots.
- [x] Lifecycle refresher service registration.
- [x] Source aggregation via pluggable fetchers (future work: production sources).
