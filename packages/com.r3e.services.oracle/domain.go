// Package oracle provides oracle data source and request management.
// This file contains domain models that are self-contained within the service package,
// following the Android OS pattern where each service owns its domain definitions.
package oracle

import "time"

// DataSource represents a configured oracle data source.
type DataSource struct {
	ID          string
	AccountID   string
	Name        string
	Description string
	URL         string
	Method      string
	Headers     map[string]string
	Body        string
	Enabled     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// RequestStatus enumerates the lifecycle of an oracle request.
type RequestStatus string

const (
	StatusPending   RequestStatus = "pending"
	StatusRunning   RequestStatus = "running"
	StatusSucceeded RequestStatus = "succeeded"
	StatusFailed    RequestStatus = "failed"
)

// Request represents a single oracle execution.
// Aligned with OracleHub.cs contract Request struct.
type Request struct {
	ID           string
	AccountID    string
	DataSourceID string // Maps to contract ServiceId
	Status       RequestStatus
	Attempts     int
	Fee          int64  // Maps to contract Fee - request fee in smallest unit
	Payload      string // Maps to contract PayloadHash (Go stores full payload)
	Result       string // Maps to contract ResultHash (Go stores full result)
	Error        string
	CreatedAt    time.Time // Maps to contract RequestedAt
	UpdatedAt    time.Time
	CompletedAt  time.Time // Maps to contract FulfilledAt
}
