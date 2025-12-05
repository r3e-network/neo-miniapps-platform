// Package mixer provides privacy-preserving transaction mixing service.
package mixer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/R3E-Network/service_layer/platform/os"
	"github.com/R3E-Network/service_layer/services/base"
)

const (
	ServiceID   = "mixer"
	ServiceName = "Mixer Service"
	Version     = "1.0.0"
)

// Manifest returns the service manifest.
func Manifest() *os.LegacyManifest {
	return &os.LegacyManifest{
		ServiceID:   ServiceID,
		Version:     Version,
		Description: "Privacy-preserving transaction mixing service with TEE protection",
		RequiredCapabilities: []os.Capability{
			os.CapSecrets,
			os.CapKeys,
			os.CapStorage,
			os.CapNeo,      // Neo N3 wallet operations
			os.CapDatabase, // Supabase database access
		},
		OptionalCapabilities: []os.Capability{
			os.CapSecretsWrite,
			os.CapKeysSign,
			os.CapNeoSign,       // Neo transaction signing
			os.CapDatabaseWrite, // Database write access
			os.CapNetwork,
			os.CapAttestation,
		},
		ResourceLimits: os.ResourceLimits{
			MaxMemory:      256 * 1024 * 1024, // 256MB
			MaxCPUTime:     60 * time.Second,
			MaxNetworkReqs: 200,
			MaxSecrets:     100,
		},
	}
}

// Service implements the Mixer service.
type Service struct {
	*base.BaseService

	mu      sync.RWMutex
	enclave *Enclave
	store   *Store
}

// New creates a new Mixer service.
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

// onAfterStart is called after enclave and store are initialized.
// This handles service-specific initialization like config hydration, metrics, etc.
func (s *Service) onAfterStart(ctx context.Context) error {
	// Optional sealed config hydrate (kept enclave-side)
	var cfg map[string]any
	if loaded, err := s.enclave.LoadConfigJSON(ctx, "mixer/config", &cfg); err != nil {
		s.Logger().Warn("mixer storage hydrate failed", "err", err)
	} else if loaded {
		// TODO: apply cfg to enclave/runtime as needed
		s.Logger().Info("mixer config loaded from sealed storage")
	}

	// Register metrics if available
	s.RegisterMetrics("mixer", func(metrics os.MetricsAPI) {
		metrics.RegisterCounter("mixer_requests_total", "Total number of mix requests")
		metrics.RegisterCounter("mixer_requests_completed", "Total number of completed mix requests")
		metrics.RegisterCounter("mixer_requests_failed", "Total number of failed mix requests")
		metrics.RegisterGauge("mixer_active_pools", "Number of active pool accounts")
		metrics.RegisterHistogram("mixer_duration_seconds", "Mix operation duration", []float64{1, 5, 10, 30, 60, 120})
	})

	s.Logger().Info("mixer service started")
	return nil
}

// NOTE: Start(), Stop(), and Health() are now handled by BaseService
// with automatic component lifecycle management.

// =============================================================================
// Mixer Operations
// =============================================================================

// CreateMixRequest creates a new mix request.
func (s *Service) CreateMixRequest(ctx context.Context, req *MixRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State() != base.StateRunning {
		return fmt.Errorf("service not running")
	}

	req.Status = RequestStatusPending
	req.SetTimestamps()

	return s.store.CreateRequest(ctx, req)
}

// GetMixRequest retrieves a mix request by ID.
func (s *Service) GetMixRequest(ctx context.Context, id string) (*MixRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.store.GetRequest(ctx, id)
}

// ListMixRequests lists all mix requests.
func (s *Service) ListMixRequests(ctx context.Context) ([]*MixRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.store.ListRequests(ctx)
}

