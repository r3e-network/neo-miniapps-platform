package gasbank

import appgas "github.com/R3E-Network/service_layer/internal/app/domain/gasbank"

type (
	Account            = appgas.Account
	Transaction        = appgas.Transaction
	WithdrawalApproval = appgas.WithdrawalApproval
	WithdrawalSchedule = appgas.WithdrawalSchedule
	SettlementAttempt  = appgas.SettlementAttempt
	ApprovalPolicy     = appgas.ApprovalPolicy
	DeadLetter         = appgas.DeadLetter
)

const (
	TransactionDeposit    = appgas.TransactionDeposit
	TransactionWithdrawal = appgas.TransactionWithdrawal

	StatusPending          = appgas.StatusPending
	StatusScheduled        = appgas.StatusScheduled
	StatusAwaitingApproval = appgas.StatusAwaitingApproval
	StatusApproved         = appgas.StatusApproved
	StatusDispatched       = appgas.StatusDispatched
	StatusCompleted        = appgas.StatusCompleted
	StatusFailed           = appgas.StatusFailed
	StatusCancelled        = appgas.StatusCancelled
	StatusDeadLetter       = appgas.StatusDeadLetter
)

const (
	ApprovalPending  = appgas.ApprovalPending
	ApprovalApproved = appgas.ApprovalApproved
	ApprovalRejected = appgas.ApprovalRejected
)
