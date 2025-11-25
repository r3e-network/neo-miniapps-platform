package runtime

import (
	"testing"

	"github.com/R3E-Network/service_layer/internal/config"
)

func TestValidateRuntimeConfig_ChainsLimits(t *testing.T) {
	cfg := &config.Config{
		Runtime: config.RuntimeConfig{
			Chains: config.ChainRPCConfig{
				Enabled:            true,
				Endpoints:          map[string]string{"eth": "http://example"},
				PerTenantPerMinute: -1,
			},
		},
	}
	if err := validateRuntimeConfig(cfg); err == nil {
		t.Fatalf("expected error for negative per_tenant_per_minute")
	}
	cfg.Runtime.Chains.PerTenantPerMinute = 0
	cfg.Runtime.Chains.PerTokenPerMinute = -2
	if err := validateRuntimeConfig(cfg); err == nil {
		t.Fatalf("expected error for negative per_token_per_minute")
	}
	cfg.Runtime.Chains.PerTokenPerMinute = 1
	cfg.Runtime.Chains.Burst = -1
	if err := validateRuntimeConfig(cfg); err == nil {
		t.Fatalf("expected error for negative burst")
	}
	cfg.Runtime.Chains.Burst = 1
	cfg.Runtime.Chains.AllowedMethods = map[string][]string{"eth": {"", "eth_blockNumber"}}
	if err := validateRuntimeConfig(cfg); err == nil {
		t.Fatalf("expected error for empty allowed method")
	}
}

func TestValidateRuntimeConfig_CryptoCapabilities(t *testing.T) {
	cfg := &config.Config{
		Runtime: config.RuntimeConfig{
			Crypto: config.CryptoConfig{
				Enabled: true,
			},
		},
	}
	if err := validateRuntimeConfig(cfg); err == nil {
		t.Fatalf("expected error for missing crypto capabilities")
	}

	cfg.Runtime.Crypto.Capabilities = []string{"zkp", "fhe"}
	if err := validateRuntimeConfig(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
