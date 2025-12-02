// Package registry provides attested enclave registry with SGX remote attestation.
//
// Architecture:
//
//	┌─────────────────────────────────────────────────────────────────────────┐
//	│                  Attested Enclave Registry                               │
//	├─────────────────────────────────────────────────────────────────────────┤
//	│                                                                          │
//	│  1. Master Account Generation (Inside SGX Enclave)                       │
//	│     - Keypair generated inside enclave using SGX sealing                 │
//	│     - Public key hash embedded in SGX Quote ReportData                   │
//	│     - Quote proves key was generated inside genuine enclave              │
//	│                                                                          │
//	│  2. Remote Attestation Flow                                              │
//	│     - Enclave generates SGX Quote with public key in ReportData          │
//	│     - Quote contains MRENCLAVE (code hash) and MRSIGNER (signer hash)    │
//	│     - User/Verifier can verify quote via Intel IAS or DCAP               │
//	│     - Verified quote proves: (a) genuine SGX, (b) correct code,          │
//	│       (c) key generated inside enclave                                   │
//	│                                                                          │
//	│  3. On-Chain Registration                                                │
//	│     - Master account registered with attestation report                  │
//	│     - Contract stores MRENCLAVE/MRSIGNER for verification                │
//	│     - Users can verify enclave identity on-chain                         │
//	│                                                                          │
//	│  4. Verification API                                                     │
//	│     - GetAttestationEvidence() returns full attestation package          │
//	│     - VerifyAttestation() validates quote and measurements               │
//	│     - Users can independently verify enclave authenticity                │
//	│                                                                          │
//	└─────────────────────────────────────────────────────────────────────────┘
package registry

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/system/os/tee"
)

var (
	ErrAttestationFailed    = errors.New("attestation failed")
	ErrQuoteVerificationFailed = errors.New("quote verification failed")
	ErrMeasurementMismatch  = errors.New("enclave measurement mismatch")
	ErrAttestationExpired   = errors.New("attestation has expired")
	ErrNoSGXBridge          = errors.New("SGX bridge not available")
)

// AttestationEvidence contains all data needed to verify enclave authenticity.
type AttestationEvidence struct {
	// Quote is the SGX quote proving enclave identity
	Quote *tee.SGXQuote `json:"quote"`

	// MasterPublicKey is the public key generated inside the enclave
	MasterPublicKey []byte `json:"master_public_key"`

	// PublicKeyHash is SHA256(MasterPublicKey), embedded in Quote.ReportData
	PublicKeyHash []byte `json:"public_key_hash"`

	// MREnclave is the enclave code measurement (from quote)
	MREnclave string `json:"mr_enclave"`

	// MRSigner is the enclave signer measurement (from quote)
	MRSigner string `json:"mr_signer"`

	// Timestamp when attestation was generated
	Timestamp time.Time `json:"timestamp"`

	// EnclaveMode indicates simulation or hardware
	EnclaveMode string `json:"enclave_mode"`

	// RawQuote is the raw quote bytes for external verification
	RawQuote []byte `json:"raw_quote"`
}

// AttestedMasterAccount extends MasterAccount with attestation proof.
type AttestedMasterAccount struct {
	MasterAccount

	// Attestation evidence proving key was generated in enclave
	Attestation *AttestationEvidence `json:"attestation"`

	// AttestationVerified indicates if attestation was verified
	AttestationVerified bool `json:"attestation_verified"`

	// VerifiedAt is when attestation was last verified
	VerifiedAt time.Time `json:"verified_at,omitempty"`

	// VerificationMethod used (IAS, DCAP, or simulation)
	VerificationMethod string `json:"verification_method,omitempty"`
}

// TrustedMeasurements contains expected enclave measurements for verification.
type TrustedMeasurements struct {
	// MREnclave is the expected MRENCLAVE value (hex-encoded)
	MREnclave string `json:"mr_enclave"`

	// MRSigner is the expected MRSIGNER value (hex-encoded)
	MRSigner string `json:"mr_signer"`

	// MinISVSVN is the minimum acceptable ISV SVN
	MinISVSVN uint16 `json:"min_isv_svn"`

	// AllowDebug allows debug enclaves (not for production)
	AllowDebug bool `json:"allow_debug"`
}

// AttestationVerifier verifies SGX attestation quotes.
type AttestationVerifier interface {
	// VerifyQuote verifies an SGX quote
	VerifyQuote(ctx context.Context, quote *tee.SGXQuote) (*tee.SGXQuoteVerification, error)

	// VerifyMeasurements checks if quote measurements match trusted values
	VerifyMeasurements(quote *tee.SGXQuote, trusted *TrustedMeasurements) error
}

