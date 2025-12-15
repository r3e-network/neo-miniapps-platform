package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/R3E-Network/service_layer/internal/logging"
	"github.com/R3E-Network/service_layer/internal/metrics"
)

type testRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f testRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestCORSMiddleware_AllowsWildcardAndPreflight(t *testing.T) {
	mw := NewCORSMiddleware(&CORSConfig{AllowedOrigins: []string{"*"}})
	nextCalled := false
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Fatalf("allow-origin = %q, want https://example.com", rr.Header().Get("Access-Control-Allow-Origin"))
	}
	if !nextCalled {
		t.Fatalf("expected handler to be called")
	}

	nextCalled = false
	req = httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
	if nextCalled {
		t.Fatalf("preflight should not call handler")
	}
}

func TestCORSMiddleware_AllowsSuffixOrigins(t *testing.T) {
	mw := NewCORSMiddleware(&CORSConfig{AllowedOrigins: []string{".example.com"}})
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://api.example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Header().Get("Access-Control-Allow-Origin") != "https://api.example.com" {
		t.Fatalf("allow-origin = %q, want https://api.example.com", rr.Header().Get("Access-Control-Allow-Origin"))
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://not-allowed.com")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("unexpected allow-origin header for disallowed origin")
	}
}

func TestMetricsMiddleware_UsesRouteTemplateAndStatusWriter(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := metrics.NewWithRegistry("test", reg)

	router := mux.NewRouter()
	router.Use(MetricsMiddleware("test", m))
	router.HandleFunc("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	}).Methods(http.MethodPost)

	req := httptest.NewRequest(http.MethodPost, "/users/123", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rr.Code)
	}
}

func TestLoggingMiddleware_SetsTraceHeaderAndContext(t *testing.T) {
	logger := logging.New("test", "error", "text")
	router := mux.NewRouter()
	router.Use(LoggingMiddleware(logger))
	router.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Seen-Trace", logging.GetTraceID(r.Context()))
		_, _ = w.Write([]byte("pong"))
	}).Methods(http.MethodGet)

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("X-Trace-ID", "trace-123")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Header().Get("X-Trace-ID") != "trace-123" {
		t.Fatalf("X-Trace-ID = %q, want trace-123", rr.Header().Get("X-Trace-ID"))
	}
	if rr.Header().Get("X-Seen-Trace") != "trace-123" {
		t.Fatalf("X-Seen-Trace = %q, want trace-123", rr.Header().Get("X-Seen-Trace"))
	}
}

func TestTracingMiddleware_GeneratesTraceID(t *testing.T) {
	logger := logging.New("test", "error", "text")
	mw := NewTracingMiddleware(logger)
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Seen-Trace", logging.GetTraceID(r.Context()))
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("X-Trace-ID") == "" {
		t.Fatalf("expected X-Trace-ID to be set")
	}
	if rr.Header().Get("X-Seen-Trace") != rr.Header().Get("X-Trace-ID") {
		t.Fatalf("trace ID mismatch between context and header")
	}
}

func TestRecoveryMiddleware_RecoversFromPanics(t *testing.T) {
	logger := logging.New("test", "error", "text")
	mw := NewRecoveryMiddleware(logger)
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type = %q, want application/json", ct)
	}
}

func TestResponseWriter_CapturesStatus(t *testing.T) {
	rr := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rr, statusCode: http.StatusOK}
	rw.WriteHeader(http.StatusCreated)
	rw.WriteHeader(http.StatusAccepted) // should be ignored

	if rw.statusCode != http.StatusCreated {
		t.Fatalf("statusCode = %d, want 201", rw.statusCode)
	}
	if rr.Code != http.StatusCreated {
		t.Fatalf("recorder code = %d, want 201", rr.Code)
	}

	rr = httptest.NewRecorder()
	rw = &responseWriter{ResponseWriter: rr, statusCode: http.StatusOK}
	_, _ = rw.Write([]byte("ok"))
	if rw.statusCode != http.StatusOK {
		t.Fatalf("statusCode = %d, want 200", rw.statusCode)
	}
}

