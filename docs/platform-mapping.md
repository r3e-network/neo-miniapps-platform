# Platform Mapping (Current Repo → Target MiniApp Platform)

This document maps the current repository structure into the target **Neo N3 MiniApp Platform** layout.

## Target Top-Level

The target structure is:

- `contracts/`: platform contracts (GAS-only payments, NEO-only governance, feeds, randomness logs, app registry, automation anchor)
- `services/`: attested SGX services (datafeed/oracle/vrf/compute/automation/tx-proxy)
- `platform/`: Next.js host + SDK + Supabase Edge + RLS policies
- `miniapps/`: builtin miniapps + developer templates
- `deploy/`: neo-express config + deployment scripts
- `docker/`: local dev compose bundles
- `k8s/`: Kubernetes manifests/helm values
- `.github/`: CI workflows (GitHub Actions)
- `docs/`: specs and operational guidance

## Current Repo Modules

### Services (current → target naming)

The repository keeps service IDs (runtime) stable, and maps them to the target
platform naming in docs:

- `services/datafeed` (`neofeeds`) → `datafeed-service`
- `services/conforacle` (`neooracle`) → `oracle-gateway`
- `services/confcompute` (`neocompute`) → `compute-service`
- `services/vrf` (`neovrf`) → `vrf-service`
- `services/automation` (`neoflow`) → `automation-service`
- `services/txproxy` (`txproxy`) → `tx-proxy`

### Existing Infrastructure (keep)

- `infrastructure/chain`: Neo N3 RPC, tx building/submission, event monitoring
- `infrastructure/globalsigner`: enclave-held signing keys
- `infrastructure/accountpool`: large account pool + locking
- `infrastructure/secrets`: per-user secrets + permissions

These remain as shared building blocks used by the platform services.

### Platform Layer

- `platform/host-app`: Next.js host (Vercel) with iframe + Module Federation loader
- `platform/builtin-app`: built-in MiniApps served as Module Federation remote
- `platform/sdk`: JS SDK (`window.MiniAppSDK`)
- `platform/edge`: Supabase Edge functions (auth/limits/routing)
- `platform/rls`: Supabase RLS SQL policies (schema lives in `migrations/`)
- `miniapps/`: built-in manifests + developer starter kits (static previews exported for iframe use)

## Notes

- The user-facing gateway is **Supabase Edge** (there is no Go gateway binary in the current codebase).
- This repo will keep a strong separation: **chain I/O** in `infrastructure/chain`, **policy enforcement** in Edge + tx-proxy, **sensitive compute/signing** inside TEE.
