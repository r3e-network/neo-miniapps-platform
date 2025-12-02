// Package sandbox provides policy configuration loading and management.
//
// This file implements policy configuration loading from YAML files,
// supporting hot-reload and hierarchical policy definitions.
package sandbox

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// Policy Configuration
// =============================================================================

// PolicyConfig represents the complete policy configuration.
type PolicyConfig struct {
	// Version of the policy format
	Version string `json:"version" yaml:"version"`

	// DefaultEffect when no rule matches (should be "deny")
	DefaultEffect PolicyEffect `json:"default_effect" yaml:"default_effect"`

	// Rules are the security rules
	Rules []PolicyRule `json:"rules" yaml:"rules"`

	// ServicePolicies are per-service policy overrides
	ServicePolicies map[string]ServicePolicyConfig `json:"service_policies" yaml:"service_policies"`

	// CapabilityProfiles are predefined capability sets
	CapabilityProfiles map[string][]Capability `json:"capability_profiles" yaml:"capability_profiles"`
}

// ServicePolicyConfig defines policy for a specific service.
type ServicePolicyConfig struct {
	// SecurityLevel override
	SecurityLevel string `json:"security_level" yaml:"security_level"`

	// AllowedCapabilities that can be granted
	AllowedCapabilities []Capability `json:"allowed_capabilities" yaml:"allowed_capabilities"`

	// DeniedCapabilities that cannot be granted
	DeniedCapabilities []Capability `json:"denied_capabilities" yaml:"denied_capabilities"`

	// AllowedTargets services this service can call
	AllowedTargets []string `json:"allowed_targets" yaml:"allowed_targets"`

	// AllowedEvents patterns this service can publish
	AllowedEvents []string `json:"allowed_events" yaml:"allowed_events"`

	// StorageQuota in bytes
	StorageQuota int64 `json:"storage_quota" yaml:"storage_quota"`

	// RateLimits for various operations
	RateLimits RateLimitConfig `json:"rate_limits" yaml:"rate_limits"`
}

// RateLimitConfig defines rate limits for a service.
type RateLimitConfig struct {
	EventsPerMinute  int `json:"events_per_minute" yaml:"events_per_minute"`
	CallsPerMinute   int `json:"calls_per_minute" yaml:"calls_per_minute"`
	StorageOpsPerMin int `json:"storage_ops_per_minute" yaml:"storage_ops_per_minute"`
}

// =============================================================================
// Policy Loader
// =============================================================================

// PolicyLoader loads and manages security policies.
type PolicyLoader struct {
	mu sync.RWMutex

	// Current loaded policy
	policy *SecurityPolicy
	config *PolicyConfig

	// File watching
	configPath    string
	lastModified  time.Time
	watchInterval time.Duration
	stopWatch     chan struct{}

	// Callbacks
	onReload func(*SecurityPolicy)
}

// PolicyLoaderConfig configures the policy loader.
type PolicyLoaderConfig struct {
	// ConfigPath is the path to the policy configuration file
	ConfigPath string

	// WatchInterval for hot-reload (0 to disable)
	WatchInterval time.Duration

	// OnReload callback when policy is reloaded
	OnReload func(*SecurityPolicy)
}

// NewPolicyLoader creates a new policy loader.
func NewPolicyLoader(cfg PolicyLoaderConfig) *PolicyLoader {
	return &PolicyLoader{
		configPath:    cfg.ConfigPath,
		watchInterval: cfg.WatchInterval,
		onReload:      cfg.OnReload,
		stopWatch:     make(chan struct{}),
	}
}

// Load loads the policy from the configured file.
func (pl *PolicyLoader) Load() (*SecurityPolicy, error) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	if pl.configPath == "" {
		// No config file, use defaults
		pl.policy = NewSecurityPolicy()
		pl.config = DefaultPolicyConfig()
		return pl.policy, nil
	}

	config, err := pl.loadConfigFile(pl.configPath)
	if err != nil {
		return nil, fmt.Errorf("load policy config: %w", err)
	}

	policy := pl.buildPolicy(config)
	pl.policy = policy
	pl.config = config

	return policy, nil
}

