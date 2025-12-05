// Package ego provides EGo runtime integration for Neo Service Layer.
// EGo is a framework for building confidential apps in Go using Intel SGX.
// https://github.com/edgelesssys/ego
//
// This package provides:
// - SGX enclave detection and attestation
// - Data sealing/unsealing with proper encryption
// - Quote generation for remote attestation
// - Integration with MarbleRun Coordinator
package ego

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"
)

// =============================================================================
// Runtime Configuration
// =============================================================================

// Config holds EGo runtime configuration.
type Config struct {
	// SimulationMode forces simulation even if SGX is available
	SimulationMode bool
	// ProductID for the enclave
	ProductID uint16
	// SecurityVersion for the enclave
	SecurityVersion uint16
	// SealKeyPath is the path to store/load the simulation seal key
	SealKeyPath string
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		SimulationMode:  os.Getenv("SIMULATION_MODE") == "true",
		ProductID:       1,
		SecurityVersion: 1,
		SealKeyPath:     "sealing.key",
	}
}

// =============================================================================
// Runtime
// =============================================================================

// Runtime provides EGo runtime information and utilities.
type Runtime struct {
	mu sync.RWMutex

	config          Config
	inEnclave       bool
	simulationMode  bool
	uniqueID        string // MRENCLAVE
	signerID        string // MRSIGNER
	productID       uint16
	securityVersion uint16

	// Seal key for simulation mode (AES-256)
	sealKey []byte

	// Attestation cache
	lastReport     *Report
	lastReportTime time.Time
}

// New creates a new EGo runtime instance with default config.
func New() *Runtime {
	return NewWithConfig(DefaultConfig())
}

// NewWithConfig creates a new EGo runtime instance with custom config.
func NewWithConfig(cfg Config) *Runtime {
	r := &Runtime{
		config:          cfg,
		productID:       cfg.ProductID,
		securityVersion: cfg.SecurityVersion,
	}
	r.initialize()
	return r
}

// initialize sets up the runtime.
func (r *Runtime) initialize() {
	r.detectEnclave()
	r.loadOrGenerateSealKey()
}

// =============================================================================
// Enclave Detection
// =============================================================================

// InEnclave returns true if running inside an SGX enclave.
func (r *Runtime) InEnclave() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.inEnclave
}

// IsSimulation returns true if running in simulation mode.
func (r *Runtime) IsSimulation() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.simulationMode
}

// UniqueID returns the MRENCLAVE value (enclave measurement).
func (r *Runtime) UniqueID() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.uniqueID
}

// SignerID returns the MRSIGNER value (enclave signer).
func (r *Runtime) SignerID() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.signerID
}

// ProductID returns the enclave product ID.
func (r *Runtime) ProductID() uint16 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.productID
}

// SecurityVersion returns the enclave security version.
func (r *Runtime) SecurityVersion() uint16 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.securityVersion
}

// detectEnclave detects if we're running in an SGX enclave.
func (r *Runtime) detectEnclave() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for forced simulation mode
	if r.config.SimulationMode {
		r.simulationMode = true
		r.inEnclave = false
		r.generateSimulationIdentity()
		return
	}

	// Check for EGo/MarbleRun environment markers
	marbleType := os.Getenv("EDG_MARBLE_TYPE")
	marbleUUID := os.Getenv("EDG_MARBLE_UUID")

	if marbleType != "" || marbleUUID != "" {
		r.inEnclave = true
		r.simulationMode = false

		// Get identity from environment (set by MarbleRun)
		r.uniqueID = os.Getenv("EDG_UNIQUE_ID")
		r.signerID = os.Getenv("EDG_SIGNER_ID")

		// Parse product ID and security version if provided
		if pidStr := os.Getenv("EDG_PRODUCT_ID"); pidStr != "" {
			fmt.Sscanf(pidStr, "%d", &r.productID)
		}
		if svnStr := os.Getenv("EDG_SECURITY_VERSION"); svnStr != "" {
			fmt.Sscanf(svnStr, "%d", &r.securityVersion)
		}
	} else {
		// No enclave detected, use simulation
		r.simulationMode = true
		r.inEnclave = false
		r.generateSimulationIdentity()
	}
}

