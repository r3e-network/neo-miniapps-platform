package plugin

import (
	"github.com/R3E-Network/service_layer/internal/chain"
	"github.com/R3E-Network/service_layer/internal/database"
	"github.com/R3E-Network/service_layer/internal/marble"
)

// ServiceConfig provides common configuration for all service plugins.
// This is passed to the ServiceFactory when creating a new service instance.
type ServiceConfig struct {
	// Marble provides TEE functionality and secrets
	Marble *marble.Marble

	// DB provides database access
	DB database.RepositoryInterface

	// ChainClient provides blockchain interaction (optional)
	ChainClient *chain.Client

	// ConfigPath is the path to service-specific configuration file (optional)
	ConfigPath string

	// ServiceSettings contains service-specific settings from config/services.yaml
	ServiceSettings *ServiceSettings
}

// ServiceSettings holds configuration for a single service from services.yaml.
type ServiceSettings struct {
	// Enabled determines if the service should run
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Port is the HTTP port for the service
	Port int `yaml:"port" json:"port"`

	// Description is a human-readable description
	Description string `yaml:"description" json:"description"`

	// Extra holds any additional service-specific configuration
	Extra map[string]interface{} `yaml:"extra,omitempty" json:"extra,omitempty"`
}

// ServicesConfig holds configuration for all services.
type ServicesConfig struct {
	Services map[string]*ServiceSettings `yaml:"services" json:"services"`
}

// IsEnabled checks if a service is enabled in the configuration.
// Returns false if the service is not found in config.
func (c *ServicesConfig) IsEnabled(serviceID string) bool {
	if c == nil || c.Services == nil {
		return false
	}
	settings, ok := c.Services[serviceID]
	if !ok {
		return false
	}
	return settings.Enabled
}

// GetSettings returns the settings for a service.
// Returns nil if the service is not found.
func (c *ServicesConfig) GetSettings(serviceID string) *ServiceSettings {
	if c == nil || c.Services == nil {
		return nil
	}
	return c.Services[serviceID]
}

// EnabledServices returns a list of enabled service IDs.
func (c *ServicesConfig) EnabledServices() []string {
	if c == nil || c.Services == nil {
		return nil
	}
	var enabled []string
	for id, settings := range c.Services {
		if settings.Enabled {
			enabled = append(enabled, id)
		}
	}
	return enabled
}

// DisabledServices returns a list of disabled service IDs.
func (c *ServicesConfig) DisabledServices() []string {
	if c == nil || c.Services == nil {
		return nil
	}
	var disabled []string
	for id, settings := range c.Services {
		if !settings.Enabled {
			disabled = append(disabled, id)
		}
	}
	return disabled
}
