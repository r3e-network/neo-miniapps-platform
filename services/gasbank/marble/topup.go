// Package neogasbank provides GasBank service for managing user gas balances.
package neogasbank

import (
	"context"
	"os"
	"strings"
	"time"

	neoaccountsclient "github.com/R3E-Network/service_layer/infrastructure/accountpool/client"
)

const (
	// TopUpThreshold is the minimum GAS balance (in 8 decimals) before top-up
	// 0.1 GAS = 10000000 (10^7)
	TopUpThreshold = 10000000

	// TopUpTargetAmount is the target GAS balance after top-up (in 8 decimals)
	// 1 GAS = 100000000 (10^8)
	TopUpTargetAmount = 100000000

	// TopUpCheckInterval is how often to check for low-balance accounts
	TopUpCheckInterval = 5 * time.Minute

	// TopUpBatchSize is the maximum number of accounts to process per run
	TopUpBatchSize = 100
)

// processAutoTopUp checks pool accounts and tops up those with low GAS balance.
// This worker runs periodically to ensure pool accounts always have sufficient GAS.
func (s *Service) processAutoTopUp(ctx context.Context) {
	if s.chainClient == nil {
		s.Logger().WithContext(ctx).Debug("chain client not configured, skipping auto top-up")
		return
	}

	// Check if auto top-up is enabled
	if !s.isAutoTopUpEnabled() {
		s.Logger().WithContext(ctx).Debug("auto top-up is disabled")
		return
	}

	// Get account pool client
	poolClient, err := s.getAccountPoolClient()
	if err != nil {
		s.Logger().WithContext(ctx).WithError(err).Warn("failed to get account pool client")
		return
	}

	// Query accounts with low GAS balance using the new ListLowBalanceAccounts method
	accounts, err := poolClient.ListLowBalanceAccounts(ctx, "GAS", TopUpThreshold, TopUpBatchSize)
	if err != nil {
		s.Logger().WithContext(ctx).WithError(err).Warn("failed to query low-balance accounts")
		return
	}

	if len(accounts) == 0 {
		s.Logger().WithContext(ctx).Debug("no accounts need top-up")
		return
	}

	s.Logger().WithContext(ctx).WithField("count", len(accounts)).Info("found accounts needing top-up")

	// Process accounts in batches
	processed := 0
	succeeded := 0
	failed := 0

	for i := 0; i < len(accounts) && processed < TopUpBatchSize; i++ {
		acc := &accounts[i]
		processed++

		// Get current GAS balance
		currentBalance := int64(0)
		if gasBalance, ok := acc.Balances["GAS"]; ok {
			currentBalance = gasBalance.Amount
		}

		// Skip if balance is already above threshold (double-check)
		if currentBalance >= TopUpThreshold {
			continue
		}

		// Calculate top-up amount
		topUpAmount := TopUpTargetAmount - currentBalance
		if topUpAmount <= 0 {
			continue
		}

		// Perform top-up transfer from master wallet
		txHash, err := s.topUpAccount(ctx, acc.Address, topUpAmount)
		if err != nil {
			s.Logger().WithContext(ctx).
				WithError(err).
				WithFields(map[string]interface{}{
					"account_id":      acc.ID,
					"account_address": acc.Address,
					"current_balance": currentBalance,
					"topup_amount":    topUpAmount,
				}).
				Warn("failed to top up account")
			failed++
			continue
		}

		s.Logger().WithContext(ctx).
			WithFields(map[string]interface{}{
				"account_id":      acc.ID,
				"account_address": acc.Address,
				"current_balance": currentBalance,
				"topup_amount":    topUpAmount,
				"new_balance":     TopUpTargetAmount,
				"tx_hash":         txHash,
			}).
			Info("successfully topped up account")
		succeeded++
	}

	s.Logger().WithContext(ctx).
		WithFields(map[string]interface{}{
			"processed": processed,
			"succeeded": succeeded,
			"failed":    failed,
		}).
		Info("auto top-up batch completed")
}

// topUpAccount transfers GAS from the master wallet to a pool account.
// Returns the transaction hash on success.
func (s *Service) topUpAccount(ctx context.Context, toAddress string, amount int64) (string, error) {
	// TODO: Implement actual GAS transfer from master wallet
	// This requires:
	// 1. Load master wallet credentials (WIF or private key) from environment
	// 2. Build NEP-17 transfer transaction (GAS contract)
	// 3. Sign transaction with master wallet
	// 4. Broadcast transaction
	// 5. Wait for confirmation
	//
	// For now, return a simulated transaction hash for testing
	s.Logger().WithContext(ctx).WithFields(map[string]interface{}{
		"to_address": toAddress,
		"amount":     amount,
	}).Info("simulated top-up transfer (master wallet integration pending)")

	return "0x0000000000000000000000000000000000000000000000000000000000000000", nil
}

// getAccountPoolClient returns a client for the account pool service.
func (s *Service) getAccountPoolClient() (*neoaccountsclient.Client, error) {
	// Get account pool service URL from environment or use default
	poolURL := strings.TrimSpace(os.Getenv("NEOACCOUNTS_SERVICE_URL"))
	if poolURL == "" {
		poolURL = "http://neoaccounts:8080" // Default service mesh URL
	}

	return neoaccountsclient.New(neoaccountsclient.Config{
		BaseURL:   poolURL,
		ServiceID: ServiceID,
	})
}

// isAutoTopUpEnabled checks if auto top-up is enabled via environment variable.
func (s *Service) isAutoTopUpEnabled() bool {
	enabled := strings.TrimSpace(os.Getenv("TOPUP_ENABLED"))
	return enabled == "true" || enabled == "1"
}
