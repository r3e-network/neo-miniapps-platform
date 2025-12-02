package mixer

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/R3E-Network/service_layer/pkg/logger"
	"github.com/R3E-Network/service_layer/system/sandbox"
	
	"github.com/R3E-Network/service_layer/system/framework"
	core "github.com/R3E-Network/service_layer/system/framework/core"
)

// Service provides privacy-preserving transaction mixing using Double-Blind HD 1/2 Multi-sig.
//
// Architecture:
// - TEE Manager: Handles online HD key derivation and signing (daily operations)
// - Master Key Provider: Provides offline Master public keys for multi-sig addresses
// - Each pool account: Neo N3 1-of-2 multi-sig (TEE OR Master can sign)
//
// Security Properties:
// - TEE signs daily transactions (online, automated)
// - Master key provides recovery capability (offline, cold storage)
// - No single point of failure
// - Each pool address is independent (no on-chain linkability)
type Service struct {
	*framework.SandboxedServiceEngine // Provides: Name, Domain, Manifest, Descriptor, ValidateAccount, Logger, sandbox capabilities
	store                             Store
	tee                               TEEManager
	master                            MasterKeyProvider
	chain                             ChainClient
}

// New constructs a mixer service with Double-Blind HD 1/2 Multi-sig architecture.
func New(accounts AccountChecker, store Store, tee TEEManager, master MasterKeyProvider, chain ChainClient, log *logger.Logger) *Service {
	svc := &Service{
		SandboxedServiceEngine: framework.NewSandboxedServiceEngine(framework.SandboxedServiceConfig{
			ServiceConfig: framework.ServiceConfig{
				Name:         "mixer",
				Domain:       "mixer",
				Description:  "Privacy-preserving transaction mixing service with HD 1/2 multi-sig",
				DependsOn:    []string{"store", "svc-accounts", "svc-confidential"},
				RequiresAPIs: []framework.APISurface{framework.APISurfaceStore},
				Capabilities: []string{"mixer.request", "mixer.withdraw"},
				Quotas:       map[string]string{"mixer": "request-limits"},
				Accounts:     accounts,
				Logger:       log,
			},
			SecurityLevel: sandbox.SecurityLevelPrivileged,
			RequestedCapabilities: []sandbox.Capability{
				sandbox.CapStorageRead,
				sandbox.CapStorageWrite,
				sandbox.CapDatabaseRead,
				sandbox.CapDatabaseWrite,
				sandbox.CapBusPublish,
				sandbox.CapServiceCall,
				sandbox.CapNetworkOutbound, // For chain interactions
			},
			StorageQuota: 20 * 1024 * 1024, // 20MB
		}),
		store:  store,
		tee:    tee,
		master: master,
		chain:  chain,
	}
	return svc
}

// Errors
var (
	ErrInvalidAmount          = errors.New("invalid amount")
	ErrInvalidTargets         = errors.New("invalid targets: at least one target required")
	ErrTargetAmountMismatch   = errors.New("target amounts do not match total amount")
	ErrInvalidSplitCount      = errors.New("split count must be between 1 and 5")
	ErrInsufficientCapacity   = errors.New("insufficient service capacity")
	ErrRequestNotFound        = errors.New("mix request not found")
	ErrRequestNotWithdrawable = errors.New("request is not withdrawable")
	ErrClaimAlreadyExists     = errors.New("withdrawal claim already exists")
	ErrInvalidProof           = errors.New("invalid proof")
)

