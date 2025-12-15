// Package serviceauth provides shared helpers for service-to-service authentication.
package serviceauth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/R3E-Network/service_layer/internal/logging"
)

// =============================================================================
// Service Authentication Constants
// =============================================================================

const (
	// ServiceTokenHeader is the header name for service-to-service tokens.
	ServiceTokenHeader = "X-Service-Token"

	// ServiceIDHeader is the header name for service identification.
	ServiceIDHeader = "X-Service-ID"

	// UserIDHeader is the header name for user identification.
	UserIDHeader = "X-User-ID"

	// DefaultServiceTokenExpiry is the default expiration time for service tokens.
	DefaultServiceTokenExpiry = 1 * time.Hour
)

// =============================================================================
// Context Helpers
// =============================================================================

type contextKey string

const (
	serviceIDKey contextKey = "service_id"
	userIDKey    contextKey = "user_id"
)

// WithServiceID returns a new context with the service ID set.
func WithServiceID(ctx context.Context, serviceID string) context.Context {
	return context.WithValue(ctx, serviceIDKey, serviceID)
}

// GetServiceID extracts service ID from context.
func GetServiceID(ctx context.Context) string {
	if v, ok := ctx.Value(serviceIDKey).(string); ok {
		return v
	}
	return ""
}

// WithUserID returns a new context with the user ID set.
// This is useful for propagating user ID through service-to-service calls.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// GetUserID extracts user ID from context.
func GetUserID(ctx context.Context) string {
	if v, ok := ctx.Value(userIDKey).(string); ok {
		return v
	}
	return ""
}

// =============================================================================
// Service Claims
// =============================================================================

// ServiceClaims represents JWT claims for service-to-service authentication.
type ServiceClaims struct {
	ServiceID string `json:"service_id"`
	jwt.RegisteredClaims
}

// =============================================================================
// Service Token Generator
// =============================================================================

// ServiceTokenGenerator generates service-to-service JWT tokens.
type ServiceTokenGenerator struct {
	privateKey *rsa.PrivateKey
	serviceID  string
	expiry     time.Duration
}

// NewServiceTokenGenerator creates a new service token generator.
func NewServiceTokenGenerator(privateKey *rsa.PrivateKey, serviceID string, expiry time.Duration) *ServiceTokenGenerator {
	if expiry == 0 {
		expiry = DefaultServiceTokenExpiry
	}
	return &ServiceTokenGenerator{
		privateKey: privateKey,
		serviceID:  serviceID,
		expiry:     expiry,
	}
}

// GenerateToken generates a new service token.
func (g *ServiceTokenGenerator) GenerateToken() (string, error) {
	now := time.Now()
	claims := &ServiceClaims{
		ServiceID: g.serviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(g.expiry)),
			Issuer:    "service-layer",
			Subject:   g.serviceID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(g.privateKey)
}

// =============================================================================
// Outbound Request Auth Helpers
// =============================================================================

// ServiceTokenRoundTripper injects X-Service-Token (and optionally X-User-ID)
// into outgoing HTTP requests.
type ServiceTokenRoundTripper struct {
	base      http.RoundTripper
	generator *ServiceTokenGenerator
}

// NewServiceTokenRoundTripper wraps a base transport with service-token injection.
func NewServiceTokenRoundTripper(base http.RoundTripper, generator *ServiceTokenGenerator) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	if generator == nil {
		return base
	}
	return &ServiceTokenRoundTripper{base: base, generator: generator}
}

// RoundTrip implements http.RoundTripper.
func (t *ServiceTokenRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())

	token, err := t.generator.GenerateToken()
	if err != nil {
		return nil, err
	}
	clone.Header.Set(ServiceTokenHeader, token)

	if traceID := logging.GetTraceID(req.Context()); traceID != "" && clone.Header.Get("X-Trace-ID") == "" {
		clone.Header.Set("X-Trace-ID", traceID)
	}

	// Propagate user context when available.
	if userID := GetUserID(req.Context()); userID != "" && clone.Header.Get(UserIDHeader) == "" {
		clone.Header.Set(UserIDHeader, userID)
	}

	return t.base.RoundTrip(clone)
}

// =============================================================================
// Key Parsing Helpers
// =============================================================================

// ParseRSAPublicKeyFromPEM parses an RSA public key from PEM bytes.
// Supported PEM types: PUBLIC KEY (PKIX), RSA PUBLIC KEY (PKCS#1), CERTIFICATE.
func ParseRSAPublicKeyFromPEM(pemBytes []byte) (*rsa.PublicKey, error) {
	rest := pemBytes
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return nil, fmt.Errorf("no PEM public key found")
		}

		switch block.Type {
		case "PUBLIC KEY":
			pubAny, err := x509.ParsePKIXPublicKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("parse PKIX public key: %w", err)
			}
			pub, ok := pubAny.(*rsa.PublicKey)
			if !ok {
				return nil, fmt.Errorf("public key is not RSA")
			}
			return pub, nil
		case "RSA PUBLIC KEY":
			pub, err := x509.ParsePKCS1PublicKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("parse PKCS#1 public key: %w", err)
			}
			return pub, nil
		case "CERTIFICATE":
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("parse certificate: %w", err)
			}
			pub, ok := cert.PublicKey.(*rsa.PublicKey)
			if !ok {
				return nil, fmt.Errorf("certificate public key is not RSA")
			}
			return pub, nil
		}

		if len(rest) == 0 {
			return nil, fmt.Errorf("no supported PEM public key found")
		}
	}
}

// ParseRSAPrivateKeyFromPEM parses an RSA private key from PEM bytes.
// Supported PEM types: RSA PRIVATE KEY (PKCS#1), PRIVATE KEY (PKCS#8).
func ParseRSAPrivateKeyFromPEM(pemBytes []byte) (*rsa.PrivateKey, error) {
	rest := pemBytes
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return nil, fmt.Errorf("no PEM private key found")
		}

		switch block.Type {
		case "RSA PRIVATE KEY":
			priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("parse PKCS#1 private key: %w", err)
			}
			return priv, nil
		case "PRIVATE KEY":
			key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("parse PKCS#8 private key: %w", err)
			}
			priv, ok := key.(*rsa.PrivateKey)
			if !ok {
				return nil, fmt.Errorf("private key is not RSA")
			}
			return priv, nil
		}

		if len(rest) == 0 {
			return nil, fmt.Errorf("no supported PEM private key found")
		}
	}
}
