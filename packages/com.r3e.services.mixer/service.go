package mixer

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/R3E-Network/service_layer/pkg/logger"
	engine "github.com/R3E-Network/service_layer/system/core"
	"github.com/R3E-Network/service_layer/system/framework"
	core "github.com/R3E-Network/service_layer/system/framework/core"
)

// Service provides privacy-preserving transaction mixing.
type Service struct {
	framework.ServiceBase
	base     *core.Base
	accounts AccountChecker
	store    Store
	tee      TEEManager
	chain    ChainClient
	log      *logger.Logger
	hooks    core.ObservationHooks
}

// Name returns the stable engine module name.
func (s *Service) Name() string { return "mixer" }

// Domain reports the service domain for engine grouping.
func (s *Service) Domain() string { return "mixer" }

// Manifest describes the service contract for the engine OS.
func (s *Service) Manifest() *framework.Manifest {
	return &framework.Manifest{
		Name:         s.Name(),
		Domain:       s.Domain(),
		Description:  "Privacy-preserving transaction mixing service",
		Layer:        "service",
		DependsOn:    []string{"store", "svc-accounts", "svc-confidential"},
		RequiresAPIs: []engine.APISurface{engine.APISurfaceStore},
		Capabilities: []string{"mixer.request", "mixer.withdraw"},
		Quotas:       map[string]string{"mixer": "request-limits"},
	}
}

// Descriptor advertises the service for system discovery.
func (s *Service) Descriptor() core.Descriptor { return s.Manifest().ToDescriptor() }

// WithObservationHooks sets observation hooks for metrics/tracing.
func (s *Service) WithObservationHooks(hooks core.ObservationHooks) {
	s.hooks = core.NormalizeHooks(hooks)
}

// New constructs a mixer service.
func New(accounts AccountChecker, store Store, tee TEEManager, chain ChainClient, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NewDefault("mixer")
	}
	svc := &Service{
		accounts: accounts,
		store:    store,
		tee:      tee,
		chain:    chain,
		log:      log,
		hooks:    core.NoopObservationHooks,
	}
	svc.SetName(svc.Name())
	return svc
}

// Errors
var (
	ErrInvalidAmount      = errors.New("invalid amount")
	ErrInvalidTargets     = errors.New("invalid targets: at least one target required")
	ErrTargetAmountMismatch = errors.New("target amounts do not match total amount")
	ErrInvalidSplitCount  = errors.New("split count must be between 1 and 5")
	ErrInsufficientCapacity = errors.New("insufficient service capacity")
	ErrRequestNotFound    = errors.New("mix request not found")
	ErrRequestNotWithdrawable = errors.New("request is not withdrawable")
	ErrClaimAlreadyExists = errors.New("withdrawal claim already exists")
	ErrInvalidProof       = errors.New("invalid proof")
)

// CreateMixRequest creates a new privacy mixing request.
func (s *Service) CreateMixRequest(ctx context.Context, req MixRequest) (MixRequest, error) {
	s.hooks.OnStart(ctx, map[string]string{"op": "create_mix_request", "account_id": req.AccountID})
	start := time.Now()
	defer func() {
		s.hooks.OnComplete(ctx, map[string]string{"op": "create_mix_request"}, nil, time.Since(start))
	}()

	// Validate account
	if err := s.accounts.AccountExists(ctx, req.AccountID); err != nil {
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
			s.log.Warn("failed to generate ZK proof", "error", err)
		} else {
			req.ZKProofHash = proofHash
		}
	}

	// Create request
	created, err := s.store.CreateMixRequest(ctx, req)
	if err != nil {
		return MixRequest{}, fmt.Errorf("create mix request: %w", err)
	}

	s.log.Info("mix request created",
		"request_id", created.ID,
		"account_id", created.AccountID,
		"amount", created.Amount,
		"targets", len(created.Targets),
		"duration", created.MixDuration,
	)

	return created, nil
}

