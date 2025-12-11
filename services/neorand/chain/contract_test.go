// Package vrfchain provides VRF-specific chain interaction.
package vrfchain

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/R3E-Network/service_layer/internal/chain"
)

// =============================================================================
// Test Helpers
// =============================================================================

func newTestClient(t *testing.T, handler http.HandlerFunc) *chain.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := chain.NewClient(chain.Config{
		RPCURL:    server.URL,
		NetworkID: 894710606, // TestNet
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

func makeRPCResponse(result interface{}) []byte {
	resultJSON, _ := json.Marshal(result)
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"result":  json.RawMessage(resultJSON),
	}
	data, _ := json.Marshal(resp)
	return data
}

func makeRPCError(code int, message string) []byte {
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}
	data, _ := json.Marshal(resp)
	return data
}

// =============================================================================
// Module Tests
// =============================================================================

func TestModule_ServiceType(t *testing.T) {
	m := &Module{}
	if m.ServiceType() != "neorand" {
		t.Errorf("ServiceType() = %s, want vrf", m.ServiceType())
	}
}

func TestModule_Initialize(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Write(makeRPCResponse(100))
	}
	client := newTestClient(t, handler)

	m := &Module{}
	err := m.Initialize(client, nil, "0x1234567890abcdef")
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if m.Contract() == nil {
		t.Error("Contract() should not be nil after Initialize")
	}
}

func TestModule_Contract(t *testing.T) {
	m := &Module{}
	if m.Contract() != nil {
		t.Error("Contract() should be nil before Initialize")
	}
}

// =============================================================================
// VRFContract Tests
// =============================================================================

func TestNewVRFContract(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Write(makeRPCResponse(100))
	}
	client := newTestClient(t, handler)

	contract := NewVRFContract(client, "0x1234", nil)
	if contract == nil {
		t.Fatal("NewVRFContract() returned nil")
	}
	if contract.contractHash != "0x1234" {
		t.Errorf("contractHash = %s, want 0x1234", contract.contractHash)
	}
}

func TestGetRandomness_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		result := chain.InvokeResult{
			State:       "HALT",
			GasConsumed: "1000000",
			Stack: []chain.StackItem{
				{Type: "ByteString", Value: json.RawMessage(`"746573742d72616e646f6d6e657373"`)}, // hex "test-randomness"
			},
		}
		w.Write(makeRPCResponse(result))
	}
	client := newTestClient(t, handler)
	contract := NewVRFContract(client, "0x1234", nil)

	randomness, err := contract.GetRandomness(context.Background(), big.NewInt(1))
	if err != nil {
		t.Fatalf("GetRandomness() error = %v", err)
	}
	if randomness == nil {
		t.Error("randomness should not be nil")
	}
}

func TestGetRandomness_ExecutionFailed(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		result := chain.InvokeResult{
			State:     "FAULT",
			Exception: "execution failed",
			Stack:     []chain.StackItem{},
		}
		w.Write(makeRPCResponse(result))
	}
	client := newTestClient(t, handler)
	contract := NewVRFContract(client, "0x1234", nil)

	_, err := contract.GetRandomness(context.Background(), big.NewInt(1))
	if err == nil {
		t.Error("GetRandomness() should return error on FAULT state")
	}
}

func TestGetRandomness_EmptyStack(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		result := chain.InvokeResult{
			State:       "HALT",
			GasConsumed: "1000000",
			Stack:       []chain.StackItem{},
		}
		w.Write(makeRPCResponse(result))
	}
	client := newTestClient(t, handler)
	contract := NewVRFContract(client, "0x1234", nil)

	_, err := contract.GetRandomness(context.Background(), big.NewInt(1))
	if err == nil {
		t.Error("GetRandomness() should return error on empty stack")
	}
}

func TestGetRandomness_RPCError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Write(makeRPCError(-100, "contract not found"))
	}
	client := newTestClient(t, handler)
	contract := NewVRFContract(client, "0x1234", nil)

	_, err := contract.GetRandomness(context.Background(), big.NewInt(1))
	if err == nil {
		t.Error("GetRandomness() should return error on RPC error")
	}
}

