# Service Layer Dashboard

This React + Vite single-page app surfaces the operational workflows described in
[`docs/requirements.md`](../../docs/requirements.md). Keep the UI routes aligned
with the documented HTTP API so operators can navigate the platform without
guessing.

## Docker

Build and run the static dashboard:

```bash
docker build -t service-layer-dashboard .
docker run -p 8081:80 service-layer-dashboard
```

Then open `http://localhost:8081` and configure API + Prometheus URLs in the UI.
For local compose defaults you can prefill from the URL:
`http://localhost:8081/?baseUrl=http://localhost:8080&token=dev-token`.
JWT login is available at `/auth/login` using the users configured via
`AUTH_USERS` (default: `admin/changeme`).

## Project structure (UI)
- `src/App.tsx` stays lean and composes domain hooks + panel components.
- Hooks are domain-scoped (`src/hooks/use*Resources.ts`) and re-exported via `src/hooks/index.ts`.
- Components live in `src/components` and re-export via `src/components/index.ts` for concise imports.
- Shared formatting helpers live in `src/utils/formatters.ts`; prefer those over ad-hoc date/number formatting.

## Local Development

```bash
cd apps/dashboard
# Requires Node.js 20+ / npm 10+
npm install
npm run dev
```

Use the in-app settings panel to store the Service Layer API URL and Prometheus
endpoint (they persist in `localStorage`). No compile-time environment variables
are required unless you choose to bake defaults into the bundle via `VITE_*`
values.

## Notes
- Prometheus auth/CORS: the dashboard uses bearer tokens on PromQL queries; if
  Prometheus is not co-hosted or CORS blocks requests, run a small proxy or
  co-host with nginx.
- Refer back to the specification before adding new views or metrics so the UX
  remains consistent across CLI, docs, and dashboard.
- Before releases, walk through the UI using the [dashboard smoke checklist](../../docs/dashboard-smoke.md)
  to catch console/API drift (datafeeds, DataLink, JAM, gas bank, system cards).
