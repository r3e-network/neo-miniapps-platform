# Host App (Next.js)

This **Next.js** host runs on **Vercel** and serves as the entry point for MiniApps.

Responsibilities:

- enforce MiniApp manifest policy (permissions/limits/assets) via Edge gating
- sandbox MiniApps: **Module Federation** for built-ins, `iframe` for third-party apps
- strict CSP + postMessage allowlists
- provide `window.MiniAppSDK` for federated apps and same-origin iframes
- surface wallet binding, intent submission, and AppRegistry workflows

Current capabilities:

- `pages/index.tsx` loads MiniApps via `entry_url` (supports `mf://` for Module Federation and `iframe` URLs).
- `pages/federated.tsx` is a dedicated Module Federation loader.
- `window.MiniAppSDK` is exposed for federated MiniApps and injected into same-origin iframes.
- Settings UI includes wallet binding (`wallet-nonce` + `wallet-bind`) and intents (`pay-gas` / `vote-neo`).
- AppRegistry workflow for `app-register` / `app-update-manifest`.
- CSP headers set via `platform/host-app/middleware.ts` with per-request nonces.

## Production Configuration

- `MINIAPP_FRAME_ORIGINS`: space-separated `frame-src` allowlist for embedded iframes.
- `NEXT_PUBLIC_MF_REMOTES`: comma-separated Module Federation remotes (e.g. `builtin@https://cdn.miniapps.com/miniapps/builtin-mf`).
- `NEXT_PUBLIC_SUPABASE_URL`: Supabase project URL for `connect-src` allowlist.

## `/api/rpc/*` Proxy (Blueprint Path)

The architectural blueprint uses the prefix `/api/rpc/*` for gateway endpoints.
In production, Supabase Edge Functions use `/functions/v1/*`.

This host app includes an optional proxy route:

- `platform/host-app/pages/api/rpc/[fn].ts`

It forwards `GET/POST/...` requests to:

- `${EDGE_BASE_URL}/<fn>` (preferred), or
- `${NEXT_PUBLIC_SUPABASE_URL}/functions/v1/<fn>` (fallback)

Set `EDGE_BASE_URL` to one of:

- `https://<project>.supabase.co/functions/v1`
- `http://localhost:8787/functions/v1` (repo Edge dev server)

## Local Runs

### Module Federation (Built-ins)

Run the built-in remote and host app together:

```bash
cd platform/builtin-app
npm install
npm run dev
```

```bash
cd platform/host-app
NEXT_PUBLIC_MF_REMOTES=builtin@http://localhost:3001 npm run dev
```

Then open:

- `http://localhost:3000/?entry_url=mf://builtin?app=builtin-price-ticker`

### iframe Runs

Static MiniApps are exported to the host public directory:

- `platform/host-app/public/miniapps/builtin/*`

Refresh them with:

```bash
./scripts/export_host_miniapps.sh
```

Then open:

- `http://localhost:3000/?entry_url=/miniapps/builtin/coin-flip/index.html`

## Module Federation (Built-ins)

The built-in remote lives in `platform/builtin-app` and exposes `./App` as `builtin/App`.
Built-in manifests use:

```
mf://builtin?app=<app_id>
```

The host resolves the remote URL using `NEXT_PUBLIC_MF_REMOTES` and loads the
federated module without an iframe sandbox.

## Wallet Binding + Intents

The host expects a Neo N3 browser wallet. The host UI currently supports **NeoLine N3**.

1. Install NeoLine N3 in your browser.
2. In the Settings panel:
   - set `Supabase Edge base URL`
   - paste an `Auth JWT` (Supabase session token; required for wallet binding)
3. In **Wallet Binding**:
   - click `Detect Wallet`
   - click `Get Bind Message`
   - click `Sign & Bind` (NeoLine will prompt to sign)
4. In **On-chain Intents**:
   - click `Create Intent` for `pay-gas` / `vote-neo`
   - click `Submit via Wallet` to call NeoLine `invoke`

If `pay-gas` / `vote-neo` returns `WALLET_REQUIRED`, bind a wallet first.

## Cross-Origin MiniApps

The host **cannot** directly inject JS into a cross-origin iframe. For production
MiniApps hosted on a CDN, the SDK must be bundled into the MiniApp itself, or
you must implement a postMessage bridge with strict origin allowlists.

This host includes a minimal postMessage-based bridge:

- Host handler: `platform/host-app/pages/index.tsx`
- MiniApp script: `platform/host-app/public/sdk/miniapp-bridge.js`

Bridge notes:

- `payments.payGAS(...)` / `governance.vote(...)` return an `invocation` intent plus a `request_id`.
- MiniApps can then call `wallet.invokeIntent(request_id)` to ask the host to submit that intent via NeoLine.
  The host only allows invoking intents it previously created (in-memory, one-time).

MiniApps can include the script from the host origin:

```html
<script src="https://<host>/sdk/miniapp-bridge.js"></script>
```

The host only responds to requests from the currently embedded iframe and the
expected origin derived from `entry_url`.
