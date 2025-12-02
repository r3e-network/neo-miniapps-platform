// Package tee provides the Trusted Execution Environment (TEE) engine for confidential computing.
//
// sys.proof - Execution Proof Generation and Verification
//
// This file implements the sys.proof API for generating and verifying execution proofs.
// Proofs provide cryptographic evidence that code was executed within a TEE enclave.
package tee

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sync"
	"time"
)

// sysProofImpl implements SysProof with real proof generation.
type sysProofImpl struct {
	mu        sync.RWMutex
	enclaveID string
	signingKey *ecdsa.PrivateKey
	attestor   Attestor
}

// NewSysProof creates a new SysProof implementation.
func NewSysProof(enclaveID string, attestor Attestor) SysProof {
	// Generate a signing key for this proof instance
	privateKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	return &sysProofImpl{
		enclaveID:  enclaveID,
		signingKey: privateKey,
		attestor:   attestor,
	}
}

// GenerateProof generates a proof of execution.
func (p *sysProofImpl) GenerateProof(ctx context.Context, data []byte) (*ExecutionProof, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.signingKey == nil {
		return nil, fmt.Errorf("signing key not initialized")
	}

	// Generate proof ID
	proofID := generateProofID()

	// Hash the input data
	inputHash := sha256.Sum256(data)

	// Create proof structure
	proof := &ExecutionProof{
		ProofID:    proofID,
		EnclaveID:  p.enclaveID,
		InputHash:  hex.EncodeToString(inputHash[:]),
		OutputHash: "", // Will be set after execution
		Timestamp:  time.Now().UTC(),
	}

	// Sign the proof
	proofData := p.serializeProofData(proof)
	signature, err := p.sign(proofData)
	if err != nil {
		return nil, fmt.Errorf("sign proof: %w", err)
	}
	proof.Signature = signature

	// Get attestation quote if available
	if p.attestor != nil {
		report, err := p.attestor.GenerateReport(ctx)
		if err == nil && report != nil {
			proof.AttestationQuote = report.Quote
		}
	}

	return proof, nil
}

// GenerateExecutionProof generates a complete proof for an execution request and result.
func (p *sysProofImpl) GenerateExecutionProof(ctx context.Context, req ExecutionRequest, result *ExecutionResult) (*ExecutionProof, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.signingKey == nil {
		return nil, fmt.Errorf("signing key not initialized")
	}

	// Generate proof ID
	proofID := generateProofID()

	// Hash the input
	inputData, _ := json.Marshal(map[string]any{
		"service_id":  req.ServiceID,
		"account_id":  req.AccountID,
		"script":      req.Script,
		"entry_point": req.EntryPoint,
		"input":       req.Input,
	})
	inputHash := sha256.Sum256(inputData)

	// Hash the output
	outputData, _ := json.Marshal(map[string]any{
		"output": result.Output,
		"status": result.Status,
	})
	outputHash := sha256.Sum256(outputData)

	// Create proof structure
	proof := &ExecutionProof{
		ProofID:    proofID,
		EnclaveID:  p.enclaveID,
		InputHash:  hex.EncodeToString(inputHash[:]),
		OutputHash: hex.EncodeToString(outputHash[:]),
		Timestamp:  time.Now().UTC(),
	}

	// Sign the proof
	proofData := p.serializeProofData(proof)
	signature, err := p.sign(proofData)
	if err != nil {
		return nil, fmt.Errorf("sign proof: %w", err)
	}
	proof.Signature = signature

	// Get attestation quote if available
	if p.attestor != nil {
		report, err := p.attestor.GenerateReport(ctx)
		if err == nil && report != nil {
			proof.AttestationQuote = report.Quote
		}
	}

	return proof, nil
}

// VerifyProof verifies an execution proof.
func (p *sysProofImpl) VerifyProof(ctx context.Context, proof *ExecutionProof) (bool, error) {
	if proof == nil {
		return false, fmt.Errorf("proof is nil")
	}

	// Verify required fields
	if proof.ProofID == "" {
		return false, fmt.Errorf("proof ID is empty")
	}
	if proof.EnclaveID == "" {
		return false, fmt.Errorf("enclave ID is empty")
	}
	if proof.InputHash == "" {
		return false, fmt.Errorf("input hash is empty")
	}
	if len(proof.Signature) == 0 {
		return false, fmt.Errorf("signature is empty")
	}

	// Verify timestamp is not in the future
	if proof.Timestamp.After(time.Now().Add(time.Minute)) {
		return false, fmt.Errorf("proof timestamp is in the future")
	}

	// Verify signature
	proofData := p.serializeProofData(proof)
	valid, err := p.verify(proofData, proof.Signature)
	if err != nil {
		return false, fmt.Errorf("verify signature: %w", err)
	}
	if !valid {
		return false, nil
	}

	// Verify attestation quote if present
	if len(proof.AttestationQuote) > 0 && p.attestor != nil {
		report := &AttestationReport{
			Quote: proof.AttestationQuote,
		}
		valid, err := p.attestor.VerifyReport(ctx, report)
		if err != nil {
			return false, fmt.Errorf("verify attestation: %w", err)
		}
		if !valid {
			return false, nil
		}
	}

	return true, nil
}