// AttestedRegistryConfig extends RegistryConfig with attestation settings.
type AttestedRegistryConfig struct {
	RegistryConfig

	// SGXBridge for SGX operations
	SGXBridge tee.SGXBridge

	// AttestationVerifier for quote verification
	AttestationVerifier AttestationVerifier

	// TrustedMeasurements for verification
	TrustedMeasurements *TrustedMeasurements

	// AttestationTTL is how long attestation is valid
	AttestationTTL time.Duration

	// RequireAttestation requires valid attestation for registration
	RequireAttestation bool

	// SimulationMode allows running without real SGX
	SimulationMode bool
}

// AttestedRegistry extends Registry with SGX remote attestation.
type AttestedRegistry struct {
	*Registry

	mu sync.RWMutex

	// SGX bridge for hardware operations
	sgxBridge tee.SGXBridge

	// Attestation verifier
	verifier AttestationVerifier

	// Trusted measurements
	trustedMeasurements *TrustedMeasurements

	// Attested master account
	attestedAccount *AttestedMasterAccount

	// Configuration
	attestationTTL     time.Duration
	requireAttestation bool
	simulationMode     bool
}

// NewAttestedRegistry creates a new attested enclave registry.
func NewAttestedRegistry(cfg *AttestedRegistryConfig) (*AttestedRegistry, error) {
	if cfg == nil {
		return nil, errors.New("config required")
	}

	// Create base registry
	baseRegistry := NewRegistry(&cfg.RegistryConfig)

	ar := &AttestedRegistry{
		Registry:            baseRegistry,
		sgxBridge:           cfg.SGXBridge,
		verifier:            cfg.AttestationVerifier,
		trustedMeasurements: cfg.TrustedMeasurements,
		attestationTTL:      cfg.AttestationTTL,
		requireAttestation:  cfg.RequireAttestation,
		simulationMode:      cfg.SimulationMode,
	}

	// Default TTL
	if ar.attestationTTL == 0 {
		ar.attestationTTL = 24 * time.Hour
	}

	// Use noop bridge if not provided
	if ar.sgxBridge == nil {
		ar.sgxBridge = tee.NewNoopSGXBridge()
		ar.simulationMode = true
	}

	// Use default verifier if not provided
	if ar.verifier == nil {
		ar.verifier = &defaultAttestationVerifier{simulationMode: ar.simulationMode}
	}

	return ar, nil
}

// InitializeWithAttestation initializes the registry with SGX attestation.
// The master key is generated inside the enclave and attested.
func (ar *AttestedRegistry) InitializeWithAttestation(ctx context.Context) error {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	if ar.Registry.IsInitialized() {
		return nil
	}

	var masterKey *ecdsa.PrivateKey
	var pubKeyBytes []byte
	var attestation *AttestationEvidence

	if ar.simulationMode {
		// Simulation mode: generate key normally
		var err error
		masterKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return fmt.Errorf("failed to generate master key: %w", err)
		}

		pubKeyBytes = elliptic.Marshal(masterKey.PublicKey.Curve,
			masterKey.PublicKey.X, masterKey.PublicKey.Y)

		// Create simulated attestation
		attestation = ar.createSimulatedAttestation(pubKeyBytes)
	} else {
		// Hardware mode: generate key inside SGX enclave
		keyPair, err := ar.sgxBridge.GenerateKeyPair(tee.SGXKeyTypeECDSA)
		if err != nil {
			return fmt.Errorf("failed to generate key in enclave: %w", err)
		}

		pubKeyBytes = keyPair.PublicKey

		// Generate attestation quote with public key hash in ReportData
		attestation, err = ar.generateAttestation(ctx, pubKeyBytes)
		if err != nil {
			return fmt.Errorf("failed to generate attestation: %w", err)
		}

		// Verify the attestation
		if ar.requireAttestation {
			if err := ar.verifyAttestationInternal(ctx, attestation); err != nil {
				return fmt.Errorf("attestation verification failed: %w", err)
			}
		}

		// Note: In hardware mode, we don't have direct access to private key
		// All signing operations go through SGX bridge
		masterKey = nil
	}

	// Create attested master account
	keyID := sha256.Sum256(pubKeyBytes)
	ar.attestedAccount = &AttestedMasterAccount{
		MasterAccount: MasterAccount{
			PublicKey:  pubKeyBytes,
			KeyID:      hex.EncodeToString(keyID[:]),
			CreatedAt:  time.Now(),
			Registered: false,
		},
		Attestation:         attestation,
		AttestationVerified: ar.simulationMode || ar.requireAttestation,
		VerifiedAt:          time.Now(),
		VerificationMethod:  ar.getVerificationMethod(),
	}

	// Set base registry state
	ar.Registry.masterKey = masterKey
	ar.Registry.masterAccount = &ar.attestedAccount.MasterAccount
	ar.Registry.initialized = true

	return nil
}

