// Package supabase provides NeoVault-specific database operations.
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
	client, _ := database.NewClient(database.Config{URL: "http://localhost", ServiceKey: "test"})
	baseRepo := database.NewRepository(client)
	repo := NewRepository(baseRepo)

	if repo == nil {
		t.Fatal("NewRepository() returned nil")
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
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]RequestRecord{{
			ID:     "id-123",
			UserID: "user-456",
			Status: "pending",
		}})
	}

	repo, server := newTestRepository(t, handler)
	defer server.Close()

	req := &RequestRecord{UserID: "user-456", Status: "pending"}
	err := repo.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
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

func TestCreate_EmptyUserID(t *testing.T) {
	repo := &Repository{}
	err := repo.Create(context.Background(), &RequestRecord{})
	if err == nil {
		t.Error("Create() with empty user_id should return error")
	}
}

func TestCreate_ServerError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}
	repo, server := newTestRepository(t, handler)
	defer server.Close()

	err := repo.Create(context.Background(), &RequestRecord{UserID: "user-456"})
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
		w.WriteHeader(http.StatusOK)
	}
	repo, server := newTestRepository(t, handler)
	defer server.Close()

	err := repo.Update(context.Background(), &RequestRecord{ID: "id-123", Status: "mixing"})
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

func TestUpdate_EmptyID(t *testing.T) {
	repo := &Repository{}
	err := repo.Update(context.Background(), &RequestRecord{})
	if err == nil {
		t.Error("Update() with empty id should return error")
	}
}

// =============================================================================
// GetByID Tests
// =============================================================================

func TestGetByID_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]RequestRecord{{ID: "id-123", UserID: "user-456", Status: "pending"}})
	}
	repo, server := newTestRepository(t, handler)
	defer server.Close()

	rec, err := repo.GetByID(context.Background(), "id-123")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if rec.ID != "id-123" {
		t.Errorf("ID = %s, want id-123", rec.ID)
	}
}

func TestGetByID_EmptyID(t *testing.T) {
	repo := &Repository{}
	_, err := repo.GetByID(context.Background(), "")
	if err == nil {
		t.Error("GetByID('') should return error")
	}
}

func TestGetByID_NotFound(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]RequestRecord{})
	}
	repo, server := newTestRepository(t, handler)
	defer server.Close()

	_, err := repo.GetByID(context.Background(), "nonexistent")
	if err == nil {
		t.Error("GetByID() should return error for not found")
	}
}

// =============================================================================
// GetByDepositAddress Tests
// =============================================================================

func TestGetByDepositAddress_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "deposit_address=eq.NAddr123") {
			t.Errorf("query = %s, want deposit_address=eq.NAddr123", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]RequestRecord{{ID: "id-123", DepositAddress: "NAddr123"}})
	}
	repo, server := newTestRepository(t, handler)
	defer server.Close()

	rec, err := repo.GetByDepositAddress(context.Background(), "NAddr123")
	if err != nil {
		t.Fatalf("GetByDepositAddress() error = %v", err)
	}
	if rec.DepositAddress != "NAddr123" {
		t.Errorf("DepositAddress = %s, want NAddr123", rec.DepositAddress)
	}
}

func TestGetByDepositAddress_EmptyAddress(t *testing.T) {
	repo := &Repository{}
	_, err := repo.GetByDepositAddress(context.Background(), "")
	if err == nil {
		t.Error("GetByDepositAddress('') should return error")
	}
}

func TestGetByDepositAddress_NotFound(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]RequestRecord{})
	}
	repo, server := newTestRepository(t, handler)
	defer server.Close()

	_, err := repo.GetByDepositAddress(context.Background(), "nonexistent")
	if err == nil {
		t.Error("GetByDepositAddress() should return error for not found")
	}
}

// =============================================================================
// ListByUser Tests
// =============================================================================

func TestListByUser_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "user_id=eq.user-456") {
			t.Errorf("query = %s, want user_id=eq.user-456", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]RequestRecord{
			{ID: "id-1", UserID: "user-456"},
			{ID: "id-2", UserID: "user-456"},
		})
	}
	repo, server := newTestRepository(t, handler)
	defer server.Close()

	records, err := repo.ListByUser(context.Background(), "user-456")
	if err != nil {
		t.Fatalf("ListByUser() error = %v", err)
	}
	if len(records) != 2 {
		t.Errorf("len(records) = %d, want 2", len(records))
	}
}

func TestListByUser_EmptyUserID(t *testing.T) {
	repo := &Repository{}
	_, err := repo.ListByUser(context.Background(), "")
	if err == nil {
		t.Error("ListByUser('') should return error")
	}
}

// =============================================================================
// ListByStatus Tests
// =============================================================================

func TestListByStatus_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]RequestRecord{{ID: "id-1", Status: "pending"}})
	}
	repo, server := newTestRepository(t, handler)
	defer server.Close()

	records, err := repo.ListByStatus(context.Background(), "pending")
	if err != nil {
		t.Fatalf("ListByStatus() error = %v", err)
	}
	if len(records) != 1 {
		t.Errorf("len(records) = %d, want 1", len(records))
	}
}

func TestListByStatus_InvalidStatus(t *testing.T) {
	repo := &Repository{}
	_, err := repo.ListByStatus(context.Background(), "invalid")
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

	validStatuses := []string{"pending", "deposited", "mixing", "delivered", "failed", "refunded"}
	for _, status := range validStatuses {
		_, err := repo.ListByStatus(context.Background(), status)
		if err != nil {
			t.Errorf("ListByStatus(%s) error = %v", status, err)
		}
	}
}

// =============================================================================
// Model Tests
// =============================================================================

func TestRequestRecordJSON(t *testing.T) {
	now := time.Now()
	rec := RequestRecord{
		ID:                    "id-123",
		UserID:                "user-456",
		TokenType:             "GAS",
		Status:                "mixing",
		TotalAmount:           1000000,
		ServiceFee:            5000,
		NetAmount:             995000,
		TargetAddresses:       []TargetAddress{{Address: "addr1", Amount: 500000}},
		InitialSplits:         3,
		MixingDurationSeconds: 1800,
		DepositAddress:        "deposit-addr",
		PoolAccounts:          []string{"acc1", "acc2"},
		CreatedAt:             now,
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
	if decoded.TotalAmount != rec.TotalAmount {
		t.Errorf("TotalAmount = %d, want %d", decoded.TotalAmount, rec.TotalAmount)
	}
}

func TestTargetAddressJSON(t *testing.T) {
	target := TargetAddress{Address: "NAddr123", Amount: 1000}
	data, err := json.Marshal(target)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded TargetAddress
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Address != target.Address {
		t.Errorf("Address = %s, want %s", decoded.Address, target.Address)
	}
}
