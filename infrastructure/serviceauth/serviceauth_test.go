package serviceauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()

	// Test WithServiceID and GetServiceID
	ctx = WithServiceID(ctx, "test-service")
	if got := GetServiceID(ctx); got != "test-service" {
		t.Errorf("GetServiceID() = %q, want %q", got, "test-service")
	}

	// Test WithUserID and GetUserID
	ctx = WithUserID(ctx, "user-123")
	if got := GetUserID(ctx); got != "user-123" {
		t.Errorf("GetUserID() = %q, want %q", got, "user-123")
	}

	// Test empty context
	emptyCtx := context.Background()
	if got := GetServiceID(emptyCtx); got != "" {
		t.Errorf("GetServiceID(empty) = %q, want empty", got)
	}
	if got := GetUserID(emptyCtx); got != "" {
		t.Errorf("GetUserID(empty) = %q, want empty", got)
	}
}

func generateTestRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	return key
}

func TestServiceTokenGenerator(t *testing.T) {
	privateKey := generateTestRSAKey(t)

	t.Run("default expiry", func(t *testing.T) {
		gen := NewServiceTokenGenerator(privateKey, "test-service", 0)
		if gen.expiry != DefaultServiceTokenExpiry {
			t.Errorf("expiry = %v, want %v", gen.expiry, DefaultServiceTokenExpiry)
		}
	})

	t.Run("custom expiry", func(t *testing.T) {
		customExpiry := 30 * time.Minute
		gen := NewServiceTokenGenerator(privateKey, "test-service", customExpiry)
		if gen.expiry != customExpiry {
			t.Errorf("expiry = %v, want %v", gen.expiry, customExpiry)
		}
	})

	t.Run("generate token", func(t *testing.T) {
		gen := NewServiceTokenGenerator(privateKey, "test-service", time.Hour)
		token, err := gen.GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken() error = %v", err)
		}
		if token == "" {
			t.Error("GenerateToken() returned empty token")
		}
	})
}

func TestServiceTokenRoundTripper(t *testing.T) {
	privateKey := generateTestRSAKey(t)
	gen := NewServiceTokenGenerator(privateKey, "test-service", time.Hour)

	t.Run("nil generator returns base", func(t *testing.T) {
		rt := NewServiceTokenRoundTripper(http.DefaultTransport, nil)
		if rt != http.DefaultTransport {
			t.Error("expected base transport when generator is nil")
		}
	})

	t.Run("nil base uses default", func(t *testing.T) {
		rt := NewServiceTokenRoundTripper(nil, gen)
		if rt == nil {
			t.Error("expected non-nil round tripper")
		}
	})

	t.Run("injects token header", func(t *testing.T) {
		var capturedHeader string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedHeader = r.Header.Get(ServiceTokenHeader)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		rt := NewServiceTokenRoundTripper(http.DefaultTransport, gen)
		client := &http.Client{Transport: rt}

		req, _ := http.NewRequest("GET", server.URL, nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp.Body.Close()

		if capturedHeader == "" {
			t.Error("ServiceTokenHeader not set")
		}
	})

	t.Run("propagates user ID", func(t *testing.T) {
		var capturedUserID string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedUserID = r.Header.Get(UserIDHeader)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		rt := NewServiceTokenRoundTripper(http.DefaultTransport, gen)
		client := &http.Client{Transport: rt}

		ctx := WithUserID(context.Background(), "user-456")
		req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp.Body.Close()

		if capturedUserID != "user-456" {
			t.Errorf("UserIDHeader = %q, want %q", capturedUserID, "user-456")
		}
	})
}

func TestParseRSAPublicKeyFromPEM(t *testing.T) {
	privateKey := generateTestRSAKey(t)

	t.Run("PKIX format", func(t *testing.T) {
		pubBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
		if err != nil {
			t.Fatalf("failed to marshal public key: %v", err)
		}
		pemBytes := pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: pubBytes,
		})

		pub, err := ParseRSAPublicKeyFromPEM(pemBytes)
		if err != nil {
			t.Fatalf("ParseRSAPublicKeyFromPEM() error = %v", err)
		}
		if pub == nil {
			t.Error("ParseRSAPublicKeyFromPEM() returned nil")
		}
	})

	t.Run("PKCS1 format", func(t *testing.T) {
		pubBytes := x509.MarshalPKCS1PublicKey(&privateKey.PublicKey)
		pemBytes := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: pubBytes,
		})

		pub, err := ParseRSAPublicKeyFromPEM(pemBytes)
		if err != nil {
			t.Fatalf("ParseRSAPublicKeyFromPEM() error = %v", err)
		}
		if pub == nil {
			t.Error("ParseRSAPublicKeyFromPEM() returned nil")
		}
	})

	t.Run("invalid PEM", func(t *testing.T) {
		_, err := ParseRSAPublicKeyFromPEM([]byte("not a pem"))
		if err == nil {
			t.Error("expected error for invalid PEM")
		}
	})

	t.Run("wrong block type", func(t *testing.T) {
		pemBytes := pem.EncodeToMemory(&pem.Block{
			Type:  "UNKNOWN TYPE",
			Bytes: []byte("data"),
		})
		_, err := ParseRSAPublicKeyFromPEM(pemBytes)
		if err == nil {
			t.Error("expected error for unknown block type")
		}
	})
}

