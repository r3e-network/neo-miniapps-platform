package datastreams

import "time"

// StreamStatus enumerates DS stream states.
type StreamStatus string

const (
	StreamStatusInactive StreamStatus = "inactive"
	StreamStatusActive   StreamStatus = "active"
	StreamStatusPaused   StreamStatus = "paused"
)

// Stream represents a high-frequency data channel.
type Stream struct {
	ID          string            `json:"id"`
	AccountID   string            `json:"account_id"`
	Name        string            `json:"name"`
	Symbol      string            `json:"symbol"`
	Description string            `json:"description"`
	Frequency   string            `json:"frequency"`
	SLAms       int               `json:"sla_ms"`
	Status      StreamStatus      `json:"status"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// FrameStatus enumerates frame outcomes.
type FrameStatus string

const (
	FrameStatusOK    FrameStatus = "ok"
	FrameStatusLate  FrameStatus = "late"
	FrameStatusError FrameStatus = "error"
)

// Frame captures a stream sample.
type Frame struct {
	ID        string            `json:"id"`
	AccountID string            `json:"account_id"`
	StreamID  string            `json:"stream_id"`
	Sequence  int64             `json:"sequence"`
	Payload   map[string]any    `json:"payload,omitempty"`
	LatencyMS int               `json:"latency_ms"`
	Status    FrameStatus       `json:"status"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}
