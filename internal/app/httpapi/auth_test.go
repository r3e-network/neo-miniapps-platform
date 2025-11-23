package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/R3E-Network/service_layer/pkg/logger"
)

func TestWrapWithAuthRejectsWhenNoTokensConfigured(t *testing.T) {
	var called bool
	wrapped := wrapWithAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}), nil, logger.NewDefault("test"), nil)

	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when tokens are missing, got %d", rec.Code)
	}
	if called {
		t.Fatalf("expected handler not to be invoked when unauthorised")
	}

	// Public endpoints should remain accessible.
	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec = httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected healthz to be served, got %d", rec.Code)
	}
}
