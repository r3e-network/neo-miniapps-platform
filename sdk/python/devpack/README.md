# Service Layer Devpack (Python)

Thin helpers that emit Devpack action payloads. The module mirrors the in-runtime Devpack surface (gas bank, oracle, price feeds, data feeds, data streams, DataLink, randomness, triggers, automation). It does **not** perform HTTP calls; your runtime should serialize collected actions for the Service Layer to execute.

## Usage

```python
from devpack import generate_random, record_price_snapshot, create_datalink_delivery

actions = [
    generate_random({"length": 16}),
    record_price_snapshot({"feedId": "feed-1", "price": "12.34"}),
    create_datalink_delivery({"channelId": "channel-1", "payload": {"foo": "bar"}}),
]
```

Version: `0.6.0` (kept in sync with the TypeScript Devpack runtime).
