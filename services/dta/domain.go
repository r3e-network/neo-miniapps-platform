// Package dta provides Data Trust Authority service.
package dta

import (
	"time"

	"github.com/R3E-Network/service_layer/services/base"
)

// CertStatus represents certificate status.
type CertStatus string

const (
	CertStatusValid   CertStatus = "valid"
	CertStatusRevoked CertStatus = "revoked"
	CertStatusExpired CertStatus = "expired"
)

// Certificate represents a data certificate.
type Certificate struct {
	base.BaseEntity
	DataHash    []byte            `json:"data_hash"`
	Signature   []byte            `json:"signature"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Status      CertStatus        `json:"status"`
	IssuedAt    time.Time         `json:"issued_at"`
	ExpiresAt   time.Time         `json:"expires_at"`
	RevokedAt   time.Time         `json:"revoked_at,omitempty"`
	RevokeReason string           `json:"revoke_reason,omitempty"`
}

// IsValid checks if the certificate is valid.
func (c *Certificate) IsValid() bool {
	if c.Status != CertStatusValid {
		return false
	}
	if time.Now().After(c.ExpiresAt) {
		return false
	}
	return true
}

// CertificateRequest represents a certificate issuance request.
type CertificateRequest struct {
	DataHash []byte            `json:"data_hash"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// VerificationResult represents certificate verification result.
type VerificationResult struct {
	Valid       bool      `json:"valid"`
	Certificate *Certificate `json:"certificate,omitempty"`
	Error       string    `json:"error,omitempty"`
	VerifiedAt  time.Time `json:"verified_at"`
}
