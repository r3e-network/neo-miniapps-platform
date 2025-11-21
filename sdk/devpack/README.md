# Service Layer Devpack SDK

This package provides TypeScript helpers for writing Service Layer functions
that target the Neo N3 execution environment. It mirrors the `Devpack` global
that is injected at runtime, making it easier to build, type-check, and bundle
functions locally before uploading them to the platform. Matching Devpack action
helpers exist in `sdk/go/devpack`, `sdk/rust/devpack`, and `sdk/python/devpack`
for polyglot authoring.

## Documentation

- Behaviour, inputs, and guarantees for every helper live in
  [`docs/requirements.md`](../../docs/requirements.md) under the "Functions Runtime"
  sections. Update that spec before changing public APIs.
- Use [`docs/README.md`](../../docs/README.md) as the entry point for locating
  architecture, operations, or Devpack-specific guidance.

## Requirements

- Node.js 20+ / npm 10+ (aligns with the dashboard toolchain)
- TypeScript 5+

## Installation

```bash
npm install @service-layer/devpack
```

The package exposes fully typed wrappers around the Devpack modules. Use your
favourite bundler (esbuild, Rollup, Webpack, etc.) to compile your function into
the single JavaScript snippet that the Service Layer expects.

## Usage

```ts
import {
  ensureGasAccount,
  createOracleRequest,
  recordPriceSnapshot,
  respond,
} from "@service-layer/devpack";

export default function handler(params: Record<string, unknown>) {
  ensureGasAccount({ wallet: String(params.wallet ?? "") });

  createOracleRequest({
    dataSourceId: String(params.oracleSource),
    // optional: provide alternates for median/quorum aggregation
    alternateSourceIds: params.altSources as string[] | undefined,
    payload: { pair: params.pair },
  });

  return respond.success({
    pair: params.pair,
    initiatedAt: new Date().toISOString(),
  });

  // Optionally record a price snapshot for offline sources
  recordPriceSnapshot({
    feedId: String(params.feedId ?? ""),
    price: Number(params.price ?? 0),
    source: "manual",
  });
}
```

The emitted execution record will include the queued actions (`gasbank.ensureAccount` and
`oracle.createRequest`) alongside the response object.

## Exposed Helpers

| Helper | Description |
| ------ | ----------- |
| `ensureGasAccount(params)` | Queue `gasbank.ensureAccount`. |
| `withdrawGas(params)` | Queue `gasbank.withdraw` (supports `scheduleAt` RFC3339 timestamps; cron is not supported). |
| `balanceGasAccount(params)` | Queue `gasbank.balance`. |
| `listGasTransactions(params)` | Queue `gasbank.listTransactions`. |
| `createOracleRequest(params)` | Queue `oracle.createRequest` (supports `alternateSourceIds` for multi-source aggregation). |
| `recordPriceSnapshot(params)` | Queue `pricefeed.recordSnapshot` with `feedId`, `price`, optional `source`, and `collectedAt`. |
| `submitDataFeedUpdate(params)` | Queue `datafeeds.submitUpdate` with `feedId`, `roundId`, `price`, optional `timestamp`, signer, signature, metadata. |
| `publishDataStreamFrame(params)` | Queue `datastreams.publishFrame` with `streamId`, `sequence`, optional `payload`, `latencyMs`, `status`, metadata. |
| `createDataLinkDelivery(params)` | Queue `datalink.createDelivery` with `channelId`, `payload`, optional `metadata`. |
| `generateRandom(params)` | Queue `random.generate` (defaults to 32 bytes; optional `requestId`). |
| `registerTrigger(params)` | Queue `triggers.register`. |
| `scheduleAutomation(params)` | Queue `automation.schedule`. |
| `respond.success(data, meta)` | Build a success payload. |
| `respond.failure(error, meta)` | Build a failure payload. |
| `context` / `currentContext()` | Inspect runtime metadata (`functionId`, `accountId`, etc.). |

All helpers return an action handle that can be converted into a structured
reference via `.asResult(meta)`, should you need to include metadata in your
own outputs.

## Local Execution

- Set `TEE_MODE=mock` when starting the Service Layer locally to disable the TEE
  and use the mock executor. This keeps the Devpack API identical while skipping
  confidential compute during development.
- The CLI (`cmd/slctl`) automatically loads compiled functions, so you can use
  `go run ./cmd/slctl functions execute ...` to exercise handlers after `npm run build`.
