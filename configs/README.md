# Configuration Reference

All configuration expectations, required environment variables, and runtime flags
are defined in the [`Neo Service Layer Specification`](../docs/requirements.md).
This directory simply contains samples you can copy when bootstrapping local
environments.

## Files
- `config.yaml` – canonical YAML sample consumed by `cmd/appserver`. Uncomment or
  edit sections before passing `-config configs/config.yaml`.
- `examples/appserver.json` – JSON version used in documentation snippets.
- `prometheus.yml` – example scrape config aligned with the `/metrics` surface.

Always update the specification first when adding/removing configuration fields. File
paths are workspace-relative; you can use absolute paths if preferred.

## Runtime Block

The `runtime` section consolidates what used to be scattered environment
variables for the orchestration runtime:

- `tee.mode` — selects between the mock executor (`"mock"`) and the enclave
  executor (`"enclave"`, or leave empty).
- `random.signing_key` — optional ed25519 private key for deterministic random
  responses (base64 or hex encoded).
- `pricefeed.fetch_url` / `pricefeed.fetch_key` — configure the background price
  feed refresher HTTP fetcher.
- `gasbank.resolver_url` / `gasbank.resolver_key` — configure the settlement
  poller HTTP resolver, with `gasbank.poll_interval` and `gasbank.max_attempts`
  controlling retry cadence.
- `oracle.ttl_seconds` / `oracle.max_attempts` / `oracle.backoff` /
  `oracle.dlq_enabled` — control resolver retry cadence and expiry; exhausted
  requests are dead-lettered when enabled.
- `oracle.runner_tokens` — optional list of runner callback tokens; when set,
  status updates must present a matching `X-Oracle-Runner-Token` (API tokens
  are still required). Environment overrides (`ORACLE_RUNNER_TOKENS`) accept
  comma- or semicolon-separated lists.
- `datafeeds.min_signers` / `datafeeds.aggregation` — defaults for Chainlink
  data feed submissions (strategies: `median`, `mean`, `min`, `max`).
- `datastreams` / `datalink` — currently rely on service defaults; no runtime
  toggles are required.
- `cre.http_runner` — enables the HTTP CRE runner integration.

Every field still honours its corresponding environment variable, so you can
mix-and-match config files and env overrides as needed.

## Auth Block

Use the `auth` section to declare static bearer tokens consumed by the HTTP
gateway. Populate `tokens` with one or more strings. Environment variables
(`API_TOKENS` / `API_TOKEN`) and the `-api-tokens` flag continue to override or
supplement the list when necessary. If no tokens are configured, JWT logins are
still accepted when `AUTH_USERS` + `AUTH_JWT_SECRET` are set; otherwise all
protected endpoints return 401 and only `/healthz` + `/system/version` remain
public for probes/discovery. Local defaults set `API_TOKENS=dev-token` and
`AUTH_USERS=admin:changeme:admin` for a quick compose experience—override both
for any shared environment. JWT tokens and static tokens can be supplied via
`Authorization: Bearer ...` or `?token=` query parameters.

## Auditing
- The HTTP layer keeps a rolling in-memory audit buffer (latest 300 entries).
- To persist audits, set `AUDIT_LOG_PATH=/var/log/service-layer-audit.jsonl` (JSONL output).
- Admin-only `/admin/audit?limit=200` returns recent entries; the dashboard Admin panel renders the most recent 20. Admin JWT is required (token-only auth is not admin).

## Security Block

The `security` section holds the AES key used to encrypt secrets at rest. The
`secret_encryption_key` accepts raw, base64, or hex encodings for 16/24/32 byte
keys. When persistence is enabled (PostgreSQL stores), this field or the
`SECRET_ENCRYPTION_KEY` environment variable must be set.
