//go:build sgx
// +build sgx

// Package tee provides the Trusted Execution Environment (TEE) engine for confidential computing.
//
// SGX Hardware Mode Implementation
//
// This file implements the SGX hardware mode using CGO to call into the Rust SGX bridge.
// It is only compiled when the "sgx" build tag is set.
//
// Build requirements:
//   - Intel SGX SDK installed
//   - Rust SGX enclave built (libsgx_bridge.so)
//   - SGX hardware or simulation mode
//
// Build command:
//   go build -tags sgx ./...
package tee

/*
#cgo CFLAGS: -I${SRCDIR}/sgx_enclave/bridge
#cgo LDFLAGS: -L${SRCDIR}/sgx_enclave/target/release -lsgx_bridge -ldl -lpthread

#include "sgx_bridge.h"
#include <stdlib.h>
#include <string.h>
*/
import "C"

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
	"unsafe"
)

// =============================================================================
// SGX Hardware Runtime Implementation
// =============================================================================

// sgxHardwareRuntime implements EnclaveRuntime using real SGX hardware.
type sgxHardwareRuntime struct {
	mu          sync.RWMutex
	initialized bool
	enclaveID   [32]byte
	enclavePath string
	debug       bool
}

// NewSGXHardwareRuntimeReal creates a new SGX hardware runtime.
func NewSGXHardwareRuntimeReal(enclavePath string, debug bool) (EnclaveRuntime, error) {
	return &sgxHardwareRuntime{
		enclavePath: enclavePath,
		debug:       debug,
	}, nil
}

func (r *sgxHardwareRuntime) Initialize(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.initialized {
		return nil
	}

	// Convert path to C string
	cPath := C.CString(r.enclavePath)
	defer C.free(unsafe.Pointer(cPath))

	// Initialize enclave
	var enclaveID [32]C.uint8_t
	debugFlag := C.int(0)
	if r.debug {
		debugFlag = 1
	}

	status := C.sgx_bridge_init(cPath, debugFlag, &enclaveID[0])
	if status != C.SGX_BRIDGE_SUCCESS {
		return fmt.Errorf("SGX enclave initialization failed: %d", status)
	}

	// Copy enclave ID
	for i := 0; i < 32; i++ {
		r.enclaveID[i] = byte(enclaveID[i])
	}

	r.initialized = true
	return nil
}

func (r *sgxHardwareRuntime) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.initialized {
		return nil
	}

	status := C.sgx_bridge_destroy()
	if status != C.SGX_BRIDGE_SUCCESS {
		return fmt.Errorf("SGX enclave destruction failed: %d", status)
	}

	r.initialized = false
	return nil
}

func (r *sgxHardwareRuntime) Health(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.initialized {
		return ErrEnclaveNotReady
	}

	status := C.sgx_bridge_health_check()
	if status != C.SGX_BRIDGE_SUCCESS {
		return fmt.Errorf("SGX health check failed: %d", status)
	}

	return nil
}

func (r *sgxHardwareRuntime) Mode() EnclaveMode {
	if C.sgx_bridge_is_hardware_mode() == 1 {
		return EnclaveModeHardware
	}
	return EnclaveModeSimulation
}

func (r *sgxHardwareRuntime) SealData(ctx context.Context, data []byte) ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.initialized {
		return nil, ErrEnclaveNotReady
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("data cannot be empty")
	}

	// Calculate required sealed size
	sealedSize := C.sgx_bridge_calc_sealed_size(C.size_t(len(data)), 0)
	sealed := make([]byte, sealedSize)
	var actualLen C.size_t

	status := C.sgx_bridge_seal_data(
		(*C.uint8_t)(unsafe.Pointer(&data[0])),
		C.size_t(len(data)),
		nil, 0, // No additional data
		(*C.uint8_t)(unsafe.Pointer(&sealed[0])),
		sealedSize,
		&actualLen,
	)

	if status != C.SGX_BRIDGE_SUCCESS {
		return nil, fmt.Errorf("SGX seal failed: %d", status)
	}

	return sealed[:actualLen], nil
}

