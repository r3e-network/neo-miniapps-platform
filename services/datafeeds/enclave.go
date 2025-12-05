// Package datafeeds provides data feed aggregation service.
package datafeeds

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

// Enclave handles TEE-protected datafeeds operations.
type Enclave struct {
	*base.BaseEnclave
	signingKey os.KeyHandle
}

// NewEnclave creates a new datafeeds enclave.
func NewEnclave(serviceOS os.ServiceOS) (*Enclave, error) {
	if !serviceOS.HasCapability(os.CapKeys) {
		return nil, errors.New("datafeeds requires keys capability")
	}
	return &Enclave{
		BaseEnclave: base.NewBaseEnclave(ServiceID, serviceOS),
	}, nil
}

// Initialize initializes the enclave.
func (e *Enclave) Initialize(ctx context.Context) error {
	if err := e.BaseEnclave.Initialize(ctx); err != nil {
		return err
	}
	handle, err := e.OS().Keys().DeriveKey(ctx, "datafeeds/signing")
	if err != nil {
		return fmt.Errorf("derive signing key: %w", err)
	}
	e.signingKey = handle
	e.Logger().Info("datafeeds enclave initialized")
	return nil
}

// SignPriceData signs price data inside the TEE.
func (e *Enclave) SignPriceData(ctx context.Context, data *PriceData) ([]byte, error) {
	if !e.IsReady() {
		return nil, errors.New("enclave not ready")
	}
	// Create hash of price data
	h := sha256.New()
	h.Write([]byte(data.Symbol))
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(data.Price*1e8))
	h.Write(buf)
	binary.BigEndian.PutUint64(buf, uint64(data.Timestamp.UnixNano()))
	h.Write(buf)
	hash := h.Sum(nil)

	return e.OS().Keys().Sign(ctx, e.signingKey, hash)
}

// GetPublicKey returns the signing public key.
func (e *Enclave) GetPublicKey(ctx context.Context) ([]byte, error) {
	if !e.IsReady() {
		return nil, errors.New("enclave not ready")
	}
	return e.OS().Keys().GetPublicKey(ctx, e.signingKey)
}
