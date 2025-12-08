// Package supabase provides Mixer-specific database operations.
package supabase

import "time"

// TargetAddress represents a delivery target.
type TargetAddress struct {
	Address string `json:"address"`
	Amount  int64  `json:"amount,omitempty"`
}

// RequestRecord represents a mixer request row.
type RequestRecord struct {
	ID                    string          `json:"id"`
	UserID                string          `json:"user_id"`
	UserAddress           string          `json:"user_address,omitempty"`
	TokenType             string          `json:"token_type"` // GAS, NEO, etc.
	Status                string          `json:"status"`
	TotalAmount           int64           `json:"total_amount"`
	ServiceFee            int64           `json:"service_fee"`
	NetAmount             int64           `json:"net_amount"`
	TargetAddresses       []TargetAddress `json:"target_addresses"`
	InitialSplits         int             `json:"initial_splits"`
	MixingDurationSeconds int64           `json:"mixing_duration_seconds"`
	DepositAddress        string          `json:"deposit_address"`
	DepositTxHash         string          `json:"deposit_tx_hash,omitempty"`
	PoolAccounts          []string        `json:"pool_accounts"`
	// TEE Commitment fields for dispute mechanism
	RequestHash  string   `json:"request_hash,omitempty"`
	TEESignature string   `json:"tee_signature,omitempty"`
	Deadline     int64    `json:"deadline,omitempty"`
	OutputTxIDs  []string `json:"output_tx_ids,omitempty"`
	// CompletionProof is generated when mixing is done (stored as JSON, NOT submitted unless disputed)
	CompletionProofJSON string `json:"completion_proof_json,omitempty"`
	// Timestamps
	CreatedAt     time.Time `json:"created_at"`
	DepositedAt   time.Time `json:"deposited_at,omitempty"`
	MixingStartAt time.Time `json:"mixing_start_at,omitempty"`
	DeliveredAt   time.Time `json:"delivered_at,omitempty"`
	Error         string    `json:"error,omitempty"`
}
