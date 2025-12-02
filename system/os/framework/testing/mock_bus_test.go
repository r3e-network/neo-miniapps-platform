package testing

import (
	"context"
	"errors"
	"testing"

	"github.com/R3E-Network/service_layer/system/os/framework"
)

func TestMockBusClient_ImplementsInterface(t *testing.T) {
	var _ framework.BusClient = (*MockBusClient)(nil)
}

func TestMockBusClient_PublishEvent(t *testing.T) {
	mock := NewMockBusClient()

	err := mock.PublishEvent(context.Background(), "test.event", map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.EventCount() != 1 {
		t.Errorf("expected 1 event, got %d", mock.EventCount())
	}

	mock.AssertEventPublished(t, "test.event")
	mock.AssertEventNotPublished(t, "other.event")
}

func TestMockBusClient_PublishEventWithError(t *testing.T) {
	mock := NewMockBusClient()
	expectedErr := errors.New("publish failed")
	mock.SetPublishError(expectedErr)

	err := mock.PublishEvent(context.Background(), "test.event", nil)
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}

	// Event should still be recorded
	if mock.EventCount() != 1 {
		t.Errorf("expected 1 event even with error, got %d", mock.EventCount())
	}
}

func TestMockBusClient_PushData(t *testing.T) {
	mock := NewMockBusClient()

	err := mock.PushData(context.Background(), "topic.test", []byte("data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.DataCount() != 1 {
		t.Errorf("expected 1 data push, got %d", mock.DataCount())
	}

	mock.AssertDataPushed(t, "topic.test")
	mock.AssertDataNotPushed(t, "other.topic")
}

func TestMockBusClient_InvokeCompute(t *testing.T) {
	mock := NewMockBusClient()
	mock.SetInvokeResults([]framework.ComputeResult{
		{Module: "test-module", Result: "result1"},
	})

	results, err := mock.InvokeCompute(context.Background(), "payload")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if results[0].Module != "test-module" {
		t.Errorf("expected module 'test-module', got %q", results[0].Module)
	}

	mock.AssertComputeInvoked(t)
}

func TestMockBusClient_Reset(t *testing.T) {
	mock := NewMockBusClient()
	mock.SetPublishError(errors.New("error"))

	_ = mock.PublishEvent(context.Background(), "event", nil)
	_ = mock.PushData(context.Background(), "topic", nil)
	_, _ = mock.InvokeCompute(context.Background(), nil)

	mock.Reset()

	if mock.EventCount() != 0 {
		t.Error("expected 0 events after reset")
	}
	if mock.DataCount() != 0 {
		t.Error("expected 0 data pushes after reset")
	}
	if mock.ComputeCount() != 0 {
		t.Error("expected 0 computes after reset")
	}
	if mock.PublishError != nil {
		t.Error("expected nil publish error after reset")
	}
}

func TestMockBusClient_AssertNoOperations(t *testing.T) {
	mock := NewMockBusClient()
	mock.AssertNoOperations(t)
}

func TestMockBusClient_LastPublishedEvent(t *testing.T) {
	mock := NewMockBusClient()

	if mock.LastPublishedEvent() != nil {
		t.Error("expected nil when no events")
	}

	_ = mock.PublishEvent(context.Background(), "first", nil)
	_ = mock.PublishEvent(context.Background(), "second", nil)

	last := mock.LastPublishedEvent()
	if last == nil {
		t.Fatal("expected non-nil last event")
	}
	if last.Event != "second" {
		t.Errorf("expected 'second', got %q", last.Event)
	}
}

func TestMockBusClient_ConcurrentAccess(t *testing.T) {
	mock := NewMockBusClient()
	done := make(chan struct{})

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = mock.PublishEvent(context.Background(), "event", nil)
				_ = mock.PushData(context.Background(), "topic", nil)
				_, _ = mock.InvokeCompute(context.Background(), nil)
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if mock.EventCount() != 1000 {
		t.Errorf("expected 1000 events, got %d", mock.EventCount())
	}
}
