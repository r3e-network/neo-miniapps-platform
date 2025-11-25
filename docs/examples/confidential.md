# Confidential Computing & DTA Quickstart

Guide to the Confidential Computing (TEE) and Decentralized Trading Architecture services.

## Overview

| Service | Purpose | Use Case |
|---------|---------|----------|
| **Confidential** | TEE enclave management | Secure key storage, attestation |
| **DTA** | Trading infrastructure | Product listing, order management |

## Prerequisites

```bash
export TOKEN=dev-token
export TENANT=tenant-a
export API=http://localhost:8080
export ACCOUNT_ID=<your-account-id>
```

---

# Confidential Computing Service

## Overview

The Confidential service manages Trusted Execution Environment (TEE) enclaves for secure computation and key storage.

### Key Concepts

- **Enclave**: Isolated secure execution environment
- **Sealed Key**: Cryptographic key bound to enclave
- **Attestation**: Proof of enclave integrity

## Quick Start

### 1. Create Enclave

```bash
ENCLAVE_ID=$(curl -s -X POST $API/accounts/$ACCOUNT_ID/confidential/enclaves \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "secure-signer",
    "type": "sgx",
    "measurement": "mrenclave-measurement-hash...",
    "signer_id": "mrsigner-hash...",
    "product_id": 1,
    "security_version": 1,
    "attributes": {
      "debug": false,
      "mode64bit": true
    },
    "status": "active"
  }' | jq -r .ID)

echo "Enclave ID: $ENCLAVE_ID"
```

### 2. Seal Key in Enclave

```bash
KEY_ID=$(curl -s -X POST $API/accounts/$ACCOUNT_ID/confidential/enclaves/$ENCLAVE_ID/keys \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "transaction-signer",
    "algorithm": "secp256k1",
    "purpose": "signing",
    "policy": {
      "require_attestation": true,
      "allowed_operations": ["sign", "verify"],
      "max_uses_per_hour": 1000
    }
  }' | jq -r .ID)

echo "Sealed Key ID: $KEY_ID"
```

### 3. Request Attestation

```bash
ATTEST_ID=$(curl -s -X POST $API/accounts/$ACCOUNT_ID/confidential/enclaves/$ENCLAVE_ID/attest \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "challenge": "random-nonce-'"$(openssl rand -hex 16)"'",
    "include_quote": true
  }' | jq -r .ID)

echo "Attestation ID: $ATTEST_ID"
```

### 4. Check Attestation

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  $API/accounts/$ACCOUNT_ID/confidential/attestations/$ATTEST_ID | jq
```

**Response**:
```json
{
  "ID": "attest-uuid",
  "EnclaveID": "enclave-uuid",
  "Challenge": "random-nonce-abc123...",
  "Quote": "sgx-quote-base64...",
  "Status": "verified",
  "VerifiedAt": "2025-01-15T10:00:01Z",
  "Report": {
    "mrenclave": "measurement-hash...",
    "mrsigner": "signer-hash...",
    "isv_prod_id": 1,
    "isv_svn": 1
  },
  "CreatedAt": "2025-01-15T10:00:00Z"
}
```

## API Reference

### Create Enclave

```http
POST /accounts/{account}/confidential/enclaves
```

```json
{
  "name": "enclave-name",
  "type": "sgx|sev|trustzone",
  "measurement": "mrenclave-hash",
  "signer_id": "mrsigner-hash",
  "product_id": 1,
  "security_version": 1,
  "status": "active"
}
```

### Seal Key

```http
POST /accounts/{account}/confidential/enclaves/{id}/keys
```

```json
{
  "name": "key-name",
  "algorithm": "secp256k1|ed25519|rsa2048",
  "purpose": "signing|encryption|key_exchange",
  "policy": {
    "require_attestation": true,
    "allowed_operations": ["sign", "verify"]
  }
}
```

### Request Attestation

```http
POST /accounts/{account}/confidential/enclaves/{id}/attest
```

```json
{
  "challenge": "random-nonce",
  "include_quote": true,
  "user_data": "optional-binding-data"
}
```

## CLI Usage

```bash
# List enclaves
slctl confcompute enclaves list --account $ACCOUNT_ID

# Create enclave
slctl confcompute enclaves create --account $ACCOUNT_ID \
  --name "my-enclave" \
  --type sgx \
  --measurement "mrenclave..."

# List sealed keys
slctl confcompute keys list --account $ACCOUNT_ID --enclave $ENCLAVE_ID

# Request attestation
slctl confcompute attest --account $ACCOUNT_ID \
  --enclave $ENCLAVE_ID \
  --challenge "nonce-123"