// CreateMixRequest creates a new privacy mixing request.
func (s *Service) CreateMixRequest(ctx context.Context, req MixRequest) (_ MixRequest, err error) {
	attrs := map[string]string{"account_id": req.AccountID, "resource": "mix_request"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer func() { finish(err) }()

	// Validate account
	if err := s.ValidateAccountExists(ctx, req.AccountID); err != nil {
		return MixRequest{}, fmt.Errorf("account validation: %w", err)
	}

	// Validate amount
	amount, ok := new(big.Int).SetString(req.Amount, 10)
	if !ok || amount.Sign() <= 0 {
		return MixRequest{}, ErrInvalidAmount
	}

	// Validate targets
	if len(req.Targets) == 0 {
		return MixRequest{}, ErrInvalidTargets
	}

	// Validate target amounts sum to total
	totalTarget := new(big.Int)
	for _, t := range req.Targets {
		tAmount, ok := new(big.Int).SetString(t.Amount, 10)
		if !ok || tAmount.Sign() <= 0 {
			return MixRequest{}, ErrInvalidAmount
		}
		totalTarget.Add(totalTarget, tAmount)
	}
	if amount.Cmp(totalTarget) != 0 {
		return MixRequest{}, ErrTargetAmountMismatch
	}

	// Validate split count
	if req.SplitCount < MinSplitCount || req.SplitCount > MaxSplitCount {
		return MixRequest{}, ErrInvalidSplitCount
	}

	// Auto-determine split count if not specified
	if req.SplitCount == 0 {
		threshold, _ := new(big.Int).SetString(AutoSplitThreshold, 10)
		if amount.Cmp(threshold) > 0 {
			req.SplitCount = 3
		} else {
			req.SplitCount = 1
		}
	}

	// Check service capacity
	deposit, err := s.store.GetServiceDeposit(ctx)
	if err != nil {
		return MixRequest{}, fmt.Errorf("get service deposit: %w", err)
	}
	available, _ := new(big.Int).SetString(deposit.AvailableAmount, 10)
	if available == nil || available.Cmp(amount) < 0 {
		return MixRequest{}, ErrInsufficientCapacity
	}

	// Set timing
	now := time.Now()
	mixDuration := req.MixDuration.ToDuration()
	req.Status = RequestStatusPending
	req.MixStartAt = now
	req.MixEndAt = now.Add(mixDuration)
	req.WithdrawableAt = req.MixEndAt.Add(time.Duration(DefaultWithdrawWaitDays) * 24 * time.Hour)
	req.CreatedAt = now
	req.UpdatedAt = now

	// Generate ZK proof commitment
	if s.tee != nil {
		proofHash, err := s.tee.GenerateZKProof(ctx, req)
		if err != nil {
			s.Logger().Warn("failed to generate ZK proof", "error", err)
		} else {
			req.ZKProofHash = proofHash
		}
	}

	// Create request
	created, err := s.store.CreateMixRequest(ctx, req)
	if err != nil {
		return MixRequest{}, fmt.Errorf("create mix request: %w", err)
	}
	attrs["request_id"] = created.ID

	s.Logger().Info("mix request created",
		"request_id", created.ID,
		"account_id", created.AccountID,
		"amount", created.Amount,
		"targets", len(created.Targets),
		"duration", created.MixDuration,
	)
	s.LogCreated("mix_request", created.ID, created.AccountID)
	s.IncrementCounter("mixer_requests_created_total", map[string]string{"account_id": created.AccountID})
	eventPayload := map[string]any{
		"request_id": created.ID,
		"account_id": created.AccountID,
		"amount":     created.Amount,
	}
	if err := s.PublishEvent(ctx, "mixer.request.created", eventPayload); err != nil {
		if errors.Is(err, core.ErrBusUnavailable) {
			s.Logger().WithError(err).Warn("bus unavailable for mixer request event")
		} else {
			return MixRequest{}, fmt.Errorf("publish mixer event: %w", err)
		}
	}

	return created, nil
}

// GetMixRequest retrieves a mix request by ID.
func (s *Service) GetMixRequest(ctx context.Context, accountID, requestID string) (MixRequest, error) {
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return MixRequest{}, fmt.Errorf("account validation: %w", err)
	}

	req, err := s.store.GetMixRequest(ctx, requestID)
	if err != nil {
		return MixRequest{}, ErrRequestNotFound
	}

	if req.AccountID != accountID {
		return MixRequest{}, ErrRequestNotFound
	}

	return req, nil
}

// ListMixRequests lists mix requests for an account.
func (s *Service) ListMixRequests(ctx context.Context, accountID string, limit int) ([]MixRequest, error) {
	if err := s.ValidateAccountExists(ctx, accountID); err != nil {
		return nil, fmt.Errorf("account validation: %w", err)
	}

	if limit <= 0 {
		limit = 50
	}

	return s.store.ListMixRequests(ctx, accountID, limit)
}

