# Neo N3 Contract Set for Service Layer

Goal: split Service Layer functionality across focused Neo N3 C# contracts, coordinated by a Manager, to avoid oversized single contracts and enable modular upgrades.

## Contracts
- **Manager**: registry of module contract hashes, admin/multisig checks, global/perservice pause flags, role issuance.
- **ServiceRegistry**: service metadata (owner, version, capabilities, off-chain code hash/config hash), service pause, capability flags.
- **AccountManager**: workspace/account records, linked wallets, role delegation per account.
- **SecretsVault**: secret reference registry (never store plaintext), ACL checks via Manager roles, emits access-request events.
- **AutomationScheduler**: job definitions (cron/spec/hash), emits `JobDue` events for off-chain runners, tracks job status.
- **OracleHub**: request/response lifecycle, runner authorization, status transitions, emits request/fulfill events.
- **RandomnessHub**: VRF request tracking, fee/limit enforcement, accepts proofs, stores last output hash.
- **DataFeedHub** (optional): feed definitions, signer sets, signed submissions, latest round storage.
- **JAMInbox**: minimal JAM bridge—accept preimage/package/report hashes, enforce quotas, append receipts `{hash, service_id, entry_type, seq, prev_root, new_root, status, processed_at}` and expose roots.

## Manager responsibilities
- Store contract hash per module (`ServiceRegistry`, `AccountManager`, `SecretsVault`, `AutomationScheduler`, `OracleHub`, `RandomnessHub`, `DataFeedHub`, `JAMInbox`).
- Multisig admin: `SetModule(name, hash)`, `Pause(name, flag)`, `GrantRole(account, role)`, `RevokeRole(account, role)`.
- Expose getters so clients and other contracts fetch trusted hashes instead of hard-coding.
- Common role constants: `RoleAdmin`, `RoleScheduler`, `RoleOracleRunner`, `RoleRandomnessRunner`, `RoleJamRunner`, `RoleDataFeedSigner`.

## Storage hints per contract
- Use storage prefixes per module to avoid collisions.
- Prefer compact structs (byte arrays, hashes) instead of large payloads.
- For lists, store per-key maps plus counters for iteration/pagination when needed.

## Event sketches
- Manager: `ModuleUpgraded(name, hash)`, `RoleGranted(addr, role)`, `Paused(name, flag)`.
- ServiceRegistry: `ServiceRegistered(id, owner, version)`, `ServiceUpdated(id, version)`.
- Automation: `JobCreated(id, serviceId)`, `JobDue(id, serviceId)`, `JobCompleted(id, status)`.
- Oracle: `OracleRequested(id, serviceId)`, `OracleFulfilled(id, status)`.
- Randomness: `RandomnessRequested(id, serviceId)`, `RandomnessFulfilled(id, outputHash)`.
- JAMInbox: `ReceiptAppended(hash, serviceId, seq, newRoot)`.

## Method skeletons (C# devpack style)
- Manager
  - `SetModule(string name, UInt160 hash)` (admin/multisig)
  - `GetModule(string name) -> UInt160`
  - `GrantRole(UInt160 account, byte role)` / `RevokeRole`
  - `Pause(string name, bool flag)`
  - `HasRole(UInt160 account, byte role) -> bool`
- ServiceRegistry
  - `RegisterService(ByteString id, UInt160 owner, ByteString codeHash, ByteString configHash, byte capabilities)`
  - `UpdateService(ByteString id, ByteString codeHash, ByteString configHash, byte capabilities)` (owner or admin)
  - `PauseService(ByteString id, bool flag)` (admin)
  - `GetService(ByteString id) -> Service`
- AutomationScheduler
  - `CreateJob(ByteString id, ByteString serviceId, string spec, ByteString payloadHash, int maxRuns)` (owner/role)
  - `MarkDue(ByteString id)` (off-chain runner via role) emits `JobDue`
  - `CompleteJob(ByteString id, byte status)` (runner)
- OracleHub
  - `Request(ByteString id, ByteString serviceId, ByteString payloadHash, long fee, uint ttl)`
  - `Fulfill(ByteString id, ByteString resultHash)` (runner role)
  - `GetRequest(ByteString id) -> Request`
- RandomnessHub
  - `RequestVRF(ByteString id, ByteString serviceId, ByteString seedHash, long fee)`
  - `FulfillVRF(ByteString id, ByteString proof, ByteString output)` (runner role)
- DataFeedHub
  - `DefineFeed(ByteString id, ByteString pair, UInt160[] signers, int threshold)`
  - `Submit(ByteString feedId, ByteString roundId, ByteString price, ByteString signature)` (signer role)
  - `GetLatest(ByteString feedId) -> Round`
- JAMInbox
  - `AppendReceipt(ByteString hash, ByteString serviceId, byte entryType, ByteString prevRoot, ByteString newRoot, byte status, BigInteger seq)` (runner/admin)
  - `GetReceipt(ByteString hash) -> Receipt`
  - `GetRoot(ByteString serviceId) -> Root`

## Upgrade & call pattern
- Off-chain components read module hashes from Manager before calling.
- On-chain interop: modules may call Manager to validate roles or fetch peer hashes, then `Contract.Call` the peer.
- Prefer role checks via Manager rather than hard-coded addresses.

## Next steps
- Scaffold C# devpack contracts for Manager and JAMInbox (storage layout, events, role checks).
- Add deployment script to set module hashes in Manager.
- Provide example client helpers to read module hashes and construct `Contract.Call` invocations.
