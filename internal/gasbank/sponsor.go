package gasbank

import (
	"context"
	"fmt"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/google/uuid"
)

// =============================================================================
// Sponsor Operations
// =============================================================================

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
