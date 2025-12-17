package database

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCreateAPIKeyNilKey(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.CreateAPIKey(context.Background(), nil)
	if err == nil {
		t.Error("CreateAPIKey() should return error for nil key")
	}
	if !IsInvalidInput(err) {
		t.Error("error should be ErrInvalidInput")
	}
}

func TestCreateAPIKeyInvalidUserID(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	key := &APIKey{UserID: ""}
	err := repo.CreateAPIKey(context.Background(), key)
	if err == nil {
		t.Error("CreateAPIKey() should return error for empty user ID")
	}
}

func TestCreateAPIKeySuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Method = %s, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]APIKey{{
			ID:        "new-key-id",
			UserID:    "user-123",
			CreatedAt: time.Now(),
		}})
	})
	defer cleanup()

	key := &APIKey{UserID: "user-123", KeyHash: "hash123"}
	err := repo.CreateAPIKey(context.Background(), key)
	if err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}
	if key.ID != "new-key-id" {
		t.Errorf("key.ID = %q, want %q", key.ID, "new-key-id")
	}
}

func TestCreateAPIKeyRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	key := &APIKey{UserID: "user-123", KeyHash: "hash123"}
	err := repo.CreateAPIKey(context.Background(), key)
	if err == nil {
		t.Error("CreateAPIKey() should return error on server error")
	}
}

func TestCreateAPIKeyUnmarshalError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	})
	defer cleanup()

	key := &APIKey{UserID: "user-123", KeyHash: "hash123"}
	err := repo.CreateAPIKey(context.Background(), key)
	if err == nil {
		t.Error("CreateAPIKey() should return error for invalid JSON")
	}
}

func TestGetAPIKeysInvalidUserID(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	_, err := repo.GetAPIKeys(context.Background(), "")
	if err == nil {
		t.Error("GetAPIKeys() should return error for empty user ID")
	}
}

func TestGetAPIKeysSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Method = %s, want GET", r.Method)
		}
		if !strings.Contains(r.URL.RawQuery, "user_id=eq.user-123") {
			t.Errorf("Query = %s, should contain user_id filter", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]APIKey{
			{ID: "key-1", UserID: "user-123"},
			{ID: "key-2", UserID: "user-123"},
		})
	})
	defer cleanup()

	keys, err := repo.GetAPIKeys(context.Background(), "user-123")
	if err != nil {
		t.Fatalf("GetAPIKeys() error = %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("len(keys) = %d, want 2", len(keys))
	}
}

func TestGetAPIKeysRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	_, err := repo.GetAPIKeys(context.Background(), "user-123")
	if err == nil {
		t.Error("GetAPIKeys() should return error on server error")
	}
}

func TestGetAPIKeysUnmarshalError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	})
	defer cleanup()

	_, err := repo.GetAPIKeys(context.Background(), "user-123")
	if err == nil {
		t.Error("GetAPIKeys() should return error for invalid JSON")
	}
}

func TestGetAPIKeyByHashEmptyHash(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	_, err := repo.GetAPIKeyByHash(context.Background(), "")
	if err == nil {
		t.Error("GetAPIKeyByHash() should return error for empty hash")
	}
	if !IsInvalidInput(err) {
		t.Error("error should be ErrInvalidInput")
	}
}

func TestGetAPIKeyByHashSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "key_hash=eq.") {
			t.Errorf("Query = %s, should contain key_hash filter", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]APIKey{{ID: "key-1", KeyHash: "hash123"}})
	})
	defer cleanup()

	key, err := repo.GetAPIKeyByHash(context.Background(), "hash123")
	if err != nil {
		t.Fatalf("GetAPIKeyByHash() error = %v", err)
	}
	if key.ID != "key-1" {
		t.Errorf("key.ID = %q, want %q", key.ID, "key-1")
	}
}

func TestGetAPIKeyByHashNotFound(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]APIKey{})
	})
	defer cleanup()

	_, err := repo.GetAPIKeyByHash(context.Background(), "nonexistent")
	if err == nil {
		t.Error("GetAPIKeyByHash() should return error for not found")
	}
	if !IsNotFound(err) {
		t.Error("error should be NotFoundError")
	}
}

func TestGetAPIKeyByHashRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	_, err := repo.GetAPIKeyByHash(context.Background(), "hash123")
	if err == nil {
		t.Error("GetAPIKeyByHash() should return error on server error")
	}
}

func TestGetAPIKeyByHashUnmarshalError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	})
	defer cleanup()

	_, err := repo.GetAPIKeyByHash(context.Background(), "hash123")
	if err == nil {
		t.Error("GetAPIKeyByHash() should return error for invalid JSON")
	}
}

