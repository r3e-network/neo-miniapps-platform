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
	data, err := r.client.request(ctx, "GET", "gasbank_accounts", nil, "user_id=eq."+userID+"&limit=1")
	if err != nil {
		return nil, err
	}

	var accounts []GasBankAccount
	if err := json.Unmarshal(data, &accounts); err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, fmt.Errorf("account not found")
	}
	return &accounts[0], nil
}

// CreateGasBankAccount creates a new gas bank account.
func (r *Repository) CreateGasBankAccount(ctx context.Context, account *GasBankAccount) error {
	data, err := r.client.request(ctx, "POST", "gasbank_accounts", account, "")
	if err != nil {
		return err
	}
	var accounts []GasBankAccount
	if err := json.Unmarshal(data, &accounts); err != nil {
		return err
	}
	if len(accounts) > 0 {
		account.ID = accounts[0].ID
	}
	return nil
}

// GetOrCreateGasBankAccount gets or creates a gas bank account for a user.
func (r *Repository) GetOrCreateGasBankAccount(ctx context.Context, userID string) (*GasBankAccount, error) {
	account, err := r.GetGasBankAccount(ctx, userID)
	if err == nil {
		return account, nil
	}

	newAccount := &GasBankAccount{
		UserID:   userID,
		Balance:  0,
		Reserved: 0,
	}
	if err := r.CreateGasBankAccount(ctx, newAccount); err != nil {
		return nil, err
	}
	return newAccount, nil
}

// UpdateGasBankBalance updates a gas bank account balance.
func (r *Repository) UpdateGasBankBalance(ctx context.Context, userID string, balance, reserved int64) error {
	update := map[string]interface{}{
		"balance":    balance,
		"reserved":   reserved,
		"updated_at": time.Now(),
	}
	_, err := r.client.request(ctx, "PATCH", "gasbank_accounts", update, "user_id=eq."+userID)
	return err
}

// =============================================================================
// Gas Bank Transaction Operations
// =============================================================================

// CreateGasBankTransaction creates a new gas bank transaction record.
func (r *Repository) CreateGasBankTransaction(ctx context.Context, tx *GasBankTransaction) error {
	_, err := r.client.request(ctx, "POST", "gasbank_transactions", tx, "")
	return err
}

// GetGasBankTransactions retrieves transaction history for an account.
func (r *Repository) GetGasBankTransactions(ctx context.Context, accountID string, limit int) ([]GasBankTransaction, error) {
	query := fmt.Sprintf("account_id=eq.%s&order=created_at.desc&limit=%d", accountID, limit)
	data, err := r.client.request(ctx, "GET", "gasbank_transactions", nil, query)
	if err != nil {
		return nil, err
	}

	var txs []GasBankTransaction
	if err := json.Unmarshal(data, &txs); err != nil {
		return nil, err
	}
	return txs, nil
}

// =============================================================================
// Deposit Operations
// =============================================================================

// CreateDepositRequest creates a new deposit request.
func (r *Repository) CreateDepositRequest(ctx context.Context, deposit *DepositRequest) error {
	data, err := r.client.request(ctx, "POST", "deposit_requests", deposit, "")
	if err != nil {
		return err
	}
	var deposits []DepositRequest
	if err := json.Unmarshal(data, &deposits); err != nil {
		return err
	}
	if len(deposits) > 0 {
		deposit.ID = deposits[0].ID
	}
	return nil
}

// GetDepositRequests retrieves deposit requests for a user.
func (r *Repository) GetDepositRequests(ctx context.Context, userID string, limit int) ([]DepositRequest, error) {
	query := fmt.Sprintf("user_id=eq.%s&order=created_at.desc&limit=%d", userID, limit)
	data, err := r.client.request(ctx, "GET", "deposit_requests", nil, query)
	if err != nil {
		return nil, err
	}

	var deposits []DepositRequest
	if err := json.Unmarshal(data, &deposits); err != nil {
		return nil, err
	}
	return deposits, nil
}

// GetDepositByTxHash retrieves a deposit by transaction hash.
func (r *Repository) GetDepositByTxHash(ctx context.Context, txHash string) (*DepositRequest, error) {
	query := fmt.Sprintf("tx_hash=eq.%s&limit=1", txHash)
	data, err := r.client.request(ctx, "GET", "deposit_requests", nil, query)
	if err != nil {
		return nil, err
	}

	var deposits []DepositRequest
	if err := json.Unmarshal(data, &deposits); err != nil {
		return nil, err
	}
	if len(deposits) == 0 {
		return nil, fmt.Errorf("deposit not found")
	}
	return &deposits[0], nil
}

// UpdateDepositStatus updates a deposit request status.
func (r *Repository) UpdateDepositStatus(ctx context.Context, depositID, status string, confirmations int) error {
	update := map[string]interface{}{
		"status":        status,
		"confirmations": confirmations,
	}
	if status == "confirmed" {
		update["confirmed_at"] = time.Now()
	}
	_, err := r.client.request(ctx, "PATCH", "deposit_requests", update, "id=eq."+depositID)
	return err
}
