// Package mixer provides privacy-preserving transaction mixing service.
package mixer

import "time"

// MixDuration represents the mixing time period options.
type MixDuration string

const (
	MixDuration30Min MixDuration = "30m"
	MixDuration1Hour MixDuration = "1h"
	MixDuration24Hour MixDuration = "24h"
	MixDuration7Day  MixDuration = "7d"
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

// RequestStatus represents the lifecycle state of a mix request.
type RequestStatus string

const (
	RequestStatusPending    RequestStatus = "pending"     // Awaiting deposit confirmation
	RequestStatusDeposited  RequestStatus = "deposited"   // Funds received in pool accounts
	RequestStatusMixing     RequestStatus = "mixing"      // Mixing transactions in progress
	RequestStatusCompleted  RequestStatus = "completed"   // Funds delivered to targets
	RequestStatusFailed     RequestStatus = "failed"      // Mix failed, refund initiated
	RequestStatusRefunding  RequestStatus = "refunding"   // Refund in progress
	RequestStatusRefunded   RequestStatus = "refunded"    // Refund completed
	RequestStatusWithdrawable RequestStatus = "withdrawable" // Service unavailable, user can withdraw
)

// PoolAccountStatus represents the state of a TEE-managed pool account.
type PoolAccountStatus string

const (
	PoolAccountStatusActive   PoolAccountStatus = "active"   // Available for mixing
	PoolAccountStatusBusy     PoolAccountStatus = "busy"     // Currently processing transactions
	PoolAccountStatusRetiring PoolAccountStatus = "retiring" // Being phased out
	PoolAccountStatusRetired  PoolAccountStatus = "retired"  // No longer in use
)

// MixRequest represents a user's privacy mixing request.
type MixRequest struct {
	ID              string            `json:"id"`
	AccountID       string            `json:"account_id"`
	Status          RequestStatus     `json:"status"`

	// Input configuration
	SourceWallet    string            `json:"source_wallet"`     // User's source wallet
	Amount          string            `json:"amount"`            // Total amount to mix (decimal string)
	TokenAddress    string            `json:"token_address"`     // Token contract (empty for native)
	MixDuration     MixDuration       `json:"mix_duration"`      // Selected mixing period
	SplitCount      int               `json:"split_count"`       // Number of deposit transactions (1-5)

	// Target configuration
	Targets         []MixTarget       `json:"targets"`           // Destination addresses and amounts

	// Deposit tracking
	DepositTxHashes []string          `json:"deposit_tx_hashes"` // User deposit transaction hashes
	DepositPoolIDs  []string          `json:"deposit_pool_ids"`  // Pool accounts that received deposits
	DepositedAmount string            `json:"deposited_amount"`  // Total deposited so far

	// Proof and security
	ZKProofHash     string            `json:"zk_proof_hash"`     // ZKP commitment hash
	TEESignature    string            `json:"tee_signature"`     // TEE attestation signature
	OnChainProofTx  string            `json:"on_chain_proof_tx"` // Proof submission tx hash

	// Timing
	MixStartAt      time.Time         `json:"mix_start_at"`      // When mixing begins
	MixEndAt        time.Time         `json:"mix_end_at"`        // Expected completion time
	WithdrawableAt  time.Time         `json:"withdrawable_at"`   // When user can force withdraw (7 days after mix_end)
	CompletedAt     time.Time         `json:"completed_at"`      // Actual completion time

	// Completion tracking
	CompletionProofTx string          `json:"completion_proof_tx"` // Completion proof tx hash
	DeliveredAmount   string          `json:"delivered_amount"`    // Total delivered to targets

	// Error handling
	Error           string            `json:"error,omitempty"`
	RefundTxHash    string            `json:"refund_tx_hash,omitempty"`

	Metadata        map[string]string `json:"metadata,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// MixTarget represents a destination for mixed funds.
type MixTarget struct {
	Address   string `json:"address"`    // Destination wallet address
	Amount    string `json:"amount"`     // Amount to deliver (decimal string)
	Delivered bool   `json:"delivered"`  // Whether funds have been delivered
	TxHash    string `json:"tx_hash"`    // Delivery transaction hash
}

// PoolAccount represents a TEE-managed mixing pool account.
type PoolAccount struct {
	ID              string            `json:"id"`
	WalletAddress   string            `json:"wallet_address"`
	Status          PoolAccountStatus `json:"status"`

	// Balance tracking
	Balance         string            `json:"balance"`           // Current balance (decimal string)
	PendingIn       string            `json:"pending_in"`        // Pending incoming amount
	PendingOut      string            `json:"pending_out"`       // Pending outgoing amount

	// TEE management
	TEEKeyID        string            `json:"tee_key_id"`        // TEE enclave key identifier
	EncryptedPrivKey string           `json:"encrypted_priv_key"` // Encrypted private key (TEE sealed)

	// Activity tracking
	TotalReceived   string            `json:"total_received"`    // Lifetime received
	TotalSent       string            `json:"total_sent"`        // Lifetime sent
	TransactionCount int64            `json:"transaction_count"` // Total transactions

	// Lifecycle
	RetireAfter     time.Time         `json:"retire_after"`      // Scheduled retirement time
	LastActivityAt  time.Time         `json:"last_activity_at"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// MixTransaction represents an internal mixing transaction between pool accounts.
type MixTransaction struct {
	ID              string            `json:"id"`
	Type            MixTxType         `json:"type"`
	Status          MixTxStatus       `json:"status"`

	FromPoolID      string            `json:"from_pool_id"`
	ToPoolID        string            `json:"to_pool_id"`
	Amount          string            `json:"amount"`

	// For user-related transactions
	RequestID       string            `json:"request_id,omitempty"`
	TargetAddress   string            `json:"target_address,omitempty"` // For delivery transactions

	TxHash          string            `json:"tx_hash"`
	BlockNumber     int64             `json:"block_number"`
	GasUsed         string            `json:"gas_used"`

	Error           string            `json:"error,omitempty"`
	ScheduledAt     time.Time         `json:"scheduled_at"`
	ExecutedAt      time.Time         `json:"executed_at"`
	ConfirmedAt     time.Time         `json:"confirmed_at"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// MixTxType categorizes mixing transactions.
type MixTxType string

const (
	MixTxTypeDeposit   MixTxType = "deposit"   // User deposit to pool
	MixTxTypeInternal  MixTxType = "internal"  // Pool-to-pool obfuscation
	MixTxTypeDelivery  MixTxType = "delivery"  // Pool to target address
	MixTxTypeRefund    MixTxType = "refund"    // Return to user on failure
	MixTxTypeDecoy     MixTxType = "decoy"     // Fake transaction for obfuscation
)

// MixTxStatus represents transaction execution state.
type MixTxStatus string

const (
	MixTxStatusScheduled  MixTxStatus = "scheduled"
	MixTxStatusPending    MixTxStatus = "pending"
	MixTxStatusSubmitted  MixTxStatus = "submitted"
	MixTxStatusConfirmed  MixTxStatus = "confirmed"
	MixTxStatusFailed     MixTxStatus = "failed"
)

// ServiceDeposit represents the service's collateral/guarantee deposit.
type ServiceDeposit struct {
	ID              string    `json:"id"`
	Amount          string    `json:"amount"`           // Total deposited collateral
	LockedAmount    string    `json:"locked_amount"`    // Amount locked for pending requests
	AvailableAmount string    `json:"available_amount"` // Available for new requests
	WalletAddress   string    `json:"wallet_address"`   // Collateral wallet
	LastTopUpAt     time.Time `json:"last_top_up_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// WithdrawalClaim represents a user's emergency withdrawal claim when service is unavailable.
type WithdrawalClaim struct {
	ID              string            `json:"id"`
	RequestID       string            `json:"request_id"`
	AccountID       string            `json:"account_id"`
	ClaimAmount     string            `json:"claim_amount"`
	ClaimAddress    string            `json:"claim_address"`    // Where to send funds
	Status          ClaimStatus       `json:"status"`

	// On-chain claim
	ClaimTxHash     string            `json:"claim_tx_hash"`    // Claim submission tx
	ClaimBlockNumber int64            `json:"claim_block_number"`
	ClaimableAt     time.Time         `json:"claimable_at"`     // When claim can be executed (7 days)

	// Resolution
	ResolutionTxHash string           `json:"resolution_tx_hash"`
	ResolvedAt      time.Time         `json:"resolved_at"`

	Error           string            `json:"error,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// ClaimStatus represents withdrawal claim state.
type ClaimStatus string

const (
	ClaimStatusPending   ClaimStatus = "pending"   // Claim submitted, waiting period
	ClaimStatusClaimable ClaimStatus = "claimable" // Waiting period passed, can execute
	ClaimStatusExecuted  ClaimStatus = "executed"  // Funds released to user
	ClaimStatusCancelled ClaimStatus = "cancelled" // Service completed mix, claim cancelled
	ClaimStatusRejected  ClaimStatus = "rejected"  // Invalid claim
)

// MixStats provides service statistics.
type MixStats struct {
	TotalRequests      int64     `json:"total_requests"`
	ActiveRequests     int64     `json:"active_requests"`
	CompletedRequests  int64     `json:"completed_requests"`
	TotalVolume        string    `json:"total_volume"`
	ActivePoolAccounts int64     `json:"active_pool_accounts"`
	ServiceDeposit     string    `json:"service_deposit"`
	AvailableCapacity  string    `json:"available_capacity"`
	GeneratedAt        time.Time `json:"generated_at"`
}

// DefaultWithdrawWaitDays is the waiting period for emergency withdrawals.
const DefaultWithdrawWaitDays = 7

// MaxSplitCount is the maximum number of deposit splits allowed.
const MaxSplitCount = 5

// MinSplitCount is the minimum number of deposit splits.
const MinSplitCount = 1

// AutoSplitThreshold is the amount above which auto-splitting is recommended.
const AutoSplitThreshold = "10000" // In token units
