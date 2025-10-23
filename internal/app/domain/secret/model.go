package secret

import "time"

// Secret represents a stored secret. Value holds the encrypted payload.
type Secret struct {
	ID        string
	AccountID string
	Name      string
	Value     string
	Version   int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Metadata contains public information about a secret without the value.
type Metadata struct {
	ID        string    `json:"id"`
	AccountID string    `json:"account_id"`
	Name      string    `json:"name"`
	Version   int       `json:"version"`
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
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}
