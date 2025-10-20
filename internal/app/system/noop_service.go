package system

import "context"

// NoopService is a convenient implementation of Service for modules that do
// not require background processing. Embed or use directly when the lifecycle
// hooks are optional.
type NoopService struct {
	ServiceName string
}

func (n NoopService) Name() string { return n.ServiceName }

func (NoopService) Start(context.Context) error { return nil }

func (NoopService) Stop(context.Context) error { return nil }
