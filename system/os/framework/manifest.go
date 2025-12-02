package framework

import (
	"fmt"
	"strings"

	engine "github.com/R3E-Network/service_layer/system/engine/core"
	service "github.com/R3E-Network/service_layer/system/os/framework/core"
)

// Manifest captures a service's contract with the engine OS: required surfaces,
// dependencies, quotas, and descriptive notes. Services can return a Manifest
// so the engine can expose richer status and perform basic validation.
// Aligned with ServiceRegistry.cs contract Service struct.
type Manifest struct {
	Name         string              `json:"name,omitempty"`
	Domain       string              `json:"domain,omitempty"`
	Description  string              `json:"description,omitempty"`
	Version      string              `json:"version,omitempty"`     // semantic version for compatibility
	CodeHash     string              `json:"code_hash,omitempty"`   // SHA256 hash of service code for verification
	ConfigHash   string              `json:"config_hash,omitempty"` // SHA256 hash of service config for verification
	RequiresAPIs []engine.APISurface `json:"requires_apis,omitempty"`
	DependsOn    []string            `json:"depends_on,omitempty"`
	Quotas       map[string]string   `json:"quotas,omitempty"` // freeform: e.g. gas, rpc, data
	Capabilities []string            `json:"capabilities,omitempty"`
	Layer        string              `json:"layer,omitempty"`   // service|runner|infra
	Tags         map[string]string   `json:"tags,omitempty"`    // freeform metadata for filtering
	Enabled      *bool               `json:"enabled,omitempty"` // nil means default (enabled)
}

// Normalize cleans up whitespace and dedupes fields.
func (m *Manifest) Normalize() {
	if m == nil {
		return
	}
	m.Name = strings.TrimSpace(m.Name)
	m.Domain = strings.TrimSpace(m.Domain)
	m.Description = strings.TrimSpace(m.Description)
	m.Version = strings.TrimSpace(m.Version)
	m.CodeHash = strings.TrimSpace(m.CodeHash)
	m.ConfigHash = strings.TrimSpace(m.ConfigHash)
	m.Layer = strings.TrimSpace(strings.ToLower(m.Layer))
	m.RequiresAPIs = dedupeSurfaces(m.RequiresAPIs)
	m.DependsOn = dedupeStrings(m.DependsOn)
	m.Capabilities = dedupeStrings(m.Capabilities)

	// Clean quotas
	if m.Quotas != nil {
		cleaned := make(map[string]string)
		for k, v := range m.Quotas {
			k, v = strings.TrimSpace(k), strings.TrimSpace(v)
			if k != "" && v != "" {
				cleaned[k] = v
			}
		}
		m.Quotas = cleaned
	}

	// Clean tags
	if m.Tags != nil {
		cleaned := make(map[string]string)
		for k, v := range m.Tags {
			k = strings.TrimSpace(k)
			if k != "" {
				cleaned[k] = strings.TrimSpace(v)
			}
		}
		m.Tags = cleaned
	}
}

// Validate performs lightweight checks for operator visibility.
func (m *Manifest) Validate() error {
	if m == nil {
		return nil
	}
	if m.Name == "" {
		return fmt.Errorf("manifest name required")
	}
	return nil
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || seen[strings.ToLower(v)] {
			continue
		}
		seen[strings.ToLower(v)] = true
		out = append(out, v)
	}
	return out
}

func dedupeSurfaces(in []engine.APISurface) []engine.APISurface {
	seen := make(map[string]bool)
	var out []engine.APISurface
	for _, v := range in {
		key := strings.ToLower(string(v))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, v)
	}
	return out
}

// IsEnabled returns whether the service is enabled (defaults to true if nil).
func (m *Manifest) IsEnabled() bool {
	if m == nil || m.Enabled == nil {
		return true
	}
	return *m.Enabled
}

// SetEnabled sets the enabled flag.
func (m *Manifest) SetEnabled(enabled bool) {
	m.Enabled = &enabled
}

// HasCapability checks if the manifest declares a specific capability.
func (m *Manifest) HasCapability(cap string) bool {
	if m == nil {
		return false
	}
	capLower := strings.ToLower(strings.TrimSpace(cap))
	for _, c := range m.Capabilities {
		if strings.ToLower(c) == capLower {
			return true
		}
	}
	return false
}

