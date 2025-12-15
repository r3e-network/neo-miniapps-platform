package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
)

func TestProxyHandler_ForwardsIdentityAndStripsSpoofableHeaders(t *testing.T) {
	var gotUserID, gotRole, gotAuth, gotCookie, gotServiceToken, gotAPIKey, gotXFF, gotRealIP string

	prevTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotUserID = r.Header.Get("X-User-ID")
		gotRole = r.Header.Get("X-User-Role")
		gotAuth = r.Header.Get("Authorization")
		gotCookie = r.Header.Get("Cookie")
		gotServiceToken = r.Header.Get("X-Service-Token")
		gotAPIKey = r.Header.Get("X-API-Key")
		gotXFF = r.Header.Get("X-Forwarded-For")
		gotRealIP = r.Header.Get("X-Real-IP")
		return jsonResponse(r, http.StatusOK, "{}"), nil
	})
	t.Cleanup(func() { http.DefaultTransport = prevTransport })

	prev := serviceEndpoints["neovault"]
	serviceEndpoints["neovault"] = "http://neovault.example"
	t.Cleanup(func() { serviceEndpoints["neovault"] = prev })

	req := httptest.NewRequest(http.MethodGet, "/api/v1/neovault/admin/registrations", nil)
	req = mux.SetURLVars(req, map[string]string{"path": "admin/registrations"})

	req.Header.Set("X-User-ID", "user-123")
	req.Header.Set("X-User-Role", "admin")
	req.Header.Set("Authorization", "Bearer evil")
	req.Header.Set("Cookie", "evil=1")
	req.Header.Set("X-Service-Token", "evil")
	req.Header.Set("X-API-Key", "evil")
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set("X-Real-IP", "9.9.9.9")
	req.RemoteAddr = "203.0.113.10:1234"

	rr := httptest.NewRecorder()
	proxyHandler("neovault", nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if gotUserID != "user-123" {
		t.Fatalf("upstream X-User-ID = %q, want %q", gotUserID, "user-123")
	}
	if gotRole != "admin" {
		t.Fatalf("upstream X-User-Role = %q, want %q", gotRole, "admin")
	}
	if gotAuth != "" {
		t.Fatalf("upstream Authorization = %q, want empty", gotAuth)
	}
	if gotCookie != "" {
		t.Fatalf("upstream Cookie = %q, want empty", gotCookie)
	}
	if gotServiceToken != "" {
		t.Fatalf("upstream X-Service-Token = %q, want empty", gotServiceToken)
	}
	if gotAPIKey != "" {
		t.Fatalf("upstream X-API-Key = %q, want empty", gotAPIKey)
	}
	if gotXFF != "203.0.113.10" {
		t.Fatalf("upstream X-Forwarded-For = %q, want %q", gotXFF, "203.0.113.10")
	}
	if gotRealIP != "203.0.113.10" {
		t.Fatalf("upstream X-Real-IP = %q, want %q", gotRealIP, "203.0.113.10")
	}
}
