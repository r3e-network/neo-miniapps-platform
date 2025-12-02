package vrf

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
	envTracer := stubTracer{name: "env"}
	svc.SetEnvironment(framework.Environment{Tracer: envTracer})
	if svc.dispatchTracer() != envTracer {
		t.Fatalf("expected env tracer applied to dispatch")
	}
}

func TestServiceCustomTracerOverridesEnv(t *testing.T) {
	svc := New(nil, nil, nil)
	svc.SetEnvironment(framework.Environment{Tracer: stubTracer{name: "env"}})

	custom := stubTracer{name: "custom"}
	svc.WithTracer(custom)
	if svc.dispatchTracer() != custom {
		t.Fatalf("expected custom tracer applied")
	}

	svc.SetEnvironment(framework.Environment{Tracer: stubTracer{name: "env2"}})
	if svc.dispatchTracer() != custom {
		t.Fatalf("expected custom tracer to persist")
	}

	svc.WithTracer(nil)
	if svc.dispatchTracer().(stubTracer).name != "env2" {
		t.Fatalf("expected environment tracer restored after clearing custom tracer")
	}
}

func (s *Service) dispatchTracer() core.Tracer {
	return s.dispatch.Tracer()
}
