// Package e2e provides end-to-end tests for service integrations.
package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/marble"
	accountpool "github.com/R3E-Network/service_layer/services/accountpool/marble"
	mixer "github.com/R3E-Network/service_layer/services/mixer/marble"
)

// TestMixerAccountPoolIntegration tests the integration between Mixer and AccountPool services.
func TestMixerAccountPoolIntegration(t *testing.T) {
	// Create AccountPool service
	apMarble, _ := marble.New(marble.Config{MarbleType: "accountpool"})
	apMarble.SetTestSecret("POOL_MASTER_KEY", []byte("e2e-test-pool-master-key-32b!!!"))

	apSvc, err := accountpool.New(accountpool.Config{Marble: apMarble})
	if err != nil {
		t.Fatalf("accountpool.New: %v", err)
	}

	// Start AccountPool HTTP server
	apServer := httptest.NewServer(apSvc.Router())
	defer apServer.Close()

	// Create Mixer service pointing to AccountPool
	mixerMarble, _ := marble.New(marble.Config{MarbleType: "mixer"})
	mixerMarble.SetTestSecret("MIXER_MASTER_KEY", []byte("e2e-test-mixer-master-key-32b!!"))

	mixerSvc, err := mixer.New(mixer.Config{
		Marble:         mixerMarble,
		AccountPoolURL: apServer.URL,
	})
	if err != nil {
		t.Fatalf("mixer.New: %v", err)
	}

	t.Run("mixer service creation with accountpool url", func(t *testing.T) {
		if mixerSvc == nil {
			t.Fatal("mixer service should not be nil")
		}
		if mixerSvc.ID() != "mixer" {
			t.Errorf("expected ID 'mixer', got '%s'", mixerSvc.ID())
		}
	})

	t.Run("accountpool service responds to health check", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		apSvc.Router().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("mixer service responds to health check", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		mixerSvc.Router().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})
}

