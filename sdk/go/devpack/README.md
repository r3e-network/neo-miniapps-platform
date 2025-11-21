# Service Layer Devpack (Go)

Lightweight helpers that mirror the Devpack actions available at function runtime. This is a data model, **not** an HTTP client; the function runtime collects queued actions and executes them after your code returns.

## Usage

```go
import dp "github.com/R3E-Network/service_layer/sdk/go/devpack"

func handler() []dp.Action {
    return []dp.Action{
        dp.EnsureGasAccount(map[string]interface{}{"wallet": "NWALLET"}),
        dp.RecordPriceSnapshot(map[string]interface{}{"feedId": "feed-1", "price": 12.34}),
        dp.CreateDataLinkDelivery(map[string]interface{}{"channelId": "channel-1", "payload": map[string]interface{}{"foo": "bar"}}),
    }
}
```

Published version alignment: `v0.6.0` matches the TypeScript Devpack runtime (0.6.0).
