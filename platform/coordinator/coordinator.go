// Package coordinator implements the MarbleRun-style Coordinator pattern.
// The Coordinator is the control plane that verifies service integrity,
// manages secrets, and orchestrates the confidential microservices mesh.
package coordinator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/tee"
	"github.com/R3E-Network/service_layer/tee/types"
)

// =============================================================================
// Coordinator State Machine
// =============================================================================

// State represents the coordinator's lifecycle state.
type State int

const (
	// StateUninitialized - coordinator not yet started
	StateUninitialized State = iota

	// StateRecovery - waiting for recovery keys to unseal state
	StateRecovery

	// StateAcceptingManifest - ready to accept a manifest
	StateAcceptingManifest

	// StateAcceptingMarbles - manifest set, accepting marble activations
	StateAcceptingMarbles

	// StateMax - sentinel value
	StateMax
)

func (s State) String() string {
	switch s {
	case StateUninitialized:
		return "Uninitialized"
	case StateRecovery:
		return "Recovery"
	case StateAcceptingManifest:
		return "AcceptingManifest"
	case StateAcceptingMarbles:
		return "AcceptingMarbles"
	default:
		return "Unknown"
	}
}

// =============================================================================
// Coordinator
// =============================================================================

// Coordinator manages the confidential microservices mesh.
// It verifies marble integrity, distributes secrets, and manages TLS.
type Coordinator struct {
	mu sync.RWMutex

	// Configuration
	config Config

	// Core components
	trustRoot *tee.TrustRoot
	manifest  *os.Manifest
	state     State

	// Managers
	secretMgr   *SecretManager
	recoveryMgr *RecoveryManager
	tlsMgr      *TLSManager
	packageMgr  *PackageManager
	userMgr     *UserManager

	// Active marbles
	marbles map[string]*MarbleInstance

	// Metrics
	activationCount   uint64
	lastActivationAt  time.Time
	failedActivations uint64
}

// Config holds coordinator configuration.
type Config struct {
	// TrustRoot is the TEE trust root
	TrustRoot *tee.TrustRoot

	// SealMode determines how state is sealed
	SealMode SealMode

	// RecoveryThreshold is the minimum recovery keys needed
	RecoveryThreshold int

	// DNSNames for the coordinator's TLS certificate
	DNSNames []string

	// DebugMode enables debug logging
	DebugMode bool
}

// SealMode determines how coordinator state is sealed.
type SealMode string

const (
	// SealModeProductKey seals to the product key (survives updates)
	SealModeProductKey SealMode = "ProductKey"

	// SealModeUniqueKey seals to the unique enclave key
	SealModeUniqueKey SealMode = "UniqueKey"

	// SealModeDisabled disables sealing (development only)
	SealModeDisabled SealMode = "Disabled"
)

// New creates a new Coordinator.
func New(cfg Config) (*Coordinator, error) {
	if cfg.TrustRoot == nil {
		return nil, fmt.Errorf("TrustRoot is required")
	}

	if cfg.SealMode == "" {
		cfg.SealMode = SealModeProductKey
	}

	c := &Coordinator{
		config:    cfg,
		trustRoot: cfg.TrustRoot,
		state:     StateUninitialized,
		marbles:   make(map[string]*MarbleInstance),
	}

	return c, nil
}

// Start initializes and starts the coordinator.
func (c *Coordinator) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state != StateUninitialized {
		return fmt.Errorf("coordinator already started (state: %s)", c.state)
	}

	// Initialize managers
	var err error

	c.secretMgr, err = NewSecretManager(c.trustRoot)
	if err != nil {
		return fmt.Errorf("create secret manager: %w", err)
	}

	c.recoveryMgr = NewRecoveryManager(c.config.RecoveryThreshold)

	c.tlsMgr, err = NewTLSManager(c.trustRoot)
	if err != nil {
		return fmt.Errorf("create TLS manager: %w", err)
	}

	c.packageMgr = NewPackageManager()
	c.userMgr = NewUserManager()

	// Try to recover sealed state
	if c.config.SealMode != SealModeDisabled {
		if err := c.tryRecoverState(ctx); err != nil {
			// No sealed state or recovery needed
			c.state = StateAcceptingManifest
		} else {
			// State recovered, ready for marbles
			c.state = StateAcceptingMarbles
		}
	} else {
		c.state = StateAcceptingManifest
	}

	return nil
}

// Stop shuts down the coordinator.
func (c *Coordinator) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Seal state if enabled
	if c.config.SealMode != SealModeDisabled && c.manifest != nil {
		if err := c.sealState(ctx); err != nil {
			return fmt.Errorf("seal state: %w", err)
		}
	}

	// Zero sensitive data
	if c.secretMgr != nil {
		c.secretMgr.Zero()
	}

	c.state = StateUninitialized
	return nil
}

// =============================================================================
// Manifest Management
// =============================================================================

