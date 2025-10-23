package gasbank

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/gasbank"
	"github.com/R3E-Network/service_layer/internal/app/storage"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Service wraps business logic for the gas bank module.
type Service struct {
	accounts storage.AccountStore
	store    storage.GasBankStore
	log      *logger.Logger
}

// New constructs a gas bank service.
func New(accounts storage.AccountStore, store storage.GasBankStore, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("gasbank")
	}
	return &Service{accounts: accounts, store: store, log: log}
}

var (
	errInvalidAmount     = errors.New("amount must be positive")
	errInsufficientFunds = errors.New("insufficient funds")
	ErrWalletInUse       = errors.New("wallet address already assigned to another account")
)

// EnsureAccount retrieves a gas account for the given owner account, creating
// one if it does not exist.
func (s *Service) EnsureAccount(ctx context.Context, accountID string, walletAddress string) (gasbank.Account, error) {
	if accountID == "" {
		return gasbank.Account{}, fmt.Errorf("account_id required")
	}

	if s.accounts != nil {
		if _, err := s.accounts.GetAccount(ctx, accountID); err != nil {
			return gasbank.Account{}, fmt.Errorf("account validation failed: %w", err)
		}
	}

	normalizedWallet := NormalizeWallet(walletAddress)
	if normalizedWallet != "" {
		// Ensure the wallet is not already linked to a different account.
		allAccounts, err := s.store.ListGasAccounts(ctx, "")
		if err != nil {
			return gasbank.Account{}, err
		}
		for _, existing := range allAccounts {
			if NormalizeWallet(existing.WalletAddress) == normalizedWallet && existing.AccountID != accountID {
				return gasbank.Account{}, ErrWalletInUse
			}
		}
	}

	accounts, err := s.store.ListGasAccounts(ctx, accountID)
	if err != nil {
		return gasbank.Account{}, err
	}
	if len(accounts) > 0 {
		acct := accounts[0]
		if normalizedWallet != "" && NormalizeWallet(acct.WalletAddress) != normalizedWallet {
			acct.WalletAddress = normalizedWallet
			updated, err := s.store.UpdateGasAccount(ctx, acct)
			if err != nil {
				return gasbank.Account{}, err
			}
			return updated, nil
		}
		return acct, nil
	}

	acct := gasbank.Account{AccountID: accountID, WalletAddress: normalizedWallet}
	created, err := s.store.CreateGasAccount(ctx, acct)
	if err != nil {
		return gasbank.Account{}, err
	}
	s.log.WithField("gas_account_id", created.ID).
		WithField("account_id", accountID).
		WithField("wallet", created.WalletAddress).
		Info("gas account ensured")
	return created, nil
}

// Deposit credits the specified amount and records a transaction.
func (s *Service) Deposit(ctx context.Context, gasAccountID string, amount float64, txID string, from string, to string) (gasbank.Account, gasbank.Transaction, error) {
	if amount <= 0 {
		return gasbank.Account{}, gasbank.Transaction{}, errInvalidAmount
	}

	acct, err := s.store.GetGasAccount(ctx, gasAccountID)
	if err != nil {
		return gasbank.Account{}, gasbank.Transaction{}, err
	}
	original := acct

	if acct.AccountID != "" && s.accounts != nil {
		if _, err := s.accounts.GetAccount(ctx, acct.AccountID); err != nil {
			return gasbank.Account{}, gasbank.Transaction{}, fmt.Errorf("account validation failed: %w", err)
		}
	}

	updated := acct
	updated.Balance += amount
	updated.Available += amount
	updated.UpdatedAt = time.Now().UTC()
	if updated, err = s.store.UpdateGasAccount(ctx, updated); err != nil {
		return gasbank.Account{}, gasbank.Transaction{}, err
	}

	tx := gasbank.Transaction{
		AccountID:      updated.ID,
		UserAccountID:  updated.AccountID,
		Type:           gasbank.TransactionDeposit,
		Amount:         amount,
		NetAmount:      amount,
		Status:         gasbank.StatusCompleted,
		BlockchainTxID: txID,
		FromAddress:    from,
		ToAddress:      to,
	}
	tx, err = s.store.CreateGasTransaction(ctx, tx)
	if err != nil {
		if _, rollbackErr := s.store.UpdateGasAccount(ctx, original); rollbackErr != nil {
			s.log.WithError(rollbackErr).
				WithField("gas_account_id", original.ID).
				Error("failed to rollback gas account after deposit failure")
		}
		return gasbank.Account{}, gasbank.Transaction{}, fmt.Errorf("create gas transaction: %w", err)
	}
	s.log.WithField("gas_account_id", updated.ID).
		WithField("account_id", updated.AccountID).
		WithField("amount", amount).
		WithField("tx_id", txID).
		Info("gas deposit recorded")
	return updated, tx, nil
}

