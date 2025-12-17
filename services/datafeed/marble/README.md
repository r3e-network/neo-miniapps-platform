# NeoFeeds Marble Service

TEE-secured price aggregation + on-chain anchoring service running inside the
MarbleRun/EGo enclave mesh.

## Responsibilities

- Poll multiple external HTTP sources (default: **Binance**, **Coinbase**, **OKX**) on a fixed interval (default: **1s**).
- Aggregate values via **weighted median**.
- Sign responses with an enclave-held key (`NEOFEEDS_SIGNING_KEY`).
- Optionally push updates on-chain to the platform `PriceFeed` contract (preferred).
- Enforce publish policy defaults aligned with the platform blueprint:
  - **Threshold**: `10 bps` (0.10%)
  - **Hysteresis**: `8 bps` (0.08%)
  - **Min interval**: `5s` (≤ 1 publish / 5s / symbol)
  - **Max rate**: `30/min` per symbol

## Endpoints

- `GET /health`, `GET /info` (provided by the shared `BaseService`)
- `GET /price/{pair}` (canonical: `BTC-USD`, legacy `BTC/USD` accepted)
- `GET /prices` (latest cached prices from storage, when DB is configured)
- `GET /feeds`, `GET /sources`, `GET /config` (introspection)

## Configuration

NeoFeeds can be configured via:

1. A YAML/JSON file (`ConfigFile`), or
2. A programmatic `FeedsConfig` (tests/embedding), or
3. Built-in defaults (`DefaultConfig()`).

### URL & Pair Templating

Each `source.url` supports placeholders:

- `{base}` / `{quote}`: derived from the feed ID (e.g., `BTC-USD`)
- `{pair}`: constructed using `pair_template` (recommended for exchanges)

Per-source overrides are supported:

- `base_override`: replace base symbol for a single source
- `quote_override`: replace quote symbol (e.g., map `USD -> USDT`)

Example:

```yaml
update_interval: 1s
publish_policy:
  threshold_bps: 10
  hysteresis_bps: 8
  min_interval: 5s
  max_per_minute: 30

default_sources: [binance, coinbase, okx]

sources:
  - id: binance
    url: "https://api.binance.com/api/v3/ticker/price?symbol={pair}"
    json_path: price
    pair_template: "{base}{quote}"
    quote_override: USDT

  - id: coinbase
    url: "https://api.coinbase.com/v2/prices/{base}-{quote}/spot"
    json_path: data.amount

  - id: okx
    url: "https://www.okx.com/api/v5/market/ticker?instId={pair}"
    json_path: data.0.last
    pair_template: "{base}-{quote}"
    quote_override: USDT

feeds:
  - id: BTC-USD
    enabled: true
```

## On-Chain Anchoring (PriceFeed)

When `EnableChainPush` is enabled and `PriceFeedHash` is configured, NeoFeeds
periodically evaluates all enabled feeds and anchors qualifying updates on-chain
via `PriceFeed.update(...)`.

Anchoring uses:

- a TEE signer (`ChainSigner`) — ideally backed by `GlobalSigner`, and
- an attestation-derived hash included in the contract record.

The `PriceFeed` contract enforces monotonic `round_id` to prevent replay.

## Optional Chainlink

The codebase contains an optional Chainlink Arbitrum reader. It is **disabled by
default** to keep default behavior aligned with the platform blueprint (3 HTTP
sources + median). To enable it, set `ARBITRUM_RPC` for the `neofeeds` marble
and pass it via `Config.ArbitrumRPC`.

## Required Secrets

- `NEOFEEDS_SIGNING_KEY`: stable signing material for response signatures.

In strict identity / enclave mode, outbound sources must use HTTPS (enforced by
configuration validation).
