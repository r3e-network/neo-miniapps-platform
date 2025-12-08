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

const (
	// Transaction types
	TxTypeDeposit       = "deposit"
	TxTypeWithdraw      = "withdraw"
	TxTypeServiceFee    = "service_fee"
	TxTypeRefund        = "refund"
	TxTypeSponsor       = "sponsor"        // Sponsor payment for contract/user
	TxTypeSponsorCredit = "sponsor_credit" // Credit received from sponsor

	// Reservation status
	ReservationPending  = "pending"
	ReservationConsumed = "consumed"
	ReservationReleased = "released"
)

// ServiceFees defines the fee for each service (in GAS smallest unit, 1e-8 GAS).
var ServiceFees = map[string]int64{
	"vrf":          100000,  // 0.001 GAS per VRF request
	"automation":   50000,   // 0.0005 GAS per trigger execution
	"datafeeds":    10000,   // 0.0001 GAS per price query
	"mixer":        5000000, // 0.05 GAS base fee (+ 0.5% of amount)
	"confidential": 100000,  // 0.001 GAS per compute job
}

// Reservation represents a fee reservation for a pending operation.
type Reservation struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	ServiceID   string    `json:"service_id"`
	ReferenceID string    `json:"reference_id"`
	Amount      int64     `json:"amount"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	ConsumedAt  time.Time `json:"consumed_at,omitempty"`
}

// Manager handles all balance operations for the service layer.
type Manager struct {
	db           *database.Repository
	mu           sync.RWMutex
	reservations map[string]*Reservation
}

// NewManager creates a new balance manager.
func NewManager(db *database.Repository) *Manager {
	return &Manager{
		db:           db,
		reservations: make(map[string]*Reservation),
	}
}

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

// Reserve reserves funds for a pending service operation.
func (m *Manager) Reserve(ctx context.Context, userID, serviceID, referenceID string, amount int64) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	account, err := m.db.GetGasBankAccount(ctx, userID)
	if err != nil {
		return "", err
	}

	available := account.Balance - account.Reserved
	if amount > available {
		return "", fmt.Errorf("insufficient balance: available %d, required %d", available, amount)
	}

	reservation := &Reservation{
		ID:          uuid.New().String(),
		UserID:      userID,
		ServiceID:   serviceID,
		ReferenceID: referenceID,
		Amount:      amount,
		Status:      ReservationPending,
		CreatedAt:   time.Now(),
	}

	newReserved := account.Reserved + amount
	if err := m.db.UpdateGasBankBalance(ctx, userID, account.Balance, newReserved); err != nil {
		return "", err
	}

	m.reservations[reservation.ID] = reservation
	return reservation.ID, nil
}

// Release releases a reservation back to the user.
func (m *Manager) Release(ctx context.Context, userID, reservationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	reservation, ok := m.reservations[reservationID]
	if !ok {
		return nil // Idempotent: treat as already released
	}

	delete(m.reservations, reservationID)

	if reservation.UserID != userID {
		return fmt.Errorf("unauthorized")
	}

	if reservation.Status != ReservationPending {
		return fmt.Errorf("reservation already %s", reservation.Status)
	}

	account, err := m.db.GetGasBankAccount(ctx, userID)
	if err != nil {
		return err
	}

	newReserved := account.Reserved - reservation.Amount
	if newReserved < 0 {
		newReserved = 0
	}

	if err := m.db.UpdateGasBankBalance(ctx, userID, account.Balance, newReserved); err != nil {
		return err
	}

	reservation.Status = ReservationReleased
	return nil
}

// Consume consumes a reservation (service completed successfully).
func (m *Manager) Consume(ctx context.Context, userID, reservationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	reservation, ok := m.reservations[reservationID]
	if !ok {
		return fmt.Errorf("reservation not found")
	}

	if reservation.UserID != userID {
		return fmt.Errorf("unauthorized")
	}

	if reservation.Status != ReservationPending {
		return fmt.Errorf("reservation already %s", reservation.Status)
	}

	account, err := m.db.GetGasBankAccount(ctx, userID)
	if err != nil {
		return err
	}

	newBalance := account.Balance - reservation.Amount
	newReserved := account.Reserved - reservation.Amount
	if newReserved < 0 {
		newReserved = 0
	}

	if err := m.db.UpdateGasBankBalance(ctx, userID, newBalance, newReserved); err != nil {
		return err
	}

	if err := m.db.CreateGasBankTransaction(ctx, &database.GasBankTransaction{
		ID:           uuid.New().String(),
		AccountID:    account.ID,
		TxType:       TxTypeServiceFee,
		Amount:       -reservation.Amount,
		BalanceAfter: newBalance,
		ReferenceID:  reservation.ReferenceID,
		Status:       "completed",
		CreatedAt:    time.Now(),
	}); err != nil {
		return err
	}

	reservation.Status = ReservationConsumed
	reservation.ConsumedAt = time.Now()
	delete(m.reservations, reservationID)

	return nil
}

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

// GetServiceFee returns the fee for a service.
func GetServiceFee(serviceID string) int64 {
	if fee, ok := ServiceFees[serviceID]; ok {
		return fee
	}
	return 0
}

// GetTransactions returns recent transactions for a user.
func (m *Manager) GetTransactions(ctx context.Context, userID string, limit int) ([]database.GasBankTransaction, error) {
	return m.db.GetGasBankTransactions(ctx, userID, limit)
}

// PayForContract transfers funds from sponsor to a contract's balance.
// This allows a user (sponsor) to pay service fees on behalf of a smart contract.
func (m *Manager) PayForContract(ctx context.Context, sponsorUserID, contractAddress string, amount int64, note string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check sponsor balance
	sponsorAccount, err := m.db.GetGasBankAccount(ctx, sponsorUserID)
	if err != nil {
		return fmt.Errorf("get sponsor account: %w", err)
	}

	available := sponsorAccount.Balance - sponsorAccount.Reserved
	if amount > available {
		return fmt.Errorf("insufficient balance: available %d, required %d", available, amount)
	}

	// Get or create contract account (contracts use address as userID)
	contractAccount, err := m.db.GetOrCreateGasBankAccount(ctx, contractAddress)
	if err != nil {
		return fmt.Errorf("get contract account: %w", err)
	}

	// Deduct from sponsor
	sponsorNewBalance := sponsorAccount.Balance - amount
	if err := m.db.UpdateGasBankBalance(ctx, sponsorUserID, sponsorNewBalance, sponsorAccount.Reserved); err != nil {
		return fmt.Errorf("update sponsor balance: %w", err)
	}

	// Credit contract
	contractNewBalance := contractAccount.Balance + amount
	if err := m.db.UpdateGasBankBalance(ctx, contractAddress, contractNewBalance, contractAccount.Reserved); err != nil {
		// Rollback sponsor balance on failure
		_ = m.db.UpdateGasBankBalance(ctx, sponsorUserID, sponsorAccount.Balance, sponsorAccount.Reserved)
		return fmt.Errorf("update contract balance: %w", err)
	}

	// Record sponsor transaction (debit)
	refID := fmt.Sprintf("sponsor:contract:%s:%s", contractAddress, note)
	if err := m.db.CreateGasBankTransaction(ctx, &database.GasBankTransaction{
		ID:           uuid.New().String(),
		AccountID:    sponsorAccount.ID,
		TxType:       TxTypeSponsor,
		Amount:       -amount,
		BalanceAfter: sponsorNewBalance,
		ReferenceID:  refID,
		ToAddress:    contractAddress,
		Status:       "completed",
		CreatedAt:    time.Now(),
	}); err != nil {
		return fmt.Errorf("record sponsor transaction: %w", err)
	}

	// Record credit transaction for contract
	return m.db.CreateGasBankTransaction(ctx, &database.GasBankTransaction{
		ID:           uuid.New().String(),
		AccountID:    contractAccount.ID,
		TxType:       TxTypeSponsorCredit,
		Amount:       amount,
		BalanceAfter: contractNewBalance,
		ReferenceID:  refID,
		FromAddress:  sponsorUserID,
		Status:       "completed",
		CreatedAt:    time.Now(),
	})
}

// PayForUser transfers funds from sponsor to another user's balance.
// This allows a user (sponsor) to pay service fees on behalf of another user.
func (m *Manager) PayForUser(ctx context.Context, sponsorUserID, recipientUserID string, amount int64, note string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sponsorUserID == recipientUserID {
		return fmt.Errorf("cannot sponsor yourself")
	}

	// Check sponsor balance
	sponsorAccount, err := m.db.GetGasBankAccount(ctx, sponsorUserID)
	if err != nil {
		return fmt.Errorf("get sponsor account: %w", err)
	}

	available := sponsorAccount.Balance - sponsorAccount.Reserved
	if amount > available {
		return fmt.Errorf("insufficient balance: available %d, required %d", available, amount)
	}

	// Get or create recipient account
	recipientAccount, err := m.db.GetOrCreateGasBankAccount(ctx, recipientUserID)
	if err != nil {
		return fmt.Errorf("get recipient account: %w", err)
	}

	// Deduct from sponsor
	sponsorNewBalance := sponsorAccount.Balance - amount
	if err := m.db.UpdateGasBankBalance(ctx, sponsorUserID, sponsorNewBalance, sponsorAccount.Reserved); err != nil {
		return fmt.Errorf("update sponsor balance: %w", err)
	}

	// Credit recipient
	recipientNewBalance := recipientAccount.Balance + amount
	if err := m.db.UpdateGasBankBalance(ctx, recipientUserID, recipientNewBalance, recipientAccount.Reserved); err != nil {
		// Rollback sponsor balance on failure
		_ = m.db.UpdateGasBankBalance(ctx, sponsorUserID, sponsorAccount.Balance, sponsorAccount.Reserved)
		return fmt.Errorf("update recipient balance: %w", err)
	}

	// Record sponsor transaction (debit)
	refID := fmt.Sprintf("sponsor:user:%s:%s", recipientUserID, note)
	if err := m.db.CreateGasBankTransaction(ctx, &database.GasBankTransaction{
		ID:           uuid.New().String(),
		AccountID:    sponsorAccount.ID,
		TxType:       TxTypeSponsor,
		Amount:       -amount,
		BalanceAfter: sponsorNewBalance,
		ReferenceID:  refID,
		ToAddress:    recipientUserID,
		Status:       "completed",
		CreatedAt:    time.Now(),
	}); err != nil {
		return fmt.Errorf("record sponsor transaction: %w", err)
	}

	// Record credit transaction for recipient
	return m.db.CreateGasBankTransaction(ctx, &database.GasBankTransaction{
		ID:           uuid.New().String(),
		AccountID:    recipientAccount.ID,
		TxType:       TxTypeSponsorCredit,
		Amount:       amount,
		BalanceAfter: recipientNewBalance,
		ReferenceID:  refID,
		FromAddress:  sponsorUserID,
		Status:       "completed",
		CreatedAt:    time.Now(),
	})
}
