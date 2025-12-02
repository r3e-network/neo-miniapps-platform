package tee

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewSysAPI(t *testing.T) {
	vault := newSimulationVault(nil)
	_ = vault.Initialize(context.Background())

	handler := NewOCALLHandler(DefaultOCALLHandlerConfig())
	api := NewSysAPI("test-service", "test-account", vault, handler)

	if api == nil {
		t.Fatal("expected non-nil SysAPI")
	}

	// Verify all sub-APIs are available
	if api.HTTP() == nil {
		t.Error("expected non-nil HTTP API")
	}
	if api.Secrets() == nil {
		t.Error("expected non-nil Secrets API")
	}
	if api.Crypto() == nil {
		t.Error("expected non-nil Crypto API")
	}
	if api.Proof() == nil {
		t.Error("expected non-nil Proof API")
	}
	if api.Storage() == nil {
		t.Error("expected non-nil Storage API")
	}
	if api.Chain() == nil {
		t.Error("expected non-nil Chain API")
	}
	if api.Log() == nil {
		t.Error("expected non-nil Log API")
	}
}

func TestSysSecrets(t *testing.T) {
	ctx := context.Background()
	vault := newSimulationVault(nil)
	_ = vault.Initialize(ctx)

	// Store a secret first
	_ = vault.StoreSecret(ctx, "test-service", "test-account", "api_key", []byte("secret123"))

	api := NewSysAPI("test-service", "test-account", vault, nil)
	secrets := api.Secrets()

	// Test Get
	value, err := secrets.Get(ctx, "api_key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if value != "secret123" {
		t.Errorf("expected 'secret123', got '%s'", value)
	}

	// Test Set
	err = secrets.Set(ctx, "new_key", "new_value")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify Set worked
	value, err = secrets.Get(ctx, "new_key")
	if err != nil {
		t.Fatalf("Get after Set failed: %v", err)
	}
	if value != "new_value" {
		t.Errorf("expected 'new_value', got '%s'", value)
	}

	// Test List
	names, err := secrets.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(names) != 2 {
		t.Errorf("expected 2 secrets, got %d", len(names))
	}

	// Test GetMultiple
	values, err := secrets.GetMultiple(ctx, []string{"api_key", "new_key"})
	if err != nil {
		t.Fatalf("GetMultiple failed: %v", err)
	}
	if len(values) != 2 {
		t.Errorf("expected 2 values, got %d", len(values))
	}

	// Test Delete
	err = secrets.Delete(ctx, "new_key")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify Delete worked
	names, _ = secrets.List(ctx)
	if len(names) != 1 {
		t.Errorf("expected 1 secret after delete, got %d", len(names))
	}
}

func TestSysLog(t *testing.T) {
	api := NewSysAPI("test-service", "test-account", nil, nil)
	log := api.Log()

	log.Debug("debug message")
	log.Info("info message")
	log.Warn("warn message")
	log.Error("error message")

	// Get logs from the implementation
	impl := api.(*sysAPIImpl)
	logs := impl.GetLogs()

	if len(logs) != 4 {
		t.Errorf("expected 4 logs, got %d", len(logs))
	}

	expectedPrefixes := []string{"[DEBUG]", "[INFO]", "[WARN]", "[ERROR]"}
	for i, prefix := range expectedPrefixes {
		if len(logs) > i && logs[i][:len(prefix)] != prefix {
			t.Errorf("log %d: expected prefix '%s', got '%s'", i, prefix, logs[i])
		}
	}
}

