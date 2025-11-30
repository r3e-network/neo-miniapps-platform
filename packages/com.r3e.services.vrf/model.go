// Package vrf provides the VRF Service as a ServicePackage.
package vrf

import "time"

// KeyStatus enumerates VRF key lifecycle states.
type KeyStatus string

const (
	KeyStatusInactive        KeyStatus = "inactive"
	KeyStatusPendingApproval KeyStatus = "pending_approval"
	KeyStatusActive          KeyStatus = "active"
	KeyStatusRevoked         KeyStatus = "revoked"
)

// Key represents a VRF signer owned by an account.
type Key struct {
	ID            string            `json:"id"`
	AccountID     string            `json:"account_id"`
	PublicKey     string            `json:"public_key"`
	Label         string            `json:"label"`
	Status        KeyStatus         `json:"status"`
	WalletAddress string            `json:"wallet_address"`
	Attestation   string            `json:"attestation"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// RequestStatus enumerates request lifecycles.
type RequestStatus string

const (
	RequestStatusPending   RequestStatus = "pending"
	RequestStatusFulfilled RequestStatus = "fulfilled"
	RequestStatusFailed    RequestStatus = "failed"
)

// Request captures a VRF consumer invocation.
// Aligned with RandomnessHub.cs contract Request struct.
type Request struct {
	ID          string            `json:"id"`
	AccountID   string            `json:"account_id"`
	KeyID       string            `json:"key_id"` // Maps to contract ServiceId
	Consumer    string            `json:"consumer"`
	Seed        string            `json:"seed"`   // Maps to contract SeedHash
	Status      RequestStatus     `json:"status"` // Maps to contract Status byte
	Result      string            `json:"result"` // Maps to contract Output
	Error       string            `json:"error"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"` // Maps to contract RequestedAt
	UpdatedAt   time.Time         `json:"updated_at"`
	FulfilledAt time.Time         `json:"fulfilled_at"` // Maps to contract FulfilledAt
}
