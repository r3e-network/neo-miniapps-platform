// Package functions provides adapter interfaces for cross-service communication.
// These interfaces decouple the Functions service from concrete service implementations.
package functions

import (
	"context"
	"time"
)

// =============================================================================
// Service Adapter Interfaces for ActionProcessor implementations
// =============================================================================

// AutomationAdapter provides automation job scheduling capabilities.
type AutomationAdapter interface {
	CreateJob(ctx context.Context, accountID, functionID, name, schedule, description string) (AutomationJob, error)
	UpdateJob(ctx context.Context, jobID string, name, schedule, description *string, enabled *bool) (AutomationJob, error)
	SetEnabled(ctx context.Context, jobID string, enabled bool) (AutomationJob, error)
}

// AutomationJob represents a scheduled automation job.
type AutomationJob struct {
	ID          string
	AccountID   string
	FunctionID  string
	Name        string
	Description string
	Schedule    string
	Enabled     bool
}

// OracleAdapter provides oracle data request capabilities.
type OracleAdapter interface {
	CreateRequest(ctx context.Context, accountID, dataSourceID, payload string) (OracleRequest, error)
}

// OracleRequest represents an oracle data request.
type OracleRequest struct {
	ID           string
	AccountID    string
	DataSourceID string
	Payload      string
	Result       string
	Status       string
}

// DataFeedsAdapter provides data feed submission capabilities.
type DataFeedsAdapter interface {
	SubmitUpdate(ctx context.Context, accountID, feedID string, roundID int64, price string, ts time.Time, signer, signature string, metadata map[string]string) (DataFeedUpdate, error)
}

// DataFeedUpdate represents a submitted data feed update.
type DataFeedUpdate struct {
	ID        string
	FeedID    string
	RoundID   int64
	Price     string
	Timestamp time.Time
	Signer    string
	Signature string
	Metadata  map[string]string
}

// DataStreamsAdapter provides data stream publishing capabilities.
type DataStreamsAdapter interface {
	CreateFrame(ctx context.Context, accountID, streamID string, sequence int64, payload map[string]any, latencyMs int, status string, metadata map[string]string) (DataStreamFrame, error)
}

// DataStreamFrame represents a published data stream frame.
type DataStreamFrame struct {
	ID        string
	StreamID  string
	Sequence  int64
	Payload   map[string]any
	LatencyMs int
	Status    string
	Metadata  map[string]string
}

// DataLinkAdapter provides data link delivery capabilities.
type DataLinkAdapter interface {
	CreateDelivery(ctx context.Context, accountID, channelID string, payload map[string]any, metadata map[string]string) (DataLinkDelivery, error)
}

// DataLinkDelivery represents a data link delivery.
type DataLinkDelivery struct {
	ID        string
	ChannelID string
	Payload   map[string]any
	Metadata  map[string]string
	Status    string
}

// VRFAdapter provides verifiable random function capabilities.
type VRFAdapter interface {
	RequestRandomness(ctx context.Context, accountID, keyID string, seed []byte) (VRFResult, error)
}

// VRFResult represents a VRF randomness result.
type VRFResult struct {
	RequestID string
	Output    []byte
	Proof     []byte
}
