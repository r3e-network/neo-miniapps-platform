// Package supabase provides VRF-specific database operations.
package supabase

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/database"
)

// =============================================================================
// Test Helpers
// =============================================================================

// newTestRepository creates a repository with a mock HTTP server.
func newTestRepository(t *testing.T, handler http.HandlerFunc) (*Repository, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
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
	client, err := database.NewClient(database.Config{
		URL:        "http://localhost",
		ServiceKey: "test-key",
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	baseRepo := database.NewRepository(client)
	repo := NewRepository(baseRepo)

	if repo == nil {
		t.Fatal("NewRepository() returned nil")
	}
	if repo.base != baseRepo {
		t.Error("base repository not set correctly")
	}
}

// =============================================================================
// Create Tests
// =============================================================================

func TestCreate_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if !strings.Contains(r.URL.Path, "vrf_requests") {
			t.Errorf("path = %s, want vrf_requests", r.URL.Path)
		}

		// Return the created record
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]RequestRecord{{
			ID:        "id-123",
			RequestID: "req-456",
			UserID:    "user-789",
			Status:    "pending",
			CreatedAt: time.Now(),
		}})
	}

	repo, server := newTestRepository(t, handler)
	defer server.Close()

	req := &RequestRecord{
		RequestID: "req-456",
		UserID:    "user-789",
		Seed:      "test-seed",
		NumWords:  3,
		Status:    "pending",
	}

	err := repo.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Check that the ID was populated from response
	if req.ID != "id-123" {
		t.Errorf("ID = %s, want id-123", req.ID)
	}
}

func TestCreate_NilRequest(t *testing.T) {
	repo := &Repository{}
	err := repo.Create(context.Background(), nil)
	if err == nil {
		t.Error("Create(nil) should return error")
	}
}

func TestCreate_EmptyRequestID(t *testing.T) {
	repo := &Repository{}
	err := repo.Create(context.Background(), &RequestRecord{})
	if err == nil {
		t.Error("Create() with empty request_id should return error")
	}
}

func TestCreate_ServerError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}

	repo, server := newTestRepository(t, handler)
	defer server.Close()

	req := &RequestRecord{
		RequestID: "req-456",
		Status:    "pending",
	}

	err := repo.Create(context.Background(), req)
	if err == nil {
		t.Error("Create() should return error on server error")
	}
}

// =============================================================================
// Update Tests
// =============================================================================

func TestUpdate_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		if !strings.Contains(r.URL.RawQuery, "request_id=eq.req-456") {
			t.Errorf("query = %s, want request_id=eq.req-456", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusOK)
	}

	repo, server := newTestRepository(t, handler)
	defer server.Close()

	req := &RequestRecord{
		RequestID:   "req-456",
		Status:      "fulfilled",
		RandomWords: []string{"word1", "word2"},
	}

	err := repo.Update(context.Background(), req)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
}

func TestUpdate_NilRequest(t *testing.T) {
	repo := &Repository{}
	err := repo.Update(context.Background(), nil)
	if err == nil {
		t.Error("Update(nil) should return error")
	}
}

func TestUpdate_EmptyRequestID(t *testing.T) {
	repo := &Repository{}
	err := repo.Update(context.Background(), &RequestRecord{})
	if err == nil {
		t.Error("Update() with empty request_id should return error")
	}
}

func TestUpdate_ServerError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}

	repo, server := newTestRepository(t, handler)
	defer server.Close()

	err := repo.Update(context.Background(), &RequestRecord{RequestID: "req-456"})
	if err == nil {
		t.Error("Update() should return error on server error")
	}
}

// =============================================================================
// GetByRequestID Tests
// =============================================================================

func TestGetByRequestID_Success(t *testing.T) {
	now := time.Now()
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if !strings.Contains(r.URL.RawQuery, "request_id=eq.req-456") {
			t.Errorf("query = %s, want request_id=eq.req-456", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]RequestRecord{{
			ID:          "id-123",
			RequestID:   "req-456",
			UserID:      "user-789",
			Seed:        "test-seed",
			NumWords:    3,
			Status:      "fulfilled",
			RandomWords: []string{"word1", "word2", "word3"},
			CreatedAt:   now,
		}})
	}

	repo, server := newTestRepository(t, handler)
	defer server.Close()

	rec, err := repo.GetByRequestID(context.Background(), "req-456")
	if err != nil {
		t.Fatalf("GetByRequestID() error = %v", err)
	}

	if rec.RequestID != "req-456" {
		t.Errorf("RequestID = %s, want req-456", rec.RequestID)
	}
	if rec.Status != "fulfilled" {
		t.Errorf("Status = %s, want fulfilled", rec.Status)
	}
	if len(rec.RandomWords) != 3 {
		t.Errorf("len(RandomWords) = %d, want 3", len(rec.RandomWords))
	}
}

func TestGetByRequestID_EmptyID(t *testing.T) {
	repo := &Repository{}
	_, err := repo.GetByRequestID(context.Background(), "")
	if err == nil {
		t.Error("GetByRequestID('') should return error")
	}
}

