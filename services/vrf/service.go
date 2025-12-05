// Package vrf provides Verifiable Random Function service.
package vrf

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

const (
	ServiceID   = "vrf"
	ServiceName = "VRF Service"
	Version     = "1.0.0"
)

// Manifest returns the service manifest.
func Manifest() *os.LegacyManifest {
	return &os.LegacyManifest{
		ServiceID:   ServiceID,
		Version:     Version,
		Description: "Verifiable Random Function service with TEE protection",
		RequiredCapabilities: []os.Capability{
			os.CapKeys,
			os.CapKeysSign,
			os.CapStorage,
		},
		OptionalCapabilities: []os.Capability{
			os.CapNetwork,
			os.CapAttestation,
			os.CapDatabase,      // Database storage
			os.CapDatabaseWrite, // Database write
			os.CapMetrics,       // Metrics collection
			os.CapCache,         // Result caching
			os.CapNeo,           // Neo N3 blockchain
			os.CapNeoSign,       // Neo transaction signing
			os.CapContract,      // Contract interaction
			os.CapContractWrite, // Contract callbacks
		},
		ResourceLimits: os.ResourceLimits{
			MaxMemory:  64 * 1024 * 1024,
			MaxCPUTime: 10 * time.Second,
		},
	}
}

// Service implements the VRF service.
type Service struct {
	*base.BaseService

	mu      sync.RWMutex
	enclave *Enclave
	store   *Store
}

// New creates a new VRF service.
func New(serviceOS os.ServiceOS) (*Service, error) {
	baseService := base.NewBaseService(ServiceID, ServiceName, Version, serviceOS)

	enclave, err := NewEnclave(serviceOS)
	if err != nil {
		return nil, fmt.Errorf("create enclave: %w", err)
	}

	store := NewStore()

	svc := &Service{
		BaseService: baseService,
		enclave:     enclave,
		store:       store,
	}

	// Register components with BaseService for automatic lifecycle management
	baseService.SetEnclave(enclave)
	baseService.SetStore(store)

	// Set lifecycle hooks for service-specific initialization
	baseService.SetHooks(base.LifecycleHooks{
		OnAfterStart: svc.onAfterStart,
	})

	return svc, nil
}

// Store exposes the VRF store.
func (s *Service) Store() *Store {
	return s.store
}

// Store exposes the VRF store (used by API handlers).
func (s *Service) Store() *Store {
	return s.store
}

// onAfterStart is called after enclave and store are initialized.
// This handles service-specific initialization like metrics, config hydration, etc.
func (s *Service) onAfterStart(ctx context.Context) error {
	// Optional sealed config hydrate inside enclave
	var cfg map[string]any
	if loaded, err := s.enclave.LoadConfigJSON(ctx, "vrf/config", &cfg); err != nil {
		s.Logger().Warn("vrf storage hydrate failed", "err", err)
	} else if loaded {
		// TODO: apply cfg to enclave/runtime as needed
	}

	// Register metrics using BaseService helper
	s.RegisterMetrics("vrf", func(metrics os.MetricsAPI) {
		metrics.RegisterCounter("vrf_requests_total", "Total number of VRF requests")
		metrics.RegisterCounter("vrf_requests_fulfilled", "Total number of fulfilled VRF requests")
		metrics.RegisterCounter("vrf_requests_failed", "Total number of failed VRF requests")
		metrics.RegisterHistogram("vrf_generation_duration_seconds", "VRF generation duration", []float64{0.01, 0.05, 0.1, 0.5, 1})
	})

	// Subscribe to contract requests using BaseService helper
	s.StartContractListener(ctx, s.handleContractRequest)

	return nil
}

// NOTE: Start(), Stop(), and Health() are now handled by BaseService
// with automatic component lifecycle management.

// =============================================================================
// VRF Operations
// =============================================================================

