// Package supabase provides Secrets-specific database operations.
package supabase

import "time"

// Secret represents an encrypted secret.
type Secret struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	Name           string    `json:"name"`
	EncryptedValue []byte    `json:"encrypted_value"`
	Version        int       `json:"version"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Policy represents an allowed service for a secret.
type Policy struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	SecretName string    `json:"secret_name"`
	ServiceID  string    `json:"service_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// AuditLog represents an audit log entry for secret operations.
type AuditLog struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	SecretName   string    `json:"secret_name"`
	Action       string    `json:"action"` // create, read, update, delete, grant, revoke
	ServiceID    string    `json:"service_id,omitempty"` // Service that accessed the secret
	IPAddress    string    `json:"ip_address,omitempty"`
	UserAgent    string    `json:"user_agent,omitempty"`
	Success      bool      `json:"success"`
	ErrorMessage string    `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}
