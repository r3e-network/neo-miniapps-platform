// Package events provides structured event logging for the engine.
// Events capture significant occurrences in the engine lifecycle such as
// module state changes, bus operations, errors, and recovery actions.
package events

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/internal/engine/state"
)

// EventType classifies the kind of engine event.
type EventType string

const (
	// Module lifecycle events
	EventModuleRegistered EventType = "module.registered"
	EventModuleStarting   EventType = "module.starting"
	EventModuleStarted    EventType = "module.started"
	EventModuleStartFailed EventType = "module.start_failed"
	EventModuleStopping   EventType = "module.stopping"
	EventModuleStopped    EventType = "module.stopped"
	EventModuleStopFailed EventType = "module.stop_failed"
	EventModuleUnregistered EventType = "module.unregistered"

	// Readiness events
	EventReadinessChanged EventType = "readiness.changed"
	EventReadinessProbe   EventType = "readiness.probe"

	// Bus events
	EventBusPublish       EventType = "bus.publish"
	EventBusPublishFailed EventType = "bus.publish_failed"
	EventBusPush          EventType = "bus.push"
	EventBusPushFailed    EventType = "bus.push_failed"
	EventBusInvoke        EventType = "bus.invoke"
	EventBusInvokeFailed  EventType = "bus.invoke_failed"
	EventBusTimeout       EventType = "bus.timeout"

	// Recovery events
	EventRecoveryStarted  EventType = "recovery.started"
	EventRecoverySucceeded EventType = "recovery.succeeded"
	EventRecoveryFailed   EventType = "recovery.failed"
	EventRecoveryAborted  EventType = "recovery.aborted"

	// Engine events
	EventEngineStarting EventType = "engine.starting"
	EventEngineStarted  EventType = "engine.started"
	EventEngineStopping EventType = "engine.stopping"
	EventEngineStopped  EventType = "engine.stopped"

	// Dependency events
	EventDependencyMet     EventType = "dependency.met"
	EventDependencyMissing EventType = "dependency.missing"
	EventDependencyCycle   EventType = "dependency.cycle"
)

// Severity indicates the importance of an event.
type Severity string

