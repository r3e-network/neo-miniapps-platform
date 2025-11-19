# Service Layer Review Checklist

_Last reviewed: 2025-11-19 (UTC)_

Use this checklist whenever you touch platform behaviour. Work through it in
order, and reset the checkboxes the next time you audit the repo.

## Global Consistency
- [ ] Update [`docs/requirements.md`](requirements.md) before changing code so it
      remains the single source of truth.
- [ ] Keep [`docs/README.md`](README.md) and the root `README.md` pointed at the
      specification and this checklist.
- [ ] Remove or archive stale documentation so `docs/` only carries the
      specification, index, and this checklist.
- [ ] Verify `go test ./...` succeeds after code changes.
- [ ] Verify `npm run build` inside `apps/dashboard` compiles the dashboard with
      all service panels present.
- [ ] Confirm CLI documentation matches the subcommands currently shipped in `cmd/slctl`.
- [ ] Confirm every service descriptor reports the `platform` layer so all capabilities share the same priority.

## Service Coverage (HTTP API + Dashboard)
Each item requires an HTTP surface documented in
[`docs/requirements.md`](requirements.md) and a corresponding panel or control in
`apps/dashboard/src/App.tsx`.

- [ ] Accounts & Authentication — `/accounts`, account grid on the dashboard header card.
- [ ] Workspace Wallets — `/accounts/{id}/workspace-wallets`, wallet list within each account card.
- [ ] Secrets Vault — `/accounts/{id}/secrets`, secrets panel with inline fetch button.
- [ ] Functions Runtime — `/accounts/{id}/functions`, functions + recent executions panel.
- [ ] Automation & Triggers — `/accounts/{id}/automation/jobs` and `/triggers`, automation block displaying jobs + triggers.
- [ ] Oracle Adapter — `/accounts/{id}/oracle/sources` and `/oracle/requests`, oracle sources + request feed.
- [ ] Price Feed Service — `/accounts/{id}/pricefeeds`, dashboard price feed panel showing feeds + snapshots.
- [ ] Gas Bank — `/accounts/{id}/gasbank` plus `/gasbank/transactions`, dashboard balance + transaction list.
- [ ] Randomness Service — `/accounts/{id}/random/requests`, randomness request list for each account.
- [ ] CRE Orchestrator — `/accounts/{id}/cre/*`, CRE block covering executors, playbooks, and runs.
- [ ] CCIP — `/accounts/{id}/ccip/*`, CCIP lanes + messages section.
- [ ] Data Feeds — `/accounts/{id}/datafeeds`, feed + updates block.
- [ ] Data Streams — `/accounts/{id}/datastreams/*`, stream definition + frame listing.
- [ ] DataLink — `/accounts/{id}/datalink/*`, channels + delivery status list.
- [ ] DTA — `/accounts/{id}/dta/*`, DTA products + orders table.
- [ ] VRF — `/accounts/{id}/vrf/*`, VRF keys + requests view.
- [ ] Confidential Compute — `/accounts/{id}/confcompute/*`, enclave inventory card.
- [ ] Observability & System — `/metrics`, `/system/descriptors`, and dashboard system/metrics cards.

## CLI Coverage
- [ ] `slctl accounts` — docs mention list/create/get/delete flows.
- [ ] `slctl functions` — README references deploy & inspection commands.
- [ ] `slctl automation` / `slctl gasbank` / `slctl oracle` — document job, balance, and source management stories.
- [ ] `slctl pricefeeds` — spec + README describe feed CRUD/snapshot flows.
- [ ] `slctl cre playbooks|executors|runs` — docs show how to inspect CRE inventory and run history.
- [ ] `slctl ccip lanes|messages` — CLI docs mirror CCIP lane/message inspection APIs.
- [ ] `slctl vrf keys|requests` — README/spec reference VRF CLI coverage for keys and requests.
- [ ] `slctl datalink channels|deliveries` — CLI docs reflect DataLink channel + delivery inspection flows.
- [ ] `slctl dta products|orders` — docs describe DTA product/order inspection via CLI.
- [ ] `slctl datastreams streams|frames` — docs reference stream + frame inspection flows.
- [ ] `slctl confcompute enclaves` — README/spec explain confidential-compute inventory checks.
- [ ] `slctl workspace-wallets list` — README/spec mention wallet inventory coverage.
- [ ] `slctl random generate` and `slctl random list` — README demonstrates both flows and matches `/random` + `/random/requests`.
- [ ] `slctl services` — docs point to descriptor introspection for feature discovery.

Reset these boxes before the next review so the checklist reflects the newest audit.
