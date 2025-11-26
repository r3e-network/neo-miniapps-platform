package datastreams

import (
	"context"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/storage/memory"
	"github.com/R3E-Network/service_layer/internal/domain/account"
	domainds "github.com/R3E-Network/service_layer/internal/domain/datastreams"
	core "github.com/R3E-Network/service_layer/internal/services/core"
)

func TestService_CreateStreamAndList(t *testing.T) {
	store := memory.New()
	acct, err := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	svc := New(store, store, nil)
	stream, err := svc.CreateStream(context.Background(), domainds.Stream{
		AccountID: acct.ID,
		Name:      "Market",
		Symbol:    "ETH-USD",
	})
	if err != nil {
		t.Fatalf("create stream: %v", err)
	}
	if stream.Symbol != "ETH-USD" {
		t.Fatalf("expected upper symbol")
	}
	streams, err := svc.ListStreams(context.Background(), acct.ID)
	if err != nil {
		t.Fatalf("list streams: %v", err)
	}
	if len(streams) != 1 {
		t.Fatalf("expected one stream")
	}
}

func TestService_FrameLifecycle(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	stream, _ := svc.CreateStream(context.Background(), domainds.Stream{AccountID: acct.ID, Name: "Market", Symbol: "BTC"})

	frame, err := svc.CreateFrame(context.Background(), acct.ID, stream.ID, 1, map[string]any{"price": 100}, 50, domainds.FrameStatusOK, map[string]string{"env": "prod"})
	if err != nil {
		t.Fatalf("create frame: %v", err)
	}
	if frame.Sequence != 1 {
		t.Fatalf("sequence mismatch")
	}
	frames, err := svc.ListFrames(context.Background(), acct.ID, stream.ID, 10)
	if err != nil {
		t.Fatalf("list frames: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("expected one frame")
	}
	latest, err := svc.LatestFrame(context.Background(), acct.ID, stream.ID)
	if err != nil {
		t.Fatalf("latest frame: %v", err)
	}
	if latest.ID != frame.ID {
		t.Fatalf("latest mismatch")
	}
}

func TestService_UpdateStream(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	stream, _ := svc.CreateStream(context.Background(), domainds.Stream{AccountID: acct.ID, Name: "Market", Symbol: "BTC"})

	updated, err := svc.UpdateStream(context.Background(), domainds.Stream{ID: stream.ID, AccountID: acct.ID, Name: "Updated", Symbol: "ETH", Status: domainds.StreamStatusActive})
	if err != nil {
		t.Fatalf("update stream: %v", err)
	}
	if updated.Name != "Updated" {
		t.Fatalf("expected updated name")
	}
	if updated.Symbol != "ETH" {
		t.Fatalf("expected ETH symbol")
	}
}

func TestService_UpdateStreamOwnership(t *testing.T) {
	store := memory.New()
	acct1, _ := store.CreateAccount(context.Background(), account.Account{Owner: "one"})
	acct2, _ := store.CreateAccount(context.Background(), account.Account{Owner: "two"})
	svc := New(store, store, nil)
	stream, _ := svc.CreateStream(context.Background(), domainds.Stream{AccountID: acct1.ID, Name: "Market", Symbol: "BTC"})

	if _, err := svc.UpdateStream(context.Background(), domainds.Stream{ID: stream.ID, AccountID: acct2.ID, Name: "Hacked", Symbol: "HACKED"}); err == nil {
		t.Fatalf("expected ownership error")
	}
}

func TestService_GetStream(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	stream, _ := svc.CreateStream(context.Background(), domainds.Stream{AccountID: acct.ID, Name: "Market", Symbol: "BTC"})

	got, err := svc.GetStream(context.Background(), acct.ID, stream.ID)
	if err != nil {
		t.Fatalf("get stream: %v", err)
	}
	if got.ID != stream.ID {
		t.Fatalf("stream mismatch")
	}
}

func TestService_GetStreamOwnership(t *testing.T) {
	store := memory.New()
	acct1, _ := store.CreateAccount(context.Background(), account.Account{Owner: "one"})
	acct2, _ := store.CreateAccount(context.Background(), account.Account{Owner: "two"})
	svc := New(store, store, nil)
	stream, _ := svc.CreateStream(context.Background(), domainds.Stream{AccountID: acct1.ID, Name: "Market", Symbol: "BTC"})

	if _, err := svc.GetStream(context.Background(), acct2.ID, stream.ID); err == nil {
		t.Fatalf("expected ownership error")
	}
}

func TestService_StreamValidation(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)

	// Missing name
	if _, err := svc.CreateStream(context.Background(), domainds.Stream{AccountID: acct.ID, Symbol: "BTC"}); err == nil {
		t.Fatalf("expected name required error")
	}
	// Missing symbol
	if _, err := svc.CreateStream(context.Background(), domainds.Stream{AccountID: acct.ID, Name: "Market"}); err == nil {
		t.Fatalf("expected symbol required error")
	}
	// Invalid status
	if _, err := svc.CreateStream(context.Background(), domainds.Stream{AccountID: acct.ID, Name: "Market", Symbol: "BTC", Status: "invalid"}); err == nil {
		t.Fatalf("expected invalid status error")
	}
}

