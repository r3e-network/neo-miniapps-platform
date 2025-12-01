# DTA Service

**Package ID:** `com.r3e.services.dta`
**Version:** 1.0.0
**Description:** Decentralized Token Automation - manages subscription and redemption products with order lifecycle tracking

## Overview

The DTA (Decentralized Token Automation) service provides a complete product and order management system for tokenized financial instruments. It enables accounts to define tradable products (e.g., bonds, securities, derivatives) and process subscription/redemption orders with wallet ownership validation and event-driven observability.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        DTA Service                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐      ┌──────────────┐                   │
│  │   Product    │      │    Order     │                   │
│  │  Management  │      │  Management  │                   │
│  └──────┬───────┘      └──────┬───────┘                   │
│         │                     │                            │
│         └──────────┬──────────┘                            │
│                    │                                       │
│         ┌──────────▼──────────┐                           │
│         │  Service Engine     │                           │
│         │  - Observability    │                           │
│         │  - Event Bus        │                           │
│         │  - Metrics          │                           │
│         └──────────┬──────────┘                           │
│                    │                                       │
└────────────────────┼───────────────────────────────────────┘
                     │
         ┌───────────┼───────────┐
         │           │           │
    ┌────▼────┐ ┌───▼────┐ ┌───▼────────┐
    │ Store   │ │Accounts│ │  Wallets   │
    │(Postgres│ │Checker │ │  Checker   │
    └─────────┘ └────────┘ └────────────┘
```

## Key Components

### Service (`Service`)
Core service implementation that orchestrates product and order operations.

**Responsibilities:**
- Product lifecycle management (create, update, retrieve, list)
- Order creation and tracking
- Account and wallet ownership validation
- Observability hooks integration
- Event publishing for order creation
- HTTP API endpoint handlers

**Dependencies:**
- `Store`: Persistence layer for products and orders
- `AccountChecker`: Validates account existence
- `WalletChecker`: Validates wallet ownership (optional)
- `Logger`: Structured logging
- `ServiceEngine`: Framework capabilities (metrics, events, observability)

### Store Interface (`Store`)
Defines the persistence contract for DTA entities.

**Methods:**
- `CreateProduct(ctx, Product) (Product, error)`
- `UpdateProduct(ctx, Product) (Product, error)`
- `GetProduct(ctx, id) (Product, error)`
- `ListProducts(ctx, accountID) ([]Product, error)`
- `CreateOrder(ctx, Order) (Order, error)`
- `GetOrder(ctx, id) (Order, error)`
- `ListOrders(ctx, accountID, limit) ([]Order, error)`

**Implementation:** `PostgresStore` (store_postgres.go)

### Package (`Package`)
ServicePackage implementation for runtime registration and initialization.

**Initialization:**
- Registers package ID `com.r3e.services.dta` at init time
- Creates PostgreSQL store with database connection
- Wires AccountChecker and Logger dependencies
- Returns configured Service instance

## Domain Types

### Product
Represents a tradable financial product (bond, security, derivative).

```go
type Product struct {
    ID              string            // Unique identifier
    AccountID       string            // Owning account
    Name            string            // Product name (required)
    Symbol          string            // Trading symbol (required, uppercase)
    Type            string            // Product type (normalized lowercase)
    Status          ProductStatus     // inactive|active|suspended
    SettlementTerms string            // Settlement description
    Metadata        map[string]string // Extensible key-value data
    CreatedAt       time.Time
    UpdatedAt       time.Time
}
```

**Product Status Values:**
- `inactive`: Product not available for trading (default)
- `active`: Product available for orders
- `suspended`: Temporarily disabled

**Validation Rules:**
- `Name` and `Symbol` are required
- `Symbol` is normalized to uppercase
- `Type` is normalized to lowercase
- `Status` defaults to `inactive` if not specified

### Order
Represents a subscription or redemption request for a product.

```go
type Order struct {
    ID        string            // Unique identifier
    AccountID string            // Owning account
    ProductID string            // Target product
    Type      OrderType         // subscription|redemption
    Amount    string            // Order amount (required)
    Wallet    string            // Wallet address (required, lowercase)
    Status    OrderStatus       // pending|approved|settled|rejected|canceled
    Error     string            // Error message if failed
    Metadata  map[string]string // Extensible key-value data
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

**Order Types:**
- `subscription`: Purchase/subscribe to product
- `redemption`: Sell/redeem product

**Order Status Lifecycle:**
- `pending`: Initial state after creation
- `approved`: Order validated and approved
- `settled`: Order completed and settled
- `rejected`: Order rejected (see Error field)
- `canceled`: Order canceled by user

**Validation Rules:**
- `ProductID` must exist and belong to account
- `Type` must be `subscription` or `redemption`
- `Amount` is required (non-empty string)
- `Wallet` is required and normalized to lowercase
- Wallet ownership is validated if WalletChecker is configured

## API Endpoints

All endpoints are automatically registered via the declarative HTTP method naming convention (`HTTP{Method}{Path}`).

### Products

#### List Products
```
GET /products
```
Returns all products for the authenticated account.

**Response:** `[]Product`

#### Create Product
```
POST /products
```
Creates a new product for the authenticated account.

**Request Body:**
```json
{
  "name": "Corporate Bond Series A",
  "symbol": "CBSA",
  "type": "bond",
  "settlement_terms": "T+2 settlement",
  "status": "active",
  "metadata": {
    "issuer": "ACME Corp",
    "maturity": "2030-12-31"
  }
}
```

**Response:** `Product`

#### Get Product
```
GET /products/{id}
```
Retrieves a specific product. Validates ownership.

**Response:** `Product`

#### Update Product
```
PATCH /products/{id}
```
Updates product fields. Only provided fields are modified.

**Request Body:**
```json
{
  "status": "suspended",
  "settlement_terms": "T+3 settlement"
}
```

**Response:** `Product`

### Orders

#### List Orders
```
GET /orders?limit=50
```
Returns recent orders for the authenticated account.

**Query Parameters:**
- `limit`: Maximum results (default: 100, max: 1000)

**Response:** `[]Order`

#### Create Order
```
POST /orders
```
Creates a subscription or redemption order.

**Request Body:**
```json
{
  "product_id": "prod_abc123",
  "type": "subscription",
  "amount": "10000.00",
  "wallet_address": "0x742d35cc6634c0532925a3b844bc9e7595f0beb",
  "metadata": {
    "reference": "ORDER-2025-001"
  }
}
```

**Response:** `Order`

**Events Published:**
- `dta.order.created` with payload:
  ```json
  {
    "order_id": "ord_xyz789",
    "account_id": "acc_123",
    "product_id": "prod_abc123",
    "type": "subscription"
  }
  ```

#### Get Order
```
GET /orders/{id}
```
Retrieves a specific order. Validates ownership.

**Response:** `Order`

## Configuration

### Service Configuration
Defined in `manifest.yaml`:

```yaml
package_id: com.r3e.services.dta
version: "1.0.0"
display_name: "DTA Service"
description: "Decentralized Token Automation"
```

### Permissions
- `system.api.storage` (required): Data persistence
- `system.api.bus` (optional): Event publishing

### Resource Limits
- Max storage: 100 MB
- Max concurrent requests: 1000
- Max requests/second: 5000
- Max events/second: 1000

### Service Dependencies
- `store`: Database persistence (required)
- `svc-accounts`: Account validation (required)
- Wallet service: Wallet ownership validation (optional)

## Dependencies

### Required
- `github.com/R3E-Network/service_layer/pkg/logger`: Structured logging
- `github.com/R3E-Network/service_layer/system/core`: Core engine types
- `github.com/R3E-Network/service_layer/system/framework`: Service framework
- `github.com/R3E-Network/service_layer/system/runtime`: Package runtime

### Database
- PostgreSQL 12+ (via `store_postgres.go`)
- Tables: `dta_products`, `dta_orders`

## Observability

### Metrics
The service emits the following counters:
- `dta_products_created_total{account_id}`
- `dta_products_updated_total{account_id}`
- `dta_orders_created_total{account_id,product_id}`

### Logging
Structured logs with fields:
- `product_id`: Product identifier
- `order_id`: Order identifier
- `account_id`: Account identifier
- `order_type`: subscription|redemption

### Events
- `dta.order.created`: Published when order is successfully created

### Observation Hooks
Supports custom observation hooks via `WithObservationHooks()` for:
- Request tracing
- Performance monitoring
- Custom analytics

## Testing

### Run Unit Tests
```bash
cd /home/neo/git/service_layer/packages/com.r3e.services.dta
go test -v
```

### Test Coverage
```bash
go test -cover
```

### Integration Tests
Requires:
- PostgreSQL test database
- Mock AccountChecker
- Mock WalletChecker (optional)

See `service_test.go` for test examples.

## Usage Example

```go
package main

import (
    "context"
    "github.com/R3E-Network/service_layer/packages/com.r3e.services.dta"
    "github.com/R3E-Network/service_layer/pkg/logger"
)

func main() {
    log := logger.New()
    accounts := NewAccountChecker()
    store := dta.NewPostgresStore(db, accounts)

    svc := dta.New(accounts, store, log)

    // Optional: Add wallet validation
    wallets := NewWalletChecker()
    svc.WithWalletChecker(wallets)

    // Create a product
    product := dta.Product{
        AccountID: "acc_123",
        Name:      "Treasury Bond",
        Symbol:    "TBOND",
        Type:      "bond",
        Status:    dta.ProductStatusActive,
    }

    created, err := svc.CreateProduct(context.Background(), product)
    if err != nil {
        log.Fatal(err)
    }

    // Create an order
    order, err := svc.CreateOrder(
        context.Background(),
        "acc_123",
        created.ID,
        dta.OrderTypeSubscription,
        "50000.00",
        "0x742d35cc6634c0532925a3b844bc9e7595f0beb",
        map[string]string{"ref": "ORD-001"},
    )
    if err != nil {
        log.Fatal(err)
    }
}
```

## Security Considerations

1. **Ownership Validation**: All operations validate that the requesting account owns the resource
2. **Wallet Verification**: Orders require wallet ownership validation when WalletChecker is configured
3. **Input Normalization**: All string inputs are trimmed and normalized (symbols uppercase, types lowercase)
4. **Metadata Sanitization**: Metadata keys/values are normalized via `core.NormalizeMetadata()`
5. **Error Handling**: Sensitive error details are not exposed to clients

## Error Handling

Common error scenarios:
- `account not found`: Account does not exist
- `product not found`: Product ID invalid or not owned by account
- `invalid order type`: Order type must be subscription or redemption
- `wallet_address required`: Wallet address is missing
- `wallet not owned by account`: Wallet ownership validation failed
- `invalid status`: Product status must be inactive, active, or suspended

## Future Enhancements

Potential improvements:
- Order status transitions (approve, settle, reject, cancel)
- Product search and filtering
- Order matching engine
- Settlement workflow integration
- Multi-currency support
- Batch order processing
- Audit trail for compliance

## License

MIT License - Copyright (c) R3E Network

## Support

For issues or questions:
- File an issue in the service_layer repository
- Contact: R3E Network development team
