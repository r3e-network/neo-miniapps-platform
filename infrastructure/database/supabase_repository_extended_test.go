package database

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// User Operations Tests
// =============================================================================

func TestGetUserByEmailEmpty(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	_, err := repo.GetUserByEmail(context.Background(), "")
	if err == nil {
		t.Error("GetUserByEmail() should return error for empty email")
	}
}

func TestGetUserByEmailInvalid(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	_, err := repo.GetUserByEmail(context.Background(), "invalid-email")
	if err == nil {
		t.Error("GetUserByEmail() should return error for invalid email")
	}
}

func TestGetUserByEmailSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "email=eq.") {
			t.Errorf("Query = %s, should contain email filter", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]User{{ID: "user-1", Email: "test@example.com"}})
	})
	defer cleanup()

	user, err := repo.GetUserByEmail(context.Background(), "test@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail() error = %v", err)
	}
	if user.ID != "user-1" {
		t.Errorf("user.ID = %q, want %q", user.ID, "user-1")
	}
}

func TestGetUserByEmailNotFound(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]User{})
	})
	defer cleanup()

	_, err := repo.GetUserByEmail(context.Background(), "notfound@example.com")
	if err == nil {
		t.Error("GetUserByEmail() should return error for not found")
	}
	if !IsNotFound(err) {
		t.Error("error should be NotFoundError")
	}
}

func TestGetUserByEmailRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	_, err := repo.GetUserByEmail(context.Background(), "test@example.com")
	if err == nil {
		t.Error("GetUserByEmail() should return error on server error")
	}
}

func TestGetUserByEmailUnmarshalError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	})
	defer cleanup()

	_, err := repo.GetUserByEmail(context.Background(), "test@example.com")
	if err == nil {
		t.Error("GetUserByEmail() should return error for invalid JSON")
	}
}

func TestUpdateUserEmailInvalidUserID(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.UpdateUserEmail(context.Background(), "", "test@example.com")
	if err == nil {
		t.Error("UpdateUserEmail() should return error for empty user ID")
	}
}

func TestUpdateUserEmailInvalidEmail(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.UpdateUserEmail(context.Background(), "user-123", "invalid-email")
	if err == nil {
		t.Error("UpdateUserEmail() should return error for invalid email")
	}
}

func TestUpdateUserEmailSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("Method = %s, want PATCH", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.UpdateUserEmail(context.Background(), "user-123", "new@example.com")
	if err != nil {
		t.Fatalf("UpdateUserEmail() error = %v", err)
	}
}

func TestUpdateUserEmailRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	err := repo.UpdateUserEmail(context.Background(), "user-123", "new@example.com")
	if err == nil {
		t.Error("UpdateUserEmail() should return error on server error")
	}
}

func TestUpdateUserNonceInvalidUserID(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.UpdateUserNonce(context.Background(), "", "nonce")
	if err == nil {
		t.Error("UpdateUserNonce() should return error for empty user ID")
	}
}

func TestUpdateUserNonceSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("Method = %s, want PATCH", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.UpdateUserNonce(context.Background(), "user-123", "new-nonce")
	if err != nil {
		t.Fatalf("UpdateUserNonce() error = %v", err)
	}
}

func TestUpdateUserNonceRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	err := repo.UpdateUserNonce(context.Background(), "user-123", "nonce")
	if err == nil {
		t.Error("UpdateUserNonce() should return error on server error")
	}
}

func TestHealthCheckSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]User{})
	})
	defer cleanup()

	err := repo.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}
}

func TestHealthCheckRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	err := repo.HealthCheck(context.Background())
	if err == nil {
		t.Error("HealthCheck() should return error on server error")
	}
}

func TestHealthCheckNilRepository(t *testing.T) {
	var repo *Repository
	err := repo.HealthCheck(context.Background())
	if err == nil {
		t.Error("HealthCheck() should return error for nil repository")
	}
}

// =============================================================================
// Service Request Tests
// =============================================================================

func TestCreateServiceRequestNil(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.CreateServiceRequest(context.Background(), nil)
	if err == nil {
		t.Error("CreateServiceRequest() should return error for nil request")
	}
}

func TestCreateServiceRequestSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Method = %s, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]ServiceRequest{{ID: "req-1"}})
	})
	defer cleanup()

	req := &ServiceRequest{ID: "req-123", UserID: "user-123", ServiceType: "test"}
	err := repo.CreateServiceRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateServiceRequest() error = %v", err)
	}
}

func TestCreateServiceRequestRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	req := &ServiceRequest{ID: "req-123", UserID: "user-123", ServiceType: "test"}
	err := repo.CreateServiceRequest(context.Background(), req)
	if err == nil {
		t.Error("CreateServiceRequest() should return error on server error")
	}
}

func TestUpdateServiceRequestNil(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.UpdateServiceRequest(context.Background(), nil)
	if err == nil {
		t.Error("UpdateServiceRequest() should return error for nil request")
	}
}

