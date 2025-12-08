// Package gasbank provides core balance management for the service layer.
//
// This is NOT a service but core infrastructure used by all services for fee management.
// Balance operations are managed via Supabase database.
//
// Fee Flow:
// 1. User deposits GAS to Service Layer deposit address
// 2. TEE verifies deposit and credits user's balance
// 3. When user uses a service, fee is reserved from balance
// 4. After service execution, reserved fee is consumed
// 5. If service fails, reserved fee is released back to user
package gasbank

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/google/uuid"
)

// Manager handles all balance operations for the service layer.
type Manager struct {
	db           database.RepositoryInterface
	mu           sync.RWMutex
	reservations map[string]*Reservation
}

// NewManager creates a new balance manager.
func NewManager(db database.RepositoryInterface) *Manager {
	return &Manager{
		db:           db,
		reservations: make(map[string]*Reservation),
	}
}

// =============================================================================
// Core Balance Operations
// =============================================================================

// GetBalance returns the user's balance information.
func (m *Manager) GetBalance(ctx context.Context, userID string) (balance, reserved, available int64, err error) {
	account, err := m.db.GetGasBankAccount(ctx, userID)
	if err != nil {
		return 0, 0, 0, err
	}
	return account.Balance, account.Reserved, account.Balance - account.Reserved, nil
}

// Deposit adds funds to a user's account.
func (m *Manager) Deposit(ctx context.Context, userID string, amount int64, txHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	account, err := m.db.GetGasBankAccount(ctx, userID)
	if err != nil {
		return err
	}

	newBalance := account.Balance + amount
	if err := m.db.UpdateGasBankBalance(ctx, userID, newBalance, account.Reserved); err != nil {
		return err
	}

	return m.db.CreateGasBankTransaction(ctx, &database.GasBankTransaction{
		ID:           uuid.New().String(),
		AccountID:    account.ID,
		TxType:       TxTypeDeposit,
		Amount:       amount,
		BalanceAfter: newBalance,
		ReferenceID:  txHash,
		Status:       "completed",
		CreatedAt:    time.Now(),
	})
}

// Withdraw removes funds from a user's account.
func (m *Manager) Withdraw(ctx context.Context, userID string, amount int64, address string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	account, err := m.db.GetGasBankAccount(ctx, userID)
	if err != nil {
		return err
	}

	available := account.Balance - account.Reserved
	if amount > available {
		return fmt.Errorf("insufficient balance: available %d, requested %d", available, amount)
	}

	newBalance := account.Balance - amount
	if err := m.db.UpdateGasBankBalance(ctx, userID, newBalance, account.Reserved); err != nil {
		return err
	}

	return m.db.CreateGasBankTransaction(ctx, &database.GasBankTransaction{
		ID:           uuid.New().String(),
		AccountID:    account.ID,
		TxType:       TxTypeWithdraw,
		Amount:       -amount,
		BalanceAfter: newBalance,
		ReferenceID:  address,
		Status:       "completed",
		CreatedAt:    time.Now(),
	})
}

// =============================================================================
// Fee Operations
// =============================================================================

// ChargeServiceFee directly charges a service fee (without reservation).
func (m *Manager) ChargeServiceFee(ctx context.Context, userID, serviceID, referenceID string, amount int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	account, err := m.db.GetGasBankAccount(ctx, userID)
	if err != nil {
		return err
	}

	available := account.Balance - account.Reserved
	if amount > available {
		return fmt.Errorf("insufficient balance: available %d, required %d", available, amount)
	}

	newBalance := account.Balance - amount
	if err := m.db.UpdateGasBankBalance(ctx, userID, newBalance, account.Reserved); err != nil {
		return err
	}

	return m.db.CreateGasBankTransaction(ctx, &database.GasBankTransaction{
		ID:           uuid.New().String(),
		AccountID:    account.ID,
		TxType:       TxTypeServiceFee,
		Amount:       -amount,
		BalanceAfter: newBalance,
		ReferenceID:  referenceID,
		Status:       "completed",
		CreatedAt:    time.Now(),
	})
}

// CheckBalance checks if user has sufficient balance for a service.
func (m *Manager) CheckBalance(ctx context.Context, userID, serviceID string) (bool, int64, error) {
	account, err := m.db.GetGasBankAccount(ctx, userID)
	if err != nil {
		return false, 0, err
	}

	fee, ok := ServiceFees[serviceID]
	if !ok {
		return false, 0, fmt.Errorf("unknown service: %s", serviceID)
	}

	available := account.Balance - account.Reserved
	return available >= fee, available, nil
}

// GetTransactions returns recent transactions for a user.
func (m *Manager) GetTransactions(ctx context.Context, userID string, limit int) ([]database.GasBankTransaction, error) {
	return m.db.GetGasBankTransactions(ctx, userID, limit)
}