// HasTag checks if the manifest has a specific tag key.
func (m *Manifest) HasTag(key string) bool {
	if m == nil || m.Tags == nil {
		return false
	}
	_, ok := m.Tags[key]
	return ok
}

// GetTag returns the value for a tag key.
func (m *Manifest) GetTag(key string) (string, bool) {
	if m == nil || m.Tags == nil {
		return "", false
	}
	v, ok := m.Tags[key]
	return v, ok
}

// SetTag sets a tag key-value pair.
func (m *Manifest) SetTag(key, value string) {
	if m.Tags == nil {
		m.Tags = make(map[string]string)
	}
	m.Tags[key] = value
}

// RequiresAPI checks if the manifest requires a specific API surface.
func (m *Manifest) RequiresAPI(api engine.APISurface) bool {
	if m == nil {
		return false
	}
	apiLower := strings.ToLower(string(api))
	for _, a := range m.RequiresAPIs {
		if strings.ToLower(string(a)) == apiLower {
			return true
		}
	}
	return false
}

// DependsOnService checks if the manifest depends on a specific service.
func (m *Manifest) DependsOnService(svc string) bool {
	if m == nil {
		return false
	}
	svcLower := strings.ToLower(strings.TrimSpace(svc))
	for _, d := range m.DependsOn {
		if strings.ToLower(d) == svcLower {
			return true
		}
	}
	return false
}

// GetQuota returns the quota value for a key.
func (m *Manifest) GetQuota(key string) (string, bool) {
	if m == nil || m.Quotas == nil {
		return "", false
	}
	v, ok := m.Quotas[key]
	return v, ok
}

// SetQuota sets a quota key-value pair.
func (m *Manifest) SetQuota(key, value string) {
	if m.Quotas == nil {
		m.Quotas = make(map[string]string)
	}
	m.Quotas[key] = value
}

// SetCodeHash sets the code hash for verification.
// Aligned with ServiceRegistry.cs contract CodeHash field.
func (m *Manifest) SetCodeHash(hash string) {
	m.CodeHash = strings.TrimSpace(hash)
}

// SetConfigHash sets the config hash for verification.
// Aligned with ServiceRegistry.cs contract ConfigHash field.
func (m *Manifest) SetConfigHash(hash string) {
	m.ConfigHash = strings.TrimSpace(hash)
}

// VerifyCodeHash checks if the provided hash matches the manifest's code hash.
// Returns true if hashes match or if no code hash is set.
func (m *Manifest) VerifyCodeHash(hash string) bool {
	if m == nil || m.CodeHash == "" {
		return true // No verification required
	}
	return strings.EqualFold(m.CodeHash, strings.TrimSpace(hash))
}

// VerifyConfigHash checks if the provided hash matches the manifest's config hash.
// Returns true if hashes match or if no config hash is set.
func (m *Manifest) VerifyConfigHash(hash string) bool {
	if m == nil || m.ConfigHash == "" {
		return true // No verification required
	}
	return strings.EqualFold(m.ConfigHash, strings.TrimSpace(hash))
}

// Merge combines another manifest into this one. The other manifest's
// non-empty values take precedence. Lists and maps are merged additively.
func (m *Manifest) Merge(other *Manifest) {
	if m == nil || other == nil {
		return
	}

	// Non-empty strings take precedence
	if other.Name != "" {
		m.Name = other.Name
	}
	if other.Domain != "" {
		m.Domain = other.Domain
	}
	if other.Description != "" {
		m.Description = other.Description
	}
	if other.Version != "" {
		m.Version = other.Version
	}
	if other.CodeHash != "" {
		m.CodeHash = other.CodeHash
	}
	if other.ConfigHash != "" {
		m.ConfigHash = other.ConfigHash
	}
	if other.Layer != "" {
		m.Layer = other.Layer
	}
	if other.Enabled != nil {
		m.Enabled = other.Enabled
	}

	// Merge lists (additive)
	m.RequiresAPIs = append(m.RequiresAPIs, other.RequiresAPIs...)
	m.DependsOn = append(m.DependsOn, other.DependsOn...)
	m.Capabilities = append(m.Capabilities, other.Capabilities...)

	// Merge maps
	if len(other.Quotas) > 0 {
		if m.Quotas == nil {
			m.Quotas = make(map[string]string)
		}
		for k, v := range other.Quotas {
			m.Quotas[k] = v
		}
	}
	if len(other.Tags) > 0 {
		if m.Tags == nil {
			m.Tags = make(map[string]string)
		}
		for k, v := range other.Tags {
			m.Tags[k] = v
		}
	}
}

