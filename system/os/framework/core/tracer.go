package service

import "context"

// Tracer starts/finishes spans for observability.
type Tracer interface {
	// StartSpan returns a derived context and a completion callback. The callback
	// must be invoked with the final error (if any) when the operation ends.
	StartSpan(ctx context.Context, name string, attributes map[string]string) (context.Context, func(error))
}

type noopTracer struct{}

func (noopTracer) StartSpan(ctx context.Context, _ string, _ map[string]string) (context.Context, func(error)) {
	return ctx, func(error) {}
}

// NoopTracer is the default tracer used when none is configured.
var NoopTracer Tracer = noopTracer{}
