package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadServicesConfig loads the services configuration from config/services.yaml
func LoadServicesConfig() (*ServicesConfig, error) {
	return LoadServicesConfigFromPath(filepath.Join("config", "services.yaml"))
}

// LoadServicesConfigFromPath loads the services configuration from a specific path
func LoadServicesConfigFromPath(path string) (*ServicesConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read services config: %w", err)
	}

	var cfg ServicesConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse services config: %w", err)
	}

	// Validate that all services have required fields
	for id, settings := range cfg.Services {
		if settings.Port == 0 {
			return nil, fmt.Errorf("service %s: port is required", id)
		}
	}

	return &cfg, nil
}

// LoadServicesConfigOrDefault loads services config or returns default if file not found
func LoadServicesConfigOrDefault() *ServicesConfig {
	cfg, err := LoadServicesConfig()
	if err != nil {
		// Return default configuration with all services enabled
		return DefaultServicesConfig()
	}
	return cfg
}

// DefaultServicesConfig returns the default services configuration
func DefaultServicesConfig() *ServicesConfig {
	return &ServicesConfig{
		Services: map[string]*ServiceSettings{
			"globalsigner": {
				Enabled:     true,
				Port:        8092,
				Description: "TEE master key management and domain-separated signing",
			},
			"txproxy": {
				Enabled:     true,
				Port:        8090,
				Description: "Allowlisted transaction signing + broadcast proxy",
			},
			"neofeeds": {
				Enabled:     true,
				Port:        8083,
				Description: "Decentralized market data",
			},
			"neoflow": {
				Enabled:     true,
				Port:        8084,
				Description: "Automated smart contract execution",
			},
			"neoaccounts": {
				Enabled:     true,
				Port:        8085,
				Description: "Account pool management",
			},
			"neocompute": {
				Enabled:     true,
				Port:        8086,
				Description: "Secure JavaScript execution",
			},
			"neovrf": {
				Enabled:     true,
				Port:        8087,
				Description: "TEE-backed verifiable randomness",
			},
			"neooracle": {
				Enabled:     true,
				Port:        8088,
				Description: "External data delivery with proofs",
			},
		},
	}
}

// ServiceNameMapping provides mapping from old service names to new Neo names
var ServiceNameMapping = map[string]string{
	"oracle":      "neooracle",
	"neofeeds":    "neofeeds",
	"neoaccounts": "neoaccounts",
	"neocompute":  "neocompute",
	"neoflow":     "neoflow",
	"vrf":         "neovrf",
	"tx-proxy":    "txproxy",
}

// GetNeoServiceName converts old service name to new Neo name
func GetNeoServiceName(oldName string) string {
	if newName, ok := ServiceNameMapping[oldName]; ok {
		return newName
	}
	return oldName // Return as-is if not found (might already be new name)
}
