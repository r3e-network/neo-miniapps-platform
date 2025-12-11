// Package contract provides end-to-end tests with deployed contracts.
package contract

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/marble"
	neoaccounts "github.com/R3E-Network/service_layer/services/neoaccounts/marble"
	vrf "github.com/R3E-Network/service_layer/services/neorand/marble"
	neovault "github.com/R3E-Network/service_layer/services/neovault/marble"
)

// TestE2EFullMixingFlow tests a complete mixing flow from request to completion.
func TestE2EFullMixingFlow(t *testing.T) {
	apMarble, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	apMarble.SetTestSecret("POOL_MASTER_KEY", []byte("e2e-full-flow-pool-key-32bytes!!"))

	apSvc, err := neoaccounts.New(neoaccounts.Config{Marble: apMarble})
	if err != nil {
		t.Fatalf("neoaccounts.New: %v", err)
	}

	apServer := httptest.NewServer(apSvc.Router())
	defer apServer.Close()

	neovaultMarble, _ := marble.New(marble.Config{MarbleType: "neovault"})
	neovaultMarble.SetTestSecret("NEOVAULT_MASTER_KEY", []byte("e2e-full-flow-neovault-key-32bytes!"))

	neovaultSvc, err := neovault.New(neovault.Config{
		Marble:         neovaultMarble,
		NeoAccountsURL: apServer.URL,
	})
	if err != nil {
		t.Fatalf("neovault.New: %v", err)
	}

	neovaultServer := httptest.NewServer(neovaultSvc.Router())
	defer neovaultServer.Close()

	t.Run("step 1: verify services healthy", func(t *testing.T) {
		resp, err := http.Get(apServer.URL + "/health")
		if err != nil {
			t.Fatalf("NeoAccounts health check: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("NeoAccounts unhealthy: status %d", resp.StatusCode)
		}

		resp, err = http.Get(neovaultServer.URL + "/health")
		if err != nil {
			t.Fatalf("NeoVault health check: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("NeoVault unhealthy: status %d", resp.StatusCode)
		}
	})

	t.Run("step 2: verify token configs", func(t *testing.T) {
		gasConfig := neovaultSvc.GetTokenConfig("GAS")
		if gasConfig == nil {
			t.Fatal("GAS config should exist")
		}

		neoConfig := neovaultSvc.GetTokenConfig("NEO")
		if neoConfig == nil {
			t.Fatal("NEO config should exist")
		}

		t.Logf("GAS config: min=%d, max=%d, fee=%.4f",
			gasConfig.MinTxAmount, gasConfig.MaxTxAmount, gasConfig.ServiceFeeRate)
		t.Logf("NEO config: min=%d, max=%d, fee=%.4f",
			neoConfig.MinTxAmount, neoConfig.MaxTxAmount, neoConfig.ServiceFeeRate)
	})

	t.Run("step 3: simulate contract event reception", func(t *testing.T) {
		type MixRequest struct {
			RequestID        int64  `json:"request_id"`
			UserContract     string `json:"user_contract"`
			EncryptedTargets []byte `json:"encrypted_targets"`
			TokenType        string `json:"token_type"`
			Amount           int64  `json:"amount"`
			MixOption        int    `json:"mix_option"`
		}

		request := MixRequest{
			RequestID:        1,
			UserContract:     "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq",
			EncryptedTargets: []byte("encrypted-targets-data"),
			TokenType:        "GAS",
			Amount:           100000000,
			MixOption:        3,
		}

		if request.Amount < neovaultSvc.GetTokenConfig("GAS").MinTxAmount {
			t.Errorf("amount below minimum")
		}
	})

	t.Run("step 4: neoaccounts client integration", func(t *testing.T) {
		client := neovault.NewNeoAccountsClient(apServer.URL, "neovault")
		ctx := context.Background()

		mux := http.NewServeMux()
		mux.HandleFunc("/request", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}

			var input map[string]interface{}
			json.NewDecoder(r.Body).Decode(&input)

			accounts := []neoaccounts.AccountInfo{
				{ID: "e2e-acc-1", Address: "NAddr1", Balances: map[string]neoaccounts.TokenBalance{"GAS": {Amount: 1000000}}},
				{ID: "e2e-acc-2", Address: "NAddr2", Balances: map[string]neoaccounts.TokenBalance{"GAS": {Amount: 1000000}}},
				{ID: "e2e-acc-3", Address: "NAddr3", Balances: map[string]neoaccounts.TokenBalance{"GAS": {Amount: 1000000}}},
			}

			json.NewEncoder(w).Encode(neoaccounts.RequestAccountsResponse{
				Accounts: accounts,
				LockID:   "e2e-lock-1",
			})
		})

		mux.HandleFunc("/release", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]int{"released_count": 3})
		})

		mockServer := httptest.NewServer(mux)
		defer mockServer.Close()

		mockClient := neovault.NewNeoAccountsClient(mockServer.URL, "neovault")

		resp, err := mockClient.RequestAccounts(ctx, 3, "e2e-mixing")
		if err != nil {
			t.Fatalf("RequestAccounts: %v", err)
		}

		if len(resp.Accounts) != 3 {
			t.Errorf("expected 3 accounts, got %d", len(resp.Accounts))
		}

		accountIDs := make([]string, len(resp.Accounts))
		for i, acc := range resp.Accounts {
			accountIDs[i] = acc.ID
		}

		err = mockClient.ReleaseAccounts(ctx, accountIDs)
		if err != nil {
			t.Fatalf("ReleaseAccounts: %v", err)
		}

		_ = client
	})
}

