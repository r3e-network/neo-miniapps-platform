// Package supabase provides NeoFlow-specific database operations.
package supabase

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/testutil"
)

// =============================================================================
// Test Helpers
// =============================================================================

func newTestRepository(t *testing.T, handler http.HandlerFunc) (*Repository, *httptest.Server) {
	t.Helper()
	server := testutil.NewHTTPTestServer(t, handler)
	client, err := database.NewClient(database.Config{
		URL:        server.URL,
		ServiceKey: "test-api-key",
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	baseRepo := database.NewRepository(client)
	return NewRepository(baseRepo), server
}

// =============================================================================
// NewRepository Tests
// =============================================================================

func TestNewRepository(t *testing.T) {
	client, _ := database.NewClient(database.Config{URL: "http://localhost", ServiceKey: "test"})
	baseRepo := database.NewRepository(client)
	repo := NewRepository(baseRepo)

	if repo == nil {
		t.Fatal("NewRepository() returned nil")
	}
}

// =============================================================================
// GetTriggers Tests
// =============================================================================

func TestGetTriggers_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Trigger{
			{ID: "t1", UserID: "user-123", Name: "trigger1"},
			{ID: "t2", UserID: "user-123", Name: "trigger2"},
		})
	}
	repo, server := newTestRepository(t, handler)
	defer server.Close()

	triggers, err := repo.GetTriggers(context.Background(), "user-123")
	if err != nil {
		t.Fatalf("GetTriggers() error = %v", err)
	}
	if len(triggers) != 2 {
		t.Errorf("len(triggers) = %d, want 2", len(triggers))
	}
}

func TestGetTriggers_EmptyUserID(t *testing.T) {
	repo := &Repository{}
	_, err := repo.GetTriggers(context.Background(), "")
	if err == nil {
		t.Error("GetTriggers('') should return error")
	}
}

// =============================================================================
// GetTrigger Tests
// =============================================================================

func TestGetTrigger_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Trigger{{ID: "t1", UserID: "user-123", Name: "trigger1"}})
	}
	repo, server := newTestRepository(t, handler)
	defer server.Close()

	trigger, err := repo.GetTrigger(context.Background(), "t1", "user-123")
	if err != nil {
		t.Fatalf("GetTrigger() error = %v", err)
	}
	if trigger.ID != "t1" {
		t.Errorf("ID = %s, want t1", trigger.ID)
	}
}

func TestGetTrigger_EmptyParams(t *testing.T) {
	repo := &Repository{}
	_, err := repo.GetTrigger(context.Background(), "", "user-123")
	if err == nil {
		t.Error("GetTrigger('', user) should return error")
	}
	_, err = repo.GetTrigger(context.Background(), "t1", "")
	if err == nil {
		t.Error("GetTrigger(id, '') should return error")
	}
}

func TestGetTrigger_NotFound(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Trigger{})
	}
	repo, server := newTestRepository(t, handler)
	defer server.Close()

	_, err := repo.GetTrigger(context.Background(), "nonexistent", "user-123")
	if err == nil {
		t.Error("GetTrigger() should return error for not found")
	}
}

// =============================================================================
// CreateTrigger Tests
// =============================================================================

func TestCreateTrigger_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Trigger{{ID: "t1", UserID: "user-123"}})
	}
	repo, server := newTestRepository(t, handler)
	defer server.Close()

	trigger := &Trigger{UserID: "user-123", Name: "test"}
	err := repo.CreateTrigger(context.Background(), trigger)
	if err != nil {
		t.Fatalf("CreateTrigger() error = %v", err)
	}
	if trigger.ID != "t1" {
		t.Errorf("ID = %s, want t1", trigger.ID)
	}
}

func TestCreateTrigger_NilTrigger(t *testing.T) {
	repo := &Repository{}
	err := repo.CreateTrigger(context.Background(), nil)
	if err == nil {
		t.Error("CreateTrigger(nil) should return error")
	}
}

func TestCreateTrigger_EmptyUserID(t *testing.T) {
	repo := &Repository{}
	err := repo.CreateTrigger(context.Background(), &Trigger{})
	if err == nil {
		t.Error("CreateTrigger() with empty user_id should return error")
	}
}

// =============================================================================
// UpdateTrigger Tests
// =============================================================================

func TestUpdateTrigger_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
	repo, server := newTestRepository(t, handler)
	defer server.Close()

	err := repo.UpdateTrigger(context.Background(), &Trigger{ID: "t1", UserID: "user-123"})
	if err != nil {
		t.Fatalf("UpdateTrigger() error = %v", err)
	}
}

func TestUpdateTrigger_NilTrigger(t *testing.T) {
	repo := &Repository{}
	err := repo.UpdateTrigger(context.Background(), nil)
	if err == nil {
		t.Error("UpdateTrigger(nil) should return error")
	}
}

func TestUpdateTrigger_EmptyParams(t *testing.T) {
	repo := &Repository{}
	err := repo.UpdateTrigger(context.Background(), &Trigger{})
	if err == nil {
		t.Error("UpdateTrigger() with empty id/user_id should return error")
	}
}

// =============================================================================
// DeleteTrigger Tests
// =============================================================================

func TestDeleteTrigger_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}
	repo, server := newTestRepository(t, handler)
	defer server.Close()

	err := repo.DeleteTrigger(context.Background(), "t1", "user-123")
	if err != nil {
		t.Fatalf("DeleteTrigger() error = %v", err)
	}
}

func TestDeleteTrigger_EmptyParams(t *testing.T) {
	repo := &Repository{}
	err := repo.DeleteTrigger(context.Background(), "", "user-123")
	if err == nil {
		t.Error("DeleteTrigger('', user) should return error")
	}
	err = repo.DeleteTrigger(context.Background(), "t1", "")
	if err == nil {
		t.Error("DeleteTrigger(id, '') should return error")
	}
}