// SetManifest sets the coordinator's manifest.
func (c *Coordinator) SetManifest(ctx context.Context, manifest *os.Manifest) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state != StateAcceptingManifest {
		return fmt.Errorf("cannot set manifest in state %s", c.state)
	}

	// Validate manifest
	if err := manifest.Validate(); err != nil {
		return fmt.Errorf("invalid manifest: %w", err)
	}

	// Initialize managers with manifest
	c.packageMgr.SetPackages(manifest.Packages)
	c.userMgr.SetUsersAndRoles(manifest.Users, manifest.Roles)

	// Set recovery keys
	if len(manifest.RecoveryKeys) > 0 {
		if err := c.recoveryMgr.SetRecoveryKeys(manifest.RecoveryKeys); err != nil {
			return fmt.Errorf("set recovery keys: %w", err)
		}
	}

	// Generate secrets
	for name, def := range manifest.Secrets {
		if !def.UserDefined {
			if err := c.secretMgr.GenerateSecret(ctx, name, &def); err != nil {
				return fmt.Errorf("generate secret %q: %w", name, err)
			}
		}
	}

	// Initialize TLS
	if err := c.tlsMgr.Initialize(ctx, &manifest.TLS); err != nil {
		return fmt.Errorf("initialize TLS: %w", err)
	}

	c.manifest = manifest
	c.state = StateAcceptingMarbles

	return nil
}

// GetManifest returns the current manifest (without secrets).
func (c *Coordinator) GetManifest() *os.Manifest {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.manifest
}

// =============================================================================
// Marble Activation
// =============================================================================

// ActivationRequest is sent by a marble to activate.
type ActivationRequest struct {
	// MarbleName is the marble type from the manifest
	MarbleName string

	// Quote is the attestation quote
	Quote *types.Quote

	// CSR is the certificate signing request (optional)
	CSR []byte

	// UUID is a unique identifier for this instance
	UUID string
}

// ActivationResponse is returned to an activated marble.
type ActivationResponse struct {
	// Secrets contains the marble's secrets
	Secrets map[string][]byte

	// Env contains environment variables
	Env map[string]string

	// Files contains file contents
	Files map[string][]byte

	// Certificate is the marble's TLS certificate (PEM)
	Certificate []byte

	// PrivateKey is the marble's TLS private key (PEM)
	PrivateKey []byte

	// RootCA is the mesh root CA certificate (PEM)
	RootCA []byte
}

// Activate activates a marble after verification.
func (c *Coordinator) Activate(ctx context.Context, req *ActivationRequest) (*ActivationResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state != StateAcceptingMarbles {
		return nil, fmt.Errorf("not accepting marbles (state: %s)", c.state)
	}

	// Get marble definition
	marble, ok := c.manifest.Marbles[req.MarbleName]
	if !ok {
		c.failedActivations++
		return nil, fmt.Errorf("unknown marble: %s", req.MarbleName)
	}

	// Check max activations
	if marble.MaxActivations > 0 {
		count := c.countActiveMarbles(req.MarbleName)
		if count >= marble.MaxActivations {
			c.failedActivations++
			return nil, fmt.Errorf("max activations reached for %s", req.MarbleName)
		}
	}

	// Verify quote against package
	if err := c.packageMgr.VerifyQuote(req.Quote, marble.Package); err != nil {
		c.failedActivations++
		return nil, fmt.Errorf("quote verification failed: %w", err)
	}

	// Collect secrets for this marble
	secretNames := c.manifest.GetMarbleSecrets(req.MarbleName)
	secrets := make(map[string][]byte, len(secretNames))
	for _, name := range secretNames {
		secret, err := c.secretMgr.GetSecret(ctx, name)
		if err != nil {
			c.failedActivations++
			return nil, fmt.Errorf("get secret %q: %w", name, err)
		}
		secrets[name] = secret
	}

	// Process environment variables (substitute secrets)
	env := make(map[string]string, len(marble.Parameters.Env))
	for k, v := range marble.Parameters.Env {
		env[k] = c.substituteSecrets(v, secrets)
	}

	// Process files (substitute secrets)
	files := make(map[string][]byte, len(marble.Parameters.Files))
	for path, content := range marble.Parameters.Files {
		files[path] = []byte(c.substituteSecrets(content, secrets))
	}

	// Issue TLS certificate
	var cert, key, rootCA []byte
	if len(marble.TLS.Incoming) > 0 || len(marble.TLS.Outgoing) > 0 {
		var err error
		cert, key, rootCA, err = c.tlsMgr.IssueCertificate(ctx, req.MarbleName, req.UUID)
		if err != nil {
			c.failedActivations++
			return nil, fmt.Errorf("issue certificate: %w", err)
		}
	}

	// Register marble instance
	instance := &MarbleInstance{
		Name:        req.MarbleName,
		UUID:        req.UUID,
		ActivatedAt: time.Now(),
		Quote:       req.Quote,
	}
	c.marbles[req.UUID] = instance

	c.activationCount++
	c.lastActivationAt = time.Now()

	return &ActivationResponse{
		Secrets:     secrets,
		Env:         env,
		Files:       files,
		Certificate: cert,
		PrivateKey:  key,
		RootCA:      rootCA,
	}, nil
}

