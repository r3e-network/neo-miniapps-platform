package oracle

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/R3E-Network/service_layer/internal/app/domain/account"
	domain "github.com/R3E-Network/service_layer/internal/app/domain/oracle"
	"github.com/R3E-Network/service_layer/internal/app/storage/memory"
)

func TestHTTPResolver_GetRequest(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)

	var receivedQuery string
	server := newOracleHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"value":42}`))
	}))
	defer server.Close()

	source, err := svc.CreateSource(context.Background(), acct.ID, "prices", server.URL, "GET", "", nil, "")
	if err != nil {
		t.Fatalf("create source: %v", err)
	}

	resolver := NewHTTPResolver(svc, server.Client(), nil)
	req := domain.Request{ID: "req-1", AccountID: acct.ID, DataSourceID: source.ID, Payload: `{"asset":"NEO"}`}

	done, success, result, errMsg, retry, err := resolver.Resolve(context.Background(), req)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !done || !success || retry != 0 {
		t.Fatalf("unexpected state: done=%v success=%v retry=%v", done, success, retry)
	}
	if strings.TrimSpace(result) != `{"value":42}` || errMsg != "" {
		t.Fatalf("unexpected payload result=%q error=%q", result, errMsg)
	}
	if !strings.Contains(receivedQuery, "asset=NEO") {
		t.Fatalf("expected query to include asset, got %q", receivedQuery)
	}
}

func TestHTTPResolver_PostRequest(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)

	var body string
	server := newOracleHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		body = string(data)
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected content type application/json, got %s", ct)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	source, err := svc.CreateSource(context.Background(), acct.ID, "prices-post", server.URL, "POST", "", nil, `{"default":true}`)
	if err != nil {
		t.Fatalf("create source: %v", err)
	}

	resolver := NewHTTPResolver(svc, server.Client(), nil)
	req := domain.Request{ID: "req-2", AccountID: acct.ID, DataSourceID: source.ID, Payload: `{"override":true}`}

	done, success, result, errMsg, retry, err := resolver.Resolve(context.Background(), req)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !done || !success {
		t.Fatalf("expected success")
	}
	if result != "OK" || errMsg != "" || retry != 0 {
		t.Fatalf("unexpected response result=%q err=%q retry=%v", result, errMsg, retry)
	}
	if body != `{"override":true}` {
		t.Fatalf("expected payload override, got %q", body)
	}
}

func TestHTTPResolver_HandlesErrorStatus(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)

	server := newOracleHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing parameter", http.StatusBadRequest)
	}))
	defer server.Close()

	source, _ := svc.CreateSource(context.Background(), acct.ID, "error-source", server.URL, "GET", "", nil, "")

	resolver := NewHTTPResolver(svc, server.Client(), nil)
	req := domain.Request{ID: "req-3", AccountID: acct.ID, DataSourceID: source.ID}

	done, success, result, errMsg, retry, err := resolver.Resolve(context.Background(), req)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !done || success || retry != 0 {
		t.Fatalf("expected final failure, got done=%v success=%v retry=%v", done, success, retry)
	}
	if result != "" || !strings.Contains(errMsg, "missing parameter") {
		t.Fatalf("unexpected error message: %q", errMsg)
	}
}

func TestHTTPResolver_RetryableStatus(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)

	server := newOracleHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	source, _ := svc.CreateSource(context.Background(), acct.ID, "retry-source", server.URL, "GET", "", nil, "")

	resolver := NewHTTPResolver(svc, server.Client(), nil)
	req := domain.Request{ID: "req-4", AccountID: acct.ID, DataSourceID: source.ID}

	done, success, result, errMsg, retry, err := resolver.Resolve(context.Background(), req)
	if err == nil {
		t.Fatalf("expected error for retryable status")
	}
	if done || success || result != "" || errMsg != "" {
		t.Fatalf("unexpected state for retry: done=%v success=%v result=%q errMsg=%q", done, success, result, errMsg)
	}
	if retry <= 0 || retry > defaultHTTPResolverRetry {
		t.Fatalf("unexpected retry duration: %v", retry)
	}
}

func TestHTTPResolver_MultiSourceAggregate(t *testing.T) {
	store := memory.New()
	acct, _ := store.CreateAccount(context.Background(), account.Account{Owner: "owner"})
	svc := New(store, store, nil)

	server1 := newOracleHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("10"))
	}))
	defer server1.Close()
	server2 := newOracleHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("30"))
	}))
	defer server2.Close()

	src1, _ := svc.CreateSource(context.Background(), acct.ID, "oracle-1", server1.URL, "GET", "", nil, "")
	src2, _ := svc.CreateSource(context.Background(), acct.ID, "oracle-2", server2.URL, "GET", "", nil, "")

	resolver := NewHTTPResolver(svc, server1.Client(), nil)
	req := domain.Request{
		ID:           "req-agg",
		AccountID:    acct.ID,
		DataSourceID: src1.ID,
		Payload:      fmt.Sprintf(`{"alternate_source_ids":["%s"]}`, src2.ID),
	}

	done, success, result, errMsg, retry, err := resolver.Resolve(context.Background(), req)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !done || !success || retry != 0 || errMsg != "" {
		t.Fatalf("unexpected state: done=%v success=%v retry=%v errMsg=%q", done, success, retry, errMsg)
	}
	if strings.TrimSpace(result) != "20" && strings.TrimSpace(result) != "20.0" {
		t.Fatalf("expected median aggregated result, got %q", result)
	}
}

func newOracleHTTPServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("tcp4 listener unavailable: %v", err)
	}
	server := &httptest.Server{
		Listener: l,
		Config:   &http.Server{Handler: handler},
	}
	server.Start()
	return server
}
