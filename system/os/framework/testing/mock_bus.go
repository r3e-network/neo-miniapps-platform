// Package testing provides test utilities for the service framework.
package testing

import (
	"context"
	"sync"
	"testing"

	"github.com/R3E-Network/service_layer/system/os/framework"
)

// PublishedEvent records an event that was published via the mock bus.
type PublishedEvent struct {
	Event   string
	Payload any
}

// PushedData records data that was pushed via the mock bus.
type PushedData struct {
	Topic   string
	Payload any
}

// InvokedCompute records a compute invocation via the mock bus.
type InvokedCompute struct {
	Payload any
}

// MockBusClient is a test double for framework.BusClient that records all operations.
// It is safe for concurrent use.
type MockBusClient struct {
	mu sync.Mutex

	// Recorded operations
	PublishedEvents []PublishedEvent
	PushedData      []PushedData
	InvokedComputes []InvokedCompute

	// Configurable responses
	PublishError  error
	PushError     error
	InvokeError   error
	InvokeResults []framework.ComputeResult
}

// Ensure MockBusClient implements BusClient at compile time.
var _ framework.BusClient = (*MockBusClient)(nil)

// NewMockBusClient creates a new mock bus client for testing.
func NewMockBusClient() *MockBusClient {
	return &MockBusClient{
		PublishedEvents: make([]PublishedEvent, 0),
		PushedData:      make([]PushedData, 0),
		InvokedComputes: make([]InvokedCompute, 0),
	}
}

// PublishEvent records the event and returns the configured error.
func (m *MockBusClient) PublishEvent(ctx context.Context, event string, payload any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PublishedEvents = append(m.PublishedEvents, PublishedEvent{
		Event:   event,
		Payload: payload,
	})
	return m.PublishError
}

// PushData records the data push and returns the configured error.
func (m *MockBusClient) PushData(ctx context.Context, topic string, payload any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PushedData = append(m.PushedData, PushedData{
		Topic:   topic,
		Payload: payload,
	})
	return m.PushError
}

// InvokeCompute records the invocation and returns the configured results.
func (m *MockBusClient) InvokeCompute(ctx context.Context, payload any) ([]framework.ComputeResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.InvokedComputes = append(m.InvokedComputes, InvokedCompute{
		Payload: payload,
	})
	return m.InvokeResults, m.InvokeError
}

// Reset clears all recorded operations and configured responses.
func (m *MockBusClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PublishedEvents = make([]PublishedEvent, 0)
	m.PushedData = make([]PushedData, 0)
	m.InvokedComputes = make([]InvokedCompute, 0)
	m.PublishError = nil
	m.PushError = nil
	m.InvokeError = nil
	m.InvokeResults = nil
}

// SetPublishError configures the error to return from PublishEvent.
func (m *MockBusClient) SetPublishError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PublishError = err
}

// SetPushError configures the error to return from PushData.
func (m *MockBusClient) SetPushError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PushError = err
}

// SetInvokeError configures the error to return from InvokeCompute.
func (m *MockBusClient) SetInvokeError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.InvokeError = err
}

// SetInvokeResults configures the results to return from InvokeCompute.
func (m *MockBusClient) SetInvokeResults(results []framework.ComputeResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.InvokeResults = results
}

// EventCount returns the number of events published.
func (m *MockBusClient) EventCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.PublishedEvents)
}

// DataCount returns the number of data pushes.
func (m *MockBusClient) DataCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.PushedData)
}

// ComputeCount returns the number of compute invocations.
func (m *MockBusClient) ComputeCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.InvokedComputes)
}

// --- Test Assertions ---

// AssertEventPublished asserts that an event with the given name was published.
func (m *MockBusClient) AssertEventPublished(t *testing.T, event string) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, e := range m.PublishedEvents {
		if e.Event == event {
			return
		}
	}
	t.Errorf("expected event %q to be published, but it was not", event)
}

// AssertEventNotPublished asserts that an event with the given name was NOT published.
func (m *MockBusClient) AssertEventNotPublished(t *testing.T, event string) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, e := range m.PublishedEvents {
		if e.Event == event {
			t.Errorf("expected event %q to NOT be published, but it was", event)
			return
		}
	}
}

// AssertEventPublishedN asserts that exactly n events with the given name were published.
func (m *MockBusClient) AssertEventPublishedN(t *testing.T, event string, n int) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for _, e := range m.PublishedEvents {
		if e.Event == event {
			count++
		}
	}
	if count != n {
		t.Errorf("expected event %q to be published %d times, but was published %d times", event, n, count)
	}
}

// AssertDataPushed asserts that data was pushed to the given topic.
func (m *MockBusClient) AssertDataPushed(t *testing.T, topic string) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, d := range m.PushedData {
		if d.Topic == topic {
			return
		}
	}
	t.Errorf("expected data to be pushed to topic %q, but it was not", topic)
}

// AssertDataNotPushed asserts that data was NOT pushed to the given topic.
func (m *MockBusClient) AssertDataNotPushed(t *testing.T, topic string) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, d := range m.PushedData {
		if d.Topic == topic {
			t.Errorf("expected data to NOT be pushed to topic %q, but it was", topic)
			return
		}
	}
}

// AssertComputeInvoked asserts that compute was invoked at least once.
func (m *MockBusClient) AssertComputeInvoked(t *testing.T) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.InvokedComputes) == 0 {
		t.Error("expected compute to be invoked, but it was not")
	}
}

// AssertComputeNotInvoked asserts that compute was NOT invoked.
func (m *MockBusClient) AssertComputeNotInvoked(t *testing.T) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.InvokedComputes) > 0 {
		t.Errorf("expected compute to NOT be invoked, but it was invoked %d times", len(m.InvokedComputes))
	}
}

// AssertNoOperations asserts that no bus operations were performed.
func (m *MockBusClient) AssertNoOperations(t *testing.T) {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()

	total := len(m.PublishedEvents) + len(m.PushedData) + len(m.InvokedComputes)
	if total > 0 {
		t.Errorf("expected no bus operations, but found %d events, %d data pushes, %d computes",
			len(m.PublishedEvents), len(m.PushedData), len(m.InvokedComputes))
	}
}

// GetPublishedEvents returns a copy of all published events.
func (m *MockBusClient) GetPublishedEvents() []PublishedEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]PublishedEvent, len(m.PublishedEvents))
	copy(result, m.PublishedEvents)
	return result
}

// GetPushedData returns a copy of all pushed data.
func (m *MockBusClient) GetPushedData() []PushedData {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]PushedData, len(m.PushedData))
	copy(result, m.PushedData)
	return result
}

// GetInvokedComputes returns a copy of all compute invocations.
func (m *MockBusClient) GetInvokedComputes() []InvokedCompute {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]InvokedCompute, len(m.InvokedComputes))
	copy(result, m.InvokedComputes)
	return result
}

// LastPublishedEvent returns the most recently published event, or nil if none.
func (m *MockBusClient) LastPublishedEvent() *PublishedEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.PublishedEvents) == 0 {
		return nil
	}
	event := m.PublishedEvents[len(m.PublishedEvents)-1]
	return &event
}

// LastPushedData returns the most recently pushed data, or nil if none.
func (m *MockBusClient) LastPushedData() *PushedData {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.PushedData) == 0 {
		return nil
	}
	data := m.PushedData[len(m.PushedData)-1]
	return &data
}