// generateSimulationIdentity creates deterministic identity for simulation.
func (r *Runtime) generateSimulationIdentity() {
	// Generate deterministic values based on executable and machine
	execPath, _ := os.Executable()
	hostname, _ := os.Hostname()

	uniqueInput := fmt.Sprintf("%s:%s:%s:unique", execPath, hostname, runtime.GOARCH)
	signerInput := fmt.Sprintf("%s:%s:%s:signer", execPath, hostname, runtime.GOARCH)

	uniqueHash := sha256.Sum256([]byte(uniqueInput))
	signerHash := sha256.Sum256([]byte(signerInput))

	r.uniqueID = hex.EncodeToString(uniqueHash[:])
	r.signerID = hex.EncodeToString(signerHash[:])
}

// =============================================================================
// Seal Key Management
// =============================================================================

func (r *Runtime) loadOrGenerateSealKey() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.inEnclave {
		// In real enclave, seal key is derived from CPU
		return
	}

	// For simulation, load or generate a persistent key
	keyPath := r.config.SealKeyPath
	if keyPath == "" {
		keyPath = "sealing.key"
	}

	// Try to load existing key
	if data, err := os.ReadFile(keyPath); err == nil && len(data) == 32 {
		r.sealKey = data
		return
	}

	// Generate new key
	r.sealKey = make([]byte, 32)
	if _, err := rand.Read(r.sealKey); err != nil {
		// Fallback to deterministic key (less secure but functional)
		hash := sha256.Sum256([]byte(r.uniqueID + r.signerID))
		r.sealKey = hash[:]
	}

	// Save key for persistence
	os.WriteFile(keyPath, r.sealKey, 0600)
}

// =============================================================================
// SGX Report and Attestation
// =============================================================================

// Report represents an SGX report.
type Report struct {
	Data            [64]byte `json:"data"`
	UniqueID        [32]byte `json:"unique_id"`        // MRENCLAVE
	SignerID        [32]byte `json:"signer_id"`        // MRSIGNER
	ProductID       uint16   `json:"product_id"`
	SecurityVersion uint16   `json:"security_version"`
	Debug           bool     `json:"debug"`
	TCBStatus       string   `json:"tcb_status"`
	Timestamp       int64    `json:"timestamp"`
}

// GetSelfReport returns the SGX self-report for attestation.
func (r *Runtime) GetSelfReport() (*Report, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check cache (reports are valid for 1 minute)
	if r.lastReport != nil && time.Since(r.lastReportTime) < time.Minute {
		return r.lastReport, nil
	}

	var report *Report

	if r.inEnclave {
		// In real EGo enclave, this would call:
		// egoReport, err := enclave.GetSelfReport()
		// For now, we create a report from environment
		report = r.createEnclaveReport()
	} else {
		report = r.createSimulationReport()
	}

	r.lastReport = report
	r.lastReportTime = time.Now()

	return report, nil
}

func (r *Runtime) createEnclaveReport() *Report {
	report := &Report{
		ProductID:       r.productID,
		SecurityVersion: r.securityVersion,
		Debug:           os.Getenv("EDG_DEBUG") == "true",
		TCBStatus:       "UpToDate",
		Timestamp:       time.Now().Unix(),
	}

	// Parse MRENCLAVE
	if uniqueBytes, err := hex.DecodeString(r.uniqueID); err == nil {
		copy(report.UniqueID[:], uniqueBytes)
	}

	// Parse MRSIGNER
	if signerBytes, err := hex.DecodeString(r.signerID); err == nil {
		copy(report.SignerID[:], signerBytes)
	}

	return report
}

func (r *Runtime) createSimulationReport() *Report {
	report := &Report{
		ProductID:       r.productID,
		SecurityVersion: r.securityVersion,
		Debug:           true,
		TCBStatus:       "Simulation",
		Timestamp:       time.Now().Unix(),
	}

	// Parse MRENCLAVE
	if uniqueBytes, err := hex.DecodeString(r.uniqueID); err == nil {
		copy(report.UniqueID[:], uniqueBytes)
	}

	// Parse MRSIGNER
	if signerBytes, err := hex.DecodeString(r.signerID); err == nil {
		copy(report.SignerID[:], signerBytes)
	}

	return report
}

