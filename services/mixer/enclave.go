// Package mixer provides privacy-preserving transaction mixing service.
// This implementation follows a single-node mixer design with TEE as the trust root,
// using 1-of-2 multisig addresses (TEE + Master) for fund custody.
package mixer

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

// MixOutput represents the output of a mix operation.
type MixOutput struct {
	OutputAddress []byte `json:"output_address"`
	Proof         []byte `json:"proof"`
	Commitment    []byte `json:"commitment"`
}

// PendingMix represents a pending mix operation in the enclave.
type PendingMix struct {
	RequestID  string   `json:"request_id"`
	InputHash  []byte   `json:"input_hash"`
	OutputHash []byte   `json:"output_hash"`
	Amount     *big.Int `json:"amount"`
	Status     string   `json:"status"`
}

// Enclave handles all mixer operations within the TEE.
// Critical operations:
// - HD key derivation (keys never leave TEE)
// - Transaction signing (inside TEE)
// - 1-of-2 multisig address generation (TEE + Master pubkeys)
// - ECIES encryption/decryption for target addresses
// - Merkle tree completion proof generation
// - Mixing pool management with decoy transactions
type Enclave struct {
	*base.BaseEnclave
	mu sync.RWMutex

	// Key handles for derived keys (actual keys stay in TEE)
	derivedKeys map[string]os.KeyHandle

	// Neo wallet handles for Neo N3 operations
	neoWallets map[string]os.WalletHandle

	// Master public key pool (received from offline Master)
	// Master private keys NEVER enter the TEE - only pubkeys
	masterPubKeys     []MasterPubKeyEntry
	masterPubKeyIndex uint32 // Next available index

	// TEE key index for HD derivation
	teeKeyIndex uint32

	// Mixing pool data (encrypted in storage)
	mixingPool   map[string]*PoolAccount
	pendingMixes map[string]*PendingMix

	// Service registration info
	registration *ServiceRegistration

	// Decoy transaction scheduler
	lastDecoyTx time.Time
}

// NewEnclave creates a new mixer enclave.
func NewEnclave(serviceOS os.ServiceOS) (*Enclave, error) {
	// Verify required capabilities
	if !serviceOS.HasCapability(os.CapKeys) {
		return nil, errors.New("mixer requires keys capability")
	}
	if !serviceOS.HasCapability(os.CapSecrets) {
		return nil, errors.New("mixer requires secrets capability")
	}
	if !serviceOS.HasCapability(os.CapNeo) {
		return nil, errors.New("mixer requires neo capability for Neo N3 operations")
	}

	return &Enclave{
		BaseEnclave:   base.NewBaseEnclave(ServiceID, serviceOS),
		derivedKeys:   make(map[string]os.KeyHandle),
		neoWallets:    make(map[string]os.WalletHandle),
		masterPubKeys: make([]MasterPubKeyEntry, 0),
		mixingPool:    make(map[string]*PoolAccount),
		pendingMixes:  make(map[string]*PendingMix),
	}, nil
}

// Initialize initializes the enclave.
func (e *Enclave) Initialize(ctx context.Context) error {
	if err := e.BaseEnclave.Initialize(ctx); err != nil {
		return err
	}
	e.Logger().Info("mixer enclave initialized")
	return nil
}

// DeriveKey derives a child key using HD key derivation within the enclave.
// The derived key NEVER leaves the TEE - only a handle is returned.
func (e *Enclave) DeriveKey(ctx context.Context, path string) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.IsReady() {
		return nil, errors.New("enclave not ready")
	}

	// Check if already derived
	if handle, exists := e.derivedKeys[path]; exists {
		return e.OS().Keys().GetPublicKey(ctx, handle)
	}

	// Derive key using TEE (key stays in enclave)
	handle, err := e.OS().Keys().DeriveKey(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	e.derivedKeys[path] = handle

	// Return public key (safe to export)
	return e.OS().Keys().GetPublicKey(ctx, handle)
}

