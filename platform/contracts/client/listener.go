// Package client provides Go bindings for interacting with Neo N3 service contracts.
package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/platform/contracts/types"
)

// =============================================================================
// Event Listener
// =============================================================================

// EventListener monitors Neo N3 contract events and dispatches them to handlers.
type EventListener struct {
	mu     sync.RWMutex
	client Client
	config ListenerConfig

	// Handlers
	requestHandlers  []ServiceRequestHandler
	priceHandlers    []PriceUpdateHandler
	triggerHandlers  []TriggerHandler
	responseHandlers []ServiceResponseHandler

	// State
	running     bool
	lastBlock   uint32
	stopCh      chan struct{}
	parser      *EventParser
}

// ListenerConfig configures the event listener.
type ListenerConfig struct {
	// Contracts to monitor
	Contracts types.ContractAddresses

	// Starting block height (0 = latest)
	StartBlock uint32

	// Polling interval when WebSocket is not available
	PollInterval time.Duration

	// Number of confirmations required
	Confirmations uint32
}

// ServiceRequestHandler handles service request events.
type ServiceRequestHandler func(ctx context.Context, event *ServiceRequestEvent) error

// PriceUpdateHandler handles price update events.
type PriceUpdateHandler func(ctx context.Context, event *PriceUpdatedEvent) error

// TriggerHandler handles trigger events.
type TriggerHandler func(ctx context.Context, event *TriggerCreatedEvent) error

// ServiceResponseHandler handles service response events.
type ServiceResponseHandler func(ctx context.Context, requestID string, success bool) error

// NewEventListener creates a new event listener.
func NewEventListener(client Client, config ListenerConfig) *EventListener {
	if config.PollInterval == 0 {
		config.PollInterval = 5 * time.Second
	}
	if config.Confirmations == 0 {
		config.Confirmations = 1
	}

	return &EventListener{
		client: client,
		config: config,
		stopCh: make(chan struct{}),
		parser: &EventParser{},
	}
}

// OnServiceRequest registers a handler for service request events.
func (l *EventListener) OnServiceRequest(handler ServiceRequestHandler) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.requestHandlers = append(l.requestHandlers, handler)
}

// OnPriceUpdate registers a handler for price update events.
func (l *EventListener) OnPriceUpdate(handler PriceUpdateHandler) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.priceHandlers = append(l.priceHandlers, handler)
}

// OnTrigger registers a handler for trigger events.
func (l *EventListener) OnTrigger(handler TriggerHandler) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.triggerHandlers = append(l.triggerHandlers, handler)
}

// OnServiceResponse registers a handler for service response events.
func (l *EventListener) OnServiceResponse(handler ServiceResponseHandler) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.responseHandlers = append(l.responseHandlers, handler)
}

// Start starts the event listener.
func (l *EventListener) Start(ctx context.Context) error {
	l.mu.Lock()
	if l.running {
		l.mu.Unlock()
		return fmt.Errorf("listener already running")
	}
	l.running = true
	l.mu.Unlock()

	// Subscribe to events
	filter := EventFilter{
		Contracts: []types.ScriptHash{
			l.config.Contracts.Gateway,
			l.config.Contracts.DataFeeds,
			l.config.Contracts.Automation,
			l.config.Contracts.Oracle,
			l.config.Contracts.VRF,
		},
		FromBlock: l.config.StartBlock,
	}

	eventCh, err := l.client.SubscribeEvents(ctx, filter)
	if err != nil {
		l.mu.Lock()
		l.running = false
		l.mu.Unlock()
		return fmt.Errorf("subscribe events: %w", err)
	}

	// Start event processing goroutine
	go l.processEvents(ctx, eventCh)

	return nil
}

// Stop stops the event listener.
func (l *EventListener) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.running {
		return nil
	}

	l.running = false
	close(l.stopCh)
	return nil
}

// IsRunning returns whether the listener is running.
func (l *EventListener) IsRunning() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.running
}

// LastProcessedBlock returns the last processed block height.
func (l *EventListener) LastProcessedBlock() uint32 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.lastBlock
}