func TestOCALLHandler_HTTP(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	config := DefaultOCALLHandlerConfig()
	config.AllowedHosts = []string{} // Allow all for testing
	config.BlockedHosts = []string{} // Don't block localhost for testing
	handler := NewOCALLHandler(config)

	ctx := context.Background()

	// Create HTTP request
	httpReq := HTTPRequest{
		Method: "GET",
		URL:    server.URL,
	}
	payload, _ := json.Marshal(httpReq)

	req := OCALLRequest{
		Type:      OCALLTypeHTTP,
		RequestID: "test-1",
		Payload:   payload,
	}

	resp, err := handler.HandleOCALL(ctx, req)
	if err != nil {
		t.Fatalf("HandleOCALL failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	var httpResp HTTPResponse
	if err := json.Unmarshal(resp.Payload, &httpResp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", httpResp.StatusCode)
	}
}

func TestOCALLHandler_BlockedHost(t *testing.T) {
	config := DefaultOCALLHandlerConfig()
	// localhost is blocked by default
	handler := NewOCALLHandler(config)

	ctx := context.Background()

	httpReq := HTTPRequest{
		Method: "GET",
		URL:    "http://localhost:8080/test",
	}
	payload, _ := json.Marshal(httpReq)

	req := OCALLRequest{
		Type:      OCALLTypeHTTP,
		RequestID: "test-blocked",
		Payload:   payload,
	}

	resp, err := handler.HandleOCALL(ctx, req)
	if err != nil {
		t.Fatalf("HandleOCALL failed: %v", err)
	}

	if resp.Success {
		t.Error("expected failure for blocked host")
	}

	if resp.Error == "" {
		t.Error("expected error message for blocked host")
	}
}

func TestOCALLHandler_ChainRPC(t *testing.T) {
	// Create a mock RPC server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  "0x1234",
		})
	}))
	defer server.Close()

	config := DefaultOCALLHandlerConfig()
	config.ChainRPCEndpoints = map[string]string{
		"ethereum": server.URL,
	}
	handler := NewOCALLHandler(config)

	ctx := context.Background()

	callReq := ChainCallRequest{
		Chain:  "ethereum",
		Method: "eth_blockNumber",
	}
	payload, _ := json.Marshal(callReq)

	req := OCALLRequest{
		Type:      OCALLTypeChainRPC,
		RequestID: "test-rpc",
		Payload:   payload,
	}

	resp, err := handler.HandleOCALL(ctx, req)
	if err != nil {
		t.Fatalf("HandleOCALL failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}
}

func TestOCALLHandler_UnknownChain(t *testing.T) {
	config := DefaultOCALLHandlerConfig()
	handler := NewOCALLHandler(config)

	ctx := context.Background()

	callReq := ChainCallRequest{
		Chain:  "unknown-chain",
		Method: "eth_blockNumber",
	}
	payload, _ := json.Marshal(callReq)

	req := OCALLRequest{
		Type:      OCALLTypeChainRPC,
		RequestID: "test-unknown",
		Payload:   payload,
	}

	resp, err := handler.HandleOCALL(ctx, req)
	if err != nil {
		t.Fatalf("HandleOCALL failed: %v", err)
	}

	if resp.Success {
		t.Error("expected failure for unknown chain")
	}
}

func TestEnhancedScriptEngine(t *testing.T) {
	engine := NewEnhancedScriptEngine(DefaultMemoryLimit)
	ctx := context.Background()

	if err := engine.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer engine.Shutdown(ctx)

	// Test basic execution
	result, err := engine.Execute(ctx, ScriptExecutionRequest{
		Script:     `function main(input) { return { sum: input.a + input.b }; }`,
		EntryPoint: "main",
		Input:      map[string]any{"a": 1, "b": 2},
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// goja may return int64 or float64 depending on the value
	var sum float64
	switch v := result.Output["sum"].(type) {
	case float64:
		sum = v
	case int64:
		sum = float64(v)
	case int:
		sum = float64(v)
	default:
		t.Fatalf("unexpected type for sum: %T", result.Output["sum"])
	}
	if sum != 3 {
		t.Errorf("expected sum=3, got %v", sum)
	}
}

func TestEnhancedScriptEngine_WithSysAPI(t *testing.T) {
	engine := NewEnhancedScriptEngine(DefaultMemoryLimit)
	ctx := context.Background()

	if err := engine.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer engine.Shutdown(ctx)

	// Create vault and store a secret
	vault := newSimulationVault(nil)
	_ = vault.Initialize(ctx)
	_ = vault.StoreSecret(ctx, "test-service", "test-account", "api_key", []byte("secret123"))

	// Create sys API
	sysAPI := NewSysAPI("test-service", "test-account", vault, nil)

	// Test execution with sys.secrets
	result, err := engine.ExecuteWithSysAPI(ctx, EnhancedScriptRequest{
		ScriptExecutionRequest: ScriptExecutionRequest{
			Script: `
				function main(input) {
					var secret = sys.secrets.get("api_key");
					sys.log.info("Got secret");
					return { hasSecret: secret !== undefined && secret !== null };
				}
			`,
			EntryPoint: "main",
			Input:      map[string]any{},
		},
		ServiceID: "test-service",
		AccountID: "test-account",
		SysAPI:    sysAPI,
	})

	if err != nil {
		t.Fatalf("ExecuteWithSysAPI failed: %v", err)
	}

	if result.Output["hasSecret"] != true {
		t.Errorf("expected hasSecret=true, got %v", result.Output["hasSecret"])
	}

	// Check logs
	if len(result.Logs) == 0 {
		t.Error("expected logs from sys.log.info")
	}
}

func TestEnhancedScriptEngine_SysHTTP(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "hello"})
	}))
	defer server.Close()

	engine := NewEnhancedScriptEngine(DefaultMemoryLimit)
	ctx := context.Background()

	if err := engine.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	defer engine.Shutdown(ctx)

	// Create OCALL handler
	config := DefaultOCALLHandlerConfig()
	config.BlockedHosts = []string{} // Allow localhost for testing
	handler := NewOCALLHandler(config)

	// Create sys API
	sysAPI := NewSysAPI("test-service", "test-account", nil, handler)

	// Test execution with sys.http
	result, err := engine.ExecuteWithSysAPI(ctx, EnhancedScriptRequest{
		ScriptExecutionRequest: ScriptExecutionRequest{
			Script: `
				function main(input) {
					var resp = sys.http.get(input.url);
					if (resp.error) {
						return { error: resp.error };
					}
					return { status: resp.status, body: resp.body };
				}
			`,
			EntryPoint: "main",
			Input:      map[string]any{"url": server.URL},
		},
		ServiceID: "test-service",
		AccountID: "test-account",
		SysAPI:    sysAPI,
	})

	if err != nil {
		t.Fatalf("ExecuteWithSysAPI failed: %v", err)
	}

	if result.Output["error"] != nil {
		t.Errorf("unexpected error: %v", result.Output["error"])
	}

	// goja may return int64 or float64 depending on the value
	var status float64
	switch v := result.Output["status"].(type) {
	case float64:
		status = v
	case int64:
		status = float64(v)
	case int:
		status = float64(v)
	default:
		t.Fatalf("unexpected type for status: %T", result.Output["status"])
	}
	if status != 200 {
		t.Errorf("expected status=200, got %v", status)
	}
}

