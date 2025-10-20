package account

import "time"

// Account represents a logical tenant or owner of resources within the service
// layer.
type Account struct {
	ID        string
	Owner     string
	Metadata  map[string]string
	CreatedAt time.Time
	UpdatedAt time.Time
}
