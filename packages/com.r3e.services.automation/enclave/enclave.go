// Package enclave provides TEE-protected automation operations.
// Job scheduling and execution verification run inside the enclave
// to ensure tamper-proof automation.
//
// This package integrates with the Enclave SDK for unified TEE operations.
package enclave

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"errors"

	"github.com/R3E-Network/service_layer/system/tee/sdk"
)

// EnclaveAutomation handles automation operations within the TEE.
type EnclaveAutomation struct {
	*sdk.BaseEnclave
	jobs map[string]*JobExecution
}

// JobExecution represents a job execution record.
type JobExecution struct {
	JobID     string
	Trigger   []byte
	Result    []byte
	Signature []byte
}

// AutomationConfig holds configuration for the automation enclave.
type AutomationConfig struct {
	ServiceID string
	RequestID string
	CallerID  string
	AccountID string
	SealKey   []byte
}

// NewEnclaveAutomation creates a new enclave automation handler.
func NewEnclaveAutomation() (*EnclaveAutomation, error) {
	base, err := sdk.NewBaseEnclave("automation")
	if err != nil {
		return nil, err
	}
	return &EnclaveAutomation{
		BaseEnclave: base,
		jobs:        make(map[string]*JobExecution),
	}, nil
}

// NewEnclaveAutomationWithSDK creates an automation handler with full SDK integration.
func NewEnclaveAutomationWithSDK(cfg *AutomationConfig) (*EnclaveAutomation, error) {
	baseCfg := &sdk.BaseConfig{
		ServiceID:   cfg.ServiceID,
		ServiceName: "automation",
		RequestID:   cfg.RequestID,
		CallerID:    cfg.CallerID,
		AccountID:   cfg.AccountID,
		SealKey:     cfg.SealKey,
	}

	base, err := sdk.NewBaseEnclaveWithSDK(baseCfg)
	if err != nil {
		return nil, err
	}

	return &EnclaveAutomation{
		BaseEnclave: base,
		jobs:        make(map[string]*JobExecution),
	}, nil
}

// InitializeWithSDK initializes the automation handler with an existing SDK instance.
func (e *EnclaveAutomation) InitializeWithSDK(enclaveSDK sdk.EnclaveSDK) error {
	return e.BaseEnclave.InitializeWithSDK(enclaveSDK)
}

// ExecuteAndSign executes a job and signs the result.
func (e *EnclaveAutomation) ExecuteAndSign(jobID string, trigger []byte, executor func([]byte) ([]byte, error)) (*JobExecution, error) {
	e.Lock()
	defer e.Unlock()

	result, err := executor(trigger)
	if err != nil {
		return nil, err
	}

	hash := sha256.New()
	hash.Write([]byte(jobID))
	hash.Write(trigger)
	hash.Write(result)

	signingKey := e.GetSigningKey()
	r, s, err := ecdsa.Sign(rand.Reader, signingKey, hash.Sum(nil))
	if err != nil {
		return nil, err
	}

	job := &JobExecution{
		JobID:     jobID,
		Trigger:   trigger,
		Result:    result,
		Signature: append(r.Bytes(), s.Bytes()...),
	}
	e.jobs[jobID] = job
	return job, nil
}

// VerifyExecution verifies a job execution signature.
func VerifyExecution(job *JobExecution, pubKey []byte) (bool, error) {
	if len(job.Signature) < 64 || len(pubKey) == 0 {
		return false, errors.New("invalid signature or key")
	}

	hash := sha256.New()
	hash.Write([]byte(job.JobID))
	hash.Write(job.Trigger)
	hash.Write(job.Result)

	return sdk.VerifySignature(pubKey, hash.Sum(nil), job.Signature)
}