// GetAttestation returns the current TEE attestation.
func (p *sysProofImpl) GetAttestation(ctx context.Context) (*AttestationReport, error) {
	if p.attestor == nil {
		return nil, fmt.Errorf("attestor not available")
	}
	return p.attestor.GenerateReport(ctx)
}

// GetPublicKey returns the public key used for signing proofs.
func (p *sysProofImpl) GetPublicKey() []byte {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.signingKey == nil {
		return nil
	}
	return elliptic.Marshal(elliptic.P256(), p.signingKey.PublicKey.X, p.signingKey.PublicKey.Y)
}

// serializeProofData creates a canonical representation of proof data for signing.
func (p *sysProofImpl) serializeProofData(proof *ExecutionProof) []byte {
	data := fmt.Sprintf("%s|%s|%s|%s|%d",
		proof.ProofID,
		proof.EnclaveID,
		proof.InputHash,
		proof.OutputHash,
		proof.Timestamp.Unix(),
	)
	return []byte(data)
}

// sign signs data using the enclave's signing key.
func (p *sysProofImpl) sign(data []byte) ([]byte, error) {
	hash := sha256.Sum256(data)
	r, s, err := ecdsa.Sign(rand.Reader, p.signingKey, hash[:])
	if err != nil {
		return nil, err
	}

	// Encode r and s as 32-byte big-endian integers
	sig := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(sig[32-len(rBytes):32], rBytes)
	copy(sig[64-len(sBytes):64], sBytes)
	return sig, nil
}

// verify verifies a signature.
func (p *sysProofImpl) verify(data []byte, signature []byte) (bool, error) {
	if len(signature) != 64 {
		return false, fmt.Errorf("invalid signature length")
	}

	hash := sha256.Sum256(data)
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])

	return ecdsa.Verify(&p.signingKey.PublicKey, hash[:], r, s), nil
}

// generateProofID generates a unique proof ID.
func generateProofID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return "proof_" + hex.EncodeToString(bytes)
}

// =============================================================================
// Proof Verifier - For external verification
// =============================================================================

// ProofVerifier verifies execution proofs using a public key.
type ProofVerifier struct {
	publicKey *ecdsa.PublicKey
}

// NewProofVerifier creates a new proof verifier from a public key.
func NewProofVerifier(publicKeyBytes []byte) (*ProofVerifier, error) {
	if len(publicKeyBytes) != 65 || publicKeyBytes[0] != 0x04 {
		return nil, fmt.Errorf("invalid public key format")
	}

	x, y := elliptic.Unmarshal(elliptic.P256(), publicKeyBytes)
	if x == nil {
		return nil, fmt.Errorf("failed to parse public key")
	}

	return &ProofVerifier{
		publicKey: &ecdsa.PublicKey{
			Curve: elliptic.P256(),
			X:     x,
			Y:     y,
		},
	}, nil
}

// Verify verifies an execution proof.
func (v *ProofVerifier) Verify(proof *ExecutionProof) (bool, error) {
	if proof == nil {
		return false, fmt.Errorf("proof is nil")
	}

	if len(proof.Signature) != 64 {
		return false, fmt.Errorf("invalid signature length")
	}

	// Serialize proof data
	data := fmt.Sprintf("%s|%s|%s|%s|%d",
		proof.ProofID,
		proof.EnclaveID,
		proof.InputHash,
		proof.OutputHash,
		proof.Timestamp.Unix(),
	)

	hash := sha256.Sum256([]byte(data))
	r := new(big.Int).SetBytes(proof.Signature[:32])
	s := new(big.Int).SetBytes(proof.Signature[32:])

	return ecdsa.Verify(v.publicKey, hash[:], r, s), nil
}

// =============================================================================
// Proof Chain - For linking multiple proofs
// =============================================================================

// ProofChain represents a chain of linked execution proofs.
type ProofChain struct {
	mu     sync.RWMutex
	proofs []*ExecutionProof
}

// NewProofChain creates a new proof chain.
func NewProofChain() *ProofChain {
	return &ProofChain{
		proofs: make([]*ExecutionProof, 0),
	}
}

// Add adds a proof to the chain.
func (c *ProofChain) Add(proof *ExecutionProof) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.proofs = append(c.proofs, proof)
}

// GetAll returns all proofs in the chain.
func (c *ProofChain) GetAll() []*ExecutionProof {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]*ExecutionProof, len(c.proofs))
	copy(result, c.proofs)
	return result
}

// GetByID returns a proof by ID.
func (c *ProofChain) GetByID(proofID string) *ExecutionProof {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, p := range c.proofs {
		if p.ProofID == proofID {
			return p
		}
	}
	return nil
}

// Len returns the number of proofs in the chain.
func (c *ProofChain) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.proofs)
}

// ComputeChainHash computes a hash of all proofs in the chain.
func (c *ProofChain) ComputeChainHash() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	h := sha256.New()
	for _, p := range c.proofs {
		h.Write([]byte(p.ProofID))
		h.Write([]byte(p.InputHash))
		h.Write([]byte(p.OutputHash))
	}
	return hex.EncodeToString(h.Sum(nil))
}
