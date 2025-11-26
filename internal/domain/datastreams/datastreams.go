package datastreams

import appds "github.com/R3E-Network/service_layer/internal/app/domain/datastreams"

type (
	Stream       = appds.Stream
	Frame        = appds.Frame
	StreamStatus = appds.StreamStatus
	FrameStatus  = appds.FrameStatus
)

const (
	StreamStatusActive   = appds.StreamStatusActive
	StreamStatusInactive = appds.StreamStatusInactive
	StreamStatusPaused   = appds.StreamStatusPaused

	FrameStatusOK    = appds.FrameStatusOK
	FrameStatusError = appds.FrameStatusError
)
