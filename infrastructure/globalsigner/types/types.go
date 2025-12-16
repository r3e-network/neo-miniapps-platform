// Package types provides shared types for the GlobalSigner service.
// This package exists to break import cycles between marble and supabase packages.
package types

import (
	"time"
)

// =============================================================================
// Service Constants
// =============================================================================

const (
	ServiceID   = "globalsigner"
	ServiceName = "GlobalSigner Service"
	Version     = "1.0.0"

	// Key rotation schedule
	DefaultRotationPeriod = 30 * 24 * time.Hour // 30 days
	DefaultOverlapPeriod  = 7 * 24 * time.Hour  // 7 days overlap
)

// =============================================================================
// Key Status
// =============================================================================

// KeyStatus represents the lifecycle state of a signing key.
type KeyStatus string

const (
	KeyStatusPending     KeyStatus = "pending"     // Created, awaiting on-chain anchor
	KeyStatusActive      KeyStatus = "active"      // Currently active for signing
	KeyStatusOverlapping KeyStatus = "overlapping" // Previous key, still valid during overlap
	KeyStatusRevoked     KeyStatus = "revoked"     // No longer valid
)

// =============================================================================
// Configuration
// =============================================================================

// RotationConfig holds key rotation configuration.
type RotationConfig struct {
	// RotationPeriod is how often keys rotate (default 30 days).
	RotationPeriod time.Duration `json:"rotation_period"`

	// OverlapPeriod is how long old keys remain valid (default 7 days).
	OverlapPeriod time.Duration `json:"overlap_period"`

	// AutoRotate enables automatic rotation via background worker.
	AutoRotate bool `json:"auto_rotate"`

	// RequireOnChainAnchor requires successful chain anchor before activation.
	RequireOnChainAnchor bool `json:"require_on_chain_anchor"`
}

// DefaultRotationConfig returns sensible defaults.
func DefaultRotationConfig() *RotationConfig {
	return &RotationConfig{
		RotationPeriod:       DefaultRotationPeriod,
		OverlapPeriod:        DefaultOverlapPeriod,
		AutoRotate:           true,
		RequireOnChainAnchor: true,
	}
}

// =============================================================================
// Key Version
// =============================================================================

// KeyVersion represents a versioned signing key.
type KeyVersion struct {
	// Version is the unique identifier (e.g., "v2025-01" for monthly rotation).
	Version string `json:"version"`

	// Status is the current lifecycle state.
	Status KeyStatus `json:"status"`

	// PubKeyHex is the compressed public key in hex.
	PubKeyHex string `json:"pubkey_hex"`

	// PubKeyHash is SHA-256(pubkey) used for attestation binding.
	PubKeyHash string `json:"pubkey_hash"`

	// CreatedAt is when the key was generated.
	CreatedAt time.Time `json:"created_at"`

	// ActivatedAt is when the key became active (on-chain anchor confirmed).
	ActivatedAt *time.Time `json:"activated_at,omitempty"`

	// OverlapEndsAt is when the overlap period ends (for overlapping keys).
	OverlapEndsAt *time.Time `json:"overlap_ends_at,omitempty"`

	// RevokedAt is when the key was revoked.
	RevokedAt *time.Time `json:"revoked_at,omitempty"`

	// OnChainTxHash is the transaction hash of the anchor operation.
	OnChainTxHash string `json:"on_chain_tx_hash,omitempty"`
}

// =============================================================================
// Attestation
// =============================================================================

// MasterKeyAttestation is the attestation bundle for a master key.
type MasterKeyAttestation struct {
	// KeyVersion identifies which key this attestation is for.
	KeyVersion string `json:"key_version"`

	// PubKeyHex is the compressed public key.
	PubKeyHex string `json:"pubkey_hex"`

	// PubKeyHash is SHA-256(pubkey), bound to SGX report data.
	PubKeyHash string `json:"pubkey_hash"`

	// Quote is the base64-encoded SGX quote.
	Quote string `json:"quote,omitempty"`

	// MRENCLAVE is the enclave measurement.
	MRENCLAVE string `json:"mrenclave,omitempty"`

	// MRSIGNER is the signer measurement.
	MRSIGNER string `json:"mrsigner,omitempty"`

	// ProdID is the product ID.
	ProdID uint16 `json:"prod_id,omitempty"`

	// ISVSVN is the security version number.
	ISVSVN uint16 `json:"isvsvn,omitempty"`

	// Timestamp is when the attestation was generated.
	Timestamp string `json:"timestamp"`

	// Simulated indicates if running in simulation mode.
	Simulated bool `json:"simulated"`
}