func TestGetProof_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		result := chain.InvokeResult{
			State:       "HALT",
			GasConsumed: "1000000",
			Stack: []chain.StackItem{
				{Type: "ByteString", Value: json.RawMessage(`"70726f6f662d64617461"`)}, // hex "proof-data"
			},
		}
		w.Write(makeRPCResponse(result))
	}
	client := newTestClient(t, handler)
	contract := NewVRFContract(client, "0x1234", nil)

	proof, err := contract.GetProof(context.Background(), big.NewInt(1))
	if err != nil {
		t.Fatalf("GetProof() error = %v", err)
	}
	if proof == nil {
		t.Error("proof should not be nil")
	}
}

func TestGetProof_ExecutionFailed(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		result := chain.InvokeResult{
			State:     "FAULT",
			Exception: "execution failed",
		}
		w.Write(makeRPCResponse(result))
	}
	client := newTestClient(t, handler)
	contract := NewVRFContract(client, "0x1234", nil)

	_, err := contract.GetProof(context.Background(), big.NewInt(1))
	if err == nil {
		t.Error("GetProof() should return error on FAULT state")
	}
}

func TestGetVRFPublicKey_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		result := chain.InvokeResult{
			State:       "HALT",
			GasConsumed: "1000000",
			Stack: []chain.StackItem{
				{Type: "ByteString", Value: json.RawMessage(`"7075626c69632d6b6579"`)}, // hex "public-key"
			},
		}
		w.Write(makeRPCResponse(result))
	}
	client := newTestClient(t, handler)
	contract := NewVRFContract(client, "0x1234", nil)

	pubKey, err := contract.GetVRFPublicKey(context.Background())
	if err != nil {
		t.Fatalf("GetVRFPublicKey() error = %v", err)
	}
	if pubKey == nil {
		t.Error("pubKey should not be nil")
	}
}

func TestGetVRFPublicKey_ExecutionFailed(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		result := chain.InvokeResult{
			State:     "FAULT",
			Exception: "execution failed",
		}
		w.Write(makeRPCResponse(result))
	}
	client := newTestClient(t, handler)
	contract := NewVRFContract(client, "0x1234", nil)

	_, err := contract.GetVRFPublicKey(context.Background())
	if err == nil {
		t.Error("GetVRFPublicKey() should return error on FAULT state")
	}
}

func TestVerifyProof_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		result := chain.InvokeResult{
			State:       "HALT",
			GasConsumed: "1000000",
			Stack: []chain.StackItem{
				{Type: "Boolean", Value: json.RawMessage(`true`)},
			},
		}
		w.Write(makeRPCResponse(result))
	}
	client := newTestClient(t, handler)
	contract := NewVRFContract(client, "0x1234", nil)

	valid, err := contract.VerifyProof(context.Background(), []byte("seed"), []byte("words"), []byte("proof"))
	if err != nil {
		t.Fatalf("VerifyProof() error = %v", err)
	}
	if !valid {
		t.Error("VerifyProof() = false, want true")
	}
}

func TestVerifyProof_Invalid(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		result := chain.InvokeResult{
			State:       "HALT",
			GasConsumed: "1000000",
			Stack: []chain.StackItem{
				{Type: "Boolean", Value: json.RawMessage(`false`)},
			},
		}
		w.Write(makeRPCResponse(result))
	}
	client := newTestClient(t, handler)
	contract := NewVRFContract(client, "0x1234", nil)

	valid, err := contract.VerifyProof(context.Background(), []byte("seed"), []byte("words"), []byte("proof"))
	if err != nil {
		t.Fatalf("VerifyProof() error = %v", err)
	}
	if valid {
		t.Error("VerifyProof() = true, want false")
	}
}

func TestVerifyProof_ExecutionFailed(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		result := chain.InvokeResult{
			State:     "FAULT",
			Exception: "execution failed",
		}
		w.Write(makeRPCResponse(result))
	}
	client := newTestClient(t, handler)
	contract := NewVRFContract(client, "0x1234", nil)

	_, err := contract.VerifyProof(context.Background(), []byte("seed"), []byte("words"), []byte("proof"))
	if err == nil {
		t.Error("VerifyProof() should return error on FAULT state")
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkGetRandomness(b *testing.B) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		result := chain.InvokeResult{
			State:       "HALT",
			GasConsumed: "1000000",
			Stack: []chain.StackItem{
				{Type: "ByteString", Value: json.RawMessage(`"746573742d72616e646f6d6e657373"`)}, // hex
			},
		}
		w.Write(makeRPCResponse(result))
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	client, _ := chain.NewClient(chain.Config{RPCURL: server.URL})
	contract := NewVRFContract(client, "0x1234", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = contract.GetRandomness(context.Background(), big.NewInt(1))
	}
}
