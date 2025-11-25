# Dashboard E2E Playwright Smoke

Run a light Playwright suite to ensure the dashboard renders core flows:

Tests:
- `neo-smoke.spec.ts` — deep link with `?api&token&tenant`, loads system overview and NEO panel, refreshes, and (optionally) triggers snapshot verify.
- NEO smoke stubs `/neo/blocks`, `/neo/blocks/{height}`, `/neo/storage-summary/{height}`, `/neo/storage/{height}`, `/neo/storage-diff/{height}`, and `/neo/snapshots` so the dashboard storage-summary and blob-fetch flows are exercised without relying on indexed data.
- `bus-smoke.spec.ts` — exercises the Engine Bus Console (event/data/compute) to ensure the UI and endpoints are reachable.

## Prerequisites
- Running stack: `docker compose up -d` (API 8080, dashboard 8081, Postgres 5432). Defaults include `API_TOKENS=dev-token` and sample `AUTH_USERS`.
- Node 18+ with npm.
- Playwright browsers installed: `cd apps/dashboard && npx playwright install --with-deps` (required once per environment). Git-ignored caches live under `~/.cache/ms-playwright` and `apps/dashboard/test-results/`.

## Run locally
```bash
cd apps/dashboard
npm install
# one-time browser install
npx playwright install --with-deps
# run both smoke tests headless chromium
API_URL=http://localhost:8080 API_TOKEN=dev-token DASHBOARD_URL=http://localhost:8081 npm run e2e -- --project=chromium
```

## CI workflow
- `.github/workflows/dashboard-e2e.yml` runs `npm install`, installs browsers, starts the compose stack, then runs `npm run e2e` with `API_URL/API_TOKEN/DASHBOARD_URL`.
- Once stable, mark `dashboard-e2e` as a required check alongside `neo-smoke` (see `docs/branch-protection.md`).

## Troubleshooting
- Missing browsers: run `npx playwright install --with-deps`.
- Auth failures: ensure `API_TOKENS` matches `API_TOKEN` env, or login via admin/changeme.
- Network flakiness: increase `use.timeout` in `playwright.config.ts` or rerun locally to repro.