func TestService_FrameValidation(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "acct"})
	svc := New(store, store, nil)
	stream, _ := svc.CreateStream(context.Background(), domainds.Stream{AccountID: acct.ID, Name: "Market", Symbol: "BTC"})

	// Invalid sequence
	if _, err := svc.CreateFrame(context.Background(), acct.ID, stream.ID, 0, map[string]any{"price": 100}, 50, domainds.FrameStatusOK, nil); err == nil {
		t.Fatalf("expected sequence positive error")
	}

	// Negative latency gets corrected
	frame, err := svc.CreateFrame(context.Background(), acct.ID, stream.ID, 1, map[string]any{"price": 100}, -10, "", nil)
	if err != nil {
		t.Fatalf("create frame: %v", err)
	}
	if frame.LatencyMS != 0 {
		t.Fatalf("expected latency to be corrected to 0")
	}
}

func TestService_Push(t *testing.T) {
	svc := New(nil, nil, nil)
	svc.Start(context.Background())

	// Empty topic
	if err := svc.Push(context.Background(), "", map[string]any{"price": 100}); err == nil {
		t.Fatalf("expected stream ID required error")
	}

	// Invalid payload
	if err := svc.Push(context.Background(), "some-stream", "invalid"); err == nil {
		t.Fatalf("expected payload must be map error")
	}

	// Test Push with not ready service
	svc.Stop(context.Background())
	if err := svc.Push(context.Background(), "stream", map[string]any{"price": 100}); err == nil {
		t.Fatalf("expected not ready error")
	}
}

func TestService_Lifecycle(t *testing.T) {
	svc := New(nil, nil, nil)
	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := svc.Ready(context.Background()); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := svc.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if svc.Ready(context.Background()) == nil {
		t.Fatalf("expected not ready after stop")
	}
}

func TestService_Manifest(t *testing.T) {
	svc := New(nil, nil, nil)
	m := svc.Manifest()
	if m.Name != "datastreams" {
		t.Fatalf("expected name datastreams")
	}
}

func TestService_Descriptor(t *testing.T) {
	svc := New(nil, nil, nil)
	d := svc.Descriptor()
	if d.Name != "datastreams" {
		t.Fatalf("expected name datastreams")
	}
}

func TestService_WithObservationHooks(t *testing.T) {
	svc := New(nil, nil, nil)
	// With nil hooks
	svc.WithObservationHooks(core.ObservationHooks{})
	// With real hooks
	svc.WithObservationHooks(core.ObservationHooks{
		OnStart:    func(ctx context.Context, attrs map[string]string) {},
		OnComplete: func(ctx context.Context, attrs map[string]string, err error, dur time.Duration) {},
	})
}

func TestService_CreateStream_MissingAccount(t *testing.T) {
	store := memory.New()
	svc := New(store, store, nil)
	_, err := svc.CreateStream(context.Background(), domainds.Stream{AccountID: "nonexistent"})
	if err == nil {
		t.Fatalf("expected error for nonexistent account")
	}
}

func TestService_CreateStream_MissingName(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)
	_, err := svc.CreateStream(context.Background(), domainds.Stream{
		AccountID: acct.ID,
		Symbol:    "TEST",
	})
	if err == nil {
		t.Fatalf("expected error for missing name")
	}
}