func TestParseRSAPrivateKeyFromPEM(t *testing.T) {
	privateKey := generateTestRSAKey(t)

	t.Run("PKCS1 format", func(t *testing.T) {
		privBytes := x509.MarshalPKCS1PrivateKey(privateKey)
		pemBytes := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: privBytes,
		})

		priv, err := ParseRSAPrivateKeyFromPEM(pemBytes)
		if err != nil {
			t.Fatalf("ParseRSAPrivateKeyFromPEM() error = %v", err)
		}
		if priv == nil {
			t.Error("ParseRSAPrivateKeyFromPEM() returned nil")
		}
	})

	t.Run("PKCS8 format", func(t *testing.T) {
		privBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
		if err != nil {
			t.Fatalf("failed to marshal private key: %v", err)
		}
		pemBytes := pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: privBytes,
		})

		priv, err := ParseRSAPrivateKeyFromPEM(pemBytes)
		if err != nil {
			t.Fatalf("ParseRSAPrivateKeyFromPEM() error = %v", err)
		}
		if priv == nil {
			t.Error("ParseRSAPrivateKeyFromPEM() returned nil")
		}
	})

	t.Run("invalid PEM", func(t *testing.T) {
		_, err := ParseRSAPrivateKeyFromPEM([]byte("not a pem"))
		if err == nil {
			t.Error("expected error for invalid PEM")
		}
	})
}

func TestConstants(t *testing.T) {
	if ServiceTokenHeader != "X-Service-Token" {
		t.Errorf("ServiceTokenHeader = %q, want %q", ServiceTokenHeader, "X-Service-Token")
	}
	if ServiceIDHeader != "X-Service-ID" {
		t.Errorf("ServiceIDHeader = %q, want %q", ServiceIDHeader, "X-Service-ID")
	}
	if UserIDHeader != "X-User-ID" {
		t.Errorf("UserIDHeader = %q, want %q", UserIDHeader, "X-User-ID")
	}
	if DefaultServiceTokenExpiry != time.Hour {
		t.Errorf("DefaultServiceTokenExpiry = %v, want %v", DefaultServiceTokenExpiry, time.Hour)
	}
}

func TestParseRSAPublicKeyFromPEMCertificate(t *testing.T) {
	privateKey := generateTestRSAKey(t)

	t.Run("CERTIFICATE format", func(t *testing.T) {
		// Create a self-signed certificate
		template := &x509.Certificate{
			SerialNumber: big.NewInt(1),
			Subject: pkix.Name{
				Organization: []string{"Test"},
			},
			NotBefore:             time.Now(),
			NotAfter:              time.Now().Add(time.Hour),
			KeyUsage:              x509.KeyUsageDigitalSignature,
			BasicConstraintsValid: true,
		}

		certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
		if err != nil {
			t.Fatalf("failed to create certificate: %v", err)
		}

		pemBytes := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: certDER,
		})

		pub, err := ParseRSAPublicKeyFromPEM(pemBytes)
		if err != nil {
			t.Fatalf("ParseRSAPublicKeyFromPEM() error = %v", err)
		}
		if pub == nil {
			t.Error("ParseRSAPublicKeyFromPEM() returned nil")
		}
	})

	t.Run("invalid certificate data", func(t *testing.T) {
		pemBytes := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: []byte("invalid certificate data"),
		})

		_, err := ParseRSAPublicKeyFromPEM(pemBytes)
		if err == nil {
			t.Error("expected error for invalid certificate")
		}
	})

	t.Run("invalid PKIX data", func(t *testing.T) {
		pemBytes := pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: []byte("invalid pkix data"),
		})

		_, err := ParseRSAPublicKeyFromPEM(pemBytes)
		if err == nil {
			t.Error("expected error for invalid PKIX data")
		}
	})

	t.Run("invalid PKCS1 data", func(t *testing.T) {
		pemBytes := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: []byte("invalid pkcs1 data"),
		})

		_, err := ParseRSAPublicKeyFromPEM(pemBytes)
		if err == nil {
			t.Error("expected error for invalid PKCS1 data")
		}
	})

	t.Run("unsupported PEM type skipped", func(t *testing.T) {
		// First block is unsupported, second is valid
		pubBytes, _ := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
		pemBytes := pem.EncodeToMemory(&pem.Block{
			Type:  "UNSUPPORTED TYPE",
			Bytes: []byte("data"),
		})
		pemBytes = append(pemBytes, pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: pubBytes,
		})...)

		pub, err := ParseRSAPublicKeyFromPEM(pemBytes)
		if err != nil {
			t.Fatalf("ParseRSAPublicKeyFromPEM() error = %v", err)
		}
		if pub == nil {
			t.Error("ParseRSAPublicKeyFromPEM() returned nil")
		}
	})
}

func TestParseRSAPrivateKeyFromPEMEdgeCases(t *testing.T) {
	t.Run("invalid PKCS1 data", func(t *testing.T) {
		pemBytes := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: []byte("invalid pkcs1 data"),
		})

		_, err := ParseRSAPrivateKeyFromPEM(pemBytes)
		if err == nil {
			t.Error("expected error for invalid PKCS1 data")
		}
	})

	t.Run("invalid PKCS8 data", func(t *testing.T) {
		pemBytes := pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: []byte("invalid pkcs8 data"),
		})

		_, err := ParseRSAPrivateKeyFromPEM(pemBytes)
		if err == nil {
			t.Error("expected error for invalid PKCS8 data")
		}
	})

	t.Run("unsupported PEM type only", func(t *testing.T) {
		pemBytes := pem.EncodeToMemory(&pem.Block{
			Type:  "EC PRIVATE KEY",
			Bytes: []byte("data"),
		})

		_, err := ParseRSAPrivateKeyFromPEM(pemBytes)
		if err == nil {
			t.Error("expected error for unsupported PEM type")
		}
	})
}
