// Package datalink provides data linking service.
package datalink

import (
	"time"

	"github.com/R3E-Network/service_layer/services/base"
)

// LinkStatus represents link status.
type LinkStatus string

const (
	LinkStatusActive   LinkStatus = "active"
	LinkStatusInactive LinkStatus = "inactive"
	LinkStatusError    LinkStatus = "error"
)

// DataLink represents a data link configuration.
type DataLink struct {
	base.BaseEntity
	Name        string            `json:"name"`
	SourceURL   string            `json:"source_url"`
	TargetURL   string            `json:"target_url"`
	Status      LinkStatus        `json:"status"`
	Interval    time.Duration     `json:"interval"`
	LastSyncAt  time.Time         `json:"last_sync_at,omitempty"`
	ErrorCount  int               `json:"error_count"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// SyncResult represents sync result.
type SyncResult struct {
	LinkID      string    `json:"link_id"`
	Success     bool      `json:"success"`
	RecordCount int       `json:"record_count"`
	Error       string    `json:"error,omitempty"`
	SyncedAt    time.Time `json:"synced_at"`
}
