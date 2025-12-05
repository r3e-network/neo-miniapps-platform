// Package confidential provides confidential computing service.
package confidential

import (
	"context"
	"errors"
	"time"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

// Enclave handles TEE-protected confidential operations.
type Enclave struct {
	*base.BaseEnclave
}

// NewEnclave creates a new confidential enclave.
func NewEnclave(serviceOS os.ServiceOS) *Enclave {
	return &Enclave{
		BaseEnclave: base.NewBaseEnclave(ServiceID, serviceOS),
	}
}

// Config represents sealed configuration for confidential compute.
type Config struct {
	AllowedHosts []string `json:"allowed_hosts,omitempty"`
}

// Execute executes confidential computation inside the TEE.
func (e *Enclave) Execute(ctx context.Context, req *ComputeRequest) (*ComputeResult, error) {
	if !e.IsReady() {
		return nil, errors.New("enclave not ready")
	}

	startTime := time.Now()

	// Execute computation in TEE
	osReq := os.ComputeRequest{
		Script:     req.Script,
		EntryPoint: req.EntryPoint,
		Input:      req.Input,
		Timeout:    req.Timeout,
	}

	var result *os.ComputeResult
	var err error

	if len(req.SecretNames) > 0 {
		result, err = e.ExecuteWithSecrets(ctx, osReq, req.SecretNames)
	} else {
		result, err = e.BaseEnclave.Execute(ctx, osReq)
	}

	if err != nil {
		return &ComputeResult{
			RequestID: req.ID,
			Success:   false,
			Error:     err.Error(),
			Duration:  time.Since(startTime),
		}, nil
	}

	return &ComputeResult{
		RequestID: req.ID,
		Success:   result.Success,
		Output:    result.Output,
		Logs:      result.Logs,
		Error:     result.Error,
		Duration:  time.Since(startTime),
	}, nil
}