// loadConfigFile loads a policy configuration from a file.
func (pl *PolicyLoader) loadConfigFile(path string) (*PolicyConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var config PolicyConfig

	// Determine format by extension
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("parse JSON: %w", err)
		}
	case ".yaml", ".yml":
		// Simple YAML parsing (for production, use gopkg.in/yaml.v3)
		if err := parseSimpleYAML(data, &config); err != nil {
			return nil, fmt.Errorf("parse YAML: %w", err)
		}
	default:
		// Try JSON first, then YAML
		if err := json.Unmarshal(data, &config); err != nil {
			if err := parseSimpleYAML(data, &config); err != nil {
				return nil, fmt.Errorf("unknown format: %s", ext)
			}
		}
	}

	// Validate configuration
	if err := pl.validateConfig(&config); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &config, nil
}

// validateConfig validates a policy configuration.
func (pl *PolicyLoader) validateConfig(config *PolicyConfig) error {
	if config.Version == "" {
		config.Version = "1.0"
	}

	if config.DefaultEffect == "" {
		config.DefaultEffect = PolicyEffectDeny
	}

	// Validate rules
	for i, rule := range config.Rules {
		if rule.Subject == "" {
			return fmt.Errorf("rule %d: subject is required", i)
		}
		if rule.Object == "" {
			return fmt.Errorf("rule %d: object is required", i)
		}
		if rule.Action == "" {
			return fmt.Errorf("rule %d: action is required", i)
		}
		if rule.Effect != PolicyEffectAllow && rule.Effect != PolicyEffectDeny {
			return fmt.Errorf("rule %d: invalid effect %q", i, rule.Effect)
		}
	}

	return nil
}

// buildPolicy builds a SecurityPolicy from configuration.
func (pl *PolicyLoader) buildPolicy(config *PolicyConfig) *SecurityPolicy {
	policy := &SecurityPolicy{
		rules: make([]PolicyRule, 0, len(config.Rules)+len(defaultSecurityRules())),
	}

	// Add default rules first (lowest priority)
	policy.rules = append(policy.rules, defaultSecurityRules()...)

	// Add configured rules
	policy.rules = append(policy.rules, config.Rules...)

	return policy
}

// Policy returns the current policy.
func (pl *PolicyLoader) Policy() *SecurityPolicy {
	pl.mu.RLock()
	defer pl.mu.RUnlock()
	return pl.policy
}

// Config returns the current configuration.
func (pl *PolicyLoader) Config() *PolicyConfig {
	pl.mu.RLock()
	defer pl.mu.RUnlock()
	return pl.config
}

// StartWatching starts watching for configuration changes.
func (pl *PolicyLoader) StartWatching() {
	if pl.watchInterval <= 0 || pl.configPath == "" {
		return
	}

	go pl.watchLoop()
}

// StopWatching stops watching for configuration changes.
func (pl *PolicyLoader) StopWatching() {
	close(pl.stopWatch)
}

func (pl *PolicyLoader) watchLoop() {
	ticker := time.NewTicker(pl.watchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pl.stopWatch:
			return
		case <-ticker.C:
			pl.checkAndReload()
		}
	}
}

func (pl *PolicyLoader) checkAndReload() {
	info, err := os.Stat(pl.configPath)
	if err != nil {
		return
	}

	pl.mu.RLock()
	lastMod := pl.lastModified
	pl.mu.RUnlock()

	if info.ModTime().After(lastMod) {
		if _, err := pl.Load(); err == nil {
			pl.mu.Lock()
			pl.lastModified = info.ModTime()
			pl.mu.Unlock()

			if pl.onReload != nil {
				pl.onReload(pl.policy)
			}
		}
	}
}

// =============================================================================
// Default Policy Configuration
// =============================================================================

