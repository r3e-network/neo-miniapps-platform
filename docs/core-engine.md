# Service Core Engine

`internal/engine` provides a minimal orchestration layer so every service
module shares a common lifecycle contract. It is intentionally light and
framework-free, so it can be embedded into the existing application runtime
without invasive changes.

## Android-style model
- The Service Engine behaves like a tiny OS: it owns lifecycle, readiness
  probing, module ordering, and the shared buses (data/event/compute).
- Services are applications: they register with the engine, declare their
  domains/capabilities, and implement the standard interfaces instead of
  inventing bespoke lifecycles.
- System APIs are stable: lifecycle/readiness, store/account/compute/data/event
  surfaces are always present and surfaced in `/system/status` under `apis` so
  operators can see exactly which standard OS surfaces a module plugs into.
- Custom APIs are additive: services can advertise extra surfaces, but the
  engine always lists the core ones and enforces bus permissions so modules
  cannot claim surfaces they are not allowed to use.

## Interfaces
- `ServiceModule` — base contract: `Name() string`, `Domain() string`, `Start(ctx)`, `Stop(ctx)`.
- Specialized interfaces embed `ServiceModule` to describe intent:
  - `AccountEngine` — account lifecycle (`CreateAccount`, `ListAccounts`).
  - `StoreEngine` — persistence backends (`Ping`).
  - `ComputeEngine` — execution engine (`Invoke`).
  - `DataEngine` — data-plane push APIs (`Push`).
  - `EventEngine` — pub/sub style (`Publish`, `Subscribe`).
  - Infra-oriented markers: `LedgerEngine`, `IndexerEngine`, `RPCEngine`, `DataSourceEngine`, `ContractsEngine`, `ServiceBankEngine`, `CryptoEngine` (used for status/visibility; methods come from `ServiceModule` unless extended).
- Ordering: use `WithOrder(...)` when constructing the engine to enforce startup
  ordering (e.g., `store-postgres` → `core-application` → services → runners).

## Engine
- Registry with uniqueness checks for module names.
- Deterministic lifecycle: Start in registration order; Stop in reverse order.
- Rollback: if a module fails during Start, already-started modules are stopped.
- Thread-safe with internal locking; accepts an optional logger.
- Recommended lifecycle: engine-owned. Register services with lifecycle enabled so the engine calls their `Start/Stop/Ready`. Avoid mixing another lifecycle manager in this mode to prevent double-start/stop and misleading health.
- Reflection helpers (use only when lifecycle is external): `MarkStarted/MarkStopped/MarkReady` mirror health without invoking hooks. When a module implements `ReadySetter` (`SetReady(status, err)`), the engine will invoke it during probes/mark calls to keep internals aligned.
- Exposed via `/system/status` — module list now includes name, domain, inferred category (`store`, `account`, `compute`, `data`, `event`), lifecycle status (`registered`, `starting`, `started`, `stopped`, `failed`, `stop-error`), readiness (`ready|not-ready|unknown`), timestamps (`started_at`, `stopped_at`, `updated_at`), start/stop durations (`start_nanos`, `stop_nanos`), human-friendly timing summaries (`modules_timings`), uptime (`modules_uptime`), and aggregated counts (`modules_meta`). A `modules_slow` array highlights modules with start/stop > 1s. Configure ordering with `WithOrder`. The payload also echoes the resolved HTTP `listen_addr` when the runtime registers the HTTP service with the engine, making it easy for operators/CLIs to discover ephemeral bind addresses (e.g., `:0`).
- Slow threshold: override with `MODULE_SLOW_MS` or `MODULE_SLOW_THRESHOLD_MS` (milliseconds). `/system/status` echoes `modules_slow_threshold_ms`.
- The appserver also accepts `-slow-ms <ms>` and config supports `runtime.slow_module_threshold_ms` to set the slow threshold declaratively.
- Standard APIs: modules now advertise an `apis` array in `/system/status`, derived
  from the interfaces they implement plus any custom `APIDescriber` entries. This
  is the OS-style contract list (lifecycle/readiness/store/account/compute/data/event)
  so integrators can treat services as apps using the same core surfaces.

## API descriptors
- `APISurface` constants cover the system-level surfaces: `lifecycle`,
  `readiness`, `store`, `account`, `compute`, `data`, and `event`.
- The engine auto-populates `APIDescriptor`s for a module based on implemented
  interfaces and bus permissions. Modules can implement `engine.APIDescriber` to
  append more surfaces (e.g., telemetry, admin). Duplicate names/surfaces are
  deduped with standard system surfaces winning.
- `/system/status` includes `apis` under each module entry so operators and
  dashboards can see which OS-level APIs the module participates in, and
  `modules_api_summary` groups modules by surface (compute/data/event/etc.).

