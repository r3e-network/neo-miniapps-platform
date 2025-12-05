// Package client provides Go bindings for interacting with Neo N3 service contracts.
package client

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/platform/contracts/types"
)

// =============================================================================
// Contract Client Interface
// =============================================================================

// Client provides methods to interact with Neo N3 service contracts.
type Client interface {
	// Gateway operations
	SubmitCallback(ctx context.Context, resp *types.ServiceResponse) error
	GetRequest(ctx context.Context, requestID string) (*types.ServiceRequest, error)
	GetPendingRequests(ctx context.Context, serviceID types.ServiceID) ([]*types.ServiceRequest, error)

	// DataFeeds operations
	UpdatePrice(ctx context.Context, feedID string, price int64, decimals uint8, timestamp time.Time, signature []byte) error
	GetLatestPrice(ctx context.Context, feedID string) (*types.PriceData, error)

	// Automation operations
	ExecuteTrigger(ctx context.Context, triggerID string, signature []byte) error
	GetTrigger(ctx context.Context, triggerID string) (*types.AutomationTrigger, error)

	// Event subscription
	SubscribeEvents(ctx context.Context, filter EventFilter) (<-chan *types.ContractEvent, error)
	Close() error
}

// EventFilter specifies which events to subscribe to.
type EventFilter struct {
	Contracts  []types.ScriptHash
	EventNames []types.EventType
	FromBlock  uint32
}

// =============================================================================
// Client Configuration
// =============================================================================

// Config holds client configuration.
type Config struct {
	// RPC endpoint for Neo N3 node
	RPCEndpoint string `json:"rpc_endpoint"`

	// WebSocket endpoint for event subscription
	WSEndpoint string `json:"ws_endpoint"`

	// Contract addresses
	Contracts types.ContractAddresses `json:"contracts"`

	// Service layer wallet for signing transactions
	WalletPath string `json:"wallet_path"`

	// Network magic number
	NetworkMagic uint32 `json:"network_magic"`

	// Timeout for RPC calls
	Timeout time.Duration `json:"timeout"`
}

// =============================================================================
// Neo N3 Client Implementation
// =============================================================================

// NeoClient implements Client for Neo N3 blockchain.
type NeoClient struct {
	mu     sync.RWMutex
	config Config
	closed bool

	// Event channels
	eventCh chan *types.ContractEvent
	stopCh  chan struct{}
}

// NewNeoClient creates a new Neo N3 client.
func NewNeoClient(config Config) (*NeoClient, error) {
	if config.RPCEndpoint == "" {
		return nil, fmt.Errorf("RPC endpoint is required")
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &NeoClient{
		config:  config,
		eventCh: make(chan *types.ContractEvent, 100),
		stopCh:  make(chan struct{}),
	}, nil
}

// SubmitCallback submits a service response callback to the gateway contract.
func (c *NeoClient) SubmitCallback(ctx context.Context, resp *types.ServiceResponse) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	// Build transaction to call Gateway.Callback
	// Parameters: requestId, success, result, error, gasUsed, signature
	params := []interface{}{
		resp.RequestID,
		resp.Success,
		resp.Result,
		resp.Error,
		resp.GasUsed,
		resp.Signature,
	}

	_, err := c.invokeContract(ctx, c.config.Contracts.Gateway, "callback", params)
	return err
}

// GetRequest retrieves a request from the gateway contract.
func (c *NeoClient) GetRequest(ctx context.Context, requestID string) (*types.ServiceRequest, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	result, err := c.callContract(ctx, c.config.Contracts.Gateway, "getRequest", []interface{}{requestID})
	if err != nil {
		return nil, err
	}

	return parseServiceRequest(result)
}

// GetPendingRequests retrieves all pending requests for a service.
func (c *NeoClient) GetPendingRequests(ctx context.Context, serviceID types.ServiceID) ([]*types.ServiceRequest, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	// Note: This would typically be done by monitoring events rather than
	// querying the contract, as Neo N3 doesn't support efficient iteration.
	// For now, return empty - the event listener will track pending requests.
	return nil, nil
}

