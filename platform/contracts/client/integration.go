// Package client provides Go bindings for interacting with Neo N3 service contracts.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/platform/contracts/types"
)

// =============================================================================
// Service Layer Integration
// =============================================================================

// ServiceLayerBridge bridges the Go service layer with Neo N3 contracts.
// It handles:
// - Monitoring contract events for service requests
// - Dispatching requests to appropriate services
// - Sending callbacks with results
// - Managing DataFeeds push updates
// - Managing Automation trigger execution
type ServiceLayerBridge struct {
	mu     sync.RWMutex
	config BridgeConfig

	client    Client
	listener  *EventListener
	tracker   *RequestTracker
	scheduler *TriggerScheduler

	// Service handlers
	serviceHandlers map[types.ServiceID]ServiceHandler

	// State
	running bool
	stopCh  chan struct{}
}

// BridgeConfig configures the service layer bridge.
type BridgeConfig struct {
	// Contract client configuration
	ClientConfig Config

	// Listener configuration
	ListenerConfig ListenerConfig

	// DataFeeds configuration
	DataFeedsConfig DataFeedsConfig

	// Automation configuration
	AutomationConfig AutomationConfig
}

// DataFeedsConfig configures DataFeeds push behavior.
type DataFeedsConfig struct {
	// Feeds to monitor and push
	Feeds []FeedConfig

	// Default update interval
	DefaultInterval time.Duration

	// Enable push updates
	Enabled bool
}

// FeedConfig configures a single price feed.
type FeedConfig struct {
	FeedID    string        `json:"feed_id"`
	Pair      string        `json:"pair"`
	Sources   []string      `json:"sources"`
	Interval  time.Duration `json:"interval"`
	Deviation uint32        `json:"deviation"` // Basis points
}

// AutomationConfig configures Automation behavior.
type AutomationConfig struct {
	// Enable automation processing
	Enabled bool

	// Maximum concurrent trigger executions
	MaxConcurrent int
}

// ServiceHandler handles service requests.
type ServiceHandler interface {
	// HandleRequest processes a service request and returns the result.
	HandleRequest(ctx context.Context, req *ServiceRequestEvent) (*ServiceResult, error)

	// ServiceID returns the service identifier.
	ServiceID() types.ServiceID
}

// ServiceResult contains the result of a service request.
type ServiceResult struct {
	Success bool
	Result  []byte
	Error   string
	GasUsed int64
}

// NewServiceLayerBridge creates a new service layer bridge.
func NewServiceLayerBridge(config BridgeConfig) (*ServiceLayerBridge, error) {
	client, err := NewNeoClient(config.ClientConfig)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	listener := NewEventListener(client, config.ListenerConfig)
	tracker := NewRequestTracker()
	scheduler := NewTriggerScheduler(client)

	return &ServiceLayerBridge{
		config:          config,
		client:          client,
		listener:        listener,
		tracker:         tracker,
		scheduler:       scheduler,
		serviceHandlers: make(map[types.ServiceID]ServiceHandler),
		stopCh:          make(chan struct{}),
	}, nil
}

// RegisterService registers a service handler.
func (b *ServiceLayerBridge) RegisterService(handler ServiceHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.serviceHandlers[handler.ServiceID()] = handler
}

// Start starts the service layer bridge.
func (b *ServiceLayerBridge) Start(ctx context.Context) error {
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return fmt.Errorf("bridge already running")
	}
	b.running = true
	b.mu.Unlock()

	// Register event handlers
	b.listener.OnServiceRequest(b.handleServiceRequest)
	b.listener.OnServiceResponse(b.handleServiceResponse)
	b.listener.OnTrigger(b.handleTriggerCreated)

	// Start event listener
	if err := b.listener.Start(ctx); err != nil {
		b.mu.Lock()
		b.running = false
		b.mu.Unlock()
		return fmt.Errorf("start listener: %w", err)
	}

	// Start trigger scheduler if automation is enabled
	if b.config.AutomationConfig.Enabled {
		go b.scheduler.Start(ctx)
	}

	// Start DataFeeds pusher if enabled
	if b.config.DataFeedsConfig.Enabled {
		go b.runDataFeedsPusher(ctx)
	}

	// Start cleanup goroutine
	go b.runCleanup(ctx)

	return nil
}

// Stop stops the service layer bridge.
func (b *ServiceLayerBridge) Stop() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running {
		return nil
	}

	b.running = false
	close(b.stopCh)

	b.listener.Stop()
	b.scheduler.Stop()
	b.client.Close()

	return nil
}

// =============================================================================
// Event Handlers
// =============================================================================