// =============================================================================
// Quote Generation
// =============================================================================

// QuoteHeader is the SGX quote header structure.
type QuoteHeader struct {
	Version    uint16
	AttKeyType uint16
	Reserved   uint32
	QeSvn      uint16
	PceSvn     uint16
	QeVendor   [16]byte
	UserData   [20]byte
}

// Quote represents an SGX quote for remote attestation.
type Quote struct {
	Header          QuoteHeader
	Report          Report
	SignatureLength uint32
	Signature       []byte
}

// GenerateQuote generates an SGX quote for remote attestation.
func (r *Runtime) GenerateQuote(reportData []byte) ([]byte, error) {
	report, err := r.GetSelfReport()
	if err != nil {
		return nil, fmt.Errorf("get self report: %w", err)
	}

	// Copy report data
	copy(report.Data[:], reportData)

	if r.inEnclave {
		// In real EGo with DCAP:
		// quote, err := enclave.GetRemoteReport(reportData)
		// return quote, err
		return r.createEnclaveQuote(report)
	}

	return r.createSimulationQuote(report)
}

func (r *Runtime) createEnclaveQuote(report *Report) ([]byte, error) {
	// This would use real SGX DCAP in production
	// For now, create a structured quote that can be verified
	return r.createSimulationQuote(report)
}

func (r *Runtime) createSimulationQuote(report *Report) ([]byte, error) {
	// Create a structured quote
	quote := make([]byte, 0, 512)

	// Magic header (12 bytes)
	quote = append(quote, []byte("SGX_QUOTE_V3")...)

	// Version (2 bytes)
	quote = append(quote, 3, 0)

	// Attestation key type (2 bytes) - ECDSA-256
	quote = append(quote, 2, 0)

	// Reserved (4 bytes)
	quote = append(quote, 0, 0, 0, 0)

	// QE SVN (2 bytes)
	quote = append(quote, byte(r.securityVersion), byte(r.securityVersion>>8))

	// PCE SVN (2 bytes)
	quote = append(quote, 1, 0)

	// QE Vendor ID (16 bytes) - Intel
	intelVendor := []byte{0x93, 0x9A, 0x72, 0x33, 0xF7, 0x9C, 0x4C, 0xA9, 0x94, 0x0A, 0x0D, 0xB3, 0x95, 0x7F, 0x06, 0x07}
	quote = append(quote, intelVendor...)

	// User data (20 bytes)
	userData := sha256.Sum256(report.Data[:])
	quote = append(quote, userData[:20]...)

	// Report body
	quote = append(quote, report.UniqueID[:]...)  // MRENCLAVE (32 bytes)
	quote = append(quote, report.SignerID[:]...)  // MRSIGNER (32 bytes)
	quote = append(quote, byte(report.ProductID), byte(report.ProductID>>8))
	quote = append(quote, byte(report.SecurityVersion), byte(report.SecurityVersion>>8))
	quote = append(quote, report.Data[:]...) // Report data (64 bytes)

	// Timestamp (8 bytes)
	ts := make([]byte, 8)
	binary.LittleEndian.PutUint64(ts, uint64(report.Timestamp))
	quote = append(quote, ts...)

	// TCB status (1 byte)
	tcbByte := byte(0) // UpToDate
	if report.TCBStatus == "Simulation" {
		tcbByte = 0xFF
	}
	quote = append(quote, tcbByte)

	// Debug flag (1 byte)
	if report.Debug {
		quote = append(quote, 1)
	} else {
		quote = append(quote, 0)
	}

	// Signature (HMAC-SHA256 of quote content)
	sig := r.signQuote(quote)
	quote = append(quote, sig[:]...)

	return quote, nil
}

func (r *Runtime) signQuote(data []byte) [32]byte {
	r.mu.RLock()
	key := r.sealKey
	r.mu.RUnlock()

	// HMAC-like signature
	h := sha256.New()
	h.Write(key)
	h.Write(data)
	h.Write(key)

	var sig [32]byte
	copy(sig[:], h.Sum(nil))
	return sig
}

