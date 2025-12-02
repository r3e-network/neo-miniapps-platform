// Package tee provides the Trusted Execution Environment (TEE) engine for confidential computing.
//
// sys.storage - Sealed Persistent Storage
//
// This file implements sealed storage for the TEE enclave. Data is encrypted using
// AES-256-GCM with keys derived from the enclave's sealing key. In simulation mode,
// a deterministic key is used for testing purposes.
//
// Architecture:
//
//	┌─────────────────────────────────────────────────────────────────────────┐
//	│                         Enclave (Trusted)                                │
//	│  ┌─────────────────────────────────────────────────────────────────────┐ │
//	│  │  sys.storage.set(key, value)                                         │ │
//	│  │    → Encrypt with sealing key                                        │ │
//	│  │    → OCALL to persist encrypted data                                 │ │
//	│  │                                                                       │ │
//	│  │  sys.storage.get(key)                                                │ │
//	│  │    → OCALL to retrieve encrypted data                                │ │
//	│  │    → Decrypt with sealing key                                        │ │
//	│  └─────────────────────────────────────────────────────────────────────┘ │
//	└─────────────────────────────────────────────────────────────────────────┘
//	                                 │
//	                                 │ OCALL (encrypted data only)
//	                                 ▼
//	┌─────────────────────────────────────────────────────────────────────────┐
//	│                      Go Service Engine (Untrusted)                       │
//	│  Persistent storage (file system, database, etc.)                        │
//	│  Only sees encrypted blobs - cannot decrypt without sealing key          │
//	└─────────────────────────────────────────────────────────────────────────┘
package tee

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// SealedStorageConfig configures the sealed storage implementation.
type SealedStorageConfig struct {
	// EnclaveID uniquely identifies this enclave for key derivation.
	EnclaveID string

	// SealingKey is the master key for sealing data. In real SGX, this comes
	// from EGETKEY. In simulation mode, it's derived from EnclaveID.
	SealingKey []byte

	// OCALLHandler handles persistence operations outside the enclave.
	OCALLHandler OCALLHandler

	// MaxValueSize limits the size of stored values (default: 1MB).
	MaxValueSize int

	// MaxKeyLength limits the length of storage keys (default: 256).
	MaxKeyLength int
}

// DefaultSealedStorageConfig returns a default configuration.
func DefaultSealedStorageConfig(enclaveID string, handler OCALLHandler) SealedStorageConfig {
	return SealedStorageConfig{
		EnclaveID:    enclaveID,
		OCALLHandler: handler,
		MaxValueSize: 1 << 20, // 1MB
		MaxKeyLength: 256,
	}
}

// sealedStorageImpl implements SysStorage with real encryption.
type sealedStorageImpl struct {
	mu           sync.RWMutex
	config       SealedStorageConfig
	sealingKey   []byte
	gcm          cipher.AEAD
	localCache   map[string][]byte // In-memory cache for simulation mode
	useLocalOnly bool              // True when no OCALL handler is available
}

// NewSealedStorage creates a new sealed storage implementation.
func NewSealedStorage(config SealedStorageConfig) (SysStorage, error) {
	impl := &sealedStorageImpl{
		config:       config,
		localCache:   make(map[string][]byte),
		useLocalOnly: config.OCALLHandler == nil,
	}

	// Derive or use provided sealing key
	if len(config.SealingKey) > 0 {
		impl.sealingKey = config.SealingKey
	} else {
		// In simulation mode, derive key from enclave ID
		impl.sealingKey = impl.deriveSimulationKey(config.EnclaveID)
	}

	// Initialize AES-GCM cipher
	block, err := aes.NewCipher(impl.sealingKey)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}
	impl.gcm = gcm

	return impl, nil
}

// deriveSimulationKey derives a deterministic key for simulation mode.
// In production SGX, this would use EGETKEY with KEYNAME_SEAL.
func (s *sealedStorageImpl) deriveSimulationKey(enclaveID string) []byte {
	// Use HKDF-like derivation for simulation
	h := sha256.New()
	h.Write([]byte("SGX_SEAL_KEY_SIMULATION_V1"))
	h.Write([]byte(enclaveID))
	h.Write([]byte("MRSIGNER")) // Would be actual MRSIGNER in real SGX
	return h.Sum(nil)           // 32 bytes for AES-256
}