func TestUpdateServiceRequestInvalidID(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	req := &ServiceRequest{ID: ""}
	err := repo.UpdateServiceRequest(context.Background(), req)
	if err == nil {
		t.Error("UpdateServiceRequest() should return error for empty ID")
	}
}

func TestUpdateServiceRequestSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("Method = %s, want PATCH", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	req := &ServiceRequest{ID: "req-123", Status: "completed"}
	err := repo.UpdateServiceRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("UpdateServiceRequest() error = %v", err)
	}
}

func TestUpdateServiceRequestRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	req := &ServiceRequest{ID: "req-123", Status: "completed"}
	err := repo.UpdateServiceRequest(context.Background(), req)
	if err == nil {
		t.Error("UpdateServiceRequest() should return error on server error")
	}
}

// =============================================================================
// Price Feed Tests
// =============================================================================

func TestCreatePriceFeedNil(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.CreatePriceFeed(context.Background(), nil)
	if err == nil {
		t.Error("CreatePriceFeed() should return error for nil feed")
	}
}

func TestCreatePriceFeedSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Method = %s, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]PriceFeed{{ID: "feed-1"}})
	})
	defer cleanup()

	feed := &PriceFeed{FeedID: "feed-123", Pair: "NEO/USD", Price: 1050, Timestamp: time.Now()}
	err := repo.CreatePriceFeed(context.Background(), feed)
	if err != nil {
		t.Fatalf("CreatePriceFeed() error = %v", err)
	}
}

func TestCreatePriceFeedRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	feed := &PriceFeed{FeedID: "feed-123", Pair: "NEO/USD", Price: 1050, Timestamp: time.Now()}
	err := repo.CreatePriceFeed(context.Background(), feed)
	if err == nil {
		t.Error("CreatePriceFeed() should return error on server error")
	}
}

// =============================================================================
// Wallet Operations Tests
// =============================================================================

func TestCreateWalletNil(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.CreateWallet(context.Background(), nil)
	if err == nil {
		t.Error("CreateWallet() should return error for nil wallet")
	}
}

func TestCreateWalletInvalidUserID(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	wallet := &UserWallet{UserID: ""}
	err := repo.CreateWallet(context.Background(), wallet)
	if err == nil {
		t.Error("CreateWallet() should return error for empty user ID")
	}
}

func TestCreateWalletSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Method = %s, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]UserWallet{{ID: "wallet-1"}})
	})
	defer cleanup()

	wallet := &UserWallet{UserID: "user-123", Address: "NXV7ZhHiyM1aHXwpVsRZC6BEaDPjpfbNGN"}
	err := repo.CreateWallet(context.Background(), wallet)
	if err != nil {
		t.Fatalf("CreateWallet() error = %v", err)
	}
	if wallet.ID != "wallet-1" {
		t.Errorf("wallet.ID = %q, want %q", wallet.ID, "wallet-1")
	}
}

func TestCreateWalletRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	wallet := &UserWallet{UserID: "user-123", Address: "NXV7ZhHiyM1aHXwpVsRZC6BEaDPjpfbNGN"}
	err := repo.CreateWallet(context.Background(), wallet)
	if err == nil {
		t.Error("CreateWallet() should return error on server error")
	}
}

func TestCreateWalletUnmarshalError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	})
	defer cleanup()

	wallet := &UserWallet{UserID: "user-123", Address: "NXV7ZhHiyM1aHXwpVsRZC6BEaDPjpfbNGN"}
	err := repo.CreateWallet(context.Background(), wallet)
	if err == nil {
		t.Error("CreateWallet() should return error for invalid JSON")
	}
}

func TestGetUserWalletsInvalidUserID(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	_, err := repo.GetUserWallets(context.Background(), "")
	if err == nil {
		t.Error("GetUserWallets() should return error for empty user ID")
	}
}

func TestGetUserWalletsSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]UserWallet{
			{ID: "wallet-1", UserID: "user-123"},
			{ID: "wallet-2", UserID: "user-123"},
		})
	})
	defer cleanup()

	wallets, err := repo.GetUserWallets(context.Background(), "user-123")
	if err != nil {
		t.Fatalf("GetUserWallets() error = %v", err)
	}
	if len(wallets) != 2 {
		t.Errorf("len(wallets) = %d, want 2", len(wallets))
	}
}

func TestGetUserWalletsRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	_, err := repo.GetUserWallets(context.Background(), "user-123")
	if err == nil {
		t.Error("GetUserWallets() should return error on server error")
	}
}

func TestGetUserWalletsUnmarshalError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	})
	defer cleanup()

	_, err := repo.GetUserWallets(context.Background(), "user-123")
	if err == nil {
		t.Error("GetUserWallets() should return error for invalid JSON")
	}
}

func TestGetWalletByAddressInvalid(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	_, err := repo.GetWalletByAddress(context.Background(), "invalid")
	if err == nil {
		t.Error("GetWalletByAddress() should return error for invalid address")
	}
}

func TestGetWalletByAddressSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]UserWallet{{ID: "wallet-1", Address: "NXV7ZhHiyM1aHXwpVsRZC6BEaDPjpfbNGN"}})
	})
	defer cleanup()

	wallet, err := repo.GetWalletByAddress(context.Background(), "NXV7ZhHiyM1aHXwpVsRZC6BEaDPjpfbNGN")
	if err != nil {
		t.Fatalf("GetWalletByAddress() error = %v", err)
	}
	if wallet.ID != "wallet-1" {
		t.Errorf("wallet.ID = %q, want %q", wallet.ID, "wallet-1")
	}
}

func TestGetWalletByAddressNotFound(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]UserWallet{})
	})
	defer cleanup()

	_, err := repo.GetWalletByAddress(context.Background(), "NXV7ZhHiyM1aHXwpVsRZC6BEaDPjpfbNGN")
	if err == nil {
		t.Error("GetWalletByAddress() should return error for not found")
	}
	if !IsNotFound(err) {
		t.Error("error should be NotFoundError")
	}
}

func TestGetWalletByAddressRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	_, err := repo.GetWalletByAddress(context.Background(), "NXV7ZhHiyM1aHXwpVsRZC6BEaDPjpfbNGN")
	if err == nil {
		t.Error("GetWalletByAddress() should return error on server error")
	}
}

func TestGetWalletByAddressUnmarshalError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	})
	defer cleanup()

	_, err := repo.GetWalletByAddress(context.Background(), "NXV7ZhHiyM1aHXwpVsRZC6BEaDPjpfbNGN")
	if err == nil {
		t.Error("GetWalletByAddress() should return error for invalid JSON")
	}
}

func TestGetWalletInvalidWalletID(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	_, err := repo.GetWallet(context.Background(), "", "user-123")
	if err == nil {
		t.Error("GetWallet() should return error for empty wallet ID")
	}
}

func TestGetWalletInvalidUserID(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	_, err := repo.GetWallet(context.Background(), "wallet-123", "")
	if err == nil {
		t.Error("GetWallet() should return error for empty user ID")
	}
}

func TestGetWalletSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]UserWallet{{ID: "wallet-123", UserID: "user-123"}})
	})
	defer cleanup()

	wallet, err := repo.GetWallet(context.Background(), "wallet-123", "user-123")
	if err != nil {
		t.Fatalf("GetWallet() error = %v", err)
	}
	if wallet.ID != "wallet-123" {
		t.Errorf("wallet.ID = %q, want %q", wallet.ID, "wallet-123")
	}
}

func TestGetWalletNotFound(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]UserWallet{})
	})
	defer cleanup()

	_, err := repo.GetWallet(context.Background(), "wallet-123", "user-123")
	if err == nil {
		t.Error("GetWallet() should return error for not found")
	}
	if !IsNotFound(err) {
		t.Error("error should be NotFoundError")
	}
}

func TestGetWalletRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	_, err := repo.GetWallet(context.Background(), "wallet-123", "user-123")
	if err == nil {
		t.Error("GetWallet() should return error on server error")
	}
}

func TestGetWalletUnmarshalError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	})
	defer cleanup()

	_, err := repo.GetWallet(context.Background(), "wallet-123", "user-123")
	if err == nil {
		t.Error("GetWallet() should return error for invalid JSON")
	}
}

func TestSetPrimaryWalletSuccess(t *testing.T) {
	callCount := 0
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.Method != "PATCH" {
			t.Errorf("Method = %s, want PATCH", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.SetPrimaryWallet(context.Background(), "user-123", "wallet-123")
	if err != nil {
		t.Fatalf("SetPrimaryWallet() error = %v", err)
	}
	if callCount != 2 {
		t.Errorf("callCount = %d, want 2 (unset all + set primary)", callCount)
	}
}

func TestSetPrimaryWalletFirstRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	err := repo.SetPrimaryWallet(context.Background(), "user-123", "wallet-123")
	if err == nil {
		t.Error("SetPrimaryWallet() should return error on server error")
	}
}

func TestVerifyWalletSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("Method = %s, want PATCH", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.VerifyWallet(context.Background(), "wallet-123", "signature")
	if err != nil {
		t.Fatalf("VerifyWallet() error = %v", err)
	}
}

func TestVerifyWalletRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	err := repo.VerifyWallet(context.Background(), "wallet-123", "signature")
	if err == nil {
		t.Error("VerifyWallet() should return error on server error")
	}
}

func TestDeleteWalletSuccess(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("Method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	err := repo.DeleteWallet(context.Background(), "wallet-123", "user-123")
	if err != nil {
		t.Fatalf("DeleteWallet() error = %v", err)
	}
}

func TestDeleteWalletRequestError(t *testing.T) {
	repo, cleanup := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	err := repo.DeleteWallet(context.Background(), "wallet-123", "user-123")
	if err == nil {
		t.Error("DeleteWallet() should return error on server error")
	}
}

// Helper function (reused from generic_repository_test.go)
func setupExtendedTestServer(t *testing.T, handler http.HandlerFunc) (*Repository, func()) {
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
