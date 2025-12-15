package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/R3E-Network/service_layer/internal/plugin"
)

// LoadServicesConfig loads the services configuration from config/services.yaml
func LoadServicesConfig() (*plugin.ServicesConfig, error) {
	return LoadServicesConfigFromPath(filepath.Join("config", "services.yaml"))
}

// LoadServicesConfigFromPath loads the services configuration from a specific path
func LoadServicesConfigFromPath(path string) (*plugin.ServicesConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read services config: %w", err)
	}

	var cfg plugin.ServicesConfig
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
func LoadServicesConfigOrDefault() *plugin.ServicesConfig {
	cfg, err := LoadServicesConfig()
	if err != nil {
		// Return default configuration with all services enabled
		return DefaultServicesConfig()
	}
	return cfg
}

// DefaultServicesConfig returns the default services configuration
func DefaultServicesConfig() *plugin.ServicesConfig {
	return &plugin.ServicesConfig{
		Services: map[string]*plugin.ServiceSettings{
			"neorand": {
				Enabled:     true,
				Port:        8081,
				Description: "Verifiable random number generation",
			},
			"neovault": {
				Enabled:     true,
				Port:        8082,
				Description: "Secure cryptographic operations",
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
			"neostore": {
				Enabled:     true,
				Port:        8087,
				Description: "Encrypted data management",
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
	"vrf":         "neorand",
	"oracle":      "neooracle",
	"neofeeds":    "neofeeds",
	"neoaccounts": "neoaccounts",
	"neovault":    "neovault",
	"neocompute":  "neocompute",
	"neoflow":     "neoflow",
	"secrets":     "neostore",
}

// GetNeoServiceName converts old service name to new Neo name
func GetNeoServiceName(oldName string) string {
	if newName, ok := ServiceNameMapping[oldName]; ok {
		return newName
	}
	return oldName // Return as-is if not found (might already be new name)
}
