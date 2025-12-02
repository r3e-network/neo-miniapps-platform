// Package events provides the event dispatcher for processing blockchain events.
// It connects the neo-indexer to service layer components, routing contract events
// to appropriate service handlers.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/pkg/logger"
)

// ContractEvent represents a blockchain contract event/notification.
type ContractEvent struct {
	TxHash    string         `json:"tx_hash"`
	Contract  string         `json:"contract"`
	EventName string         `json:"event_name"`
	State     map[string]any `json:"state"`
	Height    int64          `json:"height"`
	Timestamp time.Time      `json:"timestamp"`
}

// EventHandler processes contract events.
type EventHandler interface {
	// HandleEvent processes a contract event.
	// Returns error if processing fails (event may be retried).
	HandleEvent(ctx context.Context, event *ContractEvent) error

	// SupportedEvents returns the list of event names this handler supports.
	SupportedEvents() []string

	// SupportedContracts returns the list of contract hashes this handler supports.
	// Empty slice means all contracts.
	SupportedContracts() []string
}

// EventFilter defines criteria for filtering events.
type EventFilter struct {
	Contracts  []string // Contract hashes to match (empty = all)
	EventNames []string // Event names to match (empty = all)
}

// Match checks if an event matches this filter.
func (f *EventFilter) Match(event *ContractEvent) bool {
	if len(f.Contracts) > 0 {
		matched := false
		for _, c := range f.Contracts {
			if strings.EqualFold(c, event.Contract) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	if len(f.EventNames) > 0 {
		matched := false
		for _, e := range f.EventNames {
			if strings.EqualFold(e, event.EventName) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// HandlerRegistration holds a handler and its filter.
type HandlerRegistration struct {
	ID      string
	Handler EventHandler
	Filter  *EventFilter
}

// Dispatcher routes contract events to registered handlers.
type Dispatcher struct {
	handlers map[string]*HandlerRegistration
	log      *logger.Logger

	// Event queue for async processing
	eventQueue chan *ContractEvent
	queueSize  int

	// Metrics
	eventsProcessed int64
	eventsDropped   int64
	eventsFailed    int64

	mu       sync.RWMutex
	running  bool
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// DispatcherConfig configures the event dispatcher.
type DispatcherConfig struct {
	QueueSize   int
	WorkerCount int
	Logger      *logger.Logger
}

// NewDispatcher creates a new event dispatcher.
func NewDispatcher(cfg DispatcherConfig) *Dispatcher {
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 1000
	}
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 4
	}
	if cfg.Logger == nil {
		cfg.Logger = logger.NewDefault("events")
	}

	return &Dispatcher{
		handlers:   make(map[string]*HandlerRegistration),
		log:        cfg.Logger,
		eventQueue: make(chan *ContractEvent, cfg.QueueSize),
		queueSize:  cfg.QueueSize,
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}
}

// RegisterHandler registers an event handler.
func (d *Dispatcher) RegisterHandler(id string, handler EventHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()

	filter := &EventFilter{
		Contracts:  handler.SupportedContracts(),
		EventNames: handler.SupportedEvents(),
	}

	d.handlers[id] = &HandlerRegistration{
		ID:      id,
		Handler: handler,
		Filter:  filter,
	}

	d.log.WithField("handler_id", id).
		WithField("events", filter.EventNames).
		WithField("contracts", filter.Contracts).
		Info("event handler registered")
}

// UnregisterHandler removes an event handler.
func (d *Dispatcher) UnregisterHandler(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.handlers, id)
	d.log.WithField("handler_id", id).Info("event handler unregistered")
}

// Start begins processing events.
func (d *Dispatcher) Start(ctx context.Context, workerCount int) error {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return fmt.Errorf("dispatcher already running")
	}
	d.running = true
	d.stopCh = make(chan struct{})
	d.doneCh = make(chan struct{})
	d.mu.Unlock()

	if workerCount <= 0 {
		workerCount = 4
	}

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			d.worker(ctx, workerID)
		}(i)
	}

	go func() {
		wg.Wait()
		close(d.doneCh)
	}()

	d.log.WithField("workers", workerCount).Info("event dispatcher started")
	return nil
}

