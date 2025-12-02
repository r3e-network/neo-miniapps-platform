// Package confidential provides the CONFIDENTIAL Service as a ServicePackage.
package confidential

import (
	"context"

	"github.com/R3E-Network/service_layer/system/os/framework"
)

// Store defines the persistence interface for confidential.
type Store interface {
	CreateEnclave(ctx context.Context, enclave Enclave) (Enclave, error)
	UpdateEnclave(ctx context.Context, enclave Enclave) (Enclave, error)
	GetEnclave(ctx context.Context, id string) (Enclave, error)
	ListEnclaves(ctx context.Context, accountID string) ([]Enclave, error)

	CreateSealedKey(ctx context.Context, key SealedKey) (SealedKey, error)
	ListSealedKeys(ctx context.Context, accountID, enclaveID string, limit int) ([]SealedKey, error)

	CreateAttestation(ctx context.Context, att Attestation) (Attestation, error)
	ListAttestations(ctx context.Context, accountID, enclaveID string, limit int) ([]Attestation, error)
	ListAccountAttestations(ctx context.Context, accountID string, limit int) ([]Attestation, error)
}

// AccountChecker is an alias for the canonical framework.AccountChecker interface.
type AccountChecker = framework.AccountChecker