const (
	SeverityDebug   Severity = "debug"
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// Event represents a structured engine event.
type Event struct {
	// Core fields
	ID        string    `json:"id"`
	Type      EventType `json:"type"`
	Severity  Severity  `json:"severity"`
	Timestamp time.Time `json:"timestamp"`

	// Context fields
	Module    string `json:"module,omitempty"`
	Domain    string `json:"domain,omitempty"`
	Component string `json:"component,omitempty"` // engine|bus|lifecycle|recovery

	// State fields
	Status    state.Status    `json:"status,omitempty"`
	Readiness state.Readiness `json:"readiness,omitempty"`

	// Details
	Message  string            `json:"message,omitempty"`
	Error    string            `json:"error,omitempty"`
	Duration time.Duration     `json:"duration_ns,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`

	// Correlation
	TraceID   string `json:"trace_id,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// String returns a human-readable representation.
func (e Event) String() string {
	data, _ := json.Marshal(e)
	return string(data)
}

// EventHandler processes events as they occur.
type EventHandler func(Event)

// EventFilter decides whether an event should be processed.
type EventFilter func(Event) bool

// EventLogger is the interface for event logging.
type EventLogger interface {
	// Log records an event.
	Log(event Event)

	// LogWithContext records an event with context for tracing.
	LogWithContext(ctx context.Context, event Event)

	// Subscribe registers a handler for events.
	Subscribe(handler EventHandler) func()

	// SubscribeFiltered registers a handler with a filter.
	SubscribeFiltered(filter EventFilter, handler EventHandler) func()

	// Recent returns the most recent N events.
	Recent(n int) []Event

	// RecentByModule returns recent events for a specific module.
	RecentByModule(module string, n int) []Event

	// RecentByType returns recent events of a specific type.
	RecentByType(eventType EventType, n int) []Event
}

// RingBuffer is a thread-safe circular buffer for events.
type RingBuffer struct {
	mu       sync.RWMutex
	events   []Event
	size     int
	head     int
	count    int
	handlers []handlerEntry
	nextID   int64
}

type handlerEntry struct {
	id      int64
	filter  EventFilter
	handler EventHandler
}

// NewRingBuffer creates a new event ring buffer.
func NewRingBuffer(size int) *RingBuffer {
	if size <= 0 {
		size = 1000
	}
	return &RingBuffer{
		events: make([]Event, size),
		size:   size,
	}
}

// Log adds an event to the buffer and notifies handlers.
func (rb *RingBuffer) Log(event Event) {
	rb.mu.Lock()
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	if event.ID == "" {
		event.ID = generateEventID()
	}

	rb.events[rb.head] = event
	rb.head = (rb.head + 1) % rb.size
	if rb.count < rb.size {
		rb.count++
	}

	handlers := make([]handlerEntry, len(rb.handlers))
	copy(handlers, rb.handlers)
	rb.mu.Unlock()

	// Notify handlers outside the lock
	for _, h := range handlers {
		if h.filter == nil || h.filter(event) {
			h.handler(event)
		}
	}
}

// LogWithContext adds context information to the event before logging.
func (rb *RingBuffer) LogWithContext(ctx context.Context, event Event) {
	// Extract trace info from context if available
	if traceID := ctx.Value(traceIDKey); traceID != nil {
		if s, ok := traceID.(string); ok {
			event.TraceID = s
		}
	}
	if requestID := ctx.Value(requestIDKey); requestID != nil {
		if s, ok := requestID.(string); ok {
			event.RequestID = s
		}
	}
	rb.Log(event)
}

// Subscribe registers a handler for all events.
func (rb *RingBuffer) Subscribe(handler EventHandler) func() {
	return rb.SubscribeFiltered(nil, handler)
}

// SubscribeFiltered registers a handler with a filter.
func (rb *RingBuffer) SubscribeFiltered(filter EventFilter, handler EventHandler) func() {
	rb.mu.Lock()
	id := rb.nextID
	rb.nextID++
	rb.handlers = append(rb.handlers, handlerEntry{
		id:      id,
		filter:  filter,
		handler: handler,
	})
	rb.mu.Unlock()

	// Return unsubscribe function
	return func() {
		rb.mu.Lock()
		defer rb.mu.Unlock()
		for i, h := range rb.handlers {
			if h.id == id {
				rb.handlers = append(rb.handlers[:i], rb.handlers[i+1:]...)
				return
			}
		}
	}
}

// Recent returns the most recent N events in reverse chronological order.
func (rb *RingBuffer) Recent(n int) []Event {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if n <= 0 || rb.count == 0 {
		return nil
	}
	if n > rb.count {
		n = rb.count
	}

	result := make([]Event, n)
	for i := 0; i < n; i++ {
		idx := (rb.head - 1 - i + rb.size) % rb.size
		result[i] = rb.events[idx]
	}
	return result
}

// RecentByModule returns recent events for a specific module.
func (rb *RingBuffer) RecentByModule(module string, n int) []Event {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if n <= 0 || rb.count == 0 {
		return nil
	}

	var result []Event
	for i := 0; i < rb.count && len(result) < n; i++ {
		idx := (rb.head - 1 - i + rb.size) % rb.size
		if rb.events[idx].Module == module {
			result = append(result, rb.events[idx])
		}
	}
	return result
}

// RecentByType returns recent events of a specific type.
func (rb *RingBuffer) RecentByType(eventType EventType, n int) []Event {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if n <= 0 || rb.count == 0 {
		return nil
	}

	var result []Event
	for i := 0; i < rb.count && len(result) < n; i++ {
		idx := (rb.head - 1 - i + rb.size) % rb.size
		if rb.events[idx].Type == eventType {
			result = append(result, rb.events[idx])
		}
	}
	return result
}

// Count returns the number of events in the buffer.
func (rb *RingBuffer) Count() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.count
}

// Clear removes all events from the buffer.
func (rb *RingBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.events = make([]Event, rb.size)
	rb.head = 0
	rb.count = 0
}

// Context keys for tracing
type contextKey string

const (
	traceIDKey   contextKey = "trace_id"
	requestIDKey contextKey = "request_id"
)

// WithTraceID adds a trace ID to the context.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// WithRequestID adds a request ID to the context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// generateEventID generates a unique event ID.
func generateEventID() string {
	return time.Now().UTC().Format("20060102150405.000000000")
}

// EventBuilder provides a fluent API for creating events.
type EventBuilder struct {
	event Event
}

// NewEvent creates a new EventBuilder.
func NewEvent(eventType EventType) *EventBuilder {
	return &EventBuilder{
		event: Event{
			Type:      eventType,
			Severity:  SeverityInfo,
			Timestamp: time.Now().UTC(),
		},
	}
}

// Module sets the module name.
func (b *EventBuilder) Module(name string) *EventBuilder {
	b.event.Module = name
	return b
}

// Domain sets the domain.
func (b *EventBuilder) Domain(domain string) *EventBuilder {
	b.event.Domain = domain
	return b
}

// Component sets the component.
func (b *EventBuilder) Component(component string) *EventBuilder {
	b.event.Component = component
	return b
}

// Status sets the status.
func (b *EventBuilder) Status(status state.Status) *EventBuilder {
	b.event.Status = status
	return b
}

// Readiness sets the readiness.
func (b *EventBuilder) Readiness(readiness state.Readiness) *EventBuilder {
	b.event.Readiness = readiness
	return b
}

// Severity sets the severity.
func (b *EventBuilder) Severity(severity Severity) *EventBuilder {
	b.event.Severity = severity
	return b
}

// Message sets the message.
func (b *EventBuilder) Message(msg string) *EventBuilder {
	b.event.Message = msg
	return b
}

// Error sets the error.
func (b *EventBuilder) Error(err string) *EventBuilder {
	b.event.Error = err
	b.event.Severity = SeverityError
	return b
}

// ErrorFrom sets the error from an error value.
func (b *EventBuilder) ErrorFrom(err error) *EventBuilder {
	if err != nil {
		b.event.Error = err.Error()
		b.event.Severity = SeverityError
	}
	return b
}

// Duration sets the duration.
func (b *EventBuilder) Duration(d time.Duration) *EventBuilder {
	b.event.Duration = d
	return b
}

// Metadata adds metadata.
func (b *EventBuilder) Metadata(key, value string) *EventBuilder {
	if b.event.Metadata == nil {
		b.event.Metadata = make(map[string]string)
	}
	b.event.Metadata[key] = value
	return b
}

// TraceID sets the trace ID.
func (b *EventBuilder) TraceID(id string) *EventBuilder {
	b.event.TraceID = id
	return b
}

// RequestID sets the request ID.
func (b *EventBuilder) RequestID(id string) *EventBuilder {
	b.event.RequestID = id
	return b
}

// Build returns the constructed event.
func (b *EventBuilder) Build() Event {
	if b.event.ID == "" {
		b.event.ID = generateEventID()
	}
	return b.event
}

// LogTo logs the event to the given logger.
func (b *EventBuilder) LogTo(logger EventLogger) {
	logger.Log(b.Build())
}

// LogToWithContext logs the event with context.
func (b *EventBuilder) LogToWithContext(ctx context.Context, logger EventLogger) {
	logger.LogWithContext(ctx, b.Build())
}

// NoOpLogger is an event logger that discards all events.
type NoOpLogger struct{}

func (NoOpLogger) Log(Event)                                                   {}
func (NoOpLogger) LogWithContext(context.Context, Event)                       {}
func (NoOpLogger) Subscribe(EventHandler) func()                               { return func() {} }
func (NoOpLogger) SubscribeFiltered(EventFilter, EventHandler) func()          { return func() {} }
func (NoOpLogger) Recent(int) []Event                                          { return nil }
func (NoOpLogger) RecentByModule(string, int) []Event                          { return nil }
func (NoOpLogger) RecentByType(EventType, int) []Event                         { return nil }
