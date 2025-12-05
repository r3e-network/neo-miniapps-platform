// Package manifest defines the MarbleRun-compatible manifest structure
// for Neo Service Layer confidential microservices.
package manifest

import (
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Manifest is the complete service mesh configuration.
type Manifest struct {
	// Packages defines enclave software packages
	Packages map[string]Package `json:"Packages,omitempty" yaml:"Packages,omitempty"`

	// Marbles defines service instances
	Marbles map[string]Marble `json:"Marbles,omitempty" yaml:"Marbles,omitempty"`

	// Secrets defines cryptographic secrets
	Secrets map[string]Secret `json:"Secrets,omitempty" yaml:"Secrets,omitempty"`

	// Users defines authorized users
	Users map[string]User `json:"Users,omitempty" yaml:"Users,omitempty"`

	// Roles defines permission roles
	Roles map[string]Role `json:"Roles,omitempty" yaml:"Roles,omitempty"`

	// RecoveryKeys for multi-party recovery (name -> RSA public key PEM)
	RecoveryKeys map[string]string `json:"RecoveryKeys,omitempty" yaml:"RecoveryKeys,omitempty"`

	// TLS defines TLS policies
	TLS TLSConfig `json:"TLS,omitempty" yaml:"TLS,omitempty"`
}

// Package defines an enclave software package.
type Package struct {
	UniqueID            string   `json:"UniqueID,omitempty" yaml:"UniqueID,omitempty"`
	SignerID            string   `json:"SignerID,omitempty" yaml:"SignerID,omitempty"`
	ProductID           uint16   `json:"ProductID,omitempty" yaml:"ProductID,omitempty"`
	SecurityVersion     uint16   `json:"SecurityVersion,omitempty" yaml:"SecurityVersion,omitempty"`
	Debug               bool     `json:"Debug,omitempty" yaml:"Debug,omitempty"`
	AcceptedTCBStatuses []string `json:"AcceptedTCBStatuses,omitempty" yaml:"AcceptedTCBStatuses,omitempty"`
}

// Marble defines a service instance.
type Marble struct {
	Package        string           `json:"Package" yaml:"Package"`
	Parameters     MarbleParameters `json:"Parameters,omitempty" yaml:"Parameters,omitempty"`
	TLS            MarbleTLS        `json:"TLS,omitempty" yaml:"TLS,omitempty"`
	MaxActivations uint             `json:"MaxActivations,omitempty" yaml:"MaxActivations,omitempty"`
}

// MarbleParameters defines parameters for a Marble.
type MarbleParameters struct {
	Env   map[string]string `json:"Env,omitempty" yaml:"Env,omitempty"`
	Files map[string]string `json:"Files,omitempty" yaml:"Files,omitempty"`
	Argv  []string          `json:"Argv,omitempty" yaml:"Argv,omitempty"`
}

// MarbleTLS defines TLS configuration for a Marble.
type MarbleTLS struct {
	Incoming []TLSEndpoint `json:"Incoming,omitempty" yaml:"Incoming,omitempty"`
	Outgoing []TLSEndpoint `json:"Outgoing,omitempty" yaml:"Outgoing,omitempty"`
}

// TLSEndpoint defines a TLS endpoint.
type TLSEndpoint struct {
	Port              string   `json:"Port" yaml:"Port"`
	Cert              string   `json:"Cert" yaml:"Cert"`
	AllowedMarbles    []string `json:"AllowedMarbles,omitempty" yaml:"AllowedMarbles,omitempty"`
	DisableClientAuth bool     `json:"DisableClientAuth,omitempty" yaml:"DisableClientAuth,omitempty"`
}

// SecretType defines the type of secret.
type SecretType string

const (
	SecretTypeSymmetricKey SecretType = "symmetric-key"
	SecretTypeCertRSA      SecretType = "cert-rsa"
	SecretTypeCertECDSA    SecretType = "cert-ecdsa"
	SecretTypeCertED25519  SecretType = "cert-ed25519"
	SecretTypePlain        SecretType = "plain"
)

// Secret defines a cryptographic secret.
type Secret struct {
	Type        SecretType  `json:"Type" yaml:"Type"`
	Size        uint        `json:"Size,omitempty" yaml:"Size,omitempty"`
	Shared      bool        `json:"Shared,omitempty" yaml:"Shared,omitempty"`
	UserDefined bool        `json:"UserDefined,omitempty" yaml:"UserDefined,omitempty"`
	Cert        *CertConfig `json:"Cert,omitempty" yaml:"Cert,omitempty"`
	Private     string      `json:"Private,omitempty" yaml:"Private,omitempty"`
	Public      string      `json:"Public,omitempty" yaml:"Public,omitempty"`
}

// CertConfig defines certificate parameters.
type CertConfig struct {
	Subject      pkix.Name `json:"Subject,omitempty" yaml:"Subject,omitempty"`
	DNSNames     []string  `json:"DNSNames,omitempty" yaml:"DNSNames,omitempty"`
	IPAddresses  []string  `json:"IPAddresses,omitempty" yaml:"IPAddresses,omitempty"`
	ValidityDays int       `json:"ValidityDays,omitempty" yaml:"ValidityDays,omitempty"`
	IsCA         bool      `json:"IsCA,omitempty" yaml:"IsCA,omitempty"`
	SignedBy     string    `json:"SignedBy,omitempty" yaml:"SignedBy,omitempty"`
}

// User defines an authorized user.
type User struct {
	Certificate string   `json:"Certificate" yaml:"Certificate"`
	Roles       []string `json:"Roles" yaml:"Roles"`
}

// Role defines a permission role.
type Role struct {
	ResourceType  string   `json:"ResourceType" yaml:"ResourceType"`
	ResourceNames []string `json:"ResourceNames,omitempty" yaml:"ResourceNames,omitempty"`
	Actions       []string `json:"Actions" yaml:"Actions"`
}

// TLSConfig defines global TLS policies.
type TLSConfig struct {
	Incoming map[string][]TLSPolicy `json:"Incoming,omitempty" yaml:"Incoming,omitempty"`
	Outgoing map[string][]TLSPolicy `json:"Outgoing,omitempty" yaml:"Outgoing,omitempty"`
}

// TLSPolicy defines a TLS policy.
type TLSPolicy struct {
	Port              string   `json:"Port" yaml:"Port"`
	Cert              string   `json:"Cert" yaml:"Cert"`
	AllowedMarbles    []string `json:"AllowedMarbles,omitempty" yaml:"AllowedMarbles,omitempty"`
	DisableClientAuth bool     `json:"DisableClientAuth,omitempty" yaml:"DisableClientAuth,omitempty"`
}

// ResourceLimits specifies resource constraints.
type ResourceLimits struct {
	MaxMemory      int64         `json:"max_memory,omitempty" yaml:"MaxMemory,omitempty"`
	MaxCPUTime     time.Duration `json:"max_cpu_time,omitempty" yaml:"MaxCPUTime,omitempty"`
	MaxNetworkReqs int           `json:"max_network_reqs,omitempty" yaml:"MaxNetworkReqs,omitempty"`
	MaxSecrets     int           `json:"max_secrets,omitempty" yaml:"MaxSecrets,omitempty"`
}

// Load loads a manifest from a file.
func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	return Parse(data, path)
}

