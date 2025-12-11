// Package neovault provides mixing logic for the privacy neovault service.
package neovaultmarble

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/crypto"
)

// =============================================================================
// Secure Random Helpers (crypto/rand based)
// =============================================================================

// deliveryAccount is a local struct for tracking account balance during delivery.
// This is used instead of PoolAccount to avoid mutating the shared Balances map.
type deliveryAccount struct {
	ID      string
	Address string
	Balance int64 // Local balance for a specific token type during delivery
}

// secureInt64n returns a cryptographically secure random int64 in [0, n).
func secureInt64n(n int64) int64 {
	if n <= 0 {
		return 0
	}
	max := new(big.Int).SetInt64(n)
	result, err := rand.Int(rand.Reader, max)
	if err != nil {
		// Fallback should never happen with crypto/rand
		return 0
	}
	return result.Int64()
}

// secureShuffle shuffles a slice using crypto/rand (Fisher-Yates algorithm).
func secureShuffle(n int, swap func(i, j int)) {
	for i := n - 1; i > 0; i-- {
		j := int(secureInt64n(int64(i + 1)))
		swap(i, j)
	}
}

// =============================================================================
// Mixing Logic
// =============================================================================

// startMixing begins the mixing process for a request.
func (s *Service) startMixing(ctx context.Context, request *MixRequest) {
	if s.repo == nil {
		return
	}
	request.Status = StatusMixing
	request.MixingStartAt = time.Now()

	// Persist status
	if err := s.repo.Update(ctx, RequestToRecord(request)); err != nil {
		log.Printf("[neovault] failed to update request status to mixing: %v", err)
	}

	// Get deposit account info from neoaccounts
	depositAccountID := request.PoolAccounts[0]

	// Get available accounts (from neoaccounts service)
	targetAccounts, err := s.getAvailableAccounts(ctx, request.InitialSplits)
	if err != nil || len(targetAccounts) == 0 {
		return
	}
	splitAmounts := s.randomSplit(request.NetAmount, len(targetAccounts))

	// Update balances via neoaccounts service
	for i, acc := range targetAccounts {
		if acc.ID != depositAccountID {
			request.PoolAccounts = append(request.PoolAccounts, acc.ID)
		}
		// Update balance via neoaccounts
		if err := s.updateAccountBalance(ctx, acc.ID, splitAmounts[i]); err != nil {
			log.Printf("[neovault] failed to update balance for account %s: %v", acc.ID, err)
		}
	}
	// Deduct from deposit account
	if err := s.updateAccountBalance(ctx, depositAccountID, -request.NetAmount); err != nil {
		log.Printf("[neovault] failed to deduct from deposit account %s: %v", depositAccountID, err)
	}

	// Persist updated pool accounts list
	if err := s.repo.Update(ctx, RequestToRecord(request)); err != nil {
		log.Printf("[neovault] failed to persist pool accounts list: %v", err)
	}
}

// runMixingLoop continuously generates random mixing transactions.
func (s *Service) runMixingLoop(ctx context.Context) {
	// Random interval between transactions
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.StopChan():
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
				delay += time.Duration(secureInt64n(int64(delayRange)))
			}
			time.Sleep(delay)

			s.executeMixingTransaction(ctx)
		}
	}
}

