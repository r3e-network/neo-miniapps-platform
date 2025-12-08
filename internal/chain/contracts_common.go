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
	VRF        string `json:"vrf"`
	Mixer      string `json:"mixer"`
	DataFeeds  string `json:"datafeeds"`
	GasBank    string `json:"gasbank"`
	Automation string `json:"automation"`
}

// LoadFromEnv loads contract addresses from environment variables.
func (c *ContractAddresses) LoadFromEnv() {
	if h := os.Getenv("CONTRACT_GATEWAY_HASH"); h != "" {
		c.Gateway = h
	}
	if h := os.Getenv("CONTRACT_VRF_HASH"); h != "" {
		c.VRF = h
	}
	if h := os.Getenv("CONTRACT_MIXER_HASH"); h != "" {
		c.Mixer = h
	}
	if h := os.Getenv("CONTRACT_DATAFEEDS_HASH"); h != "" {
		c.DataFeeds = h
	}
	if h := os.Getenv("CONTRACT_AUTOMATION_HASH"); h != "" {
		c.Automation = h
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

// ServiceRequest represents a service request from the contract.
type ServiceRequest struct {
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
// Mixer Types
// =============================================================================

// MixerPool represents a mixer pool from the contract.
type MixerPool struct {
	Denomination *big.Int
	LeafCount    *big.Int
	Active       bool
}
