package system

import "context"

// Lifecycle implements the LifecycleService interface and can be embedded into services
// to provide default no-op start/stop/readiness handling. Override methods as needed.
type Lifecycle struct{}

func (Lifecycle) Name() string { return "" }

func (Lifecycle) Start(ctx context.Context) error {
	_ = ctx
	return nil
}

func (Lifecycle) Stop(ctx context.Context) error {
	_ = ctx
	return nil
}

func (Lifecycle) Ready(ctx context.Context) error {
	_ = ctx
	return nil
}