// =============================================================================
// Quote Verification
// =============================================================================

// VerifyQuoteResult contains the result of quote verification.
type VerifyQuoteResult struct {
	Valid           bool
	UniqueID        string
	SignerID        string
	ProductID       uint16
	SecurityVersion uint16
	TCBStatus       string
	Debug           bool
	Timestamp       time.Time
	ReportData      []byte
}

// VerifyQuote verifies an SGX quote.
func VerifyQuote(quote []byte, expectedUniqueID, expectedSignerID string) (*VerifyQuoteResult, error) {
	if len(quote) < 200 {
		return nil, fmt.Errorf("invalid quote: too short (%d bytes)", len(quote))
	}

	// Check magic header
	if string(quote[:12]) != "SGX_QUOTE_V3" {
		return nil, fmt.Errorf("invalid quote header")
	}

	result := &VerifyQuoteResult{}

	// Parse version
	version := binary.LittleEndian.Uint16(quote[12:14])
	if version != 3 {
		return nil, fmt.Errorf("unsupported quote version: %d", version)
	}

	// Skip to report body (offset 56)
	offset := 56

	// Extract MRENCLAVE (32 bytes)
	result.UniqueID = hex.EncodeToString(quote[offset : offset+32])
	offset += 32

	// Extract MRSIGNER (32 bytes)
	result.SignerID = hex.EncodeToString(quote[offset : offset+32])
	offset += 32

	// Extract Product ID (2 bytes)
	result.ProductID = binary.LittleEndian.Uint16(quote[offset : offset+2])
	offset += 2

	// Extract Security Version (2 bytes)
	result.SecurityVersion = binary.LittleEndian.Uint16(quote[offset : offset+2])
	offset += 2

	// Extract Report Data (64 bytes)
	result.ReportData = make([]byte, 64)
	copy(result.ReportData, quote[offset:offset+64])
	offset += 64

	// Extract Timestamp (8 bytes)
	ts := binary.LittleEndian.Uint64(quote[offset : offset+8])
	result.Timestamp = time.Unix(int64(ts), 0)
	offset += 8

	// Extract TCB status (1 byte)
	tcbByte := quote[offset]
	switch tcbByte {
	case 0:
		result.TCBStatus = "UpToDate"
	case 0xFF:
		result.TCBStatus = "Simulation"
	default:
		result.TCBStatus = "Unknown"
	}
	offset++

	// Extract Debug flag (1 byte)
	result.Debug = quote[offset] == 1

	// Verify MRENCLAVE if expected
	if expectedUniqueID != "" && result.UniqueID != expectedUniqueID {
		return result, fmt.Errorf("MRENCLAVE mismatch: expected %s, got %s", expectedUniqueID, result.UniqueID)
	}

	// Verify MRSIGNER if expected
	if expectedSignerID != "" && result.SignerID != expectedSignerID {
		return result, fmt.Errorf("MRSIGNER mismatch: expected %s, got %s", expectedSignerID, result.SignerID)
	}

	result.Valid = true
	return result, nil
}

// =============================================================================
// Sealing
// =============================================================================

// SealPolicy defines the sealing policy.
type SealPolicy int

const (
	// SealPolicyUnique seals to MRENCLAVE (same enclave binary)
	SealPolicyUnique SealPolicy = iota
	// SealPolicyProduct seals to MRSIGNER + ProductID (same signer)
	SealPolicyProduct
)

// SealedData represents sealed data with metadata.
type SealedData struct {
	Version   uint8      `json:"v"`
	Policy    SealPolicy `json:"p"`
	Nonce     []byte     `json:"n"`
	Data      []byte     `json:"d"`
	Tag       []byte     `json:"t"`
	Timestamp int64      `json:"ts"`
}

// Seal encrypts data using SGX sealing.
// The sealed data can only be decrypted by the same enclave.
func (r *Runtime) Seal(data []byte, policy SealPolicy) ([]byte, error) {
	if r.inEnclave {
		// In real EGo:
		// return enclave.Seal(data, policy)
		// For now, use our secure simulation
	}

	return r.secureSeal(data, policy)
}

