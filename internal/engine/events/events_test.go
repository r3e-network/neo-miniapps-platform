package events

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/engine/state"
)

func TestRingBuffer_Log(t *testing.T) {
	rb := NewRingBuffer(10)

	e := Event{
		Type:    EventModuleStarted,
		Module:  "test-module",
		Message: "test message",
	}

	rb.Log(e)

	if rb.Count() != 1 {
		t.Errorf("Count() = %d, want 1", rb.Count())
	}

	recent := rb.Recent(1)
	if len(recent) != 1 {
		t.Fatalf("Recent(1) len = %d, want 1", len(recent))
	}

	if recent[0].Module != "test-module" {
		t.Errorf("Module = %q, want 'test-module'", recent[0].Module)
	}
	if recent[0].ID == "" {
		t.Error("ID should be auto-generated")
	}
	if recent[0].Timestamp.IsZero() {
		t.Error("Timestamp should be auto-set")
	}
}

func TestRingBuffer_Overflow(t *testing.T) {
	rb := NewRingBuffer(5)

	// Fill beyond capacity
	for i := 0; i < 10; i++ {
		rb.Log(Event{
			Type:    EventModuleStarted,
			Message: string(rune('A' + i)),
		})
	}

	if rb.Count() != 5 {
		t.Errorf("Count() = %d, want 5 (capped)", rb.Count())
	}

	recent := rb.Recent(5)
	// Should have F, G, H, I, J (most recent)
	if len(recent) != 5 {
		t.Fatalf("Recent(5) len = %d, want 5", len(recent))
	}

	// Most recent first
	if recent[0].Message != "J" {
		t.Errorf("Most recent message = %q, want 'J'", recent[0].Message)
	}
	if recent[4].Message != "F" {
		t.Errorf("Oldest message = %q, want 'F'", recent[4].Message)
	}
}

func TestRingBuffer_Recent(t *testing.T) {
	rb := NewRingBuffer(10)

	for i := 0; i < 5; i++ {
		rb.Log(Event{Type: EventModuleStarted, Message: string(rune('A' + i))})
	}

	t.Run("request more than available", func(t *testing.T) {
		recent := rb.Recent(100)
		if len(recent) != 5 {
			t.Errorf("len = %d, want 5", len(recent))
		}
	})

	t.Run("request zero", func(t *testing.T) {
		recent := rb.Recent(0)
		if recent != nil {
			t.Error("Recent(0) should return nil")
		}
	})

	t.Run("request negative", func(t *testing.T) {
		recent := rb.Recent(-1)
		if recent != nil {
			t.Error("Recent(-1) should return nil")
		}
	})
}

func TestRingBuffer_RecentByModule(t *testing.T) {
	rb := NewRingBuffer(100)

	rb.Log(Event{Type: EventModuleStarted, Module: "module-a"})
	rb.Log(Event{Type: EventModuleStarted, Module: "module-b"})
	rb.Log(Event{Type: EventModuleStopped, Module: "module-a"})
	rb.Log(Event{Type: EventModuleStopped, Module: "module-b"})
	rb.Log(Event{Type: EventReadinessChanged, Module: "module-a"})

	recent := rb.RecentByModule("module-a", 10)
	if len(recent) != 3 {
		t.Errorf("len = %d, want 3", len(recent))
	}

	for _, e := range recent {
		if e.Module != "module-a" {
			t.Errorf("Module = %q, want 'module-a'", e.Module)
		}
	}
}

func TestRingBuffer_RecentByType(t *testing.T) {
	rb := NewRingBuffer(100)

	rb.Log(Event{Type: EventModuleStarted, Module: "a"})
	rb.Log(Event{Type: EventModuleStopped, Module: "a"})
	rb.Log(Event{Type: EventModuleStarted, Module: "b"})
	rb.Log(Event{Type: EventReadinessChanged, Module: "a"})

	recent := rb.RecentByType(EventModuleStarted, 10)
	if len(recent) != 2 {
		t.Errorf("len = %d, want 2", len(recent))
	}

	for _, e := range recent {
		if e.Type != EventModuleStarted {
			t.Errorf("Type = %v, want EventModuleStarted", e.Type)
		}
	}
}

