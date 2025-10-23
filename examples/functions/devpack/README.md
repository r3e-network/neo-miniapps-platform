# Devpack Function Example

This example shows how to author a Service Layer function in TypeScript using
the `@service-layer/devpack` helpers. The function ensures a gas bank account,
creates an oracle request, and returns a structured response.

## Prerequisites

- Node.js 18+
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

The compiled JavaScript is emitted to `dist/function.js`. Copy the function body
into the Service Layer when registering your function definition.

## Notes

- The example imports `@service-layer/devpack` via a workspace file reference
  (`file:../../sdk/devpack`). When the SDK is published you can replace this with
  the registry version.
- Set `TEE_MODE=mock` locally if you want to execute the function without the
  hardware-backed TEE.

## Plain JavaScript Examples

If you prefer to author raw JavaScript, the `js/` directory contains ready-to-use
functions that demonstrate common workflows (gas bank funding, oracle requests,
and automation orchestration). Each file exports a single function and assumes
the `Devpack` global is available at runtime.
