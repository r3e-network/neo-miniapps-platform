// Package supabase provides NeoAccounts-specific database operations.
package supabase

import "time"

// Well-known token configurations
const (
	TokenTypeNEO = "NEO"
	TokenTypeGAS = "GAS"

	// Neo N3 MainNet script hashes
	NEOScriptHash = "0xef4073a0f2b305a38ec4050e4d3d28bc40ea63f5"
	GASScriptHash = "0xd2a4cff31913016155e38e474a2c06d08be276cf"

	// Decimals
	NEODecimals = 0
	GASDecimals = 8
)

// Account represents an account pool account with locking support.
// Balance is now tracked per-token in the AccountBalance table.
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

// AccountBalance represents a per-token balance for an account.
// Stored in pool_account_balances table.
type AccountBalance struct {
	AccountID  string    `json:"account_id"`
	TokenType  string    `json:"token_type"`  // "NEO", "GAS", or custom NEP-17
	ScriptHash string    `json:"script_hash"` // NEP-17 contract hash
	Amount     int64     `json:"amount"`
	Decimals   int       `json:"decimals"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// TokenBalance is the API representation of a token balance.
type TokenBalance struct {
	TokenType  string    `json:"token_type"`
	ScriptHash string    `json:"script_hash"`
	Amount     int64     `json:"amount"`
	Decimals   int       `json:"decimals"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

// AccountWithBalances combines account metadata with all token balances.
type AccountWithBalances struct {
	Account
	Balances map[string]TokenBalance `json:"balances"` // key: token_type
}

// TokenStats represents aggregated statistics for a token type.
type TokenStats struct {
	TokenType        string `json:"token_type"`
	ScriptHash       string `json:"script_hash"`
	TotalBalance     int64  `json:"total_balance"`
	LockedBalance    int64  `json:"locked_balance"`
	AvailableBalance int64  `json:"available_balance"`
}

// NewAccountWithBalances creates an AccountWithBalances from an Account.
func NewAccountWithBalances(acc Account) *AccountWithBalances {
	return &AccountWithBalances{
		Account:  acc,
		Balances: make(map[string]TokenBalance),
	}
}

// AddBalance adds a token balance to the account.
func (a *AccountWithBalances) AddBalance(bal AccountBalance) {
	a.Balances[bal.TokenType] = TokenBalance{
		TokenType:  bal.TokenType,
		ScriptHash: bal.ScriptHash,
		Amount:     bal.Amount,
		Decimals:   bal.Decimals,
		UpdatedAt:  bal.UpdatedAt,
	}
}

// GetBalance returns the balance for a specific token type.
// Returns 0 if the token type is not found.
func (a *AccountWithBalances) GetBalance(tokenType string) int64 {
	if bal, ok := a.Balances[tokenType]; ok {
		return bal.Amount
	}
	return 0
}

// HasSufficientBalance checks if account has at least minAmount of the specified token.
func (a *AccountWithBalances) HasSufficientBalance(tokenType string, minAmount int64) bool {
	return a.GetBalance(tokenType) >= minAmount
}

// IsEmpty returns true if all token balances are zero.
func (a *AccountWithBalances) IsEmpty() bool {
	for _, bal := range a.Balances {
		if bal.Amount > 0 {
			return false
		}
	}
	return true
}

// GetDefaultTokenConfig returns the script hash and decimals for well-known tokens.
func GetDefaultTokenConfig(tokenType string) (scriptHash string, decimals int) {
	switch tokenType {
	case TokenTypeNEO:
		return NEOScriptHash, NEODecimals
	case TokenTypeGAS:
		return GASScriptHash, GASDecimals
	default:
		return "", 8 // Default decimals for unknown tokens
	}
}