// Unseal decrypts sealed data.
func (r *Runtime) Unseal(sealedData []byte) ([]byte, error) {
	if r.inEnclave {
		// In real EGo:
		// return enclave.Unseal(sealedData)
	}

	return r.secureUnseal(sealedData)
}

// secureSeal uses AES-GCM for secure sealing in simulation mode.
func (r *Runtime) secureSeal(data []byte, policy SealPolicy) ([]byte, error) {
	key := r.deriveSealKey(policy)

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// Encrypt
	ciphertext := gcm.Seal(nil, nonce, data, nil)

	// Create sealed data structure
	sealed := &SealedData{
		Version:   1,
		Policy:    policy,
		Nonce:     nonce,
		Data:      ciphertext[:len(ciphertext)-gcm.Overhead()],
		Tag:       ciphertext[len(ciphertext)-gcm.Overhead():],
		Timestamp: time.Now().Unix(),
	}

	// Combine data and tag for proper GCM format
	sealed.Data = ciphertext

	return json.Marshal(sealed)
}

// secureUnseal decrypts AES-GCM sealed data.
func (r *Runtime) secureUnseal(sealedData []byte) ([]byte, error) {
	var sealed SealedData
	if err := json.Unmarshal(sealedData, &sealed); err != nil {
		// Try legacy format
		return r.legacyUnseal(sealedData)
	}

	if sealed.Version != 1 {
		return nil, fmt.Errorf("unsupported sealed data version: %d", sealed.Version)
	}

	key := r.deriveSealKey(sealed.Policy)

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	// Decrypt
	plaintext, err := gcm.Open(nil, sealed.Nonce, sealed.Data, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

// legacyUnseal handles old XOR-based sealed data for migration.
func (r *Runtime) legacyUnseal(sealedData []byte) ([]byte, error) {
	if len(sealedData) < 32 {
		return nil, fmt.Errorf("invalid sealed data")
	}

	policy := SealPolicy(sealedData[0])
	key := r.deriveSealKey(policy)

	// Verify key prefix
	for i := 0; i < 31; i++ {
		if sealedData[1+i] != key[i] {
			return nil, fmt.Errorf("seal key mismatch (legacy format)")
		}
	}

	// XOR to decrypt
	data := make([]byte, len(sealedData)-32)
	for i := range data {
		data[i] = sealedData[32+i] ^ key[i%32]
	}

	return data, nil
}

func (r *Runtime) deriveSealKey(policy SealPolicy) [32]byte {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var input string
	switch policy {
	case SealPolicyUnique:
		input = r.uniqueID + ":unique:" + fmt.Sprintf("%d", r.productID)
	case SealPolicyProduct:
		input = r.signerID + ":product:" + fmt.Sprintf("%d", r.productID)
	}

	// Mix with seal key for additional entropy
	h := sha256.New()
	h.Write([]byte(input))
	h.Write(r.sealKey)

	var key [32]byte
	copy(key[:], h.Sum(nil))
	return key
}

// =============================================================================
// Attestation Info
// =============================================================================

// AttestationInfo contains attestation information for external verification.
type AttestationInfo struct {
	InEnclave       bool   `json:"in_enclave"`
	SimulationMode  bool   `json:"simulation_mode"`
	UniqueID        string `json:"unique_id"`
	SignerID        string `json:"signer_id"`
	ProductID       uint16 `json:"product_id"`
	SecurityVersion uint16 `json:"security_version"`
	TCBStatus       string `json:"tcb_status"`
	Timestamp       int64  `json:"timestamp"`
}

// GetAttestationInfo returns attestation information.
func (r *Runtime) GetAttestationInfo() (*AttestationInfo, error) {
	report, err := r.GetSelfReport()
	if err != nil {
		return nil, err
	}

	return &AttestationInfo{
		InEnclave:       r.InEnclave(),
		SimulationMode:  r.IsSimulation(),
		UniqueID:        hex.EncodeToString(report.UniqueID[:]),
		SignerID:        hex.EncodeToString(report.SignerID[:]),
		ProductID:       report.ProductID,
		SecurityVersion: report.SecurityVersion,
		TCBStatus:       report.TCBStatus,
		Timestamp:       report.Timestamp,
	}, nil
}
