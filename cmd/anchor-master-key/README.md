# anchor-master-key

Helper CLI to anchor the Coordinator master TEE key on the ServiceLayerGateway contract.

## Usage

```
go run ./cmd/anchor-master-key \
  --rpc https://neo-rpc.example \
  --gateway 0x<gateway_script_hash> \
  --priv <admin_private_key_hex> \
  --pubkey <compressed_pubkey_hex> \
  --pubkey-hash <sha256_pubkey_hex> \
  --attest-hash <attestation_bundle_hash_hex_or_cid>

# Auto-mode: fetch pubkey/hash from NeoAccounts (AccountPool)
go run ./cmd/anchor-master-key \
  --rpc https://neo-rpc.example \
  --gateway 0x<gateway_script_hash> \
  --priv <admin_private_key_hex> \
  --neoaccounts https://neoaccounts:8085 \
  --attest-hash <attestation_bundle_hash_hex_or_cid>

# Auto-attestation hash from bundle
go run ./cmd/anchor-master-key \
  --rpc https://neo-rpc.example \
  --gateway 0x<gateway_script_hash> \
  --priv <admin_private_key_hex> \
  --neoaccounts https://neoaccounts:8085 \
  --bundle file:///path/to/bundle.json   # SHA-256 used as attest-hash
```

Inputs:
- `priv`: admin key for the gateway (hex, no 0x).
- `pubkey`: compressed EC pubkey of the Coordinator master key.
- `pubkey-hash`: SHA-256 of the pubkey (32 bytes hex).
- `attest-hash`: hash/CID of the attestation bundle that binds `pubkey-hash` in report data.

Effect:
- Calls `setTEEMasterKey` on the gateway, storing pubkey, pubkey hash, and attestation hash, and emits `TEEMasterKeyAnchored`.

Prereqs:
- Gateway contract >= v3.0.2 (includes `setTEEMasterKey`).
- Coordinator attestation bundle available for verifiers.
```
