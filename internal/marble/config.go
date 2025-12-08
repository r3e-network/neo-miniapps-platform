// Package marble provides common service configuration for MarbleRun services.
package marble

import (
	"os"

	"github.com/R3E-Network/service_layer/internal/chain"
	"github.com/R3E-Network/service_layer/internal/database"
)

// CommonConfig holds common configuration for all services.
// Services can embed this struct and add service-specific fields.
type CommonConfig struct {
	// Core dependencies
	Marble *Marble
	DB     *database.Repository

	// Chain dependencies (optional, for services that interact with chain)
	ChainClient   *chain.Client
	TEEFulfiller  *chain.TEEFulfiller
	EventListener *chain.EventListener

	// Contract hashes (optional)
	GatewayHash    string
	ServiceHash    string // Service-specific contract hash
	DataFeedsHash  string
	VRFHash        string
	MixerHash      string
	AutomationHash string
}

// LoadContractHashesFromEnv loads contract hashes from environment variables.
func (c *CommonConfig) LoadContractHashesFromEnv() {
	if h := os.Getenv("CONTRACT_GATEWAY_HASH"); h != "" {
		c.GatewayHash = h
	}
	if h := os.Getenv("CONTRACT_DATAFEEDS_HASH"); h != "" {
		c.DataFeedsHash = h
	}
	if h := os.Getenv("CONTRACT_VRF_HASH"); h != "" {
		c.VRFHash = h
	}
	if h := os.Getenv("CONTRACT_MIXER_HASH"); h != "" {
		c.MixerHash = h
	}
	if h := os.Getenv("CONTRACT_AUTOMATION_HASH"); h != "" {
		c.AutomationHash = h
	}
}

// Validate validates the common configuration.
func (c *CommonConfig) Validate() error {
	// Marble is required for all services
	// DB is optional but recommended
	// Chain dependencies are optional
	return nil
}

// HasChainClient returns true if chain client is configured.
func (c *CommonConfig) HasChainClient() bool {
	return c.ChainClient != nil
}

// HasTEEFulfiller returns true if TEE fulfiller is configured.
func (c *CommonConfig) HasTEEFulfiller() bool {
	return c.TEEFulfiller != nil
}

// HasEventListener returns true if event listener is configured.
func (c *CommonConfig) HasEventListener() bool {
	return c.EventListener != nil
}

// HasDB returns true if database is configured.
func (c *CommonConfig) HasDB() bool {
	return c.DB != nil
}

// CanFulfillRequests returns true if the service can fulfill on-chain requests.
func (c *CommonConfig) CanFulfillRequests() bool {
	return c.HasChainClient() && c.HasTEEFulfiller()
}

// CanListenEvents returns true if the service can listen to chain events.
func (c *CommonConfig) CanListenEvents() bool {
	return c.HasEventListener()
}