// SignTransaction signs a transaction hash within the enclave.
// The private key NEVER leaves the TEE.
func (e *Enclave) SignTransaction(ctx context.Context, keyPath string, txHash []byte) ([]byte, error) {
	e.mu.RLock()
	handle, exists := e.derivedKeys[keyPath]
	e.mu.RUnlock()

	if !exists {
		// Derive the key first
		_, err := e.DeriveKey(ctx, keyPath)
		if err != nil {
			return nil, err
		}
		e.mu.RLock()
		handle = e.derivedKeys[keyPath]
		e.mu.RUnlock()
	}

	// Sign inside TEE (key never leaves enclave)
	return e.OS().Keys().Sign(ctx, handle, txHash)
}

// GetAddress returns the blockchain address for a derived key.
func (e *Enclave) GetAddress(ctx context.Context, keyPath string, chain os.ChainType) (string, error) {
	e.mu.RLock()
	handle, exists := e.derivedKeys[keyPath]
	e.mu.RUnlock()

	if !exists {
		// Derive the key first
		_, err := e.DeriveKey(ctx, keyPath)
		if err != nil {
			return "", err
		}
		e.mu.RLock()
		handle = e.derivedKeys[keyPath]
		e.mu.RUnlock()
	}

	return e.OS().Keys().GetAddress(ctx, handle, chain)
}

// CreateMixCommitment creates a cryptographic commitment for a mix operation.
func (e *Enclave) CreateMixCommitment(amount *big.Int, blinding []byte) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Create Pedersen-like commitment: C = H(amount || blinding)
	h := sha256.New()
	h.Write([]byte("MIX_COMMITMENT"))
	h.Write(amount.Bytes())
	h.Write(blinding)

	return h.Sum(nil), nil
}

// ProcessMix processes a mix request within the enclave.
func (e *Enclave) ProcessMix(ctx context.Context, requestID string, inputData []byte, outputPath string) (*MixOutput, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.IsReady() {
		return nil, errors.New("enclave not ready")
	}

	// Generate output address (key stays in TEE)
	outputPubKey, err := e.DeriveKey(ctx, outputPath)
	if err != nil {
		return nil, fmt.Errorf("derive output key: %w", err)
	}

	// Create proof of correct mixing
	proofHash := sha256.New()
	proofHash.Write([]byte("MIX_PROOF"))
	proofHash.Write(inputData)
	proofHash.Write(outputPubKey)
	proof := proofHash.Sum(nil)

	// Create commitment
	commitmentHash := sha256.New()
	commitmentHash.Write([]byte("MIX_COMMITMENT"))
	commitmentHash.Write(inputData)
	commitment := commitmentHash.Sum(nil)

	return &MixOutput{
		OutputAddress: outputPubKey,
		Proof:         proof,
		Commitment:    commitment,
	}, nil
}

// StorePoolAccount stores a pool account in the enclave's pool map.
func (e *Enclave) StorePoolAccount(ctx context.Context, pool *PoolAccount) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.IsReady() {
		return errors.New("enclave not ready")
	}

	e.mixingPool[pool.GetID()] = pool
	return nil
}

// GetPoolAccount retrieves a pool account by ID.
func (e *Enclave) GetPoolAccount(ctx context.Context, poolID string) (*PoolAccount, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	pool, exists := e.mixingPool[poolID]
	if !exists {
		return nil, errors.New("pool not found")
	}

	return pool, nil
}

// DeletePoolAccount removes a pool account from the enclave.
func (e *Enclave) DeletePoolAccount(ctx context.Context, poolID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	delete(e.mixingPool, poolID)
	return nil
}

// ListPoolAccounts returns all pool accounts.
func (e *Enclave) ListPoolAccounts() []*PoolAccount {
	e.mu.RLock()
	defer e.mu.RUnlock()

	pools := make([]*PoolAccount, 0, len(e.mixingPool))
	for _, pool := range e.mixingPool {
		pools = append(pools, pool)
	}
	return pools
}

