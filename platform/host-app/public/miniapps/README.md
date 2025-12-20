# MiniApps Export Target

This folder is an **export target** for the canonical static MiniApps under:

- `miniapps/`

The host app serves these files from `platform/host-app/public/miniapps/*` so you
can load them via `/?entry_url=/miniapps/...`.

To refresh the copies, run:

```bash
./scripts/export_host_miniapps.sh
```
