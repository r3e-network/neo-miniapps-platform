// Package chain provides contract interaction for the Service Layer.
package chain

import (
	"math/big"
	"os"
)

// =============================================================================
// Contract Addresses (configurable)
// =============================================================================

// ContractAddresses holds the deployed contract addresses.
type ContractAddresses struct {
	Gateway    string `json:"gateway"`
	VRF        string `json:"neorand"`
	NeoVault      string `json:"neovault"`
	NeoFeeds  string `json:"neofeeds"`
	GasBank    string `json:"gasbank"`
	NeoFlow string `json:"neoflow"`
}

// LoadFromEnv loads contract addresses from environment variables.
func (c *ContractAddresses) LoadFromEnv() {
	if h := os.Getenv("CONTRACT_GATEWAY_HASH"); h != "" {
		c.Gateway = h
	}
	if h := os.Getenv("CONTRACT_VRF_HASH"); h != "" {
		c.VRF = h
	}
	if h := os.Getenv("CONTRACT_NEOVAULT_HASH"); h != "" {
		c.NeoVault = h
	}
	if h := os.Getenv("CONTRACT_NEOFEEDS_HASH"); h != "" {
		c.NeoFeeds = h
	}
	if h := os.Getenv("CONTRACT_NEOFLOW_HASH"); h != "" {
		c.NeoFlow = h
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
// NeoVault Types
// =============================================================================

// NeoVaultPool represents a neovault pool from the contract.
type NeoVaultPool struct {
	Denomination *big.Int
	LeafCount    *big.Int
	Active       bool
}

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
