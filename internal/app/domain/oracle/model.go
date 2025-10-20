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
type Request struct {
	ID           string
	AccountID    string
	DataSourceID string
	Status       RequestStatus
	Payload      string
	Result       string
	Error        string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	CompletedAt  time.Time
}
