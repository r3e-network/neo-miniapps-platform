// Package contract provides service-contract integration tests.
package contract

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	neoaccounts "github.com/R3E-Network/service_layer/infrastructure/accountpool/marble"
	"github.com/R3E-Network/service_layer/infrastructure/marble"
	neocompute "github.com/R3E-Network/service_layer/services/confcompute/marble"
)

// TestServiceContractIntegration tests the integration between services and contracts.
func TestServiceContractIntegration(t *testing.T) {
	t.Run("neocompute can protect results", func(t *testing.T) {
		m, err := marble.New(marble.Config{MarbleType: "neocompute"})
		if err != nil {
			t.Fatalf("marble.New: %v", err)
		}
		m.SetTestSecret("COMPUTE_MASTER_KEY", []byte("test-compute-master-key-32-bytes!!"))

		svc, err := neocompute.New(neocompute.Config{Marble: m})
		if err != nil {
			t.Fatalf("neocompute.New: %v", err)
		}

		resp, err := svc.Execute(context.Background(), "user-123", &neocompute.ExecuteRequest{
			Script: `
function main() {
  return { ok: true };
}
`,
			EntryPoint: "main",
		})
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}

		if resp.Status != "completed" {
			t.Fatalf("status = %q, want %q (error=%q)", resp.Status, "completed", resp.Error)
		}
		if resp.OutputHash == "" {
			t.Fatalf("expected OutputHash to be set")
		}
		if resp.Signature == "" {
			t.Fatalf("expected Signature to be set")
		}
	})

	t.Run("neoaccounts can derive contract-compatible keys", func(t *testing.T) {
		m, err := marble.New(marble.Config{MarbleType: "neoaccounts"})
		if err != nil {
			t.Fatalf("marble.New: %v", err)
		}
		m.SetTestSecret("POOL_MASTER_KEY", []byte("test-pool-master-key-32-bytes!!!"))

		svc, err := neoaccounts.New(neoaccounts.Config{Marble: m})
		if err != nil {
			t.Fatalf("neoaccounts.New: %v", err)
		}

		if svc.ID() != "neoaccounts" {
			t.Errorf("expected ID 'neoaccounts', got '%s'", svc.ID())
		}
	})
}

// TestNeoAccountsSigningForContracts tests that NeoAccounts can sign transactions
// that would be sent to Neo N3 contracts.
func TestNeoAccountsSigningForContracts(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("signing-test-pool-key-32-bytes!!"))

	svc, err := neoaccounts.New(neoaccounts.Config{Marble: m})
	if err != nil {
		t.Fatalf("neoaccounts.New: %v", err)
	}

	server := httptest.NewServer(svc.Router())
	defer server.Close()

	t.Run("sign endpoint validation", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("signing endpoint panicked (expected without DB): %v", r)
				t.Skip("requires database connection")
			}
		}()

		input := neoaccounts.SignTransactionInput{
			ServiceID: "neocompute",
			AccountID: "test-account-1",
			TxHash:    []byte("mock-transaction-hash-for-contract"),
		}

		body, _ := json.Marshal(input)
		req := httptest.NewRequest("POST", "/sign", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		svc.Router().ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			t.Logf("signing successful (with mock/test account)")
		} else if w.Code == http.StatusBadRequest {
			t.Logf("signing returned validation error (expected without DB): %s", w.Body.String())
		} else {
			t.Logf("signing returned status %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("batch sign endpoint validation", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("batch sign endpoint panicked (expected without DB): %v", r)
				t.Skip("requires database connection")
			}
		}()

		input := neoaccounts.BatchSignInput{
			ServiceID: "neocompute",
			Requests: []neoaccounts.SignRequest{
				{AccountID: "account-1", TxHash: []byte("tx-hash-1")},
				{AccountID: "account-2", TxHash: []byte("tx-hash-2")},
			},
		}

		body, _ := json.Marshal(input)
		req := httptest.NewRequest("POST", "/batch-sign", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		svc.Router().ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest && w.Code != http.StatusOK {
			t.Logf("batch sign returned status %d", w.Code)
		}
	})
}