// AddPendingMix adds a pending mix request.
func (e *Enclave) AddPendingMix(request *PendingMix) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.pendingMixes[request.RequestID] = request
}

// GetPendingMix retrieves a pending mix request.
func (e *Enclave) GetPendingMix(requestID string) (*PendingMix, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	req, ok := e.pendingMixes[requestID]
	return req, ok
}

// CompleteMix marks a mix as completed.
func (e *Enclave) CompleteMix(requestID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.pendingMixes, requestID)
}

// =============================================================================
// Helper Functions
// =============================================================================

// GetMixProofHash returns the hash of a mix proof for verification.
func GetMixProofHash(proof []byte) string {
	h := sha256.Sum256(proof)
	return hex.EncodeToString(h[:])
}

// VerifyMixProof verifies a mix proof.
func VerifyMixProof(inputData, outputPubKey, proof []byte) bool {
	expectedHash := sha256.New()
	expectedHash.Write([]byte("MIX_PROOF"))
	expectedHash.Write(inputData)
	expectedHash.Write(outputPubKey)
	expected := expectedHash.Sum(nil)

	if len(proof) != len(expected) {
		return false
	}
	for i := range proof {
		if proof[i] != expected[i] {
			return false
		}
	}
	return true
}

// =============================================================================
// Master Public Key Pool Management
// =============================================================================

// LoadMasterPubKeys loads pre-generated Master public keys into the TEE.
// Master private keys NEVER enter the TEE - only public keys are loaded.
// These are used to construct 1-of-2 multisig addresses with TEE keys.
func (e *Enclave) LoadMasterPubKeys(ctx context.Context, pubKeys []MasterPubKeyEntry) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.IsReady() {
		return errors.New("enclave not ready")
	}

	// Validate and add pubkeys
	for _, pk := range pubKeys {
		if pk.PublicKey == "" {
			continue
		}
		// Verify it's a valid compressed public key (33 bytes hex = 66 chars)
		if len(pk.PublicKey) != 66 {
			e.Logger().Warn("invalid master pubkey length", "index", pk.Index)
			continue
		}
		e.masterPubKeys = append(e.masterPubKeys, pk)
	}

	e.Logger().Info("loaded master pubkeys", "count", len(pubKeys), "total", len(e.masterPubKeys))
	return nil
}

// GetNextMasterPubKey returns the next available Master public key.
func (e *Enclave) GetNextMasterPubKey() (*MasterPubKeyEntry, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i := range e.masterPubKeys {
		if !e.masterPubKeys[i].Used {
			e.masterPubKeys[i].Used = true
			e.masterPubKeys[i].UsedAt = time.Now()
			return &e.masterPubKeys[i], nil
		}
	}

	return nil, errors.New("no available master pubkeys - pool exhausted")
}

// GetAvailableMasterPubKeyCount returns the count of unused Master pubkeys.
func (e *Enclave) GetAvailableMasterPubKeyCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	count := 0
	for _, pk := range e.masterPubKeys {
		if !pk.Used {
			count++
		}
	}
	return count
}

// =============================================================================
// 1-of-2 Multisig Address Generation (TEE + Master)
// =============================================================================

