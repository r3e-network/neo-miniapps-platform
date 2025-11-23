# Service Layer API Examples

Pragmatic, copy-pasteable examples for each major service. Replace `<TOKEN>` with a valid bearer and `<ACCOUNT_ID>` with your account ID.

All commands assume the appserver is running at `http://localhost:8080` and that you set `export TOKEN=<TOKEN>`.

## Accounts
```bash
curl -s -X POST http://localhost:8080/accounts \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"owner":"alice"}'

curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/accounts
```

## Secrets
```bash
curl -s -X POST http://localhost:8080/accounts/<ACCOUNT_ID>/secrets \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"apiKey","value":"super-secret"}'
```

## Functions
```bash
FUNC_ID=$(curl -s -X POST http://localhost:8080/accounts/<ACCOUNT_ID>/functions \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"hello","runtime":"js","source":"(params,secrets)=>({echo:params.msg,secret:secrets.apiKey})","secrets":["apiKey"]}' | jq -r .ID)

curl -s -X POST http://localhost:8080/accounts/<ACCOUNT_ID>/functions/$FUNC_ID/execute \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"msg":"hi"}'
```

## Automation
```bash
curl -s -X POST http://localhost:8080/accounts/<ACCOUNT_ID>/automation/jobs \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"function_id":"'"$FUNC_ID"'","schedule":"*/5 * * * *"}'
```

## Oracle (HTTP adapter)
Create a source, request, fail, and retry:
```bash
SRC_ID=$(curl -s -X POST http://localhost:8080/accounts/<ACCOUNT_ID>/oracle/sources \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"prices","url":"https://api.example.com","method":"GET"}' | jq -r .ID)

REQ_ID=$(curl -s -X POST http://localhost:8080/accounts/<ACCOUNT_ID>/oracle/requests \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"data_source_id":"'"$SRC_ID"'","payload":"{}"}' | jq -r .ID)

# runner marks running/succeeded (requires X-Oracle-Runner-Token when runner
# tokens are configured; API token still required)
# configure runner tokens via env: export ORACLE_RUNNER_TOKENS=runner-secret
curl -s -X PATCH http://localhost:8080/accounts/<ACCOUNT_ID>/oracle/requests/$REQ_ID \
  -H "Authorization: Bearer $TOKEN" -H "X-Oracle-Runner-Token: runner-secret" \
  -H "Content-Type: application/json" -d '{"status":"running"}'

curl -s -X PATCH http://localhost:8080/accounts/<ACCOUNT_ID>/oracle/requests/$REQ_ID \
  -H "Authorization: Bearer $TOKEN" -H "X-Oracle-Runner-Token: runner-secret" \
  -H "Content-Type: application/json" -d '{"status":"failed","error":"upstream error"}'

# retry is allowed without runner token
curl -s -X PATCH http://localhost:8080/accounts/<ACCOUNT_ID>/oracle/requests/$REQ_ID \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"status":"retry"}'

# slctl runner callback example (env-supplied runner tokens)
# export SERVICE_LAYER_TOKEN=$TOKEN
# export ORACLE_RUNNER_TOKENS=runner-secret
slctl oracle requests create --account <ACCOUNT_ID> --source "$SRC_ID" --token "$TOKEN" --payload '{}'
slctl oracle requests list --account <ACCOUNT_ID> --token "$TOKEN" --status pending --limit 1
REQ_ID=$(slctl oracle requests list --account <ACCOUNT_ID> --token "$TOKEN" --status pending --limit 1 | jq -r '.[0].ID')
curl -s -X PATCH http://localhost:8080/accounts/<ACCOUNT_ID>/oracle/requests/$REQ_ID \
  -H "Authorization: Bearer $TOKEN" -H "X-Oracle-Runner-Token: runner-secret" \
  -H "Content-Type: application/json" -d '{"status":"running"}'

# paginate + filter
curl -i -H "Authorization: Bearer $TOKEN" "http://localhost:8080/accounts/<ACCOUNT_ID>/oracle/requests?status=failed&limit=1"
```

## Datafeeds
Create a feed with per-feed aggregation (median|mean|min|max):
```bash
curl -s -X POST http://localhost:8080/accounts/<ACCOUNT_ID>/datafeeds \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"pair":"NEO/USD","decimals":8,"aggregation":"median","threshold_ppm":5000,"signer_set":["'"$TEST_WALLET"'"]}'
```

