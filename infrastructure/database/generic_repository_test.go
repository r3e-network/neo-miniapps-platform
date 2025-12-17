package database

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// =============================================================================
// QueryBuilder Tests
// =============================================================================

func TestNewQuery(t *testing.T) {
	q := NewQuery()
	if q == nil {
		t.Fatal("NewQuery() returned nil")
	}
	if len(q.filters) != 0 {
		t.Error("NewQuery() should have empty filters")
	}
	if q.order != "" {
		t.Error("NewQuery() should have empty order")
	}
	if q.limit != 0 {
		t.Error("NewQuery() should have zero limit")
	}
}

func TestQueryBuilderEq(t *testing.T) {
	q := NewQuery().Eq("name", "test")
	result := q.Build()
	if result != "name=eq.test" {
		t.Errorf("Build() = %q, want %q", result, "name=eq.test")
	}
}

func TestQueryBuilderEqWithSpecialChars(t *testing.T) {
	q := NewQuery().Eq("name", "test value&special")
	result := q.Build()
	// URL encoded
	if !strings.Contains(result, "name=eq.") {
		t.Errorf("Build() = %q, should contain encoded value", result)
	}
}

func TestQueryBuilderIsNull(t *testing.T) {
	q := NewQuery().IsNull("deleted_at")
	result := q.Build()
	if result != "deleted_at=is.null" {
		t.Errorf("Build() = %q, want %q", result, "deleted_at=is.null")
	}
}

func TestQueryBuilderIsFalse(t *testing.T) {
	q := NewQuery().IsFalse("is_active")
	result := q.Build()
	if result != "is_active=eq.false" {
		t.Errorf("Build() = %q, want %q", result, "is_active=eq.false")
	}
}

func TestQueryBuilderIsTrue(t *testing.T) {
	q := NewQuery().IsTrue("is_active")
	result := q.Build()
	if result != "is_active=eq.true" {
		t.Errorf("Build() = %q, want %q", result, "is_active=eq.true")
	}
}

func TestQueryBuilderLte(t *testing.T) {
	q := NewQuery().Lte("created_at", "2024-01-01")
	result := q.Build()
	if result != "created_at=lte.2024-01-01" {
		t.Errorf("Build() = %q, want %q", result, "created_at=lte.2024-01-01")
	}
}

func TestQueryBuilderGte(t *testing.T) {
	q := NewQuery().Gte("created_at", "2024-01-01")
	result := q.Build()
	if result != "created_at=gte.2024-01-01" {
		t.Errorf("Build() = %q, want %q", result, "created_at=gte.2024-01-01")
	}
}

func TestQueryBuilderOrderAsc(t *testing.T) {
	q := NewQuery().OrderAsc("name")
	result := q.Build()
	if result != "order=name.asc" {
		t.Errorf("Build() = %q, want %q", result, "order=name.asc")
	}
}

func TestQueryBuilderOrderDesc(t *testing.T) {
	q := NewQuery().OrderDesc("created_at")
	result := q.Build()
	if result != "order=created_at.desc" {
		t.Errorf("Build() = %q, want %q", result, "order=created_at.desc")
	}
}

func TestQueryBuilderLimit(t *testing.T) {
	q := NewQuery().Limit(10)
	result := q.Build()
	if result != "limit=10" {
		t.Errorf("Build() = %q, want %q", result, "limit=10")
	}
}

func TestQueryBuilderChaining(t *testing.T) {
	q := NewQuery().
		Eq("user_id", "123").
		IsNull("deleted_at").
		OrderDesc("created_at").
		Limit(50)

	result := q.Build()

	// Check all parts are present
	if !strings.Contains(result, "user_id=eq.123") {
		t.Error("Build() should contain user_id filter")
	}
	if !strings.Contains(result, "deleted_at=is.null") {
		t.Error("Build() should contain deleted_at filter")
	}
	if !strings.Contains(result, "order=created_at.desc") {
		t.Error("Build() should contain order")
	}
	if !strings.Contains(result, "limit=50") {
		t.Error("Build() should contain limit")
	}
}

func TestQueryBuilderEmptyBuild(t *testing.T) {
	q := NewQuery()
	result := q.Build()
	if result != "" {
		t.Errorf("Build() = %q, want empty string", result)
	}
}

func TestQueryBuilderOnlyOrder(t *testing.T) {
	q := NewQuery().OrderAsc("name")
	result := q.Build()
	if result != "order=name.asc" {
		t.Errorf("Build() = %q, want %q", result, "order=name.asc")
	}
}

func TestQueryBuilderOnlyLimit(t *testing.T) {
	q := NewQuery().Limit(100)
	result := q.Build()
	if result != "limit=100" {
		t.Errorf("Build() = %q, want %q", result, "limit=100")
	}
}

