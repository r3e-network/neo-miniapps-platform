# Polkadot JAM-inspired Integration for Service Layer

## Goal
Borrow proven ideas from Polkadot's JAM chain to harden the Service Layer's JAM subsystem: make preimage publishing, package processing, and reporting more trustable, metered, and auditable while keeping API ergonomics.

## Quick Primer: What JAM Chain Does Well
- **Content-addressed services**: Everything (code, inputs) is referenced by hash; execution is reproducible and verifiable.
- **Accumulator-first state**: State transitions commit into compact accumulators that can be proven externally.
- **Service isolation + capabilities**: Services interact through narrow interfaces and capability tokens; no ambient authority.
- **Economic metering**: Work is priced by weight; quotas and rate limits are enforced at ingress.
- **Availability + erasure**: Preimages and proofs are widely available (DAS-style) so validators can reconstruct execution.
- **Observability hooks**: Execution receipts, events, and health surfaces for operators to monitor services.

## What to Copy Into Our Service Layer
1) **Content-addressed flow**  
   - Treat preimages, packages, and reports as content-addressed blobs (already hashed) and preserve hash/version in all APIs.  
   - Add optional proof-friendly envelopes (hash of payload, hash of metadata) for downstream verification.

2) **Ingress metering + capabilities**  
   - Keep auth tokens as capability keys (per-service, per-tenant).  
   - Enforce per-token quotas/rate limits (already in place) and add weight hints so heavy packages can be throttled.

3) **Accumulator-style attestations**  
   - Maintain a Merkle/accumulator over accepted packages and reports per service-id and expose roots via `/jam/status`.  
   - Emit lightweight receipts (hash, seq, root, timestamp) that clients can pin or relay.

4) **Deterministic execution contracts**  
   - Define a minimal execution contract for JAM packages (input schema, deterministic transforms) and reject non-deterministic options.  
   - Version execution contracts and include the version in the package envelope to allow replay on upgraded nodes.

5) **Availability strategy**  
   - Keep preimages in configured blobstores (memory/S3/PG).  
   - Add optional erasure-coded sidecar (future) or multi-region replication policies surfaced in status.

6) **Observability + health**  
   - Continue surfacing auth/rate/size/pending caps.  
   - Add per-store metrics (hits/misses, latency), accumulator lag, and per-service quota usage.  
   - Provide CLI verbs to fetch receipts and recent roots.

7) **Compatibility/gradual rollout**  
   - Preserve legacy list responses for existing clients while enabling paginated, filterable, envelope-based listings.

## Proposed Design

### Architecture
- **Ingress layer**: Current HTTP JAM API with middleware for auth, rate-limit, and quota enforcement; extend with weight hints and token-scoped limits.  
- **Preimage/Package store**: Pluggable stores (memory, PG, S3) with content-addressed keys; add receipt accumulator journal (e.g., per service `root_n = H(root_{n-1} || package_hash || metadata_hash)` stored in PG).  
- **Execution/Processing**: Existing package processing path; require deterministic contract version and record execution receipt (status, root, time).  
- **Observability**: Health/status endpoint exposes store type, limits, accumulator heads, and replication policy; metrics exported to Prometheus.

### Data Model Additions
- `accumulator_root`: per service-id, updated on accepted package/report.  
- `receipt`: `{seq, hash, prev_root, new_root, status, processed_at}` persisted and retrievable by hash.  
- `weight_hint`: optional client-provided integer for scheduling/throttling.  
- `capability_id`: derived from auth token to support per-tenant quotas.

### API/CLI Changes (incremental)
- **Status**: Already exposes auth/limits; extend with `accumulator_roots` (map of service-id → root) and replication policy.  
- **Packages**: Default paginated, envelope responses; add `include_receipt=true` to return the new_root and seq.  
- **Reports**: List/report endpoints return envelopes and can optionally include receipt roots.  
- **CLI** (`slctl`): Flags to print receipts (`--with-receipt`), fetch accumulator heads, and check quota usage.

### Processing & Validation
- Reject packages whose declared hash/version do not match computed hash or supported execution contract.  
- Apply rate-limit/quota per token and per service-id (weight-aware).  
- After successful ingest/process, append to accumulator and persist receipt atomically with package/report record.

## Rollout Plan
1. **Schema & config**: Add accumulator tables/columns in PG store; expose config flags for receipts and replication policy.  
2. **API surface**: Extend status/packages/reports endpoints to include roots and receipts; keep legacy responses behind `legacy_list_response`.  
3. **CLI**: Add receipt/roots commands and quota display.  
4. **Processing**: Enforce deterministic contract versioning and weight hints; add per-service accumulators.  
5. **Observability**: Ship metrics and logs; update operator runbook.  
6. **Backfill**: Optional migration to compute roots over existing packages/reports.  
7. **Gradual enablement**: Enable per service-id; monitor latency and storage growth.

## Risks & Mitigations
- **Accumulator correctness**: Use deterministic ordering and atomic writes; add tests for root evolution.  
- **Quota regressions**: Keep `legacy_list_response` and make new limits opt-in by token.  
- **Storage growth**: Bound receipt retention or offload to blobstore; expose counts in status.  
- **Client breakage**: Preserve legacy JSON and pagination defaults; document new envelopes and receipts.

## Open Questions
- Do we need cross-service proofs (aggregate root across services) or per-service roots suffice?  
- Should weight hints be mandatory for large packages to avoid unfair scheduling?  
- Which accumulator shape to standardize on (simple hash chain vs Merkle tree) for proof interoperability?