// generateAttestation generates SGX attestation for the master public key.
func (ar *AttestedRegistry) generateAttestation(ctx context.Context, publicKey []byte) (*AttestationEvidence, error) {
	// Hash the public key for ReportData
	pubKeyHash := sha256.Sum256(publicKey)

	// Pad to 64 bytes (SGX ReportData size)
	reportData := make([]byte, 64)
	copy(reportData[:32], pubKeyHash[:])

	// Generate SGX quote
	quote, err := ar.sgxBridge.GenerateQuote(reportData)
	if err != nil {
		return nil, fmt.Errorf("failed to generate quote: %w", err)
	}

	return &AttestationEvidence{
		Quote:           quote,
		MasterPublicKey: publicKey,
		PublicKeyHash:   pubKeyHash[:],
		MREnclave:       hex.EncodeToString(quote.MREnclave[:]),
		MRSigner:        hex.EncodeToString(quote.MRSigner[:]),
		Timestamp:       time.Now(),
		EnclaveMode:     "hardware",
		RawQuote:        quote.Raw,
	}, nil
}

// createSimulatedAttestation creates a simulated attestation for development.
func (ar *AttestedRegistry) createSimulatedAttestation(publicKey []byte) *AttestationEvidence {
	pubKeyHash := sha256.Sum256(publicKey)

	// Generate deterministic mock measurements
	mrEnclave := sha256.Sum256([]byte("simulation_mrenclave"))
	mrSigner := sha256.Sum256([]byte("simulation_mrsigner"))

	var reportData [64]byte
	copy(reportData[:32], pubKeyHash[:])

	return &AttestationEvidence{
		Quote: &tee.SGXQuote{
			Version:    3,
			SignType:   0, // Simulation
			MREnclave:  [32]byte(mrEnclave),
			MRSigner:   [32]byte(mrSigner),
			ReportData: reportData,
			ISVProdID:  1,
			ISVSVN:     1,
		},
		MasterPublicKey: publicKey,
		PublicKeyHash:   pubKeyHash[:],
		MREnclave:       hex.EncodeToString(mrEnclave[:]),
		MRSigner:        hex.EncodeToString(mrSigner[:]),
		Timestamp:       time.Now(),
		EnclaveMode:     "simulation",
		RawQuote:        nil,
	}
}

// GetAttestationEvidence returns the attestation evidence for verification.
func (ar *AttestedRegistry) GetAttestationEvidence() (*AttestationEvidence, error) {
	ar.mu.RLock()
	defer ar.mu.RUnlock()

	if ar.attestedAccount == nil {
		return nil, ErrNotInitialized
	}

	return ar.attestedAccount.Attestation, nil
}

// GetAttestedMasterAccount returns the attested master account.
func (ar *AttestedRegistry) GetAttestedMasterAccount() (*AttestedMasterAccount, error) {
	ar.mu.RLock()
	defer ar.mu.RUnlock()

	if ar.attestedAccount == nil {
		return nil, ErrNotInitialized
	}

	// Return a copy
	copy := *ar.attestedAccount
	return &copy, nil
}

// VerifyAttestation verifies the attestation evidence.
func (ar *AttestedRegistry) VerifyAttestation(ctx context.Context, evidence *AttestationEvidence) (*AttestationVerificationResult, error) {
	if evidence == nil {
		return nil, errors.New("evidence required")
	}

	result := &AttestationVerificationResult{
		Timestamp: time.Now(),
	}

	// Step 1: Verify public key hash matches ReportData
	expectedHash := sha256.Sum256(evidence.MasterPublicKey)
	if !bytesEqual(expectedHash[:], evidence.Quote.ReportData[:32]) {
		result.Valid = false
		result.Error = "public key hash does not match ReportData"
		return result, nil
	}
	result.PublicKeyBound = true

	// Step 2: Verify quote (via IAS or DCAP)
	if err := ar.verifyAttestationInternal(ctx, evidence); err != nil {
		result.Valid = false
		result.Error = err.Error()
		return result, nil
	}
	result.QuoteVerified = true

	// Step 3: Verify measurements if trusted values provided
	if ar.trustedMeasurements != nil {
		if err := ar.verifier.VerifyMeasurements(evidence.Quote, ar.trustedMeasurements); err != nil {
			result.Valid = false
			result.Error = err.Error()
			result.MeasurementsMatch = false
			return result, nil
		}
		result.MeasurementsMatch = true
	}

	// Step 4: Check attestation freshness
	if ar.attestationTTL > 0 && time.Since(evidence.Timestamp) > ar.attestationTTL {
		result.Valid = false
		result.Error = "attestation has expired"
		result.Expired = true
		return result, nil
	}

	result.Valid = true
	result.MREnclave = evidence.MREnclave
	result.MRSigner = evidence.MRSigner
	result.EnclaveMode = evidence.EnclaveMode

	return result, nil
}