func TestService_CreateStream_MissingSymbol(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)
	_, err := svc.CreateStream(context.Background(), domainds.Stream{
		AccountID: acct.ID,
		Name:      "test-stream",
	})
	if err == nil {
		t.Fatalf("expected error for missing symbol")
	}
}

func TestService_UpdateStream_NotFound(t *testing.T) {
	store := memory.New()
	svc := New(store, store, nil)
	_, err := svc.UpdateStream(context.Background(), domainds.Stream{ID: "nonexistent"})
	if err == nil {
		t.Fatalf("expected error for nonexistent stream")
	}
}

func TestService_UpdateStream_WrongAccount(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	acct2, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner2"})
	svc := New(store, store, nil)
	stream, _ := svc.CreateStream(context.Background(), domainds.Stream{
		AccountID:   acct.ID,
		Name:        "test-stream",
		Symbol:      "TEST",
		Description: "test",
	})
	_, err := svc.UpdateStream(context.Background(), domainds.Stream{
		ID:        stream.ID,
		AccountID: acct2.ID,
	})
	if err == nil {
		t.Fatalf("expected error for wrong account")
	}
}

func TestService_ListStreams_MissingAccount(t *testing.T) {
	store := memory.New()
	svc := New(store, store, nil)
	_, err := svc.ListStreams(context.Background(), "nonexistent")
	if err == nil {
		t.Fatalf("expected error for nonexistent account")
	}
}

func TestService_LatestFrame_NotFound(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)
	_, err := svc.LatestFrame(context.Background(), acct.ID, "nonexistent")
	if err == nil {
		t.Fatalf("expected error for nonexistent stream")
	}
}

func TestService_Push_MissingStreamID(t *testing.T) {
	store := memory.New()
	svc := New(store, store, nil)
	err := svc.Push(context.Background(), "", nil)
	if err == nil {
		t.Fatalf("expected error for missing stream_id")
	}
}

func TestService_ListFrames_MissingStreamID(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)
	_, err := svc.ListFrames(context.Background(), acct.ID, "", 10)
	if err == nil {
		t.Fatalf("expected error for missing stream_id")
	}
}

func TestService_CreateStream_WithHooks(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)
	svc.Start(context.Background())
	svc.WithObservationHooks(core.ObservationHooks{
		OnStart:    func(ctx context.Context, attrs map[string]string) {},
		OnComplete: func(ctx context.Context, attrs map[string]string, err error, dur time.Duration) {},
	})
	stream, err := svc.CreateStream(context.Background(), domainds.Stream{
		AccountID:   acct.ID,
		Name:        "test-stream",
		Symbol:      "TEST",
		Description: "test",
	})
	if err != nil {
		t.Fatalf("create stream: %v", err)
	}
	if stream.ID == "" {
		t.Fatalf("expected stream ID")
	}
}

func TestService_UpdateStream_WithHooks(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)
	svc.Start(context.Background())
	svc.WithObservationHooks(core.ObservationHooks{
		OnStart:    func(ctx context.Context, attrs map[string]string) {},
		OnComplete: func(ctx context.Context, attrs map[string]string, err error, dur time.Duration) {},
	})
	stream, _ := svc.CreateStream(context.Background(), domainds.Stream{
		AccountID:   acct.ID,
		Name:        "test-stream",
		Symbol:      "TEST",
		Description: "test",
	})
	updated, err := svc.UpdateStream(context.Background(), domainds.Stream{
		ID:          stream.ID,
		AccountID:   acct.ID,
		Name:        "updated-stream",
		Symbol:      "UPD",
		Description: "updated",
	})
	if err != nil {
		t.Fatalf("update stream: %v", err)
	}
	if updated.Name != "updated-stream" {
		t.Fatalf("expected updated name")
	}
}

func TestService_Push_NotStarted(t *testing.T) {
	store := memory.New()
	svc := New(store, store, nil)
	err := svc.Push(context.Background(), "stream-1", map[string]any{"value": 123})
	if err == nil {
		t.Fatalf("expected error when service not started")
	}
}

func TestService_Push_InvalidPayload(t *testing.T) {
	store := memory.New()
	svc := New(store, store, nil)
	svc.Start(context.Background())
	err := svc.Push(context.Background(), "stream-1", "not-a-map")
	if err == nil {
		t.Fatalf("expected error for invalid payload type")
	}
}