// UpdatePrice updates a price feed in the DataFeeds contract.
func (c *NeoClient) UpdatePrice(ctx context.Context, feedID string, price int64, decimals uint8, timestamp time.Time, signature []byte) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	// This is called via Gateway, not directly
	// Build payload for DataFeeds service
	params := []interface{}{
		feedID,
		price,
		timestamp.UnixMilli(),
		signature,
	}

	_, err := c.invokeContract(ctx, c.config.Contracts.DataFeeds, "updatePrice", params)
	return err
}

// GetLatestPrice retrieves the latest price for a feed.
func (c *NeoClient) GetLatestPrice(ctx context.Context, feedID string) (*types.PriceData, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	result, err := c.callContract(ctx, c.config.Contracts.DataFeeds, "getLatestPriceData", []interface{}{feedID})
	if err != nil {
		return nil, err
	}

	return parsePriceData(result)
}

// ExecuteTrigger executes an automation trigger.
func (c *NeoClient) ExecuteTrigger(ctx context.Context, triggerID string, signature []byte) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	params := []interface{}{
		triggerID,
		signature,
	}

	_, err := c.invokeContract(ctx, c.config.Contracts.Automation, "executeTrigger", params)
	return err
}

// GetTrigger retrieves a trigger configuration.
func (c *NeoClient) GetTrigger(ctx context.Context, triggerID string) (*types.AutomationTrigger, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	result, err := c.callContract(ctx, c.config.Contracts.Automation, "getTrigger", []interface{}{triggerID})
	if err != nil {
		return nil, err
	}

	return parseTrigger(result)
}

// SubscribeEvents subscribes to contract events.
func (c *NeoClient) SubscribeEvents(ctx context.Context, filter EventFilter) (<-chan *types.ContractEvent, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	// Start event listener goroutine
	go c.eventListener(ctx, filter)

	return c.eventCh, nil
}

// Close closes the client.
func (c *NeoClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	close(c.stopCh)
	close(c.eventCh)
	return nil
}

// =============================================================================
// Internal Methods
// =============================================================================

// invokeContract invokes a contract method (state-changing).
func (c *NeoClient) invokeContract(ctx context.Context, contract types.ScriptHash, method string, params []interface{}) ([]byte, error) {
	// TODO: Implement actual Neo N3 RPC call
	// This would use neo-go or similar library to:
	// 1. Build the transaction
	// 2. Sign with service layer wallet
	// 3. Send to network
	// 4. Wait for confirmation

	return nil, fmt.Errorf("not implemented: invokeContract %s.%s", contract, method)
}

// callContract calls a contract method (read-only).
func (c *NeoClient) callContract(ctx context.Context, contract types.ScriptHash, method string, params []interface{}) ([]byte, error) {
	// TODO: Implement actual Neo N3 RPC call
	// This would use invokefunction RPC method

	return nil, fmt.Errorf("not implemented: callContract %s.%s", contract, method)
}

// eventListener listens for contract events.
func (c *NeoClient) eventListener(ctx context.Context, filter EventFilter) {
	// TODO: Implement WebSocket subscription to Neo N3 events
	// This would:
	// 1. Connect to WebSocket endpoint
	// 2. Subscribe to application log notifications
	// 3. Filter events by contract and event name
	// 4. Parse and send to event channel

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			// Poll for new blocks and events
			// In production, use WebSocket subscription
		}
	}
}

// =============================================================================
// Parsing Helpers
// =============================================================================

func parseServiceRequest(data []byte) (*types.ServiceRequest, error) {
	if data == nil {
		return nil, nil
	}
	var req types.ServiceRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("parse service request: %w", err)
	}
	return &req, nil
}

func parsePriceData(data []byte) (*types.PriceData, error) {
	if data == nil {
		return nil, nil
	}
	var price types.PriceData
	if err := json.Unmarshal(data, &price); err != nil {
		return nil, fmt.Errorf("parse price data: %w", err)
	}
	return &price, nil
}

func parseTrigger(data []byte) (*types.AutomationTrigger, error) {
	if data == nil {
		return nil, nil
	}
	var trigger types.AutomationTrigger
	if err := json.Unmarshal(data, &trigger); err != nil {
		return nil, fmt.Errorf("parse trigger: %w", err)
	}
	return &trigger, nil
}

// =============================================================================
// Event Parser
// =============================================================================

// EventParser parses contract events into typed structures.
type EventParser struct{}

