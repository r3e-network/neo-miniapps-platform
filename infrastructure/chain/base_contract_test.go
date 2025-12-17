package chain

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"testing"
)

func mockTransport(handler func(*http.Request) (*http.Response, error)) roundTripperFunc {
	return roundTripperFunc(handler)
}

func mockInvokeResponse(state string, stack []StackItem, exception string) roundTripperFunc {
	return mockTransport(func(r *http.Request) (*http.Response, error) {
		result := InvokeResult{
			State:     state,
			Stack:     stack,
			Exception: exception,
		}
		raw, _ := json.Marshal(result)
		resp := RPCResponse{JSONRPC: "2.0", ID: 1, Result: raw}
		payload, _ := json.Marshal(resp)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(payload)),
		}, nil
	})
}

func TestNewBaseContract(t *testing.T) {
	client, _ := NewClient(Config{RPCURL: "http://example"})
	wallet := &Wallet{address: "NTest"}

	bc := NewBaseContract(client, "0x1234", wallet)

	if bc.Client() != client {
		t.Error("Client() should return the configured client")
	}
	if bc.ContractHash() != "0x1234" {
		t.Errorf("ContractHash() = %s, want 0x1234", bc.ContractHash())
	}
	if bc.Wallet() != wallet {
		t.Error("Wallet() should return the configured wallet")
	}
}

func TestBaseContractInvokeRaw(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client, _ := NewClient(Config{RPCURL: "http://example"})
		client.httpClient.Transport = mockInvokeResponse("HALT", []StackItem{
			{Type: "Integer", Value: json.RawMessage(`"42"`)},
		}, "")

		bc := NewBaseContract(client, "0x1234", nil)
		ctx := context.Background()

		result, err := bc.InvokeRaw(ctx, "testMethod")
		if err != nil {
			t.Fatalf("InvokeRaw() error = %v", err)
		}
		if result.State != "HALT" {
			t.Errorf("State = %s, want HALT", result.State)
		}
	})

	t.Run("execution failed", func(t *testing.T) {
		client, _ := NewClient(Config{RPCURL: "http://example"})
		client.httpClient.Transport = mockInvokeResponse("FAULT", nil, "test exception")

		bc := NewBaseContract(client, "0x1234", nil)
		ctx := context.Background()

		_, err := bc.InvokeRaw(ctx, "testMethod")
		if err == nil {
			t.Error("expected error for FAULT state")
		}
	})
}

func TestBaseContractInvokeInteger(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client, _ := NewClient(Config{RPCURL: "http://example"})
		client.httpClient.Transport = mockInvokeResponse("HALT", []StackItem{
			{Type: "Integer", Value: json.RawMessage(`"12345"`)},
		}, "")

		bc := NewBaseContract(client, "0x1234", nil)
		ctx := context.Background()

		result, err := bc.InvokeInteger(ctx, "getValue")
		if err != nil {
			t.Fatalf("InvokeInteger() error = %v", err)
		}
		if result.Cmp(big.NewInt(12345)) != 0 {
			t.Errorf("result = %s, want 12345", result.String())
		}
	})

	t.Run("empty stack", func(t *testing.T) {
		client, _ := NewClient(Config{RPCURL: "http://example"})
		client.httpClient.Transport = mockInvokeResponse("HALT", []StackItem{}, "")

		bc := NewBaseContract(client, "0x1234", nil)
		ctx := context.Background()

		_, err := bc.InvokeInteger(ctx, "getValue")
		if err == nil {
			t.Error("expected error for empty stack")
		}
	})
}

func TestBaseContractInvokeBoolean(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		client, _ := NewClient(Config{RPCURL: "http://example"})
		client.httpClient.Transport = mockInvokeResponse("HALT", []StackItem{
			{Type: "Boolean", Value: json.RawMessage(`true`)},
		}, "")

		bc := NewBaseContract(client, "0x1234", nil)
		ctx := context.Background()

		result, err := bc.InvokeBoolean(ctx, "isEnabled")
		if err != nil {
			t.Fatalf("InvokeBoolean() error = %v", err)
		}
		if !result {
			t.Error("expected true")
		}
	})

	t.Run("empty stack", func(t *testing.T) {
		client, _ := NewClient(Config{RPCURL: "http://example"})
		client.httpClient.Transport = mockInvokeResponse("HALT", []StackItem{}, "")

		bc := NewBaseContract(client, "0x1234", nil)
		ctx := context.Background()

		_, err := bc.InvokeBoolean(ctx, "isEnabled")
		if err == nil {
			t.Error("expected error for empty stack")
		}
	})
}

