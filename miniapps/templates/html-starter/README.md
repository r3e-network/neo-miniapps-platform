# MiniApp HTML Starter Kit

This directory contains a **build-free** HTML MiniApp starter that integrates with
`window.MiniAppSDK` for payments, governance, datafeeds, and randomness.

Contents:

- `manifest.json` (see `docs/manifest-spec.md`)
- `index.html` + `app.js`: a lightweight UI that calls `window.MiniAppSDK`
- `miniapps/_shared/miniapp-bridge.js`: optional postMessage bridge for cross-origin iframes

Usage:

1. Copy this directory to your MiniApp workspace.
2. Update `manifest.json` (`app_id`, `entry_url`, `developer_pubkey`, permissions).
3. Host the files on a CDN and register the manifest via the `app-register` Edge function.
4. Ensure your host injects the SDK for same-origin iframes or ships the bridge script for cross-origin apps.

When loaded in a host that provides the SDK, this starter can call:

- `MiniAppSDK.wallet.getAddress()`
- `MiniAppSDK.payments.payGAS(appId, amount, memo)` → invocation intent + `request_id`
- `MiniAppSDK.governance.vote(appId, proposalId, neoAmount, support)` → invocation intent + `request_id`
- `MiniAppSDK.wallet.invokeIntent(request_id)` (host submits the invocation via the user wallet)
- `MiniAppSDK.datafeed.getPrice("BTC-USD")`
- `MiniAppSDK.rng.requestRandom(appId)`
