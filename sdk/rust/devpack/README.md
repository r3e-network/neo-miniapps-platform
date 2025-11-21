# Service Layer Devpack (Rust)

Experimental Rust helpers that construct Devpack action payloads matching the Service Layer runtime. This is a data model only; queue actions in your function environment and let the platform execute them after the function completes.

## Usage

```rust
use std::collections::HashMap;
use service_layer_devpack as dp;

fn build_actions() -> Vec<dp::Action> {
    let mut payload = HashMap::new();
    payload.insert("feedId".into(), serde_json::json!("feed-1"));
    payload.insert("price".into(), serde_json::json!(12.34));

    vec![
        dp::ensure_gas_account(None),
        dp::record_price_snapshot(payload),
        dp::create_datalink_delivery(HashMap::from([
            ("channelId".into(), serde_json::json!("channel-1")),
            ("payload".into(), serde_json::json!({"foo": "bar"})),
        ])),
    ]
}
```

Version: `0.6.0` (aligned with the TypeScript Devpack runtime).
