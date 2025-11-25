package runtime

import (
	"context"
	"testing"

	app "github.com/R3E-Network/service_layer/internal/app"
	engine "github.com/R3E-Network/service_layer/internal/engine"
)

// Ensures the engine-owned lifecycle starts and stops services cleanly and updates health.
func TestEngineLifecycleWithApplicationServices(t *testing.T) {
	application, err := app.New(app.Stores{}, nil)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}

	eng := engine.New()
	if err := eng.Register(newAppModule(application)); err != nil {
		t.Fatalf("register app module: %v", err)
	}
	if err := wrapServices(application, eng); err != nil {
		t.Fatalf("wrap services: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := eng.Start(ctx); err != nil {
		t.Fatalf("engine start: %v", err)
	}

	eng.ProbeReadiness(ctx)
	health := eng.ModulesHealth()
	if len(health) == 0 {
		t.Fatalf("expected modules in health")
	}
	for _, h := range health {
		if h.Status != "started" {
			t.Fatalf("module %s not started (status=%s)", h.Name, h.Status)
		}
		if h.ReadyStatus != "" && h.ReadyStatus != "ready" {
			t.Fatalf("module %s not ready (ready=%s err=%s)", h.Name, h.ReadyStatus, h.ReadyError)
		}
	}

	if err := eng.Stop(ctx); err != nil {
		t.Fatalf("engine stop: %v", err)
	}
	health = eng.ModulesHealth()
	for _, h := range health {
		if h.Status != "stopped" && h.Status != "stop-error" {
			t.Fatalf("module %s not stopped (status=%s)", h.Name, h.Status)
		}
		if h.ReadyStatus != "" && h.ReadyStatus != "not-ready" {
			t.Fatalf("module %s ready status not cleared: %s", h.Name, h.ReadyStatus)
		}
	}
}
