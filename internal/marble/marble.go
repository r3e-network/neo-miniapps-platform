// Package marble provides the core Marble SDK for MarbleRun integration.
// Each service runs as a Marble inside an EGo SGX enclave.
package marble

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/edgelesssys/ego/attestation"
	"github.com/edgelesssys/ego/enclave"

	"github.com/R3E-Network/service_layer/internal/logging"
)

// Marble represents a MarbleRun Marble instance.
// It handles attestation, secrets injection, and secure communication.
type Marble struct {
	mu sync.RWMutex

	// Identity
	marbleType string
	uuid       string

	// TLS credentials from Coordinator
	cert               tls.Certificate
	rootCA             *x509.CertPool
	tlsConfig          *tls.Config
	httpClientUsesMTLS bool
	httpClient         *http.Client
	externalHTTPClient *http.Client

	// Secrets injected by Coordinator
	secrets map[string][]byte

	// Enclave report
	report *attestation.Report

	// State
	initialized bool
}

// Config holds Marble configuration.
type Config struct {
	MarbleType string
	DNSNames   []string
}

// New creates a new Marble instance.
func New(cfg Config) (*Marble, error) {
	m := &Marble{
		marbleType: cfg.MarbleType,
		secrets:    make(map[string][]byte),
	}

	// Get enclave self-report for attestation
	report, err := enclave.GetSelfReport()
	if err != nil {
		// Running outside enclave (simulation mode)
		m.report = nil
	} else {
		m.report = &report
	}

	return m, nil
}

// Initialize performs Marble initialization with the Coordinator.
// This is called automatically by MarbleRun before the application starts.
func (m *Marble) Initialize(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initialized {
		return nil
	}

	// In MarbleRun, the Coordinator injects:
	// 1. TLS certificate and private key via environment variables
	// 2. Root CA certificate for verifying other Marbles
	// 3. Secrets defined in the manifest

	// Load TLS certificate from environment (injected by Coordinator)
	certPEM := strings.TrimSpace(os.Getenv("MARBLE_CERT"))
	keyPEM := strings.TrimSpace(os.Getenv("MARBLE_KEY"))
	rootPEM := strings.TrimSpace(os.Getenv("MARBLE_ROOT_CA"))

	hasCertKey := certPEM != "" && keyPEM != ""
	hasRootCA := rootPEM != ""

	// MarbleRun mTLS requires a private root CA for the mesh. If the Coordinator
	// injected a leaf certificate/key but no root CA, fail fast instead of falling
	// back to system roots (which would silently weaken trust boundaries).
	if hasCertKey && !hasRootCA {
		return fmt.Errorf("MARBLE_ROOT_CA is required when MARBLE_CERT and MARBLE_KEY are set")
	}

	if hasCertKey {
		cert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
		if err != nil {
			return fmt.Errorf("parse TLS certificate: %w", err)
		}
		m.cert = cert
	}

	if hasRootCA {
		m.rootCA = x509.NewCertPool()
		if !m.rootCA.AppendCertsFromPEM([]byte(rootPEM)) {
			return fmt.Errorf("parse root CA certificate")
		}
	}

	// Configure TLS for mTLS communication only if we have valid certificates
	// Without certificates from Coordinator, run in HTTP mode (development/simulation)
	if hasCertKey {
		if m.rootCA == nil {
			return fmt.Errorf("failed to initialize MarbleRun mTLS: missing root CA pool")
		}
		m.tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{m.cert},
			RootCAs:      m.rootCA,
			ClientCAs:    m.rootCA,
			ClientAuth:   tls.RequireAndVerifyClientCert,
			MinVersion:   tls.VersionTLS13,
		}
	}

	// Load secrets from environment (injected by Coordinator)
	secretsJSON := os.Getenv("MARBLE_SECRETS")
	if secretsJSON != "" {
		if err := json.Unmarshal([]byte(secretsJSON), &m.secrets); err != nil {
			return fmt.Errorf("parse secrets: %w", err)
		}
	}

	// Get UUID assigned by Coordinator
	m.uuid = os.Getenv("MARBLE_UUID")

	m.initialized = true
	return nil
}

// TLSConfig returns the TLS configuration for secure communication.
func (m *Marble) TLSConfig() *tls.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tlsConfig
}

