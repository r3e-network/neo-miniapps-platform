// Package neovault provides pool account management via neoaccounts service.
package neovaultmarble

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	neoaccounts "github.com/R3E-Network/service_layer/services/neoaccounts/marble"
)

// NeoAccountsClient wraps HTTP calls to the neoaccounts service.
type NeoAccountsClient struct {
	baseURL    string
	serviceID  string
	httpClient *http.Client
}

// NewNeoAccountsClient creates a new neoaccounts client.
func NewNeoAccountsClient(baseURL, serviceID string) *NeoAccountsClient {
	return &NeoAccountsClient{
		baseURL:    baseURL,
		serviceID:  serviceID,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// WithHTTPClient overrides the HTTP client (e.g., to use Marble mTLS).
func (c *NeoAccountsClient) WithHTTPClient(client *http.Client) *NeoAccountsClient {
	if client != nil {
		c.httpClient = client
	}
	return c
}

// Types AccountInfo, RequestAccountsResponse, PoolInfoResponse, ListAccountsResponse
// are imported from github.com/R3E-Network/service_layer/services/neoaccounts

// RequestAccounts requests and locks accounts from the pool.
func (c *NeoAccountsClient) RequestAccounts(ctx context.Context, count int, purpose string) (*neoaccounts.RequestAccountsResponse, error) {
	body := map[string]interface{}{
		"service_id": c.serviceID,
		"count":      count,
		"purpose":    purpose,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/request", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("neoaccounts error %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
		return nil, fmt.Errorf("neoaccounts error %d: %s", resp.StatusCode, string(respBody))
	}

	var result neoaccounts.RequestAccountsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ReleaseAccounts releases accounts back to the pool.
func (c *NeoAccountsClient) ReleaseAccounts(ctx context.Context, accountIDs []string) error {
	body := map[string]interface{}{
		"service_id":  c.serviceID,
		"account_ids": accountIDs,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/release", bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("neoaccounts error %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
		return fmt.Errorf("neoaccounts error %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// UpdateBalance updates an account's balance.
func (c *NeoAccountsClient) UpdateBalance(ctx context.Context, accountID string, delta int64, absolute *int64) error {
	body := map[string]interface{}{
		"service_id": c.serviceID,
		"account_id": accountID,
		"delta":      delta,
	}
	if absolute != nil {
		body["absolute"] = *absolute
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/balance", bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("neoaccounts error %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
		return fmt.Errorf("neoaccounts error %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// GetPoolInfo returns pool statistics.
func (c *NeoAccountsClient) GetPoolInfo(ctx context.Context) (*neoaccounts.PoolInfoResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/info", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("neoaccounts error %d: %s", resp.StatusCode, string(respBody))
	}

	var result neoaccounts.PoolInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetLockedAccounts returns accounts locked by this service with optional balance filter.
func (c *NeoAccountsClient) GetLockedAccounts(ctx context.Context, minBalance *int64) ([]neoaccounts.AccountInfo, error) {
	url := fmt.Sprintf("%s/accounts?service_id=%s", c.baseURL, c.serviceID)
	if minBalance != nil {
		url = fmt.Sprintf("%s&min_balance=%d", url, *minBalance)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("neoaccounts error %d: %s", resp.StatusCode, string(respBody))
	}

	var result neoaccounts.ListAccountsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Accounts, nil
}

// SignTransactionRequest for signing a transaction hash.
type SignTransactionRequest struct {
	ServiceID string `json:"service_id"`
	AccountID string `json:"account_id"`
	TxHash    string `json:"tx_hash"` // base64 encoded
}

// SignTransactionResult from neoaccounts service.
type SignTransactionResult struct {
	AccountID string `json:"account_id"`
	Signature string `json:"signature"`  // base64 encoded
	PublicKey string `json:"public_key"` // base64 encoded
}

// SignTransaction signs a transaction hash with an account's private key.
func (c *NeoAccountsClient) SignTransaction(ctx context.Context, accountID string, txHash []byte) (*SignTransactionResult, error) {
	body := SignTransactionRequest{
		ServiceID: c.serviceID,
		AccountID: accountID,
		TxHash:    base64.StdEncoding.EncodeToString(txHash),
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/sign", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("neoaccounts error %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
		return nil, fmt.Errorf("neoaccounts error %d: %s", resp.StatusCode, string(respBody))
	}

	var result SignTransactionResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// TransferRequest for transferring tokens from a pool account.
type TransferRequest struct {
	ServiceID   string `json:"service_id"`
	AccountID   string `json:"account_id"`
	ToAddress   string `json:"to_address"`
	Amount      int64  `json:"amount"`
	TokenHash   string `json:"token_hash,omitempty"` // defaults to GAS
}

// TransferResult from neoaccounts service.
type TransferResult struct {
	TxHash    string `json:"tx_hash"`
	AccountID string `json:"account_id"`
	Amount    int64  `json:"amount"`
}

// Transfer transfers tokens from a pool account to an external address.
// This constructs, signs, and broadcasts the transaction via neoaccounts.
func (c *NeoAccountsClient) Transfer(ctx context.Context, accountID, toAddress string, amount int64, tokenHash string) (*TransferResult, error) {
	body := TransferRequest{
		ServiceID: c.serviceID,
		AccountID: accountID,
		ToAddress: toAddress,
		Amount:    amount,
		TokenHash: tokenHash,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/transfer", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("neoaccounts error %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
		return nil, fmt.Errorf("neoaccounts error %d: %s", resp.StatusCode, string(respBody))
	}

	var result TransferResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// =============================================================================
// Service Pool Methods (using neoaccounts client)
// =============================================================================

// getNeoAccountsClient returns the neoaccounts client.
// SECURITY: Inter-service calls should use Marble mTLS client for authentication.
func (s *Service) getNeoAccountsClient() *NeoAccountsClient {
	client := NewNeoAccountsClient(s.neoAccountsURL, ServiceID)

	// Prefer the Marble-provided mTLS client for cross-marble traffic.
	if s.Marble() != nil {
		if hc := s.Marble().HTTPClient(); hc != nil {
			// Preserve existing timeout semantics if not set.
			if hc.Timeout == 0 {
				hc.Timeout = 30 * time.Second
			}
			client = client.WithHTTPClient(hc)
		} else {
			log.Printf("[neovault] WARNING: Marble mTLS client not available, using plain HTTP for neoaccounts calls")
		}
	} else {
		log.Printf("[neovault] WARNING: Marble not initialized, using plain HTTP for neoaccounts calls (insecure in production)")
	}
	return client
}

// createPoolAccount requests a single account from neoaccounts for deposit address.
func (s *Service) createPoolAccount(ctx context.Context) (*PoolAccount, error) {
	client := s.getNeoAccountsClient()
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
	client := s.getNeoAccountsClient()
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
// Uses neoaccounts service to get locked accounts with balance > 0.
func (s *Service) getActiveAccounts(ctx context.Context) ([]*PoolAccount, error) {
	client := s.getNeoAccountsClient()
	minBalance := int64(1)
	accounts, err := client.GetLockedAccounts(ctx, &minBalance)
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
func accountInfoToPoolAccount(acc *neoaccounts.AccountInfo) *PoolAccount {
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
func (s *Service) updateAccountBalance(ctx context.Context, accountID string, delta int64) error {
	client := s.getNeoAccountsClient()
	return client.UpdateBalance(ctx, accountID, delta, nil)
}

// releasePoolAccounts releases accounts back to neoaccounts when mixing is done.
func (s *Service) releasePoolAccounts(ctx context.Context, accountIDs []string) error {
	if len(accountIDs) == 0 {
		return nil
	}
	client := s.getNeoAccountsClient()
	return client.ReleaseAccounts(ctx, accountIDs)
}