// AttestationArtifact is the database record for attestation storage.
type AttestationArtifact struct {
	ID              int64     `json:"id"`
	KeyID           string    `json:"key_id"`
	ArtifactType    string    `json:"artifact_type"` // "sgx_quote", "bundle"
	ArtifactData    []byte    `json:"artifact_data"`
	PubKeyHash      string    `json:"pubkey_hash"`
	AttestationHash string    `json:"attestation_hash"`
	Metadata        string    `json:"metadata,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// =============================================================================
// API Types
// =============================================================================

// RotateRequest is the request to trigger key rotation.
type RotateRequest struct {
	// Force bypasses the rotation schedule check.
	Force bool `json:"force,omitempty"`
}

// RotateResponse is the response from key rotation.
type RotateResponse struct {
	// OldVersion is the previous active key version.
	OldVersion string `json:"old_version,omitempty"`

	// NewVersion is the newly activated key version.
	NewVersion string `json:"new_version"`

	// OverlapEndsAt is when the old key's overlap period ends.
	OverlapEndsAt *time.Time `json:"overlap_ends_at,omitempty"`

	// RotatedAt is when the rotation occurred.
	RotatedAt time.Time `json:"rotated_at"`

	// Rotated indicates if a rotation actually happened.
	Rotated bool `json:"rotated"`

	// OnChainTxHash is the anchor transaction hash.
	OnChainTxHash string `json:"on_chain_tx_hash,omitempty"`
}

// SignRequest is a request for domain-separated signing.
type SignRequest struct {
	// Domain is the signing domain (e.g., "neocompute", "neoaccounts").
	Domain string `json:"domain"`

	// Data is the data to sign (hex-encoded).
	Data string `json:"data"`

	// KeyVersion optionally specifies which key version to use.
	KeyVersion string `json:"key_version,omitempty"`
}

// SignRawRequest is a request for raw signing without domain separation.
// This is primarily intended for signing Neo transaction witness payloads and
// legacy on-chain messages that do not include a domain prefix.
type SignRawRequest struct {
	// Data is the data to sign (hex-encoded).
	Data string `json:"data"`

	// KeyVersion optionally specifies which key version to use.
	KeyVersion string `json:"key_version,omitempty"`
}

// SignResponse is the response from signing.
type SignResponse struct {
	// Signature is the signature (hex-encoded).
	Signature string `json:"signature"`

	// KeyVersion is the key version used for signing.
	KeyVersion string `json:"key_version"`

	// PubKeyHex is the public key that can verify this signature.
	PubKeyHex string `json:"pubkey_hex"`
}

// DeriveRequest is a request for deterministic key derivation.
type DeriveRequest struct {
	// Domain is the derivation domain.
	Domain string `json:"domain"`

	// Path is the derivation path within the domain.
	Path string `json:"path"`

	// KeyVersion optionally specifies which master key version to use.
	KeyVersion string `json:"key_version,omitempty"`
}

// DeriveResponse is the response from key derivation.
type DeriveResponse struct {
	// PubKeyHex is the derived public key (hex-encoded).
	PubKeyHex string `json:"pubkey_hex"`

	// KeyVersion is the master key version used.
	KeyVersion string `json:"key_version"`
}

// StatusResponse is the service status response.
type StatusResponse struct {
	Service          string        `json:"service"`
	Version          string        `json:"version"`
	Healthy          bool          `json:"healthy"`
	ActiveKeyVersion string        `json:"active_key_version"`
	KeyVersions      []*KeyVersion `json:"key_versions"`
	NextRotation     *time.Time    `json:"next_rotation,omitempty"`
	Uptime           string        `json:"uptime"`
	IsEnclave        bool          `json:"is_enclave"`
}