func TestGetByRequestID_NotFound(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]RequestRecord{}) // Empty array
	}

	repo, server := newTestRepository(t, handler)
	defer server.Close()

	_, err := repo.GetByRequestID(context.Background(), "nonexistent")
	if err == nil {
		t.Error("GetByRequestID() should return error for not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v, want 'not found'", err)
	}
}

func TestGetByRequestID_ServerError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}

	repo, server := newTestRepository(t, handler)
	defer server.Close()

	_, err := repo.GetByRequestID(context.Background(), "req-456")
	if err == nil {
		t.Error("GetByRequestID() should return error on server error")
	}
}

func TestGetByRequestID_InvalidJSON(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}

	repo, server := newTestRepository(t, handler)
	defer server.Close()

	_, err := repo.GetByRequestID(context.Background(), "req-456")
	if err == nil {
		t.Error("GetByRequestID() should return error on invalid JSON")
	}
}

// =============================================================================
// ListByStatus Tests
// =============================================================================

func TestListByStatus_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if !strings.Contains(r.URL.RawQuery, "status=eq.pending") {
			t.Errorf("query = %s, want status=eq.pending", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]RequestRecord{
			{ID: "id-1", RequestID: "req-1", Status: "pending"},
			{ID: "id-2", RequestID: "req-2", Status: "pending"},
		})
	}

	repo, server := newTestRepository(t, handler)
	defer server.Close()

	records, err := repo.ListByStatus(context.Background(), "pending")
	if err != nil {
		t.Fatalf("ListByStatus() error = %v", err)
	}

	if len(records) != 2 {
		t.Errorf("len(records) = %d, want 2", len(records))
	}
}

func TestListByStatus_InvalidStatus(t *testing.T) {
	repo := &Repository{}
	_, err := repo.ListByStatus(context.Background(), "invalid-status")
	if err == nil {
		t.Error("ListByStatus() with invalid status should return error")
	}
}

func TestListByStatus_ValidStatuses(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]RequestRecord{})
	}

	repo, server := newTestRepository(t, handler)
	defer server.Close()

	validStatuses := []string{"pending", "processing", "fulfilled", "failed"}
	for _, status := range validStatuses {
		_, err := repo.ListByStatus(context.Background(), status)
		if err != nil {
			t.Errorf("ListByStatus(%s) error = %v", status, err)
		}
	}
}

func TestListByStatus_ServerError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}

	repo, server := newTestRepository(t, handler)
	defer server.Close()

	_, err := repo.ListByStatus(context.Background(), "pending")
	if err == nil {
		t.Error("ListByStatus() should return error on server error")
	}
}

func TestListByStatus_InvalidJSON(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}

	repo, server := newTestRepository(t, handler)
	defer server.Close()

	_, err := repo.ListByStatus(context.Background(), "pending")
	if err == nil {
		t.Error("ListByStatus() should return error on invalid JSON")
	}
}

// =============================================================================
// RequestRecord Tests
// =============================================================================

func TestRequestRecordJSON(t *testing.T) {
	now := time.Now()
	rec := RequestRecord{
		ID:               "id-123",
		RequestID:        "req-456",
		UserID:           "user-789",
		RequesterAddress: "NAddr123",
		Seed:             "test-seed",
		NumWords:         3,
		CallbackGasLimit: 100000,
		Status:           "fulfilled",
		RandomWords:      []string{"word1", "word2", "word3"},
		Proof:            "proof-hex",
		FulfillTxHash:    "0x123",
		CreatedAt:        now,
		FulfilledAt:      now.Add(time.Minute),
	}

	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded RequestRecord
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ID != rec.ID {
		t.Errorf("ID = %s, want %s", decoded.ID, rec.ID)
	}
	if decoded.RequestID != rec.RequestID {
		t.Errorf("RequestID = %s, want %s", decoded.RequestID, rec.RequestID)
	}
	if decoded.Status != rec.Status {
		t.Errorf("Status = %s, want %s", decoded.Status, rec.Status)
	}
	if len(decoded.RandomWords) != len(rec.RandomWords) {
		t.Errorf("len(RandomWords) = %d, want %d", len(decoded.RandomWords), len(rec.RandomWords))
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkCreate(b *testing.B) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]RequestRecord{{ID: "id-123", RequestID: "req-456"}})
	}

	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	client, _ := database.NewClient(database.Config{
		URL:        server.URL,
		ServiceKey: "test-api-key",
	})
	baseRepo := database.NewRepository(client)
	repo := NewRepository(baseRepo)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = repo.Create(context.Background(), &RequestRecord{RequestID: "req-456"})
	}
}

func BenchmarkGetByRequestID(b *testing.B) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]RequestRecord{{ID: "id-123", RequestID: "req-456"}})
	}

	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	client, _ := database.NewClient(database.Config{
		URL:        server.URL,
		ServiceKey: "test-api-key",
	})
	baseRepo := database.NewRepository(client)
	repo := NewRepository(baseRepo)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = repo.GetByRequestID(context.Background(), "req-456")
	}
}
