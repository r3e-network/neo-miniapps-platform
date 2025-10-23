# Function Devpack SDK

The Function Devpack exposes a lightweight JavaScript helper layer that lets
function authors compose richer workflows without dealing directly with the
Service Layer APIs. It is injected automatically by the runtime for every
function execution and can be consumed from the `Devpack` global.

The Devpack focuses on two goals:

1. **Declarative side-effects.** Functions queue actions (gas bank, triggers,
   automation, oracle) that are executed *after* the JavaScript has completed.
   This keeps the runtime deterministic—if the script throws an error no
   side-effects are committed.
2. **Consistent responses.** Every recorded execution includes a structured log
   of the actions that were requested, whether they succeeded, and any payload
   returned by the underlying service.

> ✅ In production leave `TEE_MODE` unset (the default) so the full TEE executor
> runs. During local development you can set `TEE_MODE=mock` to swap in the
> mock executor, which echoes inputs without hitting confidential compute.

## Runtime API

The `Devpack` global is available inside every function:

```js
const { gasBank, oracle, triggers, automation, respond } = Devpack;
```

### Context

```js
Devpack.context
// {
//   functionId: "fn-123",
//   accountId: "acct-456"
// }
```

Use this metadata when you need to include identifiers in action parameters or
diagnostic logs. The `params` and `secrets` objects that were previously
available remain unchanged.

### Respond helpers

```js
return respond.success({ processed: true });
return respond.failure({ code: "INVALID_STATE", message: "..." });
```

These helpers simply shape the response object; you may still return any serial
isable value if you prefer.

### Gas Bank

```js
gasBank.ensureAccount({ wallet: params.wallet });
gasBank.withdraw({
  gasAccountId: "account-id",      // optional when wallet provided
  wallet: params.wallet,           // optional if gasAccountId provided
  amount: 1.5,
  to: "NQ79Cmx..."                 // optional
});
```

The actions execute after the script finishes. The persisted execution record
will include the resulting account and transaction objects under the
`actions` field.

### Oracle

```js
oracle.createRequest({
  dataSourceId: "datasource-id",
  payload: { symbol: "NEO" }       // objects are serialised to JSON
});
```

### Triggers

```js
triggers.register({
  type: "cron",
  rule: "0 * * * *",               // CRON expression
  config: { timezone: "UTC" },
  enabled: true
});
```

### Automation

```js
automation.schedule({
  name: "HourlyRefresh",
  schedule: "0 * * * *",
  description: "Run on the hour",
  enabled: true                    // optional, defaults to true
});
```

## Execution Lifecycle

1. The Devpack resets its internal queue at the start of every execution.
2. Your function runs with access to `params`, `secrets`, `Devpack`, and the
   built-in `console`.
3. Every call to a Devpack module appends an action (`type`, `params`, `id`) to
   the queue and returns a lightweight handle for convenience. Actions do not
   execute immediately.
4. When the script completes successfully the function service:
   - processes the queued actions in order,
   - records whether each action succeeded, along with any result payload, and
   - marks the entire execution as `failed` if an action fails.
5. The original function output is stored unchanged; action results are exposed
   via `execution.Actions` alongside the usual input/output/logs metadata.

If the JavaScript throws an error the action queue is discarded.

## Action Reference

| Type                      | Params                                                        | Result                                               |
|---------------------------|---------------------------------------------------------------|------------------------------------------------------|
| `gasbank.ensureAccount`   | `wallet` _(optional)_                                         | `account` map                                        |
| `gasbank.withdraw`        | `gasAccountId` _(optional)_, `wallet` _(optional)_, `amount`, `to` _(optional)_ | `account`, `transaction`                             |
| `oracle.createRequest`    | `dataSourceId`, `payload` (string or serialisable object)     | `request`                                            |
| `triggers.register`       | `type`, `rule` _(optional)_, `config` _(map)_, `enabled` _(bool, default true)_ | `trigger`                                            |
| `automation.schedule`     | `name`, `schedule`, `description` _(optional)_, `enabled` _(optional)_ | `job`                                                |

Additional modules can be added incrementally. Validators inside the service
guard against missing parameters and produce meaningful error messages when an
action fails.

## TypeScript SDK

For local development and strong typing, install the companion package:

```bash
npm install @service-layer/devpack
```

```ts
import { ensureGasAccount, createOracleRequest, respond } from "@service-layer/devpack";

export default function handler(params: Record<string, unknown>) {
  ensureGasAccount({ wallet: String(params.wallet ?? "") });

  createOracleRequest({
    dataSourceId: String(params.oracleSource),
    payload: { pair: params.pair },
  });

  return respond.success({
    pair: params.pair,
    initiatedAt: new Date().toISOString(),
  });
}
```

Bundle the module output into the single function body uploaded to the Service
Layer. At runtime the helper functions forward to the injected `Devpack` global.

See `examples/functions/devpack` for a ready-to-build TypeScript project that
ensures a gas account and queues an oracle request before returning a success
payload.

### Ready-to-use JavaScript Samples

If you prefer plain JavaScript, the repository ships curated samples under
`examples/functions/devpack/js`:

| File | Purpose |
|------|---------|
| `gasbank_topup.js` | Ensures a gas account and queues a withdrawal to replenish funds. |
| `oracle_price_update.js` | Submits an oracle data request for a symbol. |
| `automation_guardrail.js` | Schedules automation and optionally registers a matching trigger. |

Each file exports a single function compatible with the Service Layer runtime.
Adjust the parameters and secrets to suit your workflow, then paste the body
when creating the function definition.

## Example Function

```js
const { gasBank, oracle, respond } = Devpack;

function handler(params) {
  gasBank.ensureAccount({ wallet: params.wallet });

  oracle.createRequest({
    dataSourceId: params.oracleSource,
    payload: { pair: params.pair }
  });

  return respond.success({
    pair: params.pair,
    initiatedAt: new Date().toISOString()
  });
}

handler;
```

After execution the persisted record (`execution.Actions`) will include the gas
bank account details and the oracle request metadata, allowing API consumers to
inspect the side-effects alongside the function response.
