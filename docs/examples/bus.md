# Engine Bus Quickstart

Use the core engine bus to fan-out events/data/compute across all registered
services. These endpoints require an API token or login.

## Endpoints
- `POST /system/events` — `{ "event": "<name>", "payload": {...} }`
- `POST /system/data` — `{ "topic": "<topic>", "payload": {...} }`
- `POST /system/compute` — `{ "payload": {...} }`

## Quick commands (default dev-token)
```bash
# Publish a pricefeed observation
curl -H "Authorization: Bearer dev-token" \
  -X POST http://localhost:8080/system/events \
  -d '{"event":"observation","payload":{"account_id":"<acct>","feed_id":"<feed>","price":"123.45","source":"manual"}}'

# Publish a datafeed round update
curl -H "Authorization: Bearer dev-token" \
  -X POST http://localhost:8080/system/events \
  -d '{"event":"update","payload":{"account_id":"<acct>","feed_id":"<feed>","price":"123.45","round_id":1}}'

# Queue a datalink delivery
curl -H "Authorization: Bearer dev-token" \
  -X POST http://localhost:8080/system/events \
  -d '{"event":"delivery","payload":{"account_id":"<acct>","channel_id":"<channel>","payload":{"hello":"world"},"metadata":{"trace":"demo"}}}'

# Push a datastream frame
curl -H "Authorization: Bearer dev-token" \
  -X POST http://localhost:8080/system/data \
  -d '{"topic":"<stream-id>","payload":{"price":123,"ts":"'$(date -u +"%Y-%m-%dT%H:%M:%SZ")'"}}'

# Fan-out a compute invoke (functions service)
curl -H "Authorization: Bearer dev-token" \
  -X POST http://localhost:8080/system/compute \
  -d '{"payload":{"function_id":"<fn>","account_id":"<acct>","input":{"foo":"bar"}}}'
```

## CLI shortcuts
```bash
slctl --token dev-token bus events --event observation --payload '{"account_id":"<acct>","feed_id":"<feed>","price":"123.45","source":"cli"}'
slctl --token dev-token bus data --topic <stream-id> --payload '{"price":123}'
slctl --token dev-token bus compute --payload '{"function_id":"<fn>","account_id":"<acct>","input":{"foo":"bar"}}'
# Fan-out health (prefer Prometheus when available; falls back to /system/status totals)
slctl --token dev-token bus stats --prom-url http://localhost:9090 --range 10m
slctl --token dev-token bus stats   # uses /system/status bus_fanout totals if Prom is unset
```

## Expected payload shapes
- `pricefeed.Publish`: `event=observation` with `account_id`, `feed_id`, `price`, `source`.
- `datafeeds.Publish`: `event=update` with `account_id`, `feed_id`, `price`, `round_id`.
- `oracle.Publish`: `event=request` with `account_id`, `source_id`, `payload`.
- `datalink.Publish`: `event=delivery` with `account_id`, `channel_id`, `payload`, `metadata`.
- `datastreams.Push`: `topic=<stream_id>` with frame payload map.
- `functions.Invoke`: payload map with `function_id`, `account_id`, optional `input`.

### Dashboard console
- Open the “Engine Bus Console” card (dashboard). Supports presets for observation/update/request/delivery, stream frame, and compute invoke. Requires token/login.
  - Event mapping:
    - `observation` → pricefeed observation (`account_id`, `feed_id`, `price`, optional `source`).
    - `update` → datafeed round (`account_id`, `feed_id`, `price`, `round_id`).
    - `request` → oracle request (`account_id`, `source_id`, `payload` as JSON/string).
    - `delivery` → datalink delivery (`account_id`, `channel_id`, `payload`, optional `metadata`).
  - Data mapping: `topic` should be a data stream ID; payload is a frame map (e.g., `{ "price": 123, "ts": "..." }`).
  - Compute mapping: payload `{"function_id":"...","account_id":"...","input":{...}}` invokes all compute engines (functions service).

### Observability
- `/system/status` exposes `bus_fanout` totals (ok/error per kind) for quick checks without Prometheus.
- Prometheus counter: `service_layer_engine_bus_fanout_total{kind,result}`. See `docs/alerts.md` for alert examples.
## Engine Status Examples

Fetch `/system/status` to inspect modules, timings, uptimes, and slow modules:

```bash
curl -H "Authorization: Bearer $API_TOKENS" http://localhost:8080/system/status
```

Example (truncated):
```json
{
  "modules": [
    {"name":"store-postgres","status":"started","ready_status":"ready","start_nanos":1200000,"stop_nanos":800000},
    {"name":"svc-automation","status":"failed","error":"boom","ready_status":"not-ready"}
  ],
  "modules_meta":{"total":2,"started":1,"failed":1,"stop_error":0,"not_ready":1},
  "modules_timings":{"store-postgres":{"start_ms":1.2,"stop_ms":0.8}},
  "modules_uptime":{"store-postgres":42.3},
  "modules_slow":["store-postgres"],
  "modules_slow_threshold_ms":1000,
  "listen_addr":"127.0.0.1:8080"
}
```

- Slow threshold can be set via `runtime.slow_module_threshold_ms`, env (`MODULE_SLOW_MS` / `MODULE_SLOW_THRESHOLD_MS`), or `appserver -slow-ms`. The active value is echoed as `modules_slow_threshold_ms`.
- `slctl status` renders the same data with module tables, timings, uptime, and slow-module markers.

CLI example:
```bash
slctl status --addr http://localhost:8080 --token dev-token
# Filter/exports:
# slctl status --surface compute --export modules.csv --token dev-token
# outputs:
# Status: ok
# Version: 1.0.0 (commit abc123, built 2025-01-01T00:00:00Z, go1.22.0)
# Listen Address: 127.0.0.1:8080
# Module summary: total=4 started=3 failed=1 stop_error=0 not_ready=1
# Slow modules (>1000ms start/stop): store-postgres
# Modules:
# NAME                DOMAIN      CATEGORY    INTERFACES      STATUS      READY   ERROR
# store-postgres      store       store                       started (slow)      ready
# ...
```
