// Package neovault provides HTTP handlers for the privacy neovault service.
package neovaultmarble

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/R3E-Network/service_layer/internal/httputil"
)

// =============================================================================
// HTTP Handlers
// =============================================================================

// handleInfo returns service information.
func (s *Service) handleInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get default token config for info display
	cfg := s.GetTokenConfig(DefaultToken)

	// Get pool info from neoaccounts service
	client, err := s.getNeoAccountsClient()
	if err != nil {
		s.Logger().WithContext(ctx).WithError(err).Warn("failed to create neoaccounts client")
		httputil.ServiceUnavailable(w, "neoaccounts client unavailable")
		return
	}
	poolInfo, err := client.GetPoolInfo(ctx)
	if err != nil {
		httputil.InternalError(w, "failed to get pool info from neoaccounts")
		return
	}

	pendingReqs, err := s.repo.ListByStatus(ctx, string(StatusPending))
	if err != nil {
		s.Logger().WithContext(ctx).WithError(err).WithField("status", string(StatusPending)).Warn("failed to list requests")
	}
	depositedReqs, err := s.repo.ListByStatus(ctx, string(StatusDeposited))
	if err != nil {
		s.Logger().WithContext(ctx).WithError(err).WithField("status", string(StatusDeposited)).Warn("failed to list requests")
	}
	mixingReqs, err := s.repo.ListByStatus(ctx, string(StatusMixing))
	if err != nil {
		s.Logger().WithContext(ctx).WithError(err).WithField("status", string(StatusMixing)).Warn("failed to list requests")
	}

	pendingRequests := len(pendingReqs) + len(depositedReqs)
	mixingRequests := len(mixingReqs)

	// Calculate available capacity based on compliance limits
	// Use token stats for the default token type
	totalBalance := int64(0)
	if tokenStats, ok := poolInfo.TokenStats[DefaultToken]; ok {
		totalBalance = tokenStats.TotalBalance
	}
	availableCapacity := cfg.MaxPoolBalance - totalBalance
	if availableCapacity < 0 {
		availableCapacity = 0
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":             "active",
		"version":            Version,
		"pool_accounts":      poolInfo.TotalAccounts,
		"pool_balance":       totalBalance,
		"token_stats":        poolInfo.TokenStats,
		"available_capacity": availableCapacity,
		"pending_requests":   pendingRequests,
		"mixing_requests":    mixingRequests,
		"service_fee_rate":   cfg.ServiceFeeRate,
		"supported_tokens":   s.GetSupportedTokens(),
		"compliance_limits": map[string]interface{}{
			"max_request_amount": cfg.MaxRequestAmount,
			"max_pool_balance":   cfg.MaxPoolBalance,
		},
		"min_amount": cfg.MinTxAmount * 10,
		"max_amount": cfg.MaxRequestAmount,
	})
}

