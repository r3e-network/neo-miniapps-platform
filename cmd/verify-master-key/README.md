# verify-master-key

Validate that the NeoAccounts-reported master pubkey/hash matches the on-chain gateway anchor.

## Usage
```
go run ./cmd/verify-master-key \
  --rpc https://neo-rpc.example \
  --gateway 0x<gateway_script_hash> \
  --neoaccounts https://neoaccounts:8085
```

The tool:
1) Fetches `/master-key` (pubkey, hash) from NeoAccounts (AccountPool).
2) Reads `TEEMaster` anchor from the gateway (`pubkey`, `pubkey hash`, `attestation hash`).
3) Prints both and exits non-zero if pubkey/hash differ.

Note: attestation hash is displayed for operator review; this tool does not verify MarbleRun attestation.
