// Package vrf provides Verifiable Random Function service.
package vrf

import (
	"time"

	"github.com/R3E-Network/service_layer/services/base"
)

// VRFOutput represents the output of a VRF operation.
type VRFOutput struct {
	Randomness []byte `json:"randomness"`
	Proof      []byte `json:"proof"`
	Input      []byte `json:"input"`
}

// RequestStatus represents the status of a VRF request.
type RequestStatus string

const (
	RequestStatusPending   RequestStatus = "pending"
	RequestStatusFulfilled RequestStatus = "fulfilled"
	RequestStatusFailed    RequestStatus = "failed"
)

// VRFRequest represents a request for verifiable randomness.
type VRFRequest struct {
	base.BaseEntity

	UserID      string        `json:"user_id,omitempty"`
	RequestID   string        `json:"request_id"`
	AccountID   string        `json:"account_id"`
	Status      RequestStatus `json:"status"`
	Seed        []byte        `json:"seed"`
	BlockHash   []byte        `json:"block_hash"`
	BlockNumber int64         `json:"block_number"`

	// Output (filled when fulfilled)
	Randomness []byte `json:"randomness,omitempty"`
	Proof      []byte `json:"proof,omitempty"`

	// Callback info
	CallbackAddress string `json:"callback_address,omitempty"`
	CallbackTxHash  string `json:"callback_tx_hash,omitempty"`

	// Error info
	Error string `json:"error,omitempty"`

	FulfilledAt time.Time `json:"fulfilled_at,omitempty"`
}

// VRFStats provides service statistics.
type VRFStats struct {
	TotalRequests     int64     `json:"total_requests"`
	FulfilledRequests int64     `json:"fulfilled_requests"`
	PendingRequests   int64     `json:"pending_requests"`
	FailedRequests    int64     `json:"failed_requests"`
	GeneratedAt       time.Time `json:"generated_at"`
}

// GenerateRandomnessRequest represents a request to generate randomness.
type GenerateRandomnessRequest struct {
	AccountID       string `json:"account_id"`
	Seed            []byte `json:"seed"`
	BlockHash       []byte `json:"block_hash"`
	BlockNumber     int64  `json:"block_number"`
	CallbackAddress string `json:"callback_address,omitempty"`
}
