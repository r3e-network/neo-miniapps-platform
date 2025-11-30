package secrets

import "time"

// ACL defines access control flags for secrets.
// Aligned with SecretsVault.cs contract ACL byte flags.
type ACL byte

const (
	ACLNone             ACL = 0x00 // No service access
	ACLOracleAccess     ACL = 0x01 // Oracle service can access
	ACLAutomationAccess ACL = 0x02 // Automation service can access
	ACLFunctionAccess   ACL = 0x04 // Functions service can access
	ACLJAMAccess        ACL = 0x08 // JAM service can access
)

// HasAccess checks if the given access flag is set.
func (a ACL) HasAccess(flag ACL) bool {
	return a&flag != 0
}

// Secret represents a stored secret. Value holds the encrypted payload.
// Aligned with SecretsVault.cs contract Secret struct.
type Secret struct {
	ID        string
	AccountID string
	Name      string
	Value     string // Encrypted payload (contract stores RefHash for off-chain reference)
	Version   int
	ACL       ACL // Maps to contract ACL byte - access control flags
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Metadata contains public information about a secret without the value.
type Metadata struct {
	ID        string    `json:"id"`
	AccountID string    `json:"account_id"`
	Name      string    `json:"name"`
	Version   int       `json:"version"`
	ACL       ACL       `json:"acl"` // Access control flags
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ToMetadata converts a secret into its metadata view.
func (s Secret) ToMetadata() Metadata {
	return Metadata{
		ID:        s.ID,
		AccountID: s.AccountID,
		Name:      s.Name,
		Version:   s.Version,
		ACL:       s.ACL,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}