func TestGenerateRequestID(t *testing.T) {
	id1 := generateRequestID()
	time.Sleep(time.Millisecond)
	id2 := generateRequestID()

	if id1 == id2 {
		t.Error("expected unique request IDs")
	}

	if id1 == "" || id2 == "" {
		t.Error("expected non-empty request IDs")
	}
}

// =============================================================================
// sys.neo Integration Tests - Neo N3 Transaction Signing via JavaScript
// =============================================================================

func TestSysNeoInJS_GetAddress(t *testing.T) {
	ctx := context.Background()
	engine := NewEnhancedScriptEngine(DefaultMemoryLimit)
	_ = engine.Initialize(ctx)
	defer engine.Shutdown(ctx)

	vault := newSimulationVault(nil)
	_ = vault.Initialize(ctx)
	handler := NewOCALLHandler(DefaultOCALLHandlerConfig())
	sysAPI := NewSysAPI("test-service", "test-account", vault, handler)

	script := `
function main(input) {
	var address = sys.neo.getAddress();
	var publicKey = sys.neo.getPublicKey();
	var scriptHash = sys.neo.getScriptHash();
	return {
		address: address,
		publicKey: publicKey,
		scriptHash: scriptHash
	};
}
`
	result, err := engine.ExecuteWithSysAPI(ctx, EnhancedScriptRequest{
		ScriptExecutionRequest: ScriptExecutionRequest{
			Script:     script,
			EntryPoint: "main",
			Input:      map[string]any{},
		},
		SysAPI: sysAPI,
	})

	if err != nil {
		t.Fatalf("ExecuteWithSysAPI failed: %v", err)
	}

	// Verify address starts with 'N' (Neo N3 format)
	address, ok := result.Output["address"].(string)
	if !ok || len(address) == 0 {
		t.Error("expected non-empty address")
	}
	if address[0] != 'N' {
		t.Errorf("expected address to start with 'N', got '%s'", address)
	}

	// Verify public key is hex string (66 chars for compressed)
	publicKey, ok := result.Output["publicKey"].(string)
	if !ok || len(publicKey) != 66 {
		t.Errorf("expected 66-char hex public key, got len=%d", len(publicKey))
	}

	// Verify script hash is 40 hex chars (20 bytes)
	scriptHash, ok := result.Output["scriptHash"].(string)
	if !ok || len(scriptHash) != 40 {
		t.Errorf("expected 40-char hex script hash, got len=%d", len(scriptHash))
	}
}

