package gasbank

import (
	"time"
)

// Account represents a gas bank wallet owned by an application account.
type Account struct {
	ID              string
	AccountID       string
	WalletAddress   string
	Balance         float64
	Available       float64
	Pending         float64
	DailyWithdrawal float64
	LastWithdrawal  time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Transaction captures deposits, withdrawals, and internal adjustments.
type Transaction struct {
	ID             string
	AccountID      string
	UserAccountID  string
	Type           string
	Amount         float64
	NetAmount      float64
	Status         string
	BlockchainTxID string
	FromAddress    string
	ToAddress      string
	Notes          string
	Error          string
	CompletedAt    time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

const (
	TransactionDeposit    = "deposit"
	TransactionWithdrawal = "withdrawal"
)

const (
	StatusPending   = "pending"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)
