package gasbank

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPWithdrawalResolver(t *testing.T) {
	calls := 0
	server := newGasbankHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("transaction_id") != "tx" {
			t.Fatalf("unexpected query %s", r.URL.RawQuery)
		}
		switch calls {
		case 1:
			if _, err := w.Write([]byte(`{"done": false, "retry_after_seconds": 0.1}`)); err != nil {
				t.Fatalf("write response: %v", err)
			}
		case 2:
			if _, err := w.Write([]byte(`{"done": true, "success": false, "message": "failed"}`)); err != nil {
				t.Fatalf("write response: %v", err)
			}
		default:
			t.Fatalf("too many calls")
		}
	}))
	defer server.Close()

	resolver, err := NewHTTPWithdrawalResolver(server.Client(), server.URL, "", nil)
	if err != nil {
		t.Fatalf("new resolver: %v", err)
	}

	tx := Transaction{ID: "tx"}

	done, success, msg, retry, err := resolver.Resolve(context.Background(), tx)
	if err != nil || done || success || msg != "" || retry <= 0 {
		t.Fatalf("unexpected first resolve: done=%v success=%v msg=%q err=%v retry=%v", done, success, msg, err, retry)
	}

	time.Sleep(50 * time.Millisecond)

	done, success, msg, _, err = resolver.Resolve(context.Background(), tx)
	if err != nil || !done || success || msg != "failed" {
		t.Fatalf("unexpected second resolve: done=%v success=%v msg=%q err=%v", done, success, msg, err)
	}
}

func newGasbankHTTPServer(t *testing.T, handler http.Handler) *httptest.Server {
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