Example `/system/status` payload (truncated):
```
{
  "modules": [
    {"name":"store-postgres","status":"started","ready_status":"ready","start_nanos":1200000,"stop_nanos":800000,"apis":[{"name":"lifecycle","surface":"lifecycle","stability":"stable"},{"name":"store","surface":"store","stability":"stable"}]},
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
- Fan-out helpers: `PushData` fan-outs to `DataEngine`s, `InvokeComputeAll`
  invokes `ComputeEngine`s and returns per-module results, `SubscribeEvent`
  registers with `EventEngine`s plus an in-process bus, and `PublishEvent`
  delivers to `EventEngine`s and local subscribers. Bus fan-out honors per-module
  permissions (events/data/compute) which can be overridden in config; permissions
  are surfaced in `/system/status`.
- Dependencies: declare `runtime.module_deps` to fail fast on missing deps, and
  the engine will topologically sort startup so dependencies start first. Cycles
  or unresolved deps return errors before any modules are started; readiness
  probes also mark dependants as `waiting for dependencies` until their deps are
  started and ready.
- Default deps: runtime seeds reasonable defaults so all modules wait for
  `core-application`/`store-postgres`, runners wait for their parent services,
  and the HTTP module waits for core/store. Override in `runtime.module_deps`
  when you need something different.

## Example
```go
eng := engine.New(engine.WithOrder("store", "accounts", "compute"))
// Register typed modules (implements interfaces shown below).
eng.Register(myStore)    // StoreEngine
eng.Register(myAccounts) // AccountEngine
eng.Register(myCompute)  // ComputeEngine
if err := eng.Start(ctx); err != nil { log.Fatal(err) }
defer eng.Stop(context.Background())

// Typed lookups:
stores := eng.StoreEngines()   // []StoreEngine
accounts := eng.AccountEngines() // []AccountEngine
computes := eng.ComputeEngines() // []ComputeEngine

// Invoke compute:
if len(computes) > 0 {
    res, err := computes[0].Invoke(ctx, map[string]any{"function_id": "fn-1", "account_id": "acct-1", "input": "{}"})
    _ = res; _ = err
}

// Fan-out helpers:
_ = eng.PushData(ctx, "events/new-price", payload)          // pushes to all DataEngines
_, _ = eng.InvokeComputeAll(ctx, map[string]any{"job": 1})  // invokes every ComputeEngine, returns per-module results
_ = eng.SubscribeEvent(ctx, "oracle.fulfilled", handler)    // registers with all EventEngines + in-process bus
_ = eng.PublishEvent(ctx, "oracle.fulfilled", payload)      // publishes to EventEngines and local subscribers
```

## Engine-aware service contracts
Common event/data topics understood by the current services:
- `pricefeed.Publish` expects `event: "observation"` with `account_id`, `feed_id`, `price`, `source`.
- `datafeeds.Publish` expects `event: "update"` with `account_id`, `feed_id`, `price`, `round_id`.
- `oracle.Publish` expects `event: "request"` with `account_id`, `source_id`, `payload`.
- `datalink.Publish` expects `event: "delivery"` with `account_id`, `channel_id`, `payload`, `metadata`.
- `datastreams.Push` expects `topic: <stream_id>` and `payload` as a frame map.
- `functions.Invoke` expects a map with `function_id`, `account_id`, and optional `input`.

Service → interface map (as registered in the runtime adapters):
- `store-postgres` → `StoreEngine` (ready via DB ping)
- `svc-accounts` → `AccountEngine` (Create/List)
- `svc-functions` → `ComputeEngine` (Invoke)
- `svc-datastreams` → `DataEngine` (Push)
- `svc-pricefeed`, `svc-datafeeds`, `svc-oracle`, `svc-datalink` → `EventEngine` (Publish)
- `svc-random` → lifecycle/readiness only (randomness utilities)
- `svc-http` → lifecycle/readiness only (HTTP transport)
- `runner-*` (`automation-scheduler`, `pricefeed-runner`, `oracle-runner`, `gasbank-settlement`) implement lifecycle/readiness only; they do not expose typed interfaces.
- Capability markers: modules can implement `HasAccount/HasCompute/HasData/HasEvent` to opt out of interface advertising/lookups when the adapter provides stubbed methods. The runtime adapter sets these based on attached function pointers to avoid surfacing unsupported interfaces.

### Naming conventions
- Module names are stable, kebab-style: `store-postgres`, `svc-accounts`, `svc-functions`, `svc-datastreams`, `svc-pricefeed`, `svc-datafeeds`, `svc-oracle`, `svc-datalink`, `runner-*`.
- Domains reflect service areas: `store`, `accounts`, `functions`, `datastreams`, `pricefeed`, `datafeeds`, `oracle`, `datalink`, `gasbank`, `automation`.
- If a service exposes `Name()`/`Domain()`, those take precedence when registering with the engine; otherwise defaults above are used. If a duplicate name is detected, the adapter appends the domain (e.g., `svc-foo-accounts`).

### HTTP bus helpers
- `/system/events` — POST `{ "event": "...", "payload": {...} }` to fan-out to all `EventEngine`s.
- `/system/data` — POST `{ "topic": "...", "payload": {...} }` to fan-out to all `DataEngine`s.
- `/system/compute` — POST `{ "payload": {...} }` to invoke every `ComputeEngine`; returns per-module results.

## Integration guidance
- Wrap existing services (e.g., app.Application or individual domain services)
  in small adapters that satisfy `ServiceModule` (and one of the specialized
  interfaces when applicable).
- Keep module names unique and stable; use domains like `accounts`, `oracle`,
  `neo-indexer`, `dashboard`.
- Use the Engine to coordinate startup ordering across persistence, domain
  services, and background workers without cross-wiring constructors.
- For typed lookups, implement the matching interface:
  - `StoreEngine`: `Ping(ctx)` for DB connectivity.
  - `AccountEngine`: `CreateAccount`, `ListAccounts`.
  - `ComputeEngine`: `Invoke(ctx, payload)` to execute work; payload shape is domain-defined.
  - `DataEngine`: `Push(ctx, topic, payload)` for data-plane messages.
  - `EventEngine`: `Publish(ctx, event, payload)` (optional `Subscribe`).
  The engine records supported interfaces per module and surfaces them via `/system/status`.
