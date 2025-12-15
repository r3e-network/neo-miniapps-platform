// Package neoaccountsclient provides HTTP client and API types for the neoaccounts service.
// This package should be imported by other services that need to call neoaccounts APIs.
package neoaccountsclient

import "time"

// TokenBalance represents a balance for a specific token type.
type TokenBalance struct {
	TokenType  string    `json:"token_type"`
	ScriptHash string    `json:"script_hash"`
	Amount     int64     `json:"amount"`
	Decimals   int       `json:"decimals"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

// TokenStats represents aggregated statistics for a token type.
type TokenStats struct {
	TokenType    string `json:"token_type"`
	TotalBalance int64  `json:"total_balance"`
	AvgBalance   int64  `json:"avg_balance"`
	MinBalance   int64  `json:"min_balance"`
	MaxBalance   int64  `json:"max_balance"`
}

// AccountInfo represents public account information returned to clients.
type AccountInfo struct {
	ID         string                  `json:"id"`
	Address    string                  `json:"address"`
	CreatedAt  time.Time               `json:"created_at"`
	LastUsedAt time.Time               `json:"last_used_at"`
	TxCount    int64                   `json:"tx_count"`
	IsRetiring bool                    `json:"is_retiring"`
	LockedBy   string                  `json:"locked_by,omitempty"`
	LockedAt   time.Time               `json:"locked_at,omitempty"`
	Balances   map[string]TokenBalance `json:"balances"`
}

// PoolInfoResponse returns pool statistics with per-token breakdowns.
type PoolInfoResponse struct {
	TotalAccounts    int                   `json:"total_accounts"`
	ActiveAccounts   int                   `json:"active_accounts"`
	LockedAccounts   int                   `json:"locked_accounts"`
	RetiringAccounts int                   `json:"retiring_accounts"`
	TokenStats       map[string]TokenStats `json:"token_stats"`
}

// RequestAccountsInput for requesting accounts from the pool.
type RequestAccountsInput struct {
	ServiceID string `json:"service_id"`
	Count     int    `json:"count"`
	Purpose   string `json:"purpose"`
}

// RequestAccountsResponse returns the requested accounts.
type RequestAccountsResponse struct {
	Accounts []AccountInfo `json:"accounts"`
	LockID   string        `json:"lock_id"`
}

// ReleaseAccountsInput for releasing previously requested accounts.
type ReleaseAccountsInput struct {
	ServiceID  string   `json:"service_id"`
	LockID     string   `json:"lock_id,omitempty"`
	AccountIDs []string `json:"account_ids,omitempty"`
}

// ReleaseAccountsResponse confirms release.
type ReleaseAccountsResponse struct {
	ReleasedCount int `json:"released_count"`
}

// SignTransactionInput for signing a transaction with an account's private key.
type SignTransactionInput struct {
	ServiceID string `json:"service_id"`
	AccountID string `json:"account_id"`
	TxHash    []byte `json:"tx_hash"`
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
	Token     string `json:"token"`
	Delta     int64  `json:"delta"`
	Absolute  *int64 `json:"absolute,omitempty"`
}

// UpdateBalanceResponse confirms balance update.
type UpdateBalanceResponse struct {
	AccountID  string `json:"account_id"`
	Token      string `json:"token"`
	OldBalance int64  `json:"old_balance"`
	NewBalance int64  `json:"new_balance"`
	TxCount    int64  `json:"tx_count"`
}

// ListAccountsResponse returns filtered accounts.
type ListAccountsResponse struct {
	Accounts []AccountInfo `json:"accounts"`
}