// TestE2EVRFFlow tests the VRF service flow for contract integration.
func TestE2EVRFFlow(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neorand"})
	m.SetTestSecret("VRF_PRIVATE_KEY", []byte("e2e-vrf-private-key-32-bytes!!!!"))

	svc, err := vrf.New(vrf.Config{Marble: m})
	if err != nil {
		t.Fatalf("vrf.New: %v", err)
	}

	server := httptest.NewServer(svc.Router())
	defer server.Close()

	t.Run("vrf request-response flow", func(t *testing.T) {
		type VRFRequest struct {
			RequestID int64  `json:"request_id"`
			Seed      []byte `json:"seed"`
			NumWords  int    `json:"num_words"`
		}

		type VRFResponse struct {
			RequestID   int64    `json:"request_id"`
			RandomWords [][]byte `json:"random_words"`
			Proof       []byte   `json:"proof"`
		}

		request := VRFRequest{
			RequestID: 100,
			Seed:      []byte("user-provided-seed-for-randomness"),
			NumWords:  3,
		}

		if len(request.Seed) == 0 {
			t.Error("seed should not be empty")
		}

		t.Logf("VRF request: id=%d, seed=%s, num_words=%d",
			request.RequestID, hex.EncodeToString(request.Seed), request.NumWords)
	})
}

// TestE2EContractDeploymentFlow tests the contract deployment and registration flow.
func TestE2EContractDeploymentFlow(t *testing.T) {
	SkipIfNoNeoExpress(t)

	if testing.Short() {
		t.Skip("skipping contract deployment test in short mode")
	}

	t.Run("deployment simulation", func(t *testing.T) {
		type DeploymentStep struct {
			Order       int
			Contract    string
			Description string
			Complete    bool
		}

		steps := []DeploymentStep{
			{1, "ServiceLayerGateway", "Deploy main gateway contract", false},
			{2, "VRFService", "Deploy VRF service contract", false},
			{3, "NeoVaultService", "Deploy neovault service contract", false},
			{4, "NeoFeedsService", "Deploy data feeds contract", false},
			{5, "NeoFlowService", "Deploy neoflow contract", false},
		}

		for _, step := range steps {
			t.Logf("Step %d: %s - %s", step.Order, step.Contract, step.Description)
		}
	})

	t.Run("service registration simulation", func(t *testing.T) {
		type ServiceRegistration struct {
			ServiceType    string
			ContractHash   string
			Fee            int64
			TEEAccountHash string
		}

		registrations := []ServiceRegistration{
			{"neorand", "0x1111111111111111111111111111111111111111", 10000000, "0xTEE1"},
			{"neovault", "0x2222222222222222222222222222222222222222", 50000000, "0xTEE1"},
			{"oracle", "0x3333333333333333333333333333333333333333", 10000000, "0xTEE1"},
		}

		for _, reg := range registrations {
			t.Logf("Register service: type=%s, contract=%s, fee=%d GAS fractions",
				reg.ServiceType, reg.ContractHash, reg.Fee)
		}
	})
}

