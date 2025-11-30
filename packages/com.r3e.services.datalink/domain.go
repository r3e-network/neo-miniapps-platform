package datalink

import "time"

// ChannelStatus enumerates datalink channel states.
type ChannelStatus string

const (
	ChannelStatusInactive  ChannelStatus = "inactive"
	ChannelStatusActive    ChannelStatus = "active"
	ChannelStatusSuspended ChannelStatus = "suspended"
)

// Channel describes a data provider configuration.
type Channel struct {
	ID        string            `json:"id"`
	AccountID string            `json:"account_id"`
	Name      string            `json:"name"`
	Endpoint  string            `json:"endpoint"`
	AuthToken string            `json:"auth_token"`
	SignerSet []string          `json:"signer_set,omitempty"`
	Status    ChannelStatus     `json:"status"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// DeliveryStatus enumerates delivery lifecycle states.
type DeliveryStatus string

const (
	DeliveryStatusPending    DeliveryStatus = "pending"
	DeliveryStatusDispatched DeliveryStatus = "dispatched"
	DeliveryStatusSucceeded  DeliveryStatus = "succeeded"
	DeliveryStatusFailed     DeliveryStatus = "failed"
)

// Delivery represents a datalink delivery request.
type Delivery struct {
	ID        string            `json:"id"`
	AccountID string            `json:"account_id"`
	ChannelID string            `json:"channel_id"`
	Payload   map[string]any    `json:"payload,omitempty"`
	Attempts  int               `json:"attempts"`
	Status    DeliveryStatus    `json:"status"`
	Error     string            `json:"error"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}
