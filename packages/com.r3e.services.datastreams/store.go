// Package datastreams provides the Data Streams Service as a ServicePackage.
package datastreams

import (
	"context"

	"github.com/R3E-Network/service_layer/system/framework"
)

// Store defines the persistence interface for data streams and frames.
// This interface is defined within the service package, following the principle
// that "everything of the service must be in service package".
type Store interface {
	CreateStream(ctx context.Context, stream Stream) (Stream, error)
	UpdateStream(ctx context.Context, stream Stream) (Stream, error)
	GetStream(ctx context.Context, id string) (Stream, error)
	ListStreams(ctx context.Context, accountID string) ([]Stream, error)

	CreateFrame(ctx context.Context, frame Frame) (Frame, error)
	ListFrames(ctx context.Context, streamID string, limit int) ([]Frame, error)
	GetLatestFrame(ctx context.Context, streamID string) (Frame, error)
}

// AccountChecker is an alias for the canonical framework.AccountChecker interface.
type AccountChecker = framework.AccountChecker
