# JAM-Inspired Extensions for Service Layer

Design notes for borrowing Join-Accumulate Machine (JAM) patterns and applying them to Service Layer without copying protocol-level details.

Code scaffolding for the core types/interfaces lives in `internal/app/jam`.

## Goals
- Add a structured “work package” pipeline that separates preprocessing from state mutation, with attestations before commit.
- Model deployable units as permissionless services (code + state + balance) instead of only individual functions.
- Provide async inter-service messaging with explicit handlers rather than synchronous calls.
- Introduce content-addressed blobs (preimages) for code/data to reduce payload size and enable reuse.
- Improve metering (compute, storage, bandwidth) with deposits/credits tied to accounts or services.

## JAM Concepts to Reuse (Adapted)
- **Work packages**: batches of work items per service; refined off-chain into compact work reports, then accumulated on-chain.
- **Service encapsulations**: permissionless deployment of code/state/balance; upgrades are versioned, not governance-gated.
- **Entry points**: `refine` (stateless preprocessing), `accumulate` (stateful apply), `on_transfer` (async message handler).
- **Preimages**: content-addressed blobs retrievable by hash during refine/accumulate.
- **Metered VM**: deterministic sandbox with reliable gas metering and continuation support (JAM uses RISC-V PVM).
- **Scheduling**: ticket/epoch-based assignment and pipelined execution to overlap availability and apply phases.

## Proposed Architecture for Service Layer

### Data Model (candidate)
- **Service**: `{id, owner, code_hash, version, state_meta, balance, quotas, created_at, upgraded_at, status}`
- **ServiceVersion**: `{service_id, version, code_hash, migrate_hook, created_at}`
- **WorkPackage**: `{id, service_id, items[], created_by, nonce, expiry, sig, created_at, preimage_hashes[]}`
- **WorkItem**: `{id, package_id, kind, params_hash, preimage_hashes[], max_fee, memo}`
- **WorkReport**: `{id, package_id, service_id, refine_output_hash, refine_output_compact, traces?, created_at}`
- **Attestation**: `{report_id, worker_id, signature, weight, created_at}`
- **Message**: `{id, from_service, to_service, payload_hash, tokens?, status, created_at, available_at}`
- **Preimage**: `{hash, size, media_type, created_at, uploader, storage_class, refcount}`

Hashing: default SHA-256 over canonical JSON or protobuf; keep CBOR option open if payloads grow.

### Entities
- **Service**: encapsulates code (versioned artifact), state (key/value), and balance/credits. Creation is permissionless; capacity is tied to deposits/credits.
- **WorkPackage**: batch of work items targeting one service. Carries content hashes and signatures instead of large payloads.
- **WorkReport**: refined output (compact) plus attestation set that a service’s refine step was executed correctly.

### Work Package Flow (Refine → Accumulate)
1) **Submit package**: client posts a WorkPackage (items reference preimages by hash). Stored in an inbox table/queue.
2) **Refine (off-chain workers)**:
   - Fetch needed preimages; execute `refine` entry point in sandbox.
   - Emit `WorkReport` (compact result) + trace/checksums.
3) **Attest**:
   - Multiple workers/validators attest the same WorkReport hash. Threshold required before apply.
   - If divergence, mark disputed; require manual or quorum resolution before apply.
4) **Accumulate (stateful apply)**:
   - Apply report via `accumulate` entry point with state access. Short time/gas budget per item.
   - Writes to service state and balance; emits events; can create/upgrade services.
5) **Finalize**:
   - Persist state/version, update credits, emit audit log. Support rollback if a later judgment deems a report bad.

### Service Model
- **Deployment**: `POST /services` uploads code artifact (preimage hash) and optional init state; recorded as version 1.
- **Upgrade**: new code hash + migration hook; old versions kept for audit. Deploy/upgrade is permissionless but requires balance for storage and execution.
- **State Access**: only `accumulate` and `on_transfer` can mutate state; `refine` is stateless except for preimage lookups.
- **Balance/Quota**: storage bytes and compute/time budgets priced in gas-bank credits or deposits; exhaustion pauses new packages.

### Messaging (`on_transfer`)
- Services can send messages (with optional token movement) to other services; delivery is async.
- Receiver handles messages in `on_transfer` during the same apply tick; no synchronous return path. If a reply is needed, sender must enqueue another message.
- Backed by an internal message queue (can reuse datalink/datastream infrastructure) with retries and dead-letter handling.

### Preimage / Blob Store
- Content-addressed store (sha256) for code and large payloads.
- API: `PUT /preimages/{hash}` to upload blob; `HEAD/GET` to retrieve. Packages carry hashes; refine fetches blobs on demand.
- Enables deduplication, smaller packages, and late binding of code/data.
- Implementations: in-memory and Postgres-backed stores in `internal/app/jam` (see `preimage_store.go`) plus `jam_preimages` table migration.
- Prototype HTTP mux for JAM submission/processing lives in `internal/app/jam/http.go` (enqueue package, upload preimage, process next, fetch report).