// Stop halts event processing.
func (d *Dispatcher) Stop() {
	d.mu.Lock()
	if !d.running {
		d.mu.Unlock()
		return
	}
	d.running = false
	close(d.stopCh)
	d.mu.Unlock()

	<-d.doneCh
	d.log.Info("event dispatcher stopped")
}

// Dispatch queues an event for processing.
func (d *Dispatcher) Dispatch(event *ContractEvent) error {
	d.mu.RLock()
	running := d.running
	d.mu.RUnlock()

	if !running {
		return fmt.Errorf("dispatcher not running")
	}

	select {
	case d.eventQueue <- event:
		return nil
	default:
		d.mu.Lock()
		d.eventsDropped++
		d.mu.Unlock()
		return fmt.Errorf("event queue full, event dropped")
	}
}

// DispatchSync processes an event synchronously.
func (d *Dispatcher) DispatchSync(ctx context.Context, event *ContractEvent) []error {
	d.mu.RLock()
	handlers := make([]*HandlerRegistration, 0, len(d.handlers))
	for _, h := range d.handlers {
		handlers = append(handlers, h)
	}
	d.mu.RUnlock()

	var errors []error
	for _, reg := range handlers {
		if reg.Filter.Match(event) {
			if err := reg.Handler.HandleEvent(ctx, event); err != nil {
				errors = append(errors, fmt.Errorf("handler %s: %w", reg.ID, err))
			}
		}
	}

	return errors
}

// worker processes events from the queue.
func (d *Dispatcher) worker(ctx context.Context, workerID int) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		case event := <-d.eventQueue:
			d.processEvent(ctx, event)
		}
	}
}

// processEvent dispatches an event to matching handlers.
func (d *Dispatcher) processEvent(ctx context.Context, event *ContractEvent) {
	d.mu.RLock()
	handlers := make([]*HandlerRegistration, 0)
	for _, h := range d.handlers {
		if h.Filter.Match(event) {
			handlers = append(handlers, h)
		}
	}
	d.mu.RUnlock()

	for _, reg := range handlers {
		if err := reg.Handler.HandleEvent(ctx, event); err != nil {
			d.mu.Lock()
			d.eventsFailed++
			d.mu.Unlock()

			d.log.WithField("handler_id", reg.ID).
				WithField("event", event.EventName).
				WithField("contract", event.Contract).
				WithError(err).
				Error("event handler failed")
		}
	}

	d.mu.Lock()
	d.eventsProcessed++
	d.mu.Unlock()
}

// Stats returns dispatcher statistics.
func (d *Dispatcher) Stats() DispatcherStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return DispatcherStats{
		Running:         d.running,
		HandlersCount:   len(d.handlers),
		QueueSize:       len(d.eventQueue),
		QueueCapacity:   d.queueSize,
		EventsProcessed: d.eventsProcessed,
		EventsDropped:   d.eventsDropped,
		EventsFailed:    d.eventsFailed,
	}
}

// DispatcherStats holds dispatcher metrics.
type DispatcherStats struct {
	Running         bool  `json:"running"`
	HandlersCount   int   `json:"handlers_count"`
	QueueSize       int   `json:"queue_size"`
	QueueCapacity   int   `json:"queue_capacity"`
	EventsProcessed int64 `json:"events_processed"`
	EventsDropped   int64 `json:"events_dropped"`
	EventsFailed    int64 `json:"events_failed"`
}

// ParseEventState parses event state from various formats.
func ParseEventState(state any) (map[string]any, error) {
	switch v := state.(type) {
	case map[string]any:
		return v, nil
	case []byte:
		var result map[string]any
		if err := json.Unmarshal(v, &result); err != nil {
			return nil, err
		}
		return result, nil
	case string:
		var result map[string]any
		if err := json.Unmarshal([]byte(v), &result); err != nil {
			return nil, err
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported state type: %T", state)
	}
}