# List attestations
slctl confcompute attestations list --account $ACCOUNT_ID
```

## Use Cases

### Secure Transaction Signing

```bash
# 1. Create signing enclave
ENCLAVE_ID=$(curl -s -X POST $API/accounts/$ACCOUNT_ID/confidential/enclaves \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "tx-signer",
    "type": "sgx",
    "measurement": "...",
    "status": "active"
  }' | jq -r .ID)

# 2. Seal signing key
curl -s -X POST $API/accounts/$ACCOUNT_ID/confidential/enclaves/$ENCLAVE_ID/keys \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "neo-signer",
    "algorithm": "secp256k1",
    "purpose": "signing",
    "policy": {"require_attestation": true}
  }'

# 3. Get attestation before signing
curl -s -X POST $API/accounts/$ACCOUNT_ID/confidential/enclaves/$ENCLAVE_ID/attest \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"challenge": "tx-hash-abc123"}'
```

### Key Custody Service

```javascript
// Devpack function for custodial signing
export default async function(params, secrets) {
  // Verify attestation is fresh
  const attestation = Devpack.confidential.attest({
    enclaveId: params.enclaveId,
    challenge: params.txHash
  });

  if (attestation.status !== 'verified') {
    return Devpack.respond.error('Attestation failed');
  }

  // Sign transaction in enclave
  const signature = Devpack.confidential.sign({
    enclaveId: params.enclaveId,
    keyId: params.keyId,
    data: params.txHash,
    attestationId: attestation.id
  });

  return Devpack.respond.success({
    signature: signature,
    attestation: attestation.id
  });
}
```

---

# DTA Service (Decentralized Trading Architecture)

## Overview

DTA provides infrastructure for decentralized trading products and order management.

### Key Concepts

- **Product**: Trading instrument definition
- **Order**: Trade request from user
- **Execution**: Matched and settled trade

## Quick Start

### 1. Create Trading Product

```bash
PRODUCT_ID=$(curl -s -X POST $API/accounts/$ACCOUNT_ID/dta/products \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "NEO-USDT-PERP",
    "symbol": "NEOUSDT",
    "type": "perpetual",
    "base_asset": "NEO",
    "quote_asset": "USDT",
    "tick_size": "0.001",
    "lot_size": "0.01",
    "min_order_size": "1",
    "max_order_size": "10000",
    "maker_fee_bps": 10,
    "taker_fee_bps": 20,
    "status": "active"
  }' | jq -r .ID)

echo "Product ID: $PRODUCT_ID"
```

### 2. Submit Order

```bash
ORDER_ID=$(curl -s -X POST $API/accounts/$ACCOUNT_ID/dta/orders \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": "'"$PRODUCT_ID"'",
    "client_order_id": "order-'"$(date +%s)"'",
    "side": "buy",
    "type": "limit",
    "price": "12.500",
    "quantity": "10.00",
    "time_in_force": "GTC",
    "reduce_only": false
  }' | jq -r .ID)

echo "Order ID: $ORDER_ID"
```

### 3. Check Order Status

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  $API/accounts/$ACCOUNT_ID/dta/orders/$ORDER_ID | jq
```

**Response**:
```json
{
  "ID": "order-uuid",
  "ProductID": "product-uuid",
  "ClientOrderID": "order-1705312800",
  "Side": "buy",
  "Type": "limit",
  "Price": "12.500",
  "Quantity": "10.00",
  "FilledQuantity": "5.00",
  "Status": "partially_filled",
  "TimeInForce": "GTC",
  "CreatedAt": "2025-01-15T10:00:00Z",
  "UpdatedAt": "2025-01-15T10:00:05Z"
}
```

### 4. List Orders

```bash
# All orders
curl -s -H "Authorization: Bearer $TOKEN" \
  "$API/accounts/$ACCOUNT_ID/dta/orders?limit=20" | jq

# Open orders only
curl -s -H "Authorization: Bearer $TOKEN" \
  "$API/accounts/$ACCOUNT_ID/dta/orders?status=open&limit=20" | jq
```

## API Reference

### Create Product

```http
POST /accounts/{account}/dta/products
```

```json
{
  "name": "Product Name",
  "symbol": "SYMBOL",
  "type": "spot|perpetual|future|option",
  "base_asset": "NEO",
  "quote_asset": "USDT",
  "tick_size": "0.001",
  "lot_size": "0.01",
  "min_order_size": "1",
  "max_order_size": "10000",
  "maker_fee_bps": 10,
  "taker_fee_bps": 20,
  "status": "active"
}
```

### Submit Order

```http
POST /accounts/{account}/dta/orders
```

