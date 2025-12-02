package events

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRequestRouter_Creation(t *testing.T) {
	r := NewRequestRouter(RouterConfig{
		QueueSize:   100,
		WorkerCount: 2,
	})

	if r == nil {
		t.Fatal("expected router, got nil")
	}

	stats := r.Stats()
	if stats.QueueCapacity != 100 {
		t.Errorf("expected queue capacity 100, got %d", stats.QueueCapacity)
	}
}

func TestRequestRouter_RegisterHandler(t *testing.T) {
	r := NewRequestRouter(RouterConfig{})

	handler := &testServiceHandler{
		serviceType: ServiceOracle,
	}

	r.RegisterHandler(handler)

	stats := r.Stats()
	if stats.HandlersCount != 1 {
		t.Errorf("expected 1 handler, got %d", stats.HandlersCount)
	}
}

func TestRequestRouter_CreateRequest(t *testing.T) {
	r := NewRequestRouter(RouterConfig{})

	ctx := context.Background()
	req, err := r.CreateRequest(ctx, "account-123", ServiceOracle, map[string]any{
		"url": "https://api.example.com/data",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req == nil {
		t.Fatal("expected request, got nil")
	}

	if req.AccountID != "account-123" {
		t.Errorf("expected account_id 'account-123', got '%s'", req.AccountID)
	}

	if req.ServiceType != ServiceOracle {
		t.Errorf("expected service type 'oracle', got '%s'", req.ServiceType)
	}

	if req.Status != StatusPending {
		t.Errorf("expected status 'pending', got '%s'", req.Status)
	}

	if !strings.HasPrefix(req.ID, "req_") {
		t.Errorf("expected ID to start with 'req_', got '%s'", req.ID)
	}
}

func TestRequestRouter_CreateRequestWithOptions(t *testing.T) {
	r := NewRequestRouter(RouterConfig{})

	ctx := context.Background()
	req, err := r.CreateRequest(ctx, "account-123", ServiceVRF, map[string]any{
		"seed": "random-seed",
	},
		WithExternalID("ext-123"),
		WithServiceID("vrf-service-1"),
		WithFee(1000, "fee-123"),
		WithTxHash("0xabc123"),
		WithCallback("0xdef456"),
		WithMetadata("source", "test"),
		WithMaxAttempts(5),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.ExternalID != "ext-123" {
		t.Errorf("expected external_id 'ext-123', got '%s'", req.ExternalID)
	}

	if req.ServiceID != "vrf-service-1" {
		t.Errorf("expected service_id 'vrf-service-1', got '%s'", req.ServiceID)
	}

	if req.Fee != 1000 {
		t.Errorf("expected fee 1000, got %d", req.Fee)
	}

	if req.FeeID != "fee-123" {
		t.Errorf("expected fee_id 'fee-123', got '%s'", req.FeeID)
	}

	if req.TxHash != "0xabc123" {
		t.Errorf("expected tx_hash '0xabc123', got '%s'", req.TxHash)
	}

	if req.CallbackHash != "0xdef456" {
		t.Errorf("expected callback_hash '0xdef456', got '%s'", req.CallbackHash)
	}

	if req.Metadata["source"] != "test" {
		t.Errorf("expected metadata source 'test', got '%s'", req.Metadata["source"])
	}

	if req.MaxAttempts != 5 {
		t.Errorf("expected max_attempts 5, got %d", req.MaxAttempts)
	}
}

func TestRequestRouter_ProcessRequestSync(t *testing.T) {
	r := NewRequestRouter(RouterConfig{})

	processed := false
	handler := &testServiceHandler{
		serviceType: ServiceOracle,
		processFunc: func(ctx context.Context, req *Request) error {
			processed = true
			return nil
		},
	}

	r.RegisterHandler(handler)

	ctx := context.Background()
	req, _ := r.CreateRequest(ctx, "account-123", ServiceOracle, nil)

	err := r.ProcessRequestSync(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !processed {
		t.Error("expected handler to process request")
	}

	if req.Status != StatusRunning {
		t.Errorf("expected status 'running', got '%s'", req.Status)
	}

	if req.Attempts != 1 {
		t.Errorf("expected attempts 1, got %d", req.Attempts)
	}
}

func TestRequestRouter_ProcessRequestSync_NoHandler(t *testing.T) {
	r := NewRequestRouter(RouterConfig{})

	ctx := context.Background()
	req, _ := r.CreateRequest(ctx, "account-123", ServiceOracle, nil)

	err := r.ProcessRequestSync(ctx, req)
	if err == nil {
		t.Error("expected error for missing handler")
	}
}

func TestRequestRouter_StartStop(t *testing.T) {
	r := NewRequestRouter(RouterConfig{
		QueueSize:   10,
		WorkerCount: 2,
	})

	ctx := context.Background()
	if err := r.Start(ctx); err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	// Should be running
	req := &Request{
		ID:          "test-req",
		ServiceType: ServiceOracle,
		Status:      StatusPending,
	}
	if err := r.SubmitRequest(req); err != nil {
		t.Errorf("submit failed while running: %v", err)
	}

	r.Stop()

	// Should fail after stop
	if err := r.SubmitRequest(req); err == nil {
		t.Error("expected error after stop")
	}
}

func TestRequestRouter_AsyncProcessing(t *testing.T) {
	store := newMemoryRequestStore()
	r := NewRequestRouter(RouterConfig{
		Store:       store,
		QueueSize:   100,
		WorkerCount: 2,
	})

	processedCount := 0
	handler := &testServiceHandler{
		serviceType: ServiceOracle,
		processFunc: func(ctx context.Context, req *Request) error {
			processedCount++
			return nil
		},
	}

	r.RegisterHandler(handler)

	ctx := context.Background()
	r.Start(ctx)
	defer r.Stop()

	// Create and submit multiple requests
	for i := 0; i < 5; i++ {
		req, _ := r.CreateRequest(ctx, "account-123", ServiceOracle, nil)
		r.SubmitRequest(req)
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	if processedCount != 5 {
		t.Errorf("expected 5 requests processed, got %d", processedCount)
	}
}

func TestRequestStatus_Values(t *testing.T) {
	statuses := []RequestStatus{
		StatusPending,
		StatusRunning,
		StatusSucceeded,
		StatusFailed,
		StatusCancelled,
	}

	expected := []string{"pending", "running", "succeeded", "failed", "cancelled"}

	for i, s := range statuses {
		if string(s) != expected[i] {
			t.Errorf("expected status '%s', got '%s'", expected[i], s)
		}
	}
}

func TestServiceType_Values(t *testing.T) {
	types := []ServiceType{
		ServiceOracle,
		ServiceVRF,
		ServiceDataFeeds,
		ServiceAutomation,
		ServiceSecrets,
		ServiceFunctions,
		ServiceCCIP,
	}

	expected := []string{"oracle", "vrf", "datafeeds", "automation", "secrets", "functions", "ccip"}

	for i, st := range types {
		if string(st) != expected[i] {
			t.Errorf("expected type '%s', got '%s'", expected[i], st)
		}
	}
}

// Test helpers

type testServiceHandler struct {
	serviceType ServiceType
	processFunc func(ctx context.Context, req *Request) error
	fulfillFunc func(ctx context.Context, req *Request, result map[string]any) error
}

func (h *testServiceHandler) ServiceType() ServiceType {
	return h.serviceType
}

func (h *testServiceHandler) ProcessRequest(ctx context.Context, req *Request) error {
	if h.processFunc != nil {
		return h.processFunc(ctx, req)
	}
	return nil
}

func (h *testServiceHandler) FulfillRequest(ctx context.Context, req *Request, result map[string]any) error {
	if h.fulfillFunc != nil {
		return h.fulfillFunc(ctx, req, result)
	}
	return nil
}

// In-memory request store for testing
type memoryRequestStore struct {
	requests   map[string]*Request
	byExternal map[string]*Request
}

func newMemoryRequestStore() *memoryRequestStore {
	return &memoryRequestStore{
		requests:   make(map[string]*Request),
		byExternal: make(map[string]*Request),
	}
}

func (s *memoryRequestStore) Create(ctx context.Context, req *Request) error {
	s.requests[req.ID] = req
	if req.ExternalID != "" {
		s.byExternal[req.ExternalID] = req
	}
	return nil
}

func (s *memoryRequestStore) Get(ctx context.Context, id string) (*Request, error) {
	if req, ok := s.requests[id]; ok {
		return req, nil
	}
	return nil, nil
}

func (s *memoryRequestStore) GetByExternalID(ctx context.Context, externalID string) (*Request, error) {
	if req, ok := s.byExternal[externalID]; ok {
		return req, nil
	}
	return nil, nil
}

func (s *memoryRequestStore) Update(ctx context.Context, req *Request) error {
	s.requests[req.ID] = req
	return nil
}

func (s *memoryRequestStore) List(ctx context.Context, accountID string, serviceType ServiceType, status RequestStatus, limit int) ([]*Request, error) {
	var result []*Request
	for _, req := range s.requests {
		if accountID != "" && req.AccountID != accountID {
			continue
		}
		if serviceType != "" && req.ServiceType != serviceType {
			continue
		}
		if status != "" && req.Status != status {
			continue
		}
		result = append(result, req)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (s *memoryRequestStore) ListPending(ctx context.Context, serviceType ServiceType, limit int) ([]*Request, error) {
	return s.List(ctx, "", serviceType, StatusPending, limit)
}