// DefaultPolicyConfig returns the default policy configuration.
func DefaultPolicyConfig() *PolicyConfig {
	return &PolicyConfig{
		Version:       "1.0",
		DefaultEffect: PolicyEffectDeny,
		Rules:         defaultSecurityRules(),
		ServicePolicies: map[string]ServicePolicyConfig{
			// System services get full access
			"system.*": {
				SecurityLevel: "system",
				AllowedCapabilities: []Capability{
					CapSystemAdmin,
					CapSystemConfig,
					CapSystemAudit,
				},
			},
			// Core R3E services get privileged access
			"com.r3e.services.*": {
				SecurityLevel: "privileged",
				AllowedCapabilities: []Capability{
					CapStorageRead,
					CapStorageWrite,
					CapDatabaseRead,
					CapDatabaseWrite,
					CapBusPublish,
					CapBusSubscribe,
					CapBusInvoke,
					CapNetworkOutbound,
					CapServiceCall,
				},
				RateLimits: RateLimitConfig{
					EventsPerMinute:  1000,
					CallsPerMinute:   500,
					StorageOpsPerMin: 1000,
				},
			},
		},
		CapabilityProfiles: map[string][]Capability{
			"minimal": {
				CapStorageRead,
			},
			"standard": {
				CapStorageRead,
				CapStorageWrite,
				CapBusPublish,
				CapBusSubscribe,
			},
			"full": {
				CapStorageRead,
				CapStorageWrite,
				CapStorageDelete,
				CapDatabaseRead,
				CapDatabaseWrite,
				CapBusPublish,
				CapBusSubscribe,
				CapBusInvoke,
				CapNetworkOutbound,
				CapServiceCall,
			},
		},
	}
}

// =============================================================================
// Enhanced Security Policy Methods
// =============================================================================

// AddRule adds a rule to the policy.
func (sp *SecurityPolicy) AddRule(rule PolicyRule) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.rules = append(sp.rules, rule)
}

// RemoveRule removes rules matching the given criteria.
func (sp *SecurityPolicy) RemoveRule(subject, object, action string) int {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	removed := 0
	newRules := make([]PolicyRule, 0, len(sp.rules))
	for _, rule := range sp.rules {
		if rule.Subject == subject && rule.Object == object && rule.Action == action {
			removed++
			continue
		}
		newRules = append(newRules, rule)
	}
	sp.rules = newRules
	return removed
}

// Rules returns a copy of all rules.
func (sp *SecurityPolicy) Rules() []PolicyRule {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	result := make([]PolicyRule, len(sp.rules))
	copy(result, sp.rules)
	return result
}

// EvaluateWithContext evaluates a policy with service context substitution.
func (sp *SecurityPolicy) EvaluateWithContext(serviceID, subject, object, action string) PolicyEffect {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	// Substitute ${service} in patterns
	expandedSubject := strings.ReplaceAll(subject, "${service}", serviceID)
	expandedObject := strings.ReplaceAll(object, "${service}", serviceID)

	var matchedRule *PolicyRule
	for i := range sp.rules {
		rule := &sp.rules[i]

		// Expand rule patterns
		ruleSubject := strings.ReplaceAll(rule.Subject, "${service}", serviceID)
		ruleObject := strings.ReplaceAll(rule.Object, "${service}", serviceID)

		if matchPatternGlob(ruleSubject, expandedSubject) &&
			matchPatternGlob(ruleObject, expandedObject) &&
			matchPatternGlob(rule.Action, action) {
			if matchedRule == nil || rule.Priority > matchedRule.Priority {
				matchedRule = rule
			}
		}
	}

	if matchedRule != nil {
		return matchedRule.Effect
	}

	return PolicyEffectDeny
}

// matchPatternGlob performs glob-style pattern matching.
func matchPatternGlob(pattern, value string) bool {
	if pattern == "*" {
		return true
	}

	// Convert glob to regex
	regexPattern := "^" + regexp.QuoteMeta(pattern) + "$"
	regexPattern = strings.ReplaceAll(regexPattern, `\*`, ".*")
	regexPattern = strings.ReplaceAll(regexPattern, `\?`, ".")

	matched, err := regexp.MatchString(regexPattern, value)
	if err != nil {
		return pattern == value
	}
	return matched
}

// =============================================================================
// Capability Profile Helpers
// =============================================================================

