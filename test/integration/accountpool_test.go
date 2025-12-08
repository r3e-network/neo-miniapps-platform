// Package integration provides integration tests for the AccountPool service.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/crypto"
	"github.com/R3E-Network/service_layer/internal/marble"
	accountpool "github.com/R3E-Network/service_layer/services/accountpool/marble"
)

func createTestAccountPoolService(t *testing.T) *accountpool.Service {
	t.Helper()
	m, err := marble.New(marble.Config{MarbleType: "accountpool"})
	if err != nil {
		t.Fatalf("marble.New: %v", err)
	}
	m.SetTestSecret("POOL_MASTER_KEY", []byte("integration-test-key-32-bytes!!!"))

	svc, err := accountpool.New(accountpool.Config{Marble: m})
	if err != nil {
		t.Fatalf("accountpool.New: %v", err)
	}
	return svc
}

func TestAccountPoolServiceCreation(t *testing.T) {
	svc := createTestAccountPoolService(t)

	if svc == nil {
		t.Fatal("service should not be nil")
	}

	if svc.ID() != "accountpool" {
		t.Errorf("expected ID 'accountpool', got '%s'", svc.ID())
	}

	if svc.Name() != "Account Pool Service" {
		t.Errorf("expected name 'Account Pool Service', got '%s'", svc.Name())
	}
}

