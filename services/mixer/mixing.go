// Package mixer provides mixing logic for the privacy mixer service.
package mixer

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	mrand "math/rand"
	"sort"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/crypto"
)

// =============================================================================
// Mixing Logic
// =============================================================================

// startMixing begins the mixing process for a request.
func (s *Service) startMixing(ctx context.Context, request *MixRequest) {
	request.Status = StatusMixing
	request.MixingStartAt = time.Now()

	// Persist status
	_ = s.DB().UpdateMixerRequest(ctx, RequestToRecord(request))

	// Get deposit account info from accountpool
	depositAccountID := request.PoolAccounts[0]

	// Get available accounts (from accountpool service)
	targetAccounts, err := s.getAvailableAccounts(ctx, request.InitialSplits)
	if err != nil || len(targetAccounts) == 0 {
		return
	}
	splitAmounts := s.randomSplit(request.NetAmount, len(targetAccounts))

	// Update balances via accountpool service
	for i, acc := range targetAccounts {
		if acc.ID != depositAccountID {
			request.PoolAccounts = append(request.PoolAccounts, acc.ID)
		}
		// Update balance via accountpool
		_ = s.updateAccountBalance(ctx, acc.ID, splitAmounts[i])
	}
	// Deduct from deposit account
	_ = s.updateAccountBalance(ctx, depositAccountID, -request.NetAmount)

	// Persist updated pool accounts list
	_ = s.DB().UpdateMixerRequest(ctx, RequestToRecord(request))
}

// runMixingLoop continuously generates random mixing transactions.
func (s *Service) runMixingLoop(ctx context.Context) {
	// Random interval between transactions
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		default:
			minDelay := time.Minute / time.Duration(MaxMixingTxPerMinute)
			maxDelay := time.Minute / time.Duration(MinMixingTxPerMinute)
			if maxDelay < minDelay {
				maxDelay = minDelay
			}
			delayRange := maxDelay - minDelay
			delay := minDelay
			if delayRange > 0 {
				delay += time.Duration(mrand.Int63n(int64(delayRange)))
			}
			time.Sleep(delay)

			s.executeMixingTransaction(ctx)
		}
	}
}

// executeMixingTransaction performs a random transfer between pool accounts.
// Pool accounts are managed by accountpool service.
func (s *Service) executeMixingTransaction(ctx context.Context) {
	activeAccounts, err := s.getActiveAccounts(ctx)
	if err != nil || len(activeAccounts) < 2 {
		return
	}

	// Use default token config for tx limits
	cfg := s.GetTokenConfig(DefaultToken)

	mrand.Shuffle(len(activeAccounts), func(i, j int) {
		activeAccounts[i], activeAccounts[j] = activeAccounts[j], activeAccounts[i]
	})

	source := activeAccounts[0]
	dest := activeAccounts[1]

	maxAmount := source.Balance * 9 / 10
	if maxAmount < cfg.MinTxAmount {
		return
	}
	if maxAmount > cfg.MaxTxAmount {
		maxAmount = cfg.MaxTxAmount
	}

	amount := cfg.MinTxAmount + mrand.Int63n(maxAmount-cfg.MinTxAmount)

	// Update balances via accountpool service
	_ = s.updateAccountBalance(ctx, source.ID, -amount)
	_ = s.updateAccountBalance(ctx, dest.ID, amount)
}

// =============================================================================
// Delivery Logic
// =============================================================================

// runDeliveryChecker checks for requests ready for delivery.
func (s *Service) runDeliveryChecker(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkDeliveries(ctx)
		}
	}
}

// checkDeliveries processes requests that have completed mixing.
func (s *Service) checkDeliveries(ctx context.Context) {
	now := time.Now()
	mixing, err := s.DB().ListMixerRequestsByStatus(ctx, string(StatusMixing))
	if err != nil {
		return
	}

	for i := range mixing {
		req := RequestFromRecord(&mixing[i])
		if now.Sub(req.MixingStartAt) < req.MixingDuration {
			continue
		}
		s.deliverTokens(ctx, req)
	}
}