// Get retrieves and decrypts a value by key.
func (s *sealedStorageImpl) Get(ctx context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := s.validateKey(key); err != nil {
		return nil, err
	}

	// Derive storage key (hash of user key for privacy)
	storageKey := s.deriveStorageKey(key)

	var encryptedData []byte
	var err error

	if s.useLocalOnly {
		// Use local cache in simulation mode without OCALL handler
		var ok bool
		encryptedData, ok = s.localCache[storageKey]
		if !ok {
			return nil, fmt.Errorf("key not found: %s", key)
		}
	} else {
		// Use OCALL to retrieve from persistent storage
		encryptedData, err = s.ocallGet(ctx, storageKey)
		if err != nil {
			return nil, fmt.Errorf("retrieve data: %w", err)
		}
	}

	// Decrypt the data
	plaintext, err := s.unseal(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("unseal data: %w", err)
	}

	return plaintext, nil
}

// Set encrypts and stores a value.
func (s *sealedStorageImpl) Set(ctx context.Context, key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.validateKey(key); err != nil {
		return err
	}

	if len(value) > s.config.MaxValueSize {
		return fmt.Errorf("value too large: %d bytes (max %d)", len(value), s.config.MaxValueSize)
	}

	// Derive storage key
	storageKey := s.deriveStorageKey(key)

	// Seal (encrypt) the data
	encryptedData, err := s.seal(value)
	if err != nil {
		return fmt.Errorf("seal data: %w", err)
	}

	if s.useLocalOnly {
		// Store in local cache
		s.localCache[storageKey] = encryptedData
		return nil
	}

	// Use OCALL to persist
	return s.ocallSet(ctx, storageKey, encryptedData)
}

// Delete removes a value.
func (s *sealedStorageImpl) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.validateKey(key); err != nil {
		return err
	}

	storageKey := s.deriveStorageKey(key)

	if s.useLocalOnly {
		delete(s.localCache, storageKey)
		return nil
	}

	return s.ocallDelete(ctx, storageKey)
}

// List returns all keys with the given prefix.
func (s *sealedStorageImpl) List(ctx context.Context, prefix string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Note: In a real implementation, we'd need to store key metadata
	// separately since we hash the keys for privacy. For now, we support
	// listing only in local mode where we can iterate the cache.

	if s.useLocalOnly {
		// In local mode, we can't reverse the hash, so we store original keys
		// This is a limitation of the simulation mode
		return nil, fmt.Errorf("list operation requires key metadata store")
	}

	return s.ocallList(ctx, prefix)
}

// =============================================================================
// Sealing Operations
// =============================================================================

// seal encrypts data using AES-256-GCM with the sealing key.
func (s *sealedStorageImpl) seal(plaintext []byte) ([]byte, error) {
	// Generate random nonce
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// Encrypt with authenticated encryption
	ciphertext := s.gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// unseal decrypts data using AES-256-GCM with the sealing key.
func (s *sealedStorageImpl) unseal(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < s.gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}

	// Extract nonce and decrypt
	nonce := ciphertext[:s.gcm.NonceSize()]
	ciphertext = ciphertext[s.gcm.NonceSize():]

	plaintext, err := s.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

// deriveStorageKey hashes the user key for privacy.
func (s *sealedStorageImpl) deriveStorageKey(key string) string {
	h := sha256.New()
	h.Write(s.sealingKey)
	h.Write([]byte(key))
	return hex.EncodeToString(h.Sum(nil))
}

// validateKey checks if a key is valid.
func (s *sealedStorageImpl) validateKey(key string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}
	if len(key) > s.config.MaxKeyLength {
		return fmt.Errorf("key too long: %d chars (max %d)", len(key), s.config.MaxKeyLength)
	}
	return nil
}

// =============================================================================
// OCALL Operations
// =============================================================================