func TestRevokeAPIKeyInvalidKeyID(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.RevokeAPIKey(context.Background(), "", "user-123")
	if err == nil {
		t.Error("RevokeAPIKey() should return error for empty key ID")
	}
}

func TestRevokeAPIKeyInvalidUserID(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.RevokeAPIKey(context.Background(), "key-123", "")
	if err == nil {
		t.Error("RevokeAPIKey() should return error for empty user ID")
	}
}

func TestRevokeAPIKeySuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("Method = %s, want PATCH", r.Method)
		}
		if !strings.Contains(r.URL.RawQuery, "id=eq.key-123") {
			t.Errorf("Query = %s, should contain id filter", r.URL.RawQuery)
		}
		if !strings.Contains(r.URL.RawQuery, "user_id=eq.user-123") {
			t.Errorf("Query = %s, should contain user_id filter", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.RevokeAPIKey(context.Background(), "key-123", "user-123")
	if err != nil {
		t.Fatalf("RevokeAPIKey() error = %v", err)
	}
}

func TestRevokeAPIKeyRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	err := repo.RevokeAPIKey(context.Background(), "key-123", "user-123")
	if err == nil {
		t.Error("RevokeAPIKey() should return error on server error")
	}
}

func TestUpdateAPIKeyLastUsedInvalidKeyID(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.UpdateAPIKeyLastUsed(context.Background(), "")
	if err == nil {
		t.Error("UpdateAPIKeyLastUsed() should return error for empty key ID")
	}
}

func TestUpdateAPIKeyLastUsedSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("Method = %s, want PATCH", r.Method)
		}
		if !strings.Contains(r.URL.RawQuery, "id=eq.key-123") {
			t.Errorf("Query = %s, should contain id filter", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.UpdateAPIKeyLastUsed(context.Background(), "key-123")
	if err != nil {
		t.Fatalf("UpdateAPIKeyLastUsed() error = %v", err)
	}
}

func TestUpdateAPIKeyLastUsedRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	err := repo.UpdateAPIKeyLastUsed(context.Background(), "key-123")
	if err == nil {
		t.Error("UpdateAPIKeyLastUsed() should return error on server error")
	}
}

// =============================================================================
// MockRepository Tests
// =============================================================================

func TestMockRepositoryHealthCheck(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := NewMockRepository()
		err := repo.HealthCheck(context.Background())
		if err != nil {
			t.Errorf("HealthCheck() error = %v", err)
		}
	})

	t.Run("with injected error", func(t *testing.T) {
		repo := NewMockRepository()
		expectedErr := errors.New("database unavailable")
		repo.ErrorOnNextCall = expectedErr

		err := repo.HealthCheck(context.Background())
		if err != expectedErr {
			t.Errorf("HealthCheck() error = %v, want %v", err, expectedErr)
		}

		// Error should be cleared after first call
		err = repo.HealthCheck(context.Background())
		if err != nil {
			t.Errorf("HealthCheck() second call error = %v, want nil", err)
		}
	})
}

func TestMockRepositoryUpdateUserNonce(t *testing.T) {
	repo := NewMockRepository()

	// Create a user first
	user := &User{ID: "user-123", Address: "NXV7ZhHiyM1aHXwpVsRZC6BEaDPjpfbNGN", Nonce: "old-nonce"}
	err := repo.CreateUser(context.Background(), user)
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	t.Run("success for existing user", func(t *testing.T) {
		err := repo.UpdateUserNonce(context.Background(), "user-123", "new-nonce")
		if err != nil {
			t.Fatalf("UpdateUserNonce() error = %v", err)
		}
		// Note: Mock implementation doesn't actually update the nonce,
		// it just verifies the user exists
	})

	t.Run("user not found", func(t *testing.T) {
		err := repo.UpdateUserNonce(context.Background(), "nonexistent", "nonce")
		if err == nil {
			t.Error("UpdateUserNonce() should return error for nonexistent user")
		}
		if !IsNotFound(err) {
			t.Error("error should be NotFoundError")
		}
	})

	t.Run("with injected error", func(t *testing.T) {
		expectedErr := errors.New("database error")
		repo.ErrorOnNextCall = expectedErr

		err := repo.UpdateUserNonce(context.Background(), "user-123", "nonce")
		if err != expectedErr {
			t.Errorf("UpdateUserNonce() error = %v, want %v", err, expectedErr)
		}
	})
}

// Helper to setup test server (reused from generic_repository_test.go)
func setupAPIKeyTestServer(t *testing.T, handler http.HandlerFunc) (*Repository, func()) {
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
