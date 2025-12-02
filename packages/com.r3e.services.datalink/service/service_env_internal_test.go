package datalink

import (
	"context"
	"testing"

	"github.com/R3E-Network/service_layer/system/os/framework"
	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

type stubTracer struct {
	name string
}

func (stubTracer) StartSpan(ctx context.Context, name string, attrs map[string]string) (context.Context, func(error)) {
	return ctx, func(error) {}
}

func TestServiceSetEnvironmentUpdatesTracer(t *testing.T) {
	svc := New(nil, nil, nil)
	tracer := stubTracer{name: "env"}
	svc.SetEnvironment(framework.Environment{Tracer: tracer})
	if svc.dispatchTracer() != tracer {
		t.Fatalf("expected dispatch tracer to match environment tracer")
	}
}

func TestServiceCustomTracerOverridesEnvironment(t *testing.T) {
	svc := New(nil, nil, nil)
	svc.SetEnvironment(framework.Environment{Tracer: stubTracer{name: "env"}})
	custom := stubTracer{name: "custom"}
	svc.WithTracer(custom)

	if svc.dispatchTracer() != custom {
		t.Fatalf("expected dispatch tracer to use custom tracer")
	}

	svc.SetEnvironment(framework.Environment{Tracer: stubTracer{name: "env2"}})
	if svc.dispatchTracer() != custom {
		t.Fatalf("expected custom tracer to persist after environment change")
	}

	svc.WithTracer(nil)
	if svc.dispatchTracer() == custom {
		t.Fatalf("expected custom tracer cleared")
	}
	if svc.dispatchTracer().(stubTracer).name != "env2" {
		t.Fatalf("expected fallback to latest environment tracer")
	}
}

func (s *Service) dispatchTracer() core.Tracer {
	return s.dispatch.Tracer()
}
