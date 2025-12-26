package supabase

import (
	"encoding/json"
	"time"
)

// MiniApp represents a minimal MiniApp registry row needed for on-chain requests.
type MiniApp struct {
	AppID           string          `json:"app_id"`
	DeveloperUserID string          `json:"developer_user_id"`
	Manifest        json.RawMessage `json:"manifest"`
	Status          string          `json:"status"`
	ManifestHash    string          `json:"manifest_hash"`
	EntryURL        string          `json:"entry_url"`
}

// ServiceRequest represents a service_requests row for audit tracking.
type ServiceRequest struct {
	ID          string          `json:"id,omitempty"`
	UserID      string          `json:"user_id"`
	ServiceType string          `json:"service_type"`
	Status      string          `json:"status"`
	Payload     json.RawMessage `json:"payload"`
	Result      json.RawMessage `json:"result,omitempty"`
	Error       string          `json:"error,omitempty"`
	GasUsed     int64           `json:"gas_used,omitempty"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	ChainTxID   *int64          `json:"chain_tx_id,omitempty"`
	RetryCount  int             `json:"retry_count,omitempty"`
	LastError   string          `json:"last_error,omitempty"`
	Signature   []byte          `json:"signature,omitempty"`
	SignerKeyID string          `json:"signer_key_id,omitempty"`
}

// ChainTx represents a chain_txs row for callback auditing.
type ChainTx struct {
	ID              int64           `json:"id,omitempty"`
	TxHash          string          `json:"tx_hash,omitempty"`
	RequestID       string          `json:"request_id"`
	FromService     string          `json:"from_service"`
	TxType          string          `json:"tx_type"`
	ContractAddress string          `json:"contract_address"`
	MethodName      string          `json:"method_name"`
	Params          json.RawMessage `json:"params"`
	GasConsumed     *int64          `json:"gas_consumed,omitempty"`
	Status          string          `json:"status,omitempty"`
	RetryCount      int             `json:"retry_count,omitempty"`
	ErrorMessage    string          `json:"error_message,omitempty"`
	RPCEndpoint     string          `json:"rpc_endpoint,omitempty"`
	SubmittedAt     *time.Time      `json:"submitted_at,omitempty"`
	ConfirmedAt     *time.Time      `json:"confirmed_at,omitempty"`
}

// ContractEvent represents a contract_events row.
type ContractEvent struct {
	ID           int64           `json:"id,omitempty"`
	TxHash       string          `json:"tx_hash"`
	BlockIndex   uint64          `json:"block_index"`
	ContractHash string          `json:"contract_hash"`
	EventName    string          `json:"event_name"`
	AppID        *string         `json:"app_id,omitempty"`
	State        json.RawMessage `json:"state,omitempty"`
	CreatedAt    *time.Time      `json:"created_at,omitempty"`
}

// ProcessedEvent represents a processed_events row for idempotency.
type ProcessedEvent struct {
	ID              int64           `json:"id,omitempty"`
	ChainID         string          `json:"chain_id"`
	TxHash          string          `json:"tx_hash"`
	LogIndex        int             `json:"log_index"`
	BlockHeight     uint64          `json:"block_height"`
	BlockHash       string          `json:"block_hash"`
	ContractAddress string          `json:"contract_address"`
	EventName       string          `json:"event_name"`
	Payload         json.RawMessage `json:"payload"`
	Confirmations   int             `json:"confirmations,omitempty"`
}

// Notification represents a miniapp_notifications row.
type Notification struct {
	ID               string `json:"id,omitempty"`
	AppID            string `json:"app_id"`
	Title            string `json:"title"`
	Content          string `json:"content"`
	NotificationType string `json:"notification_type"`
	Source           string `json:"source"`
	TxHash           string `json:"tx_hash,omitempty"`
	BlockNumber      int64  `json:"block_number,omitempty"`
	Priority         int    `json:"priority"`
}