// GetCapabilityProfile returns capabilities for a named profile.
func (pc *PolicyConfig) GetCapabilityProfile(name string) []Capability {
	if pc.CapabilityProfiles == nil {
		return nil
	}
	return pc.CapabilityProfiles[name]
}

// GetServicePolicy returns the policy for a specific service.
func (pc *PolicyConfig) GetServicePolicy(serviceID string) *ServicePolicyConfig {
	if pc.ServicePolicies == nil {
		return nil
	}

	// Try exact match first
	if policy, ok := pc.ServicePolicies[serviceID]; ok {
		return &policy
	}

	// Try pattern matching
	for pattern, policy := range pc.ServicePolicies {
		if matchPatternGlob(pattern, serviceID) {
			return &policy
		}
	}

	return nil
}

// IsCapabilityAllowed checks if a capability is allowed for a service.
func (pc *PolicyConfig) IsCapabilityAllowed(serviceID string, cap Capability) bool {
	policy := pc.GetServicePolicy(serviceID)
	if policy == nil {
		// No specific policy, allow by default
		return true
	}

	// Check denied list first
	for _, denied := range policy.DeniedCapabilities {
		if denied == cap {
			return false
		}
	}

	// If allowed list is specified, capability must be in it
	if len(policy.AllowedCapabilities) > 0 {
		for _, allowed := range policy.AllowedCapabilities {
			if allowed == cap {
				return true
			}
		}
		return false
	}

	return true
}

// =============================================================================
// Simple YAML Parser (for basic configs without external dependency)
// =============================================================================

// parseSimpleYAML is a basic YAML parser for simple configurations.
// For production use, replace with gopkg.in/yaml.v3.
func parseSimpleYAML(data []byte, config *PolicyConfig) error {
	// This is a placeholder - in production, use a proper YAML library
	// For now, we'll just try JSON parsing as a fallback
	return json.Unmarshal(data, config)
}

// =============================================================================
// Policy File Templates
// =============================================================================

// GenerateDefaultPolicyFile generates a default policy configuration file.
func GenerateDefaultPolicyFile() string {
	return `{
  "version": "1.0",
  "default_effect": "deny",
  "rules": [
    {
      "subject": "*",
      "object": "*",
      "action": "*",
      "effect": "deny",
      "priority": 0
    },
    {
      "subject": "${service}",
      "object": "storage:${service}/*",
      "action": "read",
      "effect": "allow",
      "priority": 100
    },
    {
      "subject": "${service}",
      "object": "storage:${service}/*",
      "action": "write",
      "effect": "allow",
      "priority": 100
    },
    {
      "subject": "${service}",
      "object": "database:${service}_*",
      "action": "read",
      "effect": "allow",
      "priority": 100
    },
    {
      "subject": "${service}",
      "object": "database:${service}_*",
      "action": "write",
      "effect": "allow",
      "priority": 100
    },
    {
      "subject": "${service}",
      "object": "bus:event:${service}.*",
      "action": "publish",
      "effect": "allow",
      "priority": 100
    },
    {
      "subject": "*",
      "object": "bus:event:public.*",
      "action": "subscribe",
      "effect": "allow",
      "priority": 50
    },
    {
      "subject": "system.*",
      "object": "*",
      "action": "*",
      "effect": "allow",
      "priority": 1000
    }
  ],
  "service_policies": {
    "com.r3e.services.*": {
      "security_level": "privileged",
      "allowed_capabilities": [
        "storage.read",
        "storage.write",
        "database.read",
        "database.write",
        "bus.publish",
        "bus.subscribe",
        "bus.invoke",
        "network.outbound",
        "service.call"
      ],
      "rate_limits": {
        "events_per_minute": 1000,
        "calls_per_minute": 500,
        "storage_ops_per_minute": 1000
      }
    }
  },
  "capability_profiles": {
    "minimal": ["storage.read"],
    "standard": ["storage.read", "storage.write", "bus.publish", "bus.subscribe"],
    "full": [
      "storage.read", "storage.write", "storage.delete",
      "database.read", "database.write",
      "bus.publish", "bus.subscribe", "bus.invoke",
      "network.outbound", "service.call"
    ]
  }
}`
}