func TestBaseContractInvokeString(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client, _ := NewClient(Config{RPCURL: "http://example"})
		client.httpClient.Transport = mockInvokeResponse("HALT", []StackItem{
			{Type: "ByteString", Value: json.RawMessage(`"48656c6c6f"`)}, // "Hello"
		}, "")

		bc := NewBaseContract(client, "0x1234", nil)
		ctx := context.Background()

		result, err := bc.InvokeString(ctx, "getName")
		if err != nil {
			t.Fatalf("InvokeString() error = %v", err)
		}
		if result != "Hello" {
			t.Errorf("result = %s, want Hello", result)
		}
	})

	t.Run("empty stack", func(t *testing.T) {
		client, _ := NewClient(Config{RPCURL: "http://example"})
		client.httpClient.Transport = mockInvokeResponse("HALT", []StackItem{}, "")

		bc := NewBaseContract(client, "0x1234", nil)
		ctx := context.Background()

		_, err := bc.InvokeString(ctx, "getName")
		if err == nil {
			t.Error("expected error for empty stack")
		}
	})
}

func TestBaseContractInvokeByteArray(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client, _ := NewClient(Config{RPCURL: "http://example"})
		// "SGVsbG8=" is base64 for "Hello"
		client.httpClient.Transport = mockInvokeResponse("HALT", []StackItem{
			{Type: "ByteString", Value: json.RawMessage(`"SGVsbG8="`)},
		}, "")

		bc := NewBaseContract(client, "0x1234", nil)
		ctx := context.Background()

		result, err := bc.InvokeByteArray(ctx, "getData")
		if err != nil {
			t.Fatalf("InvokeByteArray() error = %v", err)
		}
		expected := []byte("Hello")
		if !bytes.Equal(result, expected) {
			t.Errorf("result = %s, want %s", string(result), string(expected))
		}
	})

	t.Run("empty stack", func(t *testing.T) {
		client, _ := NewClient(Config{RPCURL: "http://example"})
		client.httpClient.Transport = mockInvokeResponse("HALT", []StackItem{}, "")

		bc := NewBaseContract(client, "0x1234", nil)
		ctx := context.Background()

		_, err := bc.InvokeByteArray(ctx, "getData")
		if err == nil {
			t.Error("expected error for empty stack")
		}
	})
}

func TestBaseContractInvokeUint64(t *testing.T) {
	client, _ := NewClient(Config{RPCURL: "http://example"})
	client.httpClient.Transport = mockInvokeResponse("HALT", []StackItem{
		{Type: "Integer", Value: json.RawMessage(`"999"`)},
	}, "")

	bc := NewBaseContract(client, "0x1234", nil)
	ctx := context.Background()

	result, err := bc.InvokeUint64(ctx, "getCount")
	if err != nil {
		t.Fatalf("InvokeUint64() error = %v", err)
	}
	if result != 999 {
		t.Errorf("result = %d, want 999", result)
	}
}

func TestBaseContractInvokeVoid(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client, _ := NewClient(Config{RPCURL: "http://example"})
		client.httpClient.Transport = mockInvokeResponse("HALT", nil, "")

		bc := NewBaseContract(client, "0x1234", nil)
		ctx := context.Background()

		err := bc.InvokeVoid(ctx, "doSomething")
		if err != nil {
			t.Errorf("InvokeVoid() error = %v", err)
		}
	})

	t.Run("failure", func(t *testing.T) {
		client, _ := NewClient(Config{RPCURL: "http://example"})
		client.httpClient.Transport = mockInvokeResponse("FAULT", nil, "error")

		bc := NewBaseContract(client, "0x1234", nil)
		ctx := context.Background()

		err := bc.InvokeVoid(ctx, "doSomething")
		if err == nil {
			t.Error("expected error for FAULT state")
		}
	})
}

