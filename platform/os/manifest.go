// Package os provides the ServiceOS abstraction layer.
// This file defines the MarbleRun-style Manifest structure for confidential microservices.
package os

import (
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// =============================================================================
// MarbleRun-Style Manifest
// =============================================================================

// Manifest is the complete service mesh configuration (MarbleRun-style).
// It defines packages, marbles (services), secrets, users, roles, and TLS policies.
type Manifest struct {
	// Packages defines enclave software packages with verification parameters
	Packages map[string]Package `json:"Packages,omitempty" yaml:"Packages,omitempty"`

	// Marbles defines service instances and their configurations
	Marbles map[string]Marble `json:"Marbles,omitempty" yaml:"Marbles,omitempty"`

	// Secrets defines cryptographic secrets to be generated or provided
	Secrets map[string]Secret `json:"Secrets,omitempty" yaml:"Secrets,omitempty"`

	// Users defines authorized users with certificate-based authentication
	Users map[string]User `json:"Users,omitempty" yaml:"Users,omitempty"`

	// Roles defines permission roles for access control
	Roles map[string]Role `json:"Roles,omitempty" yaml:"Roles,omitempty"`

	// RecoveryKeys maps user names to RSA public keys (PEM) for multi-party recovery
	RecoveryKeys map[string]string `json:"RecoveryKeys,omitempty" yaml:"RecoveryKeys,omitempty"`

	// TLS defines TLS policies for inter-service communication
	TLS TLSConfig `json:"TLS,omitempty" yaml:"TLS,omitempty"`

	// Config holds service-specific configuration entries
	Config map[string]ConfigEntry `json:"Config,omitempty" yaml:"Config,omitempty"`
}

// =============================================================================
// Package Definition (Enclave Software)
// =============================================================================

// Package defines an enclave software package with verification parameters.
// These parameters are used to verify the integrity of a marble before activation.
type Package struct {
	// UniqueID is MRENCLAVE - the hash of the enclave code and data.
	// If set, only enclaves with this exact measurement can be activated.
	UniqueID string `json:"UniqueID,omitempty" yaml:"UniqueID,omitempty"`

	// SignerID is MRSIGNER - the hash of the enclave signing key.
	// If set, only enclaves signed by this key can be activated.
	SignerID string `json:"SignerID,omitempty" yaml:"SignerID,omitempty"`

	// ProductID identifies the product within a signer's portfolio.
	ProductID uint16 `json:"ProductID,omitempty" yaml:"ProductID,omitempty"`

	// SecurityVersion is the minimum required security version (SVN).
	// Enclaves with lower SVN will be rejected.
	SecurityVersion uint16 `json:"SecurityVersion,omitempty" yaml:"SecurityVersion,omitempty"`

	// Debug allows debug-mode enclaves (development only, insecure).
	Debug bool `json:"Debug,omitempty" yaml:"Debug,omitempty"`

	// AcceptedTCBStatuses lists acceptable TCB status values.
	// Default: ["UpToDate", "SWHardeningNeeded"]
	AcceptedTCBStatuses []string `json:"AcceptedTCBStatuses,omitempty" yaml:"AcceptedTCBStatuses,omitempty"`
}

// =============================================================================
// Marble Definition (Service Instance)
// =============================================================================

// Marble defines a service instance configuration.
type Marble struct {
	// Package references a Package definition by name
	Package string `json:"Package" yaml:"Package"`

	// Parameters defines environment variables and files for the marble
	Parameters MarbleParameters `json:"Parameters,omitempty" yaml:"Parameters,omitempty"`

	// TLS defines TLS configuration for this marble's endpoints
	TLS MarbleTLS `json:"TLS,omitempty" yaml:"TLS,omitempty"`

	// MaxActivations limits concurrent instances (0 = unlimited)
	MaxActivations uint `json:"MaxActivations,omitempty" yaml:"MaxActivations,omitempty"`

	// Capabilities lists required capabilities (for ServiceOS integration)
	Capabilities []Capability `json:"Capabilities,omitempty" yaml:"Capabilities,omitempty"`

	// ResourceLimits specifies resource constraints
	ResourceLimits ResourceLimits `json:"ResourceLimits,omitempty" yaml:"ResourceLimits,omitempty"`
}

// ResourceLimits specifies resource constraints for a service.
type ResourceLimits struct {
	MaxMemory      int64         `json:"max_memory,omitempty" yaml:"MaxMemory,omitempty"`       // Max memory in bytes
	MaxCPUTime     time.Duration `json:"max_cpu_time,omitempty" yaml:"MaxCPUTime,omitempty"`   // Max CPU time
	MaxNetworkReqs int           `json:"max_network_reqs,omitempty" yaml:"MaxNetworkReqs,omitempty"` // Max network requests per minute
	MaxSecrets     int           `json:"max_secrets,omitempty" yaml:"MaxSecrets,omitempty"`    // Max secrets
}

// MarbleParameters defines parameters passed to a marble on activation.
type MarbleParameters struct {
	// Env maps environment variable names to values.
	// Values can contain secret references: {{ .Secrets.secret_name }}
	Env map[string]string `json:"Env,omitempty" yaml:"Env,omitempty"`

	// Files maps file paths to contents.
	// Contents can contain secret references: {{ raw .Secrets.secret_name }}
	Files map[string]string `json:"Files,omitempty" yaml:"Files,omitempty"`

	// Argv are command-line arguments
	Argv []string `json:"Argv,omitempty" yaml:"Argv,omitempty"`
}

// MarbleTLS defines TLS configuration for a marble.
type MarbleTLS struct {
	// Incoming defines TLS for incoming connections (server-side)
	Incoming []TLSEndpoint `json:"Incoming,omitempty" yaml:"Incoming,omitempty"`

	// Outgoing defines TLS for outgoing connections (client-side)
	Outgoing []TLSEndpoint `json:"Outgoing,omitempty" yaml:"Outgoing,omitempty"`
}

// TLSEndpoint defines a TLS endpoint configuration.
type TLSEndpoint struct {
	// Port is the port number or name
	Port string `json:"Port" yaml:"Port"`

	// Cert is the certificate secret name
	Cert string `json:"Cert" yaml:"Cert"`

	// AllowedMarbles lists marbles allowed to connect (empty = all)
	AllowedMarbles []string `json:"AllowedMarbles,omitempty" yaml:"AllowedMarbles,omitempty"`

	// DisableClientAuth disables mutual TLS (server-side only)
	DisableClientAuth bool `json:"DisableClientAuth,omitempty" yaml:"DisableClientAuth,omitempty"`
}

// =============================================================================
// Secret Definition
// =============================================================================

// SecretType defines the type of secret.
type SecretType string

const (
	// SecretTypeSymmetricKey is a symmetric encryption key
	SecretTypeSymmetricKey SecretType = "symmetric-key"

	// SecretTypeCertRSA is an RSA certificate and private key
	SecretTypeCertRSA SecretType = "cert-rsa"

	// SecretTypeCertECDSA is an ECDSA certificate and private key
	SecretTypeCertECDSA SecretType = "cert-ecdsa"

	// SecretTypeCertED25519 is an Ed25519 certificate and private key
	SecretTypeCertED25519 SecretType = "cert-ed25519"

	// SecretTypePlain is a plain secret (user-provided or static)
	SecretTypePlain SecretType = "plain"
)

// Secret defines a cryptographic secret.
type Secret struct {
	// Type specifies the secret type
	Type SecretType `json:"Type" yaml:"Type"`

	// Size is the key size in bits (for symmetric-key)
	Size uint `json:"Size,omitempty" yaml:"Size,omitempty"`

	// Shared indicates if the secret is shared across all marbles of the same type
	Shared bool `json:"Shared,omitempty" yaml:"Shared,omitempty"`

	// UserDefined indicates the secret must be provided by a user
	UserDefined bool `json:"UserDefined,omitempty" yaml:"UserDefined,omitempty"`

	// Cert defines certificate parameters (for cert-* types)
	Cert *CertConfig `json:"Cert,omitempty" yaml:"Cert,omitempty"`

	// Private holds the private key (for user-defined secrets)
	Private string `json:"Private,omitempty" yaml:"Private,omitempty"`

	// Public holds the public key or certificate (for user-defined secrets)
	Public string `json:"Public,omitempty" yaml:"Public,omitempty"`
}

// CertConfig defines certificate generation parameters.
type CertConfig struct {
	// Subject defines the certificate subject
	Subject pkix.Name `json:"Subject,omitempty" yaml:"Subject,omitempty"`

	// DNSNames are Subject Alternative Names (DNS)
	DNSNames []string `json:"DNSNames,omitempty" yaml:"DNSNames,omitempty"`

	// IPAddresses are Subject Alternative Names (IP)
	IPAddresses []string `json:"IPAddresses,omitempty" yaml:"IPAddresses,omitempty"`

	// ValidityDays is the certificate validity period
	ValidityDays int `json:"ValidityDays,omitempty" yaml:"ValidityDays,omitempty"`

	// IsCA indicates if this is a CA certificate
	IsCA bool `json:"IsCA,omitempty" yaml:"IsCA,omitempty"`

	// SignedBy references another secret to sign this certificate
	SignedBy string `json:"SignedBy,omitempty" yaml:"SignedBy,omitempty"`
}

// =============================================================================
// User & Role Definitions
// =============================================================================

// User defines an authorized user.
type User struct {
	// Certificate is the user's X.509 certificate (PEM format)
	Certificate string `json:"Certificate" yaml:"Certificate"`

	// Roles lists the roles assigned to this user
	Roles []string `json:"Roles" yaml:"Roles"`
}

// Role defines a permission role.
type Role struct {
	// ResourceType specifies what type of resource this role applies to
	// Values: "Packages", "Marbles", "Secrets", "Status", "Recovery"
	ResourceType string `json:"ResourceType" yaml:"ResourceType"`

	// ResourceNames lists specific resources (empty = all resources of type)
	ResourceNames []string `json:"ResourceNames,omitempty" yaml:"ResourceNames,omitempty"`

	// Actions lists permitted actions
	// Values: "ReadSecret", "WriteSecret", "SignQuote", "UpdateManifest", "Recover"
	Actions []string `json:"Actions" yaml:"Actions"`
}

// =============================================================================
// TLS Configuration
// =============================================================================

// TLSConfig defines global TLS policies.
type TLSConfig struct {
	// Incoming defines default incoming TLS policies per marble type
	Incoming map[string][]TLSPolicy `json:"Incoming,omitempty" yaml:"Incoming,omitempty"`

	// Outgoing defines default outgoing TLS policies per marble type
	Outgoing map[string][]TLSPolicy `json:"Outgoing,omitempty" yaml:"Outgoing,omitempty"`
}

// TLSPolicy defines a TLS policy.
type TLSPolicy struct {
	// Port is the port to apply this policy to
	Port string `json:"Port" yaml:"Port"`

	// Cert is the certificate secret name
	Cert string `json:"Cert" yaml:"Cert"`

	// AllowedMarbles lists allowed peer marbles
	AllowedMarbles []string `json:"AllowedMarbles,omitempty" yaml:"AllowedMarbles,omitempty"`

	// DisableClientAuth disables mTLS
	DisableClientAuth bool `json:"DisableClientAuth,omitempty" yaml:"DisableClientAuth,omitempty"`
}

// =============================================================================
// Config Entry
// =============================================================================

// ConfigEntry holds a configuration value.
type ConfigEntry struct {
	// Value is the configuration value
	Value string `json:"Value" yaml:"Value"`

	// Secret indicates this value should be treated as sensitive
	Secret bool `json:"Secret,omitempty" yaml:"Secret,omitempty"`
}

// =============================================================================
// Manifest Loading & Validation
// =============================================================================

// LoadManifest loads a manifest from a file (JSON or YAML).
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest file: %w", err)
	}

	return ParseManifest(data, path)
}