func (r *sgxHardwareRuntime) UnsealData(ctx context.Context, sealed []byte) ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.initialized {
		return nil, ErrEnclaveNotReady
	}

	if len(sealed) == 0 {
		return nil, fmt.Errorf("sealed data cannot be empty")
	}

	// Allocate buffer for plaintext (sealed data is always larger)
	plaintext := make([]byte, len(sealed))
	var actualLen C.size_t

	status := C.sgx_bridge_unseal_data(
		(*C.uint8_t)(unsafe.Pointer(&sealed[0])),
		C.size_t(len(sealed)),
		(*C.uint8_t)(unsafe.Pointer(&plaintext[0])),
		C.size_t(len(plaintext)),
		&actualLen,
	)

	if status != C.SGX_BRIDGE_SUCCESS {
		return nil, fmt.Errorf("SGX unseal failed: %d", status)
	}

	return plaintext[:actualLen], nil
}

// =============================================================================
// SGX Hardware Vault Implementation
// =============================================================================

// sgxHardwareVault implements SecretVault using SGX sealing.
type sgxHardwareVault struct {
	mu      sync.RWMutex
	runtime *sgxHardwareRuntime
	secrets map[string][]byte // key -> sealed value
	grants  map[string]*SecretAccessGrant
}

// NewSGXHardwareVault creates a new SGX hardware vault.
func NewSGXHardwareVault(runtime *sgxHardwareRuntime) SecretVault {
	return &sgxHardwareVault{
		runtime: runtime,
		secrets: make(map[string][]byte),
		grants:  make(map[string]*SecretAccessGrant),
	}
}

func (v *sgxHardwareVault) Initialize(ctx context.Context) error {
	return nil
}

func (v *sgxHardwareVault) Shutdown(ctx context.Context) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	// Clear secrets from memory
	for k := range v.secrets {
		delete(v.secrets, k)
	}
	return nil
}

func (v *sgxHardwareVault) secretKey(serviceID, accountID, name string) string {
	return fmt.Sprintf("%s:%s:%s", serviceID, accountID, name)
}

func (v *sgxHardwareVault) StoreSecret(ctx context.Context, serviceID, accountID, name string, value []byte) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Seal the secret using SGX
	sealed, err := v.runtime.SealData(ctx, value)
	if err != nil {
		return fmt.Errorf("seal secret: %w", err)
	}

	key := v.secretKey(serviceID, accountID, name)
	v.secrets[key] = sealed
	return nil
}

func (v *sgxHardwareVault) GetSecret(ctx context.Context, serviceID, accountID, name string) ([]byte, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	key := v.secretKey(serviceID, accountID, name)
	sealed, ok := v.secrets[key]
	if !ok {
		return nil, fmt.Errorf("secret not found: %s", name)
	}

	// Unseal the secret using SGX
	return v.runtime.UnsealData(ctx, sealed)
}

func (v *sgxHardwareVault) GetSecrets(ctx context.Context, serviceID, accountID string, names []string) (map[string]string, error) {
	result := make(map[string]string)
	for _, name := range names {
		value, err := v.GetSecret(ctx, serviceID, accountID, name)
		if err != nil {
			return nil, fmt.Errorf("secret %s: %w", name, err)
		}
		result[name] = string(value)
	}
	return result, nil
}

func (v *sgxHardwareVault) DeleteSecret(ctx context.Context, serviceID, accountID, name string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	key := v.secretKey(serviceID, accountID, name)
	delete(v.secrets, key)
	return nil
}

func (v *sgxHardwareVault) ListSecrets(ctx context.Context, serviceID, accountID string) ([]string, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	prefix := fmt.Sprintf("%s:%s:", serviceID, accountID)
	var names []string
	for key := range v.secrets {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			names = append(names, key[len(prefix):])
		}
	}
	return names, nil
}