// Clone creates a deep copy of the manifest.
func (m *Manifest) Clone() *Manifest {
	if m == nil {
		return nil
	}

	clone := &Manifest{
		Name:        m.Name,
		Domain:      m.Domain,
		Description: m.Description,
		Version:     m.Version,
		CodeHash:    m.CodeHash,
		ConfigHash:  m.ConfigHash,
		Layer:       m.Layer,
	}

	if m.Enabled != nil {
		enabled := *m.Enabled
		clone.Enabled = &enabled
	}

	if len(m.RequiresAPIs) > 0 {
		clone.RequiresAPIs = make([]engine.APISurface, len(m.RequiresAPIs))
		copy(clone.RequiresAPIs, m.RequiresAPIs)
	}

	if len(m.DependsOn) > 0 {
		clone.DependsOn = make([]string, len(m.DependsOn))
		copy(clone.DependsOn, m.DependsOn)
	}

	if len(m.Capabilities) > 0 {
		clone.Capabilities = make([]string, len(m.Capabilities))
		copy(clone.Capabilities, m.Capabilities)
	}

	if len(m.Quotas) > 0 {
		clone.Quotas = make(map[string]string, len(m.Quotas))
		for k, v := range m.Quotas {
			clone.Quotas[k] = v
		}
	}

	if len(m.Tags) > 0 {
		clone.Tags = make(map[string]string, len(m.Tags))
		for k, v := range m.Tags {
			clone.Tags[k] = v
		}
	}

	return clone
}

// ToDescriptor converts the manifest to a service.Descriptor for engine integration.
func (m *Manifest) ToDescriptor() service.Descriptor {
	if m == nil {
		return service.Descriptor{}
	}

	d := service.Descriptor{
		Name:         m.Name,
		Domain:       m.Domain,
		Capabilities: make([]string, len(m.Capabilities)),
		DependsOn:    make([]string, len(m.DependsOn)),
	}

	// Convert layer
	switch strings.ToLower(m.Layer) {
	case "service":
		d.Layer = service.LayerService
	case "runner":
		d.Layer = service.LayerRunner
	case "infra":
		d.Layer = service.LayerInfra
	case "platform":
		d.Layer = service.LayerPlatform
	default:
		d.Layer = service.LayerService // default
	}

	copy(d.Capabilities, m.Capabilities)
	copy(d.DependsOn, m.DependsOn)

	// Convert RequiresAPIs to string slice
	d.RequiresAPIs = make([]string, len(m.RequiresAPIs))
	for i, api := range m.RequiresAPIs {
		d.RequiresAPIs[i] = string(api)
	}

	return d
}

// ManifestFromDescriptor creates a Manifest from a service.Descriptor.
func ManifestFromDescriptor(d service.Descriptor) *Manifest {
	m := &Manifest{
		Name:         d.Name,
		Domain:       d.Domain,
		Layer:        string(d.Layer),
		Capabilities: make([]string, len(d.Capabilities)),
		DependsOn:    make([]string, len(d.DependsOn)),
	}

	copy(m.Capabilities, d.Capabilities)
	copy(m.DependsOn, d.DependsOn)

	m.RequiresAPIs = make([]engine.APISurface, len(d.RequiresAPIs))
	for i, api := range d.RequiresAPIs {
		m.RequiresAPIs[i] = engine.APISurface(api)
	}

	return m
}

// ManifestValidator is an interface for custom manifest validation.
type ManifestValidator interface {
	ValidateManifest(m *Manifest) error
}

// ValidateWith runs the manifest through a custom validator.
func (m *Manifest) ValidateWith(v ManifestValidator) error {
	if v == nil {
		return nil
	}
	return v.ValidateManifest(m)
}

// ManifestValidatorFunc is a function type that implements ManifestValidator.
type ManifestValidatorFunc func(*Manifest) error

// ValidateManifest implements ManifestValidator.
func (f ManifestValidatorFunc) ValidateManifest(m *Manifest) error {
	return f(m)
}

// ManifestBuilder provides a fluent API for constructing service manifests.
// This reduces boilerplate in service implementations.
type ManifestBuilder struct {
	manifest *Manifest
}

