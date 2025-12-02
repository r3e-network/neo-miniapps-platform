package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	engine "github.com/R3E-Network/service_layer/system/os/core"
)

type stubRPCEngine struct {
	endpoints map[string]string
}

func (s stubRPCEngine) Name() string                    { return "rpc" }
func (s stubRPCEngine) Domain() string                  { return "rpc" }
func (stubRPCEngine) Start(context.Context) error       { return nil }
func (stubRPCEngine) Stop(context.Context) error        { return nil }
func (stubRPCEngine) RPCInfo() string                   { return "stub" }
func (s stubRPCEngine) RPCEndpoints() map[string]string { return s.endpoints }

func TestHandleChainRPC_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{"ok":true},"id":1}`))
	}))
	defer ts.Close()

	h := &handler{
		rpcEngines: func() []engine.RPCEngine {
			return []engine.RPCEngine{stubRPCEngine{endpoints: map[string]string{"neo": ts.URL}}}
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/system/rpc", strings.NewReader(`{"chain":"neo","method":"getblockcount","params":[]}`))
	rec := httptest.NewRecorder()

	h.handleChainRPC(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestHandleChainRPC_NoEngine(t *testing.T) {
	h := &handler{}
	req := httptest.NewRequest(http.MethodPost, "/system/rpc", strings.NewReader(`{"chain":"eth","method":"eth_blockNumber"}`))
	rec := httptest.NewRecorder()

	h.handleChainRPC(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", rec.Code)
	}
}

func TestHandleChainRPC_MissingChain(t *testing.T) {
	h := &handler{
		rpcEngines: func() []engine.RPCEngine {
			return []engine.RPCEngine{stubRPCEngine{endpoints: map[string]string{"neo": "http://example"}}}
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/system/rpc", strings.NewReader(`{"chain":"eth","method":"eth_blockNumber"}`))
	rec := httptest.NewRecorder()

	h.handleChainRPC(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "available") {
		t.Fatalf("expected available chains in error, got %s", rec.Body.String())
	}
}

func TestHandleChainRPC_TenantRequired(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{"ok":true},"id":1}`))
	}))
	defer ts.Close()

	h := &handler{
		rpcEngines: func() []engine.RPCEngine {
			return []engine.RPCEngine{stubRPCEngine{endpoints: map[string]string{"neo": ts.URL}}}
		},
		rpcPolicy: newRPCPolicy(RPCPolicy{RequireTenant: true}),
	}
	req := httptest.NewRequest(http.MethodPost, "/system/rpc", strings.NewReader(`{"chain":"neo","method":"getblockcount","params":[]}`))
	rec := httptest.NewRecorder()

	h.handleChainRPC(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestHandleChainRPC_RateLimitedTenant(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{"ok":true},"id":1}`))
	}))
	defer ts.Close()

	h := &handler{
		rpcEngines: func() []engine.RPCEngine {
			return []engine.RPCEngine{stubRPCEngine{endpoints: map[string]string{"neo": ts.URL}}}
		},
		rpcPolicy: newRPCPolicy(RPCPolicy{PerTenantPerMinute: 1, Burst: 0}),
	}
	makeReq := func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/system/rpc", strings.NewReader(`{"chain":"neo","method":"getblockcount","params":[]}`))
		req.Header.Set("X-Tenant-ID", "t1")
		return req
	}

	first := httptest.NewRecorder()
	h.handleChainRPC(first, makeReq())
	if first.Code != http.StatusOK {
		t.Fatalf("expected first call ok, got %d", first.Code)
	}

	second := httptest.NewRecorder()
	h.handleChainRPC(second, makeReq())
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on second call, got %d", second.Code)
	}
	if retry := second.Header().Get("Retry-After"); retry == "" {
		t.Fatalf("expected Retry-After header")
	}
}

func TestHandleChainRPC_MethodNotAllowed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","result":{"ok":true},"id":1}`))
	}))
	defer ts.Close()

	h := &handler{
		rpcEngines: func() []engine.RPCEngine {
			return []engine.RPCEngine{stubRPCEngine{endpoints: map[string]string{"eth": ts.URL}}}
		},
		rpcPolicy: newRPCPolicy(RPCPolicy{
			AllowedMethods: map[string][]string{"eth": {"eth_blockNumber"}},
		}),
	}
	req := httptest.NewRequest(http.MethodPost, "/system/rpc", strings.NewReader(`{"chain":"eth","method":"eth_sendRawTransaction","params":[]}`))
	rec := httptest.NewRecorder()

	h.handleChainRPC(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}
