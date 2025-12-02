package events

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestDispatcher_Creation(t *testing.T) {
	d := NewDispatcher(DispatcherConfig{
		QueueSize:   100,
		WorkerCount: 2,
	})

	if d == nil {
		t.Fatal("expected dispatcher, got nil")
	}

	stats := d.Stats()
	if stats.QueueCapacity != 100 {
		t.Errorf("expected queue capacity 100, got %d", stats.QueueCapacity)
	}
}

func TestDispatcher_RegisterHandler(t *testing.T) {
	d := NewDispatcher(DispatcherConfig{})

	handler := &testEventHandler{
		events:    []string{"TestEvent"},
		contracts: []string{"0x1234"},
	}

	d.RegisterHandler("test-handler", handler)

	stats := d.Stats()
	if stats.HandlersCount != 1 {
		t.Errorf("expected 1 handler, got %d", stats.HandlersCount)
	}
}

func TestDispatcher_UnregisterHandler(t *testing.T) {
	d := NewDispatcher(DispatcherConfig{})

	handler := &testEventHandler{
		events:    []string{"TestEvent"},
		contracts: []string{"0x1234"},
	}

	d.RegisterHandler("test-handler", handler)
	d.UnregisterHandler("test-handler")

	stats := d.Stats()
	if stats.HandlersCount != 0 {
		t.Errorf("expected 0 handlers, got %d", stats.HandlersCount)
	}
}

func TestDispatcher_DispatchSync(t *testing.T) {
	d := NewDispatcher(DispatcherConfig{})

	received := false
	handler := &testEventHandler{
		events:    []string{"TestEvent"},
		contracts: []string{},
		callback: func(ctx context.Context, event *ContractEvent) error {
			received = true
			return nil
		},
	}

	d.RegisterHandler("test-handler", handler)

	event := &ContractEvent{
		TxHash:    "0xabc",
		Contract:  "0x1234",
		EventName: "TestEvent",
		State:     map[string]any{"key": "value"},
		Height:    100,
		Timestamp: time.Now(),
	}

	errors := d.DispatchSync(context.Background(), event)
	if len(errors) > 0 {
		t.Errorf("unexpected errors: %v", errors)
	}

	if !received {
		t.Error("expected handler to receive event")
	}
}

func TestDispatcher_StartStop(t *testing.T) {
	d := NewDispatcher(DispatcherConfig{
		QueueSize:   10,
		WorkerCount: 2,
	})

	ctx := context.Background()
	if err := d.Start(ctx, 2); err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	// Should be running
	if err := d.Dispatch(&ContractEvent{EventName: "Test"}); err != nil {
		t.Errorf("dispatch failed while running: %v", err)
	}

	d.Stop()

	// Should fail after stop
	if err := d.Dispatch(&ContractEvent{EventName: "Test"}); err == nil {
		t.Error("expected error after stop")
	}
}

func TestDispatcher_AsyncProcessing(t *testing.T) {
	d := NewDispatcher(DispatcherConfig{
		QueueSize:   100,
		WorkerCount: 2,
	})

	var mu sync.Mutex
	receivedCount := 0

	handler := &testEventHandler{
		events:    []string{"TestEvent"},
		contracts: []string{},
		callback: func(ctx context.Context, event *ContractEvent) error {
			mu.Lock()
			receivedCount++
			mu.Unlock()
			return nil
		},
	}

	d.RegisterHandler("test-handler", handler)

	ctx := context.Background()
	d.Start(ctx, 2)
	defer d.Stop()

	// Dispatch multiple events
	for i := 0; i < 10; i++ {
		d.Dispatch(&ContractEvent{
			EventName: "TestEvent",
			Height:    int64(i),
		})
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	count := receivedCount
	mu.Unlock()

	if count != 10 {
		t.Errorf("expected 10 events processed, got %d", count)
	}
}

func TestEventFilter_Match(t *testing.T) {
	tests := []struct {
		name     string
		filter   *EventFilter
		event    *ContractEvent
		expected bool
	}{
		{
			name:     "empty filter matches all",
			filter:   &EventFilter{},
			event:    &ContractEvent{Contract: "0x1234", EventName: "Test"},
			expected: true,
		},
		{
			name:     "contract match",
			filter:   &EventFilter{Contracts: []string{"0x1234"}},
			event:    &ContractEvent{Contract: "0x1234", EventName: "Test"},
			expected: true,
		},
		{
			name:     "contract mismatch",
			filter:   &EventFilter{Contracts: []string{"0x5678"}},
			event:    &ContractEvent{Contract: "0x1234", EventName: "Test"},
			expected: false,
		},
		{
			name:     "event match",
			filter:   &EventFilter{EventNames: []string{"TestEvent"}},
			event:    &ContractEvent{Contract: "0x1234", EventName: "TestEvent"},
			expected: true,
		},
		{
			name:     "event mismatch",
			filter:   &EventFilter{EventNames: []string{"OtherEvent"}},
			event:    &ContractEvent{Contract: "0x1234", EventName: "TestEvent"},
			expected: false,
		},
		{
			name:     "both match",
			filter:   &EventFilter{Contracts: []string{"0x1234"}, EventNames: []string{"TestEvent"}},
			event:    &ContractEvent{Contract: "0x1234", EventName: "TestEvent"},
			expected: true,
		},
		{
			name:     "case insensitive contract",
			filter:   &EventFilter{Contracts: []string{"0xABCD"}},
			event:    &ContractEvent{Contract: "0xabcd", EventName: "Test"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.filter.Match(tt.event)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// Test helper

type testEventHandler struct {
	events    []string
	contracts []string
	callback  func(ctx context.Context, event *ContractEvent) error
}

func (h *testEventHandler) SupportedEvents() []string {
	return h.events
}

func (h *testEventHandler) SupportedContracts() []string {
	return h.contracts
}

func (h *testEventHandler) HandleEvent(ctx context.Context, event *ContractEvent) error {
	if h.callback != nil {
		return h.callback(ctx, event)
	}
	return nil
}
