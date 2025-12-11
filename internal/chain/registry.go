// Package chain provides contract interaction for the Service Layer.
// This file implements a registry pattern for service-specific chain modules.
package chain

import (
	"context"
	"fmt"
	"sync"
)

// =============================================================================
// Service Chain Module Registry
// =============================================================================

// ServiceChainModule defines the interface for service-specific chain modules.
// Each service implements this interface to register its chain interaction code.
type ServiceChainModule interface {
	// ServiceType returns the service type identifier (e.g., "neorand", "neovault")
	ServiceType() string

	// Initialize initializes the chain module with the given client and wallet
	Initialize(client *Client, wallet *Wallet, contractHash string) error
}

// EventParser defines the interface for parsing contract events.
type EventParser interface {
	// CanParse returns true if this parser can handle the given event
	CanParse(event *ContractEvent) bool

	// Parse parses the event and returns the parsed result
	Parse(event *ContractEvent) (interface{}, error)
}

// registry holds all registered service chain modules
var (
	registryMu      sync.RWMutex
	moduleRegistry  = make(map[string]ServiceChainModule)
	parserRegistry  = make(map[string][]EventParser)
)

// RegisterServiceChain registers a service chain module.
// This is typically called from init() in each service's chain package.
func RegisterServiceChain(module ServiceChainModule) {
	registryMu.Lock()
	defer registryMu.Unlock()

	serviceType := module.ServiceType()
	if _, exists := moduleRegistry[serviceType]; exists {
		panic(fmt.Sprintf("service chain module already registered: %s", serviceType))
	}
	moduleRegistry[serviceType] = module
}

// GetServiceChain returns the registered chain module for a service type.
func GetServiceChain(serviceType string) (ServiceChainModule, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	module, exists := moduleRegistry[serviceType]
	return module, exists
}

// RegisterEventParser registers an event parser for a service type.
func RegisterEventParser(serviceType string, parser EventParser) {
	registryMu.Lock()
	defer registryMu.Unlock()

	parserRegistry[serviceType] = append(parserRegistry[serviceType], parser)
}

// GetEventParsers returns all registered event parsers for a service type.
func GetEventParsers(serviceType string) []EventParser {
	registryMu.RLock()
	defer registryMu.RUnlock()

	return parserRegistry[serviceType]
}

// ParseEvent attempts to parse an event using registered parsers.
func ParseEvent(serviceType string, event *ContractEvent) (interface{}, error) {
	parsers := GetEventParsers(serviceType)
	for _, parser := range parsers {
		if parser.CanParse(event) {
			return parser.Parse(event)
		}
	}
	return nil, fmt.Errorf("no parser found for event: %s", event.EventName)
}

// =============================================================================
// Contract Factory
// =============================================================================

// ContractFactory creates service-specific contracts.
type ContractFactory struct {
	client *Client
	wallet *Wallet
}

// NewContractFactory creates a new contract factory.
func NewContractFactory(client *Client, wallet *Wallet) *ContractFactory {
	return &ContractFactory{
		client: client,
		wallet: wallet,
	}
}

// Client returns the underlying RPC client.
func (f *ContractFactory) Client() *Client {
	return f.client
}

// Wallet returns the underlying wallet.
func (f *ContractFactory) Wallet() *Wallet {
	return f.wallet
}

// InitializeService initializes a service chain module.
func (f *ContractFactory) InitializeService(ctx context.Context, serviceType, contractHash string) error {
	module, exists := GetServiceChain(serviceType)
	if !exists {
		return fmt.Errorf("service chain module not registered: %s", serviceType)
	}
	return module.Initialize(f.client, f.wallet, contractHash)
}
