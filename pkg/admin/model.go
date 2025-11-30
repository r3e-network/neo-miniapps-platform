// Package admin provides administrative configuration types.
package admin

import "time"

// ChainRPC represents a blockchain RPC endpoint configuration.
type ChainRPC struct {
	ID          string            `json:"id"`
	ChainID     string            `json:"chain_id"`
	Name        string            `json:"name"`
	RPCURL      string            `json:"rpc_url"`
	WSURL       string            `json:"ws_url"`
	ChainType   string            `json:"chain_type"`
	NetworkID   int64             `json:"network_id"`
	Priority    int               `json:"priority"`
	Weight      int               `json:"weight"`
	MaxRPS      int               `json:"max_rps"`
	Timeout     int               `json:"timeout_ms"`
	Enabled     bool              `json:"enabled"`
	Healthy     bool              `json:"healthy"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	LastCheckAt time.Time         `json:"last_check_at"`
}

// DataProvider represents an external data source provider configuration.
type DataProvider struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	BaseURL     string            `json:"base_url"`
	APIKey      string            `json:"api_key"`
	RateLimit   int               `json:"rate_limit"`
	Timeout     int               `json:"timeout_ms"`
	Retries     int               `json:"retries"`
	Enabled     bool              `json:"enabled"`
	Healthy     bool              `json:"healthy"`
	Features    []string          `json:"features"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	LastCheckAt time.Time         `json:"last_check_at"`
}

// SystemSetting represents a key-value system configuration.
type SystemSetting struct {
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Type        string    `json:"type"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	Editable    bool      `json:"editable"`
	UpdatedAt   time.Time `json:"updated_at"`
	UpdatedBy   string    `json:"updated_by"`
}

// FeatureFlag represents a feature toggle.
type FeatureFlag struct {
	Key         string    `json:"key"`
	Enabled     bool      `json:"enabled"`
	Description string    `json:"description"`
	Rollout     int       `json:"rollout"`
	UpdatedAt   time.Time `json:"updated_at"`
	UpdatedBy   string    `json:"updated_by"`
}

// TenantQuota represents resource quotas for a tenant.
type TenantQuota struct {
	TenantID     string    `json:"tenant_id"`
	MaxAccounts  int       `json:"max_accounts"`
	MaxFunctions int       `json:"max_functions"`
	MaxRPCPerMin int       `json:"max_rpc_per_min"`
	MaxStorage   int64     `json:"max_storage_bytes"`
	MaxGasPerDay int64     `json:"max_gas_per_day"`
	Features     []string  `json:"features"`
	UpdatedAt    time.Time `json:"updated_at"`
	UpdatedBy    string    `json:"updated_by"`
}

// AllowedMethod defines which RPC methods are allowed per chain.
type AllowedMethod struct {
	ChainID string   `json:"chain_id"`
	Methods []string `json:"methods"`
}