// ParseManifest parses manifest data.
func ParseManifest(data []byte, filename string) (*Manifest, error) {
	var m Manifest

	// Determine format from filename or try both
	if strings.HasSuffix(filename, ".yaml") || strings.HasSuffix(filename, ".yml") {
		if err := yaml.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("parse YAML manifest: %w", err)
		}
	} else if strings.HasSuffix(filename, ".json") {
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("parse JSON manifest: %w", err)
		}
	} else {
		// Try JSON first, then YAML
		if err := json.Unmarshal(data, &m); err != nil {
			if err := yaml.Unmarshal(data, &m); err != nil {
				return nil, fmt.Errorf("parse manifest (tried JSON and YAML): %w", err)
			}
		}
	}

	return &m, nil
}

// Validate validates the manifest.
func (m *Manifest) Validate() error {
	var errs []string

	// Validate packages
	for name, pkg := range m.Packages {
		if err := pkg.Validate(); err != nil {
			errs = append(errs, fmt.Sprintf("package %q: %v", name, err))
		}
	}

	// Validate marbles
	for name, marble := range m.Marbles {
		if err := marble.Validate(m); err != nil {
			errs = append(errs, fmt.Sprintf("marble %q: %v", name, err))
		}
	}

	// Validate secrets
	for name, secret := range m.Secrets {
		if err := secret.Validate(); err != nil {
			errs = append(errs, fmt.Sprintf("secret %q: %v", name, err))
		}
	}

	// Validate users
	for name, user := range m.Users {
		if err := user.Validate(m); err != nil {
			errs = append(errs, fmt.Sprintf("user %q: %v", name, err))
		}
	}

	// Validate roles
	for name, role := range m.Roles {
		if err := role.Validate(); err != nil {
			errs = append(errs, fmt.Sprintf("role %q: %v", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("manifest validation failed:\n  %s", strings.Join(errs, "\n  "))
	}

	return nil
}

// Validate validates a package definition.
func (p *Package) Validate() error {
	// At least one verification parameter should be set (unless debug)
	if !p.Debug && p.UniqueID == "" && p.SignerID == "" {
		return fmt.Errorf("at least UniqueID or SignerID must be set (or Debug=true)")
	}
	return nil
}

// Validate validates a marble definition.
func (m *Marble) Validate(manifest *Manifest) error {
	if m.Package == "" {
		return fmt.Errorf("Package is required")
	}

	// Check package exists
	if _, ok := manifest.Packages[m.Package]; !ok {
		return fmt.Errorf("references unknown package %q", m.Package)
	}

	// Validate TLS references
	for _, ep := range m.TLS.Incoming {
		if ep.Cert != "" {
			if _, ok := manifest.Secrets[ep.Cert]; !ok {
				return fmt.Errorf("TLS incoming references unknown secret %q", ep.Cert)
			}
		}
	}

	for _, ep := range m.TLS.Outgoing {
		if ep.Cert != "" {
			if _, ok := manifest.Secrets[ep.Cert]; !ok {
				return fmt.Errorf("TLS outgoing references unknown secret %q", ep.Cert)
			}
		}
	}

	return nil
}

// Validate validates a secret definition.
func (s *Secret) Validate() error {
	switch s.Type {
	case SecretTypeSymmetricKey:
		if s.Size == 0 {
			return fmt.Errorf("Size is required for symmetric-key")
		}
		if s.Size != 128 && s.Size != 192 && s.Size != 256 {
			return fmt.Errorf("Size must be 128, 192, or 256 for symmetric-key")
		}
	case SecretTypeCertRSA, SecretTypeCertECDSA, SecretTypeCertED25519:
		// Cert config is optional, defaults will be used
	case SecretTypePlain:
		// Plain secrets can be user-defined or have a value
	default:
		return fmt.Errorf("unknown secret type %q", s.Type)
	}
	return nil
}

// Validate validates a user definition.
func (u *User) Validate(manifest *Manifest) error {
	if u.Certificate == "" {
		return fmt.Errorf("Certificate is required")
	}

	// Check roles exist
	for _, roleName := range u.Roles {
		if _, ok := manifest.Roles[roleName]; !ok {
			return fmt.Errorf("references unknown role %q", roleName)
		}
	}

	return nil
}

// Validate validates a role definition.
func (r *Role) Validate() error {
	validTypes := map[string]bool{
		"Packages": true, "Marbles": true, "Secrets": true,
		"Status": true, "Recovery": true, "Manifest": true,
	}

	if !validTypes[r.ResourceType] {
		return fmt.Errorf("invalid ResourceType %q", r.ResourceType)
	}

	if len(r.Actions) == 0 {
		return fmt.Errorf("at least one Action is required")
	}

	return nil
}

// =============================================================================
// Helper Methods
// =============================================================================

// GetMarbleSecrets returns all secrets referenced by a marble.
func (m *Manifest) GetMarbleSecrets(marbleName string) []string {
	marble, ok := m.Marbles[marbleName]
	if !ok {
		return nil
	}

	secretSet := make(map[string]bool)

	// Collect from TLS config
	for _, ep := range marble.TLS.Incoming {
		if ep.Cert != "" {
			secretSet[ep.Cert] = true
		}
	}
	for _, ep := range marble.TLS.Outgoing {
		if ep.Cert != "" {
			secretSet[ep.Cert] = true
		}
	}

	// Collect from parameters (parse template references)
	for _, v := range marble.Parameters.Env {
		for _, ref := range parseSecretRefs(v) {
			secretSet[ref] = true
		}
	}
	for _, v := range marble.Parameters.Files {
		for _, ref := range parseSecretRefs(v) {
			secretSet[ref] = true
		}
	}

	secrets := make([]string, 0, len(secretSet))
	for s := range secretSet {
		secrets = append(secrets, s)
	}
	return secrets
}

// parseSecretRefs extracts secret references from a template string.
// Supports: {{ .Secrets.name }}, {{ raw .Secrets.name }}
func parseSecretRefs(template string) []string {
	var refs []string
	// Simple parser for {{ .Secrets.name }} and {{ raw .Secrets.name }}
	for {
		start := strings.Index(template, "{{ ")
		if start == -1 {
			start = strings.Index(template, "{{")
		}
		if start == -1 {
			break
		}

		end := strings.Index(template[start:], "}}")
		if end == -1 {
			break
		}
		end += start

		content := strings.TrimSpace(template[start+2 : end])
		content = strings.TrimPrefix(content, "raw ")
		content = strings.TrimSpace(content)

		if strings.HasPrefix(content, ".Secrets.") {
			ref := strings.TrimPrefix(content, ".Secrets.")
			refs = append(refs, ref)
		}

		template = template[end+2:]
	}
	return refs
}

// ToLegacyManifest converts to the legacy Manifest format for backward compatibility.
func (m *Manifest) ToLegacyManifest(marbleName string) *LegacyManifest {
	marble, ok := m.Marbles[marbleName]
	if !ok {
		return nil
	}

	return &LegacyManifest{
		ServiceID:            marbleName,
		Version:              marble.Parameters.Env["VERSION"],
		Description:          marble.Parameters.Env["DESCRIPTION"],
		RequiredCapabilities: marble.Capabilities,
		ResourceLimits:       marble.ResourceLimits,
	}
}

// =============================================================================
// Legacy Manifest (Backward Compatibility)
// =============================================================================

// LegacyManifest is the old manifest format for backward compatibility.
type LegacyManifest struct {
	ServiceID            string         `json:"service_id"`
	Version              string         `json:"version"`
	Description          string         `json:"description"`
	AllowedHosts         []string       `json:"allowed_hosts,omitempty"`
	RequiredCapabilities []Capability   `json:"required_capabilities"`
	OptionalCapabilities []Capability   `json:"optional_capabilities"`
	ResourceLimits       ResourceLimits `json:"resource_limits"`
}

// ToManifest converts a legacy manifest to the new format.
func (lm *LegacyManifest) ToManifest() *Manifest {
	return &Manifest{
		Packages: map[string]Package{
			lm.ServiceID + "-pkg": {
				Debug: true, // Legacy manifests run in debug mode
			},
		},
		Marbles: map[string]Marble{
			lm.ServiceID: {
				Package: lm.ServiceID + "-pkg",
				Parameters: MarbleParameters{
					Env: map[string]string{
						"SERVICE_ID":  lm.ServiceID,
						"VERSION":     lm.Version,
						"DESCRIPTION": lm.Description,
					},
				},
				Capabilities:   lm.RequiredCapabilities,
				ResourceLimits: lm.ResourceLimits,
			},
		},
	}
}
