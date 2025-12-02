package tracing

import (
	"context"
	"strings"

	core "github.com/R3E-Network/service_layer/system/os/framework/core"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// OTelTracer adapts an OpenTelemetry tracer to the framework tracer interface.
type OTelTracer struct {
	tracer oteltrace.Tracer
}

// NewOTelTracer creates a tracer from the provided provider and instrumentation name.
func NewOTelTracer(provider oteltrace.TracerProvider, instrumentation string) core.Tracer {
	if provider == nil {
		provider = otel.GetTracerProvider()
	}
	if provider == nil {
		return core.NoopTracer
	}
	if strings.TrimSpace(instrumentation) == "" {
		instrumentation = "service-layer"
	}
	return &OTelTracer{tracer: provider.Tracer(instrumentation)}
}

// NewGlobalTracer returns a tracer using the global provider with the given name.
func NewGlobalTracer(instrumentation string) core.Tracer {
	return NewOTelTracer(nil, instrumentation)
}

// StartSpan implements core.Tracer using the OpenTelemetry tracer.
func (t *OTelTracer) StartSpan(ctx context.Context, name string, attrs map[string]string) (context.Context, func(error)) {
	if t == nil || t.tracer == nil {
		return ctx, func(error) {}
	}
	ctx, span := t.tracer.Start(ctx, name, oteltrace.WithAttributes(convertAttrs(attrs)...))
	return ctx, func(err error) {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
		}
		span.End()
	}
}

func convertAttrs(attrs map[string]string) []attribute.KeyValue {
	if len(attrs) == 0 {
		return nil
	}
	result := make([]attribute.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		result = append(result, attribute.String(key, v))
	}
	return result
}