func (v *sgxHardwareVault) GrantAccess(ctx context.Context, ownerServiceID, targetServiceID, accountID, secretName string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	key := v.secretKey(ownerServiceID, accountID, secretName)
	if _, ok := v.secrets[key]; !ok {
		return fmt.Errorf("secret not found")
	}

	grantKey := fmt.Sprintf("%s:%s:%s", targetServiceID, accountID, secretName)
	v.grants[grantKey] = &SecretAccessGrant{
		OwnerServiceID:  ownerServiceID,
		TargetServiceID: targetServiceID,
		AccountID:       accountID,
		SecretName:      secretName,
		GrantedAt:       time.Now().Unix(),
	}
	return nil
}

func (v *sgxHardwareVault) RevokeAccess(ctx context.Context, ownerServiceID, targetServiceID, accountID, secretName string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	grantKey := fmt.Sprintf("%s:%s:%s", targetServiceID, accountID, secretName)
	delete(v.grants, grantKey)
	return nil
}

// =============================================================================
// SGX Hardware Attestor Implementation
// =============================================================================

// sgxHardwareAttestor implements Attestor using real SGX attestation.
type sgxHardwareAttestor struct {
	runtime *sgxHardwareRuntime
}

// NewSGXHardwareAttestor creates a new SGX hardware attestor.
func NewSGXHardwareAttestor(runtime *sgxHardwareRuntime) Attestor {
	return &sgxHardwareAttestor{runtime: runtime}
}

func (a *sgxHardwareAttestor) GenerateReport(ctx context.Context) (*AttestationReport, error) {
	if !a.runtime.initialized {
		return nil, ErrEnclaveNotReady
	}

	var attestation C.sgx_bridge_attestation_t

	status := C.sgx_bridge_generate_attestation(nil, 0, &attestation)
	if status != C.SGX_BRIDGE_SUCCESS {
		return nil, fmt.Errorf("SGX attestation failed: %d", status)
	}

	// Convert to Go types
	mrEnclave := make([]byte, 32)
	mrSigner := make([]byte, 32)
	for i := 0; i < 32; i++ {
		mrEnclave[i] = byte(attestation.mr_enclave[i])
		mrSigner[i] = byte(attestation.mr_signer[i])
	}

	quote := make([]byte, attestation.quote_len)
	for i := 0; i < int(attestation.quote_len); i++ {
		quote[i] = byte(attestation.quote[i])
	}

	mode := EnclaveModeHardware
	if C.sgx_bridge_is_hardware_mode() == 0 {
		mode = EnclaveModeSimulation
	}

	return &AttestationReport{
		EnclaveID: hex.EncodeToString(a.runtime.enclaveID[:]),
		Quote:     quote,
		MREnclave: hex.EncodeToString(mrEnclave),
		MRSigner:  hex.EncodeToString(mrSigner),
		Mode:      mode,
		Timestamp: time.Now(),
		Signature: quote[:64], // First 64 bytes as signature
	}, nil
}

func (a *sgxHardwareAttestor) VerifyReport(ctx context.Context, report *AttestationReport) (bool, error) {
	if report == nil {
		return false, fmt.Errorf("report is nil")
	}
	if len(report.Quote) == 0 {
		return false, fmt.Errorf("quote is empty")
	}

	// Verify quote structure and signature using SGX bridge
	if len(report.Quote) < 432 { // Minimum SGX quote size
		return false, fmt.Errorf("quote too short: expected at least 432 bytes, got %d", len(report.Quote))
	}

	// Extract and verify quote header
	version := uint16(report.Quote[0]) | uint16(report.Quote[1])<<8
	if version != 2 && version != 3 { // SGX quote version 2 or 3
		return false, fmt.Errorf("unsupported quote version: %d", version)
	}

	// Verify quote signature using SGX bridge
	var valid C.int
	status := C.sgx_bridge_verify_quote(
		(*C.uint8_t)(unsafe.Pointer(&report.Quote[0])),
		C.size_t(len(report.Quote)),
		&valid,
	)

	if status != 0 {
		return false, fmt.Errorf("SGX quote verification failed: status %d", status)
	}

	return valid == 1, nil
}

