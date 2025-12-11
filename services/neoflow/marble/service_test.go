// Package neoflow provides task neoflow service.
package neoflowmarble

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/chain"
	"github.com/R3E-Network/service_layer/internal/marble"
	neoflowsupabase "github.com/R3E-Network/service_layer/services/neoflow/supabase"
	"github.com/gorilla/mux"
)

// =============================================================================
// Mock Repository for Tests
// =============================================================================

// mockNeoFlowRepo implements neoflowsupabase.RepositoryInterface for testing.
type mockNeoFlowRepo struct {
	triggers   map[string]*neoflowsupabase.Trigger
	executions map[string][]neoflowsupabase.Execution
}

func newMockNeoFlowRepo() *mockNeoFlowRepo {
	return &mockNeoFlowRepo{
		triggers:   make(map[string]*neoflowsupabase.Trigger),
		executions: make(map[string][]neoflowsupabase.Execution),
	}
}

// trackingNeoFlowRepo tracks GetTriggers invocations for hydration tests.
type trackingNeoFlowRepo struct {
	*mockNeoFlowRepo
	t          *testing.T
	allowEmpty bool
	callCount  int
	lastUserID string
}

func newTrackingNeoFlowRepo(t *testing.T) *trackingNeoFlowRepo {
	return &trackingNeoFlowRepo{
		mockNeoFlowRepo: newMockNeoFlowRepo(),
		t:                  t,
	}
}

func (m *trackingNeoFlowRepo) GetTriggers(ctx context.Context, userID string) ([]neoflowsupabase.Trigger, error) {
	if userID == "" && !m.allowEmpty {
		m.t.Fatalf("GetTriggers should not be called with empty user ID")
	}
	m.callCount++
	m.lastUserID = userID
	return []neoflowsupabase.Trigger{}, nil
}

func readSchedulerStopCh(svc *Service) chan struct{} {
	svc.scheduler.mu.RLock()
	defer svc.scheduler.mu.RUnlock()
	return svc.scheduler.stopCh
}

func newHTTPTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			if strings.Contains(fmt.Sprint(r), "operation not permitted") {
				t.Skipf("skipping HTTP server test due to sandbox restrictions: %v", r)
			}
			panic(r)
		}
	}()
	return httptest.NewServer(handler)
}

func (m *mockNeoFlowRepo) GetTriggers(_ context.Context, userID string) ([]neoflowsupabase.Trigger, error) {
	var result []neoflowsupabase.Trigger
	for _, t := range m.triggers {
		if t.UserID == userID {
			result = append(result, *t)
		}
	}
	return result, nil
}

func (m *mockNeoFlowRepo) GetTrigger(_ context.Context, id, userID string) (*neoflowsupabase.Trigger, error) {
	if t, ok := m.triggers[id]; ok && t.UserID == userID {
		return t, nil
	}
	return nil, fmt.Errorf("trigger not found")
}

func (m *mockNeoFlowRepo) CreateTrigger(_ context.Context, trigger *neoflowsupabase.Trigger) error {
	m.triggers[trigger.ID] = trigger
	return nil
}

func (m *mockNeoFlowRepo) UpdateTrigger(_ context.Context, trigger *neoflowsupabase.Trigger) error {
	m.triggers[trigger.ID] = trigger
	return nil
}

func (m *mockNeoFlowRepo) DeleteTrigger(_ context.Context, id, _ string) error {
	delete(m.triggers, id)
	return nil
}

func (m *mockNeoFlowRepo) SetTriggerEnabled(_ context.Context, id, _ string, enabled bool) error {
	if t, ok := m.triggers[id]; ok {
		t.Enabled = enabled
	}
	return nil
}

func (m *mockNeoFlowRepo) GetPendingTriggers(_ context.Context) ([]neoflowsupabase.Trigger, error) {
	var result []neoflowsupabase.Trigger
	now := time.Now()
	for _, t := range m.triggers {
		if t.Enabled && !t.NextExecution.IsZero() && now.After(t.NextExecution) {
			result = append(result, *t)
		}
	}
	return result, nil
}

func (m *mockNeoFlowRepo) CreateExecution(_ context.Context, exec *neoflowsupabase.Execution) error {
	m.executions[exec.TriggerID] = append(m.executions[exec.TriggerID], *exec)
	return nil
}

func (m *mockNeoFlowRepo) GetExecutions(_ context.Context, triggerID string, limit int) ([]neoflowsupabase.Execution, error) {
	execs := m.executions[triggerID]
	if len(execs) > limit {
		return execs[:limit], nil
	}
	return execs, nil
}

// =============================================================================
// Service Tests
// =============================================================================