// ProcessMix processes a mix operation.
func (s *Service) ProcessMix(ctx context.Context, requestID string, inputData []byte, outputPath string) (*MixOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State() != base.StateRunning {
		return nil, fmt.Errorf("service not running")
	}

	// Get request
	req, err := s.store.GetRequest(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("get request: %w", err)
	}

	// Update status
	req.Status = RequestStatusMixing
	req.MixStartAt = time.Now()
	if err := s.store.UpdateRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("update request: %w", err)
	}

	// Process in enclave
	output, err := s.enclave.ProcessMix(ctx, requestID, inputData, outputPath)
	if err != nil {
		req.Status = RequestStatusFailed
		req.Error = err.Error()
		s.store.UpdateRequest(ctx, req)
		return nil, fmt.Errorf("process mix: %w", err)
	}

	// Update completion
	req.Status = RequestStatusCompleted
	req.CompletedAt = time.Now()
	if err := s.store.UpdateRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("update request: %w", err)
	}

	return output, nil
}

// DeriveKey derives a key for mixing operations.
func (s *Service) DeriveKey(ctx context.Context, path string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.State() != base.StateRunning {
		return nil, fmt.Errorf("service not running")
	}

	return s.enclave.DeriveKey(ctx, path)
}

// SignTransaction signs a transaction.
func (s *Service) SignTransaction(ctx context.Context, keyPath string, txHash []byte) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.State() != base.StateRunning {
		return nil, fmt.Errorf("service not running")
	}

	return s.enclave.SignTransaction(ctx, keyPath, txHash)
}

// GetAddress returns the blockchain address for a key.
func (s *Service) GetAddress(ctx context.Context, keyPath string, chain os.ChainType) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.State() != base.StateRunning {
		return "", fmt.Errorf("service not running")
	}

	return s.enclave.GetAddress(ctx, keyPath, chain)
}

// =============================================================================
// Pool Operations
// =============================================================================

// CreatePool creates a new pool account.
func (s *Service) CreatePool(ctx context.Context, pool *PoolAccount) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State() != base.StateRunning {
		return fmt.Errorf("service not running")
	}

	pool.Status = PoolAccountStatusActive
	pool.SetTimestamps()

	return s.store.CreatePool(ctx, pool)
}

// GetPool retrieves a pool account.
func (s *Service) GetPool(ctx context.Context, id string) (*PoolAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.store.GetPool(ctx, id)
}

// ListActivePools lists active pool accounts.
func (s *Service) ListActivePools(ctx context.Context) ([]*PoolAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.store.ListActivePools(ctx)
}

// GetStats returns service statistics.
func (s *Service) GetStats(ctx context.Context) (*MixStats, error) {
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
// Full Mixing Workflow (Per Design Spec)
// =============================================================================

// ClaimRequest claims a pending mix request and creates a multisig pool.
// This is called after user deposits funds to the contract.
// Flow: Contract holds funds -> TEE generates multisig -> Contract releases to multisig
func (s *Service) ClaimRequest(ctx context.Context, requestID string) (*ClaimInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State() != base.StateRunning {
		return nil, fmt.Errorf("service not running")
	}

	// Get the request
	req, err := s.store.GetRequest(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("get request: %w", err)
	}

	if req.Status != RequestStatusPending && req.Status != RequestStatusDeposited {
		return nil, fmt.Errorf("request not claimable: status=%s", req.Status)
	}

	// Create a new 1-of-2 multisig pool for this request
	pool, err := s.enclave.CreateMultisigPool(ctx)
	if err != nil {
		return nil, fmt.Errorf("create multisig pool: %w", err)
	}

	// Generate TEE-signed claim
	claimInfo, err := s.enclave.GenerateServiceClaim(ctx, requestID, pool)
	if err != nil {
		return nil, fmt.Errorf("generate claim: %w", err)
	}

	// Update request status
	req.Status = RequestStatusClaimed
	req.ClaimInfo = claimInfo
	req.ClaimedAt = time.Now()
	req.Deadline = req.CalculateDeadline()

	if err := s.store.UpdateRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("update request: %w", err)
	}

	// Store pool in store as well
	if err := s.store.CreatePool(ctx, pool); err != nil {
		s.Logger().Warn("failed to store pool in store", "error", err)
	}

	s.Logger().Info("request claimed",
		"request_id", requestID,
		"multisig", pool.ScriptHash,
		"deadline", req.Deadline,
	)

	return claimInfo, nil
}

