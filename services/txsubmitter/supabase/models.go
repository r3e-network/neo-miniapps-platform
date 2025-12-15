// Package supabase provides Supabase repository for TxSubmitter.
package supabase

import (
	"encoding/json"
	"time"
)

// =============================================================================
// Chain Transaction Record
// =============================================================================

// ChainTxStatus represents the status of a chain transaction.
type ChainTxStatus string

const (
	// StatusPending indicates the transaction is queued but not yet submitted.
	StatusPending ChainTxStatus = "pending"

	// StatusSubmitted indicates the transaction has been broadcast to the network.
	StatusSubmitted ChainTxStatus = "submitted"

	// StatusConfirmed indicates the transaction has been confirmed on-chain.
	StatusConfirmed ChainTxStatus = "confirmed"

	// StatusFailed indicates the transaction failed (VM fault or rejected).
	StatusFailed ChainTxStatus = "failed"

	// StatusTimeout indicates the transaction timed out waiting for confirmation.
	StatusTimeout ChainTxStatus = "timeout"
)

// ChainTxRecord represents a record in the chain_txs table.
type ChainTxRecord struct {
	ID              int64           `json:"id"`
	TxHash          string          `json:"tx_hash,omitempty"`
	RequestID       string          `json:"request_id"`
	FromService     string          `json:"from_service"`
	TxType          string          `json:"tx_type"`
	ContractAddress string          `json:"contract_address"`
	MethodName      string          `json:"method_name"`
	Params          json.RawMessage `json:"params"`
	GasConsumed     int64           `json:"gas_consumed,omitempty"`
	Status          ChainTxStatus   `json:"status"`
	RetryCount      int             `json:"retry_count"`
	ErrorMessage    string          `json:"error_message,omitempty"`
	RPCEndpoint     string          `json:"rpc_endpoint,omitempty"`
	SubmittedAt     time.Time       `json:"submitted_at"`
	ConfirmedAt     *time.Time      `json:"confirmed_at,omitempty"`
}

// =============================================================================
// Request/Response Types
// =============================================================================

// CreateTxRequest represents a request to create a new transaction record.
type CreateTxRequest struct {
	RequestID       string          `json:"request_id"`
	FromService     string          `json:"from_service"`
	TxType          string          `json:"tx_type"`
	ContractAddress string          `json:"contract_address"`
	MethodName      string          `json:"method_name"`
	Params          json.RawMessage `json:"params"`
}

// UpdateTxStatusRequest represents a request to update transaction status.
type UpdateTxStatusRequest struct {
	ID           int64         `json:"id"`
	TxHash       string        `json:"tx_hash,omitempty"`
	Status       ChainTxStatus `json:"status"`
	RetryCount   int           `json:"retry_count,omitempty"`
	ErrorMessage string        `json:"error_message,omitempty"`
	RPCEndpoint  string        `json:"rpc_endpoint,omitempty"`
	GasConsumed  int64         `json:"gas_consumed,omitempty"`
	ConfirmedAt  *time.Time    `json:"confirmed_at,omitempty"`
}

// =============================================================================
// Transaction Types (for typed API)
// =============================================================================

// TxType represents the type of transaction.
type TxType string

const (
	TxTypeFulfillRequest  TxType = "fulfill_request"
	TxTypeFailRequest     TxType = "fail_request"
	TxTypeUpdatePrice     TxType = "update_price"
	TxTypeUpdatePrices    TxType = "update_prices"
	TxTypeExecuteTrigger  TxType = "execute_trigger"
	TxTypeSetTEEMasterKey TxType = "set_tee_master_key"
	TxTypeResolveDispute  TxType = "resolve_dispute"
	TxTypeGeneric         TxType = "generic"
)

// IsValidTxType checks if a transaction type is valid.
func IsValidTxType(t string) bool {
	switch TxType(t) {
	case TxTypeFulfillRequest, TxTypeFailRequest, TxTypeUpdatePrice,
		TxTypeUpdatePrices, TxTypeExecuteTrigger, TxTypeSetTEEMasterKey,
		TxTypeResolveDispute, TxTypeGeneric:
		return true
	default:
		return false
	}
}