// NewManifestBuilder creates a builder for a service manifest.
// The name is used as the default for domain and capabilities.
func NewManifestBuilder(name, description string) *ManifestBuilder {
	return &ManifestBuilder{
		manifest: &Manifest{
			Name:         strings.TrimSpace(name),
			Domain:       strings.TrimSpace(name),
			Description:  strings.TrimSpace(description),
			Layer:        "service",
			DependsOn:    []string{"store", "svc-accounts"},
			RequiresAPIs: []engine.APISurface{engine.APISurfaceStore},
			Capabilities: []string{strings.TrimSpace(name)},
			Quotas:       make(map[string]string),
			Tags:         make(map[string]string),
		},
	}
}

// Domain sets the service domain (defaults to name).
func (b *ManifestBuilder) Domain(domain string) *ManifestBuilder {
	b.manifest.Domain = strings.TrimSpace(domain)
	return b
}

// Version sets the service version.
func (b *ManifestBuilder) Version(version string) *ManifestBuilder {
	b.manifest.Version = strings.TrimSpace(version)
	return b
}

// Layer sets the service layer (service, runner, infra, platform).
func (b *ManifestBuilder) Layer(layer string) *ManifestBuilder {
	b.manifest.Layer = strings.TrimSpace(strings.ToLower(layer))
	return b
}

// DependsOn sets the service dependencies (replaces defaults).
func (b *ManifestBuilder) DependsOn(deps ...string) *ManifestBuilder {
	b.manifest.DependsOn = deps
	return b
}

// AddDependency adds a dependency to the existing list.
func (b *ManifestBuilder) AddDependency(dep string) *ManifestBuilder {
	b.manifest.DependsOn = append(b.manifest.DependsOn, strings.TrimSpace(dep))
	return b
}

// RequiresAPIs sets the required API surfaces (replaces defaults).
func (b *ManifestBuilder) RequiresAPIs(apis ...engine.APISurface) *ManifestBuilder {
	b.manifest.RequiresAPIs = apis
	return b
}

// AddAPI adds an API surface to the existing list.
func (b *ManifestBuilder) AddAPI(api engine.APISurface) *ManifestBuilder {
	b.manifest.RequiresAPIs = append(b.manifest.RequiresAPIs, api)
	return b
}

// Capabilities sets the service capabilities (replaces defaults).
func (b *ManifestBuilder) Capabilities(caps ...string) *ManifestBuilder {
	b.manifest.Capabilities = caps
	return b
}

// AddCapability adds a capability to the existing list.
func (b *ManifestBuilder) AddCapability(cap string) *ManifestBuilder {
	b.manifest.Capabilities = append(b.manifest.Capabilities, strings.TrimSpace(cap))
	return b
}

// Quota sets a quota key-value pair.
func (b *ManifestBuilder) Quota(key, value string) *ManifestBuilder {
	b.manifest.Quotas[strings.TrimSpace(key)] = strings.TrimSpace(value)
	return b
}

// Tag sets a tag key-value pair.
func (b *ManifestBuilder) Tag(key, value string) *ManifestBuilder {
	b.manifest.Tags[strings.TrimSpace(key)] = strings.TrimSpace(value)
	return b
}

// Enabled sets whether the service is enabled.
func (b *ManifestBuilder) Enabled(enabled bool) *ManifestBuilder {
	b.manifest.Enabled = &enabled
	return b
}

// Build returns the constructed manifest.
// The manifest is normalized before being returned.
func (b *ManifestBuilder) Build() *Manifest {
	b.manifest.Normalize()
	return b.manifest
}

// BuildClone returns a clone of the constructed manifest.
// Useful when the builder will be reused.
func (b *ManifestBuilder) BuildClone() *Manifest {
	return b.Build().Clone()
}

// ServiceManifest is a convenience function to create a standard service manifest.
// This is the most common pattern for service packages.
func ServiceManifest(name, description string) *Manifest {
	return NewManifestBuilder(name, description).Build()
}

// ServiceManifestWithAPIs creates a manifest with additional API requirements.
func ServiceManifestWithAPIs(name, description string, apis ...engine.APISurface) *Manifest {
	builder := NewManifestBuilder(name, description)
	for _, api := range apis {
		builder.AddAPI(api)
	}
	return builder.Build()
}
