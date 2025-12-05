// Package mixer provides privacy-preserving transaction mixing service.
// This implementation follows a single-node mixer design with TEE as the trust root,
// using 1-of-2 multisig addresses (TEE + Master) for fund custody.
package mixer

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/R3E-Network/service_layer/services/base"
)

// =============================================================================
// Mix Duration Options
// =============================================================================

// MixDuration represents the mixing time period options.
type MixDuration string

const (
	MixDuration30Min  MixDuration = "30m"
	MixDuration1Hour  MixDuration = "1h"
	MixDuration24Hour MixDuration = "24h"
	MixDuration7Day   MixDuration = "7d"
)

// ParseMixDuration converts a duration string to MixDuration.
func ParseMixDuration(s string) MixDuration {
	switch s {
	case "30m", "30min":
		return MixDuration30Min
	case "1h", "1hour":
		return MixDuration1Hour
	case "24h", "24hour", "1d":
		return MixDuration24Hour
	case "7d", "7day":
		return MixDuration7Day
	default:
		return MixDuration1Hour
	}
}

// ToDuration converts MixDuration to time.Duration.
func (d MixDuration) ToDuration() time.Duration {
	switch d {
	case MixDuration30Min:
		return 30 * time.Minute
	case MixDuration1Hour:
		return time.Hour
	case MixDuration24Hour:
		return 24 * time.Hour
	case MixDuration7Day:
		return 7 * 24 * time.Hour
	default:
		return time.Hour
	}
}

// =============================================================================
// Request Status Lifecycle
// =============================================================================

// RequestStatus represents the lifecycle state of a mix request.
// Flow: Pending -> Claimed -> Mixing -> Completed
//
//	\-> Refundable -> Refunded (if deadline exceeded)
type RequestStatus string

const (
	// RequestStatusPending - Request created, waiting for service to claim
	RequestStatusPending RequestStatus = "pending"
	// RequestStatusDeposited - User has deposited funds, waiting for claim
	RequestStatusDeposited RequestStatus = "deposited"
	// RequestStatusClaimed - Service claimed the request, funds released to multisig
	RequestStatusClaimed RequestStatus = "claimed"
	// RequestStatusMixing - Mixing in progress within TEE
	RequestStatusMixing RequestStatus = "mixing"
	// RequestStatusCompleted - Mixing completed, completion proof submitted
	RequestStatusCompleted RequestStatus = "completed"
	// RequestStatusRefundable - Deadline exceeded, user can claim refund from bond
	RequestStatusRefundable RequestStatus = "refundable"
	// RequestStatusRefunded - Refund processed from service bond
	RequestStatusRefunded RequestStatus = "refunded"
	// RequestStatusFailed - Request failed (internal error)
	RequestStatusFailed RequestStatus = "failed"
)

// =============================================================================
// Service Registration & Bond
// =============================================================================

// ServiceStatus represents the operational state of the mixer service.
type ServiceStatus string

const (
	ServiceStatusActive    ServiceStatus = "active"
	ServiceStatusSuspended ServiceStatus = "suspended"
	ServiceStatusPaused    ServiceStatus = "paused"
)

// ServiceRegistration represents the mixer service registration on-chain.
type ServiceRegistration struct {
	base.BaseEntity

	// ServiceID is the unique identifier for this service instance
	ServiceID string `json:"service_id"`
	// TEEPublicKey is used by contract to verify TEE signatures
	TEEPublicKey string `json:"tee_public_key"`
	// BondAmount is the current collateral deposited
	BondAmount string `json:"bond_amount"`
	// MinBondRequired is the minimum bond required to operate
	MinBondRequired string `json:"min_bond_required"`
	// MaxOutstandingAllowed is the maximum total pending requests value
	MaxOutstandingAllowed string `json:"max_outstanding_allowed"`
	// CurrentOutstanding is the current total value of uncompleted requests
	CurrentOutstanding string `json:"current_outstanding"`
	// Status is the current operational status
	Status ServiceStatus `json:"status"`
	// ContractAddress is the MixContract address on Neo N3
	ContractAddress string `json:"contract_address"`
	// AttestationReport is the TEE remote attestation report hash
	AttestationReport string `json:"attestation_report"`
	// RegisteredAt is when the service was registered
	RegisteredAt time.Time `json:"registered_at"`
}