// handleServiceRequest processes incoming service requests.
func (b *ServiceLayerBridge) handleServiceRequest(ctx context.Context, event *ServiceRequestEvent) error {
	// Track the request
	b.tracker.Track(event)

	// Get the service handler
	b.mu.RLock()
	handler, ok := b.serviceHandlers[event.ServiceID]
	b.mu.RUnlock()

	if !ok {
		// No handler registered, send error callback
		return b.sendCallback(ctx, event.RequestID, &ServiceResult{
			Success: false,
			Error:   fmt.Sprintf("service not found: %s", event.ServiceID),
		})
	}

	// Process the request
	result, err := handler.HandleRequest(ctx, event)
	if err != nil {
		result = &ServiceResult{
			Success: false,
			Error:   err.Error(),
		}
	}

	// Send callback
	return b.sendCallback(ctx, event.RequestID, result)
}

// handleServiceResponse handles service response events.
func (b *ServiceLayerBridge) handleServiceResponse(ctx context.Context, requestID string, success bool) error {
	b.tracker.Complete(requestID, success)
	return nil
}

// handleTriggerCreated handles new trigger creation events.
func (b *ServiceLayerBridge) handleTriggerCreated(ctx context.Context, event *TriggerCreatedEvent) error {
	// Fetch full trigger details
	trigger, err := b.client.GetTrigger(ctx, event.TriggerID)
	if err != nil {
		return err
	}

	// Add to scheduler
	b.scheduler.AddTrigger(trigger)
	return nil
}

// sendCallback sends a service response callback to the gateway.
func (b *ServiceLayerBridge) sendCallback(ctx context.Context, requestID string, result *ServiceResult) error {
	// Sign the response with TEE key
	signature, err := b.signResponse(requestID, result)
	if err != nil {
		return fmt.Errorf("sign response: %w", err)
	}

	resp := &types.ServiceResponse{
		RequestID:   requestID,
		Success:     result.Success,
		Result:      result.Result,
		Error:       result.Error,
		GasUsed:     result.GasUsed,
		ProcessedAt: time.Now(),
		Signature:   signature,
	}

	return b.client.SubmitCallback(ctx, resp)
}

// signResponse signs a service response with the TEE key.
func (b *ServiceLayerBridge) signResponse(requestID string, result *ServiceResult) ([]byte, error) {
	// TODO: Implement TEE signing
	// This should use the service layer's TEE-protected key to sign the response
	data := fmt.Sprintf("%s:%v:%s", requestID, result.Success, result.Error)
	return []byte(data), nil
}

// =============================================================================
// DataFeeds Pusher
// =============================================================================

// runDataFeedsPusher periodically pushes price updates to the DataFeeds contract.
func (b *ServiceLayerBridge) runDataFeedsPusher(ctx context.Context) {
	// Create tickers for each feed
	for _, feed := range b.config.DataFeedsConfig.Feeds {
		go b.runFeedPusher(ctx, feed)
	}
}

// runFeedPusher pushes updates for a single feed.
func (b *ServiceLayerBridge) runFeedPusher(ctx context.Context, feed FeedConfig) {
	interval := feed.Interval
	if interval == 0 {
		interval = b.config.DataFeedsConfig.DefaultInterval
	}
	if interval == 0 {
		interval = time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-b.stopCh:
			return
		case <-ticker.C:
			b.pushFeedUpdate(ctx, feed)
		}
	}
}

// pushFeedUpdate fetches and pushes a price update for a feed.
func (b *ServiceLayerBridge) pushFeedUpdate(ctx context.Context, feed FeedConfig) {
	// Get the DataFeeds service handler
	b.mu.RLock()
	handler, ok := b.serviceHandlers[types.ServiceDataFeeds]
	b.mu.RUnlock()

	if !ok {
		return
	}

	// Create a synthetic request for the feed update
	payload, _ := json.Marshal(map[string]interface{}{
		"feed_id": feed.FeedID,
		"sources": feed.Sources,
	})

	event := &ServiceRequestEvent{
		RequestID: fmt.Sprintf("feed-%s-%d", feed.FeedID, time.Now().UnixNano()),
		ServiceID: types.ServiceDataFeeds,
		Payload:   payload,
	}

	// Process the request
	result, err := handler.HandleRequest(ctx, event)
	if err != nil || !result.Success {
		return
	}

	// Parse the result to get price data
	var priceData struct {
		Price     int64  `json:"price"`
		Decimals  uint8  `json:"decimals"`
		Timestamp int64  `json:"timestamp"`
		Signature []byte `json:"signature"`
	}
	if err := json.Unmarshal(result.Result, &priceData); err != nil {
		return
	}

	// Push to contract
	_ = b.client.UpdatePrice(
		ctx,
		feed.FeedID,
		priceData.Price,
		priceData.Decimals,
		time.UnixMilli(priceData.Timestamp),
		priceData.Signature,
	)
}

