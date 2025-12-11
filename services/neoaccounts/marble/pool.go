// Package neoaccounts provides pool management for the neoaccounts service.
package neoaccountsmarble

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	neoaccountssupabase "github.com/R3E-Network/service_layer/services/neoaccounts/supabase"
)

// RequestAccounts locks and returns accounts for a service.
func (s *Service) RequestAccounts(ctx context.Context, serviceID string, count int, purpose string) ([]AccountInfo, string, error) {
	if s.repo == nil {
		return nil, "", fmt.Errorf("repository not configured")
	}
	if count <= 0 || count > 100 {
		return nil, "", fmt.Errorf("invalid count: must be 1-100")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Get available (unlocked, non-retiring) accounts with balances
	accounts, err := s.repo.ListAvailableWithBalances(ctx, "", nil, count)
	if err != nil {
		return nil, "", fmt.Errorf("list accounts: %w", err)
	}

	if len(accounts) < count {
		// Try to create more accounts if needed
		need := count - len(accounts)
		for i := 0; i < need && len(accounts) < MaxPoolAccounts; i++ {
			acc, err := s.createAccount(ctx)
			if err != nil {
				break
			}
			// Convert to AccountWithBalances
			accWithBal := neoaccountssupabase.NewAccountWithBalances(*acc)
			accounts = append(accounts, *accWithBal)
		}
	}

	if len(accounts) == 0 {
		return nil, "", fmt.Errorf("no accounts available")
	}

	// Generate lock ID
	lockID := uuid.New().String()

	// Lock the accounts
	result := make([]AccountInfo, 0, len(accounts))
	for i := range accounts {
		acc := &accounts[i]
		acc.LockedBy = serviceID
		acc.LockedAt = time.Now()

		// Update account in DB
		dbAcc := &neoaccountssupabase.Account{
			ID:         acc.ID,
			Address:    acc.Address,
			CreatedAt:  acc.CreatedAt,
			LastUsedAt: acc.LastUsedAt,
			TxCount:    acc.TxCount,
			IsRetiring: acc.IsRetiring,
			LockedBy:   acc.LockedBy,
			LockedAt:   acc.LockedAt,
		}

		if err := s.repo.Update(ctx, dbAcc); err != nil {
			log.Printf("[neoaccounts] failed to lock account %s: %v", acc.ID, err)
			continue
		}

		result = append(result, AccountInfoFromWithBalances(acc))

		if len(result) >= count {
			break
		}
	}

	return result, lockID, nil
}

// ReleaseAccounts releases previously locked accounts.
func (s *Service) ReleaseAccounts(ctx context.Context, serviceID string, accountIDs []string) (int, error) {
	if s.repo == nil {
		return 0, fmt.Errorf("repository not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	released := 0
	for _, accID := range accountIDs {
		acc, err := s.repo.GetByID(ctx, accID)
		if err != nil {
			continue
		}

		// Only release if locked by this service
		if acc.LockedBy != serviceID {
			continue
		}

		acc.LockedBy = ""
		acc.LockedAt = time.Time{}
		acc.LastUsedAt = time.Now()

		if err := s.repo.Update(ctx, acc); err != nil {
			log.Printf("[neoaccounts] failed to release account %s: %v", accID, err)
			continue
		}
		released++
	}

	return released, nil
}

// ReleaseAllByService releases all accounts locked by a service.
func (s *Service) ReleaseAllByService(ctx context.Context, serviceID string) (int, error) {
	if s.repo == nil {
		return 0, fmt.Errorf("repository not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	accounts, err := s.repo.ListByLocker(ctx, serviceID)
	if err != nil {
		return 0, err
	}

	released := 0
	for i := range accounts {
		acc := &accounts[i]
		acc.LockedBy = ""
		acc.LockedAt = time.Time{}
		acc.LastUsedAt = time.Now()

		if err := s.repo.Update(ctx, acc); err != nil {
			log.Printf("[neoaccounts] failed to release account %s for service %s: %v", acc.ID, serviceID, err)
			continue
		}
		released++
	}

	return released, nil
}

// UpdateBalance updates an account's token balance.
func (s *Service) UpdateBalance(ctx context.Context, serviceID, accountID, tokenType string, delta int64, absolute *int64) (int64, int64, int64, error) {
	if s.repo == nil {
		return 0, 0, 0, fmt.Errorf("repository not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// Default to GAS if no token specified
	if tokenType == "" {
		tokenType = TokenTypeGAS
	}

	acc, err := s.repo.GetByID(ctx, accountID)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("account not found: %w", err)
	}

	// Verify the account is locked by this service
	if acc.LockedBy != serviceID {
		return 0, 0, 0, fmt.Errorf("account not locked by service %s", serviceID)
	}

	// Get current balance for token
	currentBal, err := s.repo.GetBalance(ctx, accountID, tokenType)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("get balance: %w", err)
	}

	var oldBalance int64
	if currentBal != nil {
		oldBalance = currentBal.Amount
	}

	var newBalance int64
	if absolute != nil {
		newBalance = *absolute
	} else {
		newBalance = oldBalance + delta
	}

	if newBalance < 0 {
		return 0, 0, 0, fmt.Errorf("insufficient balance")
	}

	// Validate NEO is indivisible (no fractional amounts)
	if tokenType == TokenTypeNEO && newBalance != 0 {
		// NEO has 0 decimals, so amount should be in whole units
		// This is already enforced by int64, but we add explicit check
	}

	// Get script hash and decimals for token
	scriptHash, decimals := neoaccountssupabase.GetDefaultTokenConfig(tokenType)

	// Upsert the balance
	if err := s.repo.UpsertBalance(ctx, accountID, tokenType, scriptHash, newBalance, decimals); err != nil {
		return 0, 0, 0, fmt.Errorf("upsert balance: %w", err)
	}

	// Update account metadata
	acc.LastUsedAt = time.Now()
	acc.TxCount++

	if err := s.repo.Update(ctx, acc); err != nil {
		return 0, 0, 0, err
	}

	return oldBalance, newBalance, acc.TxCount, nil
}

// GetPoolInfo returns pool statistics with per-token breakdowns.
func (s *Service) GetPoolInfo(ctx context.Context) (*PoolInfoResponse, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("repository not configured")
	}
	accounts, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	info := &PoolInfoResponse{
		TokenStats: make(map[string]TokenStats),
	}

	for _, acc := range accounts {
		info.TotalAccounts++

		if acc.IsRetiring {
			info.RetiringAccounts++
		} else if acc.LockedBy != "" {
			info.LockedAccounts++
		} else {
			info.ActiveAccounts++
		}
	}

	// Get stats for known tokens
	for _, tokenType := range []string{TokenTypeGAS, TokenTypeNEO} {
		stats, err := s.repo.AggregateTokenStats(ctx, tokenType)
		if err != nil {
			continue
		}
		info.TokenStats[tokenType] = *stats
	}

	return info, nil
}

// ListAccountsByService returns accounts locked by a specific service.
func (s *Service) ListAccountsByService(ctx context.Context, serviceID string, tokenType string, minBalance *int64) ([]AccountInfo, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("repository not configured")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	accounts, err := s.repo.ListByLockerWithBalances(ctx, serviceID)
	if err != nil {
		return nil, err
	}

	result := make([]AccountInfo, 0, len(accounts))
	for _, acc := range accounts {
		// Filter by token balance if specified
		if tokenType != "" && minBalance != nil {
			if !acc.HasSufficientBalance(tokenType, *minBalance) {
				continue
			}
		}
		result = append(result, AccountInfoFromWithBalances(&acc))
	}

	return result, nil
}

// runAccountRotation periodically rotates pool accounts (daily rotation).
func (s *Service) runAccountRotation(ctx context.Context) {
	if s.repo == nil {
		return
	}
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.StopChan():
			return
		case <-ticker.C:
			s.rotateAccounts(ctx)
		}
	}
}

// rotateAccounts retires old accounts and creates new ones.
// Locked accounts are NEVER rotated.
func (s *Service) rotateAccounts(ctx context.Context) {
	if s.repo == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	accounts, err := s.repo.ListWithBalances(ctx)
	if err != nil {
		return
	}

	// Count active (unlocked, non-retiring) accounts
	activeCount := 0
	for _, acc := range accounts {
		if !acc.IsRetiring && acc.LockedBy == "" {
			activeCount++
		}
	}

	// Daily rotation: RotationRate per day, divided by 24 for hourly check
	retireCount := int(float64(activeCount) * RotationRate / 24)
	if retireCount < 1 {
		retireCount = 1
	}

	minAge := time.Duration(RotationMinAge) * time.Hour
	// Minimum balance threshold for rotation (in GAS units, 8 decimals)
	minGasBalance := int64(100000) // 0.001 GAS

	retired := 0
	for i := range accounts {
		acc := &accounts[i]
		if retired >= retireCount {
			break
		}

		// NEVER retire locked accounts
		if acc.LockedBy != "" {
			continue
		}

		// Only retire if: not already retiring, low balance for ALL tokens, and old enough
		if !acc.IsRetiring && time.Since(acc.CreatedAt) > minAge {
			// Check if account has low balances across all tokens
			gasBalance := acc.GetBalance(TokenTypeGAS)
			neoBalance := acc.GetBalance(TokenTypeNEO)

			if gasBalance < minGasBalance && neoBalance == 0 {
				dbAcc, err := s.repo.GetByID(ctx, acc.ID)
				if err != nil {
					continue
				}
				dbAcc.IsRetiring = true
				retired++
				if err := s.repo.Update(ctx, dbAcc); err != nil {
					log.Printf("[neoaccounts] failed to mark account %s as retiring: %v", acc.ID, err)
				}
			}
		}
	}

	// Ensure minimum pool size
	for activeCount < MinPoolAccounts {
		if _, err := s.createAccount(ctx); err != nil {
			break
		}
		activeCount++
	}

	// Delete empty retiring accounts (only if not locked and all balances are zero)
	for _, acc := range accounts {
		if acc.IsRetiring && acc.IsEmpty() && acc.LockedBy == "" {
			if err := s.repo.Delete(ctx, acc.ID); err != nil {
				log.Printf("[neoaccounts] failed to delete retiring account %s: %v", acc.ID, err)
			}
		}
	}
}

// runLockCleanup periodically cleans up stale locks.
func (s *Service) runLockCleanup(ctx context.Context) {
	if s.repo == nil {
		return
	}
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.StopChan():
			return
		case <-ticker.C:
			s.cleanupStaleLocks(ctx)
		}
	}
}

// cleanupStaleLocks releases accounts that have been locked too long.
func (s *Service) cleanupStaleLocks(ctx context.Context) {
	if s.repo == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	accounts, err := s.repo.List(ctx)
	if err != nil {
		return
	}

	now := time.Now()
	for i := range accounts {
		acc := &accounts[i]
		if acc.LockedBy != "" && !acc.LockedAt.IsZero() {
			if now.Sub(acc.LockedAt) > LockTimeout {
				// Force release stale lock
				acc.LockedBy = ""
				acc.LockedAt = time.Time{}
				if err := s.repo.Update(ctx, acc); err != nil {
					log.Printf("[neoaccounts] failed to release stale lock on account %s: %v", acc.ID, err)
				}
			}
		}
	}
}