// CreateMultisigPool creates a new 1-of-2 multisig pool account.
// The multisig is constructed from TEE_PubKey_i and Master_PubKey_i.
// This provides security: TEE can operate normally, Master can recover if TEE fails.
func (e *Enclave) CreateMultisigPool(ctx context.Context) (*PoolAccount, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.IsReady() {
		return nil, errors.New("enclave not ready")
	}

	// Get next Master pubkey
	var masterPubKey *MasterPubKeyEntry
	for i := range e.masterPubKeys {
		if !e.masterPubKeys[i].Used {
			e.masterPubKeys[i].Used = true
			e.masterPubKeys[i].UsedAt = time.Now()
			masterPubKey = &e.masterPubKeys[i]
			break
		}
	}
	if masterPubKey == nil {
		return nil, errors.New("no available master pubkeys")
	}

	// Derive TEE key for this pool
	teeKeyPath := fmt.Sprintf("mixer/pool/%d", e.teeKeyIndex)
	e.teeKeyIndex++

	// Create Neo wallet for TEE key (key stays in enclave)
	walletHandle, err := e.OS().Neo().DeriveWallet(ctx, teeKeyPath)
	if err != nil {
		return nil, fmt.Errorf("derive tee wallet: %w", err)
	}

	// Get TEE public key
	teePubKeyBytes, err := e.OS().Neo().GetPublicKey(ctx, walletHandle)
	if err != nil {
		return nil, fmt.Errorf("get tee pubkey: %w", err)
	}
	teePubKey := hex.EncodeToString(teePubKeyBytes)

	// Store wallet handle
	e.neoWallets[teeKeyPath] = walletHandle

	// Build 1-of-2 multisig verification script
	// Neo N3 multisig format: PUSHINT(1) + PUSHDATA(pubkey1) + PUSHDATA(pubkey2) + PUSHINT(2) + SYSCALL(CheckMultisig)
	multisigScript, scriptHash, err := e.buildMultisigScript(teePubKey, masterPubKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("build multisig script: %w", err)
	}

	// Create pool account
	pool := &PoolAccount{
		ScriptHash:      scriptHash,
		WalletAddress:   scriptHashToAddress(scriptHash),
		TEEPublicKey:    teePubKey,
		MasterPublicKey: masterPubKey.PublicKey,
		MultiSigScript:  multisigScript,
		TEEKeyIndex:     e.teeKeyIndex - 1,
		MasterKeyIndex:  masterPubKey.Index,
		Status:          PoolAccountStatusActive,
		Balance:         "0",
		PendingIn:       "0",
		PendingOut:      "0",
		TotalReceived:   "0",
		TotalSent:       "0",
	}
	pool.GenerateID()
	pool.SetTimestamps()

	// Update master pubkey with pool reference
	masterPubKey.PoolID = pool.GetID()

	// Store in mixing pool
	e.mixingPool[pool.GetID()] = pool

	e.Logger().Info("created multisig pool",
		"pool_id", pool.GetID(),
		"script_hash", scriptHash,
		"tee_key_index", pool.TEEKeyIndex,
		"master_key_index", pool.MasterKeyIndex,
	)

	return pool, nil
}

// buildMultisigScript builds a 1-of-2 Neo N3 multisig verification script.
// Public keys are sorted lexicographically to ensure deterministic script hash.
func (e *Enclave) buildMultisigScript(teePubKey, masterPubKey string) (script string, scriptHash string, err error) {
	// Decode public keys
	teePubKeyBytes, err := hex.DecodeString(teePubKey)
	if err != nil {
		return "", "", fmt.Errorf("decode tee pubkey: %w", err)
	}
	masterPubKeyBytes, err := hex.DecodeString(masterPubKey)
	if err != nil {
		return "", "", fmt.Errorf("decode master pubkey: %w", err)
	}

	// Sort public keys lexicographically (Neo N3 requirement)
	pubKeys := [][]byte{teePubKeyBytes, masterPubKeyBytes}
	sort.Slice(pubKeys, func(i, j int) bool {
		return hex.EncodeToString(pubKeys[i]) < hex.EncodeToString(pubKeys[j])
	})

	// Build Neo N3 1-of-2 multisig script
	// Format: PUSHINT8(1) + PUSHDATA(pubkey1) + PUSHDATA(pubkey2) + PUSHINT8(2) + SYSCALL(CheckMultisig)
	var scriptBytes []byte

	// PUSHINT8 1 (minimum signatures required)
	scriptBytes = append(scriptBytes, 0x11) // PUSH1

	// PUSHDATA for each pubkey
	for _, pk := range pubKeys {
		if len(pk) <= 75 {
			scriptBytes = append(scriptBytes, byte(len(pk))) // PUSHBYTES
		} else {
			scriptBytes = append(scriptBytes, 0x4C, byte(len(pk))) // PUSHDATA1
		}
		scriptBytes = append(scriptBytes, pk...)
	}

	// PUSHINT8 2 (total pubkeys)
	scriptBytes = append(scriptBytes, 0x12) // PUSH2

	// SYSCALL CheckMultisig (Neo N3)
	// System.Crypto.CheckMultisig = 0x9ED0DC3A
	scriptBytes = append(scriptBytes, 0x41) // SYSCALL
	scriptBytes = append(scriptBytes, 0x9E, 0xD0, 0xDC, 0x3A)

	// Calculate script hash (RIPEMD160(SHA256(script)))
	sha256Hash := sha256.Sum256(scriptBytes)
	ripemd160Hash := ripemd160Hash(sha256Hash[:])

	script = hex.EncodeToString(scriptBytes)
	scriptHash = hex.EncodeToString(ripemd160Hash)

	return script, scriptHash, nil
}

