// Package enclave provides TEE-protected CRE (Chainlink Runtime Environment) operations.
// Function execution and result signing run inside the enclave
// to ensure computation integrity and prevent tampering.
//
// This package integrates with the Enclave SDK for unified TEE operations.
package enclave

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/R3E-Network/service_layer/system/os/tee/sdk"
)

// EnclaveCRE handles all CRE operations within the TEE enclave.
// Critical operations:
// - Function execution
// - Result signing
// - Execution proof generation
type EnclaveCRE struct {
	*sdk.BaseEnclave
	execResults map[string]*ExecutionResult
}

// ExecutionResult represents a function execution result.
type ExecutionResult struct {
	RequestID  string
	FunctionID string
	Input      []byte
	Output     []byte
	GasUsed    uint64
	Signature  []byte
	PublicKey  []byte
}

// ExecutionProof represents a proof of correct execution.
type ExecutionProof struct {
	ResultHash []byte
	InputHash  []byte
	OutputHash []byte
	Signature  []byte
}

// CREConfig holds configuration for the CRE enclave.
type CREConfig struct {
	ServiceID string
	RequestID string
	CallerID  string
	AccountID string
	SealKey   []byte
}

// NewEnclaveCRE creates a new enclave CRE handler.
func NewEnclaveCRE() (*EnclaveCRE, error) {
	base, err := sdk.NewBaseEnclave("cre")
	if err != nil {
		return nil, err
	}

	return &EnclaveCRE{
		BaseEnclave: base,
		execResults: make(map[string]*ExecutionResult),
	}, nil
}

// NewEnclaveCREWithSDK creates a CRE handler with full SDK integration.
func NewEnclaveCREWithSDK(cfg *CREConfig) (*EnclaveCRE, error) {
	baseCfg := &sdk.BaseConfig{
		ServiceID:   cfg.ServiceID,
		ServiceName: "cre",
		RequestID:   cfg.RequestID,
		CallerID:    cfg.CallerID,
		AccountID:   cfg.AccountID,
		SealKey:     cfg.SealKey,
	}

	base, err := sdk.NewBaseEnclaveWithSDK(baseCfg)
	if err != nil {
		return nil, err
	}

	return &EnclaveCRE{
		BaseEnclave: base,
		execResults: make(map[string]*ExecutionResult),
	}, nil
}

// InitializeWithSDK initializes the CRE handler with an existing SDK instance.
func (e *EnclaveCRE) InitializeWithSDK(enclaveSDK sdk.EnclaveSDK) error {
	return e.BaseEnclave.InitializeWithSDK(enclaveSDK)
}

// ExecuteFunction executes a function within the enclave and signs the result.
func (e *EnclaveCRE) ExecuteFunction(requestID, functionID string, input []byte, executor func([]byte) ([]byte, uint64, error)) (*ExecutionResult, error) {
	e.Lock()
	defer e.Unlock()

	output, gasUsed, err := executor(input)
	if err != nil {
		return nil, err
	}

	message := sha256.New()
	message.Write([]byte(requestID))
	message.Write([]byte(functionID))
	message.Write(input)
	message.Write(output)
	hash := message.Sum(nil)

	signingKey := e.GetSigningKey()
	r, s, err := ecdsa.Sign(rand.Reader, signingKey, hash)
	if err != nil {
		return nil, err
	}

	signature := append(r.Bytes(), s.Bytes()...)
	pubKey := e.GetPublicKey()

	result := &ExecutionResult{
		RequestID:  requestID,
		FunctionID: functionID,
		Input:      input,
		Output:     output,
		GasUsed:    gasUsed,
		Signature:  signature,
		PublicKey:  pubKey,
	}

	e.execResults[requestID] = result
	return result, nil
}

// GenerateExecutionProof generates a proof of correct execution.
func (e *EnclaveCRE) GenerateExecutionProof(result *ExecutionResult) (*ExecutionProof, error) {
	e.Lock()
	defer e.Unlock()

	inputHash := sha256.Sum256(result.Input)
	outputHash := sha256.Sum256(result.Output)

	resultHash := sha256.New()
	resultHash.Write(inputHash[:])
	resultHash.Write(outputHash[:])
	resultHash.Write(result.Signature)

	proofMessage := resultHash.Sum(nil)
	signingKey := e.GetSigningKey()
	r, s, err := ecdsa.Sign(rand.Reader, signingKey, proofMessage)
	if err != nil {
		return nil, err
	}

	return &ExecutionProof{
		ResultHash: proofMessage,
		InputHash:  inputHash[:],
		OutputHash: outputHash[:],
		Signature:  append(r.Bytes(), s.Bytes()...),
	}, nil
}

// VerifyExecutionResult verifies an execution result signature.
func VerifyExecutionResult(result *ExecutionResult) (bool, error) {
	if len(result.Signature) < 64 {
		return false, errors.New("invalid signature length")
	}

	message := sha256.New()
	message.Write([]byte(result.RequestID))
	message.Write([]byte(result.FunctionID))
	message.Write(result.Input)
	message.Write(result.Output)
	hash := message.Sum(nil)

	return sdk.VerifySignature(result.PublicKey, hash, result.Signature)
}

// GetResultHash returns the hash of an execution result.
func GetResultHash(result *ExecutionResult) string {
	h := sha256.New()
	h.Write([]byte(result.RequestID))
	h.Write(result.Output)
	return hex.EncodeToString(h.Sum(nil))
}