func TestRingBuffer_Subscribe(t *testing.T) {
	rb := NewRingBuffer(10)

	var received []Event
	var mu sync.Mutex

	unsubscribe := rb.Subscribe(func(e Event) {
		mu.Lock()
		received = append(received, e)
		mu.Unlock()
	})

	rb.Log(Event{Type: EventModuleStarted, Module: "test"})
	rb.Log(Event{Type: EventModuleStopped, Module: "test"})

	// Give handlers time to run
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if len(received) != 2 {
		t.Errorf("received %d events, want 2", len(received))
	}
	mu.Unlock()

	// Unsubscribe
	unsubscribe()

	rb.Log(Event{Type: EventReadinessChanged, Module: "test"})
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if len(received) != 2 {
		t.Errorf("received %d events after unsubscribe, want 2", len(received))
	}
	mu.Unlock()
}

func TestRingBuffer_SubscribeFiltered(t *testing.T) {
	rb := NewRingBuffer(10)

	var received []Event
	var mu sync.Mutex

	filter := func(e Event) bool {
		return e.Type == EventModuleStarted
	}

	rb.SubscribeFiltered(filter, func(e Event) {
		mu.Lock()
		received = append(received, e)
		mu.Unlock()
	})

	rb.Log(Event{Type: EventModuleStarted, Module: "a"})
	rb.Log(Event{Type: EventModuleStopped, Module: "a"})
	rb.Log(Event{Type: EventModuleStarted, Module: "b"})

	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if len(received) != 2 {
		t.Errorf("received %d events, want 2 (only EventModuleStarted)", len(received))
	}
	mu.Unlock()
}

func TestRingBuffer_Clear(t *testing.T) {
	rb := NewRingBuffer(10)

	rb.Log(Event{Type: EventModuleStarted})
	rb.Log(Event{Type: EventModuleStopped})

	if rb.Count() != 2 {
		t.Errorf("Count() before clear = %d, want 2", rb.Count())
	}

	rb.Clear()

	if rb.Count() != 0 {
		t.Errorf("Count() after clear = %d, want 0", rb.Count())
	}
}

func TestRingBuffer_Concurrent(t *testing.T) {
	rb := NewRingBuffer(1000)

	var wg sync.WaitGroup
	var receivedCount atomic.Int64

	// Subscribe before concurrent logging
	rb.Subscribe(func(e Event) {
		receivedCount.Add(1)
	})

	// Concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				rb.Log(Event{
					Type:   EventModuleStarted,
					Module: string(rune('A' + id)),
				})
			}
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = rb.Recent(10)
				_ = rb.RecentByType(EventModuleStarted, 5)
				time.Sleep(time.Microsecond)
			}
		}()
	}

	wg.Wait()

	// Should have logged 1000 events
	if rb.Count() != 1000 {
		t.Errorf("Count() = %d, want 1000", rb.Count())
	}

	// Handler should have been called 1000 times
	if receivedCount.Load() != 1000 {
		t.Errorf("receivedCount = %d, want 1000", receivedCount.Load())
	}
}

func TestLogWithContext(t *testing.T) {
	rb := NewRingBuffer(10)

	ctx := context.Background()
	ctx = WithTraceID(ctx, "trace-123")
	ctx = WithRequestID(ctx, "req-456")

	rb.LogWithContext(ctx, Event{Type: EventModuleStarted})

	recent := rb.Recent(1)
	if len(recent) != 1 {
		t.Fatal("expected 1 event")
	}

	if recent[0].TraceID != "trace-123" {
		t.Errorf("TraceID = %q, want 'trace-123'", recent[0].TraceID)
	}
	if recent[0].RequestID != "req-456" {
		t.Errorf("RequestID = %q, want 'req-456'", recent[0].RequestID)
	}
}

