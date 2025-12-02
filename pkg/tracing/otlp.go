package tracing

import (
	"context"
	"fmt"
	"strings"

	core "github.com/R3E-Network/service_layer/system/os/framework/core"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
)

// OTLPConfig configures the OTLP tracing exporter.
type OTLPConfig struct {
	Endpoint           string
	Insecure           bool
	ServiceName        string
	ResourceAttributes map[string]string
}

// NewOTLPTracerProvider builds an OTLP gRPC tracer provider and returns it
// along with a shutdown function that should be invoked during application shutdown.
func NewOTLPTracerProvider(ctx context.Context, cfg OTLPConfig) (trace.TracerProvider, func(context.Context) error, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		return nil, nil, fmt.Errorf("otlp endpoint required")
	}

	clientOpts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
	}
	if cfg.Insecure {
		clientOpts = append(clientOpts, otlptracegrpc.WithInsecure())
	} else {
		clientOpts = append(clientOpts, otlptracegrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, "")))
	}

	exporter, err := otlptrace.New(ctx, otlptracegrpc.NewClient(clientOpts...))
	if err != nil {
		return nil, nil, fmt.Errorf("create otlp exporter: %w", err)
	}

	resAttrs := []attribute.KeyValue{}
	serviceName := strings.TrimSpace(cfg.ServiceName)
	if serviceName == "" {
		serviceName = "service-layer"
	}
	resAttrs = append(resAttrs, semconv.ServiceName(serviceName))
	for k, v := range cfg.ResourceAttributes {
		if key := strings.TrimSpace(k); key != "" {
			resAttrs = append(resAttrs, attribute.String(key, v))
		}
	}

	res, err := resource.New(ctx, resource.WithAttributes(resAttrs...))
	if err != nil {
		return nil, nil, fmt.Errorf("create resource: %w", err)
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	shutdown := func(ctx context.Context) error {
		return provider.Shutdown(ctx)
	}
	return provider, shutdown, nil
}

// ConfigureGlobalTracer installs the provided tracer provider globally and returns a framework tracer.
func ConfigureGlobalTracer(provider trace.TracerProvider, instrumentation string) core.Tracer {
	if provider == nil {
		return core.NoopTracer
	}
	otel.SetTracerProvider(provider)
	return NewOTelTracer(provider, instrumentation)
}