// TestAccountPoolClientIntegration tests the AccountPoolClient HTTP client.
func TestAccountPoolClientIntegration(t *testing.T) {
	// Create a mock AccountPool server that returns proper responses
	mux := http.NewServeMux()

	mux.HandleFunc("/request", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var input map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		serviceID, _ := input["service_id"].(string)
		if serviceID == "" {
			http.Error(w, "service_id required", http.StatusBadRequest)
			return
		}

		count := int(input["count"].(float64))
		if count <= 0 {
			count = 1
		}

		accounts := make([]accountpool.AccountInfo, count)
		for i := 0; i < count; i++ {
			accounts[i] = accountpool.AccountInfo{
				ID:         "mock-acc-" + string(rune('a'+i)),
				Address:    "NMockAddress" + string(rune('A'+i)),
				Balance:    1000000,
				CreatedAt:  time.Now(),
				LastUsedAt: time.Now(),
				TxCount:    0,
				IsRetiring: false,
				LockedBy:   serviceID,
				LockedAt:   time.Now(),
			}
		}

		resp := accountpool.RequestAccountsResponse{
			Accounts: accounts,
			LockID:   "mock-lock-123",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/release", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var input map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		serviceID, _ := input["service_id"].(string)
		if serviceID == "" {
			http.Error(w, "service_id required", http.StatusBadRequest)
			return
		}

		resp := map[string]int{"released_count": 1}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/balance", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var input map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		serviceID, _ := input["service_id"].(string)
		accountID, _ := input["account_id"].(string)
		if serviceID == "" || accountID == "" {
			http.Error(w, "service_id and account_id required", http.StatusBadRequest)
			return
		}

		resp := map[string]interface{}{
			"account_id":  accountID,
			"old_balance": 1000000,
			"new_balance": 1100000,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := mixer.NewAccountPoolClient(server.URL, "mixer")

	t.Run("request accounts", func(t *testing.T) {
		ctx := context.Background()
		resp, err := client.RequestAccounts(ctx, 3, "test-mixing")
		if err != nil {
			t.Fatalf("RequestAccounts: %v", err)
		}

		if len(resp.Accounts) != 3 {
			t.Errorf("expected 3 accounts, got %d", len(resp.Accounts))
		}
		if resp.LockID == "" {
			t.Error("lock_id should not be empty")
		}
		for _, acc := range resp.Accounts {
			if acc.LockedBy != "mixer" {
				t.Errorf("expected locked_by 'mixer', got '%s'", acc.LockedBy)
			}
		}
	})

	t.Run("release accounts", func(t *testing.T) {
		ctx := context.Background()
		err := client.ReleaseAccounts(ctx, []string{"mock-acc-a", "mock-acc-b"})
		if err != nil {
			t.Fatalf("ReleaseAccounts: %v", err)
		}
	})

	t.Run("update balance", func(t *testing.T) {
		ctx := context.Background()
		err := client.UpdateBalance(ctx, "mock-acc-a", 100000, nil)
		if err != nil {
			t.Fatalf("UpdateBalance: %v", err)
		}
	})

	t.Run("update balance absolute", func(t *testing.T) {
		ctx := context.Background()
		absolute := int64(500000)
		err := client.UpdateBalance(ctx, "mock-acc-a", 0, &absolute)
		if err != nil {
			t.Fatalf("UpdateBalance absolute: %v", err)
		}
	})
}

// TestAccountPoolClientErrorHandling tests error scenarios.
func TestAccountPoolClientErrorHandling(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/request", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no accounts available", http.StatusInternalServerError)
	})

	mux.HandleFunc("/release", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "account not locked by service", http.StatusBadRequest)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := mixer.NewAccountPoolClient(server.URL, "mixer")
	ctx := context.Background()

	t.Run("request accounts error", func(t *testing.T) {
		_, err := client.RequestAccounts(ctx, 1, "test")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("release accounts error", func(t *testing.T) {
		err := client.ReleaseAccounts(ctx, []string{"acc-1"})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

// TestAccountPoolClientTimeout tests timeout handling.
func TestAccountPoolClientTimeout(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/request", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := mixer.NewAccountPoolClient(server.URL, "mixer")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := client.RequestAccounts(ctx, 1, "test")
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

// TestMixerTokenConfigs tests mixer token configuration.
func TestMixerTokenConfigs(t *testing.T) {
	mixerMarble, _ := marble.New(marble.Config{MarbleType: "mixer"})
	mixerMarble.SetTestSecret("MIXER_MASTER_KEY", []byte("e2e-test-mixer-master-key-32b!!"))

	mixerSvc, _ := mixer.New(mixer.Config{
		Marble:         mixerMarble,
		AccountPoolURL: "http://localhost:8081",
	})

	t.Run("default token configs", func(t *testing.T) {
		tokens := mixerSvc.GetSupportedTokens()
		if len(tokens) < 2 {
			t.Errorf("expected at least 2 supported tokens, got %d", len(tokens))
		}
	})

	t.Run("get GAS config", func(t *testing.T) {
		cfg := mixerSvc.GetTokenConfig("GAS")
		if cfg == nil {
			t.Fatal("GAS config should not be nil")
		}
		if cfg.TokenType != "GAS" {
			t.Errorf("expected token type 'GAS', got '%s'", cfg.TokenType)
		}
		if cfg.ServiceFeeRate <= 0 {
			t.Error("service fee rate should be positive")
		}
	})

	t.Run("get NEO config", func(t *testing.T) {
		cfg := mixerSvc.GetTokenConfig("NEO")
		if cfg == nil {
			t.Fatal("NEO config should not be nil")
		}
		if cfg.TokenType != "NEO" {
			t.Errorf("expected token type 'NEO', got '%s'", cfg.TokenType)
		}
	})

	t.Run("unknown token returns default", func(t *testing.T) {
		cfg := mixerSvc.GetTokenConfig("UNKNOWN")
		if cfg == nil {
			t.Fatal("should return default config for unknown token")
		}
	})
}

// TestE2EServiceCoordination tests coordinated behavior between services.
func TestE2EServiceCoordination(t *testing.T) {
	// Simulate a full mixing flow with mocked services

	// Step 1: Create AccountPool mock that tracks state
	lockedAccounts := make(map[string]string) // accountID -> serviceID
	var mu sync.Mutex

	mux := http.NewServeMux()
	mux.HandleFunc("/request", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		var input map[string]interface{}
		json.NewDecoder(r.Body).Decode(&input)
		serviceID, _ := input["service_id"].(string)
		count := int(input["count"].(float64))

		accounts := make([]accountpool.AccountInfo, count)
		for i := 0; i < count; i++ {
			accID := fmt.Sprintf("acc-%d", len(lockedAccounts)+i)
			lockedAccounts[accID] = serviceID
			accounts[i] = accountpool.AccountInfo{
				ID:       accID,
				Address:  "NAddr" + accID,
				Balance:  1000000,
				LockedBy: serviceID,
			}
		}

		json.NewEncoder(w).Encode(accountpool.RequestAccountsResponse{
			Accounts: accounts,
			LockID:   "lock-" + serviceID,
		})
	})

	mux.HandleFunc("/release", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		var input map[string]interface{}
		json.NewDecoder(r.Body).Decode(&input)
		serviceID, _ := input["service_id"].(string)
		accountIDs, _ := input["account_ids"].([]interface{})

		released := 0
		for _, aid := range accountIDs {
			accID, _ := aid.(string)
			if lockedAccounts[accID] == serviceID {
				delete(lockedAccounts, accID)
				released++
			}
		}

		json.NewEncoder(w).Encode(map[string]int{"released_count": released})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := mixer.NewAccountPoolClient(server.URL, "mixer")
	ctx := context.Background()

	t.Run("request-use-release flow", func(t *testing.T) {
		// Request accounts
		resp, err := client.RequestAccounts(ctx, 5, "mixing")
		if err != nil {
			t.Fatalf("RequestAccounts: %v", err)
		}
		if len(resp.Accounts) != 5 {
			t.Fatalf("expected 5 accounts, got %d", len(resp.Accounts))
		}

		// Verify all locked
		mu.Lock()
		if len(lockedAccounts) != 5 {
			t.Errorf("expected 5 locked accounts, got %d", len(lockedAccounts))
		}
		mu.Unlock()

		// Get account IDs
		accountIDs := make([]string, len(resp.Accounts))
		for i, acc := range resp.Accounts {
			accountIDs[i] = acc.ID
		}

		// Release accounts
		err = client.ReleaseAccounts(ctx, accountIDs)
		if err != nil {
			t.Fatalf("ReleaseAccounts: %v", err)
		}

		// Verify all released
		mu.Lock()
		if len(lockedAccounts) != 0 {
			t.Errorf("expected 0 locked accounts after release, got %d", len(lockedAccounts))
		}
		mu.Unlock()
	})
}
