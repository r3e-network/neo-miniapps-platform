package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// =============================================================================
// Gas Bank Account Operations
// =============================================================================

// GetGasBankAccount retrieves a gas bank account.
func (r *Repository) GetGasBankAccount(ctx context.Context, userID string) (*GasBankAccount, error) {
	if err := ValidateUserID(userID); err != nil {
		return nil, err
	}

	data, err := r.client.request(ctx, "GET", "gasbank_accounts", nil, "user_id=eq."+userID+"&limit=1")
	if err != nil {
		return nil, fmt.Errorf("%w: get gasbank account: %v", ErrDatabaseError, err)
	}

	var accounts []GasBankAccount
	if err := json.Unmarshal(data, &accounts); err != nil {
		return nil, fmt.Errorf("%w: unmarshal gasbank accounts: %v", ErrDatabaseError, err)
	}
	if len(accounts) == 0 {
		return nil, NewNotFoundError("gasbank_account", userID)
	}
	return &accounts[0], nil
}

// CreateGasBankAccount creates a new gas bank account.
func (r *Repository) CreateGasBankAccount(ctx context.Context, account *GasBankAccount) error {
	if account == nil {
		return fmt.Errorf("%w: account cannot be nil", ErrInvalidInput)
	}
	if err := ValidateUserID(account.UserID); err != nil {
		return err
	}

	data, err := r.client.request(ctx, "POST", "gasbank_accounts", account, "")
	if err != nil {
		return fmt.Errorf("%w: create gasbank account: %v", ErrDatabaseError, err)
	}
	var accounts []GasBankAccount
	if err := json.Unmarshal(data, &accounts); err != nil {
		return fmt.Errorf("%w: unmarshal gasbank accounts: %v", ErrDatabaseError, err)
	}
	if len(accounts) > 0 {
		account.ID = accounts[0].ID
	}
	return nil
}

