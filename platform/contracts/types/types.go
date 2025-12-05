// Package types defines common types for Neo N3 service contracts.
package types

import (
	"encoding/hex"
	"time"
)

// =============================================================================
// Core Types
// =============================================================================

// ServiceID identifies a service in the contract system.
type ServiceID string

const (
	ServiceOracle     ServiceID = "oracle"
	ServiceVRF        ServiceID = "vrf"
	ServiceSecrets    ServiceID = "secrets"
	ServiceGasBank    ServiceID = "gasbank"
	ServiceDataFeeds  ServiceID = "datafeeds"
	ServiceAutomation ServiceID = "automation"
	ServiceAccounts   ServiceID = "accounts"
	ServiceCCIP       ServiceID = "ccip"
)

// RequestStatus represents the status of a service request.
type RequestStatus uint8

const (
	RequestStatusPending   RequestStatus = 0
	RequestStatusProcessed RequestStatus = 1
	RequestStatusFailed    RequestStatus = 2
	RequestStatusCancelled RequestStatus = 3
)

func (s RequestStatus) String() string {
	switch s {
	case RequestStatusPending:
		return "pending"
	case RequestStatusProcessed:
		return "processed"
	case RequestStatusFailed:
		return "failed"
	case RequestStatusCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// ScriptHash represents a Neo N3 script hash (20 bytes).
type ScriptHash [20]byte

// String returns the hex string representation.
func (s ScriptHash) String() string {
	// Neo uses little-endian for display
	reversed := make([]byte, 20)
	for i := 0; i < 20; i++ {
		reversed[i] = s[19-i]
	}
	return "0x" + hex.EncodeToString(reversed)
}

// Bytes returns the raw bytes.
func (s ScriptHash) Bytes() []byte {
	return s[:]
}

// ScriptHashFromHex parses a hex string to ScriptHash.
func ScriptHashFromHex(s string) (ScriptHash, error) {
	if len(s) >= 2 && s[:2] == "0x" {
		s = s[2:]
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return ScriptHash{}, err
	}
	var hash ScriptHash
	// Reverse for little-endian
	for i := 0; i < 20 && i < len(b); i++ {
		hash[19-i] = b[i]
	}
	return hash, nil
}

// UInt256 represents a 256-bit hash (transaction hash, block hash).
type UInt256 [32]byte

// String returns the hex string representation.
func (u UInt256) String() string {
	reversed := make([]byte, 32)
	for i := 0; i < 32; i++ {
		reversed[i] = u[31-i]
	}
	return "0x" + hex.EncodeToString(reversed)
}

// =============================================================================
// Service Request Types
// =============================================================================

// ServiceRequest represents a request from user contract to service layer.
type ServiceRequest struct {
	// RequestID is the unique identifier for this request
	RequestID string `json:"request_id"`

	// ServiceID identifies which service to invoke
	ServiceID ServiceID `json:"service_id"`

	// Requester is the script hash of the requesting contract
	Requester ScriptHash `json:"requester"`

	// CallbackContract is where to send the result (usually same as Requester)
	CallbackContract ScriptHash `json:"callback_contract"`

	// CallbackMethod is the method to call with the result
	CallbackMethod string `json:"callback_method"`

	// Payload contains service-specific request data
	Payload []byte `json:"payload"`

	// GasDeposit is the amount of GAS deposited for this request
	GasDeposit int64 `json:"gas_deposit"`

	// Status is the current status of the request
	Status RequestStatus `json:"status"`

	// CreatedAt is when the request was created
	CreatedAt time.Time `json:"created_at"`

	// ProcessedAt is when the request was processed
	ProcessedAt *time.Time `json:"processed_at,omitempty"`

	// TxHash is the transaction hash that created this request
	TxHash UInt256 `json:"tx_hash"`

	// BlockHeight is the block height when request was created
	BlockHeight uint32 `json:"block_height"`
}

// ServiceResponse represents a response from service layer to user contract.
type ServiceResponse struct {
	// RequestID matches the original request
	RequestID string `json:"request_id"`

	// ServiceID identifies which service processed this
	ServiceID ServiceID `json:"service_id"`

	// Success indicates if the service call succeeded
	Success bool `json:"success"`

	// Result contains the service response data
	Result []byte `json:"result"`

	// Error contains error message if failed
	Error string `json:"error,omitempty"`

	// GasUsed is the amount of GAS consumed
	GasUsed int64 `json:"gas_used"`

	// ProcessedAt is when the response was generated
	ProcessedAt time.Time `json:"processed_at"`

	// Signature is the TEE signature over the response
	Signature []byte `json:"signature"`
}

// =============================================================================
// Contract Event Types
// =============================================================================

// EventType identifies the type of contract event.
type EventType string

const (
	// Gateway events
	EventServiceRequest  EventType = "ServiceRequest"
	EventServiceResponse EventType = "ServiceResponse"
	EventGasDeposited    EventType = "GasDeposited"
	EventGasWithdrawn    EventType = "GasWithdrawn"

	// Oracle events
	EventOracleRequest  EventType = "OracleRequest"
	EventOracleResponse EventType = "OracleResponse"

	// VRF events
	EventVRFRequest  EventType = "VRFRequest"
	EventVRFResponse EventType = "VRFResponse"

	// DataFeeds events
	EventPriceUpdated EventType = "PriceUpdated"
	EventFeedCreated  EventType = "FeedCreated"

	// Automation events
	EventTriggerCreated   EventType = "TriggerCreated"
	EventTriggerExecuted  EventType = "TriggerExecuted"
	EventTriggerCancelled EventType = "TriggerCancelled"

	// GasBank events
	EventGasDeposit  EventType = "GasDeposit"
	EventGasWithdraw EventType = "GasWithdraw"
	EventGasCharged  EventType = "GasCharged"
)

// ContractEvent represents an event emitted by a contract.
type ContractEvent struct {
	// Contract is the script hash of the contract that emitted the event
	Contract ScriptHash `json:"contract"`

	// EventName is the name of the event
	EventName EventType `json:"event_name"`

	// State contains the event data
	State []any `json:"state"`

	// TxHash is the transaction that emitted this event
	TxHash UInt256 `json:"tx_hash"`

	// BlockHeight is the block containing this event
	BlockHeight uint32 `json:"block_height"`

	// Timestamp is when the event was emitted
	Timestamp time.Time `json:"timestamp"`
}

// =============================================================================
// Oracle Types
// =============================================================================

// OracleRequestPayload is the payload for oracle requests.
type OracleRequestPayload struct {
	// URL is the HTTP URL to fetch
	URL string `json:"url"`

	// Method is the HTTP method (GET, POST)
	Method string `json:"method"`

	// Headers are optional HTTP headers
	Headers map[string]string `json:"headers,omitempty"`

	// Body is optional request body
	Body []byte `json:"body,omitempty"`

	// JSONPath is optional path to extract from JSON response
	JSONPath string `json:"json_path,omitempty"`

	// Filter is optional filter expression
	Filter string `json:"filter,omitempty"`
}

// OracleResponsePayload is the payload for oracle responses.
type OracleResponsePayload struct {
	// Data is the fetched data
	Data []byte `json:"data"`

	// StatusCode is the HTTP status code
	StatusCode int `json:"status_code"`

	// Timestamp is when the data was fetched
	Timestamp time.Time `json:"timestamp"`
}

// =============================================================================
// VRF Types
// =============================================================================

// VRFRequestPayload is the payload for VRF requests.
type VRFRequestPayload struct {
	// Seed is the user-provided seed
	Seed []byte `json:"seed"`

	// NumWords is the number of random words to generate
	NumWords uint32 `json:"num_words"`
}

// VRFResponsePayload is the payload for VRF responses.
type VRFResponsePayload struct {
	// RandomWords are the generated random values
	RandomWords [][]byte `json:"random_words"`

	// Proof is the VRF proof
	Proof []byte `json:"proof"`

	// PublicKey is the VRF public key used
	PublicKey []byte `json:"public_key"`
}

// =============================================================================
// DataFeeds Types
// =============================================================================

// PriceFeed represents a price feed configuration.
type PriceFeed struct {
	// FeedID is the unique identifier
	FeedID string `json:"feed_id"`

	// Pair is the trading pair (e.g., "BTC/USD")
	Pair string `json:"pair"`

	// Sources are the data sources to aggregate
	Sources []string `json:"sources"`

	// Heartbeat is the maximum time between updates
	Heartbeat time.Duration `json:"heartbeat"`

	// Deviation is the minimum price change to trigger update (basis points)
	Deviation uint32 `json:"deviation"`

	// Active indicates if the feed is active
	Active bool `json:"active"`
}

// PriceData represents price data for a feed.
type PriceData struct {
	// FeedID identifies the feed
	FeedID string `json:"feed_id"`

	// Price is the aggregated price (scaled by decimals)
	Price int64 `json:"price"`

	// Decimals is the number of decimal places
	Decimals uint8 `json:"decimals"`

	// Timestamp is when the price was determined
	Timestamp time.Time `json:"timestamp"`

	// RoundID is the update round number
	RoundID uint64 `json:"round_id"`

	// Signature is the TEE signature
	Signature []byte `json:"signature"`
}

// =============================================================================
// Automation Types
// =============================================================================

// TriggerType identifies the type of automation trigger.
type TriggerType uint8

const (
	TriggerTypeCron     TriggerType = 0 // Cron-based periodic trigger
	TriggerTypeInterval TriggerType = 1 // Fixed interval trigger
	TriggerTypeEvent    TriggerType = 2 // Event-based trigger
	TriggerTypeOnce     TriggerType = 3 // One-time trigger
)

// AutomationTrigger represents an automation trigger configuration.
type AutomationTrigger struct {
	// TriggerID is the unique identifier
	TriggerID string `json:"trigger_id"`

	// Owner is the script hash of the owner contract
	Owner ScriptHash `json:"owner"`

	// Type is the trigger type
	Type TriggerType `json:"type"`

	// Target is the contract to call when triggered
	Target ScriptHash `json:"target"`

	// Method is the method to call
	Method string `json:"method"`

	// Args are the arguments to pass
	Args []byte `json:"args"`

	// Schedule is the cron expression or interval
	Schedule string `json:"schedule"`

	// GasLimit is the maximum gas to use per execution
	GasLimit int64 `json:"gas_limit"`

	// Active indicates if the trigger is active
	Active bool `json:"active"`

	// LastExecuted is when the trigger last fired
	LastExecuted *time.Time `json:"last_executed,omitempty"`

	// NextExecution is when the trigger will next fire
	NextExecution *time.Time `json:"next_execution,omitempty"`

	// ExecutionCount is the number of times executed
	ExecutionCount uint64 `json:"execution_count"`
}

// =============================================================================
// GasBank Types
// =============================================================================

// GasAccount represents a gas account for a contract.
type GasAccount struct {
	// Owner is the script hash of the account owner
	Owner ScriptHash `json:"owner"`

	// Balance is the current GAS balance (in fractions)
	Balance int64 `json:"balance"`

	// TotalDeposited is the total amount ever deposited
	TotalDeposited int64 `json:"total_deposited"`

	// TotalUsed is the total amount ever used
	TotalUsed int64 `json:"total_used"`

	// CreatedAt is when the account was created
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the account was last updated
	UpdatedAt time.Time `json:"updated_at"`
}

// =============================================================================
// Contract Addresses
// =============================================================================

// ContractAddresses holds the deployed contract addresses.
type ContractAddresses struct {
	Gateway    ScriptHash `json:"gateway"`
	Oracle     ScriptHash `json:"oracle"`
	VRF        ScriptHash `json:"vrf"`
	Secrets    ScriptHash `json:"secrets"`
	GasBank    ScriptHash `json:"gasbank"`
	DataFeeds  ScriptHash `json:"datafeeds"`
	Automation ScriptHash `json:"automation"`
}
