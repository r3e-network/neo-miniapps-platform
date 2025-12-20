package database

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// =============================================================================
// Gas Bank Operations (implements GasBankRepository)
// =============================================================================

func (m *MockRepository) GetGasBankAccount(ctx context.Context, userID string) (*GasBankAccount, error) {
	if err := m.checkError(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, account := range m.gasBankAccounts {
		if account.UserID == userID {
			return account, nil
		}
	}
	return nil, NewNotFoundError("gasbank_account", userID)
}

func (m *MockRepository) CreateGasBankAccount(ctx context.Context, account *GasBankAccount) error {
	if err := m.checkError(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if account.ID == "" {
		account.ID = uuid.New().String()
	}
	if account.CreatedAt.IsZero() {
		account.CreatedAt = time.Now()
	}
	account.UpdatedAt = time.Now()
	m.gasBankAccounts[account.ID] = account
	return nil
}

func (m *MockRepository) GetOrCreateGasBankAccount(ctx context.Context, userID string) (*GasBankAccount, error) {
	account, err := m.GetGasBankAccount(ctx, userID)
	if err == nil {
		return account, nil
	}
	newAccount := &GasBankAccount{
		UserID:   userID,
		Balance:  0,
		Reserved: 0,
	}
	if err := m.CreateGasBankAccount(ctx, newAccount); err != nil {
		return nil, err
	}
	return newAccount, nil
}

func (m *MockRepository) UpdateGasBankBalance(ctx context.Context, userID string, balance, reserved int64) error {
	if err := m.checkError(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, account := range m.gasBankAccounts {
		if account.UserID == userID {
			account.Balance = balance
			account.Reserved = reserved
			account.UpdatedAt = time.Now()
			return nil
		}
	}
	return NewNotFoundError("gasbank_account", userID)
}

func (m *MockRepository) CreateGasBankTransaction(ctx context.Context, tx *GasBankTransaction) error {
	if err := m.checkError(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if tx.ID == "" {
		tx.ID = uuid.New().String()
	}
	if tx.CreatedAt.IsZero() {
		tx.CreatedAt = time.Now()
	}
	m.gasBankTransactions[tx.ID] = tx
	return nil
}

func (m *MockRepository) GetGasBankTransactions(ctx context.Context, accountID string, limit int) ([]GasBankTransaction, error) {
	if err := m.checkError(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []GasBankTransaction
	for _, tx := range m.gasBankTransactions {
		if tx.AccountID == accountID {
			result = append(result, *tx)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

// =============================================================================
// Deposit Operations (part of GasBankRepository)
// =============================================================================

func (m *MockRepository) CreateDepositRequest(ctx context.Context, deposit *DepositRequest) error {
	if err := m.checkError(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if deposit.ID == "" {
		deposit.ID = uuid.New().String()
	}
	if deposit.CreatedAt.IsZero() {
		deposit.CreatedAt = time.Now()
	}
	m.depositRequests[deposit.ID] = deposit
	return nil
}

func (m *MockRepository) GetDepositRequests(ctx context.Context, userID string, limit int) ([]DepositRequest, error) {
	if err := m.checkError(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []DepositRequest
	for _, deposit := range m.depositRequests {
		if deposit.UserID == userID {
			result = append(result, *deposit)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (m *MockRepository) GetDepositByTxHash(ctx context.Context, txHash string) (*DepositRequest, error) {
	if err := m.checkError(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, deposit := range m.depositRequests {
		if deposit.TxHash == txHash {
			return deposit, nil
		}
	}
	return nil, NewNotFoundError("deposit", txHash)
}

func (m *MockRepository) UpdateDepositStatus(ctx context.Context, depositID, status string, confirmations int) error {
	if err := m.checkError(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if deposit, ok := m.depositRequests[depositID]; ok {
		deposit.Status = status
		deposit.Confirmations = confirmations
		if status == "confirmed" {
			deposit.ConfirmedAt = time.Now()
		}
		return nil
	}
	return NewNotFoundError("deposit", depositID)
}

func (m *MockRepository) GetPendingDeposits(ctx context.Context, limit int) ([]DepositRequest, error) {
	if err := m.checkError(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []DepositRequest
	for _, deposit := range m.depositRequests {
		if deposit.Status == "pending" || deposit.Status == "confirming" {
			result = append(result, *deposit)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}
