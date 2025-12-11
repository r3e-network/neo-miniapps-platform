// Package marble provides the core Marble SDK for MarbleRun integration.
// Each service runs as a Marble inside an EGo SGX enclave.
package marble

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/edgelesssys/ego/attestation"
	"github.com/edgelesssys/ego/enclave"
)

// Marble represents a MarbleRun Marble instance.
// It handles attestation, secrets injection, and secure communication.
type Marble struct {
	mu sync.RWMutex

	// Identity
	marbleType string
	uuid       string

	// TLS credentials from Coordinator
	cert      tls.Certificate
	rootCA    *x509.CertPool
	tlsConfig *tls.Config

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
	certPEM := os.Getenv("MARBLE_CERT")
	keyPEM := os.Getenv("MARBLE_KEY")
	rootPEM := os.Getenv("MARBLE_ROOT_CA")

	if certPEM != "" && keyPEM != "" {
		cert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
		if err != nil {
			return fmt.Errorf("parse TLS certificate: %w", err)
		}
		m.cert = cert
	}

	if rootPEM != "" {
		m.rootCA = x509.NewCertPool()
		if !m.rootCA.AppendCertsFromPEM([]byte(rootPEM)) {
			return fmt.Errorf("parse root CA certificate")
		}
	}

	// Configure TLS for mTLS communication only if we have valid certificates
	// Without certificates from Coordinator, run in HTTP mode (development/simulation)
	if certPEM != "" && keyPEM != "" {
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
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: m.TLSConfig(),
		},
	}
}

// Secret returns a secret by name.
func (m *Marble) Secret(name string) ([]byte, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	secret, ok := m.secrets[name]
	return secret, ok
}

// UseSecret provides secure access to a secret via callback.
// The secret is zeroed after the callback returns.
func (m *Marble) UseSecret(name string, fn func([]byte) error) error {
	m.mu.RLock()
	secret, ok := m.secrets[name]
	m.mu.RUnlock()

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