func TestSysNeoInJS_SignInvocation(t *testing.T) {
	ctx := context.Background()
	engine := NewEnhancedScriptEngine(DefaultMemoryLimit)
	_ = engine.Initialize(ctx)
	defer engine.Shutdown(ctx)

	vault := newSimulationVault(nil)
	_ = vault.Initialize(ctx)
	handler := NewOCALLHandler(DefaultOCALLHandlerConfig())
	sysAPI := NewSysAPI("test-service", "test-account", vault, handler)

	script := `
function main(input) {
	// Sign a contract invocation (e.g., NEP-17 transfer)
	var result = sys.neo.signInvocation({
		scriptHash: "d2a4cff31913016155e38e474a2c06d08be276cf",
		method: "symbol",
		args: []
	});
	return result;
}
`
	result, err := engine.ExecuteWithSysAPI(ctx, EnhancedScriptRequest{
		ScriptExecutionRequest: ScriptExecutionRequest{
			Script:     script,
			EntryPoint: "main",
			Input:      map[string]any{},
		},
		SysAPI: sysAPI,
	})

	if err != nil {
		t.Fatalf("ExecuteWithSysAPI failed: %v", err)
	}

	// Check for error in result
	if errMsg, ok := result.Output["error"].(string); ok && errMsg != "" {
		t.Fatalf("signInvocation returned error: %s", errMsg)
	}

	// Verify hash is present
	hash, ok := result.Output["hash"].(string)
	if !ok || len(hash) == 0 {
		t.Error("expected non-empty transaction hash")
	}

	// Verify rawTransaction is present
	rawTx, ok := result.Output["rawTransaction"].(string)
	if !ok || len(rawTx) == 0 {
		t.Error("expected non-empty raw transaction")
	}

	// Verify witnesses are present (use JSON round-trip to normalize types)
	witnessesVal := result.Output["witnesses"]
	jsonBytes, _ := json.Marshal(witnessesVal)
	var witnesses []map[string]any
	if err := json.Unmarshal(jsonBytes, &witnesses); err != nil {
		t.Fatalf("failed to parse witnesses: %v (type=%T)", err, witnessesVal)
	}
	if len(witnesses) == 0 {
		t.Error("expected at least one witness")
	}
}