// =============================================================================
// SetTriggerEnabled Tests
// =============================================================================

func TestSetTriggerEnabled_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
	repo, server := newTestRepository(t, handler)
	defer server.Close()

	err := repo.SetTriggerEnabled(context.Background(), "t1", "user-123", true)
	if err != nil {
		t.Fatalf("SetTriggerEnabled() error = %v", err)
	}

	err = repo.SetTriggerEnabled(context.Background(), "t1", "user-123", false)
	if err != nil {
		t.Fatalf("SetTriggerEnabled(false) error = %v", err)
	}
}

func TestSetTriggerEnabled_EmptyParams(t *testing.T) {
	repo := &Repository{}
	err := repo.SetTriggerEnabled(context.Background(), "", "user-123", true)
	if err == nil {
		t.Error("SetTriggerEnabled('', user, true) should return error")
	}
}

// =============================================================================
// GetPendingTriggers Tests
// =============================================================================

func TestGetPendingTriggers_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Trigger{
			{ID: "t1", Enabled: true},
			{ID: "t2", Enabled: true},
		})
	}
	repo, server := newTestRepository(t, handler)
	defer server.Close()

	triggers, err := repo.GetPendingTriggers(context.Background())
	if err != nil {
		t.Fatalf("GetPendingTriggers() error = %v", err)
	}
	if len(triggers) != 2 {
		t.Errorf("len(triggers) = %d, want 2", len(triggers))
	}
}

// =============================================================================
// CreateExecution Tests
// =============================================================================

func TestCreateExecution_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Execution{{ID: "e1", TriggerID: "t1"}})
	}
	repo, server := newTestRepository(t, handler)
	defer server.Close()

	exec := &Execution{TriggerID: "t1", Success: true}
	err := repo.CreateExecution(context.Background(), exec)
	if err != nil {
		t.Fatalf("CreateExecution() error = %v", err)
	}
	if exec.ID != "e1" {
		t.Errorf("ID = %s, want e1", exec.ID)
	}
}

func TestCreateExecution_NilExecution(t *testing.T) {
	repo := &Repository{}
	err := repo.CreateExecution(context.Background(), nil)
	if err == nil {
		t.Error("CreateExecution(nil) should return error")
	}
}

func TestCreateExecution_EmptyTriggerID(t *testing.T) {
	repo := &Repository{}
	err := repo.CreateExecution(context.Background(), &Execution{})
	if err == nil {
		t.Error("CreateExecution() with empty trigger_id should return error")
	}
}

// =============================================================================
// GetExecutions Tests
// =============================================================================

func TestGetExecutions_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Execution{
			{ID: "e1", TriggerID: "t1"},
			{ID: "e2", TriggerID: "t1"},
		})
	}
	repo, server := newTestRepository(t, handler)
	defer server.Close()

	execs, err := repo.GetExecutions(context.Background(), "t1", 10)
	if err != nil {
		t.Fatalf("GetExecutions() error = %v", err)
	}
	if len(execs) != 2 {
		t.Errorf("len(execs) = %d, want 2", len(execs))
	}
}

func TestGetExecutions_EmptyTriggerID(t *testing.T) {
	repo := &Repository{}
	_, err := repo.GetExecutions(context.Background(), "", 10)
	if err == nil {
		t.Error("GetExecutions('', 10) should return error")
	}
}

func TestGetExecutions_DefaultLimit(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Execution{})
	}
	repo, server := newTestRepository(t, handler)
	defer server.Close()

	// Test with invalid limits (should default to 50)
	_, err := repo.GetExecutions(context.Background(), "t1", 0)
	if err != nil {
		t.Fatalf("GetExecutions() error = %v", err)
	}
	_, err = repo.GetExecutions(context.Background(), "t1", -1)
	if err != nil {
		t.Fatalf("GetExecutions() error = %v", err)
	}
	_, err = repo.GetExecutions(context.Background(), "t1", 2000)
	if err != nil {
		t.Fatalf("GetExecutions() error = %v", err)
	}
}

// =============================================================================
// Model Tests
// =============================================================================

func TestTriggerJSON(t *testing.T) {
	now := time.Now()
	trigger := Trigger{
		ID:            "t1",
		UserID:        "user-123",
		Name:          "test-trigger",
		TriggerType:   "schedule",
		Schedule:      "0 * * * *",
		Condition:     json.RawMessage(`{"type":"always"}`),
		Action:        json.RawMessage(`{"type":"http","url":"http://example.com"}`),
		Enabled:       true,
		LastExecution: now,
		NextExecution: now.Add(time.Hour),
		CreatedAt:     now,
	}

	data, err := json.Marshal(trigger)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded Trigger
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ID != trigger.ID {
		t.Errorf("ID = %s, want %s", decoded.ID, trigger.ID)
	}
	if decoded.TriggerType != trigger.TriggerType {
		t.Errorf("TriggerType = %s, want %s", decoded.TriggerType, trigger.TriggerType)
	}
}

func TestExecutionJSON(t *testing.T) {
	now := time.Now()
	exec := Execution{
		ID:            "e1",
		TriggerID:     "t1",
		ExecutedAt:    now,
		Success:       true,
		ActionType:    "http",
		ActionPayload: json.RawMessage(`{"status":200}`),
	}

	data, err := json.Marshal(exec)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded Execution
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ID != exec.ID {
		t.Errorf("ID = %s, want %s", decoded.ID, exec.ID)
	}
	if decoded.Success != exec.Success {
		t.Errorf("Success = %v, want %v", decoded.Success, exec.Success)
	}
}
