// Package e2e provides end-to-end tests for service integrations.
package e2e

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/R3E-Network/service_layer/internal/marble"
	neoaccounts "github.com/R3E-Network/service_layer/services/neoaccounts/marble"
	neovault "github.com/R3E-Network/service_layer/services/neovault/marble"
)

// TestNeoVaultNeoAccountsIntegration tests the integration between NeoVault and NeoAccounts services.
func TestNeoVaultNeoAccountsIntegration(t *testing.T) {
	// Create NeoAccounts service
	apMarble, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	apMarble.SetTestSecret("POOL_MASTER_KEY", []byte("e2e-test-pool-master-key-32b!!!"))

	apSvc, err := neoaccounts.New(neoaccounts.Config{Marble: apMarble})
	if err != nil {
		t.Fatalf("neoaccounts.New: %v", err)
	}

	// Start NeoAccounts HTTP server
	apServer := httptest.NewServer(apSvc.Router())
	defer apServer.Close()

	// Create NeoVault service pointing to NeoAccounts
	neovaultMarble, _ := marble.New(marble.Config{MarbleType: "neovault"})
	neovaultMarble.SetTestSecret("NEOVAULT_MASTER_KEY", []byte("e2e-test-neovault-master-key-32b!!"))

	neovaultSvc, err := neovault.New(&neovault.Config{
		Marble:         neovaultMarble,
		NeoAccountsURL: apServer.URL,
	})
	if err != nil {
		t.Fatalf("neovault.New: %v", err)
	}

	t.Run("neovault service creation with neoaccounts url", func(t *testing.T) {
		if neovaultSvc == nil {
			t.Fatal("neovault service should not be nil")
		}
		if neovaultSvc.ID() != "neovault" {
			t.Errorf("expected ID 'neovault', got '%s'", neovaultSvc.ID())
		}
	})

	t.Run("neoaccounts service responds to health check", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		apSvc.Router().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("neovault service responds to health check", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		neovaultSvc.Router().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})
}

// TestNeoVaultTokenConfigs tests neovault token configuration.
func TestNeoVaultTokenConfigs(t *testing.T) {
	neovaultMarble, _ := marble.New(marble.Config{MarbleType: "neovault"})
	neovaultMarble.SetTestSecret("NEOVAULT_MASTER_KEY", []byte("e2e-test-neovault-master-key-32b!!"))

	neovaultSvc, _ := neovault.New(&neovault.Config{
		Marble:         neovaultMarble,
		NeoAccountsURL: "http://localhost:8081",
	})

	t.Run("default token configs", func(t *testing.T) {
		tokens := neovaultSvc.GetSupportedTokens()
		if len(tokens) < 2 {
			t.Errorf("expected at least 2 supported tokens, got %d", len(tokens))
		}
	})

	t.Run("get GAS config", func(t *testing.T) {
		cfg := neovaultSvc.GetTokenConfig("GAS")
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
		cfg := neovaultSvc.GetTokenConfig("NEO")
		if cfg == nil {
			t.Fatal("NEO config should not be nil")
		}
		if cfg.TokenType != "NEO" {
			t.Errorf("expected token type 'NEO', got '%s'", cfg.TokenType)
		}
	})

	t.Run("unknown token returns default", func(t *testing.T) {
		cfg := neovaultSvc.GetTokenConfig("UNKNOWN")
		if cfg == nil {
			t.Fatal("should return default config for unknown token")
		}
	})
}