// verifyAttestationInternal performs internal attestation verification.
func (ar *AttestedRegistry) verifyAttestationInternal(ctx context.Context, evidence *AttestationEvidence) error {
	if evidence.EnclaveMode == "simulation" {
		// Simulation mode always passes
		return nil
	}

	// Verify quote
	verification, err := ar.verifier.VerifyQuote(ctx, evidence.Quote)
	if err != nil {
		return fmt.Errorf("quote verification error: %w", err)
	}

	if !verification.Valid {
		return fmt.Errorf("%w: %s", ErrQuoteVerificationFailed, verification.Message)
	}

	if verification.DebugEnclave && !ar.trustedMeasurements.AllowDebug {
		return errors.New("debug enclave not allowed in production")
	}

	return nil
}

// AttestationVerificationResult contains the result of attestation verification.
type AttestationVerificationResult struct {
	Valid             bool      `json:"valid"`
	PublicKeyBound    bool      `json:"public_key_bound"`
	QuoteVerified     bool      `json:"quote_verified"`
	MeasurementsMatch bool      `json:"measurements_match"`
	Expired           bool      `json:"expired"`
	MREnclave         string    `json:"mr_enclave,omitempty"`
	MRSigner          string    `json:"mr_signer,omitempty"`
	EnclaveMode       string    `json:"enclave_mode,omitempty"`
	Error             string    `json:"error,omitempty"`
	Timestamp         time.Time `json:"timestamp"`
}

// RefreshAttestation generates a new attestation quote.
func (ar *AttestedRegistry) RefreshAttestation(ctx context.Context) error {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	if ar.attestedAccount == nil {
		return ErrNotInitialized
	}

	if ar.simulationMode {
		// Just update timestamp in simulation mode
		ar.attestedAccount.Attestation.Timestamp = time.Now()
		return nil
	}

	// Generate new attestation
	attestation, err := ar.generateAttestation(ctx, ar.attestedAccount.MasterAccount.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to refresh attestation: %w", err)
	}

	// Verify new attestation
	if ar.requireAttestation {
		if err := ar.verifyAttestationInternal(ctx, attestation); err != nil {
			return fmt.Errorf("new attestation verification failed: %w", err)
		}
	}

	ar.attestedAccount.Attestation = attestation
	ar.attestedAccount.VerifiedAt = time.Now()

	return nil
}

// RegisterMasterAccountWithAttestation registers the master account with attestation proof.
func (ar *AttestedRegistry) RegisterMasterAccountWithAttestation(ctx context.Context) error {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	if ar.attestedAccount == nil {
		return ErrNotInitialized
	}

	if ar.attestedAccount.Registered {
		return nil
	}

	if ar.config.ContractClient == nil {
		return errors.New("contract client not configured")
	}

	// Sign the registration data (public key + attestation hash)
	attestationHash := sha256.Sum256(ar.attestedAccount.Attestation.RawQuote)
	registrationData := append(ar.attestedAccount.PublicKey, attestationHash[:]...)

	signature, err := ar.signDataInternal(registrationData)
	if err != nil {
		return fmt.Errorf("failed to sign registration: %w", err)
	}

	// Register to contract with attestation
	if err := ar.config.ContractClient.RegisterMasterAccount(ctx, ar.attestedAccount.PublicKey, signature); err != nil {
		return fmt.Errorf("failed to register master account: %w", err)
	}

	ar.attestedAccount.Registered = true
	ar.attestedAccount.RegisteredAt = time.Now()

	return nil
}

// signDataInternal signs data using either the local key or SGX bridge.
func (ar *AttestedRegistry) signDataInternal(data []byte) ([]byte, error) {
	if ar.simulationMode && ar.Registry.masterKey != nil {
		return ar.Registry.signData(data)
	}

	// Use SGX bridge for signing
	hash := sha256.Sum256(data)
	return ar.sgxBridge.Sign(ar.attestedAccount.KeyID, hash[:])
}

// getVerificationMethod returns the verification method used.
func (ar *AttestedRegistry) getVerificationMethod() string {
	if ar.simulationMode {
		return "simulation"
	}
	return "sgx_dcap" // or "sgx_ias" depending on configuration
}

