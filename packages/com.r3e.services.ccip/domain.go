package ccip

import "time"

// Lane describes an allowed CCIP route for an account.
type Lane struct {
	ID             string            `json:"id"`
	AccountID      string            `json:"account_id"`
	Name           string            `json:"name"`
	SourceChain    string            `json:"source_chain"`
	DestChain      string            `json:"dest_chain"`
	SignerSet      []string          `json:"signer_set,omitempty"`
	AllowedTokens  []string          `json:"allowed_tokens,omitempty"`
	DeliveryPolicy map[string]any    `json:"delivery_policy,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	Tags           []string          `json:"tags,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// MessageStatus represents CCIP message lifecycle states.
type MessageStatus string

const (
	MessageStatusPending     MessageStatus = "pending"
	MessageStatusDispatching MessageStatus = "dispatching"
	MessageStatusDelivered   MessageStatus = "delivered"
	MessageStatusFailed      MessageStatus = "failed"
)

// TokenTransfer captures a token move associated with a message.
type TokenTransfer struct {
	Token     string `json:"token"`
	Amount    string `json:"amount"`
	Recipient string `json:"recipient"`
}

// Message represents a cross-chain message queued through CCIP.
type Message struct {
	ID             string            `json:"id"`
	AccountID      string            `json:"account_id"`
	LaneID         string            `json:"lane_id"`
	Status         MessageStatus     `json:"status"`
	Payload        map[string]any    `json:"payload,omitempty"`
	TokenTransfers []TokenTransfer   `json:"token_transfers,omitempty"`
	Trace          []string          `json:"trace,omitempty"`
	Error          string            `json:"error,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	Tags           []string          `json:"tags,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	DeliveredAt    *time.Time        `json:"delivered_at,omitempty"`
}
