# Data Feeds Quickstart (CLI)

This walkthrough shows how to define a data feed, choose an aggregation strategy, and submit signed rounds via `slctl`.

## Prerequisites
- Service Layer running (e.g., `go run ./cmd/appserver`).
- API token exported as `SERVICE_LAYER_TOKEN`.
- `slctl` available (`go run ./cmd/slctl ...`).

## 1) Create an account
```bash
acct=$(slctl accounts create --owner you | jq -r .id)
```

## 2) Register signer wallets
Data feed submissions are gated by workspace wallets.
```bash
signer1=0xabc123abc123abc123abc123abc123abc123abcd
signer2=0xdef456def456def456def456def456def456def0

slctl workspace-wallets create --account "$acct" --wallet "$signer1" --label primary --status active
slctl workspace-wallets create --account "$acct" --wallet "$signer2" --label backup  --status active
```

## 3) Create the feed
Pick an aggregation strategy (`median` default; also supports `mean`, `min`, `max`).
```bash
feed=$(slctl datafeeds create \
  --account "$acct" \
  --pair ETH/USD \
  --decimals 8 \
  --heartbeat-seconds 60 \
  --threshold-ppm 0 \
  --aggregation mean \
  --signer-set "$signer1,$signer2" \
  --metadata '{"env":"dev"}' \
  | jq -r .id)
```

## 4) Submit updates (round 1)
```bash
slctl datafeeds submit --account "$acct" --feed "$feed" --round 1 --price 1800.00 --signer "$signer1" --signature sig1
slctl datafeeds submit --account "$acct" --feed "$feed" --round 1 --price 1820.00 --signer "$signer2" --signature sig2
```
With `mean` aggregation, the round metadata returns `aggregated_price` ≈ `1810.00`.

## 5) Inspect results
```bash
slctl datafeeds updates --account "$acct" --feed "$feed" --limit 5
slctl datafeeds latest  --account "$acct" --feed "$feed"
```

## Notes
- Omit `--aggregation` to inherit the global default (`runtime.datafeeds.aggregation` / `DATAFEEDS_AGGREGATION`).
- Heartbeat/deviation enforcement and signer thresholds apply automatically per feed.
- For HTTP usage, the matching endpoints are:
  - `POST /accounts/{account}/datafeeds` (create)
  - `GET /accounts/{account}/datafeeds/{feed}/updates` (list)
  - `POST /accounts/{account}/datafeeds/{feed}/updates` (submit)
  - `GET /accounts/{account}/datafeeds/{feed}/latest` (latest)
