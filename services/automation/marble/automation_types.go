package automationmarble

import (
	"encoding/json"
	"time"
)

// TriggerRequest is the request body for creating/updating triggers.
type TriggerRequest struct {
	Name        string          `json:"name"`
	TriggerType string          `json:"trigger_type"`
	Schedule    string          `json:"schedule,omitempty"`
	Condition   json.RawMessage `json:"condition,omitempty"`
	Action      json.RawMessage `json:"action"`
}

// TriggerResponse is the response body for trigger operations.
type TriggerResponse struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	TriggerType   string          `json:"trigger_type"`
	Schedule      string          `json:"schedule,omitempty"`
	Condition     json.RawMessage `json:"condition,omitempty"`
	Action        json.RawMessage `json:"action"`
	Enabled       bool            `json:"enabled"`
	LastExecution *time.Time      `json:"last_execution,omitempty"`
	NextExecution *time.Time      `json:"next_execution,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// Action describes a trigger action payload.
type Action struct {
	Type   string          `json:"type"`
	URL    string          `json:"url,omitempty"`
	Method string          `json:"method,omitempty"`
	Body   json.RawMessage `json:"body,omitempty"`
}

// PriceCondition represents a price-based trigger condition.
type PriceCondition struct {
	FeedID    string `json:"feed_id"`
	Operator  string `json:"operator"` // ">", "<", ">=", "<=", "=="
	Threshold int64  `json:"threshold"`
}

// ThresholdCondition represents a threshold-based trigger condition.
type ThresholdCondition struct {
	Address   string `json:"address"`
	Asset     string `json:"asset"` // "GAS", "NEO", or contract hash
	Operator  string `json:"operator"`
	Threshold int64  `json:"threshold"`
}
