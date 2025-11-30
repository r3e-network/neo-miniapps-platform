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

// DispatchHooks is retained for backwards compatibility with dispatcher-specific code.
type DispatchHooks = ObservationHooks

// NoopDispatchHooks provides a safe default for dispatchers.
var NoopDispatchHooks = NoopObservationHooks

// StartDispatch triggers dispatcher-specific hooks and defers to StartObservation.
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
