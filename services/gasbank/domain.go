// Package gasbank provides gas fee management service.
package gasbank

import (
	"time"

	"github.com/R3E-Network/service_layer/services/base"
)

// AccountStatus represents the status of a gas account.
type AccountStatus string

const (
	AccountStatusActive   AccountStatus = "active"
	AccountStatusSuspended AccountStatus = "suspended"
	AccountStatusClosed   AccountStatus = "closed"
)

// GasAccount represents a gas fee account.
type GasAccount struct {
	base.BaseEntity

	AccountID      string        `json:"account_id"`
	Balance        string        `json:"balance"`
	Status         AccountStatus `json:"status"`
	TotalDeposited string        `json:"total_deposited"`
	TotalSpent     string        `json:"total_spent"`
	TotalRefunded  string        `json:"total_refunded"`
}

// TxType represents the type of gas transaction.
type TxType string

const (
	TxTypeDeposit    TxType = "deposit"
	TxTypeSponsorship TxType = "sponsorship"
	TxTypeRefund     TxType = "refund"
	TxTypeWithdraw   TxType = "withdraw"
)

// TxStatus represents the status of a gas transaction.
type TxStatus string

const (
	TxStatusPending   TxStatus = "pending"
	TxStatusConfirmed TxStatus = "confirmed"
	TxStatusFailed    TxStatus = "failed"
)

// GasTransaction represents a gas fee transaction.
type GasTransaction struct {
	base.BaseEntity

	AccountID   string   `json:"account_id"`
	Type        TxType   `json:"type"`
	Amount      string   `json:"amount"`
	TxHash      string   `json:"tx_hash"`
	Status      TxStatus `json:"status"`
	TargetTx    string   `json:"target_tx,omitempty"`
	Error       string   `json:"error,omitempty"`
	ConfirmedAt time.Time `json:"confirmed_at,omitempty"`
}

// GasStats provides service statistics.
type GasStats struct {
	TotalAccounts     int64     `json:"total_accounts"`
	ActiveAccounts    int64     `json:"active_accounts"`
	TotalTransactions int64     `json:"total_transactions"`
	TotalDeposited    string    `json:"total_deposited"`
	TotalSpent        string    `json:"total_spent"`
	GeneratedAt       time.Time `json:"generated_at"`
}

// DepositRequest represents a deposit request.
type DepositRequest struct {
	AccountID string `json:"account_id"`
	Amount    string `json:"amount"`
	TxHash    string `json:"tx_hash"`
}

// SponsorRequest represents a gas sponsorship request.
type SponsorRequest struct {
	AccountID string `json:"account_id"`
	GasAmount string `json:"gas_amount"`
	TargetTx  []byte `json:"target_tx"`
}
