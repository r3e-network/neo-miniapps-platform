# NeoCompute On‑Chain Contract

NeoCompute (`service_id`: `neocompute`) does not anchor results to a platform
contract by default. Verifiable randomness anchoring is handled by **NeoVRF**
via `RandomnessLog`.

If a MiniApp needs to anchor compute outputs on-chain, it should define its own
contract schema (or reuse `AutomationAnchor`/`AppRegistry` where applicable) and
invoke it via `txproxy`.
