package gasbank

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	accountsvc "github.com/R3E-Network/service_layer/packages/com.r3e.services.accounts/service"
	"github.com/R3E-Network/service_layer/pkg/logger"
	"github.com/R3E-Network/service_layer/system/sandbox"
	
	"github.com/R3E-Network/service_layer/system/framework"
	core "github.com/R3E-Network/service_layer/system/framework/core"
)

// Service wraps business logic for the gas bank module.
// Uses SandboxedServiceEngine for capability-based access control.
type Service struct {
	*framework.SandboxedServiceEngine
	store Store
}

// New constructs a gas bank service with sandbox support.
func New(accounts AccountChecker, store Store, log *logger.Logger) *Service {
	return &Service{
		SandboxedServiceEngine: framework.NewSandboxedServiceEngine(framework.SandboxedServiceConfig{
			ServiceConfig: framework.ServiceConfig{
				Name:        "gasbank",
				Domain:      "gasbank",
				Description: "Service-owned gas accounts and settlements",
				DependsOn:   []string{"store", "svc-accounts"},
				RequiresAPIs: []framework.APISurface{
					framework.APISurfaceStore,
				},
				Capabilities: []string{"gasbank"},
				Quotas:       map[string]string{"gas": "account-balances"},
				Accounts:     accounts,
				Logger:       log,
			},
			SecurityLevel: sandbox.SecurityLevelPrivileged,
			RequestedCapabilities: []sandbox.Capability{
				sandbox.CapStorageRead,
				sandbox.CapStorageWrite,
				sandbox.CapDatabaseRead,
				sandbox.CapDatabaseWrite,
				sandbox.CapBusPublish,
				sandbox.CapServiceCall,
			},
			StorageQuota: 10 * 1024 * 1024, // 10MB
		}),
		store: store,
	}
}

// Summary aggregates balances and pending withdrawals for an account.
type Summary struct {
	Accounts           []AccountSummary  `json:"accounts"`
	PendingWithdrawals int               `json:"pending_withdrawals"`
	PendingAmount      float64           `json:"pending_amount"`
	TotalBalance       float64           `json:"total_balance"`
	TotalAvailable     float64           `json:"total_available"`
	TotalLocked        float64           `json:"total_locked"`
	LastDeposit        *TransactionBrief `json:"last_deposit,omitempty"`
	LastWithdrawal     *TransactionBrief `json:"last_withdrawal,omitempty"`
	GeneratedAt        time.Time         `json:"generated_at"`
}

// AccountSummary provides per-gas-account rollups.
type AccountSummary struct {
	Account            GasBankAccount `json:"account"`
	PendingWithdrawals int            `json:"pending_withdrawals"`
	PendingAmount      float64        `json:"pending_amount"`
}

