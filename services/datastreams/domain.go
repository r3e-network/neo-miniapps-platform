// Package datastreams provides real-time data streaming service.
package datastreams

import (
	"time"

	"github.com/R3E-Network/service_layer/services/base"
)

// StreamStatus represents stream status.
type StreamStatus string

const (
	StreamStatusActive   StreamStatus = "active"
	StreamStatusPaused   StreamStatus = "paused"
	StreamStatusStopped  StreamStatus = "stopped"
)

// DataStream represents a data stream configuration.
type DataStream struct {
	base.BaseEntity
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	SourceType  string            `json:"source_type"`
	SourceURL   string            `json:"source_url"`
	Status      StreamStatus      `json:"status"`
	Interval    time.Duration     `json:"interval"`
	LastDataAt  time.Time         `json:"last_data_at,omitempty"`
	DataCount   int64             `json:"data_count"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// StreamData represents data from a stream.
type StreamData struct {
	StreamID  string         `json:"stream_id"`
	Data      map[string]any `json:"data"`
	Timestamp time.Time      `json:"timestamp"`
	Signature []byte         `json:"signature,omitempty"`
}
