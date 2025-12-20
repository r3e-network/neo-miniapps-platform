# MiniApp SDK Guide

MiniApps must not construct or sign Neo transactions directly. All sensitive actions flow through:

`MiniApp → Host SDK → Supabase Edge (auth/limits) → TEE services (attested) → Neo N3 chain`

## Runtime Model

- The host provides `window.MiniAppSDK`.
- MiniApps run in a sandbox (Module Federation or `iframe`) with strict CSP.
- MiniApps communicate with the host via a restricted message channel (allowlisted origins).

## API (Draft)

```ts
declare global {
  interface Window {
    MiniAppSDK: {
      wallet: {
        getAddress(): Promise<string>;
        // Optional: ask the host to submit a previously created invocation intent.
        // Hosts should only allow request_ids they created (one-time).
        invokeIntent?(requestId: string): Promise<unknown>;
      };
      payments: {
        // Returns a contract invocation intent. The host/wallet signs & submits.
        payGAS(appId: string, amountGAS: string, memo?: string): Promise<{
          request_id: string;
          intent: "payments";
          invocation: { contract_hash: string; method: string; params: any[] };
        }>;
      };
      governance: {
        // Returns a contract invocation intent. The host/wallet signs & submits.
        vote(appId: string, proposalId: string, neoAmount: string, support?: boolean): Promise<{
          request_id: string;
          intent: "governance";
          invocation: { contract_hash: string; method: string; params: any[] };
        }>;
      };
      rng: {
        // RNG is executed inside TEE (via `neovrf`), optional on-chain anchoring.
        requestRandom(appId: string): Promise<{ request_id: string; randomness: string; signature?: string; public_key?: string; attestation_hash?: string }>;
      };
      datafeed: {
        // Read-only price (typically proxied from `neofeeds`).
        getPrice(symbol: string): Promise<{
          feed_id: string;
          pair: string;
          price: number | string;
          decimals: number;
          timestamp: string;
          sources: string[];
          signature?: string;
          public_key?: string;
        }>;
        // Planned: stream subscription (SSE/WebSocket) via Edge proxy.
        subscribe(symbol: string, cb: (p: any) => void): () => void;
      };
    };
  }
}
```

## Host-Only APIs

The `platform/sdk` also exposes a host-only client (`HostSDK`) for workflows that
must not be exposed to untrusted MiniApps (wallet binding, secrets, API keys,
gasbank, oracle queries, compute execution, automation triggers).

Auth can be provided either as a Supabase JWT (`Authorization: Bearer`) or as a
user API key (`X-API-Key`) via `MiniAppSDKConfig.getAPIKey`. In production,
host-only endpoints (oracle/compute/automation/secrets) require API keys with
explicit scopes; bearer JWTs are rejected there.

## Example

```ts
const address = await window.MiniAppSDK.wallet.getAddress();

// User-signed flow: get an invocation intent from Supabase Edge, then have the wallet sign it.
const pay = await window.MiniAppSDK.payments.payGAS("raffle", "1.5", "entry fee");
// Option A (host-specific helper): ask the host to submit the intent via the wallet.
await window.MiniAppSDK.wallet.invokeIntent?.(pay.request_id);
// Option B: host builds tx for pay.invocation and submits via wallet dAPI (NeoLine/O3/OneGate)

const { randomness, reportHash } = await window.MiniAppSDK.rng.requestRandom("raffle");

const price = await window.MiniAppSDK.datafeed.getPrice("BTC-USD");
```

## Security Notes

- The host must strip/ignore any identity headers from MiniApps.
- Rate limits and caps are enforced on **Edge** and **TEE** (defense in depth).
- Host must enforce manifest constraints (assets/permissions/limits) at runtime.