### Enabling the prototype HTTP API
- Set `JAM_ENABLED=1` to mount `/jam/*` endpoints on the main HTTP server. By default they stay disabled.
- Optional: `JAM_STORE=postgres` to back work packages/preimages with Postgres instead of memory. Supply `JAM_PG_DSN` or `DATABASE_URL` for the connection; migrations run automatically.
- Endpoints: `PUT/GET /jam/preimages/{hash}`, `POST /jam/packages`, `POST /jam/process`, `GET /jam/packages/{id}`, `GET /jam/packages/{id}/report`.
- Config snippet (YAML):
  ```yaml
  runtime:
    jam:
      enabled: true
      store: postgres   # or memory
      pg_dsn: postgres://user:pass@localhost:5432/service_layer?sslmode=disable
  ```

### Compute Sandbox
- Keep Wasm executor for compatibility; prototype a RISC-V sandbox (e.g., rvemu or wasmtime-riscv) for better continuation support and metering.
- Enforce per-phase gas/time: `refine` (longer, e.g., seconds), `accumulate` (tens of ms), `on_transfer` (similar to accumulate).
- Allow spawning child contexts for parallelizable subtasks; propagate metering.

### Scheduling & Availability
- Ticket/epoch-style assignment: pre-select workers to refine specific packages to reduce coordination latency.
- Pipelined apply: overlap availability checks with execution by treating state root as “prior” for a block/tick and committing reports in the next tick.
- Networking: for large clusters, prefer QUIC full-mesh between workers/validators and grid diffusion for large blobs; otherwise reuse current HTTP/gRPC paths.

### API Surface (sketch)
- `POST /services` → create service (code hash + init params); returns service id/version.
- `POST /services/{id}/versions` → upgrade code hash + optional migrate hook.
- `GET /services/{id}` → metadata, quotas, versions.
- `PUT /preimages/{hash}` / `GET /preimages/{hash}` → content-addressed blob store.
- `POST /services/{id}/packages` → submit WorkPackage; references preimages by hash.
- `POST /packages/{id}/reports` → submit WorkReport (refine output) + attestation.
- `POST /reports/{id}/attestations` → add attestation (if we separate report and attest submission).
- `POST /services/{id}/messages` → send async message to another service.
- `GET /services/{id}/inbox` → list pending messages/work packages.
- `POST /admin/disputes/{report_id}` → flag dispute; `POST /admin/rollback/{state_id}` → controlled rollback (feature-gated).
CLI should wrap these for operators and developers.

### Storage Layout (Go/SQL sketch)
- `services` table keyed by uuid; `service_versions` table keyed by (service_id, version).
- `work_packages` table keyed by uuid; `work_items` keyed by (package_id, seq).
- `work_reports` keyed by package_id; `attestations` keyed by (report_id, worker_id).
- `messages` keyed by uuid with status index (pending, delivered, dlq).
- `preimages` keyed by hash with refcount and size; backing store could be filesystem/S3-compatible bucket.
- State backend: re-use existing store abstraction; add per-service namespace (prefix).

### Worker/Validator Roles
- **Refine workers**: stateless; pull packages, fetch preimages, execute `refine`, produce report hash and compact output.
- **Attestors**: sign report hash; require quorum/weight threshold. Could be same pool as refine workers.
- **Accumulate executor**: trusted runtime that applies reports once attested; can be replicated with leader election to avoid double-apply.
- **Dispute resolver**: on divergence, run deterministic re-check; optionally slash misbehaving workers in future.

### Security/Integrity
- Signatures: packages signed by submitter; reports/attestations signed by workers. Use existing auth tokens + ed25519 keys.
- Determinism: sandbox must be deterministic given inputs + preimages; bake in engine version into hashes to avoid replay after upgrades.
- Rate limits/quotas: enforced per service id; refuse packages if credits low.
- Rollback: keep state checkpoints per apply tick; allow bounded rollback window and compensating actions.

### Telemetry/Observability
- Metrics: counts and latency per phase (submit/refine/attest/accumulate), success vs dispute, bytes per preimage, gas consumed, queue depth.
- Tracing: trace ids per package/report; include worker ids.
- Audit log: immutable log of service upgrades, report applies, rollbacks.

## Integration Roadmap
- **Phase 0: Spike** – Define WorkPackage/WorkReport schemas; decide hash algorithm; mock refine/accumulate interfaces in Go.
- **Phase 1: Preimage Store** – Ship content-addressed blob API + storage pricing and integrate hashes into function payloads.
- **Phase 2: Service Encapsulation** – Add service registry (code+state+balance), permissionless deploy/upgrade, and quotas tied to gas-bank credits.
- **Phase 3: Work Pipeline** – Implement refine workers, attestation quorum, and accumulate apply path with rollback hooks; expose APIs/CLI for submit/inspect.
- **Phase 4: Messaging** – Add async message queue and `on_transfer` handler wiring between services.
- **Phase 5: Sandbox** – Evaluate RISC-V executor; add metering limits per entry point; benchmark against current Wasm runtime.
- **Phase 6: Scheduling** – Experiment with ticket-based worker assignment and pipelined commit; add metrics and toggles.

## Open Questions
- Who attests? Reuse validators, introduce a worker set, or rely on external oracles?
- How to price storage and execution (gas-bank credits vs. explicit deposits)? What are default quotas?
- Rollback policy: how far back can we revert if a report is judged bad? Do we need compensating actions?
- Do services need capabilities/ACLs to read other services’ state, or is it open read/guarded write?
- How to align with existing HTTP API and CLI surfaces without breaking current users? Need versioned endpoints?
- Governance/controls: do we gate service creation/upgrades behind configurable policies in regulated environments?
- Dispute incentives: do attestors get paid/slashed? What funds slashing pools?