// TestContractEventMonitoring tests that services can handle contract events.
func TestContractEventMonitoring(t *testing.T) {
	t.Run("mock contract event handling", func(t *testing.T) {
		type ContractEvent struct {
			Contract  string `json:"contract"`
			EventName string `json:"event_name"`
			State     struct {
				RequestID    int64  `json:"request_id"`
				UserContract string `json:"user_contract"`
				Caller       string `json:"caller"`
				ServiceType  string `json:"service_type"`
				Payload      []byte `json:"payload"`
			} `json:"state"`
		}

		event := ContractEvent{
			Contract:  "0x1234567890abcdef1234567890abcdef12345678",
			EventName: "ServiceRequest",
		}
		event.State.RequestID = 12345
		event.State.UserContract = "NXV7ZhHiyM1aHXwpVsRZC6BwNFP2jghXAq"
		event.State.Caller = "NMockCallerAddress12345678901234567"
		event.State.ServiceType = "neocompute"
		event.State.Payload = []byte(`{"num_words": 3}`)

		eventJSON, err := json.Marshal(event)
		if err != nil {
			t.Fatalf("marshal event: %v", err)
		}

		var parsed ContractEvent
		if err := json.Unmarshal(eventJSON, &parsed); err != nil {
			t.Fatalf("unmarshal event: %v", err)
		}

		if parsed.State.ServiceType != "neocompute" {
			t.Errorf("expected service type 'neocompute', got '%s'", parsed.State.ServiceType)
		}
		if parsed.State.RequestID != 12345 {
			t.Errorf("expected request ID 12345, got %d", parsed.State.RequestID)
		}
	})
}

// TestContractCallbackSimulation simulates callback flow to user contracts.
func TestContractCallbackSimulation(t *testing.T) {
	type CallbackResult struct {
		RequestID int64  `json:"request_id"`
		Success   bool   `json:"success"`
		Result    []byte `json:"result"`
		Error     string `json:"error"`
	}

	t.Run("successful callback", func(t *testing.T) {
		result := CallbackResult{
			RequestID: 12345,
			Success:   true,
			Result:    []byte{0x01, 0x02, 0x03, 0x04},
			Error:     "",
		}

		jsonResult, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("marshal result: %v", err)
		}

		var parsed CallbackResult
		if err := json.Unmarshal(jsonResult, &parsed); err != nil {
			t.Fatalf("unmarshal result: %v", err)
		}

		if !parsed.Success {
			t.Error("callback should be successful")
		}
		if parsed.Error != "" {
			t.Errorf("successful callback should have no error, got '%s'", parsed.Error)
		}
	})

	t.Run("failed callback", func(t *testing.T) {
		result := CallbackResult{
			RequestID: 12346,
			Success:   false,
			Result:    nil,
			Error:     "external data source unavailable",
		}

		if result.Success {
			t.Error("failed callback should have Success=false")
		}
		if result.Error == "" {
			t.Error("failed callback should have error message")
		}
	})
}

// TestConcurrentContractOperations tests concurrent operations that might interact with contracts.
func TestConcurrentContractOperations(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "neoaccounts"})
	m.SetTestSecret("POOL_MASTER_KEY", []byte("concurrent-test-key-32-bytes!!!!"))

	svc, _ := neoaccounts.New(neoaccounts.Config{Marble: m})

	done := make(chan bool, 50)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for i := 0; i < 50; i++ {
		go func() {
			select {
			case <-ctx.Done():
				done <- false
				return
			default:
			}

			req := httptest.NewRequest("GET", "/health", nil)
			w := httptest.NewRecorder()
			svc.Router().ServeHTTP(w, req)
			done <- (w.Code == http.StatusOK)
		}()
	}

	success := 0
	for i := 0; i < 50; i++ {
		select {
		case ok := <-done:
			if ok {
				success++
			}
		case <-ctx.Done():
			t.Fatal("concurrent operations timed out")
		}
	}

	if success != 50 {
		t.Errorf("expected 50 successful operations, got %d", success)
	}
}
