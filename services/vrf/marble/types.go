// Package vrfmarble provides the Verifiable Random Function service.
package vrfmarble

import (
	"time"

	vrfsupabase "github.com/R3E-Network/service_layer/services/vrf/supabase"
)

// =============================================================================
// Constants
// =============================================================================

const (
	ServiceID   = "vrf"
	ServiceName = "VRF Service"
	Version     = "2.0.0"

	// Request status
	StatusPending   = "pending"
	StatusFulfilled = "fulfilled"
	StatusFailed    = "failed"

	// Polling interval for chain events
	EventPollInterval = 5 * time.Second

	// Service fee per request (in GAS smallest unit)
	ServiceFeePerRequest = 100000 // 0.001 GAS
)

// =============================================================================
// Request Types
// =============================================================================

// VRFRequest represents a randomness request from a user contract.
type VRFRequest struct {
	ID               string    `json:"id"`
	RequestID        string    `json:"request_id"`        // On-chain request ID
	UserID           string    `json:"user_id"`           // Service Layer user
	RequesterAddress string    `json:"requester_address"` // User contract address
	Seed             string    `json:"seed"`
	NumWords         int       `json:"num_words"`
	CallbackGasLimit int64     `json:"callback_gas_limit"`
	Status           string    `json:"status"`
	RandomWords      []string  `json:"random_words,omitempty"`
	Proof            string    `json:"proof,omitempty"`
	FulfillTxHash    string    `json:"fulfill_tx_hash,omitempty"`
	Error            string    `json:"error,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	FulfilledAt      time.Time `json:"fulfilled_at,omitempty"`
}

// CreateRequestInput for API-initiated requests (off-chain trigger).
type CreateRequestInput struct {
	Seed             string `json:"seed"`
	NumWords         int    `json:"num_words"`
	CallbackContract string `json:"callback_contract"`
	CallbackGasLimit int64  `json:"callback_gas_limit"`
}

// =============================================================================
// API Types
// =============================================================================

// DirectRandomRequest for direct API usage.
type DirectRandomRequest struct {
	Seed     string `json:"seed"`
	NumWords int    `json:"num_words,omitempty"`
}

// DirectRandomResponse for direct API usage.
type DirectRandomResponse struct {
	RequestID   string   `json:"request_id"`
	Seed        string   `json:"seed"`
	RandomWords []string `json:"random_words"`
	Proof       string   `json:"proof"`
	PublicKey   string   `json:"public_key"`
	Timestamp   string   `json:"timestamp"`
}

// VerifyRequest represents a VRF verification request.
type VerifyRequest struct {
	Seed        string   `json:"seed"`
	RandomWords []string `json:"random_words"`
	Proof       string   `json:"proof"`
	PublicKey   string   `json:"public_key"`
}

// VerifyResponse represents a VRF verification response.
type VerifyResponse struct {
	Valid bool   `json:"valid"`
	Error string `json:"error,omitempty"`
}

// Backward-compatible aliases used by tests.
type RandomRequest = DirectRandomRequest
type RandomResponse = DirectRandomResponse

// =============================================================================
// Type Converters
// =============================================================================

func vrfRecordFromReq(req *VRFRequest) *vrfsupabase.RequestRecord {
	return &vrfsupabase.RequestRecord{
		ID:               req.ID,
		RequestID:        req.RequestID,
		UserID:           req.UserID,
		RequesterAddress: req.RequesterAddress,
		Seed:             req.Seed,
		NumWords:         req.NumWords,
		CallbackGasLimit: req.CallbackGasLimit,
		Status:           req.Status,
		RandomWords:      req.RandomWords,
		Proof:            req.Proof,
		FulfillTxHash:    req.FulfillTxHash,
		Error:            req.Error,
		CreatedAt:        req.CreatedAt,
		FulfilledAt:      req.FulfilledAt,
	}
}

func vrfReqFromRecord(rec *vrfsupabase.RequestRecord) *VRFRequest {
	return &VRFRequest{
		ID:               rec.ID,
		RequestID:        rec.RequestID,
		UserID:           rec.UserID,
		RequesterAddress: rec.RequesterAddress,
		Seed:             rec.Seed,
		NumWords:         rec.NumWords,
		CallbackGasLimit: rec.CallbackGasLimit,
		Status:           rec.Status,
		RandomWords:      rec.RandomWords,
		Proof:            rec.Proof,
		FulfillTxHash:    rec.FulfillTxHash,
		Error:            rec.Error,
		CreatedAt:        rec.CreatedAt,
		FulfilledAt:      rec.FulfilledAt,
	}
}