// ParseServiceRequestEvent parses a ServiceRequest event.
func (p *EventParser) ParseServiceRequestEvent(event *types.ContractEvent) (*ServiceRequestEvent, error) {
	if event.EventName != types.EventServiceRequest {
		return nil, fmt.Errorf("not a ServiceRequest event")
	}
	if len(event.State) < 7 {
		return nil, fmt.Errorf("invalid event state")
	}

	return &ServiceRequestEvent{
		RequestID:        toString(event.State[0]),
		ServiceID:        types.ServiceID(toString(event.State[1])),
		Requester:        toScriptHash(event.State[2]),
		CallbackContract: toScriptHash(event.State[3]),
		CallbackMethod:   toString(event.State[4]),
		Payload:          toBytes(event.State[5]),
		GasDeposit:       toInt64(event.State[6]),
		TxHash:           event.TxHash,
		BlockHeight:      event.BlockHeight,
	}, nil
}

// ParsePriceUpdatedEvent parses a PriceUpdated event.
func (p *EventParser) ParsePriceUpdatedEvent(event *types.ContractEvent) (*PriceUpdatedEvent, error) {
	if event.EventName != types.EventPriceUpdated {
		return nil, fmt.Errorf("not a PriceUpdated event")
	}
	if len(event.State) < 5 {
		return nil, fmt.Errorf("invalid event state")
	}

	return &PriceUpdatedEvent{
		FeedID:    toString(event.State[0]),
		Price:     toInt64(event.State[1]),
		Decimals:  uint8(toInt64(event.State[2])),
		RoundID:   uint64(toInt64(event.State[3])),
		Timestamp: time.UnixMilli(toInt64(event.State[4])),
	}, nil
}

// ParseTriggerCreatedEvent parses a TriggerCreated event.
func (p *EventParser) ParseTriggerCreatedEvent(event *types.ContractEvent) (*TriggerCreatedEvent, error) {
	if event.EventName != types.EventTriggerCreated {
		return nil, fmt.Errorf("not a TriggerCreated event")
	}
	if len(event.State) < 7 {
		return nil, fmt.Errorf("invalid event state")
	}

	return &TriggerCreatedEvent{
		TriggerID:   toString(event.State[0]),
		Owner:       toScriptHash(event.State[1]),
		TriggerType: types.TriggerType(toInt64(event.State[2])),
		Target:      toScriptHash(event.State[3]),
		Method:      toString(event.State[4]),
		Schedule:    toBytes(event.State[5]),
		GasLimit:    toInt64(event.State[6]),
	}, nil
}

// =============================================================================
// Parsed Event Types
// =============================================================================

// ServiceRequestEvent is a parsed ServiceRequest event.
type ServiceRequestEvent struct {
	RequestID        string
	ServiceID        types.ServiceID
	Requester        types.ScriptHash
	CallbackContract types.ScriptHash
	CallbackMethod   string
	Payload          []byte
	GasDeposit       int64
	TxHash           types.UInt256
	BlockHeight      uint32
}

// PriceUpdatedEvent is a parsed PriceUpdated event.
type PriceUpdatedEvent struct {
	FeedID    string
	Price     int64
	Decimals  uint8
	RoundID   uint64
	Timestamp time.Time
}

// TriggerCreatedEvent is a parsed TriggerCreated event.
type TriggerCreatedEvent struct {
	TriggerID   string
	Owner       types.ScriptHash
	TriggerType types.TriggerType
	Target      types.ScriptHash
	Method      string
	Schedule    []byte
	GasLimit    int64
}

// =============================================================================
// Conversion Helpers
// =============================================================================

func toString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func toBytes(v any) []byte {
	switch val := v.(type) {
	case []byte:
		return val
	case string:
		b, _ := hex.DecodeString(val)
		return b
	default:
		return nil
	}
}

func toInt64(v any) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case int:
		return int64(val)
	case float64:
		return int64(val)
	default:
		return 0
	}
}

func toScriptHash(v any) types.ScriptHash {
	var hash types.ScriptHash
	switch val := v.(type) {
	case []byte:
		if len(val) == 20 {
			copy(hash[:], val)
		}
	case string:
		h, _ := types.ScriptHashFromHex(val)
		return h
	}
	return hash
}