Submit signed updates:
```bash
curl -s -X POST http://localhost:8080/accounts/<ACCOUNT_ID>/datafeeds/<FEED_ID>/updates \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"round_id":1,"price":"12.34","signer":"'"$TEST_WALLET"'","signature":"sig"}'

curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/accounts/<ACCOUNT_ID>/datafeeds/<FEED_ID>/latest"
```

## Pricefeeds
```bash
curl -s -X POST http://localhost:8080/accounts/<ACCOUNT_ID>/pricefeeds \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"pair":"NEO/USD","deviation_percent":1.5,"heartbeat":60000000000}'
```

## Gasbank
```bash
curl -s -X POST http://localhost:8080/accounts/<ACCOUNT_ID>/gasbank/deposit \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"amount":5.0,"tx_id":"tx123"}'

curl -s -X POST http://localhost:8080/accounts/<ACCOUNT_ID>/gasbank/withdraw \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"amount":1.0,"to_address":"ADDR"}'
```

## Data Streams
```bash
STREAM_ID=$(curl -s -X POST http://localhost:8080/accounts/<ACCOUNT_ID>/datastreams \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"ticker","symbol":"TCKR","description":"demo stream","frequency":"1s","sla_ms":50,"status":"active"}' | jq -r .ID)

curl -s -X POST http://localhost:8080/accounts/<ACCOUNT_ID>/datastreams/$STREAM_ID/frames \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"sequence":1,"payload":{"price":123.45},"latency_ms":10,"status":"delivered"}'

curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/accounts/<ACCOUNT_ID>/datastreams/$STREAM_ID/frames?limit=5"
```

## DataLink
```bash
curl -s -X POST http://localhost:8080/accounts/<ACCOUNT_ID>/workspace-wallets \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"wallet_address":"<WALLET>","label":"link-signer","status":"active"}'

CH_ID=$(curl -s -X POST http://localhost:8080/accounts/<ACCOUNT_ID>/datalink/channels \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"provider-1","endpoint":"https://api.provider.test","signer_set":["<WALLET>"],"status":"active"}' | jq -r .ID)

curl -s -X POST http://localhost:8080/accounts/<ACCOUNT_ID>/datalink/channels/$CH_ID/deliveries \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"payload":{"data":"hello"},"metadata":{"trace":"123"}}'

curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/accounts/<ACCOUNT_ID>/datalink/deliveries?limit=5"
```

## Randomness

Devpack action (inside a function):
```js
const rand = Devpack.random.generate({ length: 64, requestId: "example" });
return Devpack.respond.success({ action: rand.asResult() });
```

HTTP/CLI calls:
```bash
curl -s -X POST http://localhost:8080/accounts/<ACCOUNT_ID>/random \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"length":32,"request_id":"example"}'
```

## Devpack Quick Actions (inside functions)

```js
// Data feeds: submit signer update
Devpack.dataFeeds.submitUpdate({ feedId: "feed-1", roundId: 1, price: "12.34" });
// Data streams: publish a frame
Devpack.dataStreams.publishFrame({ streamId: "stream-1", sequence: 42, payload: { price: 99.1 } });
// DataLink: enqueue delivery to a channel
Devpack.dataLink.createDelivery({ channelId: "channel-1", payload: { foo: "bar" }, metadata: { trace: "abc" } });
```

## CLI equivalents (slctl)
```bash
slctl accounts list --token "$TOKEN"
slctl oracle requests list --account <ACCOUNT_ID> --token "$TOKEN" --status failed --limit 5
slctl oracle requests retry --account <ACCOUNT_ID> --token "$TOKEN" --id "$REQ_ID"
slctl gasbank deposit --account <ACCOUNT_ID> --token "$TOKEN" --amount 5
slctl random generate --account <ACCOUNT_ID> --token "$TOKEN" --length 16
slctl datastreams create --account <ACCOUNT_ID> --name ticker --symbol TCKR --frequency 1s --status active
slctl datastreams publish --account <ACCOUNT_ID> --stream <STREAM_ID> --sequence 1 --payload '{"price":123.45}'
slctl datalink channel-create --account <ACCOUNT_ID> --name provider-1 --endpoint https://api.provider.test --signers WALLET1
slctl datalink deliver --account <ACCOUNT_ID> --channel <CHANNEL_ID> --payload '{"data":"hello"}'
```
