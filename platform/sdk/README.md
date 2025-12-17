# MiniApp SDK (Scaffold)

This package is a small TypeScript SDK scaffold matching the platform blueprint:

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

```ts
await host.secrets.upsert("binance_api_key", "<secret-value>");
await host.secrets.setPermissions("binance_api_key", ["neooracle"]);
const list = await host.secrets.list();
```

## App Submission (Host-only)

Developer app registration is wallet-signed and routed via Supabase Edge:

```ts
const res = await host.apps.register({
  appId: "com.example.demo",
  entryUrl: "https://cdn.example.com/apps/demo/index.html",
  manifestHash: "0x" + "<32-byte sha256 hex>",
  developerPubKey: "0x" + "<33-byte compressed pubkey hex>",
});
// Host app: build/sign tx using res.invocation (NeoLine/O3/OneGate) and submit.
```

## `window.MiniAppSDK`

`platform/sdk/src/window.ts` contains a helper to install the SDK on `window`.