// Parse parses manifest data.
func Parse(data []byte, filename string) (*Manifest, error) {
	var m Manifest

	if strings.HasSuffix(filename, ".yaml") || strings.HasSuffix(filename, ".yml") {
		if err := yaml.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("parse YAML: %w", err)
		}
	} else if strings.HasSuffix(filename, ".json") {
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("parse JSON: %w", err)
		}
	} else {
		if err := json.Unmarshal(data, &m); err != nil {
			if err := yaml.Unmarshal(data, &m); err != nil {
				return nil, fmt.Errorf("parse manifest: %w", err)
			}
		}
	}

	return &m, nil
}

// Validate validates the manifest.
func (m *Manifest) Validate() error {
	var errs []string

	for name, pkg := range m.Packages {
		if err := pkg.Validate(); err != nil {
			errs = append(errs, fmt.Sprintf("package %q: %v", name, err))
		}
	}

	for name, marble := range m.Marbles {
		if err := marble.Validate(m); err != nil {
			errs = append(errs, fmt.Sprintf("marble %q: %v", name, err))
		}
	}

	for name, secret := range m.Secrets {
		if err := secret.Validate(); err != nil {
			errs = append(errs, fmt.Sprintf("secret %q: %v", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("validation failed:\n  %s", strings.Join(errs, "\n  "))
	}

	return nil
}

// Validate validates a package.
func (p *Package) Validate() error {
	if !p.Debug && p.UniqueID == "" && p.SignerID == "" {
		return fmt.Errorf("UniqueID or SignerID required (or Debug=true)")
	}
	return nil
}

// Validate validates a marble.
func (m *Marble) Validate(manifest *Manifest) error {
	if m.Package == "" {
		return fmt.Errorf("Package is required")
	}
	if _, ok := manifest.Packages[m.Package]; !ok {
		return fmt.Errorf("unknown package %q", m.Package)
	}
	return nil
}

// Validate validates a secret.
func (s *Secret) Validate() error {
	switch s.Type {
	case SecretTypeSymmetricKey:
		if s.Size == 0 {
			return fmt.Errorf("Size required for symmetric-key")
		}
	case SecretTypeCertRSA, SecretTypeCertECDSA, SecretTypeCertED25519, SecretTypePlain:
		// OK
	default:
		return fmt.Errorf("unknown type %q", s.Type)
	}
	return nil
}

// GetMarbleSecrets returns secrets referenced by a marble.
func (m *Manifest) GetMarbleSecrets(marbleName string) []string {
	marble, ok := m.Marbles[marbleName]
	if !ok {
		return nil
	}

	secretSet := make(map[string]bool)

	// From TLS
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

	// From parameters
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

func parseSecretRefs(template string) []string {
	var refs []string
	for {
		start := strings.Index(template, "{{")
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
			refs = append(refs, strings.TrimPrefix(content, ".Secrets."))
		}

		template = template[end+2:]
	}
	return refs
}