// executeMixingTransaction performs a random transfer between pool accounts.
// Pool accounts are managed by neoaccounts service.
func (s *Service) executeMixingTransaction(ctx context.Context) {
	activeAccounts, err := s.getActiveAccounts(ctx)
	if err != nil || len(activeAccounts) < 2 {
		return
	}

	// Use default token config for tx limits
	cfg := s.GetTokenConfig(DefaultToken)

	secureShuffle(len(activeAccounts), func(i, j int) {
		activeAccounts[i], activeAccounts[j] = activeAccounts[j], activeAccounts[i]
	})

	source := activeAccounts[0]
	dest := activeAccounts[1]

	// Use default token for mixing transactions
	sourceBalance := source.GetBalance(DefaultToken)
	maxAmount := sourceBalance * 9 / 10
	if maxAmount < cfg.MinTxAmount {
		return
	}
	if maxAmount > cfg.MaxTxAmount {
		maxAmount = cfg.MaxTxAmount
	}

	amount := cfg.MinTxAmount + secureInt64n(maxAmount-cfg.MinTxAmount)

	// Update balances via neoaccounts service
	if err := s.updateAccountBalance(ctx, source.ID, -amount); err != nil {
		log.Printf("[neovault] mixing tx: failed to deduct from source %s: %v", source.ID, err)
		return
	}
	if err := s.updateAccountBalance(ctx, dest.ID, amount); err != nil {
		log.Printf("[neovault] mixing tx: failed to credit dest %s: %v", dest.ID, err)
	}
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
		case <-s.StopChan():
			return
		case <-ticker.C:
			s.checkDeliveries(ctx)
		}
	}
}

// checkDeliveries processes requests that have completed mixing.
func (s *Service) checkDeliveries(ctx context.Context) {
	if s.repo == nil {
		return
	}
	now := time.Now()
	mixing, err := s.repo.ListByStatus(ctx, string(StatusMixing))
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
// Privacy-First Fee Model: Fee is deducted from delivery (user receives NetAmount, not TotalAmount).
// After delivery, fee is collected from a random pool account to the master fee address.
func (s *Service) deliverTokens(ctx context.Context, request *MixRequest) {
	// Get current balances from neoaccounts
	client := s.getNeoAccountsClient()
	accounts, err := client.GetLockedAccounts(ctx, nil)
	if err != nil {
		return
	}

	// Build map of account IDs used in this request
	requestAccountSet := make(map[string]bool)
	for _, accID := range request.PoolAccounts {
		requestAccountSet[accID] = true
	}

	// Collect pool accounts for this request with local balance tracking
	// Use deliveryAccount to track balance during delivery (avoiding direct mutation of Balances map)
	poolAccounts := make([]*deliveryAccount, 0, len(request.PoolAccounts))
	for _, acc := range accounts {
		if !requestAccountSet[acc.ID] {
			continue
		}
		// Get balance for the request's token type
		balance := int64(0)
		if acc.Balances != nil {
			if tokenBal, ok := acc.Balances[request.TokenType]; ok {
				balance = tokenBal.Amount
			}
		}
		poolAccounts = append(poolAccounts, &deliveryAccount{
			ID:      acc.ID,
			Address: acc.Address,
			Balance: balance,
		})
	}

	// PRIVACY-FIRST FEE MODEL:
	// Deliver NetAmount (TotalAmount - ServiceFee) to targets.
	// Fee is collected separately from a random pool account to master fee address.
	deliveryTotal := request.NetAmount
	if deliveryTotal <= 0 {
		return
	}

	targetCount := len(request.TargetAddresses)
	deliveryAmounts := s.randomSplit(deliveryTotal, targetCount)

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

			// Transfer tokens from pool account to target address via neoaccounts service
			txHash, err := s.transferToTarget(ctx, acc.ID, target.Address, transfer, request.TokenType)
			if err != nil {
				log.Printf("Failed to transfer %d from %s to %s: %v", transfer, acc.ID, target.Address, err)
				continue
			}

			acc.Balance -= transfer
			remaining -= transfer
			outputTxIDs = append(outputTxIDs, txHash)
		}
	}

	// FEE COLLECTION: Collect service fee from random pool account to master fee address
	// This happens AFTER delivery, from a random account (not directly linked to user)
	if request.ServiceFee > 0 && s.feeCollectionAddress != "" {
		s.collectFeeFromPool(ctx, request, poolAccounts)
	}

	// Generate CompletionProof (stored, NOT submitted on-chain unless disputed)
	completionProof := s.generateCompletionProof(request, outputTxIDs)

	request.Status = StatusDelivered
	request.DeliveredAt = time.Now()
	request.OutputTxIDs = outputTxIDs
	request.CompletionProof = completionProof
	if err := s.repo.Update(ctx, RequestToRecord(request)); err != nil {
		log.Printf("[neovault] failed to update request %s to delivered status: %v", request.ID, err)
	}

	// Release accounts back to neoaccounts service
	if err := s.releasePoolAccounts(ctx, request.PoolAccounts); err != nil {
		log.Printf("[neovault] failed to release pool accounts for request %s: %v", request.ID, err)
	}
}