```json
{
  "product_id": "product-uuid",
  "client_order_id": "unique-id",
  "side": "buy|sell",
  "type": "limit|market|stop_limit|stop_market",
  "price": "12.500",
  "quantity": "10.00",
  "time_in_force": "GTC|IOC|FOK|GTD",
  "stop_price": "12.000",
  "reduce_only": false
}
```

### Order Types

| Type | Description | Required Fields |
|------|-------------|-----------------|
| `market` | Execute at best price | quantity |
| `limit` | Execute at price or better | price, quantity |
| `stop_market` | Market order when stop hit | stop_price, quantity |
| `stop_limit` | Limit order when stop hit | stop_price, price, quantity |

### Time in Force

| TIF | Description |
|-----|-------------|
| `GTC` | Good Till Cancelled |
| `IOC` | Immediate or Cancel |
| `FOK` | Fill or Kill |
| `GTD` | Good Till Date |

## CLI Usage

```bash
# List products
slctl dta products list --account $ACCOUNT_ID

# Create product
slctl dta products create --account $ACCOUNT_ID \
  --name "NEO-USDT" \
  --base NEO \
  --quote USDT \
  --type spot

# Submit order
slctl dta orders create --account $ACCOUNT_ID \
  --product $PRODUCT_ID \
  --side buy \
  --type limit \
  --price 12.50 \
  --quantity 10

# List orders
slctl dta orders list --account $ACCOUNT_ID --status open

# Get order
slctl dta orders get --account $ACCOUNT_ID --order $ORDER_ID
```

## Use Cases

### Market Making Bot

```javascript
// Devpack function for market making
export default async function(params, secrets) {
  const { productId, spread, size } = params;

  // Get current mid price from price feed
  const price = Devpack.priceFeeds.getLatest({ feedId: params.priceFeedId });
  const midPrice = price.aggregatedPrice;

  // Calculate bid/ask
  const bidPrice = midPrice * (1 - spread / 100);
  const askPrice = midPrice * (1 + spread / 100);

  // Submit bid
  const bid = Devpack.dta.submitOrder({
    productId,
    side: 'buy',
    type: 'limit',
    price: bidPrice.toFixed(3),
    quantity: size.toString(),
    timeInForce: 'GTC'
  });

  // Submit ask
  const ask = Devpack.dta.submitOrder({
    productId,
    side: 'sell',
    type: 'limit',
    price: askPrice.toFixed(3),
    quantity: size.toString(),
    timeInForce: 'GTC'
  });

  return Devpack.respond.success({
    bidOrderId: bid.id,
    askOrderId: ask.id,
    midPrice,
    spread
  });
}
```

### Automated Trading Strategy

```bash
# Create perpetual product
PRODUCT_ID=$(curl -s -X POST $API/accounts/$ACCOUNT_ID/dta/products \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "NEO-PERP",
    "type": "perpetual",
    "base_asset": "NEO",
    "quote_asset": "USDT",
    "tick_size": "0.01",
    "lot_size": "0.1",
    "status": "active"
  }' | jq -r .ID)

# Submit stop-loss order
curl -s -X POST $API/accounts/$ACCOUNT_ID/dta/orders \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": "'"$PRODUCT_ID"'",
    "side": "sell",
    "type": "stop_market",
    "stop_price": "11.00",
    "quantity": "100",
    "reduce_only": true
  }'
```

## Order Lifecycle

```
new → open → partially_filled → filled
          → cancelled
          → rejected
          → expired
```

## Best Practices

### Confidential Computing

1. **Verify Attestations**: Always verify attestation before sensitive operations
2. **Fresh Challenges**: Use unique random nonces for each attestation
3. **Key Policies**: Set strict policies on sealed keys
4. **Audit Trail**: Log all enclave operations

### DTA Trading

1. **Use Client Order IDs**: Enable idempotent order submission
2. **Set Appropriate TIF**: Use IOC/FOK for time-sensitive orders
3. **Implement Rate Limiting**: Respect API rate limits
4. **Handle Partial Fills**: Design for partially filled orders

## Error Handling

### Confidential Errors

| Status | Meaning | Action |
|--------|---------|--------|
| `attestation_failed` | Quote verification failed | Check enclave config |
| `key_not_found` | Sealed key doesn't exist | Create key first |
| `policy_violation` | Operation not allowed | Check key policy |

### DTA Errors

| Status | Meaning | Action |
|--------|---------|--------|
| `insufficient_funds` | Not enough balance | Add funds |
| `invalid_price` | Price outside tick size | Adjust price |
| `invalid_quantity` | Below/above limits | Adjust quantity |
| `product_inactive` | Product not tradeable | Check product status |

## Related Documentation

- [Service Catalog](../service-catalog.md)
- [Security Hardening](../security-hardening.md)
- [Architecture Layers](../architecture-layers.md)