// deliverTokens sends tokens to target addresses and generates CompletionProof.
func (s *Service) deliverTokens(ctx context.Context, request *MixRequest) {
	// Get current balances from accountpool
	client := s.getAccountPoolClient()
	accounts, err := client.GetLockedAccounts(ctx, nil)
	if err != nil {
		return
	}

	// Build map of account IDs used in this request
	requestAccountSet := make(map[string]bool)
	for _, accID := range request.PoolAccounts {
		requestAccountSet[accID] = true
	}

	// Calculate total available from request's accounts
	totalAvailable := int64(0)
	poolAccounts := make([]*PoolAccount, 0, len(request.PoolAccounts))
	for _, acc := range accounts {
		if !requestAccountSet[acc.ID] {
			continue
		}
		poolAccounts = append(poolAccounts, &PoolAccount{
			ID:      acc.ID,
			Address: acc.Address,
			Balance: acc.Balance,
		})
		totalAvailable += acc.Balance
	}

	if totalAvailable == 0 {
		return
	}

	targetCount := len(request.TargetAddresses)
	deliveryAmounts := s.randomSplit(totalAvailable, targetCount)

	// Track output transactions for completion proof
	outputTxIDs := make([]string, 0, targetCount)

	for i, target := range request.TargetAddresses {
		amount := deliveryAmounts[i]
		if target.Amount > 0 {
			amount = target.Amount
		}

		remaining := amount
		for _, acc := range poolAccounts {
			if remaining <= 0 {
				break
			}
			if acc.Balance <= 0 {
				continue
			}

			transfer := acc.Balance
			if transfer > remaining {
				transfer = remaining
			}
			acc.Balance -= transfer
			remaining -= transfer

			// Update balance via accountpool
			_ = s.updateAccountBalance(ctx, acc.ID, -transfer)

			// Generate output tx ID (in production: actual blockchain tx hash)
			outputTxID := fmt.Sprintf("out_%s_%s_%d", request.ID[:8], target.Address[:8], amount)
			outputTxIDs = append(outputTxIDs, outputTxID)
		}
	}

	// Generate CompletionProof (stored, NOT submitted on-chain unless disputed)
	completionProof := s.generateCompletionProof(request, outputTxIDs)

	request.Status = StatusDelivered
	request.DeliveredAt = time.Now()
	request.OutputTxIDs = outputTxIDs
	request.CompletionProof = completionProof
	_ = s.DB().UpdateMixerRequest(ctx, RequestToRecord(request))

	// Release accounts back to accountpool service
	_ = s.releasePoolAccounts(ctx, request.PoolAccounts)
}

// generateCompletionProof creates the proof that mixing was completed.
// This proof is stored but NOT submitted on-chain unless user disputes.
func (s *Service) generateCompletionProof(request *MixRequest, outputTxIDs []string) *CompletionProof {
	// Sort output tx IDs for deterministic hash
	sortedTxIDs := make([]string, len(outputTxIDs))
	copy(sortedTxIDs, outputTxIDs)
	sort.Strings(sortedTxIDs)

	// Hash the outputs
	outputsData := strings.Join(sortedTxIDs, ",")
	outputsHash := sha256.Sum256([]byte(outputsData))
	outputsHashHex := hex.EncodeToString(outputsHash[:])

	completedAt := time.Now().Unix()

	// Generate TEE signature over completion data
	signatureData := fmt.Sprintf("%s:%s:%s:%d",
		request.ID, request.RequestHash, outputsHashHex, completedAt)
	signature := crypto.HMACSign(s.masterKey, []byte(signatureData))

	return &CompletionProof{
		RequestID:    request.ID,
		RequestHash:  request.RequestHash,
		OutputsHash:  outputsHashHex,
		OutputTxIDs:  sortedTxIDs,
		CompletedAt:  completedAt,
		TEESignature: hex.EncodeToString(signature),
	}
}

// =============================================================================
// Utility Functions
// =============================================================================

// randomSplit splits an amount into n random parts.
func (s *Service) randomSplit(total int64, n int) []int64 {
	if n <= 0 {
		return nil
	}
	if n == 1 {
		return []int64{total}
	}

	splits := make([]int64, n)
	remaining := total
	for i := 0; i < n; i++ {
		left := n - i
		if left == 1 {
			splits[i] = remaining
			break
		}
		max := remaining - int64(left-1)
		if max < 1 {
			max = 1
		}
		draw := int64(1)
		b := make([]byte, 8)
		if _, err := rand.Read(b); err == nil {
			random := new(big.Int).SetBytes(b)
			random.Mod(random, big.NewInt(max))
			draw = 1 + random.Int64()
		} else {
			mrand.Seed(time.Now().UnixNano())
			draw = 1 + mrand.Int63n(max)
		}
		if draw > max {
			draw = max
		}
		splits[i] = draw
		remaining -= splits[i]
	}
	return splits
}

// signRequestHash generates TEE signature over requestHash using master key
func (s *Service) signRequestHash(requestHash string) string {
	if len(s.masterKey) == 0 {
		return ""
	}
	hashBytes, err := hex.DecodeString(requestHash)
	if err != nil {
		log.Printf("Failed to decode request hash: %v", err)
		return ""
	}
	signature := crypto.HMACSign(s.masterKey, hashBytes)
	return hex.EncodeToString(signature)
}
