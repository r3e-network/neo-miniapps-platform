// Package common provides the MarbleRun adapter for existing services.
// This adapter bridges the existing service implementations with the new
// MarbleRun + EGo + Supabase architecture.
package common

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/R3E-Network/service_layer/ego"
	"github.com/R3E-Network/service_layer/marble/sdk"
	"github.com/R3E-Network/service_layer/supabase/client"
)

// MarbleAdapter adapts existing services to run as MarbleRun Marbles.
type MarbleAdapter struct {
	// Identity
	serviceType string
	version     string

	// MarbleRun
	marble *sdk.Marble

	// EGo Runtime
	egoRuntime *ego.Runtime

	// Supabase
	supabase *client.Client

	// Neo N3
	neo *NeoClient

	// State
	startTime time.Time
	running   bool
}

// AdapterConfig holds adapter configuration.
type AdapterConfig struct {
	ServiceType     string
	Version         string
	CoordinatorAddr string
	SupabaseURL     string
	SupabaseKey     string
	NeoRPCURL       string
	SimulationMode  bool
}

// NewMarbleAdapter creates a new MarbleRun adapter.
func NewMarbleAdapter(cfg AdapterConfig) (*MarbleAdapter, error) {
	if cfg.ServiceType == "" {
		return nil, fmt.Errorf("service type is required")
	}

	// Get config from environment if not provided
	if cfg.CoordinatorAddr == "" {
		cfg.CoordinatorAddr = os.Getenv("COORDINATOR_ADDR")
		if cfg.CoordinatorAddr == "" {
			cfg.CoordinatorAddr = "localhost:4433"
		}
	}
	if cfg.SupabaseURL == "" {
		cfg.SupabaseURL = os.Getenv("SUPABASE_URL")
	}
	if cfg.SupabaseKey == "" {
		cfg.SupabaseKey = os.Getenv("SUPABASE_SERVICE_KEY")
		if cfg.SupabaseKey == "" {
			cfg.SupabaseKey = os.Getenv("SUPABASE_SECRET_KEY")
		}
	}
	if cfg.NeoRPCURL == "" {
		cfg.NeoRPCURL = os.Getenv("NEO_RPC_URL")
		if cfg.NeoRPCURL == "" {
			cfg.NeoRPCURL = os.Getenv("VITE_NEO_RPC_URL")
		}
	}
	if os.Getenv("SIMULATION_MODE") == "true" {
		cfg.SimulationMode = true
	}

	adapter := &MarbleAdapter{
		serviceType: cfg.ServiceType,
		version:     cfg.Version,
		startTime:   time.Now(),
	}

	// Initialize EGo runtime
	adapter.egoRuntime = ego.New()

	return adapter, nil
}

// Initialize initializes the adapter and activates with Coordinator.
func (a *MarbleAdapter) Initialize(ctx context.Context) error {
	log.Printf("[%s] Initializing MarbleRun adapter...", a.serviceType)

	// Create and activate Marble
	marble, err := sdk.New(sdk.Config{
		MarbleType:     a.serviceType,
		CoordinatorAddr: os.Getenv("COORDINATOR_ADDR"),
		SimulationMode: os.Getenv("SIMULATION_MODE") == "true",
	})
	if err != nil {
		return fmt.Errorf("create marble: %w", err)
	}
	a.marble = marble

	// Activate with Coordinator
	log.Printf("[%s] Activating with Coordinator...", a.serviceType)
	if err := marble.Activate(ctx); err != nil {
		log.Printf("[%s] Warning: Marble activation failed: %v (continuing in standalone mode)", a.serviceType, err)
	} else {
		log.Printf("[%s] Marble activated successfully", a.serviceType)
	}

	// Initialize Supabase client
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_SERVICE_KEY")
	if supabaseKey == "" {
		supabaseKey = os.Getenv("SUPABASE_SECRET_KEY")
	}

	// Try to get Supabase key from Marble secrets
	if supabaseKey == "" && marble.IsActivated() {
		if keyBytes, err := marble.GetSecret("supabase_service_key"); err == nil {
			supabaseKey = string(keyBytes)
		}
	}

	if supabaseURL != "" && supabaseKey != "" {
		supabase, err := client.New(client.Config{
			URL:    supabaseURL,
			APIKey: supabaseKey,
		})
		if err != nil {
			log.Printf("[%s] Warning: Failed to create Supabase client: %v", a.serviceType, err)
		} else {
			a.supabase = supabase
			log.Printf("[%s] Supabase client initialized: %s", a.serviceType, supabaseURL)
		}
	}

	// Initialize Neo N3 client
	neoRPCURL := os.Getenv("NEO_RPC_URL")
	if neoRPCURL == "" {
		neoRPCURL = os.Getenv("VITE_NEO_RPC_URL")
	}
	if neoRPCURL != "" {
		a.neo = NewNeoClient(neoRPCURL)
		log.Printf("[%s] Neo N3 client initialized: %s", a.serviceType, neoRPCURL)
	}

	a.running = true
	return nil
}

