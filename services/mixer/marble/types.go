// Package mixer provides types for the privacy mixer service.
package mixermarble

import (
	"encoding/json"
	"time"

	mixersupabase "github.com/R3E-Network/service_layer/services/mixer/supabase"
)

// MixRequestStatus represents the status of a mix request.
type MixRequestStatus string

const (
	StatusPending   MixRequestStatus = "pending"
	StatusDeposited MixRequestStatus = "deposited"
	StatusMixing    MixRequestStatus = "mixing"
	StatusDelivered MixRequestStatus = "delivered"
	StatusFailed    MixRequestStatus = "failed"
	StatusRefunded  MixRequestStatus = "refunded"
)

// PoolAccount represents an account in the mixing pool.
// Pool accounts are managed by accountpool service.
type PoolAccount struct {
	ID         string    `json:"id"`
	Address    string    `json:"address"`
	Balance    int64     `json:"balance"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	TxCount    int64     `json:"tx_count"`
	IsRetiring bool      `json:"is_retiring"`
}

// MixRequest represents a user's mix request with TEE commitment.
type MixRequest struct {
	ID              string           `json:"id"`
	UserID          string           `json:"user_id"`
	UserAddress     string           `json:"user_address"`
	TokenType       string           `json:"token_type"` // GAS, NEO, etc.
	Status          MixRequestStatus `json:"status"`
	TotalAmount     int64            `json:"total_amount"`
	ServiceFee      int64            `json:"service_fee"`
	NetAmount       int64            `json:"net_amount"`
	TargetAddresses []TargetAddress  `json:"target_addresses"`
	InitialSplits   int              `json:"initial_splits"`
	MixingDuration  time.Duration    `json:"mixing_duration"`
	DepositAddress  string           `json:"deposit_address"`
	DepositTxHash   string           `json:"deposit_tx_hash,omitempty"`
	PoolAccounts    []string         `json:"pool_accounts"`
	// TEE Commitment fields for dispute mechanism
	RequestHash     string           `json:"request_hash"`  // Hash256(canonical request bytes)
	TEESignature    string           `json:"tee_signature"` // TEE signature over requestHash
	Deadline        int64            `json:"deadline"`      // Unix timestamp for dispute deadline
	OutputTxIDs     []string         `json:"output_tx_ids,omitempty"`
	CompletionProof *CompletionProof `json:"completion_proof,omitempty"`
	// Timestamps
	CreatedAt     time.Time `json:"created_at"`
	DepositedAt   time.Time `json:"deposited_at,omitempty"`
	MixingStartAt time.Time `json:"mixing_start_at,omitempty"`
	DeliveredAt   time.Time `json:"delivered_at,omitempty"`
	Error         string    `json:"error,omitempty"`
}

// TargetAddress represents a target address for token delivery.
type TargetAddress struct {
	Address string `json:"address"`
	Amount  int64  `json:"amount,omitempty"` // 0 means split equally
}

// CompletionProof is generated when mixing is done (for dispute resolution).
// This proof is stored but NOT submitted on-chain unless user disputes.
type CompletionProof struct {
	RequestID    string   `json:"request_id"`
	RequestHash  string   `json:"request_hash"`
	OutputsHash  string   `json:"outputs_hash"`  // Hash256(sorted output tx IDs)
	OutputTxIDs  []string `json:"output_tx_ids"` // Actual output transactions
	CompletedAt  int64    `json:"completed_at"`  // Unix timestamp
	TEESignature string   `json:"tee_signature"` // TEE signature over completion data
}

// CreateRequestInput for new mix request (matches documented JSON format)
type CreateRequestInput struct {
	Version     int             `json:"version"`
	TokenType   string          `json:"token_type,omitempty"` // GAS, NEO, etc. (default: GAS)
	UserAddress string          `json:"user_address"`
	InputTxs    []string        `json:"input_txs,omitempty"` // Optional input tx hashes
	Targets     []TargetAddress `json:"targets"`
	MixOption   int64           `json:"mix_option"` // Duration in milliseconds
	Timestamp   int64           `json:"timestamp"`
	// Legacy fields for backward compatibility
	TotalAmount   int64 `json:"total_amount,omitempty"`
	InitialSplits int   `json:"initial_splits,omitempty"`
	MixingMinutes int   `json:"mixing_minutes,omitempty"`
}

// CreateRequestResponse with TEE commitment for dispute mechanism
type CreateRequestResponse struct {
	Request        *CreateRequestInput `json:"request"`
	RequestID      string              `json:"request_id"`
	RequestHash    string              `json:"request_hash"`  // Hash256(canonical request bytes)
	TEESignature   string              `json:"tee_signature"` // TEE signature for dispute proof
	DepositAddress string              `json:"deposit_address"`
	TotalAmount    int64               `json:"total_amount"`
	ServiceFee     int64               `json:"service_fee"`
	NetAmount      int64               `json:"net_amount"`
	Deadline       int64               `json:"deadline"` // Unix timestamp
	ExpiresAt      string              `json:"expires_at"`
}

// ConfirmDepositInput for deposit confirmation.
type ConfirmDepositInput struct {
	TxHash string `json:"tx_hash"`
}

// DisputeInput for user dispute request.
type DisputeInput struct {
	Reason string `json:"reason"` // Optional reason for dispute
}

// DisputeResponse returned after dispute is submitted on-chain.
type DisputeResponse struct {
	RequestID       string           `json:"request_id"`
	Status          string           `json:"status"`
	CompletionProof *CompletionProof `json:"completion_proof,omitempty"`
	OnChainTxHash   string           `json:"on_chain_tx_hash,omitempty"`
	Message         string           `json:"message"`
}

// =============================================================================
// Database Conversion Functions
// =============================================================================

// RequestFromRecord converts a database record to a MixRequest.
func RequestFromRecord(rec *mixersupabase.RequestRecord) *MixRequest {
	var completionProof *CompletionProof
	if rec.CompletionProofJSON != "" {
		completionProof = &CompletionProof{}
		_ = json.Unmarshal([]byte(rec.CompletionProofJSON), completionProof)
	}
	return &MixRequest{
		ID:              rec.ID,
		UserID:          rec.UserID,
		UserAddress:     rec.UserAddress,
		TokenType:       rec.TokenType,
		Status:          MixRequestStatus(rec.Status),
		TotalAmount:     rec.TotalAmount,
		ServiceFee:      rec.ServiceFee,
		NetAmount:       rec.NetAmount,
		TargetAddresses: convertTargetsFromDB(rec.TargetAddresses),
		InitialSplits:   rec.InitialSplits,
		MixingDuration:  time.Duration(rec.MixingDurationSeconds) * time.Second,
		DepositAddress:  rec.DepositAddress,
		DepositTxHash:   rec.DepositTxHash,
		PoolAccounts:    rec.PoolAccounts,
		RequestHash:     rec.RequestHash,
		TEESignature:    rec.TEESignature,
		Deadline:        rec.Deadline,
		OutputTxIDs:     rec.OutputTxIDs,
		CompletionProof: completionProof,
		CreatedAt:       rec.CreatedAt,
		DepositedAt:     rec.DepositedAt,
		MixingStartAt:   rec.MixingStartAt,
		DeliveredAt:     rec.DeliveredAt,
		Error:           rec.Error,
	}
}

// RequestToRecord converts a MixRequest to a database record.
func RequestToRecord(req *MixRequest) *mixersupabase.RequestRecord {
	var completionProofJSON string
	if req.CompletionProof != nil {
		if data, err := json.Marshal(req.CompletionProof); err == nil {
			completionProofJSON = string(data)
		}
	}
	return &mixersupabase.RequestRecord{
		ID:                    req.ID,
		UserID:                req.UserID,
		UserAddress:           req.UserAddress,
		TokenType:             req.TokenType,
		Status:                string(req.Status),
		TotalAmount:           req.TotalAmount,
		ServiceFee:            req.ServiceFee,
		NetAmount:             req.NetAmount,
		TargetAddresses:       convertTargetsToDB(req.TargetAddresses),
		InitialSplits:         req.InitialSplits,
		MixingDurationSeconds: int64(req.MixingDuration.Seconds()),
		DepositAddress:        req.DepositAddress,
		DepositTxHash:         req.DepositTxHash,
		PoolAccounts:          req.PoolAccounts,
		RequestHash:           req.RequestHash,
		TEESignature:          req.TEESignature,
		Deadline:              req.Deadline,
		OutputTxIDs:           req.OutputTxIDs,
		CompletionProofJSON:   completionProofJSON,
		CreatedAt:             req.CreatedAt,
		DepositedAt:           req.DepositedAt,
		MixingStartAt:         req.MixingStartAt,
		DeliveredAt:           req.DeliveredAt,
		Error:                 req.Error,
	}
}

// convertTargetsFromDB converts database target addresses to domain type.
func convertTargetsFromDB(items []mixersupabase.TargetAddress) []TargetAddress {
	out := make([]TargetAddress, 0, len(items))
	for _, t := range items {
		out = append(out, TargetAddress{Address: t.Address, Amount: t.Amount})
	}
	return out
}

func convertTargetsToDB(items []TargetAddress) []mixersupabase.TargetAddress {
	out := make([]mixersupabase.TargetAddress, 0, len(items))
	for _, t := range items {
		out = append(out, mixersupabase.TargetAddress{Address: t.Address, Amount: t.Amount})
	}
	return out
}
