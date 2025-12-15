// Package neoaccounts provides types for the neoaccounts service.
package neoaccountsmarble

import (
	"time"

	neoaccountssupabase "github.com/R3E-Network/service_layer/services/neoaccounts/supabase"
)

// Re-export token constants for convenience
const (
	TokenTypeNEO = neoaccountssupabase.TokenTypeNEO
	TokenTypeGAS = neoaccountssupabase.TokenTypeGAS
)

// TokenBalance is the API representation of a token balance.
type TokenBalance = neoaccountssupabase.TokenBalance

// TokenStats represents aggregated statistics for a token type.
type TokenStats = neoaccountssupabase.TokenStats

// AccountInfo represents public account information returned to clients.
// Private keys are never exposed. Balances are tracked per-token.
type AccountInfo struct {
	ID         string                  `json:"id"`
	Address    string                  `json:"address"`
	CreatedAt  time.Time               `json:"created_at"`
	LastUsedAt time.Time               `json:"last_used_at"`
	TxCount    int64                   `json:"tx_count"`
	IsRetiring bool                    `json:"is_retiring"`
	LockedBy   string                  `json:"locked_by,omitempty"`
	LockedAt   time.Time               `json:"locked_at,omitempty"`
	Balances   map[string]TokenBalance `json:"balances"` // key: token_type (e.g., "NEO", "GAS")
}

// RequestAccountsInput for requesting accounts from the pool.
type RequestAccountsInput struct {
	ServiceID string `json:"service_id"` // ID of requesting service (e.g., "neovault")
	Count     int    `json:"count"`      // Number of accounts needed
	Purpose   string `json:"purpose"`    // Description of purpose (for audit)
}

// RequestAccountsResponse returns the requested accounts.
type RequestAccountsResponse struct {
	Accounts []AccountInfo `json:"accounts"`
	LockID   string        `json:"lock_id"` // ID to reference this lock for release/signing
}

// ReleaseAccountsInput for releasing previously requested accounts.
type ReleaseAccountsInput struct {
	ServiceID  string   `json:"service_id"`
	LockID     string   `json:"lock_id,omitempty"`     // Release by lock ID
	AccountIDs []string `json:"account_ids,omitempty"` // Or release specific accounts
}

// ReleaseAccountsResponse confirms release.
type ReleaseAccountsResponse struct {
	ReleasedCount int `json:"released_count"`
}

// SignTransactionInput for signing a transaction with an account's private key.
type SignTransactionInput struct {
	ServiceID string `json:"service_id"`
	AccountID string `json:"account_id"`
	TxHash    []byte `json:"tx_hash"` // Transaction hash to sign
}

// SignTransactionResponse returns the signature.
type SignTransactionResponse struct {
	AccountID string `json:"account_id"`
	Signature []byte `json:"signature"`
	PublicKey []byte `json:"public_key"`
}

// BatchSignInput for signing multiple transactions.
type BatchSignInput struct {
	ServiceID string        `json:"service_id"`
	Requests  []SignRequest `json:"requests"`
}

// SignRequest represents a single signing request within a batch.
type SignRequest struct {
	AccountID string `json:"account_id"`
	TxHash    []byte `json:"tx_hash"`
}

// BatchSignResponse returns multiple signatures.
type BatchSignResponse struct {
	Signatures []SignTransactionResponse `json:"signatures"`
	Errors     []string                  `json:"errors,omitempty"`
}

// UpdateBalanceInput for updating an account's token balance.
type UpdateBalanceInput struct {
	ServiceID string `json:"service_id"`
	AccountID string `json:"account_id"`
	Token     string `json:"token"`              // Token type: "NEO", "GAS", or custom NEP-17
	Delta     int64  `json:"delta"`              // Positive to add, negative to subtract
	Absolute  *int64 `json:"absolute,omitempty"` // Or set absolute value
}

// UpdateBalanceResponse confirms balance update.
type UpdateBalanceResponse struct {
	AccountID  string `json:"account_id"`
	Token      string `json:"token"`
	OldBalance int64  `json:"old_balance"`
	NewBalance int64  `json:"new_balance"`
	TxCount    int64  `json:"tx_count"` // Updated transaction count
}

// PoolInfoResponse returns pool statistics with per-token breakdowns.
type PoolInfoResponse struct {
	TotalAccounts    int                   `json:"total_accounts"`
	ActiveAccounts   int                   `json:"active_accounts"`
	LockedAccounts   int                   `json:"locked_accounts"`
	RetiringAccounts int                   `json:"retiring_accounts"`
	TokenStats       map[string]TokenStats `json:"token_stats"` // key: token_type
}

// ListAccountsInput for listing accounts with filters.
type ListAccountsInput struct {
	ServiceID  string `json:"service_id"`            // Required: only list accounts locked by this service
	Token      string `json:"token,omitempty"`       // Optional: filter by token type
	MinBalance *int64 `json:"min_balance,omitempty"` // Optional: minimum balance for specified token
}

// ListAccountsResponse returns filtered accounts.
type ListAccountsResponse struct {
	Accounts []AccountInfo `json:"accounts"`
}

// AccountInfoFromWithBalances converts AccountWithBalances to AccountInfo.
func AccountInfoFromWithBalances(acc *neoaccountssupabase.AccountWithBalances) AccountInfo {
	return AccountInfo{
		ID:         acc.ID,
		Address:    acc.Address,
		CreatedAt:  acc.CreatedAt,
		LastUsedAt: acc.LastUsedAt,
		TxCount:    acc.TxCount,
		IsRetiring: acc.IsRetiring,
		LockedBy:   acc.LockedBy,
		LockedAt:   acc.LockedAt,
		Balances:   acc.Balances,
	}
}

// AccountInfoFromAccount converts Account to AccountInfo with empty balances.
func AccountInfoFromAccount(acc *neoaccountssupabase.Account) AccountInfo {
	return AccountInfo{
		ID:         acc.ID,
		Address:    acc.Address,
		CreatedAt:  acc.CreatedAt,
		LastUsedAt: acc.LastUsedAt,
		TxCount:    acc.TxCount,
		IsRetiring: acc.IsRetiring,
		LockedBy:   acc.LockedBy,
		LockedAt:   acc.LockedAt,
		Balances:   make(map[string]TokenBalance),
	}
}
