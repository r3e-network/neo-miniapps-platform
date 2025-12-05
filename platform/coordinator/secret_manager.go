package coordinator

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
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

// SecretManager generates and manages secrets for marbles.
type SecretManager struct {
	mu sync.RWMutex

	trustRoot *tee.TrustRoot

	// Generated secrets (sealed in TEE)
	generated map[string]*GeneratedSecret

	// User-defined secrets
	userDefined map[string][]byte
}

// GeneratedSecret holds a generated secret.
type GeneratedSecret struct {
	Type       os.SecretType
	Private    []byte // Private key (PEM for certs, raw for symmetric)
	Public     []byte // Public key (PEM)
	Cert       []byte // Certificate (PEM)
	SharedWith []string
}

// NewSecretManager creates a new SecretManager.
func NewSecretManager(trustRoot *tee.TrustRoot) (*SecretManager, error) {
	return &SecretManager{
		trustRoot:   trustRoot,
		generated:   make(map[string]*GeneratedSecret),
		userDefined: make(map[string][]byte),
	}, nil
}

// GenerateSecret generates a secret based on its definition.
func (sm *SecretManager) GenerateSecret(ctx context.Context, name string, def *os.Secret) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var secret *GeneratedSecret
	var err error

	switch def.Type {
	case os.SecretTypeSymmetricKey:
		secret, err = sm.generateSymmetricKey(def.Size)
	case os.SecretTypeCertRSA:
		secret, err = sm.generateCertRSA(name, def.Cert)
	case os.SecretTypeCertECDSA:
		secret, err = sm.generateCertECDSA(name, def.Cert)
	case os.SecretTypeCertED25519:
		secret, err = sm.generateCertED25519(name, def.Cert)
	case os.SecretTypePlain:
		if def.UserDefined {
			// Will be set later by user
			return nil
		}
		return fmt.Errorf("plain secrets must be user-defined or have a value")
	default:
		return fmt.Errorf("unknown secret type: %s", def.Type)
	}

	if err != nil {
		return err
	}

	secret.Type = def.Type
	sm.generated[name] = secret

	return nil
}

// GetSecret retrieves a secret by name.
func (sm *SecretManager) GetSecret(ctx context.Context, name string) ([]byte, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Check user-defined first
	if value, ok := sm.userDefined[name]; ok {
		return value, nil
	}

	// Check generated
	if secret, ok := sm.generated[name]; ok {
		// Return private key for certs, raw bytes for symmetric
		if secret.Private != nil {
			return secret.Private, nil
		}
		return nil, fmt.Errorf("secret %s has no value", name)
	}

	return nil, fmt.Errorf("secret not found: %s", name)
}

// GetSecretCert retrieves a certificate secret.
func (sm *SecretManager) GetSecretCert(ctx context.Context, name string) (cert, key []byte, err error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	secret, ok := sm.generated[name]
	if !ok {
		return nil, nil, fmt.Errorf("secret not found: %s", name)
	}

	if secret.Cert == nil {
		return nil, nil, fmt.Errorf("secret %s is not a certificate", name)
	}

	return secret.Cert, secret.Private, nil
}

// SetUserSecret sets a user-defined secret.
func (sm *SecretManager) SetUserSecret(ctx context.Context, name string, value []byte) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.userDefined[name] = value
	return nil
}

// Zero zeros all sensitive data.
func (sm *SecretManager) Zero() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, secret := range sm.generated {
		zeroBytes(secret.Private)
		zeroBytes(secret.Public)
		zeroBytes(secret.Cert)
	}

	for name, value := range sm.userDefined {
		zeroBytes(value)
		delete(sm.userDefined, name)
	}

	sm.generated = make(map[string]*GeneratedSecret)
}

// =============================================================================
// Secret Generation
// =============================================================================

func (sm *SecretManager) generateSymmetricKey(bits uint) (*GeneratedSecret, error) {
	if bits == 0 {
		bits = 256
	}

	key := make([]byte, bits/8)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate random key: %w", err)
	}

	return &GeneratedSecret{
		Private: key,
	}, nil
}

func (sm *SecretManager) generateCertRSA(name string, cfg *os.CertConfig) (*GeneratedSecret, error) {
	// Generate RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate RSA key: %w", err)
	}

	// Generate certificate
	cert, err := sm.generateCertificate(name, cfg, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, err
	}

	// Encode private key
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	// Encode public key
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("marshal public key: %w", err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	return &GeneratedSecret{
		Private: privateKeyPEM,
		Public:  publicKeyPEM,
		Cert:    cert,
	}, nil
}

func (sm *SecretManager) generateCertECDSA(name string, cfg *os.CertConfig) (*GeneratedSecret, error) {
	// Generate ECDSA key pair
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ECDSA key: %w", err)
	}

	// Generate certificate
	cert, err := sm.generateCertificate(name, cfg, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, err
	}

	// Encode private key
	privateKeyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// Encode public key
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("marshal public key: %w", err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	return &GeneratedSecret{
		Private: privateKeyPEM,
		Public:  publicKeyPEM,
		Cert:    cert,
	}, nil
}

func (sm *SecretManager) generateCertED25519(name string, cfg *os.CertConfig) (*GeneratedSecret, error) {
	// Generate Ed25519 key pair
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate Ed25519 key: %w", err)
	}

	// Generate certificate
	cert, err := sm.generateCertificate(name, cfg, publicKey, privateKey)
	if err != nil {
		return nil, err
	}

	// Encode private key
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// Encode public key
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("marshal public key: %w", err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	return &GeneratedSecret{
		Private: privateKeyPEM,
		Public:  publicKeyPEM,
		Cert:    cert,
	}, nil
}

func (sm *SecretManager) generateCertificate(name string, cfg *os.CertConfig, publicKey, privateKey any) ([]byte, error) {
	// Build subject
	subject := pkix.Name{
		CommonName: name,
	}
	if cfg != nil && cfg.Subject.CommonName != "" {
		subject = cfg.Subject
	}

	// Validity
	validityDays := 365
	if cfg != nil && cfg.ValidityDays > 0 {
		validityDays = cfg.ValidityDays
	}

	// Create certificate template
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               subject,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 0, validityDays),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	if cfg != nil {
		if len(cfg.DNSNames) > 0 {
			template.DNSNames = cfg.DNSNames
		}
		if cfg.IsCA {
			template.IsCA = true
			template.KeyUsage |= x509.KeyUsageCertSign
		}
	}

	// Self-sign for now (TODO: support SignedBy)
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, publicKey, privateKey)
	if err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	return certPEM, nil
}

func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
