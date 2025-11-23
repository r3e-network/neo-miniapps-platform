# Dashboard Smoke Checklist

Quick manual pass to keep the React dashboard in sync with the API after backend changes.

## Build & Load
- `npm install && npm run build` under `apps/dashboard` (or run `npm run dev`).
- Load the app and point it at a running Service Layer instance with a valid token.

## Data Feeds
- Create/edit a feed: ensure `aggregation` select lists `median|mean|min|max`, accepts per-feed override, and persists.
- Submit a round from the UI (or via CLI) and verify the latest view shows aggregated price and metadata.

## DataLink
- Create a channel and enqueue a delivery; verify status updates and metadata display. Confirm signer set is required.
- Check success/error feedback and that deliveries appear with metadata.

## JAM
- Check JAM status panel reflects `/system/status` `jam` block.
- Upload a preimage from the dashboard action panel; submit a package (using last preimage) and verify success feedback.

## Gas Bank
- Ensure summary view renders balances and pending withdrawals; retry/cancel actions succeed.

## Cross-cutting
- Verify errors surface with actionable messages (auth failures, validation).
- Confirm `/metrics` and `/system/status` links render and require auth.
- Run through navigation on mobile viewport to catch layout regressions.

Document any broken flows or missing fields; update UI/API wiring before release.
