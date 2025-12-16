// Package contract provides end-to-end tests with deployed contracts.
package contract

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	neoaccounts "github.com/R3E-Network/service_layer/infrastructure/accountpool/marble"
	"github.com/R3E-Network/service_layer/infrastructure/marble"
	neocompute "github.com/R3E-Network/service_layer/services/confcompute/marble"
)

// TestE2ENeoComputeFlow tests a minimal confidential compute flow.
func TestE2ENeoComputeFlow(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neocompute"})
	m.SetTestSecret("COMPUTE_MASTER_KEY", []byte("e2e-compute-master-key-32-bytes!!!"))

	svc, err := neocompute.New(neocompute.Config{Marble: m})
	if err != nil {
		t.Fatalf("neocompute.New: %v", err)
	}

	server := httptest.NewServer(svc.Router())
	defer server.Close()

	t.Run("execute script flow", func(t *testing.T) {
		request := neocompute.ExecuteRequest{
			Script: `
function main() {
  return { random_hex: crypto.randomBytes(32) };
}
`,
			EntryPoint: "main",
		}

		body, err := json.Marshal(request)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}

		httpReq, err := http.NewRequest(http.MethodPost, server.URL+"/execute", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("X-User-ID", "user-123")

		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			t.Fatalf("post /execute: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
		}

		var computeResp neocompute.ExecuteResponse
		if err := json.NewDecoder(resp.Body).Decode(&computeResp); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if computeResp.Status != "completed" {
			t.Fatalf("expected status 'completed', got %q (error=%q)", computeResp.Status, computeResp.Error)
		}
		if computeResp.Output == nil {
			t.Fatalf("expected output to be non-nil")
		}
		if _, ok := computeResp.Output["random_hex"]; !ok {
			t.Fatalf("expected output.random_hex to be present, got %#v", computeResp.Output)
		}
		t.Logf("NeoCompute output: %#v", computeResp.Output)
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
			{2, "NeoFeedsService", "Deploy data feeds contract", false},
			{3, "NeoFlowService", "Deploy neoflow contract", false},
			{4, "ConfidentialService", "Deploy confidential compute contract", false},
			{5, "OracleService", "Deploy confidential oracle contract", false},
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
			{"neofeeds", "0x1111111111111111111111111111111111111111", 5000000, "0xTEE1"},
			{"neoflow", "0x2222222222222222222222222222222222222222", 20000000, "0xTEE1"},
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

	computeMarble, _ := marble.New(marble.Config{MarbleType: "neocompute"})
	computeMarble.SetTestSecret("COMPUTE_MASTER_KEY", []byte("concurrent-e2e-compute-key-32bytes"))
	computeSvc, _ := neocompute.New(neocompute.Config{Marble: computeMarble})

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
			computeSvc.Router().ServeHTTP(w, req)
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

	if success != 60 {
		t.Errorf("expected 60 successful operations, got %d", success)
	}
}

// TestE2EErrorRecovery tests error recovery scenarios.
func TestE2EErrorRecovery(t *testing.T) {
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
			name:       "NeoCompute",
			marbleType: "neocompute",
			secretKey:  "COMPUTE_MASTER_KEY",
			secretVal:  []byte("metadata-test-compute-key-32-bytes!"),
			createFunc: func(m *marble.Marble) (interface{ ID() string }, error) {
				return neocompute.New(neocompute.Config{Marble: m})
			},
			expectedID: "neocompute",
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
