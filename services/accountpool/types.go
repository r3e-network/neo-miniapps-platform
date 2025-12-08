// Package accountpool provides types for the account pool service.
package accountpool

import "time"

// AccountInfo represents public account information returned to clients.
// Private keys are never exposed.
type AccountInfo struct {
	ID         string    `json:"id"`
	Address    string    `json:"address"`
	Balance    int64     `json:"balance"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	TxCount    int64     `json:"tx_count"`
	IsRetiring bool      `json:"is_retiring"`
	LockedBy   string    `json:"locked_by,omitempty"`
	LockedAt   time.Time `json:"locked_at,omitempty"`
}

// RequestAccountsInput for requesting accounts from the pool.
type RequestAccountsInput struct {
	ServiceID string `json:"service_id"` // ID of requesting service (e.g., "mixer")
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

// UpdateBalanceInput for updating an account's balance.
type UpdateBalanceInput struct {
	ServiceID string `json:"service_id"`
	AccountID string `json:"account_id"`
	Delta     int64  `json:"delta"`              // Positive to add, negative to subtract
	Absolute  *int64 `json:"absolute,omitempty"` // Or set absolute value
}

// UpdateBalanceResponse confirms balance update.
type UpdateBalanceResponse struct {
	AccountID  string `json:"account_id"`
	OldBalance int64  `json:"old_balance"`
	NewBalance int64  `json:"new_balance"`
}

// PoolInfoResponse returns pool statistics.
type PoolInfoResponse struct {
	TotalAccounts    int   `json:"total_accounts"`
	ActiveAccounts   int   `json:"active_accounts"`
	LockedAccounts   int   `json:"locked_accounts"`
	RetiringAccounts int   `json:"retiring_accounts"`
	TotalBalance     int64 `json:"total_balance"`
}

// ListAccountsInput for listing accounts with filters.
type ListAccountsInput struct {
	ServiceID  string `json:"service_id"`            // Required: only list accounts locked by this service
	MinBalance *int64 `json:"min_balance,omitempty"` // Optional: only accounts with balance >= this
}

// ListAccountsResponse returns filtered accounts.
type ListAccountsResponse struct {
	Accounts []AccountInfo `json:"accounts"`
}
