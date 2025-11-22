# JAM Performance & Benchmark Plan

Purpose: outline how to benchmark JAM components (preimages, package submit, refine/accumulate pipeline) to guide tuning and detect regressions.

## Targets
- Preimage PUT/GET throughput and latency (DB vs S3).
- Package submit/list latency under load.
- Refine/accumulate pipeline throughput with different engines (hash-refiner, Wasm, RISC-V prototype).
- Rate-limit/quota overhead impact.

## Scenarios
- **Preimage**
  - Upload 1KB, 1MB, 10MB blobs; measure p50/p90 latency, success rate.
  - GET/HEAD latency for cached and cold paths.
  - Compare DB vs S3 storage (if implemented).
- **Packages**
  - Submit packages at 10–100 req/s; observe pending count, submit latency.
  - List packages with limit/offset; measure response size and time.
- **Processing**
  - Process loop consuming pending packages; measure packages/min, failures, report sizes.
  - Engine comparison: mock/hash refiner vs Wasm vs RISC-V (if available).
- **Rate/Quotas**
  - Rate-limit enabled: ensure 429 under burst; measure added latency when under limit.
  - Quota enforcement: preimage cap and pending cap overhead.

## Tooling
- Go benchmarks and load generators (e.g., vegeta/hey) against httptest server (memory) and real server (PG).
- Bench harness in `internal/app/jam/bench/` to simulate submit/process loops.
- Metrics capture via Prometheus test registry; log summaries.

## Metrics to Collect
- Latency distributions (p50/p90/p99) per endpoint.
- Throughput (req/s, packages/min).
- Error rates (4xx/5xx).
- CPU/memory usage for in-process benchmarks.
- DB metrics: query timings for submit/list/process (PG).
- Object store metrics (S3) if enabled.

## Baselines
- Memory mode: minimal overhead; target <5ms p50 for package submit/list; preimage PUT p50 <10ms for small blobs.
 - PG mode: expect higher latencies; set baseline after first run.

## Regression Triggers
- Latency increase >20% p90 compared to baseline.
- Error rates >1% under target load.
- Pending queue growth without corresponding process throughput.

## Reporting
- Store benchmark runs as artifacts (JSON/CSV) with config: store type, limits, engine, commit hash.
- Include summary in PRs when touching JAM hot paths.
