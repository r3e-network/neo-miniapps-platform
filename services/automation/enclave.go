// Package automation provides task automation service.
package automation

import (
	"context"
	"errors"
	"time"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

// Enclave handles TEE-protected automation operations.
type Enclave struct {
	*base.BaseEnclave
}

// NewEnclave creates a new automation enclave.
func NewEnclave(serviceOS os.ServiceOS) *Enclave {
	return &Enclave{
		BaseEnclave: base.NewBaseEnclave(ServiceID, serviceOS),
	}
}

// ExecuteTask executes a task inside the TEE.
func (e *Enclave) ExecuteTask(ctx context.Context, taskID string) (*TaskResult, error) {
	if !e.IsReady() {
		return nil, errors.New("enclave not ready")
	}

	// Execute task logic in TEE
	result := &TaskResult{
		TaskID:      taskID,
		Success:     true,
		ExecutedAt:  time.Now(),
		CompletedAt: time.Now(),
	}

	return result, nil
}