// StartMixing starts the mixing process for a claimed request.
// This decrypts target addresses and begins internal transfers.
func (s *Service) StartMixing(ctx context.Context, requestID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State() != base.StateRunning {
		return fmt.Errorf("service not running")
	}

	req, err := s.store.GetRequest(ctx, requestID)
	if err != nil {
		return fmt.Errorf("get request: %w", err)
	}

	if req.Status != RequestStatusClaimed {
		return fmt.Errorf("request not ready for mixing: status=%s", req.Status)
	}

	// Decrypt target addresses (inside TEE)
	targets, err := s.enclave.DecryptTargetPayload(ctx, req.EncryptedPayload)
	if err != nil {
		return fmt.Errorf("decrypt targets: %w", err)
	}

	// Store decrypted targets in request (only in memory, not persisted)
	req.Targets = targets
	req.Status = RequestStatusMixing
	req.MixStartAt = time.Now()

	if err := s.store.UpdateRequest(ctx, req); err != nil {
		return fmt.Errorf("update request: %w", err)
	}

	s.Logger().Info("mixing started",
		"request_id", requestID,
		"target_count", len(targets),
		"mix_duration", req.MixDuration,
	)

	return nil
}

// Store exposes the Mixer store.
func (s *Service) Store() *Store {
	return s.store
}

// CompleteMixing completes the mixing process and submits completion proof.
func (s *Service) CompleteMixing(ctx context.Context, requestID string) (*CompletionProof, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State() != base.StateRunning {
		return nil, fmt.Errorf("service not running")
	}

	req, err := s.store.GetRequest(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("get request: %w", err)
	}

	if req.Status != RequestStatusMixing {
		return nil, fmt.Errorf("request not in mixing state: status=%s", req.Status)
	}

	if len(req.Targets) == 0 {
		return nil, fmt.Errorf("no targets found for request")
	}

	// Verify all targets have been delivered
	for i, target := range req.Targets {
		if !target.Delivered {
			return nil, fmt.Errorf("target %d not yet delivered", i)
		}
	}

	// Generate completion proof (Merkle root + TEE signature)
	proof, err := s.enclave.GenerateCompletionProof(ctx, requestID, req.Targets)
	if err != nil {
		return nil, fmt.Errorf("generate completion proof: %w", err)
	}

	// Update request
	req.Status = RequestStatusCompleted
	req.CompletionProof = proof
	req.CompletedAt = time.Now()

	if err := s.store.UpdateRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("update request: %w", err)
	}

	s.Logger().Info("mixing completed",
		"request_id", requestID,
		"merkle_root", proof.MerkleRoot,
		"output_count", proof.OutputCount,
	)

	return proof, nil
}

// =============================================================================
// Refund Handling
// =============================================================================

// CheckRefundableRequests checks for requests that have exceeded their deadline.
func (s *Service) CheckRefundableRequests(ctx context.Context) ([]*MixRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	requests, err := s.store.ListRequests(ctx)
	if err != nil {
		return nil, fmt.Errorf("list requests: %w", err)
	}

	var refundable []*MixRequest
	now := time.Now()

	for _, req := range requests {
		if req.IsRefundable() && req.Status != RequestStatusRefundable {
			refundable = append(refundable, req)
		}
		// Also check for requests approaching deadline
		if req.Status == RequestStatusMixing {
			timeLeft := req.Deadline.Sub(now)
			if timeLeft < 24*time.Hour {
				s.Logger().Warn("request approaching deadline",
					"request_id", req.GetID(),
					"time_left", timeLeft,
				)
			}
		}
	}

	return refundable, nil
}

