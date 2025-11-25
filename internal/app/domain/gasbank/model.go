package gasbank

import (
	"time"
)

// AccountStatus represents the lifecycle state of a gas bank account/wallet.
// Aligned with AccountManager.cs contract Wallet.Status byte.
type AccountStatus string

const (
	AccountStatusActive  AccountStatus = "active"  // Contract: 0
	AccountStatusRevoked AccountStatus = "revoked" // Contract: 1
)

// Account represents a gas bank wallet owned by an application account.
// Aligned with AccountManager.cs contract Wallet struct.
type Account struct {
	ID                    string
	AccountID             string
	WalletAddress         string        // Maps to contract Wallet.Address (UInt160)
	Status                AccountStatus // Maps to contract Wallet.Status byte
	Balance               float64
	Available             float64
	Pending               float64
	Locked                float64
	MinBalance            float64
	DailyLimit            float64
	DailyWithdrawal       float64
	NotificationThreshold float64
	RequiredApprovals     int
	Flags                 map[string]bool
	Metadata              map[string]string
	LastWithdrawal        time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// Transaction captures deposits, withdrawals, and internal adjustments.
type Transaction struct {
	ID               string
	AccountID        string
	UserAccountID    string
	Type             string
	Status           string
	Amount           float64
	NetAmount        float64
	BlockchainTxID   string
	FromAddress      string
	ToAddress        string
	ScheduleAt       time.Time
	CronExpression   string
	ApprovalPolicy   ApprovalPolicy
	Approvals        []WithdrawalApproval
	ResolverAttempt  int
	ResolverError    string
	LastAttemptAt    time.Time
	NextAttemptAt    time.Time
	DeadLetterReason string
	Notes            string
	Error            string
	Metadata         map[string]string
	DispatchedAt     time.Time
	ResolvedAt       time.Time
	CompletedAt      time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

const (
	TransactionDeposit    = "deposit"
	TransactionWithdrawal = "withdrawal"
)

const (
	StatusPending          = "pending"
	StatusScheduled        = "scheduled"
	StatusAwaitingApproval = "awaiting_approval"
	StatusApproved         = "approved"
	StatusDispatched       = "dispatched"
	StatusCompleted        = "completed"
	StatusFailed           = "failed"
	StatusCancelled        = "cancelled"
	StatusDeadLetter       = "dead_letter"
)

// ApprovalPolicy describes how many approvals are required for a withdrawal and the allowed approvers set.
type ApprovalPolicy struct {
	Required  int
	Approvers []string
}

// WithdrawalApproval captures an approval decision for a withdrawal transaction.
type WithdrawalApproval struct {
	TransactionID string
	Approver      string
	Status        string
	Signature     string
	Note          string
	DecidedAt     time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

const (
	ApprovalPending  = "pending"
	ApprovalApproved = "approved"
	ApprovalRejected = "rejected"
)

// WithdrawalSchedule tracks deferred withdrawals (schedule-at timestamp or cron expression).
type WithdrawalSchedule struct {
	TransactionID  string
	ScheduleAt     time.Time
	CronExpression string
	NextRunAt      time.Time
	LastRunAt      time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// SettlementAttempt records resolver attempts for observability/debugging.
type SettlementAttempt struct {
	TransactionID string
	Attempt       int
	StartedAt     time.Time
	CompletedAt   time.Time
	Latency       time.Duration
	Status        string
	Error         string
}

// DeadLetter captures withdrawals that exceeded retry budgets and require manual intervention.
type DeadLetter struct {
	TransactionID string
	AccountID     string
	Reason        string
	LastError     string
	LastAttemptAt time.Time
	Retries       int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
