package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	slhttputil "github.com/R3E-Network/service_layer/internal/httputil"
	"github.com/R3E-Network/service_layer/internal/serviceauth"
)

// Client is a client for the NeoAccounts service.
type Client struct {
	baseURL      string
	httpClient   *http.Client
	serviceID    string
	maxBodyBytes int64
}

// Config holds client configuration.
type Config struct {
	BaseURL string
	// ServiceID identifies the caller. In strict identity mode this is redundant
	// (caller identity is enforced by MarbleRun mTLS), but it's still useful for
	// local development and debugging.
	ServiceID string
	Timeout   time.Duration
	// HTTPClient optionally overrides the client used to execute requests.
	// For MarbleRun mesh calls, prefer using `marble.Marble.HTTPClient()` so
	// requests are sent over verified mTLS.
	HTTPClient *http.Client
	// MaxBodyBytes caps responses to prevent memory exhaustion.
	MaxBodyBytes int64
}

const (
	defaultTimeout     = 30 * time.Second
	defaultMaxBodySize = 8 << 20 // 8MiB
)

// New creates a new NeoAccounts client.
func New(cfg Config) (*Client, error) {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	baseURL, _, err := slhttputil.NormalizeServiceBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("neoaccounts: %w", err)
	}

	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	} else {
		copied := *client
		if copied.Timeout == 0 || cfg.Timeout != 0 {
			copied.Timeout = timeout
		}
		client = &copied
	}

	maxBodyBytes := cfg.MaxBodyBytes
	if maxBodyBytes <= 0 {
		maxBodyBytes = defaultMaxBodySize
	}

	return &Client{
		baseURL:      baseURL,
		serviceID:    strings.TrimSpace(cfg.ServiceID),
		httpClient:   client,
		maxBodyBytes: maxBodyBytes,
	}, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, in any, out any) error {
	if c == nil {
		return fmt.Errorf("neoaccounts: client is nil")
	}
	if c.httpClient == nil {
		return fmt.Errorf("neoaccounts: http client not configured")
	}

	var bodyReader *bytes.Reader
	if in != nil {
		body, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("neoaccounts: marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(body)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	urlStr := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return fmt.Errorf("neoaccounts: create request: %w", err)
	}

	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.serviceID != "" {
		req.Header.Set(serviceauth.ServiceIDHeader, c.serviceID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("neoaccounts: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, truncated, readErr := slhttputil.ReadAllWithLimit(resp.Body, 32<<10)
		if readErr != nil {
			return fmt.Errorf("neoaccounts: request failed: %s (failed to read body: %v)", resp.Status, readErr)
		}
		msg := strings.TrimSpace(string(body))
		if truncated {
			msg += "...(truncated)"
		}
		if msg != "" {
			return fmt.Errorf("neoaccounts: request failed: %s - %s", resp.Status, msg)
		}
		return fmt.Errorf("neoaccounts: request failed: %s", resp.Status)
	}

	if out == nil {
		return nil
	}

	respBody, err := slhttputil.ReadAllStrict(resp.Body, c.maxBodyBytes)
	if err != nil {
		return fmt.Errorf("neoaccounts: read response: %w", err)
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("neoaccounts: unmarshal response: %w", err)
	}

	return nil
}

// GetPoolInfo returns pool statistics.
func (c *Client) GetPoolInfo(ctx context.Context) (*PoolInfoResponse, error) {
	var out PoolInfoResponse
	if err := c.doJSON(ctx, http.MethodGet, "/pool-info", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RequestAccounts requests and locks accounts from the pool.
func (c *Client) RequestAccounts(ctx context.Context, count int, purpose string) (*RequestAccountsResponse, error) {
	if count <= 0 {
		count = 1
	}

	var out RequestAccountsResponse
	if err := c.doJSON(ctx, http.MethodPost, "/request", RequestAccountsInput{
		ServiceID: c.serviceID,
		Count:     count,
		Purpose:   purpose,
	}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ReleaseAccounts releases accounts back to the pool.
func (c *Client) ReleaseAccounts(ctx context.Context, accountIDs []string) (*ReleaseAccountsResponse, error) {
	var out ReleaseAccountsResponse
	if err := c.doJSON(ctx, http.MethodPost, "/release", ReleaseAccountsInput{
		ServiceID:  c.serviceID,
		AccountIDs: accountIDs,
	}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateBalance updates an account's balance.
func (c *Client) UpdateBalance(ctx context.Context, accountID string, token string, delta int64, absolute *int64) (*UpdateBalanceResponse, error) {
	var out UpdateBalanceResponse
	if err := c.doJSON(ctx, http.MethodPost, "/balance", UpdateBalanceInput{
		ServiceID: c.serviceID,
		AccountID: accountID,
		Token:     token,
		Delta:     delta,
		Absolute:  absolute,
	}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetLockedAccounts returns accounts locked by this service with optional token and balance filters.
func (c *Client) GetLockedAccounts(ctx context.Context, tokenType string, minBalance *int64) ([]AccountInfo, error) {
	q := url.Values{}
	if c.serviceID != "" {
		q.Set("service_id", c.serviceID)
	}
	if tokenType != "" {
		q.Set("token", tokenType)
	}
	if minBalance != nil {
		q.Set("min_balance", strconv.FormatInt(*minBalance, 10))
	}

	path := "/accounts"
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}

	var out ListAccountsResponse
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out.Accounts, nil
}

// SignTransaction signs a transaction hash with an account's private key.
func (c *Client) SignTransaction(ctx context.Context, accountID string, txHash []byte) (*SignTransactionResponse, error) {
	var out SignTransactionResponse
	if err := c.doJSON(ctx, http.MethodPost, "/sign", SignTransactionInput{
		ServiceID: c.serviceID,
		AccountID: accountID,
		TxHash:    txHash,
	}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// BatchSign signs multiple transaction hashes.
func (c *Client) BatchSign(ctx context.Context, requests []SignRequest) (*BatchSignResponse, error) {
	var out BatchSignResponse
	if err := c.doJSON(ctx, http.MethodPost, "/batch-sign", BatchSignInput{
		ServiceID: c.serviceID,
		Requests:  requests,
	}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Transfer transfers tokens from a pool account to an external address.
func (c *Client) Transfer(ctx context.Context, accountID, toAddress string, amount int64, tokenHash string) (*TransferResponse, error) {
	var out TransferResponse
	if err := c.doJSON(ctx, http.MethodPost, "/transfer", TransferInput{
		ServiceID: c.serviceID,
		AccountID: accountID,
		ToAddress: toAddress,
		Amount:    amount,
		TokenHash: tokenHash,
	}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetMasterKeyAttestation fetches the publicly cacheable master key attestation bundle.
func (c *Client) GetMasterKeyAttestation(ctx context.Context) (*MasterKeyAttestation, error) {
	var out MasterKeyAttestation
	if err := c.doJSON(ctx, http.MethodGet, "/master-key", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
