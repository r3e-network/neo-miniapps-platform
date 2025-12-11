# NeoAccounts Multi-Token Balance Refactoring

## Development Plan

**Created**: 2025-12-10
**Status**: Ready for Implementation
**Coverage Target**: ≥90%

---

## 1. Overview

Refactor the NeoAccounts (account pool) service to support multiple token balances per account. Currently, each account has a single `Balance` field. This refactoring introduces a flexible multi-token system supporting NEO and GAS initially, with extensibility for future NEP-17 tokens.

### Requirements Summary

| Requirement | Decision |
|-------------|----------|
| Scope | Full refactor (data model + API + handlers) |
| Token Support | NEO (indivisible), GAS (divisible) |
| TxCount Tracking | Account-level only (not per-token) |
| Extensibility | Design for arbitrary NEP-17 tokens |
| Migration | Clean slate (no data migration) |

---

## 2. Technical Design

### 2.1 Data Model Changes

#### Current Model (`services/neoaccounts/supabase/models.go`)

```go
type Account struct {
    ID         string    `json:"id"`
    Address    string    `json:"address"`
    Balance    int64     `json:"balance"`        // REMOVE: Single balance
    CreatedAt  time.Time `json:"created_at"`
    LastUsedAt time.Time `json:"last_used_at"`
    TxCount    int64     `json:"tx_count"`
    IsRetiring bool      `json:"is_retiring"`
    LockedBy   string    `json:"locked_by,omitempty"`
    LockedAt   time.Time `json:"locked_at,omitempty"`
}
```

#### New Model Design

```go
// Account - identity and locking metadata (no balance field)
type Account struct {
    ID         string    `json:"id"`
    Address    string    `json:"address"`
    CreatedAt  time.Time `json:"created_at"`
    LastUsedAt time.Time `json:"last_used_at"`
    TxCount    int64     `json:"tx_count"`
    IsRetiring bool      `json:"is_retiring"`
    LockedBy   string    `json:"locked_by,omitempty"`
    LockedAt   time.Time `json:"locked_at,omitempty"`
}

// AccountBalance - per-token balance (new table: pool_account_balances)
type AccountBalance struct {
    AccountID   string    `json:"account_id"`
    TokenType   string    `json:"token_type"`    // "NEO", "GAS", or custom
    ScriptHash  string    `json:"script_hash"`   // NEP-17 contract hash
    Amount      int64     `json:"amount"`
    Decimals    int       `json:"decimals"`
    UpdatedAt   time.Time `json:"updated_at"`
}

// AccountWithBalances - composite for API responses
type AccountWithBalances struct {
    Account
    Balances map[string]TokenBalance `json:"balances"` // key: token_type
}

// TokenBalance - API representation
type TokenBalance struct {
    TokenType  string    `json:"token_type"`
    ScriptHash string    `json:"script_hash"`
    Amount     int64     `json:"amount"`
    Decimals   int       `json:"decimals"`
    UpdatedAt  time.Time `json:"updated_at,omitempty"`
}
```

### 2.2 Database Schema

**New Table: `pool_account_balances`**

```sql
CREATE TABLE pool_account_balances (
    account_id   UUID NOT NULL REFERENCES pool_accounts(id) ON DELETE CASCADE,
    token_type   VARCHAR(32) NOT NULL,
    script_hash  VARCHAR(66) NOT NULL,
    amount       BIGINT NOT NULL DEFAULT 0,
    decimals     INT NOT NULL DEFAULT 8,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, token_type)
);

CREATE INDEX idx_pool_account_balances_token ON pool_account_balances(token_type);
CREATE INDEX idx_pool_account_balances_amount ON pool_account_balances(token_type, amount);
```

**Migration: Drop `balance` column from `pool_accounts`**

```sql
ALTER TABLE pool_accounts DROP COLUMN IF EXISTS balance;
```

### 2.3 API Contract Changes

#### AccountInfo Response (Updated)

```json
{
  "id": "uuid",
  "address": "NAddr...",
  "created_at": "2025-12-10T00:00:00Z",
  "last_used_at": "2025-12-10T00:00:00Z",
  "tx_count": 42,
  "is_retiring": false,
  "locked_by": "neovault",
  "locked_at": "2025-12-10T00:00:00Z",
  "balances": {
    "GAS": {
      "token_type": "GAS",
      "script_hash": "0xd2a4cff31913016155e38e474a2c06d08be276cf",
      "amount": 100000000,
      "decimals": 8
    },
    "NEO": {
      "token_type": "NEO",
      "script_hash": "0xef4073a0f2b305a38ec4050e4d3d28bc40ea63f5",
      "amount": 10,
      "decimals": 0
    }
  }
}
```