// GetMixRequest retrieves a mix request by ID.
func (s *Service) GetMixRequest(ctx context.Context, accountID, requestID string) (MixRequest, error) {
	if err := s.accounts.AccountExists(ctx, accountID); err != nil {
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
	if err := s.accounts.AccountExists(ctx, accountID); err != nil {
		return nil, fmt.Errorf("account validation: %w", err)
	}

	if limit <= 0 {
		limit = 50
	}

	return s.store.ListMixRequests(ctx, accountID, limit)
}

// ConfirmDeposit confirms that user has deposited funds to pool accounts.
func (s *Service) ConfirmDeposit(ctx context.Context, requestID string, txHashes []string) (MixRequest, error) {
	s.hooks.OnStart(ctx, map[string]string{"op": "confirm_deposit", "request_id": requestID})
	start := time.Now()
	defer func() {
		s.hooks.OnComplete(ctx, map[string]string{"op": "confirm_deposit"}, nil, time.Since(start))
	}()

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
			s.log.Warn("failed to sign attestation", "error", err)
		} else {
			req.TEESignature = signature
			txHash, err := s.chain.SubmitMixProof(ctx, req.ID, req.ZKProofHash, signature)
			if err != nil {
				s.log.Warn("failed to submit mix proof", "error", err)
			} else {
				req.OnChainProofTx = txHash
			}
		}
	}

	updated, err := s.store.UpdateMixRequest(ctx, req)
	if err != nil {
		return MixRequest{}, fmt.Errorf("update mix request: %w", err)
	}

	s.log.Info("deposit confirmed",
		"request_id", updated.ID,
		"tx_count", len(txHashes),
	)

	return updated, nil
}

// StartMixing begins the mixing process for a deposited request.
func (s *Service) StartMixing(ctx context.Context, requestID string) (MixRequest, error) {
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

	s.log.Info("mixing started", "request_id", updated.ID)

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
func (s *Service) CompleteMixRequest(ctx context.Context, requestID string) (MixRequest, error) {
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
			s.log.Warn("failed to submit completion proof", "error", err)
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

	s.log.Info("mix request completed",
		"request_id", updated.ID,
		"delivered_amount", updated.DeliveredAmount,
	)

	return updated, nil
}

// CreateWithdrawalClaim creates an emergency withdrawal claim when service is unavailable.
func (s *Service) CreateWithdrawalClaim(ctx context.Context, requestID, claimAddress string) (WithdrawalClaim, error) {
	s.hooks.OnStart(ctx, map[string]string{"op": "create_withdrawal_claim", "request_id": requestID})
	start := time.Now()
	defer func() {
		s.hooks.OnComplete(ctx, map[string]string{"op": "create_withdrawal_claim"}, nil, time.Since(start))
	}()

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

	// Update request status
	req.Status = RequestStatusWithdrawable
	req.UpdatedAt = now
	if _, err := s.store.UpdateMixRequest(ctx, req); err != nil {
		s.log.Warn("failed to update request status", "error", err)
	}

	s.log.Info("withdrawal claim created",
		"claim_id", created.ID,
		"request_id", created.RequestID,
		"amount", created.ClaimAmount,
	)

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

// CreatePoolAccount creates a new TEE-managed pool account.
func (s *Service) CreatePoolAccount(ctx context.Context) (PoolAccount, error) {
	if s.tee == nil {
		return PoolAccount{}, fmt.Errorf("TEE manager not configured")
	}

	keyID, walletAddress, encryptedKey, err := s.tee.GeneratePoolKey(ctx)
	if err != nil {
		return PoolAccount{}, fmt.Errorf("generate pool key: %w", err)
	}

	now := time.Now()
	pool := PoolAccount{
		WalletAddress:    walletAddress,
		Status:           PoolAccountStatusActive,
		TEEKeyID:         keyID,
		EncryptedPrivKey: encryptedKey,
		Balance:          "0",
		PendingIn:        "0",
		PendingOut:       "0",
		TotalReceived:    "0",
		TotalSent:        "0",
		RetireAfter:      now.Add(30 * 24 * time.Hour), // Retire after 30 days
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	created, err := s.store.CreatePoolAccount(ctx, pool)
	if err != nil {
		return PoolAccount{}, fmt.Errorf("create pool account: %w", err)
	}

	s.log.Info("pool account created",
		"pool_id", created.ID,
		"wallet", created.WalletAddress,
	)

	return created, nil
}

// RetirePoolAccount marks a pool account for retirement.
func (s *Service) RetirePoolAccount(ctx context.Context, poolID string) (PoolAccount, error) {
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

	s.log.Info("pool account retiring", "pool_id", updated.ID)

	return updated, nil
}