// Withdraw debits the specified amount if funds are available.
func (s *Service) Withdraw(ctx context.Context, gasAccountID string, amount float64, to string) (gasbank.Account, gasbank.Transaction, error) {
	if amount <= 0 {
		return gasbank.Account{}, gasbank.Transaction{}, errInvalidAmount
	}

	acct, err := s.store.GetGasAccount(ctx, gasAccountID)
	if err != nil {
		return gasbank.Account{}, gasbank.Transaction{}, err
	}
	original := acct

	if acct.Available < amount-Epsilon {
		return gasbank.Account{}, gasbank.Transaction{}, errInsufficientFunds
	}

	updated := acct
	updated.Available -= amount
	updated.Pending += amount
	updated.UpdatedAt = time.Now().UTC()

	tx := gasbank.Transaction{
		AccountID:     updated.ID,
		UserAccountID: updated.AccountID,
		Type:          gasbank.TransactionWithdrawal,
		Amount:        amount,
		NetAmount:     amount,
		Status:        gasbank.StatusPending,
		ToAddress:     to,
	}

	if updated, err = s.store.UpdateGasAccount(ctx, updated); err != nil {
		return gasbank.Account{}, gasbank.Transaction{}, err
	}

	tx, err = s.store.CreateGasTransaction(ctx, tx)
	if err != nil {
		if _, rollbackErr := s.store.UpdateGasAccount(ctx, original); rollbackErr != nil {
			s.log.WithError(rollbackErr).
				WithField("gas_account_id", original.ID).
				Error("failed to rollback gas account after withdrawal failure")
		}
		return gasbank.Account{}, gasbank.Transaction{}, fmt.Errorf("create gas transaction: %w", err)
	}
	s.log.WithField("gas_account_id", updated.ID).
		WithField("account_id", updated.AccountID).
		WithField("amount", amount).
		WithField("destination", to).
		Info("gas withdrawal requested")
	return updated, tx, nil
}

// GetAccount returns the requested gas account.
func (s *Service) GetAccount(ctx context.Context, id string) (gasbank.Account, error) {
	return s.store.GetGasAccount(ctx, id)
}

// ListAccounts returns gas accounts for the specified owner.
func (s *Service) ListAccounts(ctx context.Context, ownerAccountID string) ([]gasbank.Account, error) {
	return s.store.ListGasAccounts(ctx, ownerAccountID)
}

// ListTransactions returns transactions for a gas account.
func (s *Service) ListTransactions(ctx context.Context, gasAccountID string) ([]gasbank.Transaction, error) {
	return s.store.ListGasTransactions(ctx, gasAccountID)
}

// NormalizeWallet ensures wallet addresses are compared case-insensitively.
func NormalizeWallet(addr string) string {
	return strings.ToLower(strings.TrimSpace(addr))
}

const Epsilon = 1e-8

// CompleteWithdrawal finalises a pending withdrawal transaction. When success
// is false, funds are returned to the available balance.
func (s *Service) CompleteWithdrawal(ctx context.Context, txID string, success bool, errMsg string) (gasbank.Account, gasbank.Transaction, error) {
	if strings.TrimSpace(txID) == "" {
		return gasbank.Account{}, gasbank.Transaction{}, fmt.Errorf("transaction id required")
	}

	tx, err := s.store.GetGasTransaction(ctx, txID)
	if err != nil {
		return gasbank.Account{}, gasbank.Transaction{}, err
	}
	if tx.Type != gasbank.TransactionWithdrawal {
		return gasbank.Account{}, gasbank.Transaction{}, fmt.Errorf("transaction %s is not a withdrawal", txID)
	}

	acct, err := s.store.GetGasAccount(ctx, tx.AccountID)
	if err != nil {
		return gasbank.Account{}, gasbank.Transaction{}, err
	}

	if success {
		if acct.Pending < tx.Amount-Epsilon {
			return gasbank.Account{}, gasbank.Transaction{}, fmt.Errorf("pending balance insufficient to settle withdrawal")
		}
		acct.Pending -= tx.Amount
		acct.Balance = math.Max(acct.Balance-tx.Amount, 0)
		tx.Status = gasbank.StatusCompleted
		tx.Error = ""
	} else {
		acct.Pending -= tx.Amount
		acct.Available += tx.Amount
		tx.Status = gasbank.StatusFailed
		tx.Error = errMsg
		tx.NetAmount = 0
	}

	acct.UpdatedAt = time.Now().UTC()
	acct, err = s.store.UpdateGasAccount(ctx, acct)
	if err != nil {
		return gasbank.Account{}, gasbank.Transaction{}, err
	}

	tx.UpdatedAt = time.Now().UTC()
	if success {
		tx.CompletedAt = time.Now().UTC()
	}
	if tx, err = s.store.UpdateGasTransaction(ctx, tx); err != nil {
		return gasbank.Account{}, gasbank.Transaction{}, err
	}

	s.log.WithField("gas_account_id", acct.ID).
		WithField("transaction_id", tx.ID).
		WithField("account_id", acct.AccountID).
		WithField("success", success).
		Info("gas withdrawal settled")
	return acct, tx, nil
}
