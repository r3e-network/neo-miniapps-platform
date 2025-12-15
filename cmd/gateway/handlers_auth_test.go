package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/R3E-Network/service_layer/internal/database"
)

func TestLogoutHandlerCookieAuthClearsCookieAndDeletesSession(t *testing.T) {
	token := "test-token"
	expectedHash := hashToken(token)

	var gotMethod, gotPath, gotTokenHash string

	prevTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotTokenHash = r.URL.Query().Get("token_hash")
		return jsonResponse(r, http.StatusOK, "[]"), nil
	})
	t.Cleanup(func() { http.DefaultTransport = prevTransport })

	client, err := database.NewClient(database.Config{
		URL:        "https://example.com",
		ServiceKey: "service-key",
	})
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	db := database.NewRepository(client)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: oauthTokenCookieName, Value: token})
	res := httptest.NewRecorder()

	logoutHandler(db).ServeHTTP(res, req)

	if gotMethod != http.MethodDelete {
		t.Fatalf("DeleteSession method = %s, want %s", gotMethod, http.MethodDelete)
	}
	if gotPath != "/rest/v1/user_sessions" {
		t.Fatalf("DeleteSession path = %s, want /rest/v1/user_sessions", gotPath)
	}
	if gotTokenHash != "eq."+expectedHash {
		t.Fatalf("DeleteSession token_hash = %q, want %q", gotTokenHash, "eq."+expectedHash)
	}

	result := res.Result()
	defer result.Body.Close()

	var cleared *http.Cookie
	for _, c := range result.Cookies() {
		if c.Name == oauthTokenCookieName {
			cleared = c
			break
		}
	}
	if cleared == nil {
		t.Fatal("expected auth cookie to be cleared")
	}
	if cleared.MaxAge >= 0 {
		t.Fatalf("cleared cookie MaxAge = %d, want negative", cleared.MaxAge)
	}
	if cleared.Value != "" {
		t.Fatalf("cleared cookie Value = %q, want empty", cleared.Value)
	}
	if !cleared.HttpOnly {
		t.Fatalf("cleared cookie HttpOnly = false, want true")
	}
}

func TestHeaderGateMiddleware_MissingHeadersReturns401(t *testing.T) {
	sharedSecret := "correct-secret"
	mw := HeaderGateMiddleware(sharedSecret)

	cases := []struct {
		name         string
		vercelHeader string
		secretHeader string
	}{
		{name: "missing_both"},
		{name: "missing_vercel_id", secretHeader: sharedSecret},
		{name: "missing_shared_secret", vercelHeader: "vercel-app"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calledNext := false
			handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calledNext = true
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
			if tc.vercelHeader != "" {
				req.Header.Set("X-Vercel-Id", tc.vercelHeader)
			}
			if tc.secretHeader != "" {
				req.Header.Set("X-Shared-Secret", tc.secretHeader)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if calledNext {
				t.Fatalf("next handler was called for unauthorized request")
			}
			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
			}
			if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
				t.Fatalf("content-type = %q, want application/json", ct)
			}

			var payload struct {
				Message string `json:"message"`
			}
			if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if payload.Message != "unauthorized" {
				t.Fatalf("message = %q, want unauthorized", payload.Message)
			}
		})
	}
}

func TestHeaderGateMiddleware_WrongSecretReturns401(t *testing.T) {
	sharedSecret := "correct-secret"
	mw := HeaderGateMiddleware(sharedSecret)

	calledNext := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledNext = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("X-Vercel-Id", "vercel-app")
	req.Header.Set("X-Shared-Secret", "wrong-secret")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if calledNext {
		t.Fatalf("next handler was called for unauthorized request")
	}
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestHeaderGateMiddleware_ValidHeadersPassThrough(t *testing.T) {
	sharedSecret := "correct-secret"
	mw := HeaderGateMiddleware(sharedSecret)

	calledNext := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledNext = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("X-Vercel-Id", "vercel-app")
	req.Header.Set("X-Shared-Secret", sharedSecret)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !calledNext {
		t.Fatalf("next handler was not called for authorized request")
	}
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

func TestHeaderGateMiddleware_HealthBypassesValidation(t *testing.T) {
	sharedSecret := "correct-secret"
	mw := HeaderGateMiddleware(sharedSecret)

	calledNext := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledNext = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !calledNext {
		t.Fatalf("next handler was not called for /health")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHeaderGateMiddleware_MetricsBypassesValidation(t *testing.T) {
	sharedSecret := "correct-secret"
	mw := HeaderGateMiddleware(sharedSecret)

	calledNext := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledNext = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !calledNext {
		t.Fatalf("next handler was not called for /metrics")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestHeaderGateMiddleware_UsesConstantTimeCompare(t *testing.T) {
	src, err := os.ReadFile("../../internal/middleware/headergate.go")
	if err != nil {
		t.Fatalf("read internal header gate middleware: %v", err)
	}
	if !bytes.Contains(src, []byte("subtle.ConstantTimeCompare")) {
		t.Fatalf("expected header gate to use subtle.ConstantTimeCompare")
	}
}
