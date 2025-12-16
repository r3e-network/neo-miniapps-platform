// Package chain provides contract interaction for the Service Layer.
package chain

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"strings"
)

// =============================================================================
// Contract Addresses (configurable)
// =============================================================================

// ContractAddresses holds the deployed contract addresses.
type ContractAddresses struct {
	Gateway    string `json:"gateway"`
	NeoFeeds   string `json:"neofeeds"`
	NeoFlow    string `json:"neoflow"`
	NeoCompute string `json:"neocompute"`
	NeoOracle  string `json:"neooracle"`
	GasBank    string `json:"gasbank"`
}

// LoadFromEnv loads contract addresses from environment variables.
func (c *ContractAddresses) LoadFromEnv() {
	if h := os.Getenv("CONTRACT_GATEWAY_HASH"); h != "" {
		c.Gateway = h
	}
	if h := firstNonEmptyEnv("CONTRACT_DATAFEEDS_HASH", "CONTRACT_NEOFEEDS_HASH"); h != "" {
		c.NeoFeeds = h
	}
	if h := firstNonEmptyEnv("CONTRACT_AUTOMATION_HASH", "CONTRACT_NEOFLOW_HASH"); h != "" {
		c.NeoFlow = h
	}
	if h := firstNonEmptyEnv("CONTRACT_CONFIDENTIAL_HASH", "CONTRACT_NEOCOMPUTE_HASH"); h != "" {
		c.NeoCompute = h
	}
	if h := firstNonEmptyEnv("CONTRACT_ORACLE_HASH", "CONTRACT_NEOORACLE_HASH"); h != "" {
		c.NeoOracle = h
	}
	if h := os.Getenv("CONTRACT_GASBANK_HASH"); h != "" {
		c.GasBank = h
	}
}

// ContractAddressesFromEnv creates ContractAddresses from environment variables.
func ContractAddressesFromEnv() ContractAddresses {
	c := ContractAddresses{}
	c.LoadFromEnv()
	return c
}

// =============================================================================
// Service Request Types
// =============================================================================

// ContractServiceRequest represents a service request from the on-chain contract.
// Note: This is different from database.ServiceRequest which is for database storage.
type ContractServiceRequest struct {
	ID              *big.Int
	UserContract    string
	Payer           string
	ServiceType     string
	ServiceContract string
	Payload         []byte
	CallbackMethod  string
	Status          uint8
	Fee             *big.Int // DEPRECATED: Fee is managed off-chain via gasbank
	CreatedAt       uint64
	Result          []byte
	Error           string
	CompletedAt     uint64
}

// Request status constants
const (
	StatusPending    uint8 = 0
	StatusProcessing uint8 = 1
	StatusCompleted  uint8 = 2
	StatusFailed     uint8 = 3
	StatusRefunded   uint8 = 4
)

// =============================================================================
// NeoFeeds Types
// =============================================================================

// PriceData represents price data from the contract.
type PriceData struct {
	FeedID    string
	Price     *big.Int
	Decimals  *big.Int
	Timestamp uint64
	UpdatedBy string
}

// ContractFeedConfig represents on-chain price feed configuration from the smart contract.
// Note: This is different from neofeeds.FeedConfig which is for service configuration.
type ContractFeedConfig struct {
	FeedID      string
	Description string
	Decimals    *big.Int
	Active      bool
	CreatedAt   uint64
}

// =============================================================================
// NeoFlow Types
// =============================================================================

// Trigger represents an neoflow trigger from the contract.
type Trigger struct {
	TriggerID      *big.Int
	RequestID      *big.Int
	Owner          string
	TargetContract string
	CallbackMethod string
	TriggerType    uint8
	Condition      string
	CallbackData   []byte
	MaxExecutions  *big.Int
	ExecutionCount *big.Int
	Status         uint8
	CreatedAt      uint64
	LastExecutedAt uint64
	ExpiresAt      uint64
}

// ExecutionRecord represents an execution record from the contract.
type ExecutionRecord struct {
	TriggerID       *big.Int
	ExecutionNumber *big.Int
	Timestamp       uint64
	Success         bool
	ExecutedBy      string
}

// =============================================================================
// Common Invocation Result Checks
// =============================================================================

func isHaltState(state string) bool {
	return strings.HasPrefix(strings.TrimSpace(state), "HALT")
}

func requireHalt(method string, result *InvokeResult) error {
	if result == nil {
		return fmt.Errorf("%s: nil invoke result", method)
	}
	if isHaltState(result.State) {
		return nil
	}

	if msg := strings.TrimSpace(result.Exception); msg != "" {
		return fmt.Errorf("%s: execution failed (%s): %s", method, result.State, msg)
	}
	return fmt.Errorf("%s: execution failed (%s)", method, result.State)
}

func requireStack(method string, result *InvokeResult) error {
	if result == nil {
		return fmt.Errorf("%s: nil invoke result", method)
	}
	if len(result.Stack) == 0 {
		return fmt.Errorf("%s: no result", method)
	}
	return nil
}

func firstStackItem(method string, result *InvokeResult) (StackItem, error) {
	if err := requireStack(method, result); err != nil {
		return StackItem{}, err
	}
	return result.Stack[0], nil
}

// =============================================================================
// Common Contract Helpers
// =============================================================================

// IsTEEAccount checks if an account is a registered TEE account on a given contract.
func IsTEEAccount(ctx context.Context, client *Client, contractHash, account string) (bool, error) {
	return InvokeBool(ctx, client, contractHash, "isTEEAccount", NewHash160Param(account))
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}
