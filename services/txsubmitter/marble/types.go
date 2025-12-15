// Package txsubmitter provides the unified transaction submission service.
package txsubmitter

import (
	"encoding/json"
	"time"
)

// =============================================================================
// Service Constants
// =============================================================================

const (
	ServiceID   = "txsubmitter"
	ServiceName = "TxSubmitter Service"
	Version     = "1.0.0"
)

// =============================================================================
// Rate Limiting
// =============================================================================

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	// GlobalTPS is the global transactions per second limit.
	GlobalTPS int `json:"global_tps"`

	// PerServiceLimits maps service names to their TPS limits.
	PerServiceLimits map[string]int `json:"per_service_limits"`

	// BurstMultiplier allows short bursts above the limit.
	BurstMultiplier float64 `json:"burst_multiplier"`
}

// DefaultRateLimitConfig returns sensible defaults.
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		GlobalTPS: 50,
		PerServiceLimits: map[string]int{
			"neooracle": 20,
			"neofeeds":  10,
			"neorand":   10,
			"neovault":  5,
			"neoflow":   5,
		},
		BurstMultiplier: 1.5,
	}
}

// =============================================================================
// Retry Configuration
// =============================================================================

// RetryConfig holds retry configuration.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int `json:"max_retries"`

	// InitialBackoff is the initial backoff duration.
	InitialBackoff time.Duration `json:"initial_backoff"`

	// MaxBackoff is the maximum backoff duration.
	MaxBackoff time.Duration `json:"max_backoff"`

	// BackoffMultiplier is the multiplier for exponential backoff.
	BackoffMultiplier float64 `json:"backoff_multiplier"`

	// Jitter adds randomness to backoff to prevent thundering herd.
	Jitter float64 `json:"jitter"`
}

// DefaultRetryConfig returns sensible defaults.
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    200 * time.Millisecond,
		MaxBackoff:        30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.1,
	}
}

// =============================================================================
// Transaction Request/Response
// =============================================================================

// TxRequest represents a transaction submission request.
type TxRequest struct {
	// RequestID is a unique identifier for idempotency.
	RequestID string `json:"request_id"`

	// TxType is the type of transaction (e.g., "fulfill_request").
	TxType string `json:"tx_type"`

	// ContractAddress is the target contract.
	ContractAddress string `json:"contract_address"`

	// MethodName is the contract method to invoke.
	MethodName string `json:"method_name"`

	// Params are the method parameters.
	Params json.RawMessage `json:"params"`

	// Priority indicates transaction priority (normal, high, critical).
	Priority TxPriority `json:"priority"`

	// WaitForConfirmation if true, waits for on-chain confirmation.
	WaitForConfirmation bool `json:"wait_for_confirmation"`

	// Timeout for the entire operation.
	Timeout time.Duration `json:"timeout,omitempty"`
}

// TxResponse represents a transaction submission response.
type TxResponse struct {
	// ID is the database record ID.
	ID int64 `json:"id"`

	// TxHash is the transaction hash (if submitted).
	TxHash string `json:"tx_hash,omitempty"`

	// Status is the current transaction status.
	Status string `json:"status"`

	// GasConsumed is the gas consumed (if confirmed).
	GasConsumed int64 `json:"gas_consumed,omitempty"`

	// Error message if failed.
	Error string `json:"error,omitempty"`

	// SubmittedAt is when the transaction was submitted.
	SubmittedAt time.Time `json:"submitted_at"`

	// ConfirmedAt is when the transaction was confirmed.
	ConfirmedAt *time.Time `json:"confirmed_at,omitempty"`
}

// TxPriority represents transaction priority.
type TxPriority int

const (
	PriorityNormal TxPriority = iota
	PriorityHigh
	PriorityCritical
)

// =============================================================================
// Service Status
// =============================================================================

// ServiceStatus represents the current service status.
type ServiceStatus struct {
	Service          string         `json:"service"`
	Version          string         `json:"version"`
	Healthy          bool           `json:"healthy"`
	RPCEndpoints     int            `json:"rpc_endpoints"`
	HealthyEndpoints int            `json:"healthy_endpoints"`
	PendingTxs       int            `json:"pending_txs"`
	TxsSubmitted     int64          `json:"txs_submitted"`
	TxsConfirmed     int64          `json:"txs_confirmed"`
	TxsFailed        int64          `json:"txs_failed"`
	RateLimitStatus  map[string]any `json:"rate_limit_status"`
	Uptime           time.Duration  `json:"uptime"`
}

// =============================================================================
// Authorization
// =============================================================================

// ServiceAllowlist defines which services can submit which transaction types.
var ServiceAllowlist = map[string][]string{
	"neooracle":   {"fulfill_request", "fail_request"},
	"neofeeds":    {"update_price", "update_prices"},
	"neorand":     {"fulfill_request", "fail_request"},
	"neocompute":  {"fulfill_request", "fail_request"},
	"neovault":    {"fulfill_request", "fail_request", "resolve_dispute"},
	"neoflow":     {"execute_trigger"},
	"neoaccounts": {"fulfill_request", "fail_request"},
	"globalsigner": {"set_tee_master_key"},
}

// IsAuthorized checks if a service is authorized to submit a transaction type.
func IsAuthorized(service, txType string) bool {
	allowed, ok := ServiceAllowlist[service]
	if !ok {
		return false
	}
	for _, t := range allowed {
		if t == txType {
			return true
		}
	}
	return false
}
