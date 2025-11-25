# Dashboard Smoke Checklist

Quick manual pass to keep the React dashboard in sync with the API after backend changes.

## Build & Load
- `npm install && npm run build` under `apps/dashboard` (or run `npm run dev`).
- Load the app and point it at a running Service Layer instance with a valid token. Shortcut for local compose:
  `http://localhost:8081/?api=http://localhost:8080&token=dev-token&tenant=<id>` (fills endpoint/token/tenant).

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
- System modules: ensure the System Overview shows a Modules list with non-empty entries and lifecycle status is `started` for a healthy stack; errors should appear in red.
- Confirm the warning ribbon appears if any module is failed/stopped and that it clears once modules recover.
- With a healthy stack, auto-refresh (every 30s) should not flicker or reset settings/token.
- Run through navigation on mobile viewport to catch layout regressions.
- Engine bus: in the “Engine Bus Console” card, send
  - Event `observation` payload `{account_id, feed_id, price, source}` → expect 200.
  - Data topic `stream-1` payload `{"price":123}` → expect 200.
  - Compute payload `{"function_id":"<fn>","account_id":"<acct>","input":{"foo":"bar"}}` → expect a results table (and 500 when a module returns error).
- NEO: confirm NEO status renders (height/hash/root), recent blocks list, snapshots list, and block detail fetch works without errors (API endpoints `/neo/status`, `/neo/blocks`, `/neo/snapshots`).
- NEO storage summary: check `/neo/storage-summary/<height>` (or dashboard block detail) shows per-contract counts; use “Load storage blobs” to pull full KV/diffs when needed.
- NEO bundles: verify snapshot bundle hashes (full + diff) via dashboard button or `slctl neo verify` and optionally `slctl neo verify-manifest` when manifests are signed.

Document any broken flows or missing fields; update UI/API wiring before release.

## Automated smoke (optional)
- Requires the stack running locally (API at 8080, dashboard at 8081).
- Run: `cd apps/dashboard && npm install && npx playwright install --with-deps && API_URL=http://localhost:8080 API_TOKEN=dev-token DASHBOARD_URL=http://localhost:8081 npm run e2e`
- First time Playwright run may require `npx playwright install --with-deps` to download browsers; caches live in `~/.cache/ms-playwright`.
