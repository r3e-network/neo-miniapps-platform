// Package datastreams provides real-time data streaming service.
package datastreams

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

// Enclave handles TEE-protected DataStreams operations.
type Enclave struct {
	*base.BaseEnclave
	signingKey os.KeyHandle
}

// NewEnclave creates a new DataStreams enclave.
func NewEnclave(serviceOS os.ServiceOS) *Enclave {
	return &Enclave{
		BaseEnclave: base.NewBaseEnclave(ServiceID, serviceOS),
	}
}

// Initialize initializes the enclave.
func (e *Enclave) Initialize(ctx context.Context) error {
	if err := e.BaseEnclave.Initialize(ctx); err != nil {
		return err
	}
	if e.OS().HasCapability(os.CapKeys) {
		handle, err := e.OS().Keys().DeriveKey(ctx, "datastreams/signing")
		if err != nil {
			return fmt.Errorf("derive signing key: %w", err)
		}
		e.signingKey = handle
	}
	e.Logger().Info("datastreams enclave initialized")
	return nil
}

// SignData signs stream data inside the TEE.
func (e *Enclave) SignData(ctx context.Context, data *StreamData) ([]byte, error) {
	if !e.IsReady() {
		return nil, errors.New("enclave not ready")
	}
	if e.signingKey == "" {
		return nil, errors.New("signing key not available")
	}

	// Serialize and hash data
	jsonData, err := json.Marshal(data.Data)
	if err != nil {
		return nil, err
	}

	h := sha256.New()
	h.Write([]byte(data.StreamID))
	h.Write(jsonData)
	hash := h.Sum(nil)

	return e.OS().Keys().Sign(ctx, e.signingKey, hash)
}
