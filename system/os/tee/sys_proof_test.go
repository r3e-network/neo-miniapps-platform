package tee

import (
	"context"
	"testing"
	"time"
)

func TestNewSysProof(t *testing.T) {
	proof := NewSysProof("test-enclave", nil)
	if proof == nil {
		t.Fatal("expected non-nil SysProof")
	}
}

func TestSysProof_GenerateProof(t *testing.T) {
	ctx := context.Background()
	proofGen := NewSysProof("test-enclave", nil)

	data := []byte("test data to prove")
	proof, err := proofGen.GenerateProof(ctx, data)
	if err != nil {
		t.Fatalf("GenerateProof() error = %v", err)
	}

	if proof.ProofID == "" {
		t.Error("GenerateProof() ProofID should not be empty")
	}

	if proof.EnclaveID != "test-enclave" {
		t.Errorf("GenerateProof() EnclaveID = %s, want test-enclave", proof.EnclaveID)
	}

	if proof.InputHash == "" {
		t.Error("GenerateProof() InputHash should not be empty")
	}

	if len(proof.Signature) != 64 {
		t.Errorf("GenerateProof() Signature len = %d, want 64", len(proof.Signature))
	}

	if proof.Timestamp.IsZero() {
		t.Error("GenerateProof() Timestamp should not be zero")
	}
}

func TestSysProof_VerifyProof(t *testing.T) {
	ctx := context.Background()
	proofGen := NewSysProof("test-enclave", nil)

	data := []byte("test data to prove")
	proof, err := proofGen.GenerateProof(ctx, data)
	if err != nil {
		t.Fatalf("GenerateProof() error = %v", err)
	}

	valid, err := proofGen.VerifyProof(ctx, proof)
	if err != nil {
		t.Fatalf("VerifyProof() error = %v", err)
	}

	if !valid {
		t.Error("VerifyProof() should return true for valid proof")
	}
}

func TestSysProof_VerifyProof_InvalidSignature(t *testing.T) {
	ctx := context.Background()
	proofGen := NewSysProof("test-enclave", nil)

	data := []byte("test data")
	proof, _ := proofGen.GenerateProof(ctx, data)

	// Tamper with signature
	proof.Signature[0] ^= 0xFF

	valid, err := proofGen.VerifyProof(ctx, proof)
	if err != nil {
		t.Fatalf("VerifyProof() error = %v", err)
	}

	if valid {
		t.Error("VerifyProof() should return false for tampered signature")
	}
}

func TestSysProof_VerifyProof_TamperedData(t *testing.T) {
	ctx := context.Background()
	proofGen := NewSysProof("test-enclave", nil)

	data := []byte("test data")
	proof, _ := proofGen.GenerateProof(ctx, data)

	// Tamper with input hash
	proof.InputHash = "tampered_hash"

	valid, err := proofGen.VerifyProof(ctx, proof)
	if err != nil {
		t.Fatalf("VerifyProof() error = %v", err)
	}

	if valid {
		t.Error("VerifyProof() should return false for tampered data")
	}
}

func TestSysProof_VerifyProof_NilProof(t *testing.T) {
	ctx := context.Background()
	proofGen := NewSysProof("test-enclave", nil)

	_, err := proofGen.VerifyProof(ctx, nil)
	if err == nil {
		t.Error("VerifyProof() should return error for nil proof")
	}
}

func TestSysProof_VerifyProof_EmptyFields(t *testing.T) {
	ctx := context.Background()
	proofGen := NewSysProof("test-enclave", nil)

	tests := []struct {
		name  string
		proof *ExecutionProof
	}{
		{"empty proof ID", &ExecutionProof{EnclaveID: "x", InputHash: "x", Signature: make([]byte, 64)}},
		{"empty enclave ID", &ExecutionProof{ProofID: "x", InputHash: "x", Signature: make([]byte, 64)}},
		{"empty input hash", &ExecutionProof{ProofID: "x", EnclaveID: "x", Signature: make([]byte, 64)}},
		{"empty signature", &ExecutionProof{ProofID: "x", EnclaveID: "x", InputHash: "x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := proofGen.VerifyProof(ctx, tt.proof)
			if err == nil {
				t.Error("VerifyProof() should return error for invalid proof")
			}
		})
	}
}

func TestSysProof_GetPublicKey(t *testing.T) {
	proofGen := NewSysProof("test-enclave", nil).(*sysProofImpl)

	pubKey := proofGen.GetPublicKey()
	if len(pubKey) != 65 {
		t.Errorf("GetPublicKey() len = %d, want 65", len(pubKey))
	}

	if pubKey[0] != 0x04 {
		t.Error("GetPublicKey() should return uncompressed format (0x04 prefix)")
	}
}