// ripemd160Hash computes RIPEMD-160 hash (simplified implementation).
// In production, use golang.org/x/crypto/ripemd160.
func ripemd160Hash(data []byte) []byte {
	// Simplified: use SHA256 truncated to 20 bytes as placeholder
	// TODO: Replace with actual RIPEMD-160 implementation
	h := sha256.Sum256(data)
	return h[:20]
}

// scriptHashToAddress converts a script hash to Neo N3 address.
func scriptHashToAddress(scriptHash string) string {
	// Neo N3 address format: Base58Check(0x35 + scriptHash)
	// Simplified: return prefixed hex for now
	// TODO: Implement proper Base58Check encoding
	return "N" + scriptHash[:34]
}

// =============================================================================
// ECIES Encryption/Decryption for Target Addresses
// =============================================================================

// EncryptedTargetPayload represents the encrypted payload containing target addresses.
type EncryptedTargetPayload struct {
	Ciphertext   []byte `json:"ciphertext"`
	EphemeralPub []byte `json:"ephemeral_pub"`
	Nonce        []byte `json:"nonce"`
	Tag          []byte `json:"tag"`
}

// DecryptTargetPayload decrypts the ECIES-encrypted target addresses.
// The payload was encrypted by the user using the TEE's public key.
func (e *Enclave) DecryptTargetPayload(ctx context.Context, encryptedPayload string) ([]MixTarget, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.IsReady() {
		return nil, errors.New("enclave not ready")
	}

	// Decode the encrypted payload
	payloadBytes, err := hex.DecodeString(encryptedPayload)
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}

	var payload EncryptedTargetPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}

	// Use TEE's service key to decrypt
	// The decryption happens inside the enclave - plaintext never leaves
	var targets []MixTarget

	err = e.OS().Secrets().Use(ctx, "mixer_ecies_key", func(privateKey []byte) error {
		// Perform ECIES decryption inside the callback
		plaintext, err := eciesDecrypt(privateKey, payload.EphemeralPub, payload.Ciphertext, payload.Nonce, payload.Tag)
		if err != nil {
			return fmt.Errorf("ecies decrypt: %w", err)
		}

		// Parse targets from plaintext
		if err := json.Unmarshal(plaintext, &targets); err != nil {
			return fmt.Errorf("unmarshal targets: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Generate nonces for each target (for Merkle leaf computation)
	for i := range targets {
		if targets[i].Nonce == "" {
			nonce := make([]byte, 16)
			rand.Read(nonce)
			targets[i].Nonce = hex.EncodeToString(nonce)
		}
	}

	return targets, nil
}

// eciesDecrypt performs ECIES decryption.
func eciesDecrypt(privateKey, ephemeralPub, ciphertext, nonce, tag []byte) ([]byte, error) {
	// Parse ephemeral public key
	curve := elliptic.P256()
	x, y := elliptic.Unmarshal(curve, ephemeralPub)
	if x == nil {
		return nil, errors.New("invalid ephemeral public key")
	}

	// Parse private key
	privKey := new(ecdsa.PrivateKey)
	privKey.Curve = curve
	privKey.D = new(big.Int).SetBytes(privateKey)
	privKey.X, privKey.Y = curve.ScalarBaseMult(privateKey)

	// Compute shared secret using ECDH
	sharedX, _ := curve.ScalarMult(x, y, privateKey)
	sharedSecret := sha256.Sum256(sharedX.Bytes())

	// Derive AES key from shared secret
	aesKey := sharedSecret[:32]

	// Decrypt using AES-GCM
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	// Combine ciphertext and tag for GCM
	ciphertextWithTag := append(ciphertext, tag...)

	plaintext, err := gcm.Open(nil, nonce, ciphertextWithTag, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

// =============================================================================
// Service Claim Workflow
// =============================================================================

// ClaimMessage represents the message signed by TEE for service claim.
type ClaimMessage struct {
	ServiceID          string `json:"service_id"`
	RequestID          string `json:"request_id"`
	MultisigScriptHash string `json:"multisig_script_hash"`
	Nonce              string `json:"nonce"`
	Timestamp          int64  `json:"timestamp"`
}

// GenerateServiceClaim generates a signed claim for a mix request.
// This proves the TEE will handle the request and funds should be released to the multisig.
func (e *Enclave) GenerateServiceClaim(ctx context.Context, requestID string, pool *PoolAccount) (*ClaimInfo, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.IsReady() {
		return nil, errors.New("enclave not ready")
	}

	if e.registration == nil {
		return nil, errors.New("service not registered")
	}

	// Generate nonce
	nonceBytes := make([]byte, 16)
	rand.Read(nonceBytes)
	nonce := hex.EncodeToString(nonceBytes)

	// Build claim message
	claimMsg := ClaimMessage{
		ServiceID:          e.registration.ServiceID,
		RequestID:          requestID,
		MultisigScriptHash: pool.ScriptHash,
		Nonce:              nonce,
		Timestamp:          time.Now().Unix(),
	}

	// Serialize and hash the claim message
	claimBytes, err := json.Marshal(claimMsg)
	if err != nil {
		return nil, fmt.Errorf("marshal claim: %w", err)
	}
	claimHash := sha256.Sum256(claimBytes)

	// Sign with TEE's service key (signing happens inside enclave)
	teeKeyPath := "mixer/service"
	handle, exists := e.derivedKeys[teeKeyPath]
	if !exists {
		handle, err = e.OS().Keys().DeriveKey(ctx, teeKeyPath)
		if err != nil {
			return nil, fmt.Errorf("derive service key: %w", err)
		}
		e.derivedKeys[teeKeyPath] = handle
	}

	signature, err := e.OS().Keys().Sign(ctx, handle, claimHash[:])
	if err != nil {
		return nil, fmt.Errorf("sign claim: %w", err)
	}

	return &ClaimInfo{
		ServiceID:          e.registration.ServiceID,
		MultisigScriptHash: pool.ScriptHash,
		TEESignature:       hex.EncodeToString(signature),
		ClaimedAt:          time.Now(),
	}, nil
}

// =============================================================================
// Completion Proof Generation (Merkle Tree)
// =============================================================================

// GenerateCompletionProof generates a Merkle tree completion proof.
// Leaves are: H(targetAddress || amount || nonce || requestId)
func (e *Enclave) GenerateCompletionProof(ctx context.Context, requestID string, targets []MixTarget) (*CompletionProof, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.IsReady() {
		return nil, errors.New("enclave not ready")
	}

	if len(targets) == 0 {
		return nil, errors.New("no targets for completion proof")
	}

	// Build Merkle tree leaves
	leaves := make([]string, len(targets))
	for i, target := range targets {
		leaves[i] = target.LeafHash(requestID)
	}

	// Compute Merkle root
	merkleRoot := computeMerkleRoot(leaves)

	// Build message to sign: merkleRoot || requestId || serviceId || timestamp
	timestamp := time.Now()
	signMsg := fmt.Sprintf("%s|%s|%s|%d",
		merkleRoot,
		requestID,
		e.registration.ServiceID,
		timestamp.Unix(),
	)
	signHash := sha256.Sum256([]byte(signMsg))

	// Sign with TEE's service key
	teeKeyPath := "mixer/service"
	handle, exists := e.derivedKeys[teeKeyPath]
	if !exists {
		var err error
		handle, err = e.OS().Keys().DeriveKey(ctx, teeKeyPath)
		if err != nil {
			return nil, fmt.Errorf("derive service key: %w", err)
		}
		e.derivedKeys[teeKeyPath] = handle
	}

	signature, err := e.OS().Keys().Sign(ctx, handle, signHash[:])
	if err != nil {
		return nil, fmt.Errorf("sign completion: %w", err)
	}

	return &CompletionProof{
		MerkleRoot:   merkleRoot,
		TEESignature: hex.EncodeToString(signature),
		Timestamp:    timestamp,
		OutputCount:  len(targets),
	}, nil
}

// computeMerkleRoot computes the Merkle root from leaf hashes.
func computeMerkleRoot(leaves []string) string {
	if len(leaves) == 0 {
		return ""
	}
	if len(leaves) == 1 {
		return leaves[0]
	}

	// Pad to even number if needed
	if len(leaves)%2 != 0 {
		leaves = append(leaves, leaves[len(leaves)-1])
	}

	// Build tree level by level
	for len(leaves) > 1 {
		var nextLevel []string
		for i := 0; i < len(leaves); i += 2 {
			combined := leaves[i] + leaves[i+1]
			hash := sha256.Sum256([]byte(combined))
			nextLevel = append(nextLevel, hex.EncodeToString(hash[:]))
		}
		leaves = nextLevel
	}

	return leaves[0]
}

// GenerateMerkleProof generates a Merkle proof for a specific target.
func (e *Enclave) GenerateMerkleProof(requestID string, targets []MixTarget, targetIndex int) (*MerkleProof, error) {
	if targetIndex < 0 || targetIndex >= len(targets) {
		return nil, errors.New("invalid target index")
	}

	// Build all leaves
	leaves := make([]string, len(targets))
	for i, target := range targets {
		leaves[i] = target.LeafHash(requestID)
	}

	// Pad to even number if needed
	if len(leaves)%2 != 0 {
		leaves = append(leaves, leaves[len(leaves)-1])
	}

	// Build proof path
	var path []string
	var positions []bool
	idx := targetIndex

	for len(leaves) > 1 {
		// Get sibling
		siblingIdx := idx ^ 1 // XOR with 1 to get sibling
		if siblingIdx < len(leaves) {
			path = append(path, leaves[siblingIdx])
			positions = append(positions, idx%2 == 1) // true if we're on the right
		}

		// Move to next level
		var nextLevel []string
		for i := 0; i < len(leaves); i += 2 {
			combined := leaves[i] + leaves[i+1]
			hash := sha256.Sum256([]byte(combined))
			nextLevel = append(nextLevel, hex.EncodeToString(hash[:]))
		}
		leaves = nextLevel
		idx = idx / 2
	}

	return &MerkleProof{
		LeafHash: targets[targetIndex].LeafHash(requestID),
		Path:     path,
		Position: positions,
	}, nil
}

// =============================================================================
// Decoy Transaction Generation
// =============================================================================

// GenerateDecoyTransaction generates a noise transaction for privacy.
// Decoy transactions are small transfers between internal multisig addresses
// to obscure the actual mixing pattern.
func (e *Enclave) GenerateDecoyTransaction(ctx context.Context) (*MixTransaction, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.IsReady() {
		return nil, errors.New("enclave not ready")
	}

	// Check if enough time has passed since last decoy
	minInterval := DecoyTxIntervalMin
	maxInterval := DecoyTxIntervalMax
	if time.Since(e.lastDecoyTx) < minInterval {
		return nil, nil // Too soon, skip
	}

	// Get active pools
	var activePools []*PoolAccount
	for _, pool := range e.mixingPool {
		if pool.Status == PoolAccountStatusActive {
			activePools = append(activePools, pool)
		}
	}

	if len(activePools) < 2 {
		return nil, nil // Need at least 2 pools for decoy
	}

	// Select random source and destination pools
	srcIdx := randomInt(len(activePools))
	dstIdx := randomInt(len(activePools))
	for dstIdx == srcIdx {
		dstIdx = randomInt(len(activePools))
	}

	srcPool := activePools[srcIdx]
	dstPool := activePools[dstIdx]

	// Generate small random amount (0.001 - 0.1 GAS)
	amount := fmt.Sprintf("0.%03d", randomInt(100)+1)

	// Create decoy transaction
	tx := &MixTransaction{
		Type:       MixTxTypeDecoy,
		Status:     MixTxStatusScheduled,
		FromPoolID: srcPool.GetID(),
		ToPoolID:   dstPool.GetID(),
		Amount:     amount,
	}
	tx.GenerateID()
	tx.SetTimestamps()
	tx.ScheduledAt = time.Now().Add(time.Duration(randomInt(int(maxInterval-minInterval))) + minInterval)

	e.lastDecoyTx = time.Now()

	e.Logger().Debug("generated decoy transaction",
		"tx_id", tx.GetID(),
		"from", srcPool.ScriptHash[:8],
		"to", dstPool.ScriptHash[:8],
		"amount", amount,
	)

	return tx, nil
}

// randomInt returns a random integer in [0, max).
func randomInt(max int) int {
	if max <= 0 {
		return 0
	}
	b := make([]byte, 4)
	rand.Read(b)
	return int(binary.BigEndian.Uint32(b)) % max
}

// =============================================================================
// Service Registration
// =============================================================================

// SetServiceRegistration sets the service registration info.
func (e *Enclave) SetServiceRegistration(reg *ServiceRegistration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.registration = reg
}

// GetServiceRegistration returns the service registration info.
func (e *Enclave) GetServiceRegistration() *ServiceRegistration {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.registration
}

// GetTEEPublicKey returns the TEE's service public key for contract registration.
func (e *Enclave) GetTEEPublicKey(ctx context.Context) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.IsReady() {
		return "", errors.New("enclave not ready")
	}

	teeKeyPath := "mixer/service"
	handle, exists := e.derivedKeys[teeKeyPath]
	if !exists {
		var err error
		handle, err = e.OS().Keys().DeriveKey(ctx, teeKeyPath)
		if err != nil {
			return "", fmt.Errorf("derive service key: %w", err)
		}
		e.derivedKeys[teeKeyPath] = handle
	}

	pubKey, err := e.OS().Keys().GetPublicKey(ctx, handle)
	if err != nil {
		return "", fmt.Errorf("get public key: %w", err)
	}

	return hex.EncodeToString(pubKey), nil
}

// =============================================================================
// Pool Rotation and Retirement
// =============================================================================

// RetirePool marks a pool for retirement and schedules fund migration.
func (e *Enclave) RetirePool(ctx context.Context, poolID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	pool, exists := e.mixingPool[poolID]
	if !exists {
		return errors.New("pool not found")
	}

	pool.Status = PoolAccountStatusRetiring
	pool.RetireAfter = time.Now().Add(24 * time.Hour) // 24h grace period

	e.Logger().Info("pool marked for retirement", "pool_id", poolID)
	return nil
}

// GetPoolsForRotation returns pools that should be rotated based on age/usage.
func (e *Enclave) GetPoolsForRotation() []*PoolAccount {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var toRotate []*PoolAccount
	rotationAge := time.Duration(DefaultPoolRetireDays) * 24 * time.Hour

	for _, pool := range e.mixingPool {
		if pool.Status != PoolAccountStatusActive {
			continue
		}

		// Check age
		if time.Since(pool.CreatedAt) > rotationAge {
			toRotate = append(toRotate, pool)
			continue
		}

		// Check transaction count (rotate after high usage)
		if pool.TransactionCount > 1000 {
			toRotate = append(toRotate, pool)
		}
	}

	return toRotate
}
