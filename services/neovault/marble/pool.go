// Package neovault provides pool account management via neoaccounts service.
package neovaultmarble

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/runtime"
	neoaccountsclient "github.com/R3E-Network/service_layer/services/neoaccounts/client"
)

const neoAccountsTimeout = 30 * time.Second

// getNeoAccountsClient returns the shared neoaccounts client.
// SECURITY: Inter-service calls should use Marble mTLS client for authentication.
func (s *Service) getNeoAccountsClient() (*neoaccountsclient.Client, error) {
	cfg := neoaccountsclient.Config{
		BaseURL:   s.neoAccountsURL,
		ServiceID: ServiceID,
		Timeout:   neoAccountsTimeout,
	}

	if s.Marble() != nil {
		if hc := s.Marble().HTTPClient(); hc != nil {
			cfg.HTTPClient = hc
			return neoaccountsclient.New(cfg)
		}

		// In strict identity mode, never fall back to plain HTTP because it allows
		// spoofing cross-service identity headers.
		if runtime.StrictIdentityMode() {
			return nil, fmt.Errorf("neoaccounts client requires Marble mTLS in strict identity mode")
		}
		s.Logger().WithField("neoaccounts_url", s.neoAccountsURL).Warn("Marble mTLS client not available, using plain HTTP for neoaccounts calls")
		return neoaccountsclient.New(cfg)
	}

	if runtime.StrictIdentityMode() {
		return nil, fmt.Errorf("neoaccounts client requires Marble initialization in strict identity mode")
	}

	s.Logger().WithField("neoaccounts_url", s.neoAccountsURL).Warn("Marble not initialized, using plain HTTP for neoaccounts calls (insecure without mTLS)")
	return neoaccountsclient.New(cfg)
}

// createPoolAccount requests a single account from neoaccounts for deposit address.
func (s *Service) createPoolAccount(ctx context.Context) (*PoolAccount, error) {
	client, err := s.getNeoAccountsClient()
	if err != nil {
		return nil, err
	}
	resp, err := client.RequestAccounts(ctx, 1, "neovault-deposit")
	if err != nil {
		return nil, err
	}
	if len(resp.Accounts) == 0 {
		return nil, fmt.Errorf("no accounts available")
	}

	return accountInfoToPoolAccount(&resp.Accounts[0]), nil
}

// getAvailableAccounts requests accounts from neoaccounts for mixing.
func (s *Service) getAvailableAccounts(ctx context.Context, count int) ([]*PoolAccount, error) {
	client, err := s.getNeoAccountsClient()
	if err != nil {
		return nil, err
	}
	resp, err := client.RequestAccounts(ctx, count, "neovault-mixing")
	if err != nil {
		return nil, err
	}

	accounts := make([]*PoolAccount, 0, len(resp.Accounts))
	for i := range resp.Accounts {
		accounts = append(accounts, accountInfoToPoolAccount(&resp.Accounts[i]))
	}
	return accounts, nil
}

// getActiveAccounts returns accounts with balance for mixing transactions.
// Uses neoaccounts service to get locked accounts with optional balance filter.
func (s *Service) getActiveAccounts(ctx context.Context) ([]*PoolAccount, error) {
	client, err := s.getNeoAccountsClient()
	if err != nil {
		return nil, err
	}
	minBalance := int64(1)
	accounts, err := client.GetLockedAccounts(ctx, DefaultToken, &minBalance)
	if err != nil {
		return nil, err
	}

	active := make([]*PoolAccount, 0, len(accounts))
	for i := range accounts {
		active = append(active, accountInfoToPoolAccount(&accounts[i]))
	}
	return active, nil
}

// accountInfoToPoolAccount converts neoaccounts AccountInfo to NeoVault PoolAccount.
func accountInfoToPoolAccount(acc *neoaccountsclient.AccountInfo) *PoolAccount {
	return &PoolAccount{
		ID:         acc.ID,
		Address:    acc.Address,
		Balances:   acc.Balances,
		CreatedAt:  acc.CreatedAt,
		LastUsedAt: acc.LastUsedAt,
		TxCount:    acc.TxCount,
		IsRetiring: acc.IsRetiring,
	}
}

// updateAccountBalance updates an account's balance via neoaccounts service.
func (s *Service) updateAccountBalance(ctx context.Context, accountID string, tokenType string, delta int64) error {
	client, err := s.getNeoAccountsClient()
	if err != nil {
		return err
	}
	if strings.TrimSpace(tokenType) == "" {
		tokenType = DefaultToken
	}
	_, err = client.UpdateBalance(ctx, accountID, tokenType, delta, nil)
	return err
}

// releasePoolAccounts releases accounts back to neoaccounts when mixing is done.
func (s *Service) releasePoolAccounts(ctx context.Context, accountIDs []string) error {
	if len(accountIDs) == 0 {
		return nil
	}
	client, err := s.getNeoAccountsClient()
	if err != nil {
		return err
	}
	_, err = client.ReleaseAccounts(ctx, accountIDs)
	return err
}
