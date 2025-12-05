package core

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/manifest"
)

// =============================================================================
// Secret Manager
// =============================================================================

// SecretManager generates and manages secrets for Marbles.
type SecretManager struct {
	mu sync.RWMutex

	simulationMode bool
	sealingKey     []byte

	// Generated secrets
	generated map[string]*GeneratedSecret

	// User-defined secrets
	userDefined map[string][]byte
}

// GeneratedSecret holds a generated secret.
type GeneratedSecret struct {
	Type    manifest.SecretType
	Private []byte
	Public  []byte
	Cert    []byte
}

// NewSecretManager creates a new SecretManager.
func NewSecretManager(simulationMode bool) (*SecretManager, error) {
	sm := &SecretManager{
		simulationMode: simulationMode,
		generated:      make(map[string]*GeneratedSecret),
		userDefined:    make(map[string][]byte),
	}

	// Generate sealing key
	sm.sealingKey = make([]byte, 32)
	if _, err := rand.Read(sm.sealingKey); err != nil {
		return nil, fmt.Errorf("generate sealing key: %w", err)
	}

	return sm, nil
}

// GenerateSecret generates a secret based on its definition.
func (sm *SecretManager) GenerateSecret(ctx context.Context, name string, def *manifest.Secret) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var secret *GeneratedSecret
	var err error

	switch def.Type {
	case manifest.SecretTypeSymmetricKey:
		secret, err = sm.generateSymmetricKey(def.Size)
	case manifest.SecretTypeCertECDSA:
		secret, err = sm.generateCertECDSA(name, def.Cert)
	case manifest.SecretTypeCertRSA:
		secret, err = sm.generateCertRSA(name, def.Cert)
	case manifest.SecretTypePlain:
		if def.UserDefined {
			return nil // Will be set later
		}
		return fmt.Errorf("plain secrets must be user-defined")
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

	if value, ok := sm.userDefined[name]; ok {
		return value, nil
	}

	if secret, ok := sm.generated[name]; ok {
		if secret.Private != nil {
			return secret.Private, nil
		}
	}

	return nil, fmt.Errorf("secret not found: %s", name)
}

// SetUserSecret sets a user-defined secret.
func (sm *SecretManager) SetUserSecret(ctx context.Context, name string, value []byte) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.userDefined[name] = value
	return nil
}

// GetSealingKey returns the sealing key.
func (sm *SecretManager) GetSealingKey() []byte {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	result := make([]byte, len(sm.sealingKey))
	copy(result, sm.sealingKey)
	return result
}

// Zero zeros all sensitive data.
func (sm *SecretManager) Zero() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	zeroBytes(sm.sealingKey)
	for _, s := range sm.generated {
		zeroBytes(s.Private)
	}
	for k, v := range sm.userDefined {
		zeroBytes(v)
		delete(sm.userDefined, k)
	}
}

func (sm *SecretManager) generateSymmetricKey(bits uint) (*GeneratedSecret, error) {
	if bits == 0 {
		bits = 256
	}
	key := make([]byte, bits/8)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return &GeneratedSecret{Private: key}, nil
}

func (sm *SecretManager) generateCertECDSA(name string, cfg *manifest.CertConfig) (*GeneratedSecret, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	cert, err := sm.generateCertificate(name, cfg, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, err
	}

	keyBytes, _ := x509.MarshalECPrivateKey(privateKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	return &GeneratedSecret{
		Private: keyPEM,
		Cert:    cert,
	}, nil
}

func (sm *SecretManager) generateCertRSA(name string, cfg *manifest.CertConfig) (*GeneratedSecret, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	cert, err := sm.generateCertificate(name, cfg, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, err
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	return &GeneratedSecret{
		Private: keyPEM,
		Cert:    cert,
	}, nil
}

func (sm *SecretManager) generateCertificate(name string, cfg *manifest.CertConfig, pub, priv any) ([]byte, error) {
	subject := pkix.Name{CommonName: name}
	validityDays := 365

	if cfg != nil {
		if cfg.Subject.CommonName != "" {
			subject = cfg.Subject
		}
		if cfg.ValidityDays > 0 {
			validityDays = cfg.ValidityDays
		}
	}

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               subject,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 0, validityDays),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	if cfg != nil && len(cfg.DNSNames) > 0 {
		template.DNSNames = cfg.DNSNames
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, pub, priv)
	if err != nil {
		return nil, err
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}), nil
}

// =============================================================================
// TLS Manager
// =============================================================================

// TLSManager handles TLS certificate generation.
type TLSManager struct {
	mu sync.RWMutex

	dnsNames   []string
	rootCACert *x509.Certificate
	rootCAKey  *ecdsa.PrivateKey
	rootCAPEM  []byte

	issued map[string]*IssuedCert
}

// IssuedCert holds an issued certificate.
type IssuedCert struct {
	MarbleName string
	UUID       string
	CertPEM    []byte
	KeyPEM     []byte
}

// NewTLSManager creates a new TLSManager.
func NewTLSManager(dnsNames []string) (*TLSManager, error) {
	tm := &TLSManager{
		dnsNames: dnsNames,
		issued:   make(map[string]*IssuedCert),
	}

	if err := tm.generateRootCA(); err != nil {
		return nil, err
	}

	return tm, nil
}

// Initialize initializes TLS with manifest config.
func (tm *TLSManager) Initialize(ctx context.Context, cfg *manifest.TLSConfig) error {
	// TLS config from manifest can be used for policy enforcement
	return nil
}