// Marble returns the Marble instance.
func (a *MarbleAdapter) Marble() *sdk.Marble {
	return a.marble
}

// Supabase returns the Supabase client.
func (a *MarbleAdapter) Supabase() *client.Client {
	return a.supabase
}

// Neo returns the Neo N3 client.
func (a *MarbleAdapter) Neo() *NeoClient {
	return a.neo
}

// EgoRuntime returns the EGo runtime.
func (a *MarbleAdapter) EgoRuntime() *ego.Runtime {
	return a.egoRuntime
}

// ServiceType returns the service type.
func (a *MarbleAdapter) ServiceType() string {
	return a.serviceType
}

// IsRunning returns true if the adapter is running.
func (a *MarbleAdapter) IsRunning() bool {
	return a.running
}

// Stop stops the adapter.
func (a *MarbleAdapter) Stop() {
	a.running = false
}

// Health returns health status.
func (a *MarbleAdapter) Health() map[string]any {
	status := "healthy"
	checks := make(map[string]string)

	// Check Marble
	if a.marble != nil && a.marble.IsActivated() {
		checks["marble"] = "activated"
	} else {
		checks["marble"] = "standalone"
	}

	// Check Supabase
	if a.supabase != nil {
		checks["supabase"] = "connected"
	} else {
		checks["supabase"] = "not_configured"
	}

	// Check Neo
	if a.neo != nil {
		checks["neo"] = "connected"
	} else {
		checks["neo"] = "not_configured"
	}

	// TEE status
	teeMode := "simulation"
	if a.egoRuntime.InEnclave() {
		teeMode = "sgx"
	}

	return map[string]any{
		"status":   status,
		"service":  a.serviceType,
		"version":  a.version,
		"uptime":   time.Since(a.startTime).String(),
		"tee_mode": teeMode,
		"checks":   checks,
	}
}

// RegisterHealthHandler registers the health endpoint.
func (a *MarbleAdapter) RegisterHealthHandler(mux *http.ServeMux) {
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		WriteSuccess(w, a.Health())
	})
}

// =============================================================================
// Database Helpers
// =============================================================================

// SaveToSupabase saves data to a Supabase table.
func (a *MarbleAdapter) SaveToSupabase(ctx context.Context, table string, data any) error {
	if a.supabase == nil {
		return fmt.Errorf("supabase not configured")
	}

	resp, err := a.supabase.From(table).ExecuteInsert(ctx, data)
	if err != nil {
		return fmt.Errorf("insert: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	return nil
}

// QuerySupabase queries data from a Supabase table.
func (a *MarbleAdapter) QuerySupabase(ctx context.Context, table string, result any) error {
	if a.supabase == nil {
		return fmt.Errorf("supabase not configured")
	}

	resp, err := a.supabase.From(table).Select("*").Execute(ctx)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}

	if err := resp.Error(); err != nil {
		return err
	}

	return resp.JSON(result)
}

// =============================================================================
// TEE Helpers
// =============================================================================

// GenerateAttestation generates a TEE attestation quote.
func (a *MarbleAdapter) GenerateAttestation(data []byte) ([]byte, error) {
	return a.egoRuntime.GenerateQuote(data)
}

// SealData seals data using SGX sealing.
func (a *MarbleAdapter) SealData(data []byte) ([]byte, error) {
	return a.egoRuntime.Seal(data, ego.SealPolicyProduct)
}

// UnsealData unseals SGX-sealed data.
func (a *MarbleAdapter) UnsealData(sealedData []byte) ([]byte, error) {
	return a.egoRuntime.Unseal(sealedData)
}