// ConfirmDeposit confirms that user has deposited funds to pool accounts.
func (s *Service) ConfirmDeposit(ctx context.Context, requestID string, txHashes []string) (_ MixRequest, err error) {
	attrs := map[string]string{"request_id": requestID, "resource": "confirm_deposit"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer func() { finish(err) }()

	req, err := s.store.GetMixRequest(ctx, requestID)
	if err != nil {
		return MixRequest{}, ErrRequestNotFound
	}

	if req.Status != RequestStatusPending {
		return MixRequest{}, fmt.Errorf("request is not in pending status")
	}

	// Verify transactions on chain
	totalDeposited := new(big.Int)
	for _, txHash := range txHashes {
		if s.chain != nil {
			confirmed, _, err := s.chain.GetTransactionStatus(ctx, txHash)
			if err != nil {
				return MixRequest{}, fmt.Errorf("verify transaction %s: %w", txHash, err)
			}
			if !confirmed {
				return MixRequest{}, fmt.Errorf("transaction %s not confirmed", txHash)
			}
		}
	}

	// Update request
	req.DepositTxHashes = txHashes
	req.DepositedAmount = totalDeposited.String()
	req.Status = RequestStatusDeposited
	req.UpdatedAt = time.Now()

	// Submit proof to chain
	if s.chain != nil && s.tee != nil {
		signature, err := s.tee.SignAttestation(ctx, []byte(req.ZKProofHash))
		if err != nil {
			s.Logger().Warn("failed to sign attestation", "error", err)
		} else {
			req.TEESignature = signature
			txHash, err := s.chain.SubmitMixProof(ctx, req.ID, req.ZKProofHash, signature)
			if err != nil {
				s.Logger().Warn("failed to submit mix proof", "error", err)
			} else {
				req.OnChainProofTx = txHash
			}
		}
	}

	updated, err := s.store.UpdateMixRequest(ctx, req)
	if err != nil {
		return MixRequest{}, fmt.Errorf("update mix request: %w", err)
	}
	attrs["account_id"] = updated.AccountID

	s.Logger().Info("deposit confirmed",
		"request_id", updated.ID,
		"tx_count", len(txHashes),
	)
	s.LogAction("deposit_confirmed", "mix_request", updated.ID, updated.AccountID)
	s.IncrementCounter("mixer_deposits_confirmed_total", map[string]string{"account_id": updated.AccountID})

	return updated, nil
}

// StartMixing begins the mixing process for a deposited request.
func (s *Service) StartMixing(ctx context.Context, requestID string) (_ MixRequest, err error) {
	attrs := map[string]string{"request_id": requestID, "resource": "start_mixing"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer func() { finish(err) }()
	req, err := s.store.GetMixRequest(ctx, requestID)
	if err != nil {
		return MixRequest{}, ErrRequestNotFound
	}

	if req.Status != RequestStatusDeposited {
		return MixRequest{}, fmt.Errorf("request is not in deposited status")
	}

	req.Status = RequestStatusMixing
	req.UpdatedAt = time.Now()

	// Schedule internal mixing transactions
	if err := s.scheduleMixingTransactions(ctx, req); err != nil {
		return MixRequest{}, fmt.Errorf("schedule mixing: %w", err)
	}

	updated, err := s.store.UpdateMixRequest(ctx, req)
	if err != nil {
		return MixRequest{}, fmt.Errorf("update mix request: %w", err)
	}
	attrs["account_id"] = updated.AccountID

	s.Logger().Info("mixing started", "request_id", updated.ID)
	s.LogAction("mixing_started", "mix_request", updated.ID, updated.AccountID)
	s.IncrementCounter("mixer_mixes_started_total", map[string]string{"account_id": updated.AccountID})

	return updated, nil
}

// scheduleMixingTransactions creates internal obfuscation transactions.
func (s *Service) scheduleMixingTransactions(ctx context.Context, req MixRequest) error {
	pools, err := s.store.ListActivePoolAccounts(ctx)
	if err != nil {
		return fmt.Errorf("list pool accounts: %w", err)
	}

	if len(pools) < 2 {
		return fmt.Errorf("insufficient pool accounts for mixing")
	}

	// Calculate mixing schedule based on duration
	mixDuration := req.MixDuration.ToDuration()
	numInternalTxs := 5 // Base number of internal transactions
	if mixDuration >= 24*time.Hour {
		numInternalTxs = 10
	}
	if mixDuration >= 7*24*time.Hour {
		numInternalTxs = 20
	}

	// Schedule internal mixing transactions
	interval := mixDuration / time.Duration(numInternalTxs+1)
	for i := 0; i < numInternalTxs; i++ {
		tx := MixTransaction{
			Type:        MixTxTypeInternal,
			Status:      MixTxStatusScheduled,
			RequestID:   req.ID,
			FromPoolID:  pools[i%len(pools)].ID,
			ToPoolID:    pools[(i+1)%len(pools)].ID,
			ScheduledAt: req.MixStartAt.Add(interval * time.Duration(i+1)),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		if _, err := s.store.CreateMixTransaction(ctx, tx); err != nil {
			return fmt.Errorf("create mix transaction: %w", err)
		}
	}

	// Schedule delivery transactions
	deliveryTime := req.MixEndAt.Add(-5 * time.Minute)
	for _, target := range req.Targets {
		tx := MixTransaction{
			Type:          MixTxTypeDelivery,
			Status:        MixTxStatusScheduled,
			RequestID:     req.ID,
			TargetAddress: target.Address,
			Amount:        target.Amount,
			ScheduledAt:   deliveryTime,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		if _, err := s.store.CreateMixTransaction(ctx, tx); err != nil {
			return fmt.Errorf("create delivery transaction: %w", err)
		}
	}

	return nil
}

// CompleteMixRequest marks a request as completed after all deliveries.
func (s *Service) CompleteMixRequest(ctx context.Context, requestID string) (_ MixRequest, err error) {
	attrs := map[string]string{"request_id": requestID, "resource": "complete_mixing"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer func() { finish(err) }()
	req, err := s.store.GetMixRequest(ctx, requestID)
	if err != nil {
		return MixRequest{}, ErrRequestNotFound
	}

	if req.Status != RequestStatusMixing {
		return MixRequest{}, fmt.Errorf("request is not in mixing status")
	}

	// Verify all targets delivered
	allDelivered := true
	for _, t := range req.Targets {
		if !t.Delivered {
			allDelivered = false
			break
		}
	}

	if !allDelivered {
		return MixRequest{}, fmt.Errorf("not all targets delivered")
	}

	// Submit completion proof
	if s.chain != nil {
		txHash, err := s.chain.SubmitCompletionProof(ctx, req.ID, req.DeliveredAmount)
		if err != nil {
			s.Logger().Warn("failed to submit completion proof", "error", err)
		} else {
			req.CompletionProofTx = txHash
		}
	}

	req.Status = RequestStatusCompleted
	req.CompletedAt = time.Now()
	req.UpdatedAt = time.Now()

	updated, err := s.store.UpdateMixRequest(ctx, req)
	if err != nil {
		return MixRequest{}, fmt.Errorf("update mix request: %w", err)
	}
	attrs["account_id"] = updated.AccountID

	s.Logger().Info("mix request completed",
		"request_id", updated.ID,
		"delivered_amount", updated.DeliveredAmount,
	)
	s.LogAction("mix_completed", "mix_request", updated.ID, updated.AccountID)
	s.IncrementCounter("mixer_requests_completed_total", map[string]string{"account_id": updated.AccountID})

	return updated, nil
}

// CreateWithdrawalClaim creates an emergency withdrawal claim when service is unavailable.
func (s *Service) CreateWithdrawalClaim(ctx context.Context, requestID, claimAddress string) (_ WithdrawalClaim, err error) {
	attrs := map[string]string{"request_id": requestID, "resource": "withdrawal_claim"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer func() { finish(err) }()

	req, err := s.store.GetMixRequest(ctx, requestID)
	if err != nil {
		return WithdrawalClaim{}, ErrRequestNotFound
	}

	// Check if request is withdrawable
	now := time.Now()
	if now.Before(req.WithdrawableAt) {
		return WithdrawalClaim{}, ErrRequestNotWithdrawable
	}

	if req.Status == RequestStatusCompleted || req.Status == RequestStatusRefunded {
		return WithdrawalClaim{}, fmt.Errorf("request already completed or refunded")
	}

	// Check for existing claim
	existing, err := s.store.GetWithdrawalClaimByRequest(ctx, requestID)
	if err == nil && existing.ID != "" {
		return WithdrawalClaim{}, ErrClaimAlreadyExists
	}

	claim := WithdrawalClaim{
		RequestID:    requestID,
		AccountID:    req.AccountID,
		ClaimAmount:  req.Amount,
		ClaimAddress: claimAddress,
		Status:       ClaimStatusPending,
		ClaimableAt:  now.Add(time.Duration(DefaultWithdrawWaitDays) * 24 * time.Hour),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	created, err := s.store.CreateWithdrawalClaim(ctx, claim)
	if err != nil {
		return WithdrawalClaim{}, fmt.Errorf("create withdrawal claim: %w", err)
	}
	attrs["account_id"] = created.AccountID

	// Update request status
	req.Status = RequestStatusWithdrawable
	req.UpdatedAt = now
	if _, err := s.store.UpdateMixRequest(ctx, req); err != nil {
		s.Logger().Warn("failed to update request status", "error", err)
	}

	s.Logger().Info("withdrawal claim created",
		"claim_id", created.ID,
		"request_id", created.RequestID,
		"amount", created.ClaimAmount,
	)
	s.LogCreated("withdrawal_claim", created.ID, created.AccountID)
	s.IncrementCounter("mixer_withdrawal_claims_created_total", map[string]string{"account_id": created.AccountID})

	return created, nil
}

// GetMixStats returns service statistics.
func (s *Service) GetMixStats(ctx context.Context) (MixStats, error) {
	return s.store.GetMixStats(ctx)
}

// GetPoolAccounts returns active pool accounts (admin only).
func (s *Service) GetPoolAccounts(ctx context.Context) ([]PoolAccount, error) {
	return s.store.ListActivePoolAccounts(ctx)
}

// CreatePoolAccount creates a new pool account using Double-Blind HD 1/2 Multi-sig.
//
// Process:
// 1. Get next available HD index from TEE
// 2. Get Master public key at that index (from offline-derived public key store)
// 3. Derive TEE public key at that index
// 4. Create Neo N3 1-of-2 multi-sig address from both keys
// 5. Store pool account with HD configuration
//
// The resulting address can be signed by either:
// - TEE key (daily operations, automated)
// - Master key (recovery, offline signing)
func (s *Service) CreatePoolAccount(ctx context.Context) (_ PoolAccount, err error) {
	if s.tee == nil {
		return PoolAccount{}, fmt.Errorf("TEE manager not configured")
	}
	if s.master == nil {
		return PoolAccount{}, fmt.Errorf("Master key provider not configured")
	}
	attrs := map[string]string{"resource": "pool_account"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer func() { finish(err) }()

	// Step 1: Get next HD index
	hdIndex, err := s.tee.GetNextPoolIndex(ctx)
	if err != nil {
		return PoolAccount{}, fmt.Errorf("get next pool index: %w", err)
	}

	// Step 2: Get Master public key at this index
	masterPubKey, err := s.master.GetMasterPublicKey(ctx, hdIndex)
	if err != nil {
		return PoolAccount{}, fmt.Errorf("get master public key at index %d: %w", hdIndex, err)
	}

	// Step 3: Derive TEE keys and create multi-sig address
	keyPair, err := s.tee.DerivePoolKeys(ctx, hdIndex, masterPubKey)
	if err != nil {
		return PoolAccount{}, fmt.Errorf("derive pool keys at index %d: %w", hdIndex, err)
	}

	now := time.Now()
	pool := PoolAccount{
		WalletAddress:   keyPair.Address,
		Status:          PoolAccountStatusActive,
		HDIndex:         keyPair.Index,
		TEEPublicKey:    fmt.Sprintf("%x", keyPair.TEEPublicKey),
		MasterPublicKey: fmt.Sprintf("%x", keyPair.MasterPublicKey),
		MultiSigScript:  fmt.Sprintf("%x", keyPair.MultiSigScript),
		Balance:         "0",
		PendingIn:       "0",
		PendingOut:      "0",
		TotalReceived:   "0",
		TotalSent:       "0",
		RetireAfter:     now.Add(30 * 24 * time.Hour), // Retire after 30 days
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	created, err := s.store.CreatePoolAccount(ctx, pool)
	if err != nil {
		return PoolAccount{}, fmt.Errorf("create pool account: %w", err)
	}
	attrs["pool_id"] = created.ID

	s.Logger().Info("pool account created with HD 1/2 multi-sig",
		"pool_id", created.ID,
		"hd_index", created.HDIndex,
		"wallet", created.WalletAddress,
	)
	s.LogCreated("pool_account", created.ID, "")
	s.IncrementCounter("mixer_pool_accounts_created_total", nil)

	return created, nil
}

// RetirePoolAccount marks a pool account for retirement.
func (s *Service) RetirePoolAccount(ctx context.Context, poolID string) (_ PoolAccount, err error) {
	attrs := map[string]string{"pool_id": poolID, "resource": "retire_pool_account"}
	ctx, finish := s.StartObservation(ctx, attrs)
	defer func() { finish(err) }()
	pool, err := s.store.GetPoolAccount(ctx, poolID)
	if err != nil {
		return PoolAccount{}, fmt.Errorf("get pool account: %w", err)
	}

	pool.Status = PoolAccountStatusRetiring
	pool.UpdatedAt = time.Now()

	updated, err := s.store.UpdatePoolAccount(ctx, pool)
	if err != nil {
		return PoolAccount{}, fmt.Errorf("update pool account: %w", err)
	}

	s.Logger().Info("pool account retiring", "pool_id", updated.ID)
	s.LogUpdated("pool_account", updated.ID, "")
	s.IncrementCounter("mixer_pool_accounts_retired_total", nil)

	return updated, nil
}

// HTTP API Methods
// These methods follow the HTTP{Method}{Path} naming convention for automatic route discovery.

// HTTPGetRequests handles GET /requests - list mix requests for an account.
func (s *Service) HTTPGetRequests(ctx context.Context, req core.APIRequest) (any, error) {
	limit := core.ParseLimitFromQuery(req.Query)
	return s.ListMixRequests(ctx, req.AccountID, limit)
}

// HTTPPostRequests handles POST /requests - create a new mix request.
func (s *Service) HTTPPostRequests(ctx context.Context, req core.APIRequest) (any, error) {
	sourceWallet, _ := req.Body["source_wallet"].(string)
	amount, _ := req.Body["amount"].(string)
	tokenAddress, _ := req.Body["token_address"].(string)
	mixDurationStr, _ := req.Body["mix_duration"].(string)
	splitCount := 0
	if sc, ok := req.Body["split_count"].(float64); ok {
		splitCount = int(sc)
	}

	var targets []MixTarget
	if rawTargets, ok := req.Body["targets"].([]any); ok {
		for _, rt := range rawTargets {
			if t, ok := rt.(map[string]any); ok {
				target := MixTarget{
					Address: core.GetString(t, "address"),
					Amount:  core.GetString(t, "amount"),
				}
				targets = append(targets, target)
			}
		}
	}

	metadata := core.ExtractMetadataRaw(req.Body, "")

	mixReq := MixRequest{
		AccountID:    req.AccountID,
		SourceWallet: sourceWallet,
		Amount:       amount,
		TokenAddress: tokenAddress,
		MixDuration:  ParseMixDuration(mixDurationStr),
		SplitCount:   splitCount,
		Targets:      targets,
		Metadata:     metadata,
	}

	return s.CreateMixRequest(ctx, mixReq)
}

// HTTPGetRequestsById handles GET /requests/{id} - get a specific mix request.
func (s *Service) HTTPGetRequestsById(ctx context.Context, req core.APIRequest) (any, error) {
	requestID := req.PathParams["id"]
	return s.GetMixRequest(ctx, req.AccountID, requestID)
}

// HTTPPostRequestsIdDeposit handles POST /requests/{id}/deposit - confirm deposit.
func (s *Service) HTTPPostRequestsIdDeposit(ctx context.Context, req core.APIRequest) (any, error) {
	requestID := req.PathParams["id"]

	// Verify ownership first
	mixReq, err := s.GetMixRequest(ctx, req.AccountID, requestID)
	if err != nil {
		return nil, err
	}
	if mixReq.AccountID != req.AccountID {
		return nil, fmt.Errorf("forbidden: request belongs to different account")
	}

	var txHashes []string
	if rawHashes, ok := req.Body["tx_hashes"].([]any); ok {
		for _, h := range rawHashes {
			if str, ok := h.(string); ok {
				txHashes = append(txHashes, str)
			}
		}
	}

	return s.ConfirmDeposit(ctx, requestID, txHashes)
}

// HTTPPostRequestsIdClaim handles POST /requests/{id}/claim - create withdrawal claim.
func (s *Service) HTTPPostRequestsIdClaim(ctx context.Context, req core.APIRequest) (any, error) {
	requestID := req.PathParams["id"]

	// Verify ownership first
	mixReq, err := s.GetMixRequest(ctx, req.AccountID, requestID)
	if err != nil {
		return nil, err
	}
	if mixReq.AccountID != req.AccountID {
		return nil, fmt.Errorf("forbidden: request belongs to different account")
	}

	claimAddress, _ := req.Body["claim_address"].(string)
	return s.CreateWithdrawalClaim(ctx, requestID, claimAddress)
}

// HTTPGetStats handles GET /stats - get mixer statistics.
func (s *Service) HTTPGetStats(ctx context.Context, req core.APIRequest) (any, error) {
	return s.GetMixStats(ctx)
}
