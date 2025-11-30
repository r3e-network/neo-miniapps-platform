package confidential

import "time"

// EnclaveStatus enumerates enclave lifecycle states.
type EnclaveStatus string

const (
	EnclaveStatusInactive EnclaveStatus = "inactive"
	EnclaveStatusActive   EnclaveStatus = "active"
	EnclaveStatusRevoked  EnclaveStatus = "revoked"
)

// Enclave represents a registered TEE runner.
type Enclave struct {
	ID          string            `json:"id"`
	AccountID   string            `json:"account_id"`
	Name        string            `json:"name"`
	Endpoint    string            `json:"endpoint"`
	Attestation string            `json:"attestation"`
	Status      EnclaveStatus     `json:"status"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// SealedKey links confidential keys to enclaves/accounts.
type SealedKey struct {
	ID        string            `json:"id"`
	AccountID string            `json:"account_id"`
	EnclaveID string            `json:"enclave_id"`
	Name      string            `json:"name"`
	Blob      []byte            `json:"blob"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

// Attestation captures an enclave attestation proof.
type Attestation struct {
	ID         string            `json:"id"`
	AccountID  string            `json:"account_id"`
	EnclaveID  string            `json:"enclave_id"`
	Report     string            `json:"report"`
	ValidUntil *time.Time        `json:"valid_until,omitempty"`
	Status     string            `json:"status"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
}
