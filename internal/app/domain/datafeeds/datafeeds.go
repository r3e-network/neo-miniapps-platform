package datafeeds

import "time"

// Feed describes a centralized Chainlink style feed configuration.
// Aligned with DataFeedHub.cs contract Feed struct.
type Feed struct {
	ID           string            `json:"id"`
	AccountID    string            `json:"account_id"`
	Pair         string            `json:"pair"`         // Maps to contract Pair
	Description  string            `json:"description"`
	Decimals     int               `json:"decimals"`
	Heartbeat    time.Duration     `json:"heartbeat"`
	ThresholdPPM int               `json:"threshold_ppm"` // Deviation threshold in parts per million
	SignerSet    []string          `json:"signer_set"`    // Maps to contract Signers (UInt160[])
	Threshold    int               `json:"threshold"`     // Maps to contract Threshold - min signatures required
	Aggregation  string            `json:"aggregation,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// UpdateStatus enumerates update lifecycle states.
type UpdateStatus string

const (
	UpdateStatusPending  UpdateStatus = "pending"
	UpdateStatusAccepted UpdateStatus = "accepted"
	UpdateStatusRejected UpdateStatus = "rejected"
)

// Update captures a submitted price observation/round.
type Update struct {
	ID        string            `json:"id"`
	AccountID string            `json:"account_id"`
	FeedID    string            `json:"feed_id"`
	RoundID   int64             `json:"round_id"`
	Price     string            `json:"price"`
	Signer    string            `json:"signer"`
	Timestamp time.Time         `json:"timestamp"`
	Signature string            `json:"signature"`
	Status    UpdateStatus      `json:"status"`
	Error     string            `json:"error,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}