func TestGetUserRole(t *testing.T) {
	ctx := logging.WithRole(context.Background(), "admin")
	if role := GetUserRole(ctx); role != "admin" {
		t.Fatalf("GetUserRole() = %q, want admin", role)
	}
}

func TestServiceTokenRoundTripper_Defaults(t *testing.T) {
	if got := NewServiceTokenRoundTripper(nil, nil); got != http.DefaultTransport {
		t.Fatalf("expected default transport when base and generator are nil")
	}

	base := testRoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get(ServiceTokenHeader) != "" {
			t.Fatalf("unexpected token header when generator is nil")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
	})

	got := NewServiceTokenRoundTripper(base, nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, err := got.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
}

func TestBodyLimitMiddleware_RejectsWhenContentLengthTooLarge(t *testing.T) {
	mw := NewBodyLimitMiddleware(10)

	nextCalled := false
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("01234567890"))
	req.ContentLength = 11

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if nextCalled {
		t.Fatalf("expected body limit middleware to short-circuit")
	}
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rr.Code)
	}
}

func TestBodyLimitMiddleware_AllowsWhenContentLengthWithinLimit(t *testing.T) {
	mw := NewBodyLimitMiddleware(10)

	nextCalled := false
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("0123456789"))
	req.ContentLength = 10

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !nextCalled {
		t.Fatalf("expected handler to be called")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

func TestParseRSAKeysFromPEM(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}

	pkixBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey: %v", err)
	}
	pkixPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pkixBytes})
	pub, err := ParseRSAPublicKeyFromPEM(pkixPEM)
	if err != nil {
		t.Fatalf("ParseRSAPublicKeyFromPEM(PKIX): %v", err)
	}
	if pub.N.Cmp(privateKey.PublicKey.N) != 0 {
		t.Fatalf("parsed public key mismatch")
	}

	pkcs1PEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: x509.MarshalPKCS1PublicKey(&privateKey.PublicKey)})
	pub, err = ParseRSAPublicKeyFromPEM(pkcs1PEM)
	if err != nil {
		t.Fatalf("ParseRSAPublicKeyFromPEM(PKCS1): %v", err)
	}
	if pub.E != privateKey.PublicKey.E {
		t.Fatalf("parsed public key mismatch")
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	pub, err = ParseRSAPublicKeyFromPEM(certPEM)
	if err != nil {
		t.Fatalf("ParseRSAPublicKeyFromPEM(CERTIFICATE): %v", err)
	}
	if pub == nil {
		t.Fatalf("expected RSA public key in certificate")
	}
	if pub.N.Cmp(privateKey.PublicKey.N) != 0 {
		t.Fatalf("parsed public key mismatch")
	}

	pkcs1PrivPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	priv, err := ParseRSAPrivateKeyFromPEM(pkcs1PrivPEM)
	if err != nil {
		t.Fatalf("ParseRSAPrivateKeyFromPEM(PKCS1): %v", err)
	}
	if priv.PublicKey.N.Cmp(privateKey.PublicKey.N) != 0 {
		t.Fatalf("parsed private key mismatch")
	}

	pkcs8DER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	pkcs8PEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8DER})
	_, err = ParseRSAPrivateKeyFromPEM(pkcs8PEM)
	if err != nil {
		t.Fatalf("ParseRSAPrivateKeyFromPEM(PKCS8): %v", err)
	}

	if _, err := ParseRSAPublicKeyFromPEM([]byte("not pem")); err == nil {
		t.Fatalf("expected error for invalid public key input")
	}
	if _, err := ParseRSAPrivateKeyFromPEM([]byte("not pem")); err == nil {
		t.Fatalf("expected error for invalid private key input")
	}
}
