# Project Structure

The repository now focuses solely on the refactored runtime.

```text
cmd/
  appserver/              # Runtime entry point
configs/                  # Example configuration files
internal/
  app/                    # Application services, storage adapters, HTTP API
  config/                 # Configuration types and helpers
  platform/               # Platform utilities (database, migrations)
  version/                # Build/version metadata
pkg/                      # Shared utility packages
docs/                     # Documentation
scripts/                  # Utility scripts and CI helpers
```
