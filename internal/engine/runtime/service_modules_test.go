package runtime

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	app "github.com/R3E-Network/service_layer/internal/app"
	"github.com/R3E-Network/service_layer/internal/app/system"
	engine "github.com/R3E-Network/service_layer/internal/engine"
)

type readyService struct {
	system.Lifecycle
	status string
	err    string
}

func (s *readyService) Name() string   { return "ready-svc" }
func (s *readyService) Domain() string { return "ready" }
func (s *readyService) SetReady(status, err string) {
	s.status = status
	s.err = err
}
func (s *readyService) Ready(ctx context.Context) error {
	_ = ctx
	if s.status != "ready" {
		return fmt.Errorf("not ready")
	}
	return nil
}

func TestWrapServicesUsesStableEngineNames(t *testing.T) {
	appInstance, err := app.New(app.Stores{}, nil, app.WithRuntimeConfig(app.RuntimeConfig{}))
	if err != nil {
		t.Fatalf("new app: %v", err)
	}

	ordering := []string{
		"store-postgres",
		"core-application",
		"svc-accounts",
		"svc-functions",
		"svc-triggers",
		"svc-gasbank",
		"svc-automation",
		"svc-pricefeed",
		"svc-datafeeds",
		"svc-datastreams",
		"svc-datalink",
		"svc-dta",
		"svc-confidential",
		"svc-cre",
		"svc-ccip",
		"svc-vrf",
		"svc-secrets",
		"svc-random",
		"svc-oracle",
		"runner-automation",
		"runner-pricefeed",
		"runner-oracle",
		"runner-gasbank",
	}
	eng := engine.New(engine.WithOrder(ordering...))

	if err := wrapServices(appInstance, eng); err != nil {
		t.Fatalf("wrap services: %v", err)
	}

	names := eng.Modules()
	// Gas bank settlement runner is optional and nil with default config.
	expected := []string{
		"svc-accounts",
		"svc-functions",
		"svc-triggers",
		"svc-gasbank",
		"svc-automation",
		"svc-pricefeed",
		"svc-datafeeds",
		"svc-datastreams",
		"svc-datalink",
		"svc-dta",
		"svc-confidential",
		"svc-cre",
		"svc-ccip",
		"svc-vrf",
		"svc-secrets",
		"svc-random",
		"svc-oracle",
		"runner-automation",
		"runner-pricefeed",
		"runner-oracle",
	}
	if !reflect.DeepEqual(names, expected) {
		t.Fatalf("unexpected module names\nexpected: %v\ngot:      %v", expected, names)
	}
}

func TestRegisterModuleForwardsReadySetter(t *testing.T) {
	eng := engine.New()
	svc := &readyService{}
	if err := registerModule(eng, "svc-ready", "ready", svc, true); err != nil {
		t.Fatalf("register module: %v", err)
	}

	eng.MarkReady("ready", "")
	if svc.status != "ready" || svc.err != "" {
		t.Fatalf("expected ready status propagated, got status=%q err=%q", svc.status, svc.err)
	}

	eng.MarkReady("not-ready", "maintenance", "svc-ready")
	if svc.status != "not-ready" || svc.err != "maintenance" {
		t.Fatalf("expected not-ready propagated, got status=%q err=%q", svc.status, svc.err)
	}

	// Ensure ReadyFunc still delegates to underlying service implementation.
	svc.status = "not-ready"
	if err := svc.Ready(context.Background()); err == nil {
		t.Fatalf("expected ready to error when not ready")
	}
	svc.status = "ready"
	if err := svc.Ready(context.Background()); err != nil {
		t.Fatalf("expected ready to succeed: %v", err)
	}
}

func TestWrapServicesExposesTypedEngines(t *testing.T) {
	appInstance, err := app.New(app.Stores{}, nil, app.WithRuntimeConfig(app.RuntimeConfig{}))
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	eng := engine.New(engine.WithOrder(
		"svc-accounts",
		"svc-functions",
		"svc-datastreams",
		"svc-pricefeed",
		"svc-datafeeds",
		"svc-datalink",
		"svc-oracle",
	))

	if err := wrapServices(appInstance, eng); err != nil {
		t.Fatalf("wrap services: %v", err)
	}

	if got := eng.AccountEngines(); len(got) != 1 || got[0].Name() != "svc-accounts" {
		t.Fatalf("expected account engine svc-accounts, got %+v", got)
	}
	if got := eng.ComputeEngines(); len(got) != 1 || got[0].Name() != "svc-functions" {
		t.Fatalf("expected compute engine svc-functions, got %+v", got)
	}
	if got := eng.DataEngines(); len(got) != 1 || got[0].Name() != "svc-datastreams" {
		t.Fatalf("expected data engine svc-datastreams, got %+v", got)
	}
	events := eng.EventEngines()
	var names []string
	for _, e := range events {
		names = append(names, e.Name())
	}
	expectedEvents := []string{"svc-pricefeed", "svc-datafeeds", "svc-datalink", "svc-oracle"}
	if !reflect.DeepEqual(names, expectedEvents) {
		t.Fatalf("expected event engines %v, got %v", expectedEvents, names)
	}
}

type noopModule struct {
	name   string
	domain string
}

func (n noopModule) Name() string              { return n.name }
func (n noopModule) Domain() string            { return n.domain }
func (noopModule) Start(context.Context) error { return nil }
func (noopModule) Stop(context.Context) error  { return nil }
func (noopModule) Ready(context.Context) error { return nil }
func (noopModule) SetReady(string, string)     {}

func TestRegisterModuleGeneratesUniqueNames(t *testing.T) {
	eng := engine.New()
	mod := noopModule{name: "dup", domain: "d"}
	if err := registerModule(eng, mod.name, mod.domain, mod, false); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := registerModule(eng, mod.name, mod.domain, mod, false); err != nil {
		t.Fatalf("second register: %v", err)
	}
	if err := registerModule(eng, mod.name, mod.domain, mod, false); err != nil {
		t.Fatalf("third register: %v", err)
	}
	got := eng.Modules()
	want := []string{"dup", "dup-d-1", "dup-d-2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unique names mismatch: want %v got %v", want, got)
	}
}

func TestRegisterModuleRequiresNameAndDomain(t *testing.T) {
	eng := engine.New()
	if err := registerModule(eng, "", "domain", noopModule{name: "", domain: "domain"}, false); err == nil {
		t.Fatalf("expected error for empty name")
	}
	if err := registerModule(eng, "name", "", noopModule{name: "name", domain: ""}, false); err == nil {
		t.Fatalf("expected error for empty domain")
	}
}

func TestRegisterModuleRejectsNil(t *testing.T) {
	eng := engine.New()
	if err := registerModule(eng, "nil-mod", "test", nil, false); err == nil {
		t.Fatalf("expected error for nil module")
	}
}
