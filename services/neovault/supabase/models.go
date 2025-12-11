// Package supabase provides NeoVault-specific database operations.
package supabase

import (
	"errors"
	"time"
)

// ErrNotFound is returned when a record is not found.
var ErrNotFound = errors.New("record not found")

// =============================================================================
// Registration Status Constants
// =============================================================================

// RegistrationStatus represents the approval state of a user registration.
type RegistrationStatus string

const (
	RegStatusPending   RegistrationStatus = "pending"
	RegStatusApproved  RegistrationStatus = "approved"
	RegStatusRejected  RegistrationStatus = "rejected"
	RegStatusSuspended RegistrationStatus = "suspended"
	RegStatusRevoked   RegistrationStatus = "revoked"
)

// IsApproved returns true if the registration allows service usage.
func (s RegistrationStatus) IsApproved() bool {
	return s == RegStatusApproved
}

// =============================================================================
// Registration Model
// =============================================================================

// Registration represents a user's registration application for NeoVault service.
type Registration struct {
	ID     string             `json:"id"`
	UserID string             `json:"user_id"`
	Status RegistrationStatus `json:"status"`

	// Compliance Information
	Email            string    `json:"email,omitempty"`
	Jurisdiction     string    `json:"jurisdiction,omitempty"`
	TermsVersion     string    `json:"terms_version"`
	TermsAcceptedAt  time.Time `json:"terms_accepted_at"`
	Purpose          string    `json:"purpose,omitempty"`
	ExpectedVolume   string    `json:"expected_volume,omitempty"`

	// Admin Review
	ReviewedBy      string    `json:"reviewed_by,omitempty"`
	ReviewedAt      time.Time `json:"reviewed_at,omitempty"`
	ReviewNotes     string    `json:"review_notes,omitempty"`
	RejectionReason string    `json:"rejection_reason,omitempty"`

	// Custom Limits (null = use defaults)
	MaxDailyAmount   *int64 `json:"max_daily_amount,omitempty"`
	MaxMonthlyAmount *int64 `json:"max_monthly_amount,omitempty"`
	MaxSingleAmount  *int64 `json:"max_single_amount,omitempty"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// =============================================================================
// Audit Log Model
// =============================================================================

// AuditLog represents an immutable audit trail entry.
type AuditLog struct {
	ID          string                 `json:"id"`
	UserID      string                 `json:"user_id,omitempty"`
	AdminID     string                 `json:"admin_id,omitempty"`
	Action      string                 `json:"action"`
	EntityType  string                 `json:"entity_type"`
	EntityID    string                 `json:"entity_id,omitempty"`
	IPAddress   string                 `json:"ip_address,omitempty"`
	UserAgent   string                 `json:"user_agent,omitempty"`
	RequestPath string                 `json:"request_path,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
}

// Audit action constants
const (
	AuditActionRegistrationSubmitted = "registration_submitted"
	AuditActionRegistrationApproved  = "registration_approved"
	AuditActionRegistrationRejected  = "registration_rejected"
	AuditActionRegistrationSuspended = "registration_suspended"
	AuditActionRegistrationRevoked   = "registration_revoked"
	AuditActionRequestCreated        = "request_created"
	AuditActionRequestDeposited      = "request_deposited"
	AuditActionRequestMixing         = "request_mixing"
	AuditActionRequestDelivered      = "request_delivered"
	AuditActionRequestFailed         = "request_failed"
	AuditActionRequestDisputed       = "request_disputed"
)

// =============================================================================
// Request Models
// =============================================================================

// TargetAddress represents a delivery target.
type TargetAddress struct {
	Address string `json:"address"`
	Amount  int64  `json:"amount,omitempty"`
}

// RequestRecord represents a neovault request row.
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
