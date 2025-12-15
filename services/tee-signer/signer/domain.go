// Package signer provides domain-separated signing for the TEE signer service.
package signer

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"time"
)

// SigningPurpose defines the purpose of a signature for domain separation.
type SigningPurpose string

const (
	// PurposeOracle is for oracle/datafeed result signatures.
	PurposeOracle SigningPurpose = "oracle"

	// PurposeVRF is for VRF result signatures.
	PurposeVRF SigningPurpose = "vrf"

	// PurposeAutomation is for automation/flow callback signatures.
	PurposeAutomation SigningPurpose = "automation"

	// PurposePrivacy is for privacy/mixer operation signatures.
	PurposePrivacy SigningPurpose = "privacy"

	// PurposeRegistry is for key registration/rotation signatures.
	PurposeRegistry SigningPurpose = "registry"

	// PurposeAccountPool is for account pool transaction signatures.
	PurposeAccountPool SigningPurpose = "accountpool"
)

// ValidPurposes is the set of valid signing purposes.
var ValidPurposes = map[SigningPurpose]bool{
	PurposeOracle:      true,
	PurposeVRF:         true,
	PurposeAutomation:  true,
	PurposePrivacy:     true,
	PurposeRegistry:    true,
	PurposeAccountPool: true,
}

// DomainSeparatedRequest contains all fields required for domain-separated signing.
type DomainSeparatedRequest struct {
	// Purpose identifies the signing purpose (oracle, vrf, automation, etc.)
	Purpose SigningPurpose `json:"purpose"`

	// ServiceID identifies the requesting service
	ServiceID string `json:"service_id"`

	// ChainID identifies the target blockchain network
	ChainID string `json:"chain_id"`

	// RequestID is a unique identifier for the request (for idempotency)
	RequestID string `json:"request_id"`

	// PayloadHash is the hash of the payload to sign (hex-encoded)
	PayloadHash string `json:"payload_hash"`

	// Expiry is the signature expiration time (Unix timestamp)
	Expiry int64 `json:"expiry"`

	// KeyVersion is the optional key version to use
	KeyVersion string `json:"key_version,omitempty"`
}

// DomainSeparatedResponse contains the signature and metadata.
type DomainSeparatedResponse struct {
	// Signature is the domain-separated signature
	Signature []byte `json:"signature"`

	// KeyVersion is the key version used for signing
	KeyVersion string `json:"key_version"`

	// SignedAt is when the signature was created
	SignedAt time.Time `json:"signed_at"`

	// DomainHash is the hash of the domain separation data
	DomainHash []byte `json:"domain_hash"`
}

// Validate validates the domain-separated request.
func (r *DomainSeparatedRequest) Validate() error {
	if r.Purpose == "" {
		return fmt.Errorf("purpose is required")
	}
	if !ValidPurposes[r.Purpose] {
		return fmt.Errorf("invalid purpose: %s", r.Purpose)
	}
	if r.ServiceID == "" {
		return fmt.Errorf("service_id is required")
	}
	if r.ChainID == "" {
		return fmt.Errorf("chain_id is required")
	}
	if r.RequestID == "" {
		return fmt.Errorf("request_id is required")
	}
	if r.PayloadHash == "" {
		return fmt.Errorf("payload_hash is required")
	}
	if len(r.PayloadHash) != 64 {
		return fmt.Errorf("payload_hash must be 32 bytes (64 hex chars)")
	}
	if r.Expiry <= 0 {
		return fmt.Errorf("expiry is required and must be positive")
	}
	if r.Expiry < time.Now().Unix() {
		return fmt.Errorf("expiry must be in the future")
	}

	return nil
}

// ComputeDomainHash computes the domain separation hash.
// Format: SHA256(purpose || serviceId || chainId || requestId || payloadHash || expiry)
func (r *DomainSeparatedRequest) ComputeDomainHash() ([]byte, error) {
	h := sha256.New()

	// Write purpose
	h.Write([]byte(r.Purpose))
	h.Write([]byte{0}) // null separator

	// Write serviceId
	h.Write([]byte(r.ServiceID))
	h.Write([]byte{0})

	// Write chainId
	h.Write([]byte(r.ChainID))
	h.Write([]byte{0})

	// Write requestId
	h.Write([]byte(r.RequestID))
	h.Write([]byte{0})

	// Write payloadHash (as bytes, not hex string)
	payloadBytes, err := decodeHex(r.PayloadHash)
	if err != nil {
		return nil, fmt.Errorf("invalid payload_hash: %w", err)
	}
	h.Write(payloadBytes)

	// Write expiry as big-endian uint64
	expiryBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(expiryBytes, uint64(r.Expiry))
	h.Write(expiryBytes)

	return h.Sum(nil), nil
}

// decodeHex decodes a hex string to bytes.
func decodeHex(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		return nil, fmt.Errorf("odd length hex string")
	}

	result := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		b, err := hexByte(s[i], s[i+1])
		if err != nil {
			return nil, err
		}
		result[i/2] = b
	}
	return result, nil
}

func hexByte(hi, lo byte) (byte, error) {
	h, err := hexNibble(hi)
	if err != nil {
		return 0, err
	}
	l, err := hexNibble(lo)
	if err != nil {
		return 0, err
	}
	return (h << 4) | l, nil
}

func hexNibble(c byte) (byte, error) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', nil
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, nil
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, nil
	default:
		return 0, fmt.Errorf("invalid hex character: %c", c)
	}
}

// DomainSeparationPrefix returns the domain separation prefix for on-chain verification.
// This prefix should be used by smart contracts to verify signatures.
const DomainSeparationPrefix = "NEO-SERVICE-LAYER-V1"

// ComputeSigningMessage computes the final message to sign.
// Format: SHA256(DomainSeparationPrefix || domainHash)
func ComputeSigningMessage(domainHash []byte) []byte {
	h := sha256.New()
	h.Write([]byte(DomainSeparationPrefix))
	h.Write(domainHash)
	return h.Sum(nil)
}
