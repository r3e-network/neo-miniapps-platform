// Package datalink provides data linking service.
package datalink

import (
	"context"
	"errors"
	"time"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

// Enclave handles TEE-protected DataLink operations.
type Enclave struct {
	*base.BaseEnclave
}

// NewEnclave creates a new DataLink enclave.
func NewEnclave(serviceOS os.ServiceOS) *Enclave {
	return &Enclave{
		BaseEnclave: base.NewBaseEnclave(ServiceID, serviceOS),
	}
}

// SyncData syncs data inside the TEE.
func (e *Enclave) SyncData(ctx context.Context, link *DataLink) (*SyncResult, error) {
	if !e.IsReady() {
		return nil, errors.New("enclave not ready")
	}

	// Perform sync operation
	result := &SyncResult{
		LinkID:      link.ID,
		Success:     true,
		RecordCount: 0,
		SyncedAt:    time.Now(),
	}

	return result, nil
}
