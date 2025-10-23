# External Integration Endpoints

The refactored runtime defers price feed refresh, oracle completion and gas bank
withdrawal settlement to external HTTP services. The following sections describe
the minimal API surface each integration must expose.

## Price Feed Fetcher

Set both environment variables (or config entries):

```
PRICEFEED_FETCH_URL=https://prices.example.com/v1/quote
PRICEFEED_FETCH_KEY=your-token-here  # optional
```

The refresher issues a GET request with the following query parameters:

```
GET /v1/quote?base=NEO&quote=USD HTTP/1.1
Authorization: Bearer <PRICEFEED_FETCH_KEY>  (if provided)
```

Expected JSON response:

```json
{
  "price": 10.42,
  "source": "example-provider"
}
```

* `price` must be positive.
* `source` is optional; the host name is used when omitted.

## Oracle Resolver

Configure the resolver endpoint and optional token:

```
ORACLE_RESOLVER_URL=https://oracle.example.com/v1/requests/status
ORACLE_RESOLVER_KEY=service-token
```

The dispatcher will poll using:

```
GET /v1/requests/status?request_id=<uuid>
Authorization: Bearer <ORACLE_RESOLVER_KEY>
```

Expected JSON response:

```json
{
  "done": true,
  "success": true,
  "result": "{\"value\":123}",
  "error": "",
  "retry_after_seconds": 5
}
```

Fields:

* `done` – `false` indicates the dispatcher should poll again after
  `retry_after_seconds` (defaults to five seconds when omitted).
* `success` – set to `true` when the request finished successfully; otherwise
  `error` is propagated back through the API.
* `result` – raw payload stored on the request record.

## Gas Bank Withdrawal Resolver

```
GASBANK_RESOLVER_URL=https://gas.example.com/v1/withdrawals/status
GASBANK_RESOLVER_KEY=settlement-token
```

Poll request format:

```
GET /v1/withdrawals/status?transaction_id=<uuid>
Authorization: Bearer <GASBANK_RESOLVER_KEY>
```

Expected JSON response:

```json
{
  "done": true,
  "success": false,
  "message": "insufficient funds",
  "retry_after_seconds": 10
}
```

* When `done` is `false`, the poller retries after `retry_after_seconds`
  (defaults to five seconds when omitted).
* When `done` is `true`, a failed withdrawal returns the `message` which is
  persisted on the transaction as the error.

## Error Handling

All endpoints should return `200 OK` with the JSON payloads described above when
reachable. Any other status code is treated as a transient failure and retried
after a short delay. If your upstream distinguishes between temporary and
permanent failures, encode the retry interval using `retry_after_seconds`.
