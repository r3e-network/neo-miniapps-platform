package coordinator

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/tee"
)

// TLSManager handles TLS certificate generation and policies.
type TLSManager struct {
	mu sync.RWMutex

	trustRoot *tee.TrustRoot

	// Root CA
	rootCACert *x509.Certificate
	rootCAKey  *ecdsa.PrivateKey
	rootCAPEM  []byte

	// TLS policies from manifest
	config *os.TLSConfig

	// Issued certificates (marble UUID -> cert)
	issued map[string]*IssuedCert
}

// IssuedCert holds an issued certificate.
type IssuedCert struct {
	MarbleName string
	UUID       string
	Cert       *x509.Certificate
	CertPEM    []byte
	KeyPEM     []byte
	IssuedAt   time.Time
	ExpiresAt  time.Time
}

// NewTLSManager creates a new TLSManager.
func NewTLSManager(trustRoot *tee.TrustRoot) (*TLSManager, error) {
	tm := &TLSManager{
		trustRoot: trustRoot,
		issued:    make(map[string]*IssuedCert),
	}

	// Generate root CA
	if err := tm.generateRootCA(); err != nil {
		return nil, fmt.Errorf("generate root CA: %w", err)
	}

	return tm, nil
}

// Initialize initializes the TLS manager with manifest config.
func (tm *TLSManager) Initialize(ctx context.Context, config *os.TLSConfig) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.config = config
	return nil
}

// IssueCertificate issues a TLS certificate for a marble.
func (tm *TLSManager) IssueCertificate(ctx context.Context, marbleName, uuid string) (cert, key, rootCA []byte, err error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Generate key pair for the marble
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("generate key: %w", err)
	}

	// Create certificate template
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName:   marbleName,
			Organization: []string{"Neo Service Layer"},
		},
		DNSNames:              []string{marbleName, uuid, marbleName + ".service-layer.local"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0), // 1 year validity
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	// Sign with root CA
	certDER, err := x509.CreateCertificate(rand.Reader, template, tm.rootCACert, &privateKey.PublicKey, tm.rootCAKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create certificate: %w", err)
	}

	// Parse the certificate
	parsedCert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parse certificate: %w", err)
	}

	// Encode certificate
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	// Encode private key
	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshal private key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	})

	// Store issued certificate
	tm.issued[uuid] = &IssuedCert{
		MarbleName: marbleName,
		UUID:       uuid,
		Cert:       parsedCert,
		CertPEM:    certPEM,
		KeyPEM:     keyPEM,
		IssuedAt:   time.Now(),
		ExpiresAt:  template.NotAfter,
	}

	return certPEM, keyPEM, tm.rootCAPEM, nil
}

// RevokeCertificate revokes a marble's certificate.
func (tm *TLSManager) RevokeCertificate(ctx context.Context, uuid string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, ok := tm.issued[uuid]; !ok {
		return fmt.Errorf("certificate not found: %s", uuid)
	}

	delete(tm.issued, uuid)
	return nil
}

// GetRootCA returns the root CA certificate.
func (tm *TLSManager) GetRootCA() []byte {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.rootCAPEM
}

// IsAllowed checks if a connection is allowed based on TLS policies.
func (tm *TLSManager) IsAllowed(fromMarble, toMarble, port string, incoming bool) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if tm.config == nil {
		return true // No policies = allow all
	}

	var policies map[string][]os.TLSPolicy
	if incoming {
		policies = tm.config.Incoming
	} else {
		policies = tm.config.Outgoing
	}

	marblePolicies, ok := policies[toMarble]
	if !ok {
		return true // No policy for this marble = allow all
	}

	for _, policy := range marblePolicies {
		if policy.Port != port && policy.Port != "*" {
			continue
		}

		// Check if fromMarble is allowed
		if len(policy.AllowedMarbles) == 0 {
			return true // Empty = allow all
		}

		for _, allowed := range policy.AllowedMarbles {
			if allowed == fromMarble || allowed == "*" {
				return true
			}
		}
	}

	return false
}

// generateRootCA generates the mesh root CA.
func (tm *TLSManager) generateRootCA() error {
	// Generate CA key pair
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate CA key: %w", err)
	}

	// Create CA certificate template
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "Neo Service Layer Mesh CA",
			Organization: []string{"Neo Service Layer"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // 10 years
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	// Self-sign
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("create CA certificate: %w", err)
	}

	// Parse the certificate
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return fmt.Errorf("parse CA certificate: %w", err)
	}

	// Encode certificate
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	tm.rootCACert = cert
	tm.rootCAKey = privateKey
	tm.rootCAPEM = certPEM

	return nil
}

// Zero zeros sensitive data.
func (tm *TLSManager) Zero() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.rootCAKey = nil
	for uuid, cert := range tm.issued {
		zeroBytes(cert.KeyPEM)
		delete(tm.issued, uuid)
	}
}
