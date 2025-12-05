// Package accounts provides account management service.
package accounts

import (
	"time"

	"github.com/R3E-Network/service_layer/services/base"
)

// AccountStatus represents account status.
type AccountStatus string

const (
	AccountStatusActive    AccountStatus = "active"
	AccountStatusSuspended AccountStatus = "suspended"
	AccountStatusClosed    AccountStatus = "closed"
)

// Account represents a user account.
type Account struct {
	base.BaseEntity
	Email       string            `json:"email"`
	Name        string            `json:"name"`
	Status      AccountStatus     `json:"status"`
	Tier        string            `json:"tier"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	LastLoginAt time.Time         `json:"last_login_at,omitempty"`
}

// APIKey represents an API key for an account.
type APIKey struct {
	base.BaseEntity
	AccountID   string    `json:"account_id"`
	Name        string    `json:"name"`
	KeyHash     string    `json:"key_hash"`
	Permissions []string  `json:"permissions"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
	LastUsedAt  time.Time `json:"last_used_at,omitempty"`
}