func TestSysCryptoInJS_Hash(t *testing.T) {
	ctx := context.Background()
	engine := NewEnhancedScriptEngine(DefaultMemoryLimit)
	_ = engine.Initialize(ctx)
	defer engine.Shutdown(ctx)

	vault := newSimulationVault(nil)
	_ = vault.Initialize(ctx)
	handler := NewOCALLHandler(DefaultOCALLHandlerConfig())
	sysAPI := NewSysAPI("test-service", "test-account", vault, handler)

	script := `
function main(input) {
	var sha256Result = sys.crypto.hash("sha256", "hello world");
	var sha3Result = sys.crypto.hash("sha3-256", "hello world");
	return {
		sha256: sha256Result,
		sha3: sha3Result
	};
}
`
	result, err := engine.ExecuteWithSysAPI(ctx, EnhancedScriptRequest{
		ScriptExecutionRequest: ScriptExecutionRequest{
			Script:     script,
			EntryPoint: "main",
			Input:      map[string]any{},
		},
		SysAPI: sysAPI,
	})

	if err != nil {
		t.Fatalf("ExecuteWithSysAPI failed: %v", err)
	}

	// Verify SHA256 hash
	sha256Result, ok := result.Output["sha256"].(map[string]any)
	if !ok {
		t.Fatal("expected sha256 result to be a map")
	}
	sha256Hash, ok := sha256Result["hash"].(string)
	if !ok || len(sha256Hash) != 64 {
		t.Errorf("expected 64-char SHA256 hash, got len=%d", len(sha256Hash))
	}

	// Verify SHA3-256 hash
	sha3Result, ok := result.Output["sha3"].(map[string]any)
	if !ok {
		t.Fatal("expected sha3 result to be a map")
	}
	sha3Hash, ok := sha3Result["hash"].(string)
	if !ok || len(sha3Hash) != 64 {
		t.Errorf("expected 64-char SHA3-256 hash, got len=%d", len(sha3Hash))
	}
}

func TestSysProofInJS_Generate(t *testing.T) {
	ctx := context.Background()
	engine := NewEnhancedScriptEngine(DefaultMemoryLimit)
	_ = engine.Initialize(ctx)
	defer engine.Shutdown(ctx)

	vault := newSimulationVault(nil)
	_ = vault.Initialize(ctx)
	handler := NewOCALLHandler(DefaultOCALLHandlerConfig())
	sysAPI := NewSysAPI("test-service", "test-account", vault, handler)

	script := `
function main(input) {
	var proof = sys.proof.generate("test data for proof");
	return proof;
}
`
	result, err := engine.ExecuteWithSysAPI(ctx, EnhancedScriptRequest{
		ScriptExecutionRequest: ScriptExecutionRequest{
			Script:     script,
			EntryPoint: "main",
			Input:      map[string]any{},
		},
		SysAPI: sysAPI,
	})

	if err != nil {
		t.Fatalf("ExecuteWithSysAPI failed: %v", err)
	}

	// Check for error
	if errMsg, ok := result.Output["error"].(string); ok && errMsg != "" {
		t.Fatalf("proof.generate returned error: %s", errMsg)
	}

	// Verify proof fields
	proofID, ok := result.Output["proofID"].(string)
	if !ok || len(proofID) == 0 {
		t.Error("expected non-empty proofID")
	}

	enclaveID, ok := result.Output["enclaveID"].(string)
	if !ok || len(enclaveID) == 0 {
		t.Error("expected non-empty enclaveID")
	}

	inputHash, ok := result.Output["inputHash"].(string)
	if !ok || len(inputHash) != 64 {
		t.Errorf("expected 64-char inputHash, got len=%d", len(inputHash))
	}

	signature, ok := result.Output["signature"].(string)
	if !ok || len(signature) != 128 {
		t.Errorf("expected 128-char signature (64 bytes hex), got len=%d", len(signature))
	}
}