func TestQueryBuilderMultipleFilters(t *testing.T) {
	q := NewQuery().
		Eq("status", "active").
		Gte("amount", "100").
		Lte("amount", "1000")

	result := q.Build()

	// Should have & separators
	parts := strings.Split(result, "&")
	if len(parts) != 3 {
		t.Errorf("Build() should have 3 parts, got %d: %q", len(parts), result)
	}
}

// =============================================================================
// Generic Repository Tests with Mock Server
// =============================================================================

type testModel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func setupTestServer(t *testing.T, handler http.HandlerFunc) (*Repository, func()) {
	t.Helper()
	server := httptest.NewServer(handler)

	client, err := NewClient(Config{
		URL:        server.URL,
		ServiceKey: "test-key",
	})
	if err != nil {
		server.Close()
		t.Fatalf("NewClient() error = %v", err)
	}

	repo := NewRepository(client)
	return repo, server.Close
}

func TestGenericCreateNilModel(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := GenericCreate[testModel](repo, context.Background(), "test_table", nil, nil)
	if err == nil {
		t.Error("GenericCreate() should return error for nil model")
	}
	if !strings.Contains(err.Error(), "model cannot be nil") {
		t.Errorf("error = %q, should mention nil model", err.Error())
	}
}

func TestGenericCreateSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Method = %s, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]testModel{{ID: "new-id", Name: "created"}})
	})
	defer cleanup()

	model := &testModel{Name: "test"}
	var resultCalled bool
	err := GenericCreate(repo, context.Background(), "test_table", model, func(rows []testModel) {
		resultCalled = true
		if len(rows) > 0 {
			*model = rows[0]
		}
	})

	if err != nil {
		t.Fatalf("GenericCreate() error = %v", err)
	}
	if !resultCalled {
		t.Error("onResult callback should be called")
	}
	if model.ID != "new-id" {
		t.Errorf("model.ID = %q, want %q", model.ID, "new-id")
	}
}

func TestGenericCreateWithoutCallback(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]testModel{{ID: "new-id"}})
	})
	defer cleanup()

	model := &testModel{Name: "test"}
	err := GenericCreate(repo, context.Background(), "test_table", model, nil)
	if err != nil {
		t.Fatalf("GenericCreate() error = %v", err)
	}
}

func TestGenericCreateRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	model := &testModel{Name: "test"}
	err := GenericCreate(repo, context.Background(), "test_table", model, nil)
	if err == nil {
		t.Error("GenericCreate() should return error on server error")
	}
}

func TestGenericUpdateNilModel(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := GenericUpdate[testModel](repo, context.Background(), "test_table", "id", "123", nil)
	if err == nil {
		t.Error("GenericUpdate() should return error for nil model")
	}
}

func TestGenericUpdateEmptyKey(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	model := &testModel{Name: "test"}
	err := GenericUpdate(repo, context.Background(), "test_table", "id", "", model)
	if err == nil {
		t.Error("GenericUpdate() should return error for empty key")
	}
}

func TestGenericUpdateSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("Method = %s, want PATCH", r.Method)
		}
		if !strings.Contains(r.URL.RawQuery, "id=eq.123") {
			t.Errorf("Query = %s, should contain id=eq.123", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	model := &testModel{Name: "updated"}
	err := GenericUpdate(repo, context.Background(), "test_table", "id", "123", model)
	if err != nil {
		t.Fatalf("GenericUpdate() error = %v", err)
	}
}

func TestGenericGetByFieldEmptyValue(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	_, err := GenericGetByField[testModel](repo, context.Background(), "test_table", "id", "")
	if err == nil {
		t.Error("GenericGetByField() should return error for empty value")
	}
}

func TestGenericGetByFieldSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Method = %s, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]testModel{{ID: "123", Name: "found"}})
	})
	defer cleanup()

	result, err := GenericGetByField[testModel](repo, context.Background(), "test_table", "id", "123")
	if err != nil {
		t.Fatalf("GenericGetByField() error = %v", err)
	}
	if result.ID != "123" {
		t.Errorf("result.ID = %q, want %q", result.ID, "123")
	}
}

func TestGenericGetByFieldNotFound(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]testModel{})
	})
	defer cleanup()

	_, err := GenericGetByField[testModel](repo, context.Background(), "test_table", "id", "nonexistent")
	if err == nil {
		t.Error("GenericGetByField() should return error for not found")
	}
	if !IsNotFound(err) {
		t.Error("error should be NotFoundError")
	}
}

