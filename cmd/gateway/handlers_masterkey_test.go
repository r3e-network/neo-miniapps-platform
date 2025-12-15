package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMasterKeyHandler_ProxiesToNeoAccountsMasterKey(t *testing.T) {
	var gotPath string

	prevTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		return jsonResponse(r, http.StatusOK, "{}"), nil
	})
	t.Cleanup(func() { http.DefaultTransport = prevTransport })

	prev := serviceEndpoints["neoaccounts"]
	serviceEndpoints["neoaccounts"] = "http://neoaccounts.example"
	t.Cleanup(func() { serviceEndpoints["neoaccounts"] = prev })

	req := httptest.NewRequest(http.MethodGet, "/master-key", nil)
	rr := httptest.NewRecorder()
	masterKeyHandler(nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if gotPath != "/master-key" {
		t.Fatalf("upstream path = %q, want %q", gotPath, "/master-key")
	}
}