// GetOrCreateGasBankAccount gets or creates a gas bank account for a user.
// Uses upsert pattern to handle race conditions safely.
func (r *Repository) GetOrCreateGasBankAccount(ctx context.Context, userID string) (*GasBankAccount, error) {
	if err := ValidateUserID(userID); err != nil {
		return nil, err
	}

	// First try to get existing account
	account, err := r.GetGasBankAccount(ctx, userID)
	if err == nil {
		return account, nil
	}

	// Only proceed if it's a not found error
	if !IsNotFound(err) {
		return nil, err
	}

	// Create new account with upsert semantics
	// Use Supabase's on_conflict to handle race conditions
	newAccount := &GasBankAccount{
		UserID:    userID,
		Balance:   0,
		Reserved:  0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Use upsert with on_conflict=user_id to handle race conditions
	data, err := r.client.request(ctx, "POST", "gasbank_accounts", newAccount, "on_conflict=user_id")
	if err != nil {
		// If creation failed due to conflict, try to get the existing account
		account, getErr := r.GetGasBankAccount(ctx, userID)
		if getErr == nil {
			return account, nil
		}
		return nil, fmt.Errorf("%w: create gasbank account: %v", ErrDatabaseError, err)
	}

	var accounts []GasBankAccount
	if err := json.Unmarshal(data, &accounts); err != nil {
		return nil, fmt.Errorf("%w: unmarshal gasbank accounts: %v", ErrDatabaseError, err)
	}
	if len(accounts) > 0 {
		return &accounts[0], nil
	}

	// Fallback: try to get the account again
	return r.GetGasBankAccount(ctx, userID)
}

// UpdateGasBankBalance updates a gas bank account balance.
func (r *Repository) UpdateGasBankBalance(ctx context.Context, userID string, balance, reserved int64) error {
	if err := ValidateUserID(userID); err != nil {
		return err
	}
	if balance < 0 {
		return fmt.Errorf("%w: balance cannot be negative", ErrInvalidInput)
	}
	if reserved < 0 {
		return fmt.Errorf("%w: reserved cannot be negative", ErrInvalidInput)
	}

	update := map[string]interface{}{
		"balance":    balance,
		"reserved":   reserved,
		"updated_at": time.Now(),
	}
	_, err := r.client.request(ctx, "PATCH", "gasbank_accounts", update, "user_id=eq."+userID)
	if err != nil {
		return fmt.Errorf("%w: update gasbank balance: %v", ErrDatabaseError, err)
	}
	return nil
}

// =============================================================================
// Gas Bank Transaction Operations
// =============================================================================

// CreateGasBankTransaction creates a new gas bank transaction record.
func (r *Repository) CreateGasBankTransaction(ctx context.Context, tx *GasBankTransaction) error {
	if tx == nil {
		return fmt.Errorf("%w: transaction cannot be nil", ErrInvalidInput)
	}
	if tx.ID == "" {
		return fmt.Errorf("%w: transaction id cannot be empty", ErrInvalidInput)
	}

	_, err := r.client.request(ctx, "POST", "gasbank_transactions", tx, "")
	if err != nil {
		return fmt.Errorf("%w: create gasbank transaction: %v", ErrDatabaseError, err)
	}
	return nil
}

// GetGasBankTransactions retrieves transaction history for an account.
func (r *Repository) GetGasBankTransactions(ctx context.Context, accountID string, limit int) ([]GasBankTransaction, error) {
	if err := ValidateID(accountID); err != nil {
		return nil, err
	}
	limit = ValidateLimit(limit, 50, 1000)

	query := fmt.Sprintf("account_id=eq.%s&order=created_at.desc&limit=%d", accountID, limit)
	data, err := r.client.request(ctx, "GET", "gasbank_transactions", nil, query)
	if err != nil {
		return nil, fmt.Errorf("%w: get gasbank transactions: %v", ErrDatabaseError, err)
	}

	var txs []GasBankTransaction
	if err := json.Unmarshal(data, &txs); err != nil {
		return nil, fmt.Errorf("%w: unmarshal gasbank transactions: %v", ErrDatabaseError, err)
	}
	return txs, nil
}

// =============================================================================
// Deposit Operations
// =============================================================================

// CreateDepositRequest creates a new deposit request.
func (r *Repository) CreateDepositRequest(ctx context.Context, deposit *DepositRequest) error {
	if deposit == nil {
		return fmt.Errorf("%w: deposit cannot be nil", ErrInvalidInput)
	}
	if err := ValidateUserID(deposit.UserID); err != nil {
		return err
	}

	data, err := r.client.request(ctx, "POST", "deposit_requests", deposit, "")
	if err != nil {
		return fmt.Errorf("%w: create deposit request: %v", ErrDatabaseError, err)
	}
	var deposits []DepositRequest
	if err := json.Unmarshal(data, &deposits); err != nil {
		return fmt.Errorf("%w: unmarshal deposit requests: %v", ErrDatabaseError, err)
	}
	if len(deposits) > 0 {
		deposit.ID = deposits[0].ID
	}
	return nil
}

// GetDepositRequests retrieves deposit requests for a user.
func (r *Repository) GetDepositRequests(ctx context.Context, userID string, limit int) ([]DepositRequest, error) {
	if err := ValidateUserID(userID); err != nil {
		return nil, err
	}
	limit = ValidateLimit(limit, 50, 1000)

	query := fmt.Sprintf("user_id=eq.%s&order=created_at.desc&limit=%d", userID, limit)
	data, err := r.client.request(ctx, "GET", "deposit_requests", nil, query)
	if err != nil {
		return nil, fmt.Errorf("%w: get deposit requests: %v", ErrDatabaseError, err)
	}

	var deposits []DepositRequest
	if err := json.Unmarshal(data, &deposits); err != nil {
		return nil, fmt.Errorf("%w: unmarshal deposit requests: %v", ErrDatabaseError, err)
	}
	return deposits, nil
}

// GetDepositByTxHash retrieves a deposit by transaction hash.
func (r *Repository) GetDepositByTxHash(ctx context.Context, txHash string) (*DepositRequest, error) {
	if err := ValidateTxHash(txHash); err != nil {
		return nil, err
	}

	query := fmt.Sprintf("tx_hash=eq.%s&limit=1", txHash)
	data, err := r.client.request(ctx, "GET", "deposit_requests", nil, query)
	if err != nil {
		return nil, fmt.Errorf("%w: get deposit by tx_hash: %v", ErrDatabaseError, err)
	}

	var deposits []DepositRequest
	if err := json.Unmarshal(data, &deposits); err != nil {
		return nil, fmt.Errorf("%w: unmarshal deposit requests: %v", ErrDatabaseError, err)
	}
	if len(deposits) == 0 {
		return nil, NewNotFoundError("deposit", txHash)
	}
	return &deposits[0], nil
}

// UpdateDepositStatus updates a deposit request status.
func (r *Repository) UpdateDepositStatus(ctx context.Context, depositID, status string, confirmations int) error {
	if err := ValidateID(depositID); err != nil {
		return err
	}
	validStatuses := []string{"pending", "confirming", "confirmed", "failed"}
	if err := ValidateStatus(status, validStatuses); err != nil {
		return err
	}

	update := map[string]interface{}{
		"status":        status,
		"confirmations": confirmations,
	}
	if status == "confirmed" {
		update["confirmed_at"] = time.Now()
	}
	_, err := r.client.request(ctx, "PATCH", "deposit_requests", update, "id=eq."+depositID)
	if err != nil {
		return fmt.Errorf("%w: update deposit status: %v", ErrDatabaseError, err)
	}
	return nil
}
