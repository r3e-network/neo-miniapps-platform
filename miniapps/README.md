# MiniApps

- `builtin/`: first-party MiniApps maintained by this repo (Module Federation entry URLs + static iframe bundles)
- `templates/`: developer starter kits (React + HTML) for third-party MiniApps, not exported to the host public folder
- `_shared/`: shared, build-free helpers (e.g. SDK postMessage bridge)

Built-in manifests point to the Module Federation remote (`mf://builtin?app=...`).
Static HTML bundles remain available for local iframe previews or CDN fallbacks.

Note: the host export script (`scripts/export_host_miniapps.sh`) intentionally
skips `miniapps/templates/` to avoid shipping build toolchains inside the host's
`public/` folder.