// =============================================================================
// Mix Request (Enhanced)
// =============================================================================

// MixRequest represents a user's privacy mixing request.
type MixRequest struct {
	base.BaseEntity

	UserID string `json:"user_id,omitempty"`
	// RequestID mirrors ID for external references
	RequestID string `json:"request_id"`

	// User identification
	AccountID    string `json:"account_id"`
	Depositor    string `json:"depositor"` // User's Neo N3 address
	SourceWallet string `json:"source_wallet"`

	// Request parameters
	Amount       string      `json:"amount"`
	TokenAddress string      `json:"token_address"` // GAS contract hash
	MixDuration  MixDuration `json:"mix_duration"`
	SplitCount   int         `json:"split_count"`

	// Encrypted payload (ECIES encrypted with TEE public key)
	// Contains: target addresses, per-target amounts, user nonce
	EncryptedPayload string `json:"encrypted_payload"`

	// Decrypted targets (only available inside TEE)
	Targets []MixTarget `json:"targets,omitempty"`

	// Status tracking
	Status RequestStatus `json:"status"`

	// Claim information (set when service claims the request)
	ClaimInfo *ClaimInfo `json:"claim_info,omitempty"`

	// Deadline for completion (created_at + mix_duration + safety_buffer)
	Deadline time.Time `json:"deadline"`

	// Completion proof (Merkle root of outputs + TEE signature)
	CompletionProof *CompletionProof `json:"completion_proof,omitempty"`

	// Refund information (if refunded)
	RefundInfo *RefundInfo `json:"refund_info,omitempty"`

	// Timestamps
	CreatedAt   time.Time `json:"created_at"`
	ClaimedAt   time.Time `json:"claimed_at,omitempty"`
	MixStartAt  time.Time `json:"mix_start_at,omitempty"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
	RefundedAt  time.Time `json:"refunded_at,omitempty"`

	// Error tracking
	Error    string            `json:"error,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ClaimInfo contains information about the service claim.
type ClaimInfo struct {
	// ServiceID that claimed this request
	ServiceID string `json:"service_id"`
	// MultisigScriptHash is the 1-of-2 multisig address (TEE + Master)
	MultisigScriptHash string `json:"multisig_script_hash"`
	// TEESignature over the claim message
	TEESignature string `json:"tee_signature"`
	// ClaimTxHash is the on-chain transaction hash
	ClaimTxHash string `json:"claim_tx_hash"`
	// ClaimedAt timestamp
	ClaimedAt time.Time `json:"claimed_at"`
}

// CompletionProof contains the mixing completion proof.
type CompletionProof struct {
	// MerkleRoot of all output leaves: H(targetAddress || amount || nonce || requestId)
	MerkleRoot string `json:"merkle_root"`
	// TEESignature over (merkleRoot || requestId || serviceId || timestamp)
	TEESignature string `json:"tee_signature"`
	// CompletionTxHash is the on-chain proof submission transaction
	CompletionTxHash string `json:"completion_tx_hash"`
	// Timestamp when proof was generated
	Timestamp time.Time `json:"timestamp"`
	// OutputCount is the number of outputs in the Merkle tree
	OutputCount int `json:"output_count"`
}

// RefundInfo contains refund details.
type RefundInfo struct {
	// RefundAmount is the amount refunded from service bond
	RefundAmount string `json:"refund_amount"`
	// RefundTxHash is the on-chain refund transaction
	RefundTxHash string `json:"refund_tx_hash"`
	// RefundedAt timestamp
	RefundedAt time.Time `json:"refunded_at"`
	// Reason for refund
	Reason string `json:"reason"`
}

// MixTarget represents a destination for mixed funds.
type MixTarget struct {
	Address     string    `json:"address"`
	Amount      string    `json:"amount"`
	Nonce       string    `json:"nonce"` // Random nonce for Merkle leaf
	Delivered   bool      `json:"delivered"`
	TxHash      string    `json:"tx_hash,omitempty"`
	DeliveredAt time.Time `json:"delivered_at,omitempty"`
}

// LeafHash computes the Merkle leaf hash for this target.
func (t *MixTarget) LeafHash(requestID string) string {
	data := t.Address + t.Amount + t.Nonce + requestID
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// =============================================================================
// Pool Account (1-of-2 Multisig)
// =============================================================================

// PoolAccountStatus represents the state of a TEE-managed pool account.
type PoolAccountStatus string

const (
	PoolAccountStatusActive   PoolAccountStatus = "active"
	PoolAccountStatusBusy     PoolAccountStatus = "busy"
	PoolAccountStatusRetiring PoolAccountStatus = "retiring"
	PoolAccountStatusRetired  PoolAccountStatus = "retired"
)

// PoolAccount represents a TEE-managed 1-of-2 multisig pool account.
// The multisig is constructed from TEE_PubKey_i and Master_PubKey_i.
type PoolAccount struct {
	base.BaseEntity

	// Address information
	ScriptHash    string `json:"script_hash"`    // Neo N3 script hash (address)
	WalletAddress string `json:"wallet_address"` // Neo N3 address string

	// Multisig components
	TEEPublicKey    string `json:"tee_public_key"`    // Derived from TEE root seed
	MasterPublicKey string `json:"master_public_key"` // From Master's pre-generated pool
	MultiSigScript  string `json:"multisig_script"`   // 1-of-2 verification script
	TEEKeyIndex     uint32 `json:"tee_key_index"`     // HD derivation index for TEE key
	MasterKeyIndex  uint32 `json:"master_key_index"`  // Index in Master's pubkey pool

	// Status
	Status PoolAccountStatus `json:"status"`

	// Balance tracking
	Balance    string `json:"balance"`
	PendingIn  string `json:"pending_in"`
	PendingOut string `json:"pending_out"`

	// Statistics
	TotalReceived    string `json:"total_received"`
	TotalSent        string `json:"total_sent"`
	TransactionCount int64  `json:"transaction_count"`

	// Lifecycle
	RetireAfter    time.Time `json:"retire_after"`
	LastActivityAt time.Time `json:"last_activity_at"`
}

// =============================================================================
// Master Public Key Pool
// =============================================================================

// MasterPubKeyEntry represents a pre-generated Master public key.
type MasterPubKeyEntry struct {
	Index     uint32    `json:"index"`
	PublicKey string    `json:"public_key"`
	Used      bool      `json:"used"`
	UsedAt    time.Time `json:"used_at,omitempty"`
	PoolID    string    `json:"pool_id,omitempty"` // Which pool account uses this key
}

// =============================================================================
// Mix Transactions
// =============================================================================

// MixTxType categorizes mixing transactions.
type MixTxType string

const (
	MixTxTypeDeposit  MixTxType = "deposit"  // Contract -> Multisig
	MixTxTypeInternal MixTxType = "internal" // Multisig -> Multisig (mixing)
	MixTxTypeDelivery MixTxType = "delivery" // Multisig -> Target address
	MixTxTypeRefund   MixTxType = "refund"   // Bond -> User (on timeout)
	MixTxTypeDecoy    MixTxType = "decoy"    // Noise transaction for privacy
)

// MixTxStatus represents transaction execution state.
type MixTxStatus string

const (
	MixTxStatusScheduled MixTxStatus = "scheduled"
	MixTxStatusPending   MixTxStatus = "pending"
	MixTxStatusSubmitted MixTxStatus = "submitted"
	MixTxStatusConfirmed MixTxStatus = "confirmed"
	MixTxStatusFailed    MixTxStatus = "failed"
)

// MixTransaction represents an internal mixing transaction.
type MixTransaction struct {
	base.BaseEntity

	Type   MixTxType   `json:"type"`
	Status MixTxStatus `json:"status"`

	// Source and destination
	FromPoolID    string `json:"from_pool_id,omitempty"`
	ToPoolID      string `json:"to_pool_id,omitempty"`
	TargetAddress string `json:"target_address,omitempty"`

	// Amount and fees
	Amount  string `json:"amount"`
	GasUsed string `json:"gas_used"`

	// Request association
	RequestID string `json:"request_id,omitempty"`

	// Transaction details
	TxHash      string `json:"tx_hash"`
	BlockNumber int64  `json:"block_number"`

	// Retry handling
	RetryCount   int    `json:"retry_count"`
	MaxRetries   int    `json:"max_retries"`
	ErrorMessage string `json:"error_message,omitempty"`

	// Timestamps
	ScheduledAt time.Time `json:"scheduled_at"`
	ExecutedAt  time.Time `json:"executed_at,omitempty"`
	ConfirmedAt time.Time `json:"confirmed_at,omitempty"`
}

// =============================================================================
// Merkle Tree for Completion Proof
// =============================================================================

// MerkleNode represents a node in the Merkle tree.
type MerkleNode struct {
	Hash  string      `json:"hash"`
	Left  *MerkleNode `json:"left,omitempty"`
	Right *MerkleNode `json:"right,omitempty"`
}

// MerkleProof represents a proof for a single leaf.
type MerkleProof struct {
	LeafHash string   `json:"leaf_hash"`
	Path     []string `json:"path"`     // Sibling hashes from leaf to root
	Position []bool   `json:"position"` // true = right, false = left
}

// =============================================================================
// Statistics
// =============================================================================

// MixStats provides service statistics.
type MixStats struct {
	// Request counts
	TotalRequests     int64 `json:"total_requests"`
	ActiveRequests    int64 `json:"active_requests"` // Pending + Deposited + Mixing
	PendingRequests   int64 `json:"pending_requests"`
	ClaimedRequests   int64 `json:"claimed_requests"`
	MixingRequests    int64 `json:"mixing_requests"`
	CompletedRequests int64 `json:"completed_requests"`
	RefundedRequests  int64 `json:"refunded_requests"`
	FailedRequests    int64 `json:"failed_requests"`

	// Volume
	TotalVolume     string `json:"total_volume"`
	PendingVolume   string `json:"pending_volume"`
	CompletedVolume string `json:"completed_volume"`

	// Pool accounts
	TotalPoolAccounts  int64 `json:"total_pool_accounts"`
	ActivePoolAccounts int64 `json:"active_pool_accounts"`

	// Service bond
	BondAmount        string `json:"bond_amount"`
	OutstandingAmount string `json:"outstanding_amount"`
	AvailableCapacity string `json:"available_capacity"`

	// Performance
	AvgMixDuration time.Duration `json:"avg_mix_duration"`
	SuccessRate    float64       `json:"success_rate"`

	GeneratedAt time.Time `json:"generated_at"`
}

// =============================================================================
// Contract Events
// =============================================================================

// ContractEvent represents an event emitted by the MixContract.
type ContractEvent struct {
	EventType   string            `json:"event_type"`
	RequestID   string            `json:"request_id,omitempty"`
	ServiceID   string            `json:"service_id,omitempty"`
	TxHash      string            `json:"tx_hash"`
	BlockNumber int64             `json:"block_number"`
	Timestamp   time.Time         `json:"timestamp"`
	Data        map[string]string `json:"data"`
}

// Contract event types
const (
	EventRequestCreated   = "RequestCreated"
	EventRequestClaimed   = "RequestClaimed"
	EventRequestCompleted = "RequestCompleted"
	EventRefundClaimed    = "RefundClaimed"
	EventBondDeposited    = "BondDeposited"
	EventBondSlashed      = "BondSlashed"
)

// =============================================================================
// Constants
// =============================================================================

const (
	// Safety buffer added to mix duration for deadline calculation
	SafetyBufferDays = 7
	SafetyBuffer     = SafetyBufferDays * 24 * time.Hour

	// Split configuration
	MaxSplitCount      = 5
	MinSplitCount      = 1
	AutoSplitThreshold = "10000" // Auto-split if amount > 10000 GAS

	// Pool management
	DefaultPoolRetireDays = 30
	MaxPoolsPerRequest    = 10
	DecoyTxIntervalMin    = 5 * time.Minute
	DecoyTxIntervalMax    = 30 * time.Minute

	// Retry configuration
	MaxTxRetries = 3
	TxRetryDelay = 30 * time.Second

	// Bond configuration
	BondSafetyFactor = 2.0    // Bond should be 2x max outstanding
	MinBondAmount    = "1000" // Minimum 1000 GAS bond
)

// =============================================================================
// Request Helpers
// =============================================================================

// IsRefundable checks if the request can be refunded.
func (r *MixRequest) IsRefundable() bool {
	if r.Status == RequestStatusCompleted || r.Status == RequestStatusRefunded {
		return false
	}
	return time.Now().After(r.Deadline)
}

// CalculateDeadline computes the deadline for a request.
func (r *MixRequest) CalculateDeadline() time.Time {
	return r.CreatedAt.Add(r.MixDuration.ToDuration()).Add(SafetyBuffer)
}
