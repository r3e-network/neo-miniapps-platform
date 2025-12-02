// Package sdk provides the Enclave SDK implementation.
package sdk

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"sync"
	"time"
)

// attestationProviderImpl implements AttestationProvider interface.
type attestationProviderImpl struct {
	mu         sync.RWMutex
	enclaveID  string
	signingKey *ecdsa.PrivateKey
	mrEnclave  []byte
	mrSigner   []byte
	productID  uint16
	securityVer uint16
	debug      bool
}

// NewAttestationProvider creates a new attestation provider instance.
func NewAttestationProvider(enclaveID string) (AttestationProvider, error) {
	// Generate signing key for attestation
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	// Generate mock MRENCLAVE and MRSIGNER (in production, these come from SGX)
	mrEnclave := make([]byte, 32)
	mrSigner := make([]byte, 32)
	rand.Read(mrEnclave)
	rand.Read(mrSigner)

	return &attestationProviderImpl{
		enclaveID:   enclaveID,
		signingKey:  privateKey,
		mrEnclave:   mrEnclave,
		mrSigner:    mrSigner,
		productID:   1,
		securityVer: 1,
		debug:       false,
	}, nil
}

func (p *attestationProviderImpl) GenerateReport(ctx context.Context, userData []byte) (*AttestationReport, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Create report data
	reportData := sha256.New()
	reportData.Write([]byte(p.enclaveID))
	reportData.Write(p.mrEnclave)
	reportData.Write(p.mrSigner)
	reportData.Write(userData)
	reportHash := reportData.Sum(nil)

	// Sign the report
	r, s, err := ecdsa.Sign(rand.Reader, p.signingKey, reportHash)
	if err != nil {
		return nil, err
	}
	signature := append(r.Bytes(), s.Bytes()...)

	// Get public key
	publicKey := elliptic.Marshal(p.signingKey.PublicKey.Curve,
		p.signingKey.PublicKey.X, p.signingKey.PublicKey.Y)

	return &AttestationReport{
		EnclaveID:   p.enclaveID,
		ReportData:  reportHash,
		Signature:   signature,
		PublicKey:   publicKey,
		Timestamp:   time.Now(),
		ProductID:   p.productID,
		SecurityVer: p.securityVer,
	}, nil
}

func (p *attestationProviderImpl) VerifyReport(ctx context.Context, report *AttestationReport) (bool, error) {
	if report == nil {
		return false, ErrAttestationFailed
	}

	// Parse public key
	x, y := elliptic.Unmarshal(elliptic.P256(), report.PublicKey)
	if x == nil {
		return false, ErrInvalidKey
	}

	pubKey := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     y,
	}

	// Verify signature
	if len(report.Signature) < 64 {
		return false, ErrInvalidSignature
	}

	r := new(big.Int).SetBytes(report.Signature[:32])
	s := new(big.Int).SetBytes(report.Signature[32:64])

	return ecdsa.Verify(pubKey, report.ReportData, r, s), nil
}

func (p *attestationProviderImpl) GetEnclaveInfo(ctx context.Context) (*EnclaveInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return &EnclaveInfo{
		EnclaveID:   p.enclaveID,
		Version:     "1.0.0",
		MREnclave:   p.mrEnclave,
		MRSigner:    p.mrSigner,
		ProductID:   p.productID,
		SecurityVer: p.securityVer,
		Debug:       p.debug,
	}, nil
}

func (p *attestationProviderImpl) GetQuote(ctx context.Context, reportData []byte) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Generate quote (in production, this would call SGX SDK)
	quote := sha256.New()
	quote.Write([]byte("SGX_QUOTE_V3"))
	quote.Write(p.mrEnclave)
	quote.Write(p.mrSigner)
	quote.Write(reportData)

	// Sign the quote
	quoteHash := quote.Sum(nil)
	r, s, err := ecdsa.Sign(rand.Reader, p.signingKey, quoteHash)
	if err != nil {
		return nil, err
	}

	// Construct quote structure
	// Format: version(2) + mrenclave(32) + mrsigner(32) + reportdata(32) + signature(64)
	quoteBytes := make([]byte, 0, 162)
	quoteBytes = append(quoteBytes, 0x03, 0x00) // Version 3
	quoteBytes = append(quoteBytes, p.mrEnclave...)
	quoteBytes = append(quoteBytes, p.mrSigner...)
	quoteBytes = append(quoteBytes, quoteHash...)
	quoteBytes = append(quoteBytes, r.Bytes()...)
	quoteBytes = append(quoteBytes, s.Bytes()...)

	return quoteBytes, nil
}

// GetEnclaveIDFromMeasurement generates an enclave ID from measurements.
func GetEnclaveIDFromMeasurement(mrEnclave, mrSigner []byte) string {
	h := sha256.New()
	h.Write(mrEnclave)
	h.Write(mrSigner)
	return "enclave_" + hex.EncodeToString(h.Sum(nil))[:16]
}
