package service

import (
	"context"
	"time"
)

// ObservationHooks captures optional callbacks for arbitrary operations.
type ObservationHooks struct {
	OnStart    func(ctx context.Context, meta map[string]string)
	OnComplete func(ctx context.Context, meta map[string]string, err error, duration time.Duration)
}

// NoopObservationHooks provides a safe default.
var NoopObservationHooks = ObservationHooks{}

// StartObservation triggers OnStart and returns a completion callback for OnComplete.
func StartObservation(ctx context.Context, hooks ObservationHooks, meta map[string]string) func(error) {
	if hooks.OnStart != nil {
		hooks.OnStart(ctx, meta)
	}
	start := time.Now()
	return func(err error) {
		if hooks.OnComplete != nil {
			hooks.OnComplete(ctx, meta, err, time.Since(start))
		}
	}
}

// Deprecated: DispatchHooks is an alias for ObservationHooks.
// Use ObservationHooks directly in new code.
type DispatchHooks = ObservationHooks

// Deprecated: NoopDispatchHooks is an alias for NoopObservationHooks.
// Use NoopObservationHooks directly in new code.
var NoopDispatchHooks = NoopObservationHooks

// Deprecated: StartDispatch is an alias for StartObservation.
// Use StartObservation directly in new code.
func StartDispatch(ctx context.Context, hooks DispatchHooks, meta map[string]string) func(error) {
	return StartObservation(ctx, hooks, meta)
}

// NormalizeHooks returns NoopObservationHooks if both callbacks are nil,
// otherwise returns the provided hooks. This eliminates boilerplate in
// WithObservationHooks implementations.
func NormalizeHooks(h ObservationHooks) ObservationHooks {
	if h.OnStart == nil && h.OnComplete == nil {
		return NoopObservationHooks
	}
	return h
}