func TestProofVerifier(t *testing.T) {
	ctx := context.Background()
	proofGen := NewSysProof("test-enclave", nil).(*sysProofImpl)

	// Generate a proof
	data := []byte("test data")
	proof, err := proofGen.GenerateProof(ctx, data)
	if err != nil {
		t.Fatalf("GenerateProof() error = %v", err)
	}

	// Create verifier with public key
	pubKey := proofGen.GetPublicKey()
	verifier, err := NewProofVerifier(pubKey)
	if err != nil {
		t.Fatalf("NewProofVerifier() error = %v", err)
	}

	// Verify the proof
	valid, err := verifier.Verify(proof)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	if !valid {
		t.Error("Verify() should return true for valid proof")
	}
}

func TestProofVerifier_InvalidPublicKey(t *testing.T) {
	_, err := NewProofVerifier([]byte("invalid"))
	if err == nil {
		t.Error("NewProofVerifier() should return error for invalid public key")
	}
}

func TestProofChain(t *testing.T) {
	ctx := context.Background()
	proofGen := NewSysProof("test-enclave", nil)
	chain := NewProofChain()

	// Add some proofs
	for i := 0; i < 5; i++ {
		proof, _ := proofGen.GenerateProof(ctx, []byte("data"))
		chain.Add(proof)
	}

	if chain.Len() != 5 {
		t.Errorf("Len() = %d, want 5", chain.Len())
	}

	proofs := chain.GetAll()
	if len(proofs) != 5 {
		t.Errorf("GetAll() len = %d, want 5", len(proofs))
	}
}

func TestProofChain_GetByID(t *testing.T) {
	ctx := context.Background()
	proofGen := NewSysProof("test-enclave", nil)
	chain := NewProofChain()

	proof, _ := proofGen.GenerateProof(ctx, []byte("data"))
	chain.Add(proof)

	found := chain.GetByID(proof.ProofID)
	if found == nil {
		t.Error("GetByID() should find the proof")
	}

	if found.ProofID != proof.ProofID {
		t.Error("GetByID() returned wrong proof")
	}

	notFound := chain.GetByID("nonexistent")
	if notFound != nil {
		t.Error("GetByID() should return nil for nonexistent ID")
	}
}

func TestProofChain_ComputeChainHash(t *testing.T) {
	ctx := context.Background()
	proofGen := NewSysProof("test-enclave", nil)
	chain := NewProofChain()

	// Add proofs
	for i := 0; i < 3; i++ {
		proof, _ := proofGen.GenerateProof(ctx, []byte("data"))
		chain.Add(proof)
	}

	hash1 := chain.ComputeChainHash()
	if hash1 == "" {
		t.Error("ComputeChainHash() should not return empty string")
	}

	// Hash should be consistent
	hash2 := chain.ComputeChainHash()
	if hash1 != hash2 {
		t.Error("ComputeChainHash() should return consistent results")
	}
}

func TestSysProof_GenerateExecutionProof(t *testing.T) {
	ctx := context.Background()
	proofGen := NewSysProof("test-enclave", nil).(*sysProofImpl)

	req := ExecutionRequest{
		ServiceID:  "test-service",
		AccountID:  "test-account",
		Script:     "function main() { return {}; }",
		EntryPoint: "main",
		Input:      map[string]any{"key": "value"},
	}

	result := &ExecutionResult{
		Output: map[string]any{"result": "success"},
		Status: ExecutionStatusSucceeded,
	}

	proof, err := proofGen.GenerateExecutionProof(ctx, req, result)
	if err != nil {
		t.Fatalf("GenerateExecutionProof() error = %v", err)
	}

	if proof.ProofID == "" {
		t.Error("GenerateExecutionProof() ProofID should not be empty")
	}

	if proof.InputHash == "" {
		t.Error("GenerateExecutionProof() InputHash should not be empty")
	}

	if proof.OutputHash == "" {
		t.Error("GenerateExecutionProof() OutputHash should not be empty")
	}

	// Verify the proof
	valid, err := proofGen.VerifyProof(ctx, proof)
	if err != nil {
		t.Fatalf("VerifyProof() error = %v", err)
	}

	if !valid {
		t.Error("VerifyProof() should return true for valid execution proof")
	}
}

func TestSysProof_UniqueProofIDs(t *testing.T) {
	ctx := context.Background()
	proofGen := NewSysProof("test-enclave", nil)

	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		proof, _ := proofGen.GenerateProof(ctx, []byte("data"))
		if ids[proof.ProofID] {
			t.Errorf("Duplicate proof ID: %s", proof.ProofID)
		}
		ids[proof.ProofID] = true
	}
}

func TestSysProof_TimestampNotInFuture(t *testing.T) {
	ctx := context.Background()
	proofGen := NewSysProof("test-enclave", nil)

	proof, _ := proofGen.GenerateProof(ctx, []byte("data"))

	// Tamper with timestamp to be in the future
	proof.Timestamp = time.Now().Add(time.Hour)

	_, err := proofGen.VerifyProof(ctx, proof)
	if err == nil {
		t.Error("VerifyProof() should return error for future timestamp")
	}
}
