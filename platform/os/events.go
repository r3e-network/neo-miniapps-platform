// Package os provides the ServiceOS abstraction layer.
package os

import (
	"context"
	"sync"
	"time"
)

// EventBus provides pub/sub messaging between services.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string][]subscription
	closed      bool
}

// subscription represents an event subscription.
type subscription struct {
	id        string
	handler   EventHandler
	serviceID string
}

// NewEventBus creates a new event bus.
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string][]subscription),
	}
}

// Publish publishes an event to all subscribers of a topic.
func (b *EventBus) Publish(ctx context.Context, topic string, source string, data any) error {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return NewOSError("EVENT_BUS_CLOSED", "event bus is closed")
	}
	subs := b.subscribers[topic]
	b.mu.RUnlock()

	if len(subs) == 0 {
		return nil
	}

	event := Event{
		Topic:     topic,
		Data:      data,
		Timestamp: time.Now(),
		Source:    source,
	}

	// Deliver to all subscribers
	for _, sub := range subs {
		// Run handler in goroutine to avoid blocking
		go func(s subscription) {
			_ = s.handler(ctx, event)
		}(sub)
	}

	return nil
}

// Subscribe subscribes to events on a topic.
func (b *EventBus) Subscribe(topic, serviceID string, handler EventHandler) (Subscription, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, NewOSError("EVENT_BUS_CLOSED", "event bus is closed")
	}

	sub := subscription{
		id:        generateSubID(topic, serviceID),
		handler:   handler,
		serviceID: serviceID,
	}

	b.subscribers[topic] = append(b.subscribers[topic], sub)

	return &eventSubscription{
		bus:   b,
		topic: topic,
		id:    sub.id,
	}, nil
}

// Unsubscribe removes a subscription.
func (b *EventBus) Unsubscribe(topic, id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	subs := b.subscribers[topic]
	for i, sub := range subs {
		if sub.id == id {
			b.subscribers[topic] = append(subs[:i], subs[i+1:]...)
			return nil
		}
	}
	return nil
}

// Close closes the event bus.
func (b *EventBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.closed = true
	b.subscribers = make(map[string][]subscription)
	return nil
}

// eventSubscription implements Subscription.
type eventSubscription struct {
	bus   *EventBus
	topic string
	id    string
}

func (s *eventSubscription) Unsubscribe() error {
	return s.bus.Unsubscribe(s.topic, s.id)
}

// generateSubID generates a unique subscription ID.
func generateSubID(topic, serviceID string) string {
	return serviceID + ":" + topic + ":" + time.Now().Format("20060102150405.000000000")
}

// =============================================================================
// Global Event Bus Instance
// =============================================================================

var (
	globalEventBus     *EventBus
	globalEventBusOnce sync.Once
)

// GetEventBus returns the global event bus instance.
func GetEventBus() *EventBus {
	globalEventBusOnce.Do(func() {
		globalEventBus = NewEventBus()
	})
	return globalEventBus
}

// SetEventBus sets the global event bus (for testing).
func SetEventBus(bus *EventBus) {
	globalEventBus = bus
}