// Deactivate removes a marble from the active set.
func (c *Coordinator) Deactivate(ctx context.Context, uuid string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.marbles[uuid]; !ok {
		return fmt.Errorf("marble not found: %s", uuid)
	}

	delete(c.marbles, uuid)
	return nil
}

// =============================================================================
// User-Defined Secrets
// =============================================================================

// SetSecret sets a user-defined secret.
func (c *Coordinator) SetSecret(ctx context.Context, user, name string, value []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Verify user has permission
	if !c.userMgr.HasPermission(user, "Secrets", name, "WriteSecret") {
		return fmt.Errorf("permission denied")
	}

	// Verify secret is user-defined
	def, ok := c.manifest.Secrets[name]
	if !ok {
		return fmt.Errorf("unknown secret: %s", name)
	}
	if !def.UserDefined {
		return fmt.Errorf("secret %s is not user-defined", name)
	}

	return c.secretMgr.SetUserSecret(ctx, name, value)
}

// =============================================================================
// Recovery
// =============================================================================

// GetRecoveryData returns encrypted recovery data for a user.
func (c *Coordinator) GetRecoveryData(ctx context.Context, user string) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.userMgr.HasPermission(user, "Recovery", "", "Recover") {
		return nil, fmt.Errorf("permission denied")
	}

	return c.recoveryMgr.GetRecoveryData(user)
}

// Recover recovers the coordinator using provided key shares.
func (c *Coordinator) Recover(ctx context.Context, shares map[string][]byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state != StateRecovery {
		return fmt.Errorf("not in recovery state")
	}

	sealingKey, err := c.recoveryMgr.RecoverSealingKey(shares)
	if err != nil {
		return fmt.Errorf("recover sealing key: %w", err)
	}

	// Unseal state with recovered key
	if err := c.unsealState(ctx, sealingKey); err != nil {
		return fmt.Errorf("unseal state: %w", err)
	}

	c.state = StateAcceptingMarbles
	return nil
}

// =============================================================================
// Status & Metrics
// =============================================================================

// Status returns the coordinator's status.
type Status struct {
	State             string    `json:"state"`
	ManifestHash      string    `json:"manifest_hash,omitempty"`
	ActiveMarbles     int       `json:"active_marbles"`
	ActivationCount   uint64    `json:"activation_count"`
	FailedActivations uint64    `json:"failed_activations"`
	LastActivationAt  time.Time `json:"last_activation_at,omitempty"`
}

// GetStatus returns the coordinator's status.
func (c *Coordinator) GetStatus() *Status {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := &Status{
		State:             c.state.String(),
		ActiveMarbles:     len(c.marbles),
		ActivationCount:   c.activationCount,
		FailedActivations: c.failedActivations,
		LastActivationAt:  c.lastActivationAt,
	}

	if c.manifest != nil {
		status.ManifestHash = c.manifestHash()
	}

	return status
}

// =============================================================================
// Internal Methods
// =============================================================================

func (c *Coordinator) countActiveMarbles(name string) uint {
	var count uint
	for _, m := range c.marbles {
		if m.Name == name {
			count++
		}
	}
	return count
}

func (c *Coordinator) substituteSecrets(template string, secrets map[string][]byte) string {
	result := template
	for name, value := range secrets {
		// Replace {{ .Secrets.name }} and {{ raw .Secrets.name }}
		result = replaceAll(result, "{{ .Secrets."+name+" }}", string(value))
		result = replaceAll(result, "{{.Secrets."+name+"}}", string(value))
		result = replaceAll(result, "{{ raw .Secrets."+name+" }}", string(value))
		result = replaceAll(result, "{{raw .Secrets."+name+"}}", string(value))
	}
	return result
}

func replaceAll(s, old, new string) string {
	for {
		idx := indexOf(s, old)
		if idx == -1 {
			return s
		}
		s = s[:idx] + new + s[idx+len(old):]
	}
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func (c *Coordinator) manifestHash() string {
	// Simple hash of manifest for identification
	h := sha256.New()
	for name := range c.manifest.Packages {
		h.Write([]byte(name))
	}
	for name := range c.manifest.Marbles {
		h.Write([]byte(name))
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func (c *Coordinator) tryRecoverState(ctx context.Context) error {
	// Try to unseal existing state
	// If no state exists, return error to indicate fresh start
	return fmt.Errorf("no sealed state found")
}

func (c *Coordinator) sealState(ctx context.Context) error {
	// Seal coordinator state using TEE sealing
	return nil
}

func (c *Coordinator) unsealState(ctx context.Context, key []byte) error {
	// Unseal coordinator state
	return nil
}

// =============================================================================
// Marble Instance
// =============================================================================

// MarbleInstance represents an active marble.
type MarbleInstance struct {
	Name        string
	UUID        string
	ActivatedAt time.Time
	Quote       *types.Quote
}
