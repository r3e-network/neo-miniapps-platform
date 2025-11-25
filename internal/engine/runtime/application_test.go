package runtime

import (
	"testing"

	app "github.com/R3E-Network/service_layer/internal/app"
	"github.com/R3E-Network/service_layer/internal/config"
)

func TestDetermineListenAddrUsesDefaults(t *testing.T) {
	cfg := config.New()
	cfg.Server.Host = ""
	cfg.Server.Port = 0

	addr := determineListenAddr(cfg)
	if addr != "0.0.0.0:8080" {
		t.Fatalf("expected default addr, got %s", addr)
	}

	cfg.Server.Host = "127.0.0.1 "
	cfg.Server.Port = 9090
	if got := determineListenAddr(cfg); got != "127.0.0.1:9090" {
		t.Fatalf("unexpected addr %s", got)
	}
}

func TestSplitAndTrim(t *testing.T) {
	cases := map[string][]string{
		"":                 nil,
		"   ":              nil,
		"token1":           {"token1"},
		"token1, token2  ": {"token1", "token2"},
	}

	for input, expected := range cases {
		if got := splitAndTrim(input); len(got) != len(expected) {
			t.Fatalf("splitAndTrim(%q) length mismatch: %v vs %v", input, got, expected)
		}
	}
}

func TestResolveAPITokensReadsEnvAndConfig(t *testing.T) {
	cfg := config.New()
	cfg.Auth.Tokens = []string{" config "}
	t.Setenv("API_TOKENS", "alpha, beta")
	t.Setenv("API_TOKEN", "gamma")
	tokens := resolveAPITokens(cfg)
	if len(tokens) != 4 {
		t.Fatalf("expected 4 tokens, got %d", len(tokens))
	}
	if tokens[0] != "config" || tokens[1] != "alpha" || tokens[3] != "gamma" {
		t.Fatalf("unexpected tokens %v", tokens)
	}
}

func TestResolveAPITokensTrimsConfig(t *testing.T) {
	cfg := config.New()
	cfg.Auth.Tokens = []string{"  token-one  ", ""}
	tokens := resolveAPITokens(cfg)
	if len(tokens) != 1 || tokens[0] != "token-one" {
		t.Fatalf("expected trimmed config tokens, got %v", tokens)
	}
}

func TestWithAPITokensOption(t *testing.T) {
	builder := defaultBuilderOptions()
	option := WithAPITokens([]string{" alpha ", "", "beta"})
	option(&builder)
	if len(builder.tokens) != 2 || builder.tokens[0] != "alpha" {
		t.Fatalf("tokens not cleaned: %v", builder.tokens)
	}
}

func TestWithRunMigrationsOption(t *testing.T) {
	builder := defaultBuilderOptions()
	WithRunMigrations(false)(&builder)
	if builder.runMigrations == nil || *builder.runMigrations {
		t.Fatalf("expected runMigrations to be false")
	}
}

func TestWithSlowThresholdOption(t *testing.T) {
	builder := defaultBuilderOptions()
	WithSlowThresholdMS(1500)(&builder)
	if builder.slowThreshold != 1500 {
		t.Fatalf("expected slow threshold set, got %d", builder.slowThreshold)
	}
}

func TestResolveSecretEncryptionKeyPrefersEnv(t *testing.T) {
	cfg := config.New()
	cfg.Security.SecretEncryptionKey = "config"
	t.Setenv("SECRET_ENCRYPTION_KEY", "env-value")
	defer t.Setenv("SECRET_ENCRYPTION_KEY", "")
	if key := resolveSecretEncryptionKey(cfg); key != "env-value" {
		t.Fatalf("expected env key to win, got %q", key)
	}
}

func TestResolveSecretEncryptionKeyFallsBackToConfig(t *testing.T) {
	cfg := config.New()
	cfg.Security.SecretEncryptionKey = "  config-key "
	t.Setenv("SECRET_ENCRYPTION_KEY", "")
	if key := resolveSecretEncryptionKey(cfg); key != "config-key" {
		t.Fatalf("expected config key, got %q", key)
	}
}

func TestApplicationDescriptorsIncludeServices(t *testing.T) {
	application, err := app.New(app.Stores{}, nil)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}
	descriptors := application.Descriptors()
	if len(descriptors) == 0 {
		t.Fatalf("expected descriptors to be populated")
	}
	var foundAccounts bool
	for _, d := range descriptors {
		if d.Name == "accounts" {
			foundAccounts = true
			break
		}
	}
	if !foundAccounts {
		t.Fatalf("expected accounts descriptor in %v", descriptors)
	}
}

func TestRuntimeConfigOverridesBusPermissions(t *testing.T) {
	cfg := config.New()
	cfg.Database.Driver = ""
	cfg.Database.DSN = ""
	disable := false
	cfg.Runtime.BusPermissions = map[string]config.BusPermission{
		"svc-functions": {Compute: &disable},
	}

	app, err := NewApplication(
		WithConfig(cfg),
		WithRunMigrations(false),
	)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}
	if got := app.engine.ComputeEngines(); len(got) != 0 {
		t.Fatalf("expected compute engine disabled by permissions, got %d entries", len(got))
	}
}

func TestDefaultModuleDepsApplied(t *testing.T) {
	cfg := config.New()
	cfg.Database.Driver = ""
	cfg.Database.DSN = ""

	app, err := NewApplication(
		WithConfig(cfg),
		WithRunMigrations(false),
	)
	if err != nil {
		t.Fatalf("new application: %v", err)
	}
	info := app.engine.ModulesInfo()
	find := func(name string) (moduleExists bool, deps []string) {
		for _, m := range info {
			if m.Name == name {
				return true, m.DependsOn
			}
		}
		return false, nil
	}

	if ok, deps := find("svc-functions"); ok {
		if len(deps) == 0 || deps[0] != "core-application" {
			t.Fatalf("expected svc-functions to depend on core-application, got %v", deps)
		}
	} else {
		t.Fatalf("svc-functions not found")
	}

	if ok, deps := find("runner-automation"); ok {
		var hasAutomation, hasCore bool
		for _, d := range deps {
			if d == "svc-automation" {
				hasAutomation = true
			}
			if d == "core-application" {
				hasCore = true
			}
		}
		if !hasAutomation || !hasCore {
			t.Fatalf("expected runner-automation deps to include svc-automation and core-application, got %v", deps)
		}
	}
}