// processEvents processes incoming events.
func (l *EventListener) processEvents(ctx context.Context, eventCh <-chan *types.ContractEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-l.stopCh:
			return
		case event, ok := <-eventCh:
			if !ok {
				return
			}
			l.handleEvent(ctx, event)
		}
	}
}

// handleEvent dispatches an event to the appropriate handlers.
func (l *EventListener) handleEvent(ctx context.Context, event *types.ContractEvent) {
	l.mu.Lock()
	if event.BlockHeight > l.lastBlock {
		l.lastBlock = event.BlockHeight
	}
	l.mu.Unlock()

	switch event.EventName {
	case types.EventServiceRequest:
		l.handleServiceRequest(ctx, event)
	case types.EventServiceResponse:
		l.handleServiceResponse(ctx, event)
	case types.EventPriceUpdated:
		l.handlePriceUpdate(ctx, event)
	case types.EventTriggerCreated:
		l.handleTriggerCreated(ctx, event)
	case types.EventTriggerExecuted:
		l.handleTriggerExecuted(ctx, event)
	}
}

func (l *EventListener) handleServiceRequest(ctx context.Context, event *types.ContractEvent) {
	parsed, err := l.parser.ParseServiceRequestEvent(event)
	if err != nil {
		return
	}

	l.mu.RLock()
	handlers := l.requestHandlers
	l.mu.RUnlock()

	for _, handler := range handlers {
		go func(h ServiceRequestHandler) {
			_ = h(ctx, parsed)
		}(handler)
	}
}

func (l *EventListener) handleServiceResponse(ctx context.Context, event *types.ContractEvent) {
	if len(event.State) < 3 {
		return
	}

	requestID := toString(event.State[0])
	success := event.State[2] == true || event.State[2] == 1

	l.mu.RLock()
	handlers := l.responseHandlers
	l.mu.RUnlock()

	for _, handler := range handlers {
		go func(h ServiceResponseHandler) {
			_ = h(ctx, requestID, success)
		}(handler)
	}
}

func (l *EventListener) handlePriceUpdate(ctx context.Context, event *types.ContractEvent) {
	parsed, err := l.parser.ParsePriceUpdatedEvent(event)
	if err != nil {
		return
	}

	l.mu.RLock()
	handlers := l.priceHandlers
	l.mu.RUnlock()

	for _, handler := range handlers {
		go func(h PriceUpdateHandler) {
			_ = h(ctx, parsed)
		}(handler)
	}
}

func (l *EventListener) handleTriggerCreated(ctx context.Context, event *types.ContractEvent) {
	parsed, err := l.parser.ParseTriggerCreatedEvent(event)
	if err != nil {
		return
	}

	l.mu.RLock()
	handlers := l.triggerHandlers
	l.mu.RUnlock()

	for _, handler := range handlers {
		go func(h TriggerHandler) {
			_ = h(ctx, parsed)
		}(handler)
	}
}

func (l *EventListener) handleTriggerExecuted(ctx context.Context, event *types.ContractEvent) {
	// Log trigger execution for monitoring
}

// =============================================================================
// Request Tracker
// =============================================================================

// RequestTracker tracks pending service requests.
type RequestTracker struct {
	mu       sync.RWMutex
	requests map[string]*TrackedRequest
}

// TrackedRequest represents a tracked service request.
type TrackedRequest struct {
	Request   *ServiceRequestEvent
	CreatedAt time.Time
	Status    types.RequestStatus
}

// NewRequestTracker creates a new request tracker.
func NewRequestTracker() *RequestTracker {
	return &RequestTracker{
		requests: make(map[string]*TrackedRequest),
	}
}

// Track adds a request to tracking.
func (t *RequestTracker) Track(event *ServiceRequestEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.requests[event.RequestID] = &TrackedRequest{
		Request:   event,
		CreatedAt: time.Now(),
		Status:    types.RequestStatusPending,
	}
}

// Complete marks a request as completed.
func (t *RequestTracker) Complete(requestID string, success bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if req, ok := t.requests[requestID]; ok {
		if success {
			req.Status = types.RequestStatusProcessed
		} else {
			req.Status = types.RequestStatusFailed
		}
	}
}