// =============================================================================
// Cleanup
// =============================================================================

// runCleanup periodically cleans up old tracked requests.
func (b *ServiceLayerBridge) runCleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-b.stopCh:
			return
		case <-ticker.C:
			b.tracker.Cleanup(time.Hour)
		}
	}
}

// =============================================================================
// Service Handler Adapters
// =============================================================================

// OracleServiceAdapter adapts the Oracle service to the ServiceHandler interface.
type OracleServiceAdapter struct {
	// Reference to the actual Oracle service
	// oracle *oracle.Service
}

func (a *OracleServiceAdapter) ServiceID() types.ServiceID {
	return types.ServiceOracle
}

func (a *OracleServiceAdapter) HandleRequest(ctx context.Context, req *ServiceRequestEvent) (*ServiceResult, error) {
	// Parse the oracle request payload
	var oracleReq types.OracleRequestPayload
	if err := json.Unmarshal(req.Payload, &oracleReq); err != nil {
		return &ServiceResult{Success: false, Error: "invalid payload"}, nil
	}

	// TODO: Call the actual Oracle service
	// result, err := a.oracle.Fetch(ctx, oracleReq.URL, oracleReq.Method, ...)

	return &ServiceResult{
		Success: true,
		Result:  []byte(`{"data": "example"}`),
		GasUsed: 100000,
	}, nil
}

// VRFServiceAdapter adapts the VRF service to the ServiceHandler interface.
type VRFServiceAdapter struct {
	// Reference to the actual VRF service
	// vrf *vrf.Service
}

func (a *VRFServiceAdapter) ServiceID() types.ServiceID {
	return types.ServiceVRF
}

func (a *VRFServiceAdapter) HandleRequest(ctx context.Context, req *ServiceRequestEvent) (*ServiceResult, error) {
	// Parse the VRF request payload
	var vrfReq types.VRFRequestPayload
	if err := json.Unmarshal(req.Payload, &vrfReq); err != nil {
		return &ServiceResult{Success: false, Error: "invalid payload"}, nil
	}

	// TODO: Call the actual VRF service
	// result, err := a.vrf.GenerateRandomness(ctx, vrfReq.Seed, vrfReq.NumWords)

	return &ServiceResult{
		Success: true,
		Result:  []byte(`{"random_words": ["0x..."], "proof": "0x..."}`),
		GasUsed: 150000,
	}, nil
}

// DataFeedsServiceAdapter adapts the DataFeeds service to the ServiceHandler interface.
type DataFeedsServiceAdapter struct {
	// Reference to the actual DataFeeds service
	// datafeeds *datafeeds.Service
}

func (a *DataFeedsServiceAdapter) ServiceID() types.ServiceID {
	return types.ServiceDataFeeds
}

func (a *DataFeedsServiceAdapter) HandleRequest(ctx context.Context, req *ServiceRequestEvent) (*ServiceResult, error) {
	// Parse the request
	var feedReq struct {
		FeedID  string   `json:"feed_id"`
		Sources []string `json:"sources"`
	}
	if err := json.Unmarshal(req.Payload, &feedReq); err != nil {
		return &ServiceResult{Success: false, Error: "invalid payload"}, nil
	}

	// TODO: Call the actual DataFeeds service to fetch and aggregate prices
	// price, err := a.datafeeds.FetchPrice(ctx, feedReq.FeedID, feedReq.Sources)

	result, _ := json.Marshal(map[string]interface{}{
		"price":     12345678,
		"decimals":  8,
		"timestamp": time.Now().UnixMilli(),
		"signature": []byte{},
	})

	return &ServiceResult{
		Success: true,
		Result:  result,
		GasUsed: 50000,
	}, nil
}

// AutomationServiceAdapter adapts the Automation service to the ServiceHandler interface.
type AutomationServiceAdapter struct {
	// Reference to the actual Automation service
	// automation *automation.Service
}

func (a *AutomationServiceAdapter) ServiceID() types.ServiceID {
	return types.ServiceAutomation
}

func (a *AutomationServiceAdapter) HandleRequest(ctx context.Context, req *ServiceRequestEvent) (*ServiceResult, error) {
	// Automation requests are typically trigger creations
	// The actual execution is handled by the TriggerScheduler

	return &ServiceResult{
		Success: true,
		Result:  []byte(`{"status": "trigger_registered"}`),
		GasUsed: 100000,
	}, nil
}