// HTTPClient returns an HTTP client configured for mTLS.
func (m *Marble) HTTPClient() *http.Client {
	if m == nil {
		return &http.Client{
			Transport: &traceHeaderRoundTripper{base: http.DefaultTransport},
			Timeout:   30 * time.Second,
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	useMTLS := m.tlsConfig != nil
	if m.httpClient != nil && m.httpClientUsesMTLS == useMTLS {
		return m.httpClient
	}

	if !useMTLS {
		m.httpClientUsesMTLS = false
		m.httpClient = &http.Client{
			Transport: &traceHeaderRoundTripper{base: http.DefaultTransport},
			Timeout:   30 * time.Second,
		}
		return m.httpClient
	}

	tlsCfg := m.tlsConfig.Clone()

	// Clone the default transport to preserve sane defaults (proxy env support,
	// dial timeouts, HTTP/2, connection pooling) while injecting the MarbleRun
	// mTLS credentials.
	var transport http.RoundTripper
	if base, ok := http.DefaultTransport.(*http.Transport); ok {
		cloned := base.Clone()
		cloned.TLSClientConfig = tlsCfg
		transport = cloned
	} else {
		transport = &http.Transport{
			TLSClientConfig: tlsCfg,
		}
	}

	m.httpClientUsesMTLS = true
	m.httpClient = &http.Client{
		Transport: &traceHeaderRoundTripper{base: transport},
		Timeout:   30 * time.Second,
	}
	return m.httpClient
}

// ExternalHTTPClient returns an HTTP client suitable for outbound calls to
// non-Marblerun endpoints (Supabase, Neo RPC, third-party APIs).
//
// It never installs the MarbleRun root CA or client certificate, ensuring the
// connection uses the system trust store and does not attempt mTLS.
func (m *Marble) ExternalHTTPClient() *http.Client {
	if m == nil {
		return &http.Client{
			Transport: &traceHeaderRoundTripper{base: http.DefaultTransport},
			Timeout:   30 * time.Second,
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.externalHTTPClient != nil {
		return m.externalHTTPClient
	}

	var transport http.RoundTripper
	if base, ok := http.DefaultTransport.(*http.Transport); ok {
		cloned := base.Clone()
		if cloned.TLSClientConfig != nil {
			cloned.TLSClientConfig = cloned.TLSClientConfig.Clone()
			// Enforce a modern TLS baseline for external calls.
			if cloned.TLSClientConfig.MinVersion == 0 || cloned.TLSClientConfig.MinVersion < tls.VersionTLS12 {
				cloned.TLSClientConfig.MinVersion = tls.VersionTLS12
			}
		} else {
			cloned.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		}
		transport = cloned
	} else {
		transport = http.DefaultTransport
	}

	m.externalHTTPClient = &http.Client{
		Transport: &traceHeaderRoundTripper{base: transport},
		Timeout:   30 * time.Second,
	}
	return m.externalHTTPClient
}

type traceHeaderRoundTripper struct {
	base http.RoundTripper
}

func (t *traceHeaderRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.base == nil {
		t.base = http.DefaultTransport
	}

	traceID := logging.GetTraceID(req.Context())
	if traceID == "" || req.Header.Get("X-Trace-ID") != "" {
		return t.base.RoundTrip(req)
	}

	clone := req.Clone(req.Context())
	clone.Header.Set("X-Trace-ID", traceID)
	return t.base.RoundTrip(clone)
}

func decodeHexEnvSecret(value string) ([]byte, bool) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "0x")
	value = strings.TrimPrefix(value, "0X")
	if value == "" || len(value)%2 != 0 {
		return nil, false
	}

	for _, ch := range value {
		switch {
		case '0' <= ch && ch <= '9':
		case 'a' <= ch && ch <= 'f':
		case 'A' <= ch && ch <= 'F':
		default:
			return nil, false
		}
	}

	decoded, err := hex.DecodeString(value)
	if err != nil {
		return nil, false
	}
	return decoded, true
}

// Secret returns a secret by name.
func (m *Marble) Secret(name string) ([]byte, bool) {
	m.mu.RLock()
	secret, ok := m.secrets[name]
	m.mu.RUnlock()
	if ok {
		return secret, true
	}

	// Fallback: allow secrets injected as direct env vars (common in MarbleRun manifests).
	envValue, ok := os.LookupEnv(name)
	if !ok || strings.TrimSpace(envValue) == "" {
		return nil, false
	}

	decoded := []byte(envValue)
	if hexDecoded, ok := decodeHexEnvSecret(envValue); ok {
		decoded = hexDecoded
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if secret, ok := m.secrets[name]; ok {
		return secret, true
	}
	m.secrets[name] = decoded
	return decoded, true
}

// UseSecret provides secure access to a secret via callback.
// The secret is zeroed after the callback returns.
func (m *Marble) UseSecret(name string, fn func([]byte) error) error {
	secret, ok := m.Secret(name)

	if !ok {
		return fmt.Errorf("secret not found: %s", name)
	}

	// Make a copy for the callback
	secretCopy := make([]byte, len(secret))
	copy(secretCopy, secret)
	defer zeroBytes(secretCopy)

	return fn(secretCopy)
}

// Report returns the enclave self-report.
func (m *Marble) Report() *attestation.Report {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.report
}

// UUID returns the Marble's unique identifier.
func (m *Marble) UUID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.uuid
}

// MarbleType returns the Marble type.
func (m *Marble) MarbleType() string {
	return m.marbleType
}

// IsEnclave returns true if running inside an SGX enclave.
func (m *Marble) IsEnclave() bool {
	return m.report != nil
}

// zeroBytes securely zeros a byte slice.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// SetTestSecret sets a secret for testing purposes only.
// This method should only be used in tests.
func (m *Marble) SetTestSecret(name string, value []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.secrets[name] = value
}

// SetTestReport sets an enclave report for testing purposes only.
// This method should only be used in tests.
func (m *Marble) SetTestReport(report *attestation.Report) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.report = report
}
