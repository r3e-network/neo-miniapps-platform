# AccountPool Master Key Attestation & Anchoring

This flow lets operators bind the AccountPool master key to Coordinator attestation, anchor the hash on-chain (gateway), and give users everything needed for off-chain verification.

## Secrets (Coordinator → AccountPool)
- `POOL_MASTER_KEY` (32 bytes) **or** `COORD_MASTER_SEED` (>=16 bytes) to deterministically derive the master key.
- `POOL_MASTER_KEY_HASH` — SHA-256 of the master key (raw 32 bytes or hex). Required in TEE mode to pin the key.
- `POOL_MASTER_ATTESTATION_HASH` (optional) — hash of the attestation bundle embedding `POOL_MASTER_KEY_HASH`.

## Attestation bundle (AccountPool `/master-key`)
`GET /master-key` (RA-TLS) returns:
```json
{
  "hash": "<sha256(compressed_master_pubkey)>",
  "pubkey": "<compressed_pubkey_hex>", // use this when anchoring on-chain
  "quote": "<base64>",
  "mrenclave": "<base64>",
  "mrsigner": "<base64>",
  "prod_id": 1,
  "isvsvn": 3,
  "timestamp": "<rfc3339>",
  "source": "accountpool",
  "simulated": false
}
```
The quote’s report data contains `hash`, binding the master key to the TEE measurement.

## On-chain anchoring (gateway contract)
Store/emit:
- `masterPubKey` (derived externally from the master key when needed)
- `masterPubKeyHash` = SHA-256(masterPubKey) (compressed)
- `attestationHash` = SHA-256(attestation bundle) or CID of the bundle
The contract does **not** parse attestation; it anchors hashes/events for transparency.

### CLI helper (admin)
Use `cmd/anchor-master-key` to call `setTEEMasterKey` on the gateway:
```
go run ./cmd/anchor-master-key \
  --rpc https://neo-rpc.example \
  --gateway 0x...gatewayScriptHash \
  --priv <admin-priv-hex> \
  --pubkey <compressed-pubkey-hex> \
  --pubkey-hash <sha256(pubkey)-hex> \
  --attest-hash <bundle-hash-hex-or-cid>
```
This stores the pubkey, pubkey hash, and attestation hash and emits `TEEMasterKeyAnchored`.

## Verifier checklist (off-chain)
1) Fetch `masterPubKeyHash` and `attestationHash` from the gateway contract/event.
2) Fetch the attestation bundle (by hash/CID/URL).
3) Verify MarbleRun quote + chain with expected policy (signer, ProdID fixed, ISVSVN >= minimum; optionally pin manifest hash).
4) Check quote report data == `hash(master_pubkey)` from bundle.
5) Check `hash(bundle)` == `attestationHash` on-chain.
6) If also anchoring pubkey: ensure `hash(masterPubKey)` == on-chain `masterPubKeyHash`.

## Upgrade policy (Coordinator)
- Use MRSIGNER sealing; keep ProdID fixed; bump ISVSVN on each release.
- Re-issue attestation with the same master key hash; update `attestationHash` on-chain; do **not** rotate the key unless intentional.

## Rotation (intentional)
1) Generate new master key (or seed), set new `POOL_MASTER_KEY_HASH`, seal it.
2) Re-issue attestation; update `attestationHash` and on-chain anchor.
3) Publish deprecation notice for old hash if needed.
