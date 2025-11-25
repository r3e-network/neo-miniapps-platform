package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/httpapi"
	"github.com/R3E-Network/service_layer/internal/config"
	engine "github.com/R3E-Network/service_layer/internal/engine"
)

// Ensures the engine starts and stops the HTTP transport when managing lifecycle.
func TestRuntimeEngineStartsHTTPModule(t *testing.T) {
	cfg := config.New()
	cfg.Database.Driver = ""
	cfg.Database.DSN = ""
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 0

	app, err := NewApplication(
		WithConfig(cfg),
		WithRunMigrations(false),
		WithListenAddr("127.0.0.1:0"),
	)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runErr := make(chan error, 1)
	go func() {
		runErr <- app.Run(ctx)
	}()

	waitForModuleReady(t, app.engine, "svc-http")

	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()
	_ = app.Shutdown(shutdownCtx)

	if err := <-runErr; err != nil && err != context.Canceled {
		t.Fatalf("run returned error: %v", err)
	}

	app.engine.ProbeReadiness(context.Background())
	var found bool
	for _, h := range app.engine.ModulesHealth() {
		if h.Name != "svc-http" {
			continue
		}
		found = true
		if h.Status != "stopped" && h.Status != "stop-error" {
			t.Fatalf("expected http module stopped, got %s", h.Status)
		}
		if h.ReadyStatus != "" && h.ReadyStatus != "not-ready" {
			t.Fatalf("expected http module not-ready after shutdown, got %s", h.ReadyStatus)
		}
		if h.StartNanos == 0 || h.StopNanos == 0 {
			t.Fatalf("expected start/stop durations recorded, got start=%d stop=%d", h.StartNanos, h.StopNanos)
		}
	}
	if !found {
		t.Fatalf("svc-http not registered in engine")
	}
}

// Verifies /system/status is served and populated when the engine manages services.
func TestRuntimeEngineServesSystemStatus(t *testing.T) {
	cfg := config.New()
	cfg.Database.Driver = ""
	cfg.Database.DSN = ""

	app, err := NewApplication(
		WithConfig(cfg),
		WithRunMigrations(false),
		WithListenAddr("127.0.0.1:0"),
		WithAPITokens([]string{"dev-token"}),
	)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runErr := make(chan error, 1)
	go func() {
		runErr <- app.Run(ctx)
	}()

	addr := waitForBoundHTTP(t, app)

	req, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/system/status", nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("system status request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.StatusCode)
	}

	var payload struct {
		Modules []httpapi.ModuleStatus `json:"modules"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	var found bool
	for _, m := range payload.Modules {
		if m.Name == "svc-http" {
			found = true
			if m.Status != "started" {
				t.Fatalf("expected svc-http started, got %s", m.Status)
			}
		}
	}
	if !found {
		t.Fatalf("svc-http not reported in system status")
	}

	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()
	_ = app.Shutdown(shutdownCtx)

	if err := <-runErr; err != nil && err != context.Canceled {
		t.Fatalf("run returned error: %v", err)
	}
}

func waitForModuleReady(t *testing.T, eng *engine.Engine, name string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		eng.ProbeReadiness(context.Background())
		for _, h := range eng.ModulesHealth() {
			if h.Name == name && h.Status == "started" && (h.ReadyStatus == "" || h.ReadyStatus == "ready") {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("module %s did not become ready", name)
}

func waitForBoundHTTP(t *testing.T, app *Application) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if app != nil {
			if addr := strings.TrimSpace(app.ListenAddr()); addr != "" && !strings.HasSuffix(addr, ":0") {
				return addr
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("http service did not bind to an address")
	return ""
}
