# React MiniApp Starter Kit

This is a **React + Vite** starter kit for building a Neo N3 MiniApp that talks to
the platform via `window.MiniAppSDK`.

## Quick Start

```bash
cd miniapps/templates/react-starter
npm install
npm run dev
```

## Host Integration

In production, the host embeds your app in an `<iframe>` and provides
`window.MiniAppSDK` either:

- by injecting it (same-origin apps), or
- via a `postMessage` bridge (cross-origin apps).

For local host previews in this repo, the bridge helper is exported as:

- `/sdk/miniapp-bridge.js`

## Manifest

See `miniapps/templates/react-starter/manifest.json`. It enforces:

- `assets_allowed: ["GAS"]`
- `governance_assets_allowed: ["NEO"]`
