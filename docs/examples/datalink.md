# DataLink Quickstart (CLI)

Send an off-chain payload through a DataLink channel with retryable deliveries.

## Prerequisites
- Service Layer running (e.g., `go run ./cmd/appserver`).
- API token exported as `SERVICE_LAYER_TOKEN`.
- `slctl` available (`go run ./cmd/slctl ...`).

## 1) Create an account
```bash
acct=$(slctl accounts create --owner you | jq -r .id)
```

## 2) Register a signer wallet
DataLink channels are gated by workspace wallets.
```bash
signer=0xabc123abc123abc123abc123abc123abc123abcd
slctl workspace-wallets create --account "$acct" --wallet "$signer" --label link --status active
```

## 3) Create a channel
```bash
channel=$(slctl datalink channel-create \
  --account "$acct" \
  --name provider-1 \
  --endpoint https://api.provider.test \
  --signers "$signer" \
  --status active \
  --metadata '{"tier":"gold"}' \
  | jq -r .id)
```

## 4) Queue a delivery
```bash
slctl datalink deliver \
  --account "$acct" \
  --channel "$channel" \
  --payload '{"data":"hello"}' \
  --metadata '{"trace":"123"}'
```

## 5) Inspect deliveries
```bash
slctl datalink deliveries --account "$acct" --limit 10
```

## Notes
- Deliveries are retried per the server’s dispatcher hooks; failed attempts will surface in the channel’s delivery list.
- Workspace wallets gate channel ownership—register all signers up front.
- HTTP equivalents:
  - `POST /accounts/{account}/datalink/channels`
  - `POST /accounts/{account}/datalink/channels/{id}/deliveries`
  - `GET /accounts/{account}/datalink/deliveries?limit=n`
