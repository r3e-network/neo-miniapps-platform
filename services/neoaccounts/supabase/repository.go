// Package supabase provides NeoAccounts-specific database operations.
package supabase

import (
	"context"
	"fmt"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
)

const (
	tableName         = "pool_accounts"
	balancesTableName = "pool_account_balances"
)

// RepositoryInterface defines NeoAccounts-specific data access methods.
// This interface allows for easy mocking in tests.
type RepositoryInterface interface {
	// Account CRUD operations
	Create(ctx context.Context, acc *Account) error
	Update(ctx context.Context, acc *Account) error
	GetByID(ctx context.Context, id string) (*Account, error)
	List(ctx context.Context) ([]Account, error)
	ListAvailable(ctx context.Context, limit int) ([]Account, error)
	ListByLocker(ctx context.Context, lockerID string) ([]Account, error)
	Delete(ctx context.Context, id string) error

	// Balance-aware account operations
	GetWithBalances(ctx context.Context, id string) (*AccountWithBalances, error)
	ListWithBalances(ctx context.Context) ([]AccountWithBalances, error)
	ListAvailableWithBalances(ctx context.Context, tokenType string, minBalance *int64, limit int) ([]AccountWithBalances, error)
	ListByLockerWithBalances(ctx context.Context, lockerID string) ([]AccountWithBalances, error)

	// Balance operations
	UpsertBalance(ctx context.Context, accountID, tokenType, scriptHash string, amount int64, decimals int) error
	GetBalance(ctx context.Context, accountID, tokenType string) (*AccountBalance, error)
	GetBalances(ctx context.Context, accountID string) ([]AccountBalance, error)
	DeleteBalances(ctx context.Context, accountID string) error

	// Statistics
	AggregateTokenStats(ctx context.Context, tokenType string) (*TokenStats, error)
}

// Ensure Repository implements RepositoryInterface
var _ RepositoryInterface = (*Repository)(nil)

// Repository provides NeoAccounts-specific data access methods.
type Repository struct {
	base *database.Repository
}

// NewRepository creates a new NeoAccounts repository.
func NewRepository(base *database.Repository) *Repository {
	return &Repository{base: base}
}

// =============================================================================
// Account CRUD Operations
// =============================================================================

// Create inserts a new pool account.
func (r *Repository) Create(ctx context.Context, acc *Account) error {
	return database.GenericCreate(r.base, ctx, tableName, acc, func(rows []Account) {
		if len(rows) > 0 {
			*acc = rows[0]
		}
	})
}

// Update updates a pool account by ID.
func (r *Repository) Update(ctx context.Context, acc *Account) error {
	return database.GenericUpdate(r.base, ctx, tableName, "id", acc.ID, acc)
}

// GetByID fetches a pool account by ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Account, error) {
	return database.GenericGetByField[Account](r.base, ctx, tableName, "id", id)
}

// List returns all pool accounts.
func (r *Repository) List(ctx context.Context) ([]Account, error) {
	return database.GenericList[Account](r.base, ctx, tableName)
}

// ListAvailable returns unlocked, non-retiring accounts up to limit.
func (r *Repository) ListAvailable(ctx context.Context, limit int) ([]Account, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	query := database.NewQuery().
		IsFalse("is_retiring").
		IsNull("locked_by").
		OrderAsc("last_used_at").
		Limit(limit).
		Build()

	return database.GenericListWithQuery[Account](r.base, ctx, tableName, query)
}

// ListByLocker returns accounts locked by a specific service.
func (r *Repository) ListByLocker(ctx context.Context, lockerID string) ([]Account, error) {
	if lockerID == "" {
		return nil, fmt.Errorf("locker_id cannot be empty")
	}
	return database.GenericListByField[Account](r.base, ctx, tableName, "locked_by", lockerID)
}

// Delete deletes a pool account by ID.
func (r *Repository) Delete(ctx context.Context, id string) error {
	// Delete associated balances first (foreign key constraint)
	if err := r.DeleteBalances(ctx, id); err != nil {
		return fmt.Errorf("delete balances: %w", err)
	}
	return database.GenericDelete(r.base, ctx, tableName, "id", id)
}

// =============================================================================
// Balance-Aware Account Operations
// =============================================================================

// GetWithBalances fetches an account with all its token balances.
func (r *Repository) GetWithBalances(ctx context.Context, id string) (*AccountWithBalances, error) {
	acc, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	balances, err := r.GetBalances(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get balances: %w", err)
	}

	result := NewAccountWithBalances(acc)
	for i := range balances {
		bal := &balances[i]
		result.AddBalance(bal)
	}

	return result, nil
}

// ListWithBalances returns all accounts with their token balances.
func (r *Repository) ListWithBalances(ctx context.Context) ([]AccountWithBalances, error) {
	accounts, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	return r.hydrateAccountsWithBalances(ctx, accounts)
}

// ListAvailableWithBalances returns unlocked, non-retiring accounts with balances.
// If tokenType is specified, filters by minimum balance of that token.
func (r *Repository) ListAvailableWithBalances(ctx context.Context, tokenType string, minBalance *int64, limit int) ([]AccountWithBalances, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	// Get available accounts (unlocked, non-retiring)
	accounts, err := r.ListAvailable(ctx, limit*2) // Get extra in case we filter some out
	if err != nil {
		return nil, err
	}

	// Hydrate with balances
	accountsWithBalances, err := r.hydrateAccountsWithBalances(ctx, accounts)
	if err != nil {
		return nil, err
	}

	// Filter by token balance if specified
	if tokenType != "" && minBalance != nil {
		filtered := make([]AccountWithBalances, 0, len(accountsWithBalances))
		for i := range accountsWithBalances {
			acc := &accountsWithBalances[i]
			if acc.HasSufficientBalance(tokenType, *minBalance) {
				filtered = append(filtered, *acc)
				if len(filtered) >= limit {
					break
				}
			}
		}
		return filtered, nil
	}

	// Apply limit
	if len(accountsWithBalances) > limit {
		accountsWithBalances = accountsWithBalances[:limit]
	}

	return accountsWithBalances, nil
}

