package service

import "context"

// DispatchOptions centralizes tracer / hooks / retry wiring for dispatcher-style
// service operations so individual services don't have to reimplement the same
// pattern.
type DispatchOptions struct {
	tracer Tracer
	hooks  ObservationHooks
	retry  RetryPolicy
}

// NewDispatchOptions returns a DispatchOptions with safe defaults.
func NewDispatchOptions() DispatchOptions {
	return DispatchOptions{
		tracer: NoopTracer,
		hooks:  NoopDispatchHooks,
		retry:  DefaultRetryPolicy,
	}
}

// SetTracer configures the tracer used for dispatch spans.
func (o *DispatchOptions) SetTracer(tracer Tracer) {
	if tracer == nil {
		o.tracer = NoopTracer
		return
	}
	o.tracer = tracer
}

// Tracer returns the configured tracer (primarily for tests).
func (o DispatchOptions) Tracer() Tracer {
	return o.tracer
}

// SetHooks configures observation hooks for dispatch attempts.
func (o *DispatchOptions) SetHooks(h ObservationHooks) {
	if h.OnStart == nil && h.OnComplete == nil {
		o.hooks = NoopDispatchHooks
		return
	}
	o.hooks = h
}

// SetRetry configures the retry policy for dispatch attempts.
func (o *DispatchOptions) SetRetry(policy RetryPolicy) {
	if policy.Attempts <= 0 {
		o.retry = DefaultRetryPolicy
		return
	}
	o.retry = policy
}

// Run executes fn with the configured tracer/hooks/retry semantics.
func (o DispatchOptions) Run(ctx context.Context, span string, attrs map[string]string, fn func(context.Context) error) error {
	spanCtx, finishSpan := o.tracer.StartSpan(ctx, span, attrs)
	finishHooks := StartDispatch(spanCtx, o.hooks, attrs)
	err := Retry(spanCtx, o.retry, func() error {
		return fn(spanCtx)
	})
	finishHooks(err)
	finishSpan(err)
	return err
}