// IssueCertificate issues a TLS certificate for a Marble.
func (tm *TLSManager) IssueCertificate(ctx context.Context, marbleName, uuid string) (cert, key, rootCA []byte, err error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName:   marbleName,
			Organization: []string{"Neo Service Layer"},
		},
		DNSNames:              []string{marbleName, uuid, marbleName + ".neo-service-layer.local"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, tm.rootCACert, &privateKey.PublicKey, tm.rootCAKey)
	if err != nil {
		return nil, nil, nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyBytes, _ := x509.MarshalECPrivateKey(privateKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	tm.issued[uuid] = &IssuedCert{
		MarbleName: marbleName,
		UUID:       uuid,
		CertPEM:    certPEM,
		KeyPEM:     keyPEM,
	}

	return certPEM, keyPEM, tm.rootCAPEM, nil
}

// GetRootCA returns the root CA certificate.
func (tm *TLSManager) GetRootCA() []byte {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.rootCAPEM
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

func (tm *TLSManager) generateRootCA() error {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "Neo Service Layer Mesh CA",
			Organization: []string{"Neo Service Layer"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return err
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return err
	}

	tm.rootCACert = cert
	tm.rootCAKey = privateKey
	tm.rootCAPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	return nil
}

// =============================================================================
// Recovery Manager
// =============================================================================

// RecoveryManager handles multi-party key recovery.
type RecoveryManager struct {
	mu sync.Mutex

	threshold    int
	recoveryKeys map[string]*rsa.PublicKey
	keyShares    map[string][]byte
}

// NewRecoveryManager creates a new RecoveryManager.
func NewRecoveryManager(threshold int) *RecoveryManager {
	if threshold < 1 {
		threshold = 1
	}
	return &RecoveryManager{
		threshold:    threshold,
		recoveryKeys: make(map[string]*rsa.PublicKey),
		keyShares:    make(map[string][]byte),
	}
}

// SetRecoveryKeys configures recovery keys.
func (rm *RecoveryManager) SetRecoveryKeys(keys map[string]string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.recoveryKeys = make(map[string]*rsa.PublicKey, len(keys))
	for name, pemKey := range keys {
		block, _ := pem.Decode([]byte(pemKey))
		if block == nil {
			return fmt.Errorf("invalid PEM for %s", name)
		}
		pub, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("parse key %s: %w", name, err)
		}
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return fmt.Errorf("key %s is not RSA", name)
		}
		rm.recoveryKeys[name] = rsaPub
	}

	return nil
}

// GenerateRecoveryData generates encrypted key shares.
func (rm *RecoveryManager) GenerateRecoveryData(sealingKey []byte) (map[string][]byte, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	shares := make(map[string][]byte, len(rm.recoveryKeys))
	for name, pubKey := range rm.recoveryKeys {
		encrypted, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pubKey, sealingKey, nil)
		if err != nil {
			return nil, fmt.Errorf("encrypt for %s: %w", name, err)
		}
		shares[name] = encrypted
	}

	rm.keyShares = shares
	return shares, nil
}

// RecoverSealingKey recovers the sealing key from shares.
func (rm *RecoveryManager) RecoverSealingKey(shares map[string][]byte) ([]byte, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if len(shares) < rm.threshold {
		return nil, fmt.Errorf("insufficient shares: got %d, need %d", len(shares), rm.threshold)
	}

	// All shares should decrypt to the same key
	var sealingKey []byte
	for name, share := range shares {
		if _, ok := rm.recoveryKeys[name]; !ok {
			return nil, fmt.Errorf("unknown key holder: %s", name)
		}
		if sealingKey == nil {
			sealingKey = share
		} else if !bytesEqual(sealingKey, share) {
			return nil, fmt.Errorf("share mismatch from %s", name)
		}
	}

	return sealingKey, nil
}

// HasRecoveryKeys returns true if recovery keys are configured.
func (rm *RecoveryManager) HasRecoveryKeys() bool {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	return len(rm.recoveryKeys) > 0
}

// GetKeyHolders returns the names of recovery key holders.
func (rm *RecoveryManager) GetKeyHolders() []string {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	names := make([]string, 0, len(rm.recoveryKeys))
	for name := range rm.recoveryKeys {
		names = append(names, name)
	}
	return names
}

// =============================================================================
// Quote Manager
// =============================================================================

// QuoteManager handles SGX attestation quotes.
type QuoteManager struct {
	simulationMode bool
}

// NewQuoteManager creates a new QuoteManager.
func NewQuoteManager(simulationMode bool) (*QuoteManager, error) {
	return &QuoteManager{simulationMode: simulationMode}, nil
}

// GenerateQuote generates an attestation quote.
func (qm *QuoteManager) GenerateQuote(ctx context.Context, nonce []byte) ([]byte, error) {
	if qm.simulationMode {
		// Return simulated quote
		return []byte("SIMULATED_QUOTE"), nil
	}
	// TODO: Implement real SGX quote generation
	return nil, fmt.Errorf("hardware mode not implemented")
}

// VerifyQuote verifies a quote against a package definition.
func (qm *QuoteManager) VerifyQuote(quote []byte, pkg *manifest.Package) error {
	if qm.simulationMode || pkg.Debug {
		// Accept any quote in simulation/debug mode
		return nil
	}

	// TODO: Implement real SGX quote verification
	// - Verify MRENCLAVE matches pkg.UniqueID
	// - Verify MRSIGNER matches pkg.SignerID
	// - Verify ProductID matches pkg.ProductID
	// - Verify SecurityVersion >= pkg.SecurityVersion

	return nil
}

// =============================================================================
// Helpers
// =============================================================================

func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
