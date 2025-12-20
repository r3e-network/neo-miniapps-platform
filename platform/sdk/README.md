# MiniApp SDK

This package is the TypeScript SDK matching the platform blueprint:

- MiniApps do not talk to the chain directly.
- Calls go to **Supabase Edge**, which enforces policy and either:
  - returns an invocation for the wallet to sign (user-signed flows), or
  - forwards to a TEE service (RNG/compute/oracle).

Code lives under `platform/sdk/src/`.

## Usage

```ts
import { createHostSDK, createMiniAppSDK } from "@neo-miniapp/sdk";

const sdk = createMiniAppSDK({
  edgeBaseUrl: "https://<project>.supabase.co/functions/v1",
  getAuthToken: async () => "<supabase-jwt>",
});

await sdk.payments.payGAS("my-app", "1.5", "entry fee");
await sdk.governance.vote("my-app", "proposal-1", "10", true);
await sdk.rng.requestRandom("my-app");
await sdk.datafeed.getPrice("BTC-USD");
```

Notes:

- `payGAS` / `vote` return an `invocation` intent plus a `request_id`. The host (or wallet integration) should sign and submit the invocation.
- This SDK also exposes:
  - `sdk.wallet.invokeInvocation(invocation)` (NeoLine N3 integration)
  - `sdk.wallet.invokeIntent(request_id)` for intents created during this session
  - `sdk.payments.payGASAndInvoke(...)` / `sdk.governance.voteAndInvoke(...)` convenience helpers

## Oracle (Host-only)

NeoOracle is an allowlisted HTTP fetch service that can inject user secrets for auth.

The gateway endpoint is `oracle-query` (Supabase Edge), which forwards to the TEE service.

Host-only endpoints require an API key with explicit scopes in production.

```ts
const host = createHostSDK({
  edgeBaseUrl: "https://<project>.supabase.co/functions/v1",
  getAPIKey: async () => "<host-api-key>",
});

const res = await host.oracle.query({
  url: "https://api.coingecko.com/api/v3/simple/price?ids=neo&vs_currencies=usd",
});
console.log(res.status_code, res.body);
```

## Compute (Host-only)

NeoCompute executes restricted scripts inside the enclave. These endpoints are
host-only and require API-key auth (and typically a primary wallet binding).

```ts
const host = createHostSDK({
  edgeBaseUrl: "https://<project>.supabase.co/functions/v1",
  getAPIKey: async () => "<host-api-key>",
});

const job = await host.compute.execute({
  script: "function main() { return { now: Date.now(), x: input.x }; }",
  entry_point: "main",
  input: { x: 123 },
});
console.log(job.job_id, job.status);
```

## Automation (Host-only)

NeoFlow manages user triggers (currently cron + webhook execution in the service).

```ts
const host = createHostSDK({
  edgeBaseUrl: "https://<project>.supabase.co/functions/v1",
  getAPIKey: async () => "<host-api-key>",
});

const trigger = await host.automation.createTrigger({
  name: "Every 5 minutes",
  trigger_type: "cron",
  schedule: "*/5 * * * *",
  action: { type: "webhook", url: "https://hooks.miniapps.com/callback", method: "POST" },
});

const executions = await host.automation.listExecutions(trigger.id, 25);
console.log(trigger.enabled, executions.length);
```

## Wallet Binding (OAuth-first onboarding)

When a user logs in via Supabase OAuth, the platform can require them to bind a
Neo N3 address before using on-chain services:

```ts
const host = createHostSDK({
  edgeBaseUrl: "https://<project>.supabase.co/functions/v1",
  getAuthToken: async () => "<supabase-jwt>",
});

const { nonce, message } = await host.wallet.getBindMessage();

// Host app: ask wallet to sign `message` and provide publicKey+signature
await host.wallet.bindWallet({
  address: "<neo-n3-address>",
  publicKey: "<hex or base64>",
  signature: "<hex or base64>",
  message,
  nonce,
  label: "Primary",
});
```

## Secrets (Host-only)

Secrets are host-only and should not be exposed to untrusted MiniApps:

Host-only endpoints require an API key with explicit scopes in production.

```ts
await host.secrets.upsert("binance_api_key", "<secret-value>");
await host.secrets.setPermissions("binance_api_key", ["neooracle"]);
const list = await host.secrets.list();
```

## App Submission (Host-only)

Developer app registration is wallet-signed and routed via Supabase Edge:

```ts
const manifest = {
  app_id: "com.miniapps.arcade",
  entry_url: "https://cdn.miniapps.com/apps/neo-game/index.html",
  name: "Neo Arcade",
  version: "1.0.0",
  developer_pubkey: "0x" + "<33-byte compressed pubkey hex>",
  permissions: { payments: true, governance: false, rng: true, datafeed: true },
  assets_allowed: ["GAS"],
  governance_assets_allowed: ["NEO"],
  sandbox_flags: ["no-eval", "strict-csp"],
  attestation_required: true,
};

const res = await host.apps.register({ manifest });
// Host app: build/sign tx using res.invocation (NeoLine/O3/OneGate) and submit.
```

## `window.MiniAppSDK`

`platform/sdk/src/window.ts` contains a helper to install the SDK on `window`.