// ListByLockerWithBalances returns accounts locked by a service with their balances.
func (r *Repository) ListByLockerWithBalances(ctx context.Context, lockerID string) ([]AccountWithBalances, error) {
	accounts, err := r.ListByLocker(ctx, lockerID)
	if err != nil {
		return nil, err
	}

	return r.hydrateAccountsWithBalances(ctx, accounts)
}

// hydrateAccountsWithBalances adds balance information to a list of accounts.
func (r *Repository) hydrateAccountsWithBalances(ctx context.Context, accounts []Account) ([]AccountWithBalances, error) {
	result := make([]AccountWithBalances, 0, len(accounts))

	for i := range accounts {
		acc := &accounts[i]
		accWithBal := NewAccountWithBalances(acc)

		balances, err := r.GetBalances(ctx, acc.ID)
		if err != nil {
			// Log error but continue - account exists even if balances query fails
			continue
		}

		for j := range balances {
			bal := &balances[j]
			accWithBal.AddBalance(bal)
		}

		result = append(result, *accWithBal)
	}

	return result, nil
}

// =============================================================================
// Balance Operations
// =============================================================================

// UpsertBalance creates or updates a token balance for an account.
func (r *Repository) UpsertBalance(ctx context.Context, accountID, tokenType, scriptHash string, amount int64, decimals int) error {
	if accountID == "" || tokenType == "" {
		return fmt.Errorf("account_id and token_type are required")
	}

	// Check if balance exists
	existing, err := r.GetBalance(ctx, accountID, tokenType)
	if err != nil && existing == nil {
		// Create new balance
		bal := &AccountBalance{
			AccountID:  accountID,
			TokenType:  tokenType,
			ScriptHash: scriptHash,
			Amount:     amount,
			Decimals:   decimals,
			UpdatedAt:  time.Now(),
		}
		return database.GenericCreate(r.base, ctx, balancesTableName, bal, func(rows []AccountBalance) {
			if len(rows) > 0 {
				*bal = rows[0]
			}
		})
	}

	// Update existing balance
	existing.Amount = amount
	existing.ScriptHash = scriptHash
	existing.Decimals = decimals
	existing.UpdatedAt = time.Now()

	// Use composite key for update
	query := database.NewQuery().
		Eq("account_id", accountID).
		Eq("token_type", tokenType).
		Build()

	return database.GenericUpdateWithQuery(r.base, ctx, balancesTableName, query, existing)
}

// GetBalance fetches a specific token balance for an account.
func (r *Repository) GetBalance(ctx context.Context, accountID, tokenType string) (*AccountBalance, error) {
	if accountID == "" || tokenType == "" {
		return nil, fmt.Errorf("account_id and token_type are required")
	}

	query := database.NewQuery().
		Eq("account_id", accountID).
		Eq("token_type", tokenType).
		Build()

	balances, err := database.GenericListWithQuery[AccountBalance](r.base, ctx, balancesTableName, query)
	if err != nil {
		return nil, err
	}

	if len(balances) == 0 {
		return nil, nil
	}

	return &balances[0], nil
}

// GetBalances fetches all token balances for an account.
func (r *Repository) GetBalances(ctx context.Context, accountID string) ([]AccountBalance, error) {
	if accountID == "" {
		return nil, fmt.Errorf("account_id is required")
	}

	return database.GenericListByField[AccountBalance](r.base, ctx, balancesTableName, "account_id", accountID)
}

// DeleteBalances deletes all token balances for an account.
func (r *Repository) DeleteBalances(ctx context.Context, accountID string) error {
	if accountID == "" {
		return fmt.Errorf("account_id is required")
	}

	balances, err := r.GetBalances(ctx, accountID)
	if err != nil {
		return err
	}

	for i := range balances {
		bal := &balances[i]
		query := database.NewQuery().
			Eq("account_id", bal.AccountID).
			Eq("token_type", bal.TokenType).
			Build()
		if err := database.GenericDeleteWithQuery(r.base, ctx, balancesTableName, query); err != nil {
			return fmt.Errorf("delete balance %s/%s: %w", bal.AccountID, bal.TokenType, err)
		}
	}

	return nil
}

// =============================================================================
// Statistics
// =============================================================================

// AggregateTokenStats calculates aggregate statistics for a token type.
func (r *Repository) AggregateTokenStats(ctx context.Context, tokenType string) (*TokenStats, error) {
	if tokenType == "" {
		return nil, fmt.Errorf("token_type is required")
	}

	// Get all accounts
	accounts, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	scriptHash, _ := GetDefaultTokenConfig(tokenType)
	stats := &TokenStats{
		TokenType:  tokenType,
		ScriptHash: scriptHash,
	}

	for i := range accounts {
		acc := &accounts[i]
		bal, err := r.GetBalance(ctx, acc.ID, tokenType)
		if err != nil {
			continue
		}
		if bal == nil {
			continue
		}

		// Update script hash from actual data if available
		if bal.ScriptHash != "" {
			stats.ScriptHash = bal.ScriptHash
		}

		stats.TotalBalance += bal.Amount

		if acc.LockedBy != "" {
			stats.LockedBalance += bal.Amount
		} else if !acc.IsRetiring {
			stats.AvailableBalance += bal.Amount
		}
	}

	return stats, nil
}
