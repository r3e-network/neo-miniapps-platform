// Package gasaccounting provides GAS ledger and accounting service.
package gasaccounting

import (
	"time"
)

// =============================================================================
// Service Constants
// =============================================================================

const (
	ServiceID   = "gasaccounting"
	ServiceName = "GasAccounting Service"
	Version     = "1.0.0"
)

// =============================================================================
// Ledger Entry Types
// =============================================================================

// EntryType represents the type of ledger entry.
type EntryType string

const (
	EntryTypeDeposit    EntryType = "deposit"    // User deposits GAS
	EntryTypeWithdraw   EntryType = "withdraw"   // User withdraws GAS
	EntryTypeConsume    EntryType = "consume"    // Service consumes GAS for tx
	EntryTypeRefund     EntryType = "refund"     // Refund unused GAS
	EntryTypeReserve    EntryType = "reserve"    // Reserve GAS for pending tx
	EntryTypeRelease    EntryType = "release"    // Release reserved GAS
	EntryTypeFee        EntryType = "fee"        // Service fee deduction
	EntryTypeAdjustment EntryType = "adjustment" // Manual adjustment
)

// =============================================================================
// Ledger Entry
// =============================================================================

// LedgerEntry represents an immutable ledger entry.
type LedgerEntry struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	EntryType      EntryType `json:"entry_type"`
	Amount         int64     `json:"amount"`         // Positive for credit, negative for debit
	BalanceAfter   int64     `json:"balance_after"`  // Balance after this entry
	ReferenceID    string    `json:"reference_id"`   // External reference (tx hash, request ID)
	ReferenceType  string    `json:"reference_type"` // Type of reference (tx, request, etc.)
	ServiceID      string    `json:"service_id"`     // Which service triggered this
	Description    string    `json:"description"`    // Human-readable description
	Metadata       string    `json:"metadata"`       // JSON metadata
	CreatedAt      time.Time `json:"created_at"`
	IdempotencyKey string    `json:"idempotency_key"` // For deduplication
}

// =============================================================================
// Account Balance
// =============================================================================

// AccountBalance represents a user's GAS balance.
type AccountBalance struct {
	UserID           int64     `json:"user_id"`
	AvailableBalance int64     `json:"available_balance"` // Can be used
	ReservedBalance  int64     `json:"reserved_balance"`  // Reserved for pending txs
	TotalBalance     int64     `json:"total_balance"`     // Available + Reserved
	LastUpdated      time.Time `json:"last_updated"`
}

// =============================================================================
// API Types
// =============================================================================

// DepositRequest is a request to deposit GAS.
type DepositRequest struct {
	UserID    int64  `json:"user_id"`
	Amount    int64  `json:"amount"`
	TxHash    string `json:"tx_hash"`   // On-chain deposit tx
	Reference string `json:"reference"` // Optional reference
}

// DepositResponse is the response from a deposit.
type DepositResponse struct {
	EntryID     int64     `json:"entry_id"`
	NewBalance  int64     `json:"new_balance"`
	DepositedAt time.Time `json:"deposited_at"`
}

// WithdrawRequest is a request to withdraw GAS.
type WithdrawRequest struct {
	UserID    int64  `json:"user_id"`
	Amount    int64  `json:"amount"`
	ToAddress string `json:"to_address"` // Destination address
}

// WithdrawResponse is the response from a withdrawal.
type WithdrawResponse struct {
	EntryID     int64     `json:"entry_id"`
	TxHash      string    `json:"tx_hash"`
	NewBalance  int64     `json:"new_balance"`
	WithdrawnAt time.Time `json:"withdrawn_at"`
}

// ConsumeRequest is a request to consume GAS for a service operation.
type ConsumeRequest struct {
	UserID      int64  `json:"user_id"`
	Amount      int64  `json:"amount"`
	ServiceID   string `json:"service_id"`
	RequestID   string `json:"request_id"`
	Description string `json:"description"`
}

// ConsumeResponse is the response from consuming GAS.
type ConsumeResponse struct {
	EntryID    int64     `json:"entry_id"`
	NewBalance int64     `json:"new_balance"`
	ConsumedAt time.Time `json:"consumed_at"`
}

// ReserveRequest is a request to reserve GAS for a pending operation.
type ReserveRequest struct {
	UserID    int64         `json:"user_id"`
	Amount    int64         `json:"amount"`
	ServiceID string        `json:"service_id"`
	RequestID string        `json:"request_id"`
	TTL       time.Duration `json:"ttl"` // How long to hold reservation
}

// ReserveResponse is the response from reserving GAS.
type ReserveResponse struct {
	ReservationID string    `json:"reservation_id"`
	Amount        int64     `json:"amount"`
	ExpiresAt     time.Time `json:"expires_at"`
	NewAvailable  int64     `json:"new_available"`
}

// ReleaseRequest is a request to release a reservation.
type ReleaseRequest struct {
	ReservationID string `json:"reservation_id"`
	Consume       bool   `json:"consume"`       // If true, consume instead of release
	ActualAmount  int64  `json:"actual_amount"` // Actual amount consumed (if different)
}

// ReleaseResponse is the response from releasing a reservation.
type ReleaseResponse struct {
	EntryID      int64 `json:"entry_id"`
	Released     int64 `json:"released"`
	Consumed     int64 `json:"consumed"`
	NewAvailable int64 `json:"new_available"`
}

// BalanceResponse is the response for balance queries.
type BalanceResponse struct {
	UserID           int64     `json:"user_id"`
	AvailableBalance int64     `json:"available_balance"`
	ReservedBalance  int64     `json:"reserved_balance"`
	TotalBalance     int64     `json:"total_balance"`
	AsOf             time.Time `json:"as_of"`
}

// LedgerHistoryRequest is a request for ledger history.
type LedgerHistoryRequest struct {
	UserID    int64      `json:"user_id"`
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	EntryType *EntryType `json:"entry_type,omitempty"`
	Limit     int        `json:"limit"`
	Offset    int        `json:"offset"`
}

// LedgerHistoryResponse is the response for ledger history.
type LedgerHistoryResponse struct {
	Entries    []*LedgerEntry `json:"entries"`
	TotalCount int            `json:"total_count"`
	HasMore    bool           `json:"has_more"`
}

// =============================================================================
// Service Status
// =============================================================================

// ServiceStatus represents the service status.
type ServiceStatus struct {
	Service       string `json:"service"`
	Version       string `json:"version"`
	Healthy       bool   `json:"healthy"`
	TotalUsers    int64  `json:"total_users"`
	TotalBalance  int64  `json:"total_balance"`
	TotalReserved int64  `json:"total_reserved"`
	EntriesCount  int64  `json:"entries_count"`
	Uptime        string `json:"uptime"`
}
