// Package accountpool provides pool management for the account pool service.
package accountpool

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RequestAccounts locks and returns accounts for a service.
func (s *Service) RequestAccounts(ctx context.Context, serviceID string, count int, purpose string) ([]AccountInfo, string, error) {
	if count <= 0 || count > 100 {
		return nil, "", fmt.Errorf("invalid count: must be 1-100")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Get available (unlocked, non-retiring) accounts
	accounts, err := s.DB().ListAvailablePoolAccounts(ctx, count)
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
			accounts = append(accounts, *acc)
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

		if err := s.DB().UpdatePoolAccount(ctx, acc); err != nil {
			continue
		}

		result = append(result, AccountInfo{
			ID:         acc.ID,
			Address:    acc.Address,
			Balance:    acc.Balance,
			CreatedAt:  acc.CreatedAt,
			LastUsedAt: acc.LastUsedAt,
			TxCount:    acc.TxCount,
			IsRetiring: acc.IsRetiring,
			LockedBy:   acc.LockedBy,
			LockedAt:   acc.LockedAt,
		})

		if len(result) >= count {
			break
		}
	}

	return result, lockID, nil
}

// ReleaseAccounts releases previously locked accounts.
func (s *Service) ReleaseAccounts(ctx context.Context, serviceID string, accountIDs []string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	released := 0
	for _, accID := range accountIDs {
		acc, err := s.DB().GetPoolAccount(ctx, accID)
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

		if err := s.DB().UpdatePoolAccount(ctx, acc); err != nil {
			continue
		}
		released++
	}

	return released, nil
}

// ReleaseAllByService releases all accounts locked by a service.
func (s *Service) ReleaseAllByService(ctx context.Context, serviceID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	accounts, err := s.DB().ListPoolAccountsByLocker(ctx, serviceID)
	if err != nil {
		return 0, err
	}

	released := 0
	for i := range accounts {
		acc := &accounts[i]
		acc.LockedBy = ""
		acc.LockedAt = time.Time{}
		acc.LastUsedAt = time.Now()

		if err := s.DB().UpdatePoolAccount(ctx, acc); err != nil {
			continue
		}
		released++
	}

	return released, nil
}

// UpdateBalance updates an account's balance.
func (s *Service) UpdateBalance(ctx context.Context, serviceID, accountID string, delta int64, absolute *int64) (int64, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	acc, err := s.DB().GetPoolAccount(ctx, accountID)
	if err != nil {
		return 0, 0, fmt.Errorf("account not found: %w", err)
	}

	// Verify the account is locked by this service
	if acc.LockedBy != serviceID {
		return 0, 0, fmt.Errorf("account not locked by service %s", serviceID)
	}

	oldBalance := acc.Balance

	if absolute != nil {
		acc.Balance = *absolute
	} else {
		acc.Balance += delta
	}

	if acc.Balance < 0 {
		return 0, 0, fmt.Errorf("insufficient balance")
	}

	acc.LastUsedAt = time.Now()
	acc.TxCount++

	if err := s.DB().UpdatePoolAccount(ctx, acc); err != nil {
		return 0, 0, err
	}

	return oldBalance, acc.Balance, nil
}

// GetPoolInfo returns pool statistics.
func (s *Service) GetPoolInfo(ctx context.Context) (*PoolInfoResponse, error) {
	accounts, err := s.DB().ListPoolAccounts(ctx)
	if err != nil {
		return nil, err
	}

	info := &PoolInfoResponse{}
	for _, acc := range accounts {
		info.TotalAccounts++
		info.TotalBalance += acc.Balance

		if acc.IsRetiring {
			info.RetiringAccounts++
		} else if acc.LockedBy != "" {
			info.LockedAccounts++
		} else {
			info.ActiveAccounts++
		}
	}

	return info, nil
}

// ListAccountsByService returns accounts locked by a specific service.
func (s *Service) ListAccountsByService(ctx context.Context, serviceID string, minBalance *int64) ([]AccountInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	accounts, err := s.DB().ListPoolAccountsByLocker(ctx, serviceID)
	if err != nil {
		return nil, err
	}

	result := make([]AccountInfo, 0, len(accounts))
	for _, acc := range accounts {
		if minBalance != nil && acc.Balance < *minBalance {
			continue
		}
		result = append(result, AccountInfo{
			ID:         acc.ID,
			Address:    acc.Address,
			Balance:    acc.Balance,
			CreatedAt:  acc.CreatedAt,
			LastUsedAt: acc.LastUsedAt,
			TxCount:    acc.TxCount,
			IsRetiring: acc.IsRetiring,
			LockedBy:   acc.LockedBy,
			LockedAt:   acc.LockedAt,
		})
	}

	return result, nil
}

// runAccountRotation periodically rotates pool accounts (daily rotation).
func (s *Service) runAccountRotation(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.rotateAccounts(ctx)
		}
	}
}

// rotateAccounts retires old accounts and creates new ones.
// Locked accounts are NEVER rotated.
func (s *Service) rotateAccounts(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	accounts, err := s.DB().ListPoolAccounts(ctx)
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
	minBalance := int64(100000) // Minimum balance threshold for rotation

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

		// Only retire if: not already retiring, low balance, and old enough
		if !acc.IsRetiring && acc.Balance < minBalance && time.Since(acc.CreatedAt) > minAge {
			acc.IsRetiring = true
			retired++
			_ = s.DB().UpdatePoolAccount(ctx, acc)
		}
	}

	// Ensure minimum pool size
	for activeCount < MinPoolAccounts {
		if _, err := s.createAccount(ctx); err != nil {
			break
		}
		activeCount++
	}

	// Delete empty retiring accounts (only if not locked)
	for _, acc := range accounts {
		if acc.IsRetiring && acc.Balance == 0 && acc.LockedBy == "" {
			_ = s.DB().DeletePoolAccount(ctx, acc.ID)
		}
	}
}

// runLockCleanup periodically cleans up stale locks.
func (s *Service) runLockCleanup(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.cleanupStaleLocks(ctx)
		}
	}
}

// cleanupStaleLocks releases accounts that have been locked too long.
func (s *Service) cleanupStaleLocks(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	accounts, err := s.DB().ListPoolAccounts(ctx)
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
				_ = s.DB().UpdatePoolAccount(ctx, acc)
			}
		}
	}
}