// MarkRefundable marks a request as refundable (deadline exceeded).
func (s *Service) MarkRefundable(ctx context.Context, requestID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	req, err := s.store.GetRequest(ctx, requestID)
	if err != nil {
		return fmt.Errorf("get request: %w", err)
	}

	if !req.IsRefundable() {
		return fmt.Errorf("request not refundable")
	}

	req.Status = RequestStatusRefundable

	if err := s.store.UpdateRequest(ctx, req); err != nil {
		return fmt.Errorf("update request: %w", err)
	}

	s.Logger().Warn("request marked refundable",
		"request_id", requestID,
		"deadline", req.Deadline,
	)

	return nil
}

// ProcessRefund processes a refund for a refundable request.
// This is called when user claims refund from contract bond.
func (s *Service) ProcessRefund(ctx context.Context, requestID string, refundTxHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	req, err := s.store.GetRequest(ctx, requestID)
	if err != nil {
		return fmt.Errorf("get request: %w", err)
	}

	if req.Status != RequestStatusRefundable {
		return fmt.Errorf("request not in refundable state: status=%s", req.Status)
	}

	req.Status = RequestStatusRefunded
	req.RefundInfo = &RefundInfo{
		RefundAmount: req.Amount,
		RefundTxHash: refundTxHash,
		RefundedAt:   time.Now(),
		Reason:       "deadline_exceeded",
	}
	req.RefundedAt = time.Now()

	if err := s.store.UpdateRequest(ctx, req); err != nil {
		return fmt.Errorf("update request: %w", err)
	}

	s.Logger().Info("refund processed",
		"request_id", requestID,
		"amount", req.Amount,
		"tx_hash", refundTxHash,
	)

	return nil
}

// =============================================================================
// Service Registration & Bond Management
// =============================================================================

// RegisterService registers the mixer service on-chain.
func (s *Service) RegisterService(ctx context.Context, bondAmount string) (*ServiceRegistration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State() != base.StateRunning {
		return nil, fmt.Errorf("service not running")
	}

	// Get TEE public key for contract registration
	teePubKey, err := s.enclave.GetTEEPublicKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("get tee pubkey: %w", err)
	}

	reg := &ServiceRegistration{
		ServiceID:             ServiceID,
		TEEPublicKey:          teePubKey,
		BondAmount:            bondAmount,
		MinBondRequired:       MinBondAmount,
		MaxOutstandingAllowed: "100000", // 100k GAS max outstanding
		CurrentOutstanding:    "0",
		Status:                ServiceStatusActive,
		RegisteredAt:          time.Now(),
	}
	reg.GenerateID()
	reg.SetTimestamps()

	// Store in enclave
	s.enclave.SetServiceRegistration(reg)

	s.Logger().Info("service registered",
		"service_id", reg.ServiceID,
		"tee_pubkey", teePubKey[:16]+"...",
		"bond", bondAmount,
	)

	return reg, nil
}

// LoadMasterPubKeys loads Master public keys into the TEE.
func (s *Service) LoadMasterPubKeys(ctx context.Context, pubKeys []MasterPubKeyEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State() != base.StateRunning {
		return fmt.Errorf("service not running")
	}

	return s.enclave.LoadMasterPubKeys(ctx, pubKeys)
}

// GetMasterPubKeyStatus returns the status of the Master pubkey pool.
func (s *Service) GetMasterPubKeyStatus() (available int, total int) {
	available = s.enclave.GetAvailableMasterPubKeyCount()
	// Total would need to be tracked separately
	return available, available
}

// =============================================================================
// Decoy Transaction Management
// =============================================================================

