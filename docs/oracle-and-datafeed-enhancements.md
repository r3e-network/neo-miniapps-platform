# Oracle & Data Feed Enhancements

This note captures gaps and desired improvements to bring the Oracle and Data
Feed services closer to production-grade, Chainlink-style behaviours.

## Oracle
- **Lifecycle**: Requests must carry TTL/expiration, retry/backoff, and DLQ
  handling. Prevent zombie pending/running requests and allow manual retry.
- **Aggregation**: Support multiple sources per request with configurable
  aggregation (e.g., threshold/median/quorum) and per-source weights.
- **Auth & Safety**: Require authenticated runners/resolvers to mark requests
  running/complete. Allow outbound host allowlists and per-source headers.
  Validate payload/result size and schema/version fields.
- **Ownership**: Enforce per-account ownership when marking running/complete;
  attach idempotency keys to avoid duplicate completions.
- **Observability**: Emit per-request latency/success metrics, status
  timestamps, and SLA windows. Include trace IDs for resolver callbacks.
- **Attestation**: Optionally sign results (ed25519/ecdsa) for downstream
  consumers and include chain target metadata (chain ID, job/spec ID, response
  format).

## Data Feeds
- **Signer Verification**: Verify cryptographic signatures against the configured
  signer set. Enforce minimum signer thresholds per round.
- **Aggregation**: Aggregate multiple submissions per round (median/mean/quorum)
  instead of accepting a single submission. Reject rounds that do not meet the
  threshold.
- **Validation**: Enforce numeric price/decimals/unit validation, heartbeat and
  deviation enforcement, timestamp sanity, and replay protection per
  signer/round.
- **States**: Support feed active/paused states and stale-feed detection.
- **Observability**: Emit metrics for submission latency, stale/under-signed
  rounds, signer participation, and deviations. Alert on missing heartbeats.
- **Attestation**: Expose (or store) signed reports/attestations for
  chain-facing publication when applicable.

## Next Steps
1) Extend `docs/requirements.md` oracle/datafeed sections with the above
   behaviours and operational expectations.
2) Add runtime flags/settings for oracle TTL/backoff/DLQ and datafeed
   aggregation/signer thresholds.
3) Implement signer verification and aggregation in the data feed service.
4) Add oracle runner authentication, TTL/backoff, and aggregation support.
5) Surface health/metrics in the dashboard and CLI (queues, attempts, signer
   thresholds, stale feeds).

## Implementation Plan (proposed)
1. **Config & Model**: Add runtime/config fields for oracle TTL/retries/DLQ and
   datafeed signer thresholds/aggregation strategy; extend domain models with
   signer identity, signatures, and request expiry metadata.
2. **Data Feed Enforcement**:
   - Verify signatures against signer set; reject unknown signers.
   - Require a configurable minimum signer threshold and aggregate (median) per
     round; enforce deviation/heartbeat and numeric validation.
   - Record per-signer submissions to prevent replay; emit metrics.
3. **Oracle Reliability**:
   - Add request TTL/backoff/DLQ + manual retry; persist attempts.
   - Require runner/resolver auth to mark running/complete; support multi-source
     aggregation (median/quorum) with per-source weights.
4. **Observability & Surfacing**:
   - Metrics for latencies, success/failure, stale feeds, signer participation,
     pending/running queues; trace IDs on oracle callbacks.
   - Dashboard/CLI views for oracle queue/attempts and datafeed signer/round
     status; alerts for stale/under-signed feeds.
5. **Attestation/Chain Output**:
   - Define optional signed report/attestation format (ed25519/ecdsa) for oracle
     results and aggregated feed rounds; include chain ID/job/spec metadata.
