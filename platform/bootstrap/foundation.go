// Package bootstrap wires the TEE trust root into the platform layer.
// It centralizes creation of the enclave-backed TrustRoot and hands out
// ServiceOS contexts so that all services are rooted in the TEE.
// Supabase integration is also managed here for database/auth/storage.
package bootstrap

import (
	"context"
	"fmt"

	"github.com/R3E-Network/service_layer/infra/supabase"
	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/tee"
	"github.com/R3E-Network/service_layer/tee/types"
)

// Config holds parameters for bringing up the TEE foundation.
type Config struct {
	// TEE configuration
	EnclaveID      string
	Mode           string // "simulation" or "hardware"
	SealingKeyPath string
	StoragePath    string
	AllowedHosts   []string
	PinnedCerts    map[string]string // host -> hex SHA256 fingerprint
	Debug          bool

	// Supabase configuration
	SupabaseURL          string
	SupabaseAnonKey      string // Will be stored in TEE vault
	SupabaseServiceKey   string // Will be stored in TEE vault
	SupabaseJWTSecret    string // Will be stored in TEE vault
}

// Foundation owns the TrustRoot and Supabase client, producing ServiceOS contexts.
// All service capabilities are backed by enclave components.
type Foundation struct {
	trustRoot *tee.TrustRoot
	supabase  *supabase.Client
}

// NewFoundation builds a TEE-backed foundation.
func NewFoundation(cfg Config) (*Foundation, error) {
	if cfg.EnclaveID == "" {
		return nil, fmt.Errorf("enclave_id is required")
	}

	mode := types.EnclaveModeSimulation
	if cfg.Mode == string(types.EnclaveModeHardware) {
		mode = types.EnclaveModeHardware
	}

	trustRoot, err := tee.New(tee.Config{
		EnclaveID:      cfg.EnclaveID,
		Mode:           mode,
		SealingKeyPath: cfg.SealingKeyPath,
		StoragePath:    cfg.StoragePath,
		AllowedHosts:   cfg.AllowedHosts,
		PinnedCerts:    cfg.PinnedCerts,
		DebugMode:      cfg.Debug,
	})
	if err != nil {
		return nil, fmt.Errorf("create trust root: %w", err)
	}

	return &Foundation{
		trustRoot: trustRoot,
	}, nil
}

// Start boots the enclave, stores Supabase keys in TEE vault, and initializes Supabase client.
func (f *Foundation) Start(ctx context.Context) error {
	// Start TEE first
	if err := f.trustRoot.Start(ctx); err != nil {
		return fmt.Errorf("start trust root: %w", err)
	}
	return nil
}

// InitSupabase initializes the Supabase client after TEE is started.
// Keys are stored in the TEE vault and never exposed in plaintext.
func (f *Foundation) InitSupabase(ctx context.Context, cfg Config) error {
	if cfg.SupabaseURL == "" {
		return nil // Supabase is optional
	}

	// Store Supabase keys in TEE vault (they will be encrypted)
	vault := f.trustRoot.Vault()

	if cfg.SupabaseAnonKey != "" {
		if err := vault.Store(ctx, "supabase", "anon_key", []byte(cfg.SupabaseAnonKey)); err != nil {
			return fmt.Errorf("store supabase anon key: %w", err)
		}
	}

	if cfg.SupabaseServiceKey != "" {
		if err := vault.Store(ctx, "supabase", "service_key", []byte(cfg.SupabaseServiceKey)); err != nil {
			return fmt.Errorf("store supabase service key: %w", err)
		}
	}

	if cfg.SupabaseJWTSecret != "" {
		if err := vault.Store(ctx, "supabase", "jwt_secret", []byte(cfg.SupabaseJWTSecret)); err != nil {
			return fmt.Errorf("store supabase jwt secret: %w", err)
		}
	}

	// Create Supabase client (keys are referenced by name, not value)
	client, err := supabase.New(f.trustRoot, supabase.Config{
		ProjectURL:       cfg.SupabaseURL,
		AnonKeySecret:    "anon_key",
		ServiceKeySecret: "service_key",
		JWTSecret:        "jwt_secret",
	})
	if err != nil {
		return fmt.Errorf("create supabase client: %w", err)
	}

	f.supabase = client
	return nil
}

// Stop shuts down the enclave and zeroizes sensitive material.
func (f *Foundation) Stop(ctx context.Context) error {
	return f.trustRoot.Stop(ctx)
}

// ServiceOS returns a ServiceOS bound to this TrustRoot.
// Uses LegacyManifest for per-service configuration.
func (f *Foundation) ServiceOS(manifest *os.LegacyManifest, logger os.Logger) (os.ServiceOS, error) {
	return os.NewServiceContext(manifest, f.trustRoot, logger)
}

// TrustRoot exposes the underlying enclave foundation (for diagnostics).
func (f *Foundation) TrustRoot() *tee.TrustRoot {
	return f.trustRoot
}

// Supabase returns the Supabase client (nil if not configured).
func (f *Foundation) Supabase() *supabase.Client {
	return f.supabase
}