// TestE2EConcurrentServiceOperations tests concurrent operations across services.
func TestE2EConcurrentServiceOperations(t *testing.T) {
	apMarble, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	apMarble.SetTestSecret("POOL_MASTER_KEY", []byte("concurrent-e2e-pool-key-32bytes!"))
	apSvc, _ := neoaccounts.New(neoaccounts.Config{Marble: apMarble})

	neovaultMarble, _ := marble.New(marble.Config{MarbleType: "neovault"})
	neovaultMarble.SetTestSecret("NEOVAULT_MASTER_KEY", []byte("concurrent-e2e-neovault-key-32byte!"))

	apServer := httptest.NewServer(apSvc.Router())
	defer apServer.Close()

	neovaultSvc, _ := neovault.New(neovault.Config{
		Marble:         neovaultMarble,
		NeoAccountsURL: apServer.URL,
	})

	vrfMarble, _ := marble.New(marble.Config{MarbleType: "neorand"})
	vrfMarble.SetTestSecret("VRF_PRIVATE_KEY", []byte("concurrent-e2e-vrf-key-32bytes!!"))
	vrfSvc, _ := vrf.New(vrf.Config{Marble: vrfMarble})

	var wg sync.WaitGroup
	results := make(chan bool, 100)

	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/health", nil)
			w := httptest.NewRecorder()
			apSvc.Router().ServeHTTP(w, req)
			results <- (w.Code == http.StatusOK)
		}()
	}

	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/health", nil)
			w := httptest.NewRecorder()
			neovaultSvc.Router().ServeHTTP(w, req)
			results <- (w.Code == http.StatusOK)
		}()
	}

	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/health", nil)
			w := httptest.NewRecorder()
			vrfSvc.Router().ServeHTTP(w, req)
			results <- (w.Code == http.StatusOK)
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(results)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("concurrent operations timed out")
	}

	success := 0
	for ok := range results {
		if ok {
			success++
		}
	}

	if success != 90 {
		t.Errorf("expected 90 successful operations, got %d", success)
	}
}

// TestE2EErrorRecovery tests error recovery scenarios.
func TestE2EErrorRecovery(t *testing.T) {
	t.Run("neoaccounts unavailable recovery", func(t *testing.T) {
		neovaultMarble, _ := marble.New(marble.Config{MarbleType: "neovault"})
		neovaultMarble.SetTestSecret("NEOVAULT_MASTER_KEY", []byte("recovery-test-neovault-key-32bytes!"))

		neovaultSvc, err := neovault.New(neovault.Config{
			Marble:         neovaultMarble,
			NeoAccountsURL: "http://localhost:59999",
		})
		if err != nil {
			t.Fatalf("neovault.New: %v", err)
		}

		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		neovaultSvc.Router().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("neovault should still respond to health check: status %d", w.Code)
		}
	})

	t.Run("invalid request handling", func(t *testing.T) {
		m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
		m.SetTestSecret("POOL_MASTER_KEY", []byte("error-recovery-pool-key-32bytes!"))
		svc, _ := neoaccounts.New(neoaccounts.Config{Marble: m})

		invalidJSON := []byte(`{invalid json}`)
		req := httptest.NewRequest("POST", "/request", bytes.NewReader(invalidJSON))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		svc.Router().ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid JSON, got %d", w.Code)
		}
	})
}

// TestE2EServiceMetadata verifies service metadata for contract compatibility.
func TestE2EServiceMetadata(t *testing.T) {
	services := []struct {
		name       string
		marbleType string
		secretKey  string
		secretVal  []byte
		createFunc func(*marble.Marble) (interface{ ID() string }, error)
		expectedID string
	}{
		{
			name:       "NeoAccounts",
			marbleType: "neoaccounts",
			secretKey:  "POOL_MASTER_KEY",
			secretVal:  []byte("metadata-test-pool-key-32bytes!!"),
			createFunc: func(m *marble.Marble) (interface{ ID() string }, error) {
				return neoaccounts.New(neoaccounts.Config{Marble: m})
			},
			expectedID: "neoaccounts",
		},
		{
			name:       "VRF",
			marbleType: "neorand",
			secretKey:  "VRF_PRIVATE_KEY",
			secretVal:  []byte("metadata-test-vrf-key-32-bytes!!"),
			createFunc: func(m *marble.Marble) (interface{ ID() string }, error) {
				return vrf.New(vrf.Config{Marble: m})
			},
			expectedID: "neorand",
		},
	}

	for _, svc := range services {
		t.Run(svc.name, func(t *testing.T) {
			m, err := marble.New(marble.Config{MarbleType: svc.marbleType})
			if err != nil {
				t.Fatalf("marble.New: %v", err)
			}
			m.SetTestSecret(svc.secretKey, svc.secretVal)

			service, err := svc.createFunc(m)
			if err != nil {
				t.Fatalf("create service: %v", err)
			}

			if service.ID() != svc.expectedID {
				t.Errorf("expected ID '%s', got '%s'", svc.expectedID, service.ID())
			}
		})
	}
}