// handleCreateRequest creates a new mix request.
func (s *Service) handleCreateRequest(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}

	var input CreateRequestInput
	if !httputil.DecodeJSON(w, r, &input) {
		return
	}

	// Determine token type (default to GAS for backward compatibility)
	tokenType := input.TokenType
	if tokenType == "" {
		tokenType = DefaultToken
	}

	// Get config for requested token
	cfg := s.GetTokenConfig(tokenType)
	if cfg == nil {
		httputil.BadRequest(w, fmt.Sprintf("unsupported token type: %s", tokenType))
		return
	}

	// Normalize input: support both new and legacy formats
	targets := input.Targets
	if len(targets) == 0 && input.TotalAmount > 0 {
		// Legacy format: use TotalAmount split across implicit targets
		targets = []TargetAddress{{Address: input.UserAddress, Amount: input.TotalAmount}}
	}

	// Calculate total amount from targets
	totalAmount := int64(0)
	for _, t := range targets {
		totalAmount += t.Amount
	}
	if input.TotalAmount > 0 && input.TotalAmount > totalAmount {
		totalAmount = input.TotalAmount
	}

	// Validate compliance limits
	if totalAmount > cfg.MaxRequestAmount {
		httputil.BadRequest(w, fmt.Sprintf("amount exceeds limit: max %d", cfg.MaxRequestAmount))
		return
	}
	if totalAmount < cfg.MinTxAmount*10 {
		httputil.BadRequest(w, "amount too small")
		return
	}
	if len(targets) == 0 {
		httputil.BadRequest(w, "at least one target address required")
		return
	}

	// Normalize mixing duration
	mixingDuration := time.Duration(input.MixOption) * time.Millisecond
	if input.MixingMinutes > 0 {
		mixingDuration = time.Duration(input.MixingMinutes) * time.Minute
	}
	if mixingDuration < 5*time.Minute {
		mixingDuration = 30 * time.Minute
	}
	if mixingDuration > 7*24*time.Hour {
		mixingDuration = 7 * 24 * time.Hour
	}

	// Normalize splits
	initialSplits := input.InitialSplits
	if initialSplits < 2 {
		initialSplits = 3
	}
	if initialSplits > 10 {
		initialSplits = 10
	}

	// Set timestamp if not provided
	if input.Timestamp == 0 {
		input.Timestamp = time.Now().Unix()
	}
	if input.Version == 0 {
		input.Version = 1
	}

	// Calculate fees using token-specific rate
	serviceFee := int64(float64(totalAmount) * cfg.ServiceFeeRate)
	netAmount := totalAmount - serviceFee

	// Create deposit account (shared pool)
	depositAccount, err := s.createPoolAccount(r.Context())
	if err != nil {
		httputil.InternalError(w, "failed to create deposit address")
		return
	}

	// Calculate deadline (mixing duration + grace period)
	deadline := time.Now().Add(mixingDuration).Add(DisputeGracePeriod).Unix()

	// Prepare canonical request for hashing
	canonicalInput := CreateRequestInput{
		Version:     input.Version,
		UserAddress: input.UserAddress,
		InputTxs:    input.InputTxs,
		Targets:     targets,
		MixOption:   mixingDuration.Milliseconds(),
		Timestamp:   input.Timestamp,
		TokenType:   tokenType,
	}

	// Generate requestHash
	requestBytes, err := json.Marshal(canonicalInput)
	if err != nil {
		httputil.InternalError(w, "failed to serialize request")
		return
	}
	hashBytes := sha256.Sum256(requestBytes)
	requestHash := hex.EncodeToString(hashBytes[:])

	// Generate TEE signature over requestHash
	teeSignature := s.signRequestHash(requestHash)

	request := &MixRequest{
		ID:              uuid.New().String(),
		UserID:          userID,
		UserAddress:     input.UserAddress,
		TokenType:       tokenType,
		Status:          StatusPending,
		TotalAmount:     totalAmount,
		ServiceFee:      serviceFee,
		NetAmount:       netAmount,
		TargetAddresses: targets,
		InitialSplits:   initialSplits,
		MixingDuration:  mixingDuration,
		DepositAddress:  depositAccount.Address,
		PoolAccounts:    []string{depositAccount.ID},
		RequestHash:     requestHash,
		TEESignature:    teeSignature,
		Deadline:        deadline,
		CreatedAt:       time.Now(),
	}

	if err := s.repo.Create(r.Context(), RequestToRecord(request)); err != nil {
		httputil.InternalError(w, "failed to persist request")
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, CreateRequestResponse{
		Request:        &canonicalInput,
		RequestID:      request.ID,
		RequestHash:    requestHash,
		TEESignature:   teeSignature,
		DepositAddress: request.DepositAddress,
		TotalAmount:    totalAmount,
		ServiceFee:     serviceFee,
		NetAmount:      netAmount,
		Deadline:       deadline,
		ExpiresAt:      time.Now().Add(24 * time.Hour).Format(time.RFC3339),
	})
}

// handleGetStatus returns a lightweight status view of a mix request.
func (s *Service) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}

	requestID := mux.Vars(r)["id"]

	rec, err := s.repo.GetByID(r.Context(), requestID)
	if err != nil {
		httputil.NotFound(w, "request not found")
		return
	}
	request := RequestFromRecord(rec)

	// Verify ownership
	if request.UserID != userID {
		httputil.WriteError(w, http.StatusForbidden, "not authorized to view this request")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"request_id":   request.ID,
		"status":       request.Status,
		"request_hash": request.RequestHash,
		"deadline":     request.Deadline,
		"created_at":   request.CreatedAt,
		"delivered_at": request.DeliveredAt,
	})
}

// handleGetRequest returns the full details of a mix request.
func (s *Service) handleGetRequest(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}

	requestID := mux.Vars(r)["id"]

	rec, err := s.repo.GetByID(r.Context(), requestID)
	if err != nil {
		httputil.NotFound(w, "request not found")
		return
	}
	request := RequestFromRecord(rec)

	// Verify ownership
	if request.UserID != userID {
		httputil.WriteError(w, http.StatusForbidden, "not authorized to view this request")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, request)
}

// handleConfirmDeposit confirms a deposit for a mix request.
func (s *Service) handleConfirmDeposit(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}

	requestID := mux.Vars(r)["id"]

	var input ConfirmDepositInput
	if !httputil.DecodeJSON(w, r, &input) {
		return
	}

	rec, err := s.repo.GetByID(r.Context(), requestID)
	if err != nil {
		httputil.NotFound(w, "request not found")
		return
	}
	request := RequestFromRecord(rec)

	// Verify ownership
	if request.UserID != userID {
		httputil.WriteError(w, http.StatusForbidden, "not authorized to modify this request")
		return
	}

	if request.Status != StatusPending {
		httputil.BadRequest(w, "request already processed")
		return
	}

	if input.TxHash == "" {
		httputil.BadRequest(w, "tx_hash required")
		return
	}

	// Deposit verification: user deposits directly to pool account on-chain
	// No gasbank involvement - preserves privacy (no link to service layer accounts)
	request.DepositTxHash = input.TxHash
	request.Status = StatusDeposited
	request.DepositedAt = time.Now()

	if err := s.repo.Update(r.Context(), RequestToRecord(request)); err != nil {
		httputil.InternalError(w, "failed to update request")
		return
	}

	// Start mixing process asynchronously after deposit confirmation
	go s.startMixing(context.Background(), request)

	httputil.WriteJSON(w, http.StatusOK, map[string]string{
		"status":  "deposited",
		"message": "Mixing will begin shortly",
	})
}

