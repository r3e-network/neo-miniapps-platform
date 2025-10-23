# Refactored Service Layer Migration Plan

This document tracks the incremental rebuild of the Service Layer modules on the
new architecture. Each module entry contains the checklist to implement the
feature from scratch (storage, service, API, tests, docs) and notes on progress.

## Global Principles

1. Implement a storage interface per module (memory + Postgres).
2. Wire services through `internal/app/application.go` so they participate in
   lifecycle management.
3. Expose functionality via `/accounts/{accountID}/...` HTTP endpoints.
- [x] Add unit and HTTP tests
5. Update documentation and tooling as features become available.

## Module Roadmap

### 1. Secrets & TEE (Completed)
- [x] Domain models (`internal/app/domain/secret`)
- [x] Memory & Postgres stores (`storage.SecretStore`, migration `0004`)
- [x] Secrets service with AES-GCM support
- [x] Goja-based executor resolving secrets
- [x] `/accounts/{id}/secrets` endpoints + tests
- [x] Docs + README + Docker updates

### 2. Random Service (Completed)
- [x] Domain model and service (`internal/app/services/random`)
- [x] `/accounts/{id}/random` endpoint + tests

### 3. Gas Bank (Completed)
- [x] Review legacy behaviour and determine desired features
- [x] Define updated domain contracts if needed (existing models sufficient)
- [x] Implement service features: ensure account, deposit, withdraw, complete withdrawal, list transactions
- [x] Extend storage interfaces (memory + Postgres)
- [x] Expose REST endpoints (`/gasbank`, `/gasbank/deposit`, `/gasbank/withdraw`, `/gasbank/transactions`)
- [x] Reintroduce settlement poller via `system.Manager`
- [x] Update docs and quick start
- [x] Add unit and HTTP tests


### 4. Triggers & Automation (Completed)
- [x] Define trigger rule model (cron/event/webhook)
- [x] Implement triggers service (CRUD, enable/disable)
- [x] Build automation scheduler (jobs, scheduling; manual run planned separately)
- [x] Wire storage, background cron service, and HTTP endpoints
- [x] Update docs and tests

### 5. Oracle (Completed)
- [x] Data source management
- [x] Request lifecycle (pending, processing, result)
- [x] Dispatcher/background job integration
- [x] HTTP endpoints for sources/requests
- [x] Documentation and tests

### 6. Function Enhancements
- [x] Execution history storage and retrieval
- [x] API for fetching runs
- [x] Extended TEE logging/observability
- [x] Devpack action orchestration and typed SDK

### 7. Cross-Cutting Enhancements
- [x] Prometheus metrics
- [x] Trace-friendly logging consistency
- [x] Config & example refresh

### 8. Production Hardening
- [x] Authentication & authorization middleware for all HTTP endpoints
- [x] Secret encryption at rest wired via `SECRET_ENCRYPTION_KEY`
- [x] Production-grade price feed and oracle executors (no random/timeout stubs)
- [x] Health/readiness endpoints + operational docs
- [x] Config surface trimmed to active options

This document will be updated as modules progress. Mark each checkbox when the
corresponding task is completed.