func TestInvokeAndParse(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client, _ := NewClient(Config{RPCURL: "http://example"})
		client.httpClient.Transport = mockInvokeResponse("HALT", []StackItem{
			{Type: "Integer", Value: json.RawMessage(`"100"`)},
		}, "")

		bc := NewBaseContract(client, "0x1234", nil)
		ctx := context.Background()

		result, err := InvokeAndParse(bc, ctx, "getConfig", func(item StackItem) (int, error) {
			val, err := ParseInteger(item)
			if err != nil {
				return 0, err
			}
			return int(val.Int64()), nil
		})
		if err != nil {
			t.Fatalf("InvokeAndParse() error = %v", err)
		}
		if result != 100 {
			t.Errorf("result = %d, want 100", result)
		}
	})

	t.Run("empty stack", func(t *testing.T) {
		client, _ := NewClient(Config{RPCURL: "http://example"})
		client.httpClient.Transport = mockInvokeResponse("HALT", []StackItem{}, "")

		bc := NewBaseContract(client, "0x1234", nil)
		ctx := context.Background()

		_, err := InvokeAndParse(bc, ctx, "getConfig", func(item StackItem) (int, error) {
			return 0, nil
		})
		if err == nil {
			t.Error("expected error for empty stack")
		}
	})
}

func TestInvokeArray(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		client, _ := NewClient(Config{RPCURL: "http://example"})
		arrayItems := []StackItem{
			{Type: "Integer", Value: json.RawMessage(`"1"`)},
			{Type: "Integer", Value: json.RawMessage(`"2"`)},
			{Type: "Integer", Value: json.RawMessage(`"3"`)},
		}
		arrayJSON, _ := json.Marshal(arrayItems)
		client.httpClient.Transport = mockInvokeResponse("HALT", []StackItem{
			{Type: "Array", Value: arrayJSON},
		}, "")

		bc := NewBaseContract(client, "0x1234", nil)
		ctx := context.Background()

		result, err := InvokeArray(bc, ctx, "getList", func(item StackItem) (int64, error) {
			val, err := ParseInteger(item)
			if err != nil {
				return 0, err
			}
			return val.Int64(), nil
		})
		if err != nil {
			t.Fatalf("InvokeArray() error = %v", err)
		}
		if len(result) != 3 {
			t.Errorf("len(result) = %d, want 3", len(result))
		}
		if result[0] != 1 || result[1] != 2 || result[2] != 3 {
			t.Errorf("result = %v, want [1 2 3]", result)
		}
	})

	t.Run("empty stack", func(t *testing.T) {
		client, _ := NewClient(Config{RPCURL: "http://example"})
		client.httpClient.Transport = mockInvokeResponse("HALT", []StackItem{}, "")

		bc := NewBaseContract(client, "0x1234", nil)
		ctx := context.Background()

		_, err := InvokeArray(bc, ctx, "getList", func(item StackItem) (int64, error) {
			return 0, nil
		})
		if err == nil {
			t.Error("expected error for empty stack")
		}
	})

	t.Run("parse error", func(t *testing.T) {
		client, _ := NewClient(Config{RPCURL: "http://example"})
		arrayItems := []StackItem{
			{Type: "Integer", Value: json.RawMessage(`"1"`)},
			{Type: "Boolean", Value: json.RawMessage(`true`)}, // wrong type
		}
		arrayJSON, _ := json.Marshal(arrayItems)
		client.httpClient.Transport = mockInvokeResponse("HALT", []StackItem{
			{Type: "Array", Value: arrayJSON},
		}, "")

		bc := NewBaseContract(client, "0x1234", nil)
		ctx := context.Background()

		_, err := InvokeArray(bc, ctx, "getList", func(item StackItem) (int64, error) {
			val, err := ParseInteger(item)
			if err != nil {
				return 0, err
			}
			return val.Int64(), nil
		})
		if err == nil {
			t.Error("expected error for parse failure")
		}
	})
}
