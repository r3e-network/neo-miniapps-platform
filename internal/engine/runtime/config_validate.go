package runtime

import (
	"fmt"
	"strings"

	"github.com/R3E-Network/service_layer/internal/config"
)

// validateRuntimeConfig enforces basic sanity checks for runtime-enabled modules.
func validateRuntimeConfig(cfg *config.Config) error {
	if cfg == nil {
		return nil
	}
	rt := cfg.Runtime
	if rt.Chains.Enabled {
		if len(rt.Chains.Endpoints) == 0 {
			return fmt.Errorf("runtime.chains.enabled=true but no endpoints configured")
		}
		for name, url := range rt.Chains.Endpoints {
			if strings.TrimSpace(name) == "" || strings.TrimSpace(url) == "" {
				return fmt.Errorf("runtime.chains.endpoints contains empty chain name or url")
			}
		}
		for chain, methods := range rt.Chains.AllowedMethods {
			if strings.TrimSpace(chain) == "" {
				return fmt.Errorf("runtime.chains.allowed_methods contains empty chain name")
			}
			for _, m := range methods {
				if strings.TrimSpace(m) == "" {
					return fmt.Errorf("runtime.chains.allowed_methods[%s] contains empty method", chain)
				}
			}
		}
		if rt.Chains.PerTenantPerMinute < 0 {
			return fmt.Errorf("runtime.chains.per_tenant_per_minute must be non-negative")
		}
		if rt.Chains.PerTokenPerMinute < 0 {
			return fmt.Errorf("runtime.chains.per_token_per_minute must be non-negative")
		}
		if rt.Chains.Burst < 0 {
			return fmt.Errorf("runtime.chains.burst must be non-negative")
		}
	}
	if rt.DataSources.Enabled {
		if len(rt.DataSources.Sources) == 0 {
			return fmt.Errorf("runtime.data_sources.enabled=true but no sources configured")
		}
		for name, url := range rt.DataSources.Sources {
			if strings.TrimSpace(name) == "" || strings.TrimSpace(url) == "" {
				return fmt.Errorf("runtime.data_sources.sources contains empty source name or url")
			}
		}
	}
	if rt.ServiceBank.Enabled && !gasBankEnabled(rt.GasBank) && !rt.Neo.Enabled {
		return fmt.Errorf("service_bank enabled requires gasbank/neo modules to be configured")
	}
	if rt.ServiceBank.Enabled {
		for name, limit := range rt.ServiceBank.Limits {
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("service_bank limits contains empty module name")
			}
			if limit < 0 {
				return fmt.Errorf("service_bank limit for %q must be non-negative", name)
			}
		}
	}
	if rt.Crypto.Enabled {
		if len(rt.Crypto.Capabilities) == 0 {
			return fmt.Errorf("runtime.crypto.enabled=true but no capabilities configured")
		}
		for _, cap := range rt.Crypto.Capabilities {
			if strings.TrimSpace(cap) == "" {
				return fmt.Errorf("runtime.crypto.capabilities contains an empty entry")
			}
		}
	}
	if rt.RocketMQ.Enabled {
		if len(rt.RocketMQ.NameServers) == 0 {
			return fmt.Errorf("runtime.rocketmq.enabled=true but no name_servers configured")
		}
		if strings.TrimSpace(rt.RocketMQ.ConsumerGroup) == "" {
			return fmt.Errorf("runtime.rocketmq.consumer_group required when rocketmq enabled")
		}
		if rt.RocketMQ.MaxReconsume < 0 {
			return fmt.Errorf("runtime.rocketmq.max_reconsume_times must be non-negative")
		}
		if rt.RocketMQ.ConsumeBatch < 0 {
			return fmt.Errorf("runtime.rocketmq.consume_batch must be non-negative")
		}
		if val := strings.ToLower(strings.TrimSpace(rt.RocketMQ.ConsumeFrom)); val != "" && val != "latest" && val != "first" {
			return fmt.Errorf("runtime.rocketmq.consume_from must be latest or first")
		}
	}
	return nil
}

// gasBankEnabled reports whether gas bank settlement is configured.
func gasBankEnabled(g config.GasBankConfig) bool {
	return strings.TrimSpace(g.ResolverURL) != "" || g.MaxAttempts > 0 || strings.TrimSpace(g.PollInterval) != ""
}