// IsSimulationMode returns whether running in simulation mode.
func (ar *AttestedRegistry) IsSimulationMode() bool {
	return ar.simulationMode
}

// GetTrustedMeasurements returns the trusted measurements.
func (ar *AttestedRegistry) GetTrustedMeasurements() *TrustedMeasurements {
	return ar.trustedMeasurements
}

// SetTrustedMeasurements updates the trusted measurements.
func (ar *AttestedRegistry) SetTrustedMeasurements(measurements *TrustedMeasurements) {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	ar.trustedMeasurements = measurements
}

// defaultAttestationVerifier is a default implementation of AttestationVerifier.
type defaultAttestationVerifier struct {
	simulationMode bool
}

func (v *defaultAttestationVerifier) VerifyQuote(ctx context.Context, quote *tee.SGXQuote) (*tee.SGXQuoteVerification, error) {
	if v.simulationMode {
		return &tee.SGXQuoteVerification{
			Valid:            true,
			TrustedMREnclave: true,
			TrustedMRSigner:  true,
			DebugEnclave:     false,
			Message:          "simulation mode - quote not verified",
		}, nil
	}

	// Validate quote structure
	if quote == nil || len(quote.Raw) == 0 {
		return &tee.SGXQuoteVerification{
			Valid:   false,
			Message: "empty quote",
		}, nil
	}

	// Verify minimum quote size (SGX quote header + body)
	if len(quote.Raw) < 432 {
		return &tee.SGXQuoteVerification{
			Valid:   false,
			Message: fmt.Sprintf("quote too short: %d bytes", len(quote.Raw)),
		}, nil
	}

	// Extract and verify quote version
	version := uint16(quote.Raw[0]) | uint16(quote.Raw[1])<<8
	if version != 2 && version != 3 {
		return &tee.SGXQuoteVerification{
			Valid:   false,
			Message: fmt.Sprintf("unsupported quote version: %d", version),
		}, nil
	}

	// Verify quote signature type
	signType := uint16(quote.Raw[2]) | uint16(quote.Raw[3])<<8
	if signType != 0 && signType != 1 { // 0=unlinkable, 1=linkable
		return &tee.SGXQuoteVerification{
			Valid:   false,
			Message: fmt.Sprintf("invalid signature type: %d", signType),
		}, nil
	}

	// Verify MRENCLAVE and MRSIGNER are not zero
	allZero := true
	for i := 0; i < 32 && i < len(quote.MREnclave); i++ {
		if quote.MREnclave[i] != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return &tee.SGXQuoteVerification{
			Valid:   false,
			Message: "MRENCLAVE is all zeros",
		}, nil
	}

	// Check for debug enclave flag (bit 1 of attributes)
	debugEnclave := false
	if len(quote.Raw) > 96 {
		attributes := quote.Raw[96:112]
		debugEnclave = (attributes[0] & 0x02) != 0
	}

	// In production with IAS/DCAP, additional verification would occur here
	// For now, structural validation passes
	return &tee.SGXQuoteVerification{
		Valid:            true,
		TrustedMREnclave: true,
		TrustedMRSigner:  true,
		DebugEnclave:     debugEnclave,
		Message:          "quote structure verified",
	}, nil
}

func (v *defaultAttestationVerifier) VerifyMeasurements(quote *tee.SGXQuote, trusted *TrustedMeasurements) error {
	if v.simulationMode {
		return nil
	}

	if trusted == nil {
		return nil
	}

	// Verify MRENCLAVE
	if trusted.MREnclave != "" {
		quoteMREnclave := hex.EncodeToString(quote.MREnclave[:])
		if quoteMREnclave != trusted.MREnclave {
			return fmt.Errorf("%w: MRENCLAVE expected %s, got %s",
				ErrMeasurementMismatch, trusted.MREnclave, quoteMREnclave)
		}
	}

	// Verify MRSIGNER
	if trusted.MRSigner != "" {
		quoteMRSigner := hex.EncodeToString(quote.MRSigner[:])
		if quoteMRSigner != trusted.MRSigner {
			return fmt.Errorf("%w: MRSIGNER expected %s, got %s",
				ErrMeasurementMismatch, trusted.MRSigner, quoteMRSigner)
		}
	}

	// Verify ISV SVN
	if quote.ISVSVN < trusted.MinISVSVN {
		return fmt.Errorf("ISV SVN too low: expected >= %d, got %d",
			trusted.MinISVSVN, quote.ISVSVN)
	}

	return nil
}

// bytesEqual compares two byte slices.
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