// handleListRequests lists all requests for the current user.
func (s *Service) handleListRequests(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}

	rows, err := s.repo.ListByUser(r.Context(), userID)
	if err != nil {
		httputil.InternalError(w, "failed to load requests")
		return
	}
	requests := make([]*MixRequest, 0, len(rows))
	for i := range rows {
		req := RequestFromRecord(&rows[i])
		requests = append(requests, req)
	}

	httputil.WriteJSON(w, http.StatusOK, requests)
}

// handleResumeRequest re-queues a mixing/delivery for a given request.
func (s *Service) handleResumeRequest(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}

	requestID := mux.Vars(r)["id"]

	rec, err := s.repo.GetByID(r.Context(), requestID)
	if err != nil {
		httputil.NotFound(w, "request not found")
		return
	}
	req := RequestFromRecord(rec)

	// Verify ownership
	if req.UserID != userID {
		httputil.WriteError(w, http.StatusForbidden, "not authorized to resume this request")
		return
	}

	switch req.Status {
	case StatusDeposited:
		go s.startMixing(context.Background(), req)
	case StatusMixing:
		// Delivery checker will pick it up; optionally trigger immediately if duration elapsed
		go s.checkDeliveries(context.Background())
	default:
		httputil.BadRequest(w, "nothing to resume for current status")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "resumed"})
}

// handleDispute processes a user dispute and submits completion proof on-chain.
func (s *Service) handleDispute(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}

	requestID := mux.Vars(r)["id"]

	var input DisputeInput
	if !httputil.DecodeJSONOptional(w, r, &input) {
		return
	}

	rec, err := s.repo.GetByID(r.Context(), requestID)
	if err != nil {
		httputil.NotFound(w, "request not found")
		return
	}
	request := RequestFromRecord(rec)

	// Verify ownership
	if request.UserID != userID {
		httputil.WriteError(w, http.StatusForbidden, "not authorized to dispute this request")
		return
	}

	// Check if within dispute deadline
	if time.Now().Unix() > request.Deadline {
		httputil.BadRequest(w, "dispute deadline passed")
		return
	}

	response := DisputeResponse{
		RequestID: request.ID,
		Status:    string(request.Status),
	}

	switch request.Status {
	case StatusDelivered:
		// Mix completed - submit completion proof on-chain as evidence
		if request.CompletionProof == nil {
			httputil.InternalError(w, "completion proof not available")
			return
		}

		// Submit proof on-chain via TEE fulfiller (ONLY TIME we submit to chain)
		txHash, err := s.submitCompletionProofOnChain(r.Context(), request)
		if err != nil {
			httputil.InternalError(w, fmt.Sprintf("failed to submit proof: %v", err))
			return
		}

		response.CompletionProof = request.CompletionProof
		response.OnChainTxHash = txHash
		response.Message = "Completion proof submitted on-chain. Dispute resolved - mixing was completed."

	case StatusPending, StatusDeposited, StatusMixing:
		// Mix not yet complete - user can claim refund via on-chain dispute contract
		response.Message = fmt.Sprintf("Request status: %s. If not completed by deadline (%s), you can claim refund via dispute contract using your RequestProof.",
			request.Status, time.Unix(request.Deadline, 0).Format(time.RFC3339))

	case StatusFailed, StatusRefunded:
		response.Message = fmt.Sprintf("Request already %s. No dispute needed.", request.Status)

	default:
		httputil.InternalError(w, "unknown request status")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, response)
}

// handleGetCompletionProof returns the completion proof for a delivered request.
func (s *Service) handleGetCompletionProof(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}

	requestID := mux.Vars(r)["id"]

	rec, err := s.repo.GetByID(r.Context(), requestID)
	if err != nil {
		httputil.NotFound(w, "request not found")
		return
	}
	request := RequestFromRecord(rec)

	// Verify ownership
	if request.UserID != userID {
		httputil.WriteError(w, http.StatusForbidden, "not authorized to view this proof")
		return
	}

	if request.Status != StatusDelivered {
		httputil.BadRequest(w, fmt.Sprintf("request not delivered yet (status: %s)", request.Status))
		return
	}

	if request.CompletionProof == nil {
		httputil.InternalError(w, "completion proof not available")
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"request_id":       request.ID,
		"status":           request.Status,
		"completion_proof": request.CompletionProof,
		"message":          "Proof generated. Not submitted on-chain unless you file a dispute.",
	})
}
