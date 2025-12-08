package chain

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "valid config",
			cfg:     Config{RPCURL: "http://localhost:10332"},
			wantErr: false,
		},
		{
			name:    "missing URL",
			cfg:     Config{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClientCall(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req RPCRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := RPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
		}

		switch req.Method {
		case "getblockcount":
			resp.Result = json.RawMessage(`12345`)
		case "invokefunction":
			resp.Result = json.RawMessage(`{"state":"HALT","gasconsumed":"0.1","stack":[{"type":"Integer","value":"100"}]}`)
		default:
			resp.Error = &RPCError{Code: -1, Message: "unknown method"}
		}

		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewClient(Config{RPCURL: server.URL})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx := context.Background()

	// Test getblockcount
	result, err := client.Call(ctx, "getblockcount", nil)
	if err != nil {
		t.Errorf("Call(getblockcount) error = %v", err)
	}

	var count int
	json.Unmarshal(result, &count)
	if count != 12345 {
		t.Errorf("Expected block count 12345, got %d", count)
	}
}

func TestGetBlockCount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := RPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result:  json.RawMessage(`12345`),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, _ := NewClient(Config{RPCURL: server.URL})
	ctx := context.Background()

	count, err := client.GetBlockCount(ctx)
	if err != nil {
		t.Errorf("GetBlockCount() error = %v", err)
	}
	if count != 12345 {
		t.Errorf("Expected 12345, got %d", count)
	}
}

func TestInvokeFunction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := RPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result: json.RawMessage(`{
				"script": "test",
				"state": "HALT",
				"gasconsumed": "0.1234",
				"stack": [{"type": "Integer", "value": "42"}]
			}`),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, _ := NewClient(Config{RPCURL: server.URL})
	ctx := context.Background()

	result, err := client.InvokeFunction(ctx, "0x1234", "test", nil)
	if err != nil {
		t.Errorf("InvokeFunction() error = %v", err)
	}
	if result.State != "HALT" {
		t.Errorf("Expected HALT state, got %s", result.State)
	}
	if len(result.Stack) != 1 {
		t.Errorf("Expected 1 stack item, got %d", len(result.Stack))
	}
}

func TestContractParams(t *testing.T) {
	// Test string param
	strParam := NewStringParam("hello")
	if strParam.Type != "String" || strParam.Value != "hello" {
		t.Errorf("NewStringParam failed")
	}

	// Test integer param
	intParam := NewIntegerParam(big.NewInt(42))
	if intParam.Type != "Integer" || intParam.Value != "42" {
		t.Errorf("NewIntegerParam failed")
	}

	// Test bool param
	boolParam := NewBoolParam(true)
	if boolParam.Type != "Boolean" || boolParam.Value != true {
		t.Errorf("NewBoolParam failed")
	}

	// Test byte array param
	byteParam := NewByteArrayParam([]byte{0x01, 0x02, 0x03})
	if byteParam.Type != "ByteArray" || byteParam.Value != "010203" {
		t.Errorf("NewByteArrayParam failed")
	}

	// Test hash160 param
	hashParam := NewHash160Param("0x1234567890abcdef1234567890abcdef12345678")
	if hashParam.Type != "Hash160" {
		t.Errorf("NewHash160Param failed")
	}
}

func TestRPCError(t *testing.T) {
	err := &RPCError{
		Code:    -100,
		Message: "test error",
	}

	expected := "RPC error -100: test error"
	if err.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, err.Error())
	}
}

func TestParseInteger(t *testing.T) {
	item := StackItem{
		Type:  "Integer",
		Value: json.RawMessage(`"12345"`),
	}

	result, err := parseInteger(item)
	if err != nil {
		t.Errorf("parseInteger() error = %v", err)
	}
	if result.Cmp(big.NewInt(12345)) != 0 {
		t.Errorf("Expected 12345, got %s", result.String())
	}
}

func TestParseBoolean(t *testing.T) {
	item := StackItem{
		Type:  "Boolean",
		Value: json.RawMessage(`true`),
	}

	result, err := parseBoolean(item)
	if err != nil {
		t.Errorf("parseBoolean() error = %v", err)
	}
	if !result {
		t.Errorf("Expected true, got false")
	}
}

func TestParseByteArray(t *testing.T) {
	item := StackItem{
		Type:  "ByteString",
		Value: json.RawMessage(`"48656c6c6f"`), // "Hello" in hex
	}

	result, err := parseByteArray(item)
	if err != nil {
		t.Errorf("parseByteArray() error = %v", err)
	}
	if string(result) != "Hello" {
		t.Errorf("Expected 'Hello', got %q", string(result))
	}
}

func TestParseByteArrayNull(t *testing.T) {
	item := StackItem{
		Type:  "Null",
		Value: nil,
	}

	result, err := parseByteArray(item)
	if err != nil {
		t.Errorf("parseByteArray() error = %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil, got %v", result)
	}
}