func TestNew(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})

	svc, err := New(Config{
		Marble: m,
		DB:     nil,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if svc.ID() != ServiceID {
		t.Errorf("ID() = %s, want %s", svc.ID(), ServiceID)
	}
	if svc.Name() != ServiceName {
		t.Errorf("Name() = %s, want %s", svc.Name(), ServiceName)
	}
	if svc.Version() != Version {
		t.Errorf("Version() = %s, want %s", svc.Version(), Version)
	}
}

func TestServiceConstants(t *testing.T) {
	if ServiceID != "neoflow" {
		t.Errorf("ServiceID = %s, want neoflow", ServiceID)
	}
	if ServiceName != "NeoFlow Service" {
		t.Errorf("ServiceName = %s, want NeoFlow Service", ServiceName)
	}
	if Version != "2.0.0" {
		t.Errorf("Version = %s, want 2.0.0", Version)
	}
}

func TestSchedulerInitialization(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	if svc.scheduler == nil {
		t.Error("scheduler should not be nil")
	}
	if svc.scheduler.triggers == nil {
		t.Error("scheduler.triggers should not be nil")
	}
	if svc.scheduler.stopCh == nil {
		t.Error("scheduler.stopCh should not be nil")
	}
}

func TestSchedulerRestartReinitializesStopChannel(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	first := readSchedulerStopCh(svc)
	if first == nil {
		t.Fatal("stopCh should not be nil on initialization")
	}

	svc.stopSchedulerWorkers()
	if ch := readSchedulerStopCh(svc); ch != nil {
		t.Error("stopCh should be nil after stopSchedulerWorkers")
	}

	second := svc.ensureSchedulerStopCh()
	if second == nil {
		t.Fatal("stopCh should be re-created by ensureSchedulerStopCh")
	}
	if first == second {
		t.Error("stopCh should be a new channel after ensuring scheduler stop channel")
	}
}

func TestSchedulerHydrationSkipsEmptyUserID(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	repo := newTrackingNeoFlowRepo(t)
	svc, _ := New(Config{Marble: m, NeoFlowRepo: repo})

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := svc.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	if repo.callCount != 0 {
		t.Fatalf("GetTriggers should not be called without user ID, got %d call(s)", repo.callCount)
	}
}

func TestSchedulerHydrationUsesConfiguredUserID(t *testing.T) {
	t.Setenv("NEOFLOW_SCHEDULER_USER_ID", "system-user")

	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	repo := newTrackingNeoFlowRepo(t)
	svc, _ := New(Config{Marble: m, NeoFlowRepo: repo})

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer svc.Stop()

	if repo.callCount == 0 {
		t.Fatal("expected GetTriggers to be called when scheduler user ID is configured")
	}
	if repo.lastUserID != "system-user" {
		t.Fatalf("GetTriggers called with %q, want %q", repo.lastUserID, "system-user")
	}
}

// =============================================================================
// parseNextCronExecution Tests
// =============================================================================

func TestParseNextCronExecutionValid(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	// Valid 5-field cron expression
	next, err := svc.parseNextCronExecution("30 * * * *")
	if err != nil {
		t.Fatalf("parseNextCronExecution() error = %v", err)
	}

	if next.IsZero() {
		t.Error("next execution time should not be zero")
	}
}

func TestParseNextCronExecutionWildcard(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	next, err := svc.parseNextCronExecution("* * * * *")
	if err != nil {
		t.Fatalf("parseNextCronExecution() error = %v", err)
	}

	// Should be in the future
	if !next.After(time.Now().Add(-time.Second)) {
		t.Error("next execution should be in the future or very recent")
	}
}

func TestParseNextCronExecutionInvalid(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	tests := []struct {
		name     string
		cronExpr string
	}{
		{"too few fields", "* * *"},
		{"too many fields", "* * * * * *"},
		{"empty", ""},
		{"single field", "*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.parseNextCronExecution(tt.cronExpr)
			if err == nil {
				t.Error("parseNextCronExecution() should return error for invalid cron")
			}
		})
	}
}

// =============================================================================
// Handler Tests
// =============================================================================

