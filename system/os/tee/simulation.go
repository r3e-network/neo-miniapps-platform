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
	"time"
)

// Simulation mode implementations for development and testing.
// These provide the same interface as hardware TEE but without actual SGX.

// simulationRuntime implements EnclaveRuntime for simulation mode.
type simulationRuntime struct {
	mu        sync.RWMutex
	ready     bool
	sealKey   []byte
	enclaveID string
}

func newSimulationRuntime() EnclaveRuntime {
	// Generate a random sealing key for this session
	sealKey := make([]byte, 32)
	_, _ = rand.Read(sealKey)

	enclaveID := make([]byte, 16)
	_, _ = rand.Read(enclaveID)

	return &simulationRuntime{
		sealKey:   sealKey,
		enclaveID: hex.EncodeToString(enclaveID),
	}
}

func (r *simulationRuntime) Initialize(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ready = true
	return nil
}

func (r *simulationRuntime) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ready = false
	return nil
}

func (r *simulationRuntime) Health(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if !r.ready {
		return ErrEnclaveNotReady
	}
	return nil
}

func (r *simulationRuntime) Mode() EnclaveMode {
	return EnclaveModeSimulation
}

func (r *simulationRuntime) SealData(ctx context.Context, data []byte) ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.ready {
		return nil, ErrEnclaveNotReady
	}

	block, err := aes.NewCipher(r.sealKey)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	sealed := gcm.Seal(nonce, nonce, data, nil)
	return sealed, nil
}

func (r *simulationRuntime) UnsealData(ctx context.Context, sealed []byte) ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.ready {
		return nil, ErrEnclaveNotReady
	}

	block, err := aes.NewCipher(r.sealKey)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(sealed) < nonceSize {
		return nil, fmt.Errorf("sealed data too short")
	}

	nonce, ciphertext := sealed[:nonceSize], sealed[nonceSize:]
	data, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return data, nil
}

// simulationVault implements SecretVault for simulation mode.
type simulationVault struct {
	mu            sync.RWMutex
	secrets       map[string][]byte          // key: service:account:name -> encrypted value
	grants        map[string]*SecretAccessGrant // key: target:account:name -> grant
	encryptionKey []byte
}

func newSimulationVault(key []byte) SecretVault {
	if len(key) == 0 {
		key = make([]byte, 32)
		_, _ = rand.Read(key)
	}
	return &simulationVault{
		secrets:       make(map[string][]byte),
		grants:        make(map[string]*SecretAccessGrant),
		encryptionKey: key,
	}
}

func (v *simulationVault) Initialize(ctx context.Context) error {
	return nil
}

func (v *simulationVault) Shutdown(ctx context.Context) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	// Clear secrets from memory
	for k := range v.secrets {
		delete(v.secrets, k)
	}
	return nil
}

func (v *simulationVault) secretKey(serviceID, accountID, name string) string {
	return fmt.Sprintf("%s:%s:%s", serviceID, accountID, name)
}

func (v *simulationVault) grantKey(targetServiceID, accountID, name string) string {
	return fmt.Sprintf("%s:%s:%s", targetServiceID, accountID, name)
}

func (v *simulationVault) encrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(v.encryptionKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, data, nil), nil
}

