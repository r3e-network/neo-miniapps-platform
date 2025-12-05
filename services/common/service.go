// Package common provides the base service framework for Neo Service Layer.
// All services (Oracle, VRF, Secrets, etc.) extend this base to run as
// MarbleRun Marbles with EGo SGX runtime and Supabase backend.
package common

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/R3E-Network/service_layer/ego"
	"github.com/R3E-Network/service_layer/marble/sdk"
	"github.com/R3E-Network/service_layer/supabase/client"
)

// Service is the base interface for all services.
type Service interface {
	// Name returns the service name
	Name() string

	// Initialize initializes the service
	Initialize(ctx context.Context) error

	// RegisterRoutes registers HTTP routes
	RegisterRoutes(mux *http.ServeMux)

	// Start starts the service
	Start(ctx context.Context) error

	// Stop stops the service
	Stop(ctx context.Context) error

	// Health returns health status
	Health() HealthStatus
}

// HealthStatus represents service health.
type HealthStatus struct {
	Status    string            `json:"status"` // healthy, degraded, unhealthy
	Service   string            `json:"service"`
	Version   string            `json:"version"`
	Uptime    time.Duration     `json:"uptime"`
	TEE       TEEStatus         `json:"tee"`
	Checks    map[string]string `json:"checks,omitempty"`
}

// TEEStatus represents TEE attestation status.
type TEEStatus struct {
	Enabled   bool   `json:"enabled"`
	Mode      string `json:"mode"` // sgx, simulation
	Attested  bool   `json:"attested"`
	UniqueID  string `json:"unique_id,omitempty"`
}

// BaseService provides common functionality for all services.
type BaseService struct {
	name      string
	version   string
	startTime time.Time

	// MarbleRun integration
	marble *sdk.Marble

	// EGo runtime
	egoRuntime *ego.Runtime

	// Supabase client
	supabase *client.Client

	// Configuration
	config *ServiceConfig
}

// ServiceConfig holds service configuration.
type ServiceConfig struct {
	// Service identity
	Name    string
	Version string

	// MarbleRun
	MarbleType      string
	CoordinatorAddr string

	// Supabase
	SupabaseURL string
	SupabaseKey string

	// Neo N3
	NeoRPCURL     string
	NeoNetworkMagic uint32

	// Runtime
	SimulationMode bool
	Debug          bool
}

// NewBaseService creates a new base service.
func NewBaseService(cfg *ServiceConfig) (*BaseService, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("service name is required")
	}

	bs := &BaseService{
		name:      cfg.Name,
		version:   cfg.Version,
		config:    cfg,
		startTime: time.Now(),
	}

	// Initialize EGo runtime
	bs.egoRuntime = ego.New()

	return bs, nil
}

// Initialize initializes the base service components.
func (bs *BaseService) Initialize(ctx context.Context) error {
	log.Printf("[%s] Initializing service...", bs.name)

	// Create Marble
	marble, err := sdk.New(sdk.Config{
		MarbleType:      bs.config.MarbleType,
		CoordinatorAddr: bs.config.CoordinatorAddr,
		SimulationMode:  bs.config.SimulationMode,
	})
	if err != nil {
		return fmt.Errorf("create marble: %w", err)
	}
	bs.marble = marble

	// Activate with Coordinator
	log.Printf("[%s] Activating with Coordinator...", bs.name)
	if err := marble.Activate(ctx); err != nil {
		return fmt.Errorf("activate marble: %w", err)
	}
	log.Printf("[%s] Marble activated successfully", bs.name)

	// Initialize Supabase client
	supabaseKey := bs.config.SupabaseKey
	if supabaseKey == "" {
		// Try to get from Marble secrets
		if keyBytes, err := marble.GetSecret("supabase_service_key"); err == nil {
			supabaseKey = string(keyBytes)
		}
	}

	if bs.config.SupabaseURL != "" && supabaseKey != "" {
		supabase, err := client.New(client.Config{
			URL:    bs.config.SupabaseURL,
			APIKey: supabaseKey,
		})
		if err != nil {
			log.Printf("[%s] Warning: Failed to create Supabase client: %v", bs.name, err)
		} else {
			bs.supabase = supabase
			log.Printf("[%s] Supabase client initialized", bs.name)
		}
	}

	return nil
}

