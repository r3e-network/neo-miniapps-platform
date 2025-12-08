// Package mixer provides pool account management via accountpool service.
package mixermarble

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	accountpool "github.com/R3E-Network/service_layer/services/accountpool/marble"
)

// AccountPoolClient wraps HTTP calls to the accountpool service.
type AccountPoolClient struct {
	baseURL    string
	serviceID  string
	httpClient *http.Client
}

// NewAccountPoolClient creates a new accountpool client.
func NewAccountPoolClient(baseURL, serviceID string) *AccountPoolClient {
	return &AccountPoolClient{
		baseURL:    baseURL,
		serviceID:  serviceID,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// WithHTTPClient overrides the HTTP client (e.g., to use Marble mTLS).
func (c *AccountPoolClient) WithHTTPClient(client *http.Client) *AccountPoolClient {
	if client != nil {
		c.httpClient = client
	}
	return c
}

// Types AccountInfo, RequestAccountsResponse, PoolInfoResponse, ListAccountsResponse
// are imported from github.com/R3E-Network/service_layer/services/accountpool

// RequestAccounts requests and locks accounts from the pool.
func (c *AccountPoolClient) RequestAccounts(ctx context.Context, count int, purpose string) (*accountpool.RequestAccountsResponse, error) {
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
			return nil, fmt.Errorf("accountpool error %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
		return nil, fmt.Errorf("accountpool error %d: %s", resp.StatusCode, string(respBody))
	}

	var result accountpool.RequestAccountsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ReleaseAccounts releases accounts back to the pool.
func (c *AccountPoolClient) ReleaseAccounts(ctx context.Context, accountIDs []string) error {
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
			return fmt.Errorf("accountpool error %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
		return fmt.Errorf("accountpool error %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// UpdateBalance updates an account's balance.
func (c *AccountPoolClient) UpdateBalance(ctx context.Context, accountID string, delta int64, absolute *int64) error {
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
			return fmt.Errorf("accountpool error %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
		return fmt.Errorf("accountpool error %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// GetPoolInfo returns pool statistics.
func (c *AccountPoolClient) GetPoolInfo(ctx context.Context) (*accountpool.PoolInfoResponse, error) {
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
		return nil, fmt.Errorf("accountpool error %d: %s", resp.StatusCode, string(respBody))
	}

	var result accountpool.PoolInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetLockedAccounts returns accounts locked by this service with optional balance filter.
func (c *AccountPoolClient) GetLockedAccounts(ctx context.Context, minBalance *int64) ([]accountpool.AccountInfo, error) {
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
		return nil, fmt.Errorf("accountpool error %d: %s", resp.StatusCode, string(respBody))
	}

	var result accountpool.ListAccountsResponse
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

// SignTransactionResult from accountpool service.
type SignTransactionResult struct {
	AccountID string `json:"account_id"`
	Signature string `json:"signature"`  // base64 encoded
	PublicKey string `json:"public_key"` // base64 encoded
}

// SignTransaction signs a transaction hash with an account's private key.
func (c *AccountPoolClient) SignTransaction(ctx context.Context, accountID string, txHash []byte) (*SignTransactionResult, error) {
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
			return nil, fmt.Errorf("accountpool error %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
		return nil, fmt.Errorf("accountpool error %d: %s", resp.StatusCode, string(respBody))
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

// TransferResult from accountpool service.
type TransferResult struct {
	TxHash    string `json:"tx_hash"`
	AccountID string `json:"account_id"`
	Amount    int64  `json:"amount"`
}

// Transfer transfers tokens from a pool account to an external address.
// This constructs, signs, and broadcasts the transaction via accountpool.
func (c *AccountPoolClient) Transfer(ctx context.Context, accountID, toAddress string, amount int64, tokenHash string) (*TransferResult, error) {
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
			return nil, fmt.Errorf("accountpool error %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
		return nil, fmt.Errorf("accountpool error %d: %s", resp.StatusCode, string(respBody))
	}

	var result TransferResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// =============================================================================
// Service Pool Methods (using accountpool client)
// =============================================================================

// getAccountPoolClient returns the accountpool client.
func (s *Service) getAccountPoolClient() *AccountPoolClient {
	client := NewAccountPoolClient(s.accountPoolURL, ServiceID)

	// Prefer the Marble-provided mTLS client for cross-marble traffic.
	if s.Marble() != nil {
		if hc := s.Marble().HTTPClient(); hc != nil {
			// Preserve existing timeout semantics if not set.
			if hc.Timeout == 0 {
				hc.Timeout = 30 * time.Second
			}
			client = client.WithHTTPClient(hc)
		}
	}
	return client
}

// createPoolAccount requests a single account from accountpool for deposit address.
func (s *Service) createPoolAccount(ctx context.Context) (*PoolAccount, error) {
	client := s.getAccountPoolClient()
	resp, err := client.RequestAccounts(ctx, 1, "mixer-deposit")
	if err != nil {
		return nil, err
	}
	if len(resp.Accounts) == 0 {
		return nil, fmt.Errorf("no accounts available")
	}

	acc := resp.Accounts[0]
	return &PoolAccount{
		ID:         acc.ID,
		Address:    acc.Address,
		Balance:    acc.Balance,
		CreatedAt:  acc.CreatedAt,
		LastUsedAt: acc.LastUsedAt,
		TxCount:    acc.TxCount,
		IsRetiring: acc.IsRetiring,
	}, nil
}

// getAvailableAccounts requests accounts from accountpool for mixing.
func (s *Service) getAvailableAccounts(ctx context.Context, count int) ([]*PoolAccount, error) {
	client := s.getAccountPoolClient()
	resp, err := client.RequestAccounts(ctx, count, "mixer-mixing")
	if err != nil {
		return nil, err
	}

	accounts := make([]*PoolAccount, 0, len(resp.Accounts))
	for _, acc := range resp.Accounts {
		accounts = append(accounts, &PoolAccount{
			ID:         acc.ID,
			Address:    acc.Address,
			Balance:    acc.Balance,
			CreatedAt:  acc.CreatedAt,
			LastUsedAt: acc.LastUsedAt,
			TxCount:    acc.TxCount,
			IsRetiring: acc.IsRetiring,
		})
	}
	return accounts, nil
}

// getActiveAccounts returns accounts with balance for mixing transactions.
// Uses accountpool service to get locked accounts with balance > 0.
func (s *Service) getActiveAccounts(ctx context.Context) ([]*PoolAccount, error) {
	client := s.getAccountPoolClient()
	minBalance := int64(1)
	accounts, err := client.GetLockedAccounts(ctx, &minBalance)
	if err != nil {
		return nil, err
	}

	active := make([]*PoolAccount, 0, len(accounts))
	for _, acc := range accounts {
		active = append(active, &PoolAccount{
			ID:         acc.ID,
			Address:    acc.Address,
			Balance:    acc.Balance,
			CreatedAt:  acc.CreatedAt,
			LastUsedAt: acc.LastUsedAt,
			TxCount:    acc.TxCount,
			IsRetiring: acc.IsRetiring,
		})
	}
	return active, nil
}

// updateAccountBalance updates an account's balance via accountpool service.
func (s *Service) updateAccountBalance(ctx context.Context, accountID string, delta int64) error {
	client := s.getAccountPoolClient()
	return client.UpdateBalance(ctx, accountID, delta, nil)
}

// releasePoolAccounts releases accounts back to accountpool when mixing is done.
func (s *Service) releasePoolAccounts(ctx context.Context, accountIDs []string) error {
	if len(accountIDs) == 0 {
		return nil
	}
	client := s.getAccountPoolClient()
	return client.ReleaseAccounts(ctx, accountIDs)
}