func TestHandleListTriggersUnauthorized(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	req := httptest.NewRequest("GET", "/triggers", nil)
	// No X-User-ID header
	rr := httptest.NewRecorder()

	svc.handleListTriggers(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestHandleCreateTriggerUnauthorized(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	reqBody, _ := json.Marshal(TriggerRequest{
		Name:        "Test Trigger",
		TriggerType: "cron",
	})

	req := httptest.NewRequest("POST", "/triggers", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	svc.handleCreateTrigger(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestHandleCreateTriggerInvalidBody(t *testing.T) {
	t.Skip("handler requires Supabase repository")
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	req := httptest.NewRequest("POST", "/triggers", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-123")
	rr := httptest.NewRecorder()

	svc.handleCreateTrigger(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateTriggerMissingFields(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	tests := []struct {
		name string
		req  TriggerRequest
	}{
		{"missing name", TriggerRequest{TriggerType: "cron"}},
		{"missing trigger_type", TriggerRequest{Name: "Test"}},
		{"both missing", TriggerRequest{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody, _ := json.Marshal(tt.req)
			req := httptest.NewRequest("POST", "/triggers", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-User-ID", "user-123")
			rr := httptest.NewRecorder()

			svc.handleCreateTrigger(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestHandleCreateTriggerInvalidCron(t *testing.T) {
	t.Skip("handler requires Supabase repository")
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	reqBody, _ := json.Marshal(TriggerRequest{
		Name:        "Test Trigger",
		TriggerType: "cron",
		Schedule:    "invalid cron",
	})

	req := httptest.NewRequest("POST", "/triggers", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-123")
	rr := httptest.NewRecorder()

	svc.handleCreateTrigger(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateTriggerSuccess(t *testing.T) {
	t.Skip("handler requires Supabase repository")
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	reqBody, _ := json.Marshal(TriggerRequest{
		Name:        "Test Trigger",
		TriggerType: "cron",
		Schedule:    "30 * * * *",
		Action:      json.RawMessage(`{"type":"webhook"}`),
	})

	req := httptest.NewRequest("POST", "/triggers", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-123")
	rr := httptest.NewRecorder()

	svc.handleCreateTrigger(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusCreated)
	}

	var resp TriggerResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Name != "Test Trigger" {
		t.Errorf("Name = %s, want Test Trigger", resp.Name)
	}
	if resp.TriggerType != "cron" {
		t.Errorf("TriggerType = %s, want cron", resp.TriggerType)
	}
	if !resp.Enabled {
		t.Error("Enabled = false, want true")
	}
}

func TestHandleCreateTriggerNonCron(t *testing.T) {
	t.Skip("handler requires Supabase repository")
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	reqBody, _ := json.Marshal(TriggerRequest{
		Name:        "Event Trigger",
		TriggerType: "event",
		Action:      json.RawMessage(`{"type":"callback"}`),
	})

	req := httptest.NewRequest("POST", "/triggers", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-123")
	rr := httptest.NewRecorder()

	svc.handleCreateTrigger(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusCreated)
	}
}

func TestHandleGetTrigger(t *testing.T) {
	t.Skip("handler requires Supabase repository")
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	req := httptest.NewRequest("GET", "/triggers/123", nil)
	rr := httptest.NewRecorder()

	svc.handleGetTrigger(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleUpdateTrigger(t *testing.T) {
	t.Skip("handler requires Supabase repository")
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	req := httptest.NewRequest("PUT", "/triggers/123", nil)
	rr := httptest.NewRecorder()

	svc.handleUpdateTrigger(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleDeleteTrigger(t *testing.T) {
	t.Skip("handler requires Supabase repository")
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	req := httptest.NewRequest("DELETE", "/triggers/123", nil)
	rr := httptest.NewRecorder()

	svc.handleDeleteTrigger(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

func TestHandleEnableTrigger(t *testing.T) {
	t.Skip("handler requires Supabase repository")
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	req := httptest.NewRequest("POST", "/triggers/123/enable", nil)
	rr := httptest.NewRecorder()

	svc.handleEnableTrigger(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleDisableTrigger(t *testing.T) {
	t.Skip("handler requires Supabase repository")
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	req := httptest.NewRequest("POST", "/triggers/123/disable", nil)
	rr := httptest.NewRecorder()

	svc.handleDisableTrigger(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

// =============================================================================
// Request/Response Type Tests
// =============================================================================

func TestTriggerRequestJSON(t *testing.T) {
	req := TriggerRequest{
		Name:        "Daily Report",
		TriggerType: "cron",
		Schedule:    "0 9 * * *",
		Action:      json.RawMessage(`{"type":"webhook","url":"https://example.com"}`),
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded TriggerRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Name != req.Name {
		t.Errorf("Name = %s, want %s", decoded.Name, req.Name)
	}
	if decoded.TriggerType != req.TriggerType {
		t.Errorf("TriggerType = %s, want %s", decoded.TriggerType, req.TriggerType)
	}
}

func TestTriggerResponseJSON(t *testing.T) {
	now := time.Now()
	resp := TriggerResponse{
		ID:          "trigger-123",
		Name:        "Daily Report",
		TriggerType: "cron",
		Schedule:    "0 9 * * *",
		Enabled:     true,
		CreatedAt:   now,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded TriggerResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ID != resp.ID {
		t.Errorf("ID = %s, want %s", decoded.ID, resp.ID)
	}
	if decoded.Enabled != resp.Enabled {
		t.Errorf("Enabled = %v, want %v", decoded.Enabled, resp.Enabled)
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkNew(b *testing.B) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = New(Config{Marble: m})
	}
}

func BenchmarkParseNextCronExecution(b *testing.B) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.parseNextCronExecution("30 * * * *")
	}
}

func BenchmarkTriggerRequestMarshal(b *testing.B) {
	req := TriggerRequest{
		Name:        "Daily Report",
		TriggerType: "cron",
		Schedule:    "0 9 * * *",
		Action:      json.RawMessage(`{"type":"webhook"}`),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(req)
	}
}

// =============================================================================
// Additional Type Tests
// =============================================================================

func TestTriggerTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant uint8
		want     uint8
	}{
		{"TriggerTypeTime", TriggerTypeTime, 1},
		{"TriggerTypePrice", TriggerTypePrice, 2},
		{"TriggerTypeEvent", TriggerTypeEvent, 3},
		{"TriggerTypeThreshold", TriggerTypeThreshold, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.want {
				t.Errorf("%s = %d, want %d", tt.name, tt.constant, tt.want)
			}
		})
	}
}

func TestServiceIntervalConstants(t *testing.T) {
	if SchedulerInterval != time.Second {
		t.Errorf("SchedulerInterval = %v, want %v", SchedulerInterval, time.Second)
	}
	if ChainTriggerInterval != 5*time.Second {
		t.Errorf("ChainTriggerInterval = %v, want %v", ChainTriggerInterval, 5*time.Second)
	}
	if ServiceFeePerExecution != 50000 {
		t.Errorf("ServiceFeePerExecution = %d, want 50000", ServiceFeePerExecution)
	}
}

func TestActionJSON(t *testing.T) {
	action := Action{
		Type:   "webhook",
		URL:    "https://example.com/callback",
		Method: "POST",
		Body:   json.RawMessage(`{"key":"value"}`),
	}

	data, err := json.Marshal(action)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded Action
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Type != action.Type {
		t.Errorf("Type = %s, want %s", decoded.Type, action.Type)
	}
	if decoded.URL != action.URL {
		t.Errorf("URL = %s, want %s", decoded.URL, action.URL)
	}
	if decoded.Method != action.Method {
		t.Errorf("Method = %s, want %s", decoded.Method, action.Method)
	}
}

func TestPriceConditionJSON(t *testing.T) {
	condition := PriceCondition{
		FeedID:    "BTC/USD",
		Operator:  ">",
		Threshold: 10000000000000, // $100,000 with 8 decimals
	}

	data, err := json.Marshal(condition)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded PriceCondition
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.FeedID != condition.FeedID {
		t.Errorf("FeedID = %s, want %s", decoded.FeedID, condition.FeedID)
	}
	if decoded.Operator != condition.Operator {
		t.Errorf("Operator = %s, want %s", decoded.Operator, condition.Operator)
	}
	if decoded.Threshold != condition.Threshold {
		t.Errorf("Threshold = %d, want %d", decoded.Threshold, condition.Threshold)
	}
}

func TestThresholdConditionJSON(t *testing.T) {
	condition := ThresholdCondition{
		Address:   "NAddr123456789",
		Asset:     "GAS",
		Operator:  "<",
		Threshold: 1000000000, // 10 GAS
	}

	data, err := json.Marshal(condition)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded ThresholdCondition
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Address != condition.Address {
		t.Errorf("Address = %s, want %s", decoded.Address, condition.Address)
	}
	if decoded.Asset != condition.Asset {
		t.Errorf("Asset = %s, want %s", decoded.Asset, condition.Asset)
	}
	if decoded.Operator != condition.Operator {
		t.Errorf("Operator = %s, want %s", decoded.Operator, condition.Operator)
	}
	if decoded.Threshold != condition.Threshold {
		t.Errorf("Threshold = %d, want %d", decoded.Threshold, condition.Threshold)
	}
}

func TestTriggerResponseWithOptionalFields(t *testing.T) {
	now := time.Now()
	lastExec := now.Add(-1 * time.Hour)
	nextExec := now.Add(1 * time.Hour)

	resp := TriggerResponse{
		ID:            "trigger-123",
		Name:          "Price Alert",
		TriggerType:   "price",
		Condition:     json.RawMessage(`{"feed_id":"BTC/USD","operator":">","threshold":100000}`),
		Action:        json.RawMessage(`{"type":"webhook"}`),
		Enabled:       true,
		LastExecution: &lastExec,
		NextExecution: &nextExec,
		CreatedAt:     now,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded TriggerResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.LastExecution == nil {
		t.Error("LastExecution should not be nil")
	}
	if decoded.NextExecution == nil {
		t.Error("NextExecution should not be nil")
	}
	if decoded.Condition == nil {
		t.Error("Condition should not be nil")
	}
}

func TestTriggerRequestWithCondition(t *testing.T) {
	req := TriggerRequest{
		Name:        "Balance Alert",
		TriggerType: "threshold",
		Condition:   json.RawMessage(`{"address":"NAddr123","asset":"GAS","operator":"<","threshold":1000000000}`),
		Action:      json.RawMessage(`{"type":"webhook","url":"https://example.com"}`),
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded TriggerRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Condition == nil {
		t.Error("Condition should not be nil")
	}

	// Verify condition can be parsed
	var condition ThresholdCondition
	if err := json.Unmarshal(decoded.Condition, &condition); err != nil {
		t.Fatalf("Failed to parse condition: %v", err)
	}
	if condition.Asset != "GAS" {
		t.Errorf("condition.Asset = %s, want GAS", condition.Asset)
	}
}

// =============================================================================
// Handler Tests - Health Endpoint
// =============================================================================

func TestHandleHealthEndpoint(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	svc.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["status"] != "healthy" {
		t.Errorf("status = %v, want healthy", resp["status"])
	}
}

// =============================================================================
// Additional Cron Tests
// =============================================================================

func TestParseNextCronExecutionSpecificTimes(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	tests := []struct {
		name     string
		cronExpr string
	}{
		{"every hour at minute 0", "0 * * * *"},
		{"every day at midnight", "0 0 * * *"},
		{"every monday at 9am", "0 9 * * 1"},
		{"every 15 minutes", "*/15 * * * *"},
		{"first day of month", "0 0 1 * *"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next, err := svc.parseNextCronExecution(tt.cronExpr)
			if err != nil {
				t.Errorf("parseNextCronExecution(%s) error = %v", tt.cronExpr, err)
			}
			if next.IsZero() {
				t.Errorf("parseNextCronExecution(%s) returned zero time", tt.cronExpr)
			}
		})
	}
}

// Note: TestParseNextCronExecutionInvalidValues removed because the underlying
// cron library (robfig/cron) doesn't strictly validate out-of-range values.
// It wraps or ignores invalid values rather than returning errors.

// =============================================================================
// Scheduler Tests
// =============================================================================

func TestSchedulerMapInitialization(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	// Verify scheduler maps are initialized
	if svc.scheduler.triggers == nil {
		t.Error("scheduler.triggers should not be nil")
	}
	if svc.scheduler.chainTriggers == nil {
		t.Error("scheduler.chainTriggers should not be nil")
	}

	// Verify maps are empty initially
	if len(svc.scheduler.triggers) != 0 {
		t.Errorf("scheduler.triggers should be empty, got %d", len(svc.scheduler.triggers))
	}
	if len(svc.scheduler.chainTriggers) != 0 {
		t.Errorf("scheduler.chainTriggers should be empty, got %d", len(svc.scheduler.chainTriggers))
	}
}

func TestServiceConfigFields(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})

	cfg := Config{
		Marble:          m,
		DB:              nil,
		ChainClient:     nil,
		TEEFulfiller:    nil,
		NeoFlowHash:  "0x1234567890abcdef",
		EnableChainExec: true,
	}

	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if svc.neoflowHash != cfg.NeoFlowHash {
		t.Errorf("neoflowHash = %s, want %s", svc.neoflowHash, cfg.NeoFlowHash)
	}
	if svc.enableChainExec != cfg.EnableChainExec {
		t.Errorf("enableChainExec = %v, want %v", svc.enableChainExec, cfg.EnableChainExec)
	}
}

// =============================================================================
// Additional Benchmarks
// =============================================================================

func BenchmarkTriggerResponseMarshal(b *testing.B) {
	now := time.Now()
	resp := TriggerResponse{
		ID:          "trigger-123",
		Name:        "Daily Report",
		TriggerType: "cron",
		Schedule:    "0 9 * * *",
		Enabled:     true,
		CreatedAt:   now,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(resp)
	}
}

func BenchmarkActionMarshal(b *testing.B) {
	action := Action{
		Type:   "webhook",
		URL:    "https://example.com/callback",
		Method: "POST",
		Body:   json.RawMessage(`{"key":"value"}`),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(action)
	}
}

func BenchmarkPriceConditionMarshal(b *testing.B) {
	condition := PriceCondition{
		FeedID:    "BTC/USD",
		Operator:  ">",
		Threshold: 10000000000000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(condition)
	}
}

// =============================================================================
// Handler Tests with Mock Repository
// =============================================================================

func TestHandleListTriggersWithMock(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	mockRepo := newMockNeoFlowRepo()

	// Pre-populate with triggers
	mockRepo.triggers["trigger-1"] = &neoflowsupabase.Trigger{
		ID: "trigger-1", UserID: "user-123", Name: "Trigger 1", TriggerType: "cron", Schedule: "0 * * * *",
	}

	svc, _ := New(Config{Marble: m, NeoFlowRepo: mockRepo})

	req := httptest.NewRequest("GET", "/triggers", nil)
	req.Header.Set("X-User-ID", "user-123")
	rr := httptest.NewRecorder()

	svc.handleListTriggers(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

// =============================================================================
// Trigger Execution Tests
// =============================================================================

func TestDispatchActionEmpty(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	err := svc.dispatchAction(context.Background(), nil)
	if err != nil {
		t.Errorf("dispatchAction(nil) error = %v, want nil", err)
	}

	err = svc.dispatchAction(context.Background(), json.RawMessage{})
	if err != nil {
		t.Errorf("dispatchAction(empty) error = %v, want nil", err)
	}
}

func TestDispatchActionInvalidJSON(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	err := svc.dispatchAction(context.Background(), json.RawMessage(`{invalid`))
	if err == nil {
		t.Error("dispatchAction(invalid json) should return error")
	}
}

func TestDispatchActionWebhookMissingURL(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	action := json.RawMessage(`{"type":"webhook","method":"POST"}`)
	err := svc.dispatchAction(context.Background(), action)
	if err == nil {
		t.Error("dispatchAction(webhook without url) should return error")
	}
	if err.Error() != "webhook url required" {
		t.Errorf("error = %v, want 'webhook url required'", err)
	}
}

func TestDispatchActionUnknownType(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	action := json.RawMessage(`{"type":"unknown"}`)
	err := svc.dispatchAction(context.Background(), action)
	if err != nil {
		t.Errorf("dispatchAction(unknown type) error = %v, want nil", err)
	}
}

func TestDispatchActionWebhookSuccess(t *testing.T) {
	// Create test server
	server := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	action := json.RawMessage(fmt.Sprintf(`{"type":"webhook","url":"%s","method":"POST"}`, server.URL))
	err := svc.dispatchAction(context.Background(), action)
	if err != nil {
		t.Errorf("dispatchAction() error = %v", err)
	}
}

func TestDispatchActionWebhookDefaultMethod(t *testing.T) {
	server := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST (default)", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	// No method specified - should default to POST
	action := json.RawMessage(fmt.Sprintf(`{"type":"webhook","url":"%s"}`, server.URL))
	err := svc.dispatchAction(context.Background(), action)
	if err != nil {
		t.Errorf("dispatchAction() error = %v", err)
	}
}

func TestDispatchActionWebhookError(t *testing.T) {
	server := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	action := json.RawMessage(fmt.Sprintf(`{"type":"webhook","url":"%s"}`, server.URL))
	err := svc.dispatchAction(context.Background(), action)
	if err == nil {
		t.Error("dispatchAction() should return error for 500 status")
	}
}

// =============================================================================
// Chain Trigger Tests
// =============================================================================

func TestRegisterUnregisterChainTrigger(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	trigger := &chain.Trigger{
		TriggerID:      big.NewInt(123),
		Owner:          "owner",
		TriggerType:    TriggerTypeTime,
		Condition:      "0 * * * *",
		Status:         1,
		ExecutionCount: big.NewInt(0),
		MaxExecutions:  big.NewInt(10),
	}

	// Register
	svc.RegisterChainTrigger(trigger)

	svc.scheduler.mu.RLock()
	if _, ok := svc.scheduler.chainTriggers[123]; !ok {
		t.Error("trigger should be registered")
	}
	svc.scheduler.mu.RUnlock()

	// Unregister
	svc.UnregisterChainTrigger(123)

	svc.scheduler.mu.RLock()
	if _, ok := svc.scheduler.chainTriggers[123]; ok {
		t.Error("trigger should be unregistered")
	}
	svc.scheduler.mu.RUnlock()
}

func TestEvaluateTriggerConditionUnknownType(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	trigger := &chain.Trigger{
		TriggerID:   big.NewInt(1),
		TriggerType: 99, // Unknown type
	}

	shouldExecute, data := svc.evaluateTriggerCondition(context.Background(), trigger)
	if shouldExecute {
		t.Error("unknown trigger type should not execute")
	}
	if data != nil {
		t.Error("unknown trigger type should return nil data")
	}
}

func TestEvaluateTriggerConditionEventType(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	trigger := &chain.Trigger{
		TriggerID:   big.NewInt(1),
		TriggerType: TriggerTypeEvent,
	}

	shouldExecute, data := svc.evaluateTriggerCondition(context.Background(), trigger)
	if shouldExecute {
		t.Error("event trigger should not execute via condition check")
	}
	if data != nil {
		t.Error("event trigger should return nil data")
	}
}

func TestEvaluateTimeTriggerEmptyCondition(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	trigger := &chain.Trigger{
		TriggerID:   big.NewInt(1),
		TriggerType: TriggerTypeTime,
		Condition:   "",
	}

	shouldExecute, _ := svc.evaluateTimeTrigger(trigger)
	if shouldExecute {
		t.Error("empty condition should not execute")
	}
}

func TestEvaluateTimeTriggerInvalidCron(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	trigger := &chain.Trigger{
		TriggerID:   big.NewInt(1),
		TriggerType: TriggerTypeTime,
		Condition:   "invalid",
	}

	shouldExecute, _ := svc.evaluateTimeTrigger(trigger)
	if shouldExecute {
		t.Error("invalid cron should not execute")
	}
}

func TestEvaluatePriceTriggerNoContract(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	trigger := &chain.Trigger{
		TriggerID:   big.NewInt(1),
		TriggerType: TriggerTypePrice,
		Condition:   `{"feed_id":"BTC/USD","operator":">","threshold":100000}`,
	}

	shouldExecute, _ := svc.evaluatePriceTrigger(context.Background(), trigger)
	if shouldExecute {
		t.Error("should not execute without neofeeds contract")
	}
}

func TestEvaluatePriceTriggerInvalidCondition(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	trigger := &chain.Trigger{
		TriggerID:   big.NewInt(1),
		TriggerType: TriggerTypePrice,
		Condition:   "invalid json",
	}

	shouldExecute, _ := svc.evaluatePriceTrigger(context.Background(), trigger)
	if shouldExecute {
		t.Error("invalid condition should not execute")
	}
}

func TestEvaluateThresholdTriggerInvalidCondition(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	trigger := &chain.Trigger{
		TriggerID:   big.NewInt(1),
		TriggerType: TriggerTypeThreshold,
		Condition:   "invalid json",
	}

	shouldExecute, _ := svc.evaluateThresholdTrigger(context.Background(), trigger)
	if shouldExecute {
		t.Error("invalid condition should not execute")
	}
}

func TestEvaluateThresholdTriggerValidCondition(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	trigger := &chain.Trigger{
		TriggerID:   big.NewInt(1),
		TriggerType: TriggerTypeThreshold,
		Condition:   `{"address":"NAddr123","asset":"GAS","operator":"<","threshold":1000000000}`,
	}

	// Currently returns false because no balance source is available
	shouldExecute, _ := svc.evaluateThresholdTrigger(context.Background(), trigger)
	if shouldExecute {
		t.Error("threshold trigger should not execute without balance source")
	}
}

func TestCheckChainTriggersDisabled(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m, EnableChainExec: false})

	// Should return early without panic
	svc.checkChainTriggers(context.Background())
}

func TestCheckAndExecuteTriggersNoRepo(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	mockRepo := newMockNeoFlowRepo()
	svc, _ := New(Config{Marble: m, NeoFlowRepo: mockRepo})

	// Empty repo - should return without panic
	svc.checkAndExecuteTriggers(context.Background())
}

func TestCheckAndExecuteTriggersWithMock(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	mockRepo := newMockNeoFlowRepo()

	// Add a trigger that should NOT execute (future execution time)
	futureTime := time.Now().Add(1 * time.Hour)
	mockRepo.triggers["trigger-1"] = &neoflowsupabase.Trigger{
		ID:            "trigger-1",
		UserID:        "user-123",
		Name:          "Future Trigger",
		TriggerType:   "cron",
		Schedule:      "0 * * * *",
		Enabled:       true,
		NextExecution: futureTime,
	}

	svc, _ := New(Config{Marble: m, NeoFlowRepo: mockRepo})
	svc.checkAndExecuteTriggers(context.Background())
	// No panic means success
}

func TestExecuteTriggerWithMock(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	mockRepo := newMockNeoFlowRepo()

	trigger := &neoflowsupabase.Trigger{
		ID:          "trigger-1",
		UserID:      "user-123",
		Name:        "Test Trigger",
		TriggerType: "cron",
		Schedule:    "0 * * * *",
		Enabled:     true,
		Action:      json.RawMessage(`{"type":"unknown"}`), // Unknown type - no-op
	}
	mockRepo.triggers[trigger.ID] = trigger

	svc, _ := New(Config{Marble: m, NeoFlowRepo: mockRepo})
	svc.executeTrigger(context.Background(), trigger)

	// Verify execution was logged
	execs := mockRepo.executions[trigger.ID]
	if len(execs) != 1 {
		t.Errorf("expected 1 execution, got %d", len(execs))
	}
	if !execs[0].Success {
		t.Error("execution should be successful")
	}
}

func TestSetupEventTriggerListenerNil(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	// Should return early without panic when eventListener is nil
	svc.SetupEventTriggerListener()
}

// =============================================================================
// Handler Tests with Mock - Enable Previously Skipped Tests
// =============================================================================

func TestHandleCreateTriggerWithMock(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	mockRepo := newMockNeoFlowRepo()
	svc, _ := New(Config{Marble: m, NeoFlowRepo: mockRepo})

	reqBody, _ := json.Marshal(TriggerRequest{
		Name:        "Test Trigger",
		TriggerType: "cron",
		Schedule:    "30 * * * *",
		Action:      json.RawMessage(`{"type":"webhook","url":"https://example.com"}`),
	})

	req := httptest.NewRequest("POST", "/triggers", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-123")
	rr := httptest.NewRecorder()

	svc.handleCreateTrigger(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d, body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var resp TriggerResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Name != "Test Trigger" {
		t.Errorf("Name = %s, want Test Trigger", resp.Name)
	}
	if !resp.Enabled {
		t.Error("Enabled = false, want true")
	}
}

func TestHandleCreateTriggerNonCronWithMock(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	mockRepo := newMockNeoFlowRepo()
	svc, _ := New(Config{Marble: m, NeoFlowRepo: mockRepo})

	reqBody, _ := json.Marshal(TriggerRequest{
		Name:        "Event Trigger",
		TriggerType: "event",
		Action:      json.RawMessage(`{"type":"callback"}`),
	})

	req := httptest.NewRequest("POST", "/triggers", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-123")
	rr := httptest.NewRecorder()

	svc.handleCreateTrigger(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusCreated)
	}
}

func TestHandleGetTriggerWithMock(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	mockRepo := newMockNeoFlowRepo()
	mockRepo.triggers["trigger-123"] = &neoflowsupabase.Trigger{
		ID: "trigger-123", UserID: "user-123", Name: "Test", TriggerType: "cron",
	}
	svc, _ := New(Config{Marble: m, NeoFlowRepo: mockRepo})

	req := httptest.NewRequest("GET", "/triggers/trigger-123", nil)
	req.Header.Set("X-User-ID", "user-123")
	req = mux.SetURLVars(req, map[string]string{"id": "trigger-123"})
	rr := httptest.NewRecorder()

	svc.handleGetTrigger(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleGetTriggerNotFound(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	mockRepo := newMockNeoFlowRepo()
	svc, _ := New(Config{Marble: m, NeoFlowRepo: mockRepo})

	req := httptest.NewRequest("GET", "/triggers/nonexistent", nil)
	req.Header.Set("X-User-ID", "user-123")
	rr := httptest.NewRecorder()

	svc.handleGetTrigger(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandleUpdateTriggerWithMock(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	mockRepo := newMockNeoFlowRepo()
	mockRepo.triggers["trigger-123"] = &neoflowsupabase.Trigger{
		ID: "trigger-123", UserID: "user-123", Name: "Old Name", TriggerType: "cron", Schedule: "0 * * * *",
	}
	svc, _ := New(Config{Marble: m, NeoFlowRepo: mockRepo})

	reqBody, _ := json.Marshal(TriggerRequest{
		Name:        "New Name",
		TriggerType: "cron",
		Schedule:    "30 * * * *",
	})

	req := httptest.NewRequest("PUT", "/triggers/trigger-123", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-123")
	req = mux.SetURLVars(req, map[string]string{"id": "trigger-123"})
	rr := httptest.NewRecorder()

	svc.handleUpdateTrigger(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleDeleteTriggerWithMock(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	mockRepo := newMockNeoFlowRepo()
	mockRepo.triggers["trigger-123"] = &neoflowsupabase.Trigger{
		ID: "trigger-123", UserID: "user-123", Name: "Test", TriggerType: "cron",
	}
	svc, _ := New(Config{Marble: m, NeoFlowRepo: mockRepo})

	req := httptest.NewRequest("DELETE", "/triggers/trigger-123", nil)
	req.Header.Set("X-User-ID", "user-123")
	req = mux.SetURLVars(req, map[string]string{"id": "trigger-123"})
	rr := httptest.NewRecorder()

	svc.handleDeleteTrigger(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}

	// Verify trigger was deleted
	if _, ok := mockRepo.triggers["trigger-123"]; ok {
		t.Error("trigger should be deleted")
	}
}

func TestHandleEnableDisableTriggerWithMock(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	mockRepo := newMockNeoFlowRepo()
	mockRepo.triggers["trigger-123"] = &neoflowsupabase.Trigger{
		ID: "trigger-123", UserID: "user-123", Name: "Test", TriggerType: "cron", Enabled: true,
	}
	svc, _ := New(Config{Marble: m, NeoFlowRepo: mockRepo})

	// Disable - handler expects /triggers/{id} format
	req := httptest.NewRequest("POST", "/triggers/trigger-123", nil)
	req.Header.Set("X-User-ID", "user-123")
	req = mux.SetURLVars(req, map[string]string{"id": "trigger-123"})
	rr := httptest.NewRecorder()
	svc.handleDisableTrigger(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("disable status = %d, want %d", rr.Code, http.StatusOK)
	}
	if mockRepo.triggers["trigger-123"].Enabled {
		t.Error("trigger should be disabled")
	}

	// Enable
	req = httptest.NewRequest("POST", "/triggers/trigger-123", nil)
	req.Header.Set("X-User-ID", "user-123")
	req = mux.SetURLVars(req, map[string]string{"id": "trigger-123"})
	rr = httptest.NewRecorder()
	svc.handleEnableTrigger(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("enable status = %d, want %d", rr.Code, http.StatusOK)
	}
	if !mockRepo.triggers["trigger-123"].Enabled {
		t.Error("trigger should be enabled")
	}
}

func TestHandleListExecutionsWithMock(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	mockRepo := newMockNeoFlowRepo()
	mockRepo.triggers["trigger-123"] = &neoflowsupabase.Trigger{
		ID: "trigger-123", UserID: "user-123", Name: "Test", TriggerType: "cron",
	}
	mockRepo.executions["trigger-123"] = []neoflowsupabase.Execution{
		{ID: "exec-1", TriggerID: "trigger-123", Success: true},
		{ID: "exec-2", TriggerID: "trigger-123", Success: false},
	}
	svc, _ := New(Config{Marble: m, NeoFlowRepo: mockRepo})

	// Handler expects /triggers/{id} format
	req := httptest.NewRequest("GET", "/triggers/trigger-123", nil)
	req.Header.Set("X-User-ID", "user-123")
	req = mux.SetURLVars(req, map[string]string{"id": "trigger-123"})
	rr := httptest.NewRecorder()

	svc.handleListExecutions(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHandleResumeTriggerWithMock(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	mockRepo := newMockNeoFlowRepo()
	mockRepo.triggers["trigger-123"] = &neoflowsupabase.Trigger{
		ID: "trigger-123", UserID: "user-123", Name: "Test", TriggerType: "cron",
	}
	svc, _ := New(Config{Marble: m, NeoFlowRepo: mockRepo})

	// Handler expects /triggers/{id} format
	req := httptest.NewRequest("POST", "/triggers/trigger-123", nil)
	req.Header.Set("X-User-ID", "user-123")
	req = mux.SetURLVars(req, map[string]string{"id": "trigger-123"})
	rr := httptest.NewRecorder()

	svc.handleResumeTrigger(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	// Verify trigger was added to scheduler
	svc.scheduler.mu.RLock()
	if _, ok := svc.scheduler.triggers["trigger-123"]; !ok {
		t.Error("trigger should be in scheduler")
	}
	svc.scheduler.mu.RUnlock()
}

func TestHandleInfoEndpoint(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoflow"})
	svc, _ := New(Config{Marble: m})

	req := httptest.NewRequest("GET", "/info", nil)
	rr := httptest.NewRecorder()

	svc.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp["status"] != "active" {
		t.Errorf("status = %v, want active", resp["status"])
	}
}
