// Package globalsigner provides the TEE master key management service.
package globalsigner

import (
	"github.com/R3E-Network/service_layer/services/globalsigner/types"
)

// Re-export types from the types package for backward compatibility.
// This allows existing code to continue importing from marble package.

// Service constants
const (
	ServiceID   = types.ServiceID
	ServiceName = types.ServiceName
	Version     = types.Version

	DefaultRotationPeriod = types.DefaultRotationPeriod
	DefaultOverlapPeriod  = types.DefaultOverlapPeriod
)

// KeyStatus type and constants
type KeyStatus = types.KeyStatus

const (
	KeyStatusPending     = types.KeyStatusPending
	KeyStatusActive      = types.KeyStatusActive
	KeyStatusOverlapping = types.KeyStatusOverlapping
	KeyStatusRevoked     = types.KeyStatusRevoked
)

// Type aliases for backward compatibility
type (
	RotationConfig       = types.RotationConfig
	KeyVersion           = types.KeyVersion
	MasterKeyAttestation = types.MasterKeyAttestation
	AttestationArtifact  = types.AttestationArtifact
	RotateRequest        = types.RotateRequest
	RotateResponse       = types.RotateResponse
	SignRequest          = types.SignRequest
	SignResponse         = types.SignResponse
	DeriveRequest        = types.DeriveRequest
	DeriveResponse       = types.DeriveResponse
	StatusResponse       = types.StatusResponse
)

// DefaultRotationConfig returns sensible defaults.
func DefaultRotationConfig() *RotationConfig {
	return types.DefaultRotationConfig()
}