func (a *sgxHardwareAttestor) HashExecution(ctx context.Context, req ExecutionRequest, result *ExecutionResult) (string, error) {
	data := fmt.Sprintf("%s:%s:%s:%s:%v:%v",
		req.ServiceID, req.AccountID, req.Script, req.EntryPoint,
		result.Status, result.StartedAt.Unix())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:]), nil
}

// =============================================================================
// SGX Hardware Crypto Implementation
// =============================================================================

// sgxHardwareCrypto implements SysCrypto using SGX hardware.
type sgxHardwareCrypto struct {
	runtime *sgxHardwareRuntime
}

// NewSGXHardwareCrypto creates a new SGX hardware crypto implementation.
func NewSGXHardwareCrypto(runtime *sgxHardwareRuntime) SysCrypto {
	return &sgxHardwareCrypto{runtime: runtime}
}

func (c *sgxHardwareCrypto) Hash(algorithm string, data []byte) ([]byte, error) {
	if algorithm != "sha256" && algorithm != "SHA256" {
		return nil, fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	hash := make([]byte, 32)
	status := C.sgx_bridge_sha256(
		(*C.uint8_t)(unsafe.Pointer(&data[0])),
		C.size_t(len(data)),
		(*C.uint8_t)(unsafe.Pointer(&hash[0])),
	)

	if status != C.SGX_BRIDGE_SUCCESS {
		return nil, fmt.Errorf("SGX SHA256 failed: %d", status)
	}

	return hash, nil
}

func (c *sgxHardwareCrypto) Sign(data []byte) ([]byte, error) {
	// Use default key
	keyID := "default"
	cKeyID := C.CString(keyID)
	defer C.free(unsafe.Pointer(cKeyID))

	signature := make([]byte, 64)
	status := C.sgx_bridge_ecdsa_sign(
		cKeyID,
		C.size_t(len(keyID)),
		(*C.uint8_t)(unsafe.Pointer(&data[0])),
		C.size_t(len(data)),
		(*C.uint8_t)(unsafe.Pointer(&signature[0])),
	)

	if status != C.SGX_BRIDGE_SUCCESS {
		return nil, fmt.Errorf("SGX sign failed: %d", status)
	}

	return signature, nil
}

func (c *sgxHardwareCrypto) Verify(data []byte, signature []byte, publicKey []byte) (bool, error) {
	if len(publicKey) != 65 || len(signature) != 64 {
		return false, fmt.Errorf("invalid key or signature length")
	}

	var valid C.int
	status := C.sgx_bridge_ecdsa_verify(
		(*C.uint8_t)(unsafe.Pointer(&publicKey[0])),
		(*C.uint8_t)(unsafe.Pointer(&data[0])),
		C.size_t(len(data)),
		(*C.uint8_t)(unsafe.Pointer(&signature[0])),
		&valid,
	)

	if status != C.SGX_BRIDGE_SUCCESS {
		return false, fmt.Errorf("SGX verify failed: %d", status)
	}

	return valid == 1, nil
}

func (c *sgxHardwareCrypto) Encrypt(keyID string, plaintext []byte) ([]byte, error) {
	// Generate random IV
	iv := make([]byte, 12)
	status := C.sgx_bridge_random_bytes(
		(*C.uint8_t)(unsafe.Pointer(&iv[0])),
		12,
	)
	if status != C.SGX_BRIDGE_SUCCESS {
		return nil, fmt.Errorf("SGX random failed: %d", status)
	}

	// Use keyID as key (in production, would look up key)
	key := sha256.Sum256([]byte(keyID))

	ciphertext := make([]byte, len(plaintext))
	tag := make([]byte, 16)

	status = C.sgx_bridge_aes_gcm_encrypt(
		(*C.uint8_t)(unsafe.Pointer(&key[0])),
		(*C.uint8_t)(unsafe.Pointer(&iv[0])),
		(*C.uint8_t)(unsafe.Pointer(&plaintext[0])),
		C.size_t(len(plaintext)),
		nil, 0,
		(*C.uint8_t)(unsafe.Pointer(&ciphertext[0])),
		(*C.uint8_t)(unsafe.Pointer(&tag[0])),
	)

	if status != C.SGX_BRIDGE_SUCCESS {
		return nil, fmt.Errorf("SGX encrypt failed: %d", status)
	}

	// Return iv || ciphertext || tag
	result := make([]byte, 12+len(ciphertext)+16)
	copy(result[:12], iv)
	copy(result[12:12+len(ciphertext)], ciphertext)
	copy(result[12+len(ciphertext):], tag)

	return result, nil
}

func (c *sgxHardwareCrypto) Decrypt(keyID string, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < 28 { // 12 (iv) + 16 (tag) minimum
		return nil, fmt.Errorf("ciphertext too short")
	}

	iv := ciphertext[:12]
	tag := ciphertext[len(ciphertext)-16:]
	encrypted := ciphertext[12 : len(ciphertext)-16]

	key := sha256.Sum256([]byte(keyID))
	plaintext := make([]byte, len(encrypted))

	status := C.sgx_bridge_aes_gcm_decrypt(
		(*C.uint8_t)(unsafe.Pointer(&key[0])),
		(*C.uint8_t)(unsafe.Pointer(&iv[0])),
		(*C.uint8_t)(unsafe.Pointer(&encrypted[0])),
		C.size_t(len(encrypted)),
		nil, 0,
		(*C.uint8_t)(unsafe.Pointer(&tag[0])),
		(*C.uint8_t)(unsafe.Pointer(&plaintext[0])),
	)

	if status != C.SGX_BRIDGE_SUCCESS {
		return nil, fmt.Errorf("SGX decrypt failed: %d", status)
	}

	return plaintext, nil
}

func (c *sgxHardwareCrypto) GenerateKey(keyType string) (*KeyPair, error) {
	keyID := fmt.Sprintf("key_%d", time.Now().UnixNano())
	cKeyID := C.CString(keyID)
	defer C.free(unsafe.Pointer(cKeyID))

	publicKey := make([]byte, 65)
	status := C.sgx_bridge_generate_ecdsa_keypair(
		cKeyID,
		C.size_t(len(keyID)),
		(*C.uint8_t)(unsafe.Pointer(&publicKey[0])),
	)

	if status != C.SGX_BRIDGE_SUCCESS {
		return nil, fmt.Errorf("SGX key generation failed: %d", status)
	}

	return &KeyPair{
		KeyID:     keyID,
		KeyType:   keyType,
		PublicKey: publicKey,
	}, nil
}

func (c *sgxHardwareCrypto) RandomBytes(length int) ([]byte, error) {
	if length <= 0 {
		return nil, fmt.Errorf("length must be positive")
	}

	buffer := make([]byte, length)
	status := C.sgx_bridge_random_bytes(
		(*C.uint8_t)(unsafe.Pointer(&buffer[0])),
		C.size_t(length),
	)

	if status != C.SGX_BRIDGE_SUCCESS {
		return nil, fmt.Errorf("SGX random failed: %d", status)
	}

	return buffer, nil
}

// =============================================================================
// Factory Functions for Hardware Mode
// =============================================================================

// CreateHardwareEngine creates a TEE engine using real SGX hardware.
func CreateHardwareEngine(enclavePath string, debug bool) (*Engine, error) {
	runtime, err := NewSGXHardwareRuntimeReal(enclavePath, debug)
	if err != nil {
		return nil, err
	}

	hwRuntime := runtime.(*sgxHardwareRuntime)

	return &Engine{
		config:      EngineConfig{Mode: EnclaveModeHardware},
		runtime:     runtime,
		secretVault: NewSGXHardwareVault(hwRuntime),
		attestor:    NewSGXHardwareAttestor(hwRuntime),
	}, nil
}
