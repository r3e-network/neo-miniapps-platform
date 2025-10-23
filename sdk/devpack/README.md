# Service Layer Devpack SDK

This package provides TypeScript helpers for writing Service Layer functions
that target the Neo N3 execution environment. It mirrors the `Devpack` global
that is injected at runtime, making it easier to build, type-check, and bundle
functions locally before uploading them to the platform.

## Installation

```bash
npm install @service-layer/devpack
```

The package exposes fully typed wrappers around the Devpack modules. Use your
favourite bundler (esbuild, Rollup, Webpack, etc.) to compile your function into
the single JavaScript snippet that the Service Layer expects.

## Usage

```ts
import { ensureGasAccount, createOracleRequest, respond } from "@service-layer/devpack";

export default function handler(params: Record<string, unknown>) {
  ensureGasAccount({ wallet: String(params.wallet ?? "") });

  createOracleRequest({
    dataSourceId: String(params.oracleSource),
    payload: { pair: params.pair },
  });

  return respond.success({
    pair: params.pair,
    initiatedAt: new Date().toISOString(),
  });
}
```

The emitted execution record will include the queued actions (`gasbank.ensureAccount` and
`oracle.createRequest`) alongside the response object.

## Exposed Helpers

| Helper | Description |
| ------ | ----------- |
| `ensureGasAccount(params)` | Queue `gasbank.ensureAccount`. |
| `withdrawGas(params)` | Queue `gasbank.withdraw`. |
| `createOracleRequest(params)` | Queue `oracle.createRequest`. |
| `registerTrigger(params)` | Queue `triggers.register`. |
| `scheduleAutomation(params)` | Queue `automation.schedule`. |
| `respond.success(data, meta)` | Build a success payload. |
| `respond.failure(error, meta)` | Build a failure payload. |
| `context` / `currentContext()` | Inspect runtime metadata (`functionId`, `accountId`, etc.). |

All helpers return an action handle that can be converted into a structured
reference via `.asResult(meta)`, should you need to include metadata in your
own outputs.

## Mock Execution

Set `TEE_MODE=mock` when starting the Service Layer locally to disable the TEE
and use the mock executor. This keeps the Devpack API identical while skipping
confidential compute during development.