// Name returns the service name.
func (bs *BaseService) Name() string {
	return bs.name
}

// Version returns the service version.
func (bs *BaseService) Version() string {
	return bs.version
}

// Marble returns the Marble instance.
func (bs *BaseService) Marble() *sdk.Marble {
	return bs.marble
}

// Supabase returns the Supabase client.
func (bs *BaseService) Supabase() *client.Client {
	return bs.supabase
}

// EgoRuntime returns the EGo runtime.
func (bs *BaseService) EgoRuntime() *ego.Runtime {
	return bs.egoRuntime
}

// Config returns the service configuration.
func (bs *BaseService) Config() *ServiceConfig {
	return bs.config
}

// Health returns the base health status.
func (bs *BaseService) Health() HealthStatus {
	status := "healthy"
	checks := make(map[string]string)

	// Check Marble activation
	if bs.marble != nil && bs.marble.IsActivated() {
		checks["marble"] = "activated"
	} else {
		checks["marble"] = "not_activated"
		status = "degraded"
	}

	// Check Supabase
	if bs.supabase != nil {
		checks["supabase"] = "connected"
	} else {
		checks["supabase"] = "not_connected"
	}

	// TEE status
	teeStatus := TEEStatus{
		Enabled:  bs.egoRuntime.InEnclave(),
		Mode:     "simulation",
		Attested: false,
	}
	if bs.egoRuntime.InEnclave() {
		teeStatus.Mode = "sgx"
		teeStatus.Attested = true
		teeStatus.UniqueID = bs.egoRuntime.UniqueID()
	}

	return HealthStatus{
		Status:  status,
		Service: bs.name,
		Version: bs.version,
		Uptime:  time.Since(bs.startTime),
		TEE:     teeStatus,
		Checks:  checks,
	}
}

// RegisterBaseRoutes registers common routes.
func (bs *BaseService) RegisterBaseRoutes(mux *http.ServeMux) {
	// Health check
	mux.HandleFunc("/health", bs.handleHealth)

	// Service info
	mux.HandleFunc("/info", bs.handleInfo)

	// TEE attestation
	mux.HandleFunc("/attestation", bs.handleAttestation)
}

func (bs *BaseService) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := bs.Health()
	w.Header().Set("Content-Type", "application/json")

	if health.Status != "healthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(health)
}

func (bs *BaseService) handleInfo(w http.ResponseWriter, r *http.Request) {
	info := map[string]any{
		"service": bs.name,
		"version": bs.version,
		"uptime":  time.Since(bs.startTime).String(),
		"tee": map[string]any{
			"enabled": bs.egoRuntime.InEnclave(),
			"mode":    "simulation",
		},
	}

	if bs.egoRuntime.InEnclave() {
		info["tee"].(map[string]any)["mode"] = "sgx"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func (bs *BaseService) handleAttestation(w http.ResponseWriter, r *http.Request) {
	// Generate attestation quote
	nonce := []byte(r.URL.Query().Get("nonce"))
	if len(nonce) == 0 {
		nonce = []byte(fmt.Sprintf("%d", time.Now().UnixNano()))
	}

	quote, err := bs.egoRuntime.GenerateQuote(nonce)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to generate quote: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"quote":     quote,
		"nonce":     string(nonce),
		"unique_id": bs.egoRuntime.UniqueID(),
		"signer_id": bs.egoRuntime.SignerID(),
	})
}

// =============================================================================
// Request/Response Helpers
// =============================================================================

// APIResponse is a standard API response.
type APIResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

// WriteJSON writes a JSON response.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// WriteSuccess writes a success response.
func WriteSuccess(w http.ResponseWriter, data any) {
	WriteJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    data,
	})
}

// WriteError writes an error response.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, APIResponse{
		Success: false,
		Error:   message,
	})
}

// ReadJSON reads JSON from request body.
func ReadJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return fmt.Errorf("empty request body")
	}
	return json.NewDecoder(r.Body).Decode(v)
}