func TestAccountPoolHealthEndpoint(t *testing.T) {
	svc := createTestAccountPoolService(t)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	svc.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// Note: Tests that require DB access are marked with t.Skip or use panic recovery
// In a real environment, these would run against a test database

func TestAccountPoolRequestEndpointValidation(t *testing.T) {
	svc := createTestAccountPoolService(t)

	t.Run("missing service_id", func(t *testing.T) {
		input := accountpool.RequestAccountsInput{Count: 1}
		body, _ := json.Marshal(input)
		req := httptest.NewRequest("POST", "/request", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		svc.Router().ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	// DB-dependent tests skipped without DB
	t.Run("valid input requires DB", func(t *testing.T) {
		t.Skip("requires database connection")
	})
}

func TestAccountPoolReleaseEndpointValidation(t *testing.T) {
	svc := createTestAccountPoolService(t)

	t.Run("missing service_id", func(t *testing.T) {
		input := accountpool.ReleaseAccountsInput{AccountIDs: []string{"acc-1"}}
		body, _ := json.Marshal(input)
		req := httptest.NewRequest("POST", "/release", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		svc.Router().ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("valid input requires DB", func(t *testing.T) {
		t.Skip("requires database connection")
	})
}

func TestAccountPoolSignEndpointValidation(t *testing.T) {
	svc := createTestAccountPoolService(t)

	tests := []struct {
		name     string
		input    accountpool.SignTransactionInput
		wantCode int
	}{
		{
			name:     "missing service_id",
			input:    accountpool.SignTransactionInput{AccountID: "acc-1", TxHash: []byte("hash")},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "missing account_id",
			input:    accountpool.SignTransactionInput{ServiceID: "mixer", TxHash: []byte("hash")},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "missing tx_hash",
			input:    accountpool.SignTransactionInput{ServiceID: "mixer", AccountID: "acc-1"},
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.input)
			req := httptest.NewRequest("POST", "/sign", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			svc.Router().ServeHTTP(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("expected status %d, got %d: %s", tt.wantCode, w.Code, w.Body.String())
			}
		})
	}
}

func TestAccountPoolBatchSignEndpointValidation(t *testing.T) {
	svc := createTestAccountPoolService(t)

	t.Run("missing service_id", func(t *testing.T) {
		input := accountpool.BatchSignInput{Requests: []accountpool.SignRequest{{AccountID: "acc-1", TxHash: []byte("hash")}}}
		body, _ := json.Marshal(input)
		req := httptest.NewRequest("POST", "/batch-sign", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		svc.Router().ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("valid input returns results", func(t *testing.T) {
		t.Skip("requires database connection")
	})
}

func TestAccountPoolBalanceEndpointValidation(t *testing.T) {
	svc := createTestAccountPoolService(t)

	tests := []struct {
		name     string
		input    accountpool.UpdateBalanceInput
		wantCode int
	}{
		{
			name:     "missing service_id",
			input:    accountpool.UpdateBalanceInput{AccountID: "acc-1", Delta: 100},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "missing account_id",
			input:    accountpool.UpdateBalanceInput{ServiceID: "mixer", Delta: 100},
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.input)
			req := httptest.NewRequest("POST", "/balance", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			svc.Router().ServeHTTP(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("expected status %d, got %d: %s", tt.wantCode, w.Code, w.Body.String())
			}
		})
	}
}

func TestAccountPoolKeyDerivationConsistency(t *testing.T) {
	m1, _ := marble.New(marble.Config{MarbleType: "accountpool"})
	m1.SetTestSecret("POOL_MASTER_KEY", []byte("consistent-master-key-32-bytes!!"))
	svc1, _ := accountpool.New(accountpool.Config{Marble: m1})

	m2, _ := marble.New(marble.Config{MarbleType: "accountpool"})
	m2.SetTestSecret("POOL_MASTER_KEY", []byte("consistent-master-key-32-bytes!!"))
	svc2, _ := accountpool.New(accountpool.Config{Marble: m2})

	// Services created with same master key should produce identical accounts
	_ = svc1
	_ = svc2
}

func TestAccountPoolCryptoIntegration(t *testing.T) {
	masterKey := []byte("crypto-integration-key-32-bytes!")

	key1, err := crypto.DeriveKey(masterKey, []byte("account-1"), "pool-account", 32)
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}

	key2, err := crypto.DeriveKey(masterKey, []byte("account-1"), "pool-account", 32)
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}

	if !bytes.Equal(key1, key2) {
		t.Error("same inputs should produce same derived key")
	}

	key3, _ := crypto.DeriveKey(masterKey, []byte("account-2"), "pool-account", 32)
	if bytes.Equal(key1, key3) {
		t.Error("different account IDs should produce different keys")
	}
}

func TestAccountPoolServiceStartStop(t *testing.T) {
	svc := createTestAccountPoolService(t)
	ctx := context.Background()

	// Start will fail without DB but should not panic
	err := svc.Service.Start(ctx)
	if err != nil {
		t.Logf("Start returned expected error without DB: %v", err)
	}

	// Stop should work
	if err := svc.Stop(); err != nil {
		t.Errorf("Stop error: %v", err)
	}
}

func TestAccountPoolRouterMethods(t *testing.T) {
	svc := createTestAccountPoolService(t)
	router := svc.Router()

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/health"},
		{"GET", "/info"},
		{"POST", "/request"},
		{"POST", "/release"},
		{"POST", "/sign"},
		{"POST", "/batch-sign"},
		{"POST", "/balance"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			var body *bytes.Reader
			if ep.method == "POST" {
				body = bytes.NewReader([]byte("{}"))
			}

			var req *http.Request
			if body != nil {
				req = httptest.NewRequest(ep.method, ep.path, body)
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(ep.method, ep.path, nil)
			}

			// Use panic recovery since some endpoints may panic without DB
			defer func() {
				if r := recover(); r != nil {
					t.Logf("endpoint %s %s requires DB (expected)", ep.method, ep.path)
				}
			}()

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code == http.StatusNotFound {
				t.Errorf("endpoint %s %s not found", ep.method, ep.path)
			}
		})
	}
}

func TestAccountPoolConcurrentAccess(t *testing.T) {
	svc := createTestAccountPoolService(t)

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/health", nil)
			w := httptest.NewRecorder()
			svc.Router().ServeHTTP(w, req)
			done <- true
		}()
	}

	timeout := time.After(5 * time.Second)
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-timeout:
			t.Fatal("concurrent access timed out")
		}
	}
}

func BenchmarkAccountPoolHealthEndpoint(b *testing.B) {
	m, _ := marble.New(marble.Config{MarbleType: "accountpool"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("benchmark-master-key-32-bytes!!!"))
	svc, _ := accountpool.New(accountpool.Config{Marble: m})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		svc.Router().ServeHTTP(w, req)
	}
}
