# NeoVRF On‑Chain Contract

NeoVRF (`service_id`: `neovrf`) anchors randomness outputs on-chain using the
platform `RandomnessLog` contract.

## Canonical Source

- Contract implementation: `../../../contracts/RandomnessLog/RandomnessLog.cs`
- Platform contract overview: `../../../contracts/README.md`

## Purpose

`RandomnessLog` provides an append-only audit log for randomness outputs:

- Writes are restricted to the contract **Updater** (set by admin).
- Each record stores the randomness value and an `attestation_hash`.

## Key Methods / Events

- `SetUpdater(updater)`: admin sets the Updater account.
- `Record(requestId, randomness, attestationHash, timestamp)`: Updater records a randomness result (requestId is one-time).
- `Get(requestId)`: reads a previously-recorded result.
- Event: `RandomnessRecorded`.

## Configuration

The service discovers the deployed contract via:

- `CONTRACT_RANDOMNESSLOG_HASH`: deployed script hash (see `../../../.env.example`).

On-chain writes are performed via `../../txproxy/README.md` (allowlisted sign+broadcast).
