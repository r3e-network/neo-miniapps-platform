# Neo N3 Contracts (C# devpack stubs)

Early scaffolding for a modular contract set:
- `Manager.cs`: stores module hashes, roles, and pause flags; emits upgrade/role events.
- `JAMInbox.cs`: records JAM receipts and accumulator roots per service.

These stubs show storage layout, events, and method signatures. Wire role checks to your chosen admin/multisig, and extend with remaining modules (`ServiceRegistry`, `AccountManager`, `OracleHub`, `AutomationScheduler`, `RandomnessHub`, `DataFeedHub`, `SecretsVault`).