#### Endpoint Changes

| Endpoint | Change |
|----------|--------|
| `GET /accounts` | Add `?token=GAS&min_balance=1000000` query params |
| `POST /balance` | Rename to `/balances`, require `token` field |
| `GET /info` | Return `token_stats` array instead of `total_balance` |

#### UpdateBalanceInput (Updated)

```go
type UpdateBalanceInput struct {
    ServiceID string  `json:"service_id"`
    AccountID string  `json:"account_id"`
    Token     string  `json:"token"`           // NEW: "GAS" or "NEO"
    Delta     *int64  `json:"delta,omitempty"`
    Absolute  *int64  `json:"absolute,omitempty"`
}
```

#### PoolInfoResponse (Updated)

```go
type PoolInfoResponse struct {
    TotalAccounts    int                    `json:"total_accounts"`
    ActiveAccounts   int                    `json:"active_accounts"`
    LockedAccounts   int                    `json:"locked_accounts"`
    RetiringAccounts int                    `json:"retiring_accounts"`
    TokenStats       map[string]TokenStats  `json:"token_stats"`
}

type TokenStats struct {
    TokenType    string `json:"token_type"`
    ScriptHash   string `json:"script_hash"`
    TotalBalance int64  `json:"total_balance"`
    LockedBalance int64 `json:"locked_balance"`
    AvailableBalance int64 `json:"available_balance"`
}
```

### 2.4 Repository Interface Changes

```go
type RepositoryInterface interface {
    // Existing (modified)
    Create(ctx context.Context, acc *Account) error
    Update(ctx context.Context, acc *Account) error
    GetByID(ctx context.Context, id string) (*Account, error)
    List(ctx context.Context) ([]Account, error)
    Delete(ctx context.Context, id string) error

    // Balance-aware methods (new)
    GetWithBalances(ctx context.Context, id string) (*AccountWithBalances, error)
    ListWithBalances(ctx context.Context) ([]AccountWithBalances, error)
    ListAvailableWithBalances(ctx context.Context, token string, minBalance *int64, limit int) ([]AccountWithBalances, error)
    ListByLockerWithBalances(ctx context.Context, lockerID string) ([]AccountWithBalances, error)

    // Balance operations (new)
    UpsertBalance(ctx context.Context, accountID, tokenType, scriptHash string, amount int64, decimals int) error
    GetBalance(ctx context.Context, accountID, tokenType string) (*AccountBalance, error)
    GetBalances(ctx context.Context, accountID string) ([]AccountBalance, error)
    AggregateTokenStats(ctx context.Context, tokenType string) (*TokenStats, error)
}
```

---

## 3. Task Breakdown

### Task 1: Schema & Repository Foundation

**Owner**: Backend Developer
**Priority**: P0 (Blocking)
**Estimated Effort**: Medium

#### Files to Modify

| File | Changes |
|------|---------|
| `services/neoaccounts/supabase/models.go` | Remove `Balance` field, add `AccountBalance`, `TokenBalance`, `AccountWithBalances` structs |
| `services/neoaccounts/supabase/repository.go` | Add balance-aware methods, update `ListAvailable` signature |
| `migrations/XXXX_multi_token_balances.sql` | Create `pool_account_balances` table, drop `balance` column |
| `services/neoaccounts/supabase/README.md` | Update documentation |

#### Acceptance Criteria

- [ ] `Account` struct no longer has `Balance` field
- [ ] `AccountBalance` struct created with proper JSON tags
- [ ] `pool_account_balances` table created with proper indexes
- [ ] Repository methods support token-aware queries
- [ ] Unit tests for new repository methods pass

---

### Task 2: NeoAccounts Service & API Refactor

**Owner**: Backend Developer
**Priority**: P0 (Depends on Task 1)
**Estimated Effort**: High

#### Files to Modify

| File | Changes |
|------|---------|
| `services/neoaccounts/marble/types.go` | Update `AccountInfo`, `UpdateBalanceInput`, `PoolInfoResponse`, add `TokenBalance`, `TokenStats` |
| `services/neoaccounts/marble/handlers.go` | Update handlers to use token parameter, modify response serialization |
| `services/neoaccounts/marble/pool.go` | Refactor `UpdateBalance`, `runAccountRotation`, pool statistics |
| `services/neoaccounts/marble/service.go` | Update account seeding to initialize empty balances |
| `services/neoaccounts/marble/api.go` | Add query parameter parsing for `token`, `min_balance` |
| `services/neoaccounts/marble/service_test.go` | Update all test assertions for new response format |
| `services/neoaccounts/marble/README.md` | Update API documentation |

#### Acceptance Criteria

