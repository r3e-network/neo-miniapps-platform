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
	// Core service-layer contracts.
	Gateway    string `json:"gateway"`
	NeoFeeds   string `json:"neofeeds"`
	NeoFlow    string `json:"neoflow"`
	NeoCompute string `json:"neocompute"`
	NeoOracle  string `json:"neooracle"`
	GasBank    string `json:"gasbank"`

	// MiniApp platform contracts.
	PaymentHub       string `json:"paymenthub"`
	Governance       string `json:"governance"`
	PriceFeed        string `json:"pricefeed"`
	RandomnessLog    string `json:"randomnesslog"`
	AppRegistry      string `json:"appregistry"`
	AutomationAnchor string `json:"automationanchor"`
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

	// MiniApp platform contracts.
	if h := firstNonEmptyEnv("CONTRACT_PAYMENTHUB_HASH", "CONTRACT_PAYMENT_HUB_HASH"); h != "" {
		c.PaymentHub = h
	}
	if h := os.Getenv("CONTRACT_GOVERNANCE_HASH"); h != "" {
		c.Governance = h
	}
	if h := firstNonEmptyEnv("CONTRACT_PRICEFEED_HASH", "CONTRACT_PRICE_FEED_HASH"); h != "" {
		c.PriceFeed = h
	}
	if h := firstNonEmptyEnv("CONTRACT_RANDOMNESSLOG_HASH", "CONTRACT_RANDOMNESS_LOG_HASH"); h != "" {
		c.RandomnessLog = h
	}
	if h := firstNonEmptyEnv("CONTRACT_APPREGISTRY_HASH", "CONTRACT_APP_REGISTRY_HASH"); h != "" {
		c.AppRegistry = h
	}
	if h := firstNonEmptyEnv("CONTRACT_AUTOMATIONANCHOR_HASH", "CONTRACT_AUTOMATION_ANCHOR_HASH"); h != "" {
		c.AutomationAnchor = h
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
