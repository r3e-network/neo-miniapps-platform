// Package supabase provides VRF-specific database operations.
package supabase

import "time"

// RequestRecord represents a VRF request row.
type RequestRecord struct {
	ID               string    `json:"id"`
	RequestID        string    `json:"request_id"`
	UserID           string    `json:"user_id"`
	RequesterAddress string    `json:"requester_address"`
	Seed             string    `json:"seed"`
	NumWords         int       `json:"num_words"`
	CallbackGasLimit int64     `json:"callback_gas_limit"`
	Status           string    `json:"status"`
	RandomWords      []string  `json:"random_words,omitempty"`
	Proof            string    `json:"proof,omitempty"`
	FulfillTxHash    string    `json:"fulfill_tx_hash,omitempty"`
	Error            string    `json:"error,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	FulfilledAt      time.Time `json:"fulfilled_at,omitempty"`
}