func TestGenericGetByFieldUnmarshalError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	})
	defer cleanup()

	_, err := GenericGetByField[testModel](repo, context.Background(), "test_table", "id", "123")
	if err == nil {
		t.Error("GenericGetByField() should return error for invalid JSON")
	}
}

func TestGenericListSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Method = %s, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]testModel{
			{ID: "1", Name: "first"},
			{ID: "2", Name: "second"},
		})
	})
	defer cleanup()

	results, err := GenericList[testModel](repo, context.Background(), "test_table")
	if err != nil {
		t.Fatalf("GenericList() error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("len(results) = %d, want 2", len(results))
	}
}

func TestGenericListUnmarshalError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	})
	defer cleanup()

	_, err := GenericList[testModel](repo, context.Background(), "test_table")
	if err == nil {
		t.Error("GenericList() should return error for invalid JSON")
	}
}

func TestGenericListByFieldEmptyValue(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	_, err := GenericListByField[testModel](repo, context.Background(), "test_table", "user_id", "")
	if err == nil {
		t.Error("GenericListByField() should return error for empty value")
	}
}

func TestGenericListByFieldSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "user_id=eq.user123") {
			t.Errorf("Query = %s, should contain user_id filter", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]testModel{{ID: "1", Name: "test"}})
	})
	defer cleanup()

	results, err := GenericListByField[testModel](repo, context.Background(), "test_table", "user_id", "user123")
	if err != nil {
		t.Fatalf("GenericListByField() error = %v", err)
	}
	if len(results) != 1 {
		t.Errorf("len(results) = %d, want 1", len(results))
	}
}

func TestGenericListByFieldUnmarshalError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	})
	defer cleanup()

	_, err := GenericListByField[testModel](repo, context.Background(), "test_table", "user_id", "123")
	if err == nil {
		t.Error("GenericListByField() should return error for invalid JSON")
	}
}

func TestGenericListWithQuerySuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "status=eq.active") {
			t.Errorf("Query = %s, should contain custom query", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]testModel{{ID: "1"}})
	})
	defer cleanup()

	results, err := GenericListWithQuery[testModel](repo, context.Background(), "test_table", "status=eq.active")
	if err != nil {
		t.Fatalf("GenericListWithQuery() error = %v", err)
	}
	if len(results) != 1 {
		t.Errorf("len(results) = %d, want 1", len(results))
	}
}

func TestGenericListWithQueryUnmarshalError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	})
	defer cleanup()

	_, err := GenericListWithQuery[testModel](repo, context.Background(), "test_table", "status=eq.active")
	if err == nil {
		t.Error("GenericListWithQuery() should return error for invalid JSON")
	}
}

func TestGenericDeleteEmptyKey(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := GenericDelete(repo, context.Background(), "test_table", "id", "")
	if err == nil {
		t.Error("GenericDelete() should return error for empty key")
	}
}

func TestGenericDeleteSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("Method = %s, want DELETE", r.Method)
		}
		if !strings.Contains(r.URL.RawQuery, "id=eq.123") {
			t.Errorf("Query = %s, should contain id=eq.123", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := GenericDelete(repo, context.Background(), "test_table", "id", "123")
	if err != nil {
		t.Fatalf("GenericDelete() error = %v", err)
	}
}

func TestGenericUpdateWithQueryNilModel(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := GenericUpdateWithQuery[testModel](repo, context.Background(), "test_table", "id=eq.123", nil)
	if err == nil {
		t.Error("GenericUpdateWithQuery() should return error for nil model")
	}
}

func TestGenericUpdateWithQueryEmptyQuery(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	model := &testModel{Name: "test"}
	err := GenericUpdateWithQuery(repo, context.Background(), "test_table", "", model)
	if err == nil {
		t.Error("GenericUpdateWithQuery() should return error for empty query")
	}
}

func TestGenericUpdateWithQuerySuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("Method = %s, want PATCH", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	model := &testModel{Name: "updated"}
	err := GenericUpdateWithQuery(repo, context.Background(), "test_table", "id=eq.123&user_id=eq.456", model)
	if err != nil {
		t.Fatalf("GenericUpdateWithQuery() error = %v", err)
	}
}

func TestGenericDeleteWithQueryEmptyQuery(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := GenericDeleteWithQuery(repo, context.Background(), "test_table", "")
	if err == nil {
		t.Error("GenericDeleteWithQuery() should return error for empty query")
	}
}

func TestGenericDeleteWithQuerySuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("Method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := GenericDeleteWithQuery(repo, context.Background(), "test_table", "id=eq.123&user_id=eq.456")
	if err != nil {
		t.Fatalf("GenericDeleteWithQuery() error = %v", err)
	}
}