func (v *simulationVault) decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(v.encryptionKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("data too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func (v *simulationVault) StoreSecret(ctx context.Context, serviceID, accountID, name string, value []byte) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	encrypted, err := v.encrypt(value)
	if err != nil {
		return fmt.Errorf("encrypt secret: %w", err)
	}

	key := v.secretKey(serviceID, accountID, name)
	v.secrets[key] = encrypted
	return nil
}

func (v *simulationVault) GetSecret(ctx context.Context, serviceID, accountID, name string) ([]byte, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	key := v.secretKey(serviceID, accountID, name)
	encrypted, ok := v.secrets[key]
	if !ok {
		return nil, fmt.Errorf("secret not found: %s", name)
	}

	return v.decrypt(encrypted)
}

func (v *simulationVault) GetSecrets(ctx context.Context, serviceID, accountID string, names []string) (map[string]string, error) {
	result := make(map[string]string)

	for _, name := range names {
		value, err := v.GetSecret(ctx, serviceID, accountID, name)
		if err != nil {
			// Check if there's a grant from another service
			value, err = v.getSecretWithGrant(ctx, serviceID, accountID, name)
			if err != nil {
				return nil, fmt.Errorf("secret %s: %w", name, err)
			}
		}
		result[name] = string(value)
	}

	return result, nil
}

func (v *simulationVault) getSecretWithGrant(ctx context.Context, targetServiceID, accountID, name string) ([]byte, error) {
	grantKey := v.grantKey(targetServiceID, accountID, name)
	grant, ok := v.grants[grantKey]
	if !ok {
		return nil, ErrSecretAccessDenied
	}

	// Check expiration
	if grant.ExpiresAt > 0 && time.Now().Unix() > grant.ExpiresAt {
		return nil, ErrSecretAccessDenied
	}

	// Get secret from owner's namespace
	return v.GetSecret(ctx, grant.OwnerServiceID, accountID, name)
}

func (v *simulationVault) DeleteSecret(ctx context.Context, serviceID, accountID, name string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	key := v.secretKey(serviceID, accountID, name)
	delete(v.secrets, key)
	return nil
}

func (v *simulationVault) ListSecrets(ctx context.Context, serviceID, accountID string) ([]string, error) {
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

func (v *simulationVault) GrantAccess(ctx context.Context, ownerServiceID, targetServiceID, accountID, secretName string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Verify secret exists
	key := v.secretKey(ownerServiceID, accountID, secretName)
	if _, ok := v.secrets[key]; !ok {
		return fmt.Errorf("secret not found")
	}

	grantKey := v.grantKey(targetServiceID, accountID, secretName)
	v.grants[grantKey] = &SecretAccessGrant{
		OwnerServiceID:  ownerServiceID,
		TargetServiceID: targetServiceID,
		AccountID:       accountID,
		SecretName:      secretName,
		GrantedAt:       time.Now().Unix(),
	}

	return nil
}

func (v *simulationVault) RevokeAccess(ctx context.Context, ownerServiceID, targetServiceID, accountID, secretName string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	grantKey := v.grantKey(targetServiceID, accountID, secretName)
	delete(v.grants, grantKey)
	return nil
}

// simulationAttestor implements Attestor for simulation mode.
type simulationAttestor struct {
	enclaveID string
}

func newSimulationAttestor(enclaveID string) Attestor {
	if enclaveID == "" {
		id := make([]byte, 16)
		_, _ = rand.Read(id)
		enclaveID = hex.EncodeToString(id)
	}
	return &simulationAttestor{enclaveID: enclaveID}
}

func (a *simulationAttestor) GenerateReport(ctx context.Context) (*AttestationReport, error) {
	// In simulation mode, generate a placeholder report
	quote := make([]byte, 64)
	_, _ = rand.Read(quote)

	sig := make([]byte, 64)
	_, _ = rand.Read(sig)

	return &AttestationReport{
		EnclaveID: a.enclaveID,
		Quote:     quote,
		MREnclave: "SIMULATION_MRENCLAVE_" + a.enclaveID[:16],
		MRSigner:  "SIMULATION_MRSIGNER_" + a.enclaveID[:16],
		Mode:      EnclaveModeSimulation,
		Timestamp: time.Now(),
		Signature: sig,
	}, nil
}

func (a *simulationAttestor) VerifyReport(ctx context.Context, report *AttestationReport) (bool, error) {
	// In simulation mode, always return true
	if report.Mode != EnclaveModeSimulation {
		return false, fmt.Errorf("cannot verify hardware report in simulation mode")
	}
	return true, nil
}

func (a *simulationAttestor) HashExecution(ctx context.Context, req ExecutionRequest, result *ExecutionResult) (string, error) {
	// Create a deterministic hash of the execution
	data := struct {
		ServiceID  string
		AccountID  string
		Script     string
		EntryPoint string
		Status     ExecutionStatus
		StartedAt  time.Time
	}{
		ServiceID:  req.ServiceID,
		AccountID:  req.AccountID,
		Script:     req.Script,
		EntryPoint: req.EntryPoint,
		Status:     result.Status,
		StartedAt:  result.StartedAt,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(jsonData)
	return hex.EncodeToString(hash[:]), nil
}