// GenerateDecoyTransaction generates a decoy transaction for privacy.
func (s *Service) GenerateDecoyTransaction(ctx context.Context) (*MixTransaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State() != base.StateRunning {
		return nil, fmt.Errorf("service not running")
	}

	tx, err := s.enclave.GenerateDecoyTransaction(ctx)
	if err != nil {
		return nil, err
	}

	if tx != nil {
		if err := s.store.CreateTransaction(ctx, tx); err != nil {
			s.Logger().Warn("failed to store decoy tx", "error", err)
		}
	}

	return tx, nil
}

// =============================================================================
// Pool Management
// =============================================================================

// CreateMultisigPool creates a new 1-of-2 multisig pool.
func (s *Service) CreateMultisigPool(ctx context.Context) (*PoolAccount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State() != base.StateRunning {
		return nil, fmt.Errorf("service not running")
	}

	pool, err := s.enclave.CreateMultisigPool(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.store.CreatePool(ctx, pool); err != nil {
		return nil, fmt.Errorf("store pool: %w", err)
	}

	return pool, nil
}

// RotatePools rotates old/high-usage pools.
func (s *Service) RotatePools(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State() != base.StateRunning {
		return fmt.Errorf("service not running")
	}

	poolsToRotate := s.enclave.GetPoolsForRotation()

	for _, pool := range poolsToRotate {
		if err := s.enclave.RetirePool(ctx, pool.GetID()); err != nil {
			s.Logger().Warn("failed to retire pool", "pool_id", pool.GetID(), "error", err)
			continue
		}

		// Create replacement pool
		newPool, err := s.enclave.CreateMultisigPool(ctx)
		if err != nil {
			s.Logger().Warn("failed to create replacement pool", "error", err)
			continue
		}

		if err := s.store.CreatePool(ctx, newPool); err != nil {
			s.Logger().Warn("failed to store new pool", "error", err)
		}

		s.Logger().Info("pool rotated",
			"old_pool", pool.GetID(),
			"new_pool", newPool.GetID(),
		)
	}

	return nil
}

// =============================================================================
// Contract Event Handling
// =============================================================================

// HandleContractEvent handles events from the MixContract.
func (s *Service) HandleContractEvent(ctx context.Context, event *ContractEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Logger().Info("contract event received",
		"type", event.EventType,
		"request_id", event.RequestID,
		"tx_hash", event.TxHash,
	)

	switch event.EventType {
	case EventRequestCreated:
		return s.handleRequestCreated(ctx, event)
	case EventRequestClaimed:
		return s.handleRequestClaimed(ctx, event)
	case EventRequestCompleted:
		return s.handleRequestCompleted(ctx, event)
	case EventRefundClaimed:
		return s.handleRefundClaimed(ctx, event)
	case EventBondSlashed:
		return s.handleBondSlashed(ctx, event)
	default:
		s.Logger().Warn("unknown contract event", "type", event.EventType)
	}

	return nil
}

func (s *Service) handleRequestCreated(ctx context.Context, event *ContractEvent) error {
	// A new request was created on-chain
	// We should claim it if we have capacity
	s.Logger().Info("new request created on-chain", "request_id", event.RequestID)
	return nil
}

func (s *Service) handleRequestClaimed(ctx context.Context, event *ContractEvent) error {
	// Our claim was confirmed on-chain
	s.Logger().Info("claim confirmed on-chain", "request_id", event.RequestID)
	return nil
}

func (s *Service) handleRequestCompleted(ctx context.Context, event *ContractEvent) error {
	// Our completion proof was accepted on-chain
	s.Logger().Info("completion confirmed on-chain", "request_id", event.RequestID)
	return nil
}

func (s *Service) handleRefundClaimed(ctx context.Context, event *ContractEvent) error {
	// User claimed refund from our bond
	s.Logger().Warn("refund claimed from bond", "request_id", event.RequestID)
	return s.ProcessRefund(ctx, event.RequestID, event.TxHash)
}

func (s *Service) handleBondSlashed(ctx context.Context, event *ContractEvent) error {
	// Our bond was slashed
	s.Logger().Error("bond slashed!", "request_id", event.RequestID, "data", event.Data)
	return nil
}
