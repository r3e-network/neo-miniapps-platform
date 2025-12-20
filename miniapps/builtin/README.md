# Builtin MiniApps

Builtin MiniApps are delivered via **Module Federation** from `platform/builtin-app`.
Each manifest uses `mf://builtin?app=<app_id>` to route the host to the federated
remote while keeping GAS-only / NEO-only policy enforcement intact.

Included built-ins:

- `coin-flip`
- `dice-game`
- `scratch-card`
- `lottery`
- `prediction-market`
- `flashloan`
- `price-ticker`

Static HTML bundles remain in each folder for iframe-based previews or CDN
distribution builds (exported by `scripts/export_host_miniapps.sh`).