// GenerateRandomness generates verifiable randomness.
func (s *Service) GenerateRandomness(ctx context.Context, req *GenerateRandomnessRequest) (*VRFOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State() != base.StateRunning {
		return nil, fmt.Errorf("service not running")
	}

	// Generate randomness in enclave
	output, err := s.enclave.GenerateRandomness(ctx, req.Seed, req.BlockHash)
	if err != nil {
		return nil, fmt.Errorf("generate randomness: %w", err)
	}

	// Store request record
	vrfReq := &VRFRequest{
		AccountID:       req.AccountID,
		Status:          RequestStatusFulfilled,
		Seed:            req.Seed,
		BlockHash:       req.BlockHash,
		BlockNumber:     req.BlockNumber,
		Randomness:      output.Randomness,
		Proof:           output.Proof,
		CallbackAddress: req.CallbackAddress,
		FulfilledAt:     time.Now(),
	}
	vrfReq.ID = fmt.Sprintf("vrf-%d", time.Now().UnixNano())
	vrfReq.SetTimestamps()

	if err := s.store.CreateRequest(ctx, vrfReq); err != nil {
		s.Logger().Warn("failed to store VRF request", "error", err)
	}

	return output, nil
}

// VerifyRandomness verifies a VRF output.
func (s *Service) VerifyRandomness(ctx context.Context, output *VRFOutput) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.State() != base.StateRunning {
		return false, fmt.Errorf("service not running")
	}

	return s.enclave.VerifyRandomness(ctx, output)
}

// GetPublicKey returns the VRF public key.
func (s *Service) GetPublicKey(ctx context.Context) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.State() != base.StateRunning {
		return nil, fmt.Errorf("service not running")
	}

	return s.enclave.GetPublicKey(ctx)
}

// GetRequest retrieves a VRF request by ID.
func (s *Service) GetRequest(ctx context.Context, id string) (*VRFRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.store.GetRequest(ctx, id)
}

// GetStats returns service statistics.
func (s *Service) GetStats(ctx context.Context) (*VRFStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats, err := s.store.GetStats(ctx)
	if err != nil {
		return nil, err
	}
	stats.GeneratedAt = time.Now()
	return stats, nil
}

// =============================================================================
// Contract Integration
// =============================================================================

// handleContractRequest handles incoming VRF requests from contracts.
func (s *Service) handleContractRequest(ctx context.Context, req *os.ServiceRequest) error {
	s.Logger().Info("received contract VRF request",
		"request_id", req.RequestID,
		"requester", req.Requester,
	)

	// Parse the VRF request payload
	var vrfReq VRFContractRequest
	if err := json.Unmarshal(req.Payload, &vrfReq); err != nil {
		return s.SendContractError(ctx, req.RequestID, "invalid payload: "+err.Error())
	}

	// Generate randomness
	genReq := &GenerateRandomnessRequest{
		Seed:            vrfReq.Seed,
		BlockHash:       vrfReq.BlockHash,
		BlockNumber:     int64(vrfReq.BlockNumber),
		CallbackAddress: req.CallbackContract,
	}

	output, err := s.GenerateRandomness(ctx, genReq)
	if err != nil {
		return s.SendContractError(ctx, req.RequestID, "randomness generation failed: "+err.Error())
	}

	// Get public key for response
	pubKey, _ := s.GetPublicKey(ctx)

	// Prepare result
	result, err := json.Marshal(VRFContractResponse{
		Randomness: output.Randomness,
		Proof:      output.Proof,
		PublicKey:  pubKey,
	})
	if err != nil {
		return s.SendContractError(ctx, req.RequestID, "result serialization failed: "+err.Error())
	}

	// Send callback with result using BaseService helper
	return s.SendContractCallback(ctx, req.RequestID, result, output.Proof)
}

// VRFContractRequest represents a VRF request from a contract.
type VRFContractRequest struct {
	Seed        []byte `json:"seed"`
	BlockHash   []byte `json:"block_hash,omitempty"`
	BlockNumber uint64 `json:"block_number,omitempty"`
	NumWords    int    `json:"num_words,omitempty"`
}

// VRFContractResponse represents a VRF response to a contract.
type VRFContractResponse struct {
	Randomness []byte `json:"randomness"`
	Proof      []byte `json:"proof"`
	PublicKey  []byte `json:"public_key"`
}

// NOTE: sendContractCallback and sendContractError are now provided by BaseService
// as SendContractCallback and SendContractError methods.
