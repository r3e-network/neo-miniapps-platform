package gasbank

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/R3E-Network/service_layer/internal/database"
)

func setupMockSupabase(t *testing.T, accounts map[string]*database.GasBankAccount) (*httptest.Server, *database.Repository) {
	mu := &sync.Mutex{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		query := r.URL.Query()
		userIDParam := query.Get("user_id")
		userID := ""
		if len(userIDParam) > 3 && userIDParam[:3] == "eq." {
			userID = userIDParam[3:]
		}

		switch r.Method {
		case "GET":
			if userID != "" {
				if acc, ok := accounts[userID]; ok {
					json.NewEncoder(w).Encode([]database.GasBankAccount{*acc})
				} else {
					json.NewEncoder(w).Encode([]database.GasBankAccount{})
				}
			} else {
				json.NewEncoder(w).Encode([]database.GasBankAccount{})
			}
		case "PATCH":
			var update map[string]interface{}
			json.NewDecoder(r.Body).Decode(&update)
			if userID != "" {
				if acc, ok := accounts[userID]; ok {
					if bal, ok := update["balance"].(float64); ok {
						acc.Balance = int64(bal)
					}
					if res, ok := update["reserved"].(float64); ok {
						acc.Reserved = int64(res)
					}
				}
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		default:
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		}
	}))

	client, _ := database.NewClient(database.Config{
		URL:        server.URL,
		ServiceKey: "test-key",
	})
	repo := database.NewRepository(client)

	return server, repo
}

func TestNewManager(t *testing.T) {
	m := NewManager(nil)
	if m == nil {
		t.Fatal("NewManager() returned nil")
	}
	if m.reservations == nil {
		t.Error("reservations map not initialized")
	}
}

func TestDeposit(t *testing.T) {
	accounts := map[string]*database.GasBankAccount{
		"user-123": {
			ID:       "acc-123",
			UserID:   "user-123",
			Balance:  1000000,
			Reserved: 0,
		},
	}

	server, repo := setupMockSupabase(t, accounts)
	defer server.Close()

	m := NewManager(repo)

	err := m.Deposit(context.Background(), "user-123", 500000, "tx-hash-123")
	if err != nil {
		t.Fatalf("Deposit() error = %v", err)
	}

	if accounts["user-123"].Balance != 1500000 {
		t.Errorf("Balance = %d, want 1500000", accounts["user-123"].Balance)
	}
}

func TestWithdraw(t *testing.T) {
	accounts := map[string]*database.GasBankAccount{
		"user-123": {
			ID:       "acc-123",
			UserID:   "user-123",
			Balance:  1000000,
			Reserved: 100000,
		},
	}

	server, repo := setupMockSupabase(t, accounts)
	defer server.Close()

	m := NewManager(repo)

	err := m.Withdraw(context.Background(), "user-123", 500000, "NXV7...")
	if err != nil {
		t.Fatalf("Withdraw() error = %v", err)
	}

	if accounts["user-123"].Balance != 500000 {
		t.Errorf("Balance = %d, want 500000", accounts["user-123"].Balance)
	}
}

func TestWithdrawInsufficientBalance(t *testing.T) {
	accounts := map[string]*database.GasBankAccount{
		"user-123": {
			ID:       "acc-123",
			UserID:   "user-123",
			Balance:  1000000,
			Reserved: 900000,
		},
	}

	server, repo := setupMockSupabase(t, accounts)
	defer server.Close()

	m := NewManager(repo)

	err := m.Withdraw(context.Background(), "user-123", 500000, "NXV7...")
	if err == nil {
		t.Fatal("Withdraw() expected error for insufficient balance")
	}
}

func TestReserve(t *testing.T) {
	accounts := map[string]*database.GasBankAccount{
		"user-123": {
			ID:       "acc-123",
			UserID:   "user-123",
			Balance:  1000000,
			Reserved: 0,
		},
	}

	server, repo := setupMockSupabase(t, accounts)
	defer server.Close()

	m := NewManager(repo)

	reservationID, err := m.Reserve(context.Background(), "user-123", "vrf", "ref-123", 300000)
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}

	if reservationID == "" {
		t.Error("Reserve() returned empty reservation ID")
	}

	if accounts["user-123"].Reserved != 300000 {
		t.Errorf("Reserved = %d, want 300000", accounts["user-123"].Reserved)
	}
}

func TestRelease(t *testing.T) {
	accounts := map[string]*database.GasBankAccount{
		"user-123": {
			ID:       "acc-123",
			UserID:   "user-123",
			Balance:  1000000,
			Reserved: 0,
		},
	}

	server, repo := setupMockSupabase(t, accounts)
	defer server.Close()

	m := NewManager(repo)

	reservationID, _ := m.Reserve(context.Background(), "user-123", "vrf", "ref-123", 100000)

	err := m.Release(context.Background(), "user-123", reservationID)
	if err != nil {
		t.Fatalf("Release() error = %v", err)
	}
}

func TestConsume(t *testing.T) {
	accounts := map[string]*database.GasBankAccount{
		"user-123": {
			ID:       "acc-123",
			UserID:   "user-123",
			Balance:  1000000,
			Reserved: 0,
		},
	}

	server, repo := setupMockSupabase(t, accounts)
	defer server.Close()

	m := NewManager(repo)

	reservationID, _ := m.Reserve(context.Background(), "user-123", "vrf", "ref-123", 100000)

	err := m.Consume(context.Background(), "user-123", reservationID)
	if err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
}

func TestGetServiceFee(t *testing.T) {
	fee := GetServiceFee("vrf")
	if fee != 100000 {
		t.Errorf("GetServiceFee(vrf) = %d, want 100000", fee)
	}

	fee = GetServiceFee("unknown")
	if fee != 0 {
		t.Errorf("GetServiceFee(unknown) = %d, want 0", fee)
	}
}

func TestServiceFees(t *testing.T) {
	expectedServices := []string{"vrf", "automation", "datafeeds", "mixer", "confidential"}
	for _, svc := range expectedServices {
		if _, ok := ServiceFees[svc]; !ok {
			t.Errorf("ServiceFees missing %s", svc)
		}
	}
}