// Get retrieves a tracked request.
func (t *RequestTracker) Get(requestID string) *TrackedRequest {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.requests[requestID]
}

// GetPending returns all pending requests for a service.
func (t *RequestTracker) GetPending(serviceID types.ServiceID) []*TrackedRequest {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var pending []*TrackedRequest
	for _, req := range t.requests {
		if req.Request.ServiceID == serviceID && req.Status == types.RequestStatusPending {
			pending = append(pending, req)
		}
	}
	return pending
}

// Cleanup removes old completed requests.
func (t *RequestTracker) Cleanup(maxAge time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for id, req := range t.requests {
		if req.Status != types.RequestStatusPending && req.CreatedAt.Before(cutoff) {
			delete(t.requests, id)
		}
	}
}

// =============================================================================
// Trigger Scheduler
// =============================================================================

// TriggerScheduler manages automation trigger scheduling.
type TriggerScheduler struct {
	mu       sync.RWMutex
	triggers map[string]*ScheduledTrigger
	client   Client
	stopCh   chan struct{}
}

// ScheduledTrigger represents a scheduled trigger.
type ScheduledTrigger struct {
	Trigger       *types.AutomationTrigger
	NextExecution time.Time
	Active        bool
}

// NewTriggerScheduler creates a new trigger scheduler.
func NewTriggerScheduler(client Client) *TriggerScheduler {
	return &TriggerScheduler{
		triggers: make(map[string]*ScheduledTrigger),
		client:   client,
		stopCh:   make(chan struct{}),
	}
}

// AddTrigger adds a trigger to the scheduler.
func (s *TriggerScheduler) AddTrigger(trigger *types.AutomationTrigger) {
	s.mu.Lock()
	defer s.mu.Unlock()

	nextExec := s.calculateNextExecution(trigger)
	s.triggers[trigger.TriggerID] = &ScheduledTrigger{
		Trigger:       trigger,
		NextExecution: nextExec,
		Active:        trigger.Active,
	}
}

// RemoveTrigger removes a trigger from the scheduler.
func (s *TriggerScheduler) RemoveTrigger(triggerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.triggers, triggerID)
}

// Start starts the trigger scheduler.
func (s *TriggerScheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case now := <-ticker.C:
			s.checkTriggers(ctx, now)
		}
	}
}

// Stop stops the trigger scheduler.
func (s *TriggerScheduler) Stop() {
	close(s.stopCh)
}

func (s *TriggerScheduler) checkTriggers(ctx context.Context, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, scheduled := range s.triggers {
		if !scheduled.Active {
			continue
		}
		if now.After(scheduled.NextExecution) {
			go s.executeTrigger(ctx, scheduled)
			scheduled.NextExecution = s.calculateNextExecution(scheduled.Trigger)
		}
	}
}

func (s *TriggerScheduler) executeTrigger(ctx context.Context, scheduled *ScheduledTrigger) {
	// Sign the execution request
	signature := []byte{} // TODO: Sign with TEE key

	err := s.client.ExecuteTrigger(ctx, scheduled.Trigger.TriggerID, signature)
	if err != nil {
		// Log error
		return
	}

	// Update execution count
	scheduled.Trigger.ExecutionCount++
	if scheduled.Trigger.LastExecuted != nil {
		now := time.Now()
		scheduled.Trigger.LastExecuted = &now
	}
}

func (s *TriggerScheduler) calculateNextExecution(trigger *types.AutomationTrigger) time.Time {
	now := time.Now()

	switch trigger.Type {
	case types.TriggerTypeInterval:
		// Parse interval from schedule
		// For now, assume schedule is duration string
		return now.Add(time.Minute) // Default 1 minute

	case types.TriggerTypeCron:
		// Parse cron expression
		// TODO: Implement cron parsing
		return now.Add(time.Hour)

	case types.TriggerTypeOnce:
		// Parse timestamp from schedule
		if trigger.NextExecution != nil {
			return *trigger.NextExecution
		}
		return now.Add(time.Hour * 24 * 365) // Far future

	default:
		return now.Add(time.Hour)
	}
}