// TransactionBrief captures high-level transaction information for dashboards.
type TransactionBrief struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Amount      float64   `json:"amount"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	CompletedAt time.Time `json:"completed_at"`
	FromAddress string    `json:"from_address,omitempty"`
	ToAddress   string    `json:"to_address,omitempty"`
	Error       string    `json:"error,omitempty"`
}

// Start/Stop/Ready are inherited from framework.ServiceEngine.

var (
	errInvalidAmount     = errors.New("amount must be positive")
	errInsufficientFunds = errors.New("insufficient funds")
	errMinBalance        = errors.New("insufficient funds to maintain minimum balance")
	errDailyLimit        = errors.New("daily withdrawal limit exceeded")
	errCronUnsupported   = errors.New("cron expressions are not supported yet; use schedule_at for deferred withdrawals")
	ErrWalletInUse       = errors.New("wallet address already assigned to another account")
)

// EnsureAccountOptions captures optional parameters when ensuring a gas account.
type EnsureAccountOptions struct {
	WalletAddress         string
	MinBalance            *float64
	DailyLimit            *float64
	NotificationThreshold *float64
	RequiredApprovals     *int
}

// EnsureAccount retrieves a gas account for the given owner account, creating
// one if it does not exist.
func (s *Service) EnsureAccount(ctx context.Context, accountID string, walletAddress string) (GasBankAccount, error) {
	return s.EnsureAccountWithOptions(ctx, accountID, EnsureAccountOptions{WalletAddress: walletAddress})
}

// EnsureAccountWithOptions allows setting configuration parameters while ensuring the account exists.
func (s *Service) EnsureAccountWithOptions(ctx context.Context, accountID string, opts EnsureAccountOptions) (result GasBankAccount, err error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return GasBankAccount{}, core.RequiredError("account_id")
	}
	attrs := map[string]string{"account_id": accountID, "resource": "gas_account"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer func() { finish(err) }()

	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return GasBankAccount{}, fmt.Errorf("account validation failed: %w", err)
	}

	normalizedWallet := accountsvc.NormalizeWalletAddress(opts.WalletAddress)
	if normalizedWallet != "" {
		// Ensure the wallet is not already linked to a different account.
		allAccounts, err := s.store.ListGasAccounts(ctx, "")
		if err != nil {
			return GasBankAccount{}, err
		}
		for _, existing := range allAccounts {
			if accountsvc.NormalizeWalletAddress(existing.WalletAddress) == normalizedWallet && existing.AccountID != accountID {
				return GasBankAccount{}, ErrWalletInUse
			}
		}
	}

	accounts, err := s.store.ListGasAccounts(ctx, accountID)
	if err != nil {
		return GasBankAccount{}, err
	}
	if len(accounts) > 0 {
		acct := accounts[0]
		attrs["gas_account_id"] = acct.ID
		if normalizedWallet != "" && accountsvc.NormalizeWalletAddress(acct.WalletAddress) != normalizedWallet {
			acct.WalletAddress = normalizedWallet
		}
		applyEnsureOptions(&acct, opts)
		if normalizedWallet != "" || hasEnsureFields(opts) {
			updated, err := s.store.UpdateGasAccount(ctx, acct)
			if err != nil {
				return GasBankAccount{}, err
			}
			s.Logger().WithField("gas_account_id", updated.ID).
				WithField("account_id", updated.AccountID).
				Info("gas account updated")
			s.LogUpdated("gas_account", updated.ID, updated.AccountID)
			s.IncrementCounter("gasbank_accounts_updated_total", map[string]string{"account_id": updated.AccountID})
			return updated, nil
		}
		return acct, nil
	}

	acct := GasBankAccount{AccountID: accountID, WalletAddress: normalizedWallet}
	applyEnsureOptions(&acct, opts)
	created, err := s.store.CreateGasAccount(ctx, acct)
	if err != nil {
		return GasBankAccount{}, err
	}
	attrs["gas_account_id"] = created.ID
	s.Logger().WithField("gas_account_id", created.ID).
		WithField("account_id", accountID).
		WithField("wallet", created.WalletAddress).
		Info("gas account ensured")
	s.LogCreated("gas_account", created.ID, accountID)
	s.IncrementCounter("gasbank_accounts_ensured_total", map[string]string{"account_id": accountID})
	return created, nil
}

// Deposit credits the specified amount and records a transaction.
func (s *Service) Deposit(ctx context.Context, gasAccountID string, amount float64, txID string, from string, to string) (_ GasBankAccount, _ Transaction, err error) {
	if amount <= 0 {
		return GasBankAccount{}, Transaction{}, errInvalidAmount
	}
	attrs := map[string]string{"gas_account_id": gasAccountID, "resource": "gasbank_deposit"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer func() { finish(err) }()

	acct, err := s.store.GetGasAccount(ctx, gasAccountID)
	if err != nil {
		return GasBankAccount{}, Transaction{}, err
	}
	original := acct
	attrs["account_id"] = acct.AccountID

	if acct.AccountID != "" {
		if err := s.ValidateAccountExists(ctx, acct.AccountID); err != nil {
			return GasBankAccount{}, Transaction{}, fmt.Errorf("account validation failed: %w", err)
		}
	}

	updated := acct
	updated.Balance += amount
	updated.Available += amount
	updated.UpdatedAt = time.Now().UTC()
	if updated, err = s.store.UpdateGasAccount(ctx, updated); err != nil {
		return GasBankAccount{}, Transaction{}, err
	}

	tx := Transaction{
		AccountID:      updated.ID,
		UserAccountID:  updated.AccountID,
		Type:           TransactionDeposit,
		Amount:         amount,
		NetAmount:      amount,
		Status:         StatusCompleted,
		BlockchainTxID: txID,
		FromAddress:    from,
		ToAddress:      to,
	}
	tx, err = s.store.CreateGasTransaction(ctx, tx)
	if err != nil {
		if _, rollbackErr := s.store.UpdateGasAccount(ctx, original); rollbackErr != nil {
			s.Logger().WithError(rollbackErr).
				WithField("gas_account_id", original.ID).
				Error("failed to rollback gas account after deposit failure")
		}
		return GasBankAccount{}, Transaction{}, fmt.Errorf("create gas transaction: %w", err)
	}
	s.Logger().WithField("gas_account_id", updated.ID).
		WithField("account_id", updated.AccountID).
		WithField("amount", amount).
		WithField("tx_id", txID).
		Info("gas deposit recorded")
	s.LogAction("deposit", "gas_account", updated.ID, updated.AccountID)
	s.IncrementCounter("gasbank_deposits_total", map[string]string{"account_id": updated.AccountID, "gas_account_id": updated.ID})
	s.ObserveHistogram("gasbank_deposit_amount", map[string]string{"gas_account_id": updated.ID}, amount)
	return updated, tx, nil
}

// Withdraw debits the specified amount if funds are available.
func (s *Service) Withdraw(ctx context.Context, accountID, gasAccountID string, amount float64, to string) (GasBankAccount, Transaction, error) {
	return s.WithdrawWithOptions(ctx, accountID, gasAccountID, WithdrawOptions{
		Amount:    amount,
		ToAddress: to,
	})
}

// WithdrawWithOptions debits funds with scheduling and limit enforcement.
func (s *Service) WithdrawWithOptions(ctx context.Context, accountID, gasAccountID string, opts WithdrawOptions) (_ GasBankAccount, _ Transaction, err error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return GasBankAccount{}, Transaction{}, fmt.Errorf("account_id required")
	}
	attrs := map[string]string{"account_id": accountID, "gas_account_id": gasAccountID, "resource": "gasbank_withdraw"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer func() { finish(err) }()
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return GasBankAccount{}, Transaction{}, fmt.Errorf("account validation failed: %w", err)
	}

	amount := opts.Amount
	if amount <= 0 {
		return GasBankAccount{}, Transaction{}, errInvalidAmount
	}

	acct, err := s.store.GetGasAccount(ctx, gasAccountID)
	if err != nil {
		return GasBankAccount{}, Transaction{}, err
	}
	original := acct

	if err := core.EnsureOwnership(acct.AccountID, accountID, "gas account", gasAccountID); err != nil {
		return GasBankAccount{}, Transaction{}, err
	}

	if acct.Available < amount-Epsilon {
		return GasBankAccount{}, Transaction{}, errInsufficientFunds
	}

	now := time.Now().UTC()
	if acct.MinBalance > 0 && acct.Available-amount < acct.MinBalance-Epsilon {
		return GasBankAccount{}, Transaction{}, errMinBalance
	}
	dailyUsed := acct.DailyWithdrawal
	if acct.LastWithdrawal.IsZero() || !sameDay(acct.LastWithdrawal, now) {
		dailyUsed = 0
	}
	if acct.DailyLimit > 0 && dailyUsed+amount > acct.DailyLimit+Epsilon {
		return GasBankAccount{}, Transaction{}, errDailyLimit
	}

	updated := acct
	updated.Available -= amount
	updated.Pending += amount
	updated.Locked += amount
	updated.DailyWithdrawal = dailyUsed + amount
	updated.LastWithdrawal = now
	updated.UpdatedAt = now

	scheduleAt := time.Time{}
	if opts.ScheduleAt != nil {
		scheduleAt = opts.ScheduleAt.UTC()
	}
	isScheduled := false
	if !scheduleAt.IsZero() && scheduleAt.After(now) {
		isScheduled = true
	} else {
		scheduleAt = time.Time{}
	}
	cronExpr := strings.TrimSpace(opts.CronExpression)
	if cronExpr != "" {
		return GasBankAccount{}, Transaction{}, errCronUnsupported
	}

	requiredApprovals := updated.RequiredApprovals
	tx := Transaction{
		AccountID:      updated.ID,
		UserAccountID:  updated.AccountID,
		Type:           TransactionWithdrawal,
		Amount:         amount,
		NetAmount:      amount,
		Status:         StatusPending,
		ToAddress:      opts.ToAddress,
		ScheduleAt:     scheduleAt,
		CronExpression: cronExpr,
		ApprovalPolicy: ApprovalPolicy{Required: requiredApprovals},
	}
	if requiredApprovals > 0 {
		tx.Status = StatusAwaitingApproval
	}
	if isScheduled {
		tx.Status = StatusScheduled
	}

	if updated, err = s.store.UpdateGasAccount(ctx, updated); err != nil {
		return GasBankAccount{}, Transaction{}, err
	}

	tx, err = s.store.CreateGasTransaction(ctx, tx)
	if err != nil {
		if _, rollbackErr := s.store.UpdateGasAccount(ctx, original); rollbackErr != nil {
			s.Logger().WithError(rollbackErr).
				WithField("gas_account_id", original.ID).
				Error("failed to rollback gas account after withdrawal failure")
		}
		return GasBankAccount{}, Transaction{}, fmt.Errorf("create gas transaction: %w", err)
	}

	if tx.Status == StatusScheduled {
		schedule := WithdrawalSchedule{
			TransactionID:  tx.ID,
			ScheduleAt:     scheduleAt,
			CronExpression: cronExpr,
			NextRunAt:      scheduleAt,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if _, err := s.store.SaveWithdrawalSchedule(ctx, schedule); err != nil {
			s.Logger().WithError(err).
				WithField("transaction_id", tx.ID).
				Warn("failed to persist withdrawal schedule")
		}
	}

	s.Logger().WithField("gas_account_id", updated.ID).
		WithField("account_id", updated.AccountID).
		WithField("amount", amount).
		WithField("destination", opts.ToAddress).
		WithField("scheduled_at", scheduleAt).
		Info("gas withdrawal requested")
	s.LogAction("withdrawal_requested", "gas_account", updated.ID, updated.AccountID)
	s.IncrementCounter("gasbank_withdrawals_total", map[string]string{"account_id": updated.AccountID, "gas_account_id": updated.ID})
	s.ObserveHistogram("gasbank_withdraw_amount", map[string]string{"gas_account_id": updated.ID}, amount)
	return updated, tx, nil
}

// GetAccount returns the requested gas account.
func (s *Service) GetAccount(ctx context.Context, id string) (GasBankAccount, error) {
	return s.store.GetGasAccount(ctx, id)
}

// ListAccounts returns gas accounts for the specified owner.
func (s *Service) ListAccounts(ctx context.Context, ownerAccountID string) ([]GasBankAccount, error) {
	if strings.TrimSpace(ownerAccountID) != "" {
		if err := s.ValidateAccountExists(ctx, ownerAccountID); err != nil {
			return nil, err
		}
	}
	return s.store.ListGasAccounts(ctx, ownerAccountID)
}

// Summary aggregates balances and activity for the specified owner account.
func (s *Service) Summary(ctx context.Context, ownerAccountID string) (Summary, error) {
	ownerAccountID = strings.TrimSpace(ownerAccountID)
	if ownerAccountID == "" {
		return Summary{}, fmt.Errorf("account_id required")
	}
	if err := s.ValidateAccountExists(ctx, ownerAccountID); err != nil {
		return Summary{}, err
	}
	attrs := map[string]string{"account_id": ownerAccountID, "resource": "gasbank_summary"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer finish(nil)

	accts, err := s.store.ListGasAccounts(ctx, ownerAccountID)
	if err != nil {
		finish(err)
		return Summary{}, err
	}

	summary := Summary{
		Accounts:    make([]AccountSummary, 0, len(accts)),
		GeneratedAt: time.Now().UTC(),
	}

	for _, acct := range accts {
		acctSummary := AccountSummary{Account: acct}
		summary.TotalBalance += acct.Balance
		summary.TotalAvailable += acct.Available
		summary.TotalLocked += acct.Locked

		txs, err := s.store.ListGasTransactions(ctx, acct.ID, core.DefaultListLimit)
		if err != nil {
			return Summary{}, err
		}
		for _, tx := range txs {
			if tx.Type == TransactionWithdrawal && isActiveWithdrawalStatus(tx.Status) {
				summary.PendingWithdrawals++
				summary.PendingAmount += tx.Amount
				acctSummary.PendingWithdrawals++
				acctSummary.PendingAmount += tx.Amount
			}
			if tx.Type == TransactionDeposit {
				summary.LastDeposit = latestBrief(summary.LastDeposit, tx)
			}
			if tx.Type == TransactionWithdrawal {
				summary.LastWithdrawal = latestBrief(summary.LastWithdrawal, tx)
			}
		}
		summary.Accounts = append(summary.Accounts, acctSummary)
	}

	return summary, nil
}

// ListTransactions returns transactions for a gas account.
func (s *Service) ListTransactions(ctx context.Context, gasAccountID string, limit int) ([]Transaction, error) {
	return s.ListTransactionsFiltered(ctx, gasAccountID, "", "", limit)
}

// ListTransactionsFiltered returns transactions filtered by type/status.
func (s *Service) ListTransactionsFiltered(ctx context.Context, gasAccountID, txType, status string, limit int) ([]Transaction, error) {
	clamped := core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
	attrs := map[string]string{"gas_account_id": gasAccountID, "resource": "gasbank_list_transactions"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer finish(nil)
	items, err := s.store.ListGasTransactions(ctx, gasAccountID, clamped)
	if err != nil {
		return nil, err
	}
	filtered := make([]Transaction, 0, len(items))
	txType = strings.TrimSpace(txType)
	status = strings.TrimSpace(status)
	for _, tx := range items {
		if txType != "" && tx.Type != txType {
			continue
		}
		if status != "" && tx.Status != status {
			continue
		}
		filtered = append(filtered, tx)
	}
	return filtered, nil
}

// ActivateDueSchedules promotes scheduled withdrawals whose schedule is due.
func (s *Service) ActivateDueSchedules(ctx context.Context, limit int) error {
	if limit <= 0 {
		limit = 50
	}
	attrs := map[string]string{"resource": "gasbank_activate_schedules"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer finish(nil)
	due, err := s.store.ListDueWithdrawalSchedules(ctx, time.Now().UTC(), limit)
	if err != nil {
		return err
	}
	for _, schedule := range due {
		tx, err := s.store.GetGasTransaction(ctx, schedule.TransactionID)
		if err != nil {
			s.Logger().WithError(err).
				WithField("transaction_id", schedule.TransactionID).
				Warn("activate schedule: get transaction failed")
			continue
		}
		if tx.Status != StatusScheduled {
			_ = s.store.DeleteWithdrawalSchedule(ctx, schedule.TransactionID)
			continue
		}
		acct, err := s.store.GetGasAccount(ctx, tx.AccountID)
		if err != nil {
			s.Logger().WithError(err).
				WithField("transaction_id", schedule.TransactionID).
				Warn("activate schedule: get gas account failed")
			continue
		}
		nextStatus := StatusPending
		required := tx.ApprovalPolicy.Required
		if required <= 0 {
			required = acct.RequiredApprovals
		}
		if required > 0 {
			nextStatus = StatusAwaitingApproval
		}

		tx.Status = nextStatus
		tx.ScheduleAt = time.Time{}
		tx.CronExpression = ""
		tx.UpdatedAt = time.Now().UTC()
		if _, err := s.store.UpdateGasTransaction(ctx, tx); err != nil {
			s.Logger().WithError(err).
				WithField("transaction_id", tx.ID).
				Warn("activate schedule: update transaction failed")
			continue
		}
		if err := s.store.DeleteWithdrawalSchedule(ctx, tx.ID); err != nil {
			s.Logger().WithError(err).
				WithField("transaction_id", tx.ID).
				Warn("activate schedule: delete schedule failed")
		}
		s.Logger().WithField("transaction_id", tx.ID).
			WithField("account_id", acct.AccountID).
			Info("scheduled withdrawal activated")
	}
	return nil
}

// GetWithdrawal returns a withdrawal transaction for the specified account.
func (s *Service) GetWithdrawal(ctx context.Context, accountID, transactionID string) (Transaction, error) {
	accountID = strings.TrimSpace(accountID)
	transactionID = strings.TrimSpace(transactionID)
	if accountID == "" || transactionID == "" {
		return Transaction{}, fmt.Errorf("account_id and transaction_id are required")
	}
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return Transaction{}, err
	}
	attrs := map[string]string{"account_id": accountID, "transaction_id": transactionID, "resource": "gasbank_get_withdrawal"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer finish(nil)
	tx, err := s.store.GetGasTransaction(ctx, transactionID)
	if err != nil {
		return Transaction{}, err
	}
	if err := core.EnsureOwnership(tx.UserAccountID, accountID, "withdrawal", transactionID); err != nil {
		return Transaction{}, err
	}
	if tx.Type != TransactionWithdrawal {
		return Transaction{}, fmt.Errorf("transaction %s is not a withdrawal", transactionID)
	}
	return tx, nil
}

// ListApprovals returns recorded approvals for a withdrawal.
func (s *Service) ListApprovals(ctx context.Context, transactionID string) ([]WithdrawalApproval, error) {
	transactionID = strings.TrimSpace(transactionID)
	if transactionID == "" {
		return nil, fmt.Errorf("transaction_id required")
	}
	attrs := map[string]string{"transaction_id": transactionID, "resource": "gasbank_list_approvals"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer finish(nil)
	return s.store.ListWithdrawalApprovals(ctx, transactionID)
}

// ListSettlementAttempts returns resolver attempts for a withdrawal.
func (s *Service) ListSettlementAttempts(ctx context.Context, accountID, transactionID string, limit int) ([]SettlementAttempt, error) {
	accountID = strings.TrimSpace(accountID)
	transactionID = strings.TrimSpace(transactionID)
	if accountID == "" || transactionID == "" {
		return nil, fmt.Errorf("account_id and transaction_id are required")
	}
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return nil, err
	}
	tx, err := s.store.GetGasTransaction(ctx, transactionID)
	if err != nil {
		return nil, err
	}
	if err := core.EnsureOwnership(tx.UserAccountID, accountID, "transaction", transactionID); err != nil {
		return nil, err
	}
	clamped := core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
	attrs := map[string]string{"transaction_id": transactionID, "account_id": accountID, "resource": "gasbank_list_attempts"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer finish(nil)
	return s.store.ListSettlementAttempts(ctx, transactionID, clamped)
}

// ListDeadLetters returns dead-lettered withdrawals for an account.
func (s *Service) ListDeadLetters(ctx context.Context, accountID string, limit int) ([]DeadLetter, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, fmt.Errorf("account_id required")
	}
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return nil, err
	}
	clamped := core.ClampLimit(limit, core.DefaultListLimit, core.MaxListLimit)
	return s.store.ListDeadLetters(ctx, accountID, clamped)
}

// RetryDeadLetter requeues a dead-lettered withdrawal.
func (s *Service) RetryDeadLetter(ctx context.Context, accountID, transactionID string) (Transaction, error) {
	accountID = strings.TrimSpace(accountID)
	transactionID = strings.TrimSpace(transactionID)
	if accountID == "" || transactionID == "" {
		return Transaction{}, fmt.Errorf("account_id and transaction_id are required")
	}
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return Transaction{}, err
	}
	attrs := map[string]string{"account_id": accountID, "transaction_id": transactionID, "resource": "gasbank_retry_deadletter"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer finish(nil)
	entry, err := s.store.GetDeadLetter(ctx, transactionID)
	if err != nil {
		return Transaction{}, err
	}
	if err := core.EnsureOwnership(entry.AccountID, accountID, "dead letter", transactionID); err != nil {
		return Transaction{}, err
	}
	tx, err := s.store.GetGasTransaction(ctx, transactionID)
	if err != nil {
		return Transaction{}, err
	}
	nextStatus := StatusPending
	required := tx.ApprovalPolicy.Required
	if required <= 0 {
		acct, err := s.store.GetGasAccount(ctx, tx.AccountID)
		if err == nil && acct.RequiredApprovals > 0 {
			required = acct.RequiredApprovals
		}
	}
	if required > 0 {
		nextStatus = StatusAwaitingApproval
	}
	tx.Status = nextStatus
	tx.DeadLetterReason = ""
	tx.ResolverAttempt = 0
	tx.ResolverError = ""
	tx.LastAttemptAt = time.Time{}
	tx.NextAttemptAt = time.Time{}
	tx.UpdatedAt = time.Now().UTC()
	if _, err := s.store.UpdateGasTransaction(ctx, tx); err != nil {
		return Transaction{}, err
	}
	if err := s.store.RemoveDeadLetter(ctx, transactionID); err != nil {
		return Transaction{}, err
	}
	s.Logger().WithField("transaction_id", tx.ID).
		WithField("account_id", accountID).
		Info("dead-lettered withdrawal requeued")
	s.LogAction("deadletter_retried", "gas_account", tx.AccountID, accountID)
	s.IncrementCounter("gasbank_deadletter_retries_total", map[string]string{"account_id": accountID})
	return tx, nil
}

// DeleteDeadLetter cancels a dead-lettered withdrawal and removes the entry.
func (s *Service) DeleteDeadLetter(ctx context.Context, accountID, transactionID string) error {
	accountID = strings.TrimSpace(accountID)
	transactionID = strings.TrimSpace(transactionID)
	if accountID == "" || transactionID == "" {
		return fmt.Errorf("account_id and transaction_id are required")
	}
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return err
	}
	attrs := map[string]string{"account_id": accountID, "transaction_id": transactionID, "resource": "gasbank_delete_deadletter"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer finish(nil)
	entry, err := s.store.GetDeadLetter(ctx, transactionID)
	if err != nil {
		return err
	}
	if err := core.EnsureOwnership(entry.AccountID, accountID, "dead letter", transactionID); err != nil {
		return err
	}
	tx, err := s.store.GetGasTransaction(ctx, transactionID)
	if err != nil {
		return err
	}
	if tx.Status != StatusCancelled && tx.Status != StatusCompleted {
		if _, _, err := s.cancelWithdrawal(ctx, tx, "dead letter cancelled"); err != nil {
			return err
		}
	}
	if err := s.store.RemoveDeadLetter(ctx, transactionID); err != nil {
		return err
	}
	s.Logger().WithField("transaction_id", transactionID).
		WithField("account_id", accountID).
		Info("dead-lettered withdrawal removed")
	s.LogDeleted("deadletter", transactionID, accountID)
	s.IncrementCounter("gasbank_deadletter_deleted_total", map[string]string{"account_id": accountID})
	return nil
}

// MarkDeadLetter records that a withdrawal has been moved to the dead-letter queue.
func (s *Service) MarkDeadLetter(ctx context.Context, tx Transaction, reason, lastErr string) error {
	if tx.Type != TransactionWithdrawal {
		return fmt.Errorf("transaction %s is not a withdrawal", tx.ID)
	}
	if tx.UserAccountID == "" {
		return fmt.Errorf("transaction user_account_id required")
	}
	if tx.UserAccountID != "" {
		if err := s.ValidateAccountExists(ctx, tx.UserAccountID); err != nil {
			return err
		}
	}
	now := time.Now().UTC()
	tx.Status = StatusDeadLetter
	tx.DeadLetterReason = reason
	tx.ResolverError = lastErr
	tx.NextAttemptAt = time.Time{}
	tx.UpdatedAt = now
	if _, err := s.store.UpdateGasTransaction(ctx, tx); err != nil {
		return err
	}
	entry := DeadLetter{
		TransactionID: tx.ID,
		AccountID:     tx.AccountID,
		Reason:        reason,
		LastError:     lastErr,
		LastAttemptAt: tx.LastAttemptAt,
		Retries:       tx.ResolverAttempt,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if _, err := s.store.UpsertDeadLetter(ctx, entry); err != nil {
		return err
	}
	s.Logger().WithField("transaction_id", tx.ID).
		WithField("account_id", tx.AccountID).
		Warn("withdrawal moved to dead letter")
	s.LogAction("deadletter_marked", "gas_account", tx.AccountID, tx.UserAccountID)
	s.IncrementCounter("gasbank_deadletter_marked_total", map[string]string{"account_id": tx.UserAccountID})
	return nil
}

// CancelWithdrawal cancels a pending withdrawal transaction.
func (s *Service) CancelWithdrawal(ctx context.Context, accountID, transactionID, reason string) (Transaction, error) {
	attrs := map[string]string{"account_id": accountID, "transaction_id": transactionID, "resource": "gasbank_cancel_withdrawal"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer finish(nil)
	tx, err := s.GetWithdrawal(ctx, accountID, transactionID)
	if err != nil {
		return Transaction{}, err
	}
	_, updated, err := s.cancelWithdrawal(ctx, tx, reason)
	if err != nil {
		return Transaction{}, err
	}
	s.LogAction("withdrawal_cancelled", "gas_account", updated.AccountID, accountID)
	s.IncrementCounter("gasbank_withdrawals_cancelled_total", map[string]string{"account_id": accountID})
	return updated, nil
}

// SubmitApproval records an approval or rejection for the specified withdrawal.
func (s *Service) SubmitApproval(ctx context.Context, transactionID, approver, signature, note string, approve bool) (WithdrawalApproval, Transaction, error) {
	transactionID = strings.TrimSpace(transactionID)
	if transactionID == "" {
		return WithdrawalApproval{}, Transaction{}, fmt.Errorf("transaction_id required")
	}
	approver = strings.TrimSpace(approver)
	if approver == "" {
		return WithdrawalApproval{}, Transaction{}, fmt.Errorf("approver required")
	}

	tx, err := s.store.GetGasTransaction(ctx, transactionID)
	if err != nil {
		return WithdrawalApproval{}, Transaction{}, err
	}
	if tx.Type != TransactionWithdrawal {
		return WithdrawalApproval{}, Transaction{}, fmt.Errorf("transaction %s is not a withdrawal", transactionID)
	}

	status := ApprovalApproved
	if !approve {
		status = ApprovalRejected
	}

	approval := WithdrawalApproval{
		TransactionID: transactionID,
		Approver:      approver,
		Status:        status,
		Signature:     signature,
		Note:          note,
		DecidedAt:     time.Now().UTC(),
	}

	attrs := map[string]string{"transaction_id": transactionID, "approver": approver, "resource": "gasbank_submit_approval"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer finish(nil)
	recorded, err := s.store.UpsertWithdrawalApproval(ctx, approval)
	if err != nil {
		return WithdrawalApproval{}, Transaction{}, err
	}

	approvals, err := s.store.ListWithdrawalApprovals(ctx, transactionID)
	if err != nil {
		return WithdrawalApproval{}, Transaction{}, err
	}

	// Evaluate approval requirements once we know the account configuration.
	acct, err := s.store.GetGasAccount(ctx, tx.AccountID)
	if err != nil {
		return WithdrawalApproval{}, Transaction{}, err
	}
	required := tx.ApprovalPolicy.Required
	if required <= 0 {
		required = acct.RequiredApprovals
	}

	switch status {
	case ApprovalRejected:
		if tx.Status != StatusCancelled && tx.Status != StatusFailed {
			if _, _, err := s.cancelWithdrawal(ctx, tx, fmt.Sprintf("rejected by %s", approver)); err != nil {
				return WithdrawalApproval{}, Transaction{}, err
			}
		}
	case ApprovalApproved:
		if required > 0 && countApprovals(approvals) >= required && tx.Status == StatusAwaitingApproval {
			tx.Status = StatusPending
			tx.UpdatedAt = time.Now().UTC()
			if tx, err = s.store.UpdateGasTransaction(ctx, tx); err != nil {
				return WithdrawalApproval{}, Transaction{}, err
			}
		}
	}
	action := "rejected"
	if approve {
		action = "approved"
	}
	s.LogAction("withdrawal_"+action, "gas_account", tx.AccountID, tx.UserAccountID)
	s.IncrementCounter("gasbank_withdrawal_approvals_total", map[string]string{"account_id": tx.UserAccountID, "status": action})

	return recorded, tx, nil
}

const Epsilon = 1e-8

// CompleteWithdrawal finalises a pending withdrawal transaction. When success
// is false, funds are returned to the available balance.
func (s *Service) CompleteWithdrawal(ctx context.Context, txID string, success bool, errMsg string) (GasBankAccount, Transaction, error) {
	if strings.TrimSpace(txID) == "" {
		return GasBankAccount{}, Transaction{}, fmt.Errorf("transaction id required")
	}

	attrs := map[string]string{"transaction_id": txID, "resource": "gasbank_complete_withdrawal"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer finish(nil)
	tx, err := s.store.GetGasTransaction(ctx, txID)
	if err != nil {
		return GasBankAccount{}, Transaction{}, err
	}
	if tx.Type != TransactionWithdrawal {
		return GasBankAccount{}, Transaction{}, fmt.Errorf("transaction %s is not a withdrawal", txID)
	}

	acct, err := s.store.GetGasAccount(ctx, tx.AccountID)
	if err != nil {
		return GasBankAccount{}, Transaction{}, err
	}

	if success {
		if acct.Pending < tx.Amount-Epsilon {
			return GasBankAccount{}, Transaction{}, fmt.Errorf("pending balance insufficient to settle withdrawal")
		}
		acct.Pending -= tx.Amount
		acct.Balance = math.Max(acct.Balance-tx.Amount, 0)
		tx.Status = StatusCompleted
		tx.Error = ""
	} else {
		acct.Pending -= tx.Amount
		acct.Available += tx.Amount
		tx.Status = StatusFailed
		tx.Error = errMsg
		tx.NetAmount = 0
	}
	acct.Locked = math.Max(acct.Locked-tx.Amount, 0)

	acct.UpdatedAt = time.Now().UTC()
	acct, err = s.store.UpdateGasAccount(ctx, acct)
	if err != nil {
		return GasBankAccount{}, Transaction{}, err
	}

	tx.UpdatedAt = time.Now().UTC()
	if success {
		tx.CompletedAt = time.Now().UTC()
	}
	if tx, err = s.store.UpdateGasTransaction(ctx, tx); err != nil {
		return GasBankAccount{}, Transaction{}, err
	}

	s.Logger().WithField("gas_account_id", acct.ID).
		WithField("transaction_id", tx.ID).
		WithField("account_id", acct.AccountID).
		WithField("success", success).
		Info("gas withdrawal settled")
	status := "failed"
	if success {
		status = "completed"
	}
	s.LogAction("withdrawal_"+status, "gas_account", acct.ID, acct.AccountID)
	s.IncrementCounter("gasbank_withdrawal_completions_total", map[string]string{"account_id": acct.AccountID, "status": status})
	return acct, tx, nil
}

func (s *Service) cancelWithdrawal(ctx context.Context, tx Transaction, reason string) (GasBankAccount, Transaction, error) {
	acct, err := s.store.GetGasAccount(ctx, tx.AccountID)
	if err != nil {
		return GasBankAccount{}, Transaction{}, err
	}
	if acct.Pending < tx.Amount-Epsilon {
		return GasBankAccount{}, Transaction{}, fmt.Errorf("pending balance insufficient to cancel withdrawal")
	}
	acct.Pending -= tx.Amount
	acct.Available += tx.Amount
	acct.Locked = math.Max(acct.Locked-tx.Amount, 0)
	acct.UpdatedAt = time.Now().UTC()
	if _, err := s.store.UpdateGasAccount(ctx, acct); err != nil {
		return GasBankAccount{}, Transaction{}, err
	}

	tx.Status = StatusCancelled
	tx.Error = reason
	tx.NetAmount = 0
	tx.UpdatedAt = time.Now().UTC()
	if tx, err = s.store.UpdateGasTransaction(ctx, tx); err != nil {
		return GasBankAccount{}, Transaction{}, err
	}
	s.Logger().WithField("transaction_id", tx.ID).
		WithField("account_id", acct.AccountID).
		WithField("reason", reason).
		Info("gas withdrawal cancelled")
	return acct, tx, nil
}

func latestBrief(current *TransactionBrief, tx Transaction) *TransactionBrief {
	brief := transactionToBrief(tx)
	if current == nil || brief.CreatedAt.After(current.CreatedAt) {
		return &brief
	}
	return current
}

func transactionToBrief(tx Transaction) TransactionBrief {
	return TransactionBrief{
		ID:          tx.ID,
		Type:        tx.Type,
		Amount:      tx.Amount,
		Status:      tx.Status,
		CreatedAt:   tx.CreatedAt,
		CompletedAt: tx.CompletedAt,
		FromAddress: tx.FromAddress,
		ToAddress:   tx.ToAddress,
		Error:       tx.Error,
	}
}

func isActiveWithdrawalStatus(status string) bool {
	switch status {
	case StatusPending,
		StatusAwaitingApproval,
		StatusScheduled,
		StatusApproved:
		return true
	default:
		return false
	}
}

func countApprovals(approvals []WithdrawalApproval) int {
	count := 0
	for _, approval := range approvals {
		if approval.Status == ApprovalApproved {
			count++
		}
	}
	return count
}

func applyEnsureOptions(acct *GasBankAccount, opts EnsureAccountOptions) {
	if opts.MinBalance != nil {
		acct.MinBalance = math.Max(*opts.MinBalance, 0)
	}
	if opts.DailyLimit != nil {
		acct.DailyLimit = math.Max(*opts.DailyLimit, 0)
	}
	if opts.NotificationThreshold != nil {
		acct.NotificationThreshold = math.Max(*opts.NotificationThreshold, 0)
	}
	if opts.RequiredApprovals != nil {
		if *opts.RequiredApprovals < 0 {
			acct.RequiredApprovals = 0
		} else {
			acct.RequiredApprovals = *opts.RequiredApprovals
		}
	}
}

func hasEnsureFields(opts EnsureAccountOptions) bool {
	return opts.MinBalance != nil ||
		opts.DailyLimit != nil ||
		opts.NotificationThreshold != nil ||
		opts.RequiredApprovals != nil
}

func sameDay(a, b time.Time) bool {
	aYear, aMonth, aDay := a.Date()
	bYear, bMonth, bDay := b.Date()
	return aYear == bYear && aMonth == bMonth && aDay == bDay
}

// WithdrawOptions controls how withdrawals are created.
type WithdrawOptions struct {
	Amount         float64
	ToAddress      string
	ScheduleAt     *time.Time
	CronExpression string
}

// HTTP API Methods

// HTTPGetAccounts handles GET /accounts - list gas accounts.
func (s *Service) HTTPGetAccounts(ctx context.Context, req core.APIRequest) (any, error) {
	return s.ListAccounts(ctx, req.AccountID)
}

// HTTPPostAccounts handles POST /accounts - ensure gas account exists.
func (s *Service) HTTPPostAccounts(ctx context.Context, req core.APIRequest) (any, error) {
	walletAddress, _ := req.Body["wallet_address"].(string)
	return s.EnsureAccount(ctx, req.AccountID, walletAddress)
}

// HTTPGetAccountsById handles GET /accounts/{id} - get a specific gas account.
func (s *Service) HTTPGetAccountsById(ctx context.Context, req core.APIRequest) (any, error) {
	return s.GetAccount(ctx, req.PathParams["id"])
}

// HTTPGetSummary handles GET /summary - get account summary.
func (s *Service) HTTPGetSummary(ctx context.Context, req core.APIRequest) (any, error) {
	return s.Summary(ctx, req.AccountID)
}

// HTTPGetTransactions handles GET /transactions - list transactions.
func (s *Service) HTTPGetTransactions(ctx context.Context, req core.APIRequest) (any, error) {
	gasAccountID := req.Query["gas_account_id"]
	txType := req.Query["type"]
	status := req.Query["status"]
	limit := core.ParseLimitFromQuery(req.Query)
	return s.ListTransactionsFiltered(ctx, gasAccountID, txType, status, limit)
}

// HTTPPostWithdrawals handles POST /withdrawals - create withdrawal.
func (s *Service) HTTPPostWithdrawals(ctx context.Context, req core.APIRequest) (any, error) {
	gasAccountID, _ := req.Body["gas_account_id"].(string)
	amount, _ := req.Body["amount"].(float64)
	toAddress, _ := req.Body["to_address"].(string)

	_, tx, err := s.Withdraw(ctx, req.AccountID, gasAccountID, amount, toAddress)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

// HTTPGetWithdrawalsById handles GET /withdrawals/{id} - get withdrawal details.
func (s *Service) HTTPGetWithdrawalsById(ctx context.Context, req core.APIRequest) (any, error) {
	return s.GetWithdrawal(ctx, req.AccountID, req.PathParams["id"])
}

// HTTPPostWithdrawalsIdCancel handles POST /withdrawals/{id}/cancel - cancel withdrawal.
func (s *Service) HTTPPostWithdrawalsIdCancel(ctx context.Context, req core.APIRequest) (any, error) {
	reason, _ := req.Body["reason"].(string)
	return s.CancelWithdrawal(ctx, req.AccountID, req.PathParams["id"], reason)
}

// HTTPGetWithdrawalsIdApprovals handles GET /withdrawals/{id}/approvals - list approvals.
func (s *Service) HTTPGetWithdrawalsIdApprovals(ctx context.Context, req core.APIRequest) (any, error) {
	return s.ListApprovals(ctx, req.PathParams["id"])
}

// HTTPPostWithdrawalsIdApprovals handles POST /withdrawals/{id}/approvals - submit approval.
func (s *Service) HTTPPostWithdrawalsIdApprovals(ctx context.Context, req core.APIRequest) (any, error) {
	approver, _ := req.Body["approver"].(string)
	signature, _ := req.Body["signature"].(string)
	note, _ := req.Body["note"].(string)
	approve, _ := req.Body["approve"].(bool)

	approval, tx, err := s.SubmitApproval(ctx, req.PathParams["id"], approver, signature, note, approve)
	if err != nil {
		return nil, err
	}
	return map[string]any{"approval": approval, "transaction": tx}, nil
}

// HTTPGetDeadletters handles GET /deadletters - list dead-lettered withdrawals.
func (s *Service) HTTPGetDeadletters(ctx context.Context, req core.APIRequest) (any, error) {
	limit := core.ParseLimitFromQuery(req.Query)
	return s.ListDeadLetters(ctx, req.AccountID, limit)
}

// HTTPPostDeadlettersIdRetry handles POST /deadletters/{id}/retry - retry dead letter.
func (s *Service) HTTPPostDeadlettersIdRetry(ctx context.Context, req core.APIRequest) (any, error) {
	return s.RetryDeadLetter(ctx, req.AccountID, req.PathParams["id"])
}

// HTTPDeleteDeadlettersById handles DELETE /deadletters/{id} - delete dead letter.
func (s *Service) HTTPDeleteDeadlettersById(ctx context.Context, req core.APIRequest) (any, error) {
	err := s.DeleteDeadLetter(ctx, req.AccountID, req.PathParams["id"])
	if err != nil {
		return nil, err
	}
	return map[string]string{"status": "deleted"}, nil
}