- [ ] `/accounts` supports `?token=GAS&min_balance=X` filtering
- [ ] `/balances` endpoint accepts token parameter
- [ ] `/info` returns per-token statistics
- [ ] `UpdateBalance` increments `TxCount` once per call (not per token)
- [ ] Account rotation checks all token balances before marking as empty
- [ ] Unit tests achieve ≥90% coverage

---

### Task 3: NeoVault Client & Business Logic Updates

**Owner**: Backend Developer
**Priority**: P1 (Can parallel with Task 2 after API contract agreed)
**Estimated Effort**: High

#### Files to Modify

| File | Changes |
|------|---------|
| `services/neovault/marble/pool.go` | Update HTTP client calls to include token parameter |
| `services/neovault/marble/mixing.go` | Pass `TokenType` to all balance operations |
| `services/neovault/marble/handlers.go` | Update pool capacity calculation per token |
| `services/neovault/marble/types.go` | Update `PoolAccount` to embed `Balances` map |
| `services/neovault/marble/service.go` | Update account requests to specify token |
| `services/neovault/marble/service_test.go` | Update test mocks and assertions |
| `services/neovault/marble/README.md` | Update documentation |

#### Acceptance Criteria

- [ ] `NeoAccountsClient` methods accept token parameter
- [ ] `PoolAccount` provides `BalanceFor(token string) int64` helper
- [ ] Mixing/delivery flows use correct token type from `MixRequest`
- [ ] Background traffic loop handles per-token operations
- [ ] Unit tests achieve ≥90% coverage

---

### Task 4: Cross-Service Validation & Documentation

**Owner**: QA Engineer / Tech Writer
**Priority**: P1 (After Tasks 2-3)
**Estimated Effort**: Medium

#### Files to Modify

| File | Changes |
|------|---------|
| `test/integration/accountpool_test.go` | Update for multi-token assertions |
| `test/e2e/mixer_accountpool_test.go` | Add NEO and GAS mixing scenarios |
| `test/smoke/smoke_test.go` | Update balance check assertions |
| `services/neoaccounts/README.md` | Full documentation rewrite |
| `config/services.yaml` | Add token configuration examples |

#### Acceptance Criteria

- [ ] Integration tests cover NEO and GAS scenarios
- [ ] E2E tests verify cross-service token handling
- [ ] Smoke tests pass with new API format
- [ ] Documentation accurately reflects new API contract
- [ ] All tests pass with ≥90% coverage

---

## 4. Dependency Graph

```
Task 1: Schema & Repository
    │
    ▼
Task 2: NeoAccounts Service ──────┐
    │                              │
    │                              │ (API contract sync)
    │                              │
    ▼                              ▼
Task 3: NeoVault Client ◄─────────┘
    │
    ▼
Task 4: Validation & Docs
```

**Parallelization Strategy**:
- Tasks 2 and 3 can proceed in parallel once Task 1 defines the data model
- Task 4 can start documentation while Tasks 2-3 complete implementation

---

## 5. Testing Strategy

### Unit Test Coverage Requirements

| Package | Target |
|---------|--------|
| `services/neoaccounts/supabase` | ≥90% |
| `services/neoaccounts/marble` | ≥90% |
| `services/neovault/marble` | ≥90% |

### Test Scenarios

#### NeoAccounts

1. Create account with empty balances
2. Update GAS balance (delta mode)
3. Update NEO balance (absolute mode)
4. List accounts filtered by token and min_balance
5. Pool info returns correct per-token statistics
6. Account rotation respects all token balances
7. Concurrent balance updates (race condition test)

#### NeoVault

1. Request accounts for GAS mixing
2. Request accounts for NEO mixing
3. Execute mixing transaction with correct token
4. Deliver tokens updates correct balance
5. Pool capacity calculation per token

---

## 6. Risk Assessment

| Risk | Mitigation |
|------|------------|
| API breaking change | No backward compatibility needed (clean slate) |
| NEO indivisibility | Validate integer amounts, reject fractional |
| Concurrent balance updates | Use database transactions, optimistic locking |
| Performance regression | Add indexes on `(token_type, amount)` |

---

## 7. Rollout Plan

1. **Phase 1**: Deploy schema changes (Task 1)
2. **Phase 2**: Deploy NeoAccounts service (Task 2)
3. **Phase 3**: Deploy NeoVault updates (Task 3)
4. **Phase 4**: Run validation suite (Task 4)

---

## 8. Success Metrics

- [ ] All 4 tasks completed
- [ ] Test coverage ≥90% across all packages
- [ ] No regression in existing functionality
- [ ] NEO and GAS balances tracked independently
- [ ] Pool statistics accurate per token
