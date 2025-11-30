package gasbank

import (
	"context"
	"time"

	"github.com/R3E-Network/service_layer/system/framework"
)

// Store defines the data access interface for the Gas Bank service.
type Store interface {
	// Gas Accounts
	CreateGasAccount(ctx context.Context, acct GasBankAccount) (GasBankAccount, error)
	UpdateGasAccount(ctx context.Context, acct GasBankAccount) (GasBankAccount, error)
	GetGasAccount(ctx context.Context, id string) (GasBankAccount, error)
	GetGasAccountByWallet(ctx context.Context, wallet string) (GasBankAccount, error)
	ListGasAccounts(ctx context.Context, accountID string) ([]GasBankAccount, error)

	// Gas Transactions
	CreateGasTransaction(ctx context.Context, tx Transaction) (Transaction, error)
	UpdateGasTransaction(ctx context.Context, tx Transaction) (Transaction, error)
	GetGasTransaction(ctx context.Context, id string) (Transaction, error)
	ListGasTransactions(ctx context.Context, gasAccountID string, limit int) ([]Transaction, error)
	ListPendingWithdrawals(ctx context.Context) ([]Transaction, error)

	// Withdrawal Approvals
	UpsertWithdrawalApproval(ctx context.Context, approval WithdrawalApproval) (WithdrawalApproval, error)
	ListWithdrawalApprovals(ctx context.Context, transactionID string) ([]WithdrawalApproval, error)

	// Withdrawal Schedules
	SaveWithdrawalSchedule(ctx context.Context, schedule WithdrawalSchedule) (WithdrawalSchedule, error)
	GetWithdrawalSchedule(ctx context.Context, transactionID string) (WithdrawalSchedule, error)
	DeleteWithdrawalSchedule(ctx context.Context, transactionID string) error
	ListDueWithdrawalSchedules(ctx context.Context, before time.Time, limit int) ([]WithdrawalSchedule, error)

	// Settlement Attempts
	RecordSettlementAttempt(ctx context.Context, attempt SettlementAttempt) (SettlementAttempt, error)
	ListSettlementAttempts(ctx context.Context, transactionID string, limit int) ([]SettlementAttempt, error)

	// Dead Letters
	UpsertDeadLetter(ctx context.Context, entry DeadLetter) (DeadLetter, error)
	GetDeadLetter(ctx context.Context, transactionID string) (DeadLetter, error)
	ListDeadLetters(ctx context.Context, accountID string, limit int) ([]DeadLetter, error)
	RemoveDeadLetter(ctx context.Context, transactionID string) error
}

// AccountChecker is an alias for the canonical framework.AccountChecker interface.
// Use framework.AccountChecker directly in new code.
type AccountChecker = framework.AccountChecker
