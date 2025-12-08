// Package supabase provides AccountPool-specific database operations.
package supabase

import "time"

// Account represents an account pool account with locking support.
type Account struct {
	ID         string    `json:"id"`
	Address    string    `json:"address"`
	Balance    int64     `json:"balance"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	TxCount    int64     `json:"tx_count"`
	IsRetiring bool      `json:"is_retiring"`
	LockedBy   string    `json:"locked_by,omitempty"`
	LockedAt   time.Time `json:"locked_at,omitempty"`
}