func TestEventBuilder(t *testing.T) {
	event := NewEvent(EventModuleStarted).
		Module("test-module").
		Domain("test-domain").
		Component("lifecycle").
		Status(state.StatusRunning).
		Readiness(state.ReadinessReady).
		Severity(SeverityInfo).
		Message("module started successfully").
		Duration(100 * time.Millisecond).
		Metadata("version", "1.0.0").
		TraceID("trace-123").
		Build()

	if event.Type != EventModuleStarted {
		t.Errorf("Type = %v, want EventModuleStarted", event.Type)
	}
	if event.Module != "test-module" {
		t.Errorf("Module = %q, want 'test-module'", event.Module)
	}
	if event.Domain != "test-domain" {
		t.Errorf("Domain = %q, want 'test-domain'", event.Domain)
	}
	if event.Component != "lifecycle" {
		t.Errorf("Component = %q, want 'lifecycle'", event.Component)
	}
	if event.Status != state.StatusRunning {
		t.Errorf("Status = %v, want StatusRunning", event.Status)
	}
	if event.Readiness != state.ReadinessReady {
		t.Errorf("Readiness = %v, want ReadinessReady", event.Readiness)
	}
	if event.Severity != SeverityInfo {
		t.Errorf("Severity = %v, want SeverityInfo", event.Severity)
	}
	if event.Message != "module started successfully" {
		t.Errorf("Message = %q, want 'module started successfully'", event.Message)
	}
	if event.Duration != 100*time.Millisecond {
		t.Errorf("Duration = %v, want 100ms", event.Duration)
	}
	if event.Metadata["version"] != "1.0.0" {
		t.Errorf("Metadata[version] = %q, want '1.0.0'", event.Metadata["version"])
	}
	if event.TraceID != "trace-123" {
		t.Errorf("TraceID = %q, want 'trace-123'", event.TraceID)
	}
	if event.ID == "" {
		t.Error("ID should be auto-generated")
	}
}

func TestEventBuilder_ErrorFrom(t *testing.T) {
	t.Run("with error", func(t *testing.T) {
		event := NewEvent(EventModuleStartFailed).
			ErrorFrom(context.DeadlineExceeded).
			Build()

		if event.Error != context.DeadlineExceeded.Error() {
			t.Errorf("Error = %q, want %q", event.Error, context.DeadlineExceeded.Error())
		}
		if event.Severity != SeverityError {
			t.Errorf("Severity = %v, want SeverityError", event.Severity)
		}
	})

	t.Run("with nil error", func(t *testing.T) {
		event := NewEvent(EventModuleStarted).
			ErrorFrom(nil).
			Build()

		if event.Error != "" {
			t.Errorf("Error = %q, want empty", event.Error)
		}
	})
}

func TestEventBuilder_LogTo(t *testing.T) {
	rb := NewRingBuffer(10)

	NewEvent(EventModuleStarted).
		Module("test").
		Message("hello").
		LogTo(rb)

	if rb.Count() != 1 {
		t.Errorf("Count() = %d, want 1", rb.Count())
	}
}

func TestNoOpLogger(t *testing.T) {
	var logger NoOpLogger

	// Should not panic
	logger.Log(Event{})
	logger.LogWithContext(context.Background(), Event{})
	unsubscribe := logger.Subscribe(func(e Event) {})
	unsubscribe()
	_ = logger.Recent(10)
	_ = logger.RecentByModule("test", 10)
	_ = logger.RecentByType(EventModuleStarted, 10)
}

func TestEvent_String(t *testing.T) {
	event := Event{
		Type:    EventModuleStarted,
		Module:  "test",
		Message: "hello",
	}

	str := event.String()
	if str == "" {
		t.Error("String() should not be empty")
	}
	// Should be valid JSON
	if str[0] != '{' {
		t.Error("String() should return JSON")
	}
}