// StorageOCALLRequest represents a storage OCALL request.
type StorageOCALLRequest struct {
	Operation string `json:"operation"` // "get", "set", "delete", "list"
	Key       string `json:"key"`
	Value     []byte `json:"value,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
}

// StorageOCALLResponse represents a storage OCALL response.
type StorageOCALLResponse struct {
	Value []byte   `json:"value,omitempty"`
	Keys  []string `json:"keys,omitempty"`
	Found bool     `json:"found"`
}

func (s *sealedStorageImpl) ocallGet(ctx context.Context, key string) ([]byte, error) {
	req := StorageOCALLRequest{
		Operation: "get",
		Key:       key,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	ocallReq := OCALLRequest{
		Type:      OCALLTypeStorage,
		RequestID: generateRequestID(),
		Payload:   payload,
	}

	resp, err := s.config.OCALLHandler.HandleOCALL(ctx, ocallReq)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("storage OCALL failed: %s", resp.Error)
	}

	var storageResp StorageOCALLResponse
	if err := json.Unmarshal(resp.Payload, &storageResp); err != nil {
		return nil, err
	}

	if !storageResp.Found {
		return nil, fmt.Errorf("key not found")
	}

	return storageResp.Value, nil
}

func (s *sealedStorageImpl) ocallSet(ctx context.Context, key string, value []byte) error {
	req := StorageOCALLRequest{
		Operation: "set",
		Key:       key,
		Value:     value,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return err
	}

	ocallReq := OCALLRequest{
		Type:      OCALLTypeStorage,
		RequestID: generateRequestID(),
		Payload:   payload,
	}

	resp, err := s.config.OCALLHandler.HandleOCALL(ctx, ocallReq)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("storage OCALL failed: %s", resp.Error)
	}

	return nil
}

func (s *sealedStorageImpl) ocallDelete(ctx context.Context, key string) error {
	req := StorageOCALLRequest{
		Operation: "delete",
		Key:       key,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return err
	}

	ocallReq := OCALLRequest{
		Type:      OCALLTypeStorage,
		RequestID: generateRequestID(),
		Payload:   payload,
	}

	resp, err := s.config.OCALLHandler.HandleOCALL(ctx, ocallReq)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("storage OCALL failed: %s", resp.Error)
	}

	return nil
}

func (s *sealedStorageImpl) ocallList(ctx context.Context, prefix string) ([]string, error) {
	req := StorageOCALLRequest{
		Operation: "list",
		Prefix:    prefix,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	ocallReq := OCALLRequest{
		Type:      OCALLTypeStorage,
		RequestID: generateRequestID(),
		Payload:   payload,
	}

	resp, err := s.config.OCALLHandler.HandleOCALL(ctx, ocallReq)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("storage OCALL failed: %s", resp.Error)
	}

	var storageResp StorageOCALLResponse
	if err := json.Unmarshal(resp.Payload, &storageResp); err != nil {
		return nil, err
	}

	return storageResp.Keys, nil
}

// =============================================================================
// Storage Backend for OCALL Handler
// =============================================================================

// StorageBackend is implemented by the untrusted layer to persist sealed data.
type StorageBackend interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]string, error)
}

// MemoryStorageBackend is an in-memory storage backend for testing.
type MemoryStorageBackend struct {
	mu   sync.RWMutex
	data map[string][]byte
}

// NewMemoryStorageBackend creates a new in-memory storage backend.
func NewMemoryStorageBackend() *MemoryStorageBackend {
	return &MemoryStorageBackend{
		data: make(map[string][]byte),
	}
}

func (m *MemoryStorageBackend) Get(ctx context.Context, key string) ([]byte, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.data[key]
	if !ok {
		return nil, false, nil
	}
	// Return a copy to prevent mutation
	result := make([]byte, len(value))
	copy(result, value)
	return result, true, nil
}

func (m *MemoryStorageBackend) Set(ctx context.Context, key string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Store a copy to prevent mutation
	stored := make([]byte, len(value))
	copy(stored, value)
	m.data[key] = stored
	return nil
}

func (m *MemoryStorageBackend) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (m *MemoryStorageBackend) List(ctx context.Context, prefix string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var keys []string
	for k := range m.data {
		// Note: Since keys are hashed, prefix matching won't work as expected
		// This is a limitation - in production, you'd need a separate metadata store
		keys = append(keys, k)
	}
	return keys, nil
}

// HandleStorageOCALL processes storage OCALL requests.
// This should be called by the OCALL handler when it receives a storage request.
func HandleStorageOCALL(ctx context.Context, backend StorageBackend, payload json.RawMessage) (*OCALLResponse, error) {
	var req StorageOCALLRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return &OCALLResponse{
			Success: false,
			Error:   fmt.Sprintf("unmarshal request: %v", err),
		}, nil
	}

	var resp StorageOCALLResponse
	var err error

	switch req.Operation {
	case "get":
		resp.Value, resp.Found, err = backend.Get(ctx, req.Key)
	case "set":
		err = backend.Set(ctx, req.Key, req.Value)
		resp.Found = true
	case "delete":
		err = backend.Delete(ctx, req.Key)
		resp.Found = true
	case "list":
		resp.Keys, err = backend.List(ctx, req.Prefix)
		resp.Found = true
	default:
		return &OCALLResponse{
			Success: false,
			Error:   fmt.Sprintf("unknown operation: %s", req.Operation),
		}, nil
	}

	if err != nil {
		return &OCALLResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	respPayload, err := json.Marshal(resp)
	if err != nil {
		return &OCALLResponse{
			Success: false,
			Error:   fmt.Sprintf("marshal response: %v", err),
		}, nil
	}

	return &OCALLResponse{
		Success: true,
		Payload: respPayload,
	}, nil
}
