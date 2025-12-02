package gasbank

import (
	"context"
	"fmt"

	"github.com/R3E-Network/service_layer/system/os/framework"
)

// Compile-time check: FeeCollector implements framework.FeeCollector.
var _ framework.FeeCollector = (*FeeCollector)(nil)

// FeeCollector implements framework.FeeCollector using the gasbank service.
// This adapter allows oracle requests to be charged against gas accounts.
// Aligned with OracleHub.cs contract fee model.
type FeeCollector struct {
	svc *Service
}

// NewFeeCollector creates a new fee collector backed by the gasbank service.
func NewFeeCollector(svc *Service) *FeeCollector {
	return &FeeCollector{svc: svc}
}

// CollectFee deducts a fee from the account's gas bank.
// The fee is recorded as a withdrawal transaction with the given reference.
func (fc *FeeCollector) CollectFee(ctx context.Context, accountID string, amount int64, reference string) error {
	if amount <= 0 {
		return nil // No fee to collect
	}

	// Find the gas account for this user account
	accounts, err := fc.svc.store.ListGasAccounts(ctx, accountID)
	if err != nil {
		return fmt.Errorf("list gas accounts: %w", err)
	}
	if len(accounts) == 0 {
		return fmt.Errorf("no gas account found for account %s", accountID)
	}

	gasAccount := accounts[0]
	feeAmount := float64(amount) / 1e8 // Convert from smallest unit (like GAS decimals)

	// Check available balance
	if gasAccount.Available < feeAmount {
		return fmt.Errorf("insufficient gas balance: available %.8f, required %.8f", gasAccount.Available, feeAmount)
	}

	// Deduct fee by creating a withdrawal transaction
	// Use internal deduction (no blockchain tx needed for service fees)
	gasAccount.Available -= feeAmount
	gasAccount.Locked += feeAmount // Lock until service completes

	updated, err := fc.svc.store.UpdateGasAccount(ctx, gasAccount)
	if err != nil {
		return fmt.Errorf("update gas account: %w", err)
	}

	// Record the fee transaction
	tx := Transaction{
		AccountID:     updated.ID,
		UserAccountID: updated.AccountID,
		Type:          "fee",
		Amount:        feeAmount,
		NetAmount:     feeAmount,
		Status:        StatusCompleted,
		Notes:         reference,
	}
	if _, err := fc.svc.store.CreateGasTransaction(ctx, tx); err != nil {
		// Attempt rollback
		gasAccount.Available += feeAmount
		gasAccount.Locked -= feeAmount
		if _, rollbackErr := fc.svc.store.UpdateGasAccount(ctx, gasAccount); rollbackErr != nil {
			fc.svc.Logger().WithError(rollbackErr).
				WithField("account_id", accountID).
				Error("failed to rollback fee collection")
		}
		return fmt.Errorf("create fee transaction: %w", err)
	}

	fc.svc.Logger().WithField("account_id", accountID).
		WithField("gas_account_id", updated.ID).
		WithField("fee", feeAmount).
		WithField("reference", reference).
		Info("oracle fee collected")

	return nil
}

// RefundFee returns a previously collected fee to the account's gas bank.
// The refund is recorded as a deposit transaction with the given reference.
func (fc *FeeCollector) RefundFee(ctx context.Context, accountID string, amount int64, reference string) error {
	if amount <= 0 {
		return nil // No fee to refund
	}

	// Find the gas account for this user account
	accounts, err := fc.svc.store.ListGasAccounts(ctx, accountID)
	if err != nil {
		return fmt.Errorf("list gas accounts: %w", err)
	}
	if len(accounts) == 0 {
		return fmt.Errorf("no gas account found for account %s", accountID)
	}

	gasAccount := accounts[0]
	refundAmount := float64(amount) / 1e8 // Convert from smallest unit

	// Credit the refund back
	gasAccount.Available += refundAmount
	if gasAccount.Locked >= refundAmount {
		gasAccount.Locked -= refundAmount
	} else {
		gasAccount.Locked = 0
	}

	updated, err := fc.svc.store.UpdateGasAccount(ctx, gasAccount)
	if err != nil {
		return fmt.Errorf("update gas account: %w", err)
	}

	// Record the refund transaction
	tx := Transaction{
		AccountID:     updated.ID,
		UserAccountID: updated.AccountID,
		Type:          "refund",
		Amount:        refundAmount,
		NetAmount:     refundAmount,
		Status:        StatusCompleted,
		Notes:         reference,
	}
	if _, err := fc.svc.store.CreateGasTransaction(ctx, tx); err != nil {
		fc.svc.Logger().WithError(err).
			WithField("account_id", accountID).
			WithField("reference", reference).
			Warn("failed to record refund transaction (balance already updated)")
	}

	fc.svc.Logger().WithField("account_id", accountID).
		WithField("gas_account_id", updated.ID).
		WithField("refund", refundAmount).
		WithField("reference", reference).
		Info("oracle fee refunded")

	return nil
}

// SettleFee moves locked fee to final balance after successful service completion.
// This should be called when an oracle request completes successfully.
func (fc *FeeCollector) SettleFee(ctx context.Context, accountID string, amount int64, reference string) error {
	if amount <= 0 {
		return nil
	}

	accounts, err := fc.svc.store.ListGasAccounts(ctx, accountID)
	if err != nil {
		return fmt.Errorf("list gas accounts: %w", err)
	}
	if len(accounts) == 0 {
		return fmt.Errorf("no gas account found for account %s", accountID)
	}

	gasAccount := accounts[0]
	feeAmount := float64(amount) / 1e8

	// Move from locked to finalized (deduct from balance)
	if gasAccount.Locked >= feeAmount {
		gasAccount.Locked -= feeAmount
	} else {
		gasAccount.Locked = 0
	}
	gasAccount.Balance -= feeAmount

	if _, err := fc.svc.store.UpdateGasAccount(ctx, gasAccount); err != nil {
		return fmt.Errorf("update gas account: %w", err)
	}

	fc.svc.Logger().WithField("account_id", accountID).
		WithField("fee", feeAmount).
		WithField("reference", reference).
		Debug("oracle fee settled")

	return nil
}
