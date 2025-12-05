// Package ccip provides Cross-Chain Interoperability Protocol service.
package ccip

import (
	"time"

	"github.com/R3E-Network/service_layer/services/base"
)

// MessageStatus represents message status.
type MessageStatus string

const (
	MessageStatusPending   MessageStatus = "pending"
	MessageStatusSent      MessageStatus = "sent"
	MessageStatusConfirmed MessageStatus = "confirmed"
	MessageStatusFailed    MessageStatus = "failed"
)

// CrossChainMessage represents a cross-chain message.
type CrossChainMessage struct {
	base.BaseEntity
	SourceChain string        `json:"source_chain"`
	DestChain   string        `json:"dest_chain"`
	Sender      string        `json:"sender"`
	Receiver    string        `json:"receiver"`
	Payload     []byte        `json:"payload"`
	Signature   []byte        `json:"signature,omitempty"`
	Status      MessageStatus `json:"status"`
	SourceTxHash string       `json:"source_tx_hash,omitempty"`
	DestTxHash   string       `json:"dest_tx_hash,omitempty"`
	Error        string       `json:"error,omitempty"`
	SentAt       time.Time    `json:"sent_at,omitempty"`
	ConfirmedAt  time.Time    `json:"confirmed_at,omitempty"`
}
