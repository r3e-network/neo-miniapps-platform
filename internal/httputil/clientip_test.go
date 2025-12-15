package httputil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIP_TrustsForwardedHeadersFromPrivatePeer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.2:1234"
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")

	if got := ClientIP(req); got != "1.2.3.4" {
		t.Fatalf("ClientIP() = %q, want %q", got, "1.2.3.4")
	}
}

func TestClientIP_IgnoresForwardedHeadersFromPublicPeer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set("X-Real-IP", "9.9.9.9")

	if got := ClientIP(req); got != "203.0.113.10" {
		t.Fatalf("ClientIP() = %q, want %q", got, "203.0.113.10")
	}
}