// collectFeeFromPool collects the service fee from a randomly selected pool account
// and transfers it to the master fee collection address.
// Privacy: Fee is collected from random pool account, not directly from user's deposit.
func (s *Service) collectFeeFromPool(ctx context.Context, request *MixRequest, poolAccounts []*deliveryAccount) {
	if s.feeCollectionAddress == "" || request.ServiceFee <= 0 {
		return
	}

	// Shuffle pool accounts to select randomly
	secureShuffle(len(poolAccounts), func(i, j int) {
		poolAccounts[i], poolAccounts[j] = poolAccounts[j], poolAccounts[i]
	})

	// Find accounts with sufficient balance to cover the fee
	remainingFee := request.ServiceFee
	for _, acc := range poolAccounts {
		if remainingFee <= 0 {
			break
		}
		if acc.Balance <= 0 {
			continue
		}

		// Determine how much to collect from this account
		collectAmount := acc.Balance
		if collectAmount > remainingFee {
			collectAmount = remainingFee
		}

		// Transfer fee from pool account to master fee address via neoaccounts service
		if err := s.transferFeeToMaster(ctx, acc.ID, collectAmount); err != nil {
			log.Printf("Failed to collect fee from account %s: %v", acc.ID, err)
			continue
		}

		acc.Balance -= collectAmount
		remainingFee -= collectAmount

		log.Printf("Collected fee %d from pool account %s to master %s",
			collectAmount, acc.ID, s.feeCollectionAddress)
	}

	if remainingFee > 0 {
		log.Printf("Warning: Could not collect full fee, remaining: %d", remainingFee)
	}
}

// transferFeeToMaster transfers fee from a pool account to the master fee address.
// This uses the neoaccounts service to construct, sign, and broadcast the transaction.
func (s *Service) transferFeeToMaster(ctx context.Context, accountID string, amount int64) error {
	client := s.getNeoAccountsClient()

	// Get token hash for the default token (GAS)
	cfg := s.GetTokenConfig(DefaultToken)
	tokenHash := ""
	if cfg != nil {
		tokenHash = cfg.ScriptHash
	}

	// Transfer fee from pool account to master fee address via neoaccounts service
	// The neoaccounts service handles transaction construction, signing, and broadcasting
	result, err := client.Transfer(ctx, accountID, s.feeCollectionAddress, amount, tokenHash)
	if err != nil {
		return fmt.Errorf("failed to transfer fee to master: %w", err)
	}

	log.Printf("Fee transfer completed: txHash=%s, from=%s, to=%s, amount=%d",
		result.TxHash, accountID, s.feeCollectionAddress, amount)

	return nil
}

// transferToTarget transfers tokens from a pool account to a target address.
// This uses the neoaccounts service to construct, sign, and broadcast the transaction.
// Returns the transaction hash on success.
func (s *Service) transferToTarget(ctx context.Context, accountID, targetAddress string, amount int64, tokenType string) (string, error) {
	client := s.getNeoAccountsClient()

	// Get token hash for the specified token type
	cfg := s.GetTokenConfig(tokenType)
	tokenHash := ""
	if cfg != nil {
		tokenHash = cfg.ScriptHash
	}

	// Transfer tokens from pool account to target address via neoaccounts service
	result, err := client.Transfer(ctx, accountID, targetAddress, amount, tokenHash)
	if err != nil {
		return "", fmt.Errorf("failed to transfer to target: %w", err)
	}

	log.Printf("Delivery transfer completed: txHash=%s, from=%s, to=%s, amount=%d",
		result.TxHash, accountID, targetAddress, amount)

	return result.TxHash, nil
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
		// Use cryptographically secure random (no insecure fallback)
		draw := int64(1) + secureInt64n(max)
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
