# Devpack Function Example

This example shows how to author a Service Layer function in TypeScript using
the `@service-layer/devpack` helpers. The function ensures a gas bank account,
creates an oracle request, and returns a structured response. Companion SDKs in
Go, Rust, and Python expose the same Devpack action surface if you prefer a
different language runtime.

See `js/pricefeed_snapshot.js` for a Devpack example that records price feed
snapshots from a function, `js/random_generate.js` for randomness, and
`js/data_pipeline.js` for coordinating data feeds, data streams, and DataLink
from a single function. TypeScript equivalents live under `src/pricefeed.ts`
and `src/random.ts` if you prefer typed authoring.

Refer to [`docs/requirements.md`](../../../docs/requirements.md) for the full API
contract (inputs, outputs, Devpack action semantics) before modifying or extending
the sample.

## Prerequisites

- Node.js 20+
- The repository cloned locally (the example depends on the in-repo SDK)

## Install

```bash
cd examples/functions/devpack
npm install
```

## Build

```bash
npm run build
```

The compiled JavaScript is emitted to `dist/function.js` (and other compiled
entries such as `dist/pricefeed.js` or `dist/random.js` for additional examples).
Copy the function body into the Service Layer when registering your function
definition.

## Notes

- The example imports `@service-layer/devpack` via a workspace file reference
  (`file:../../sdk/devpack`). When the SDK is published you can replace this with
  the registry version.
- Set `TEE_MODE=mock` locally if you want to execute the function without the
  hardware-backed TEE.
- Use `go run ./cmd/slctl functions execute ...` to invoke the function via the
  CLI once the appserver is running.

## Plain JavaScript Examples

If you prefer to author raw JavaScript, the `js/` directory contains ready-to-use
functions that demonstrate common workflows (gas bank funding, oracle requests,
price feed snapshots, randomness, and automation orchestration). Each file exports
a single function and assumes the `Devpack` global is available at runtime. Keep
these samples aligned with the specification whenever APIs evolve.
