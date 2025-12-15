// Package client provides a client for interacting with the TxSubmitter service.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a client for the TxSubmitter service.
type Client struct {
	baseURL    string
	httpClient *http.Client
	serviceID  string // Calling service identifier
}

// Config holds client configuration.
type Config struct {
	BaseURL   string
	ServiceID string
	Timeout   time.Duration
}

// New creates a new TxSubmitter client.
func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &Client{
		baseURL:   cfg.BaseURL,
		serviceID: cfg.ServiceID,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// =============================================================================
// Request/Response Types
// =============================================================================

// SubmitRequest is the request to submit a transaction.
type SubmitRequest struct {
	RequestID           string          `json:"request_id"`
	TxType              string          `json:"tx_type"`
	ContractAddress     string          `json:"contract_address,omitempty"`
	MethodName          string          `json:"method_name,omitempty"`
	Params              json.RawMessage `json:"params"`
	Priority            int             `json:"priority,omitempty"`
	WaitForConfirmation bool            `json:"wait_for_confirmation,omitempty"`
	Timeout             time.Duration   `json:"timeout,omitempty"`
}

// SubmitResponse is the response from submitting a transaction.
type SubmitResponse struct {
	ID          int64      `json:"id"`
	TxHash      string     `json:"tx_hash,omitempty"`
	Status      string     `json:"status"`
	GasConsumed int64      `json:"gas_consumed,omitempty"`
	Error       string     `json:"error,omitempty"`
	SubmittedAt time.Time  `json:"submitted_at"`
	ConfirmedAt *time.Time `json:"confirmed_at,omitempty"`
}

// FulfillParams are parameters for fulfill_request transactions.
type FulfillParams struct {
	RequestID string `json:"request_id"`
	Result    string `json:"result"` // hex-encoded
}

// SetMasterKeyParams are parameters for set_tee_master_key transactions.
type SetMasterKeyParams struct {
	PubKey     string `json:"pubkey"`      // hex-encoded
	PubKeyHash string `json:"pubkey_hash"` // hex-encoded
	AttestHash string `json:"attest_hash"` // hex-encoded
}

// UpdatePricesParams are parameters for update_prices transactions.
type UpdatePricesParams struct {
	FeedIDs    []string `json:"feed_ids"`
	Prices     []string `json:"prices"`     // big.Int as string
	Timestamps []uint64 `json:"timestamps"` // milliseconds
}

// =============================================================================
// API Methods
// =============================================================================

// Submit submits a transaction to the blockchain via TxSubmitter.
func (c *Client) Submit(ctx context.Context, req *SubmitRequest) (*SubmitResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/submit", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Service-ID", c.serviceID)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed: %s - %s", resp.Status, string(respBody))
	}

	var result SubmitResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &result, nil
}

// =============================================================================
// Convenience Methods
// =============================================================================

// FulfillRequest submits a fulfill_request transaction.
func (c *Client) FulfillRequest(ctx context.Context, requestID string, resultHex string) (*SubmitResponse, error) {
	params, err := json.Marshal(FulfillParams{
		RequestID: requestID,
		Result:    resultHex,
	})
	if err != nil {
		return nil, err
	}

	return c.Submit(ctx, &SubmitRequest{
		RequestID: fmt.Sprintf("%s:fulfill:%s", c.serviceID, requestID),
		TxType:    "fulfill_request",
		Params:    params,
	})
}

// FailRequest submits a fail_request transaction.
func (c *Client) FailRequest(ctx context.Context, requestID string, errorMsg string) (*SubmitResponse, error) {
	params, err := json.Marshal(FulfillParams{
		RequestID: requestID,
		Result:    errorMsg,
	})
	if err != nil {
		return nil, err
	}

	return c.Submit(ctx, &SubmitRequest{
		RequestID: fmt.Sprintf("%s:fail:%s", c.serviceID, requestID),
		TxType:    "fail_request",
		Params:    params,
	})
}

// SetTEEMasterKey submits a set_tee_master_key transaction.
func (c *Client) SetTEEMasterKey(ctx context.Context, pubKey, pubKeyHash, attestHash string) (*SubmitResponse, error) {
	params, err := json.Marshal(SetMasterKeyParams{
		PubKey:     pubKey,
		PubKeyHash: pubKeyHash,
		AttestHash: attestHash,
	})
	if err != nil {
		return nil, err
	}

	return c.Submit(ctx, &SubmitRequest{
		RequestID:           fmt.Sprintf("globalsigner:masterkey:%d", time.Now().UnixNano()),
		TxType:              "set_tee_master_key",
		Params:              params,
		WaitForConfirmation: true,
	})
}

// UpdatePrices submits an update_prices transaction.
func (c *Client) UpdatePrices(ctx context.Context, feedIDs []string, prices []string, timestamps []uint64) (*SubmitResponse, error) {
	params, err := json.Marshal(UpdatePricesParams{
		FeedIDs:    feedIDs,
		Prices:     prices,
		Timestamps: timestamps,
	})
	if err != nil {
		return nil, err
	}

	return c.Submit(ctx, &SubmitRequest{
		RequestID: fmt.Sprintf("neofeeds:prices:%d", time.Now().UnixNano()),
		TxType:    "update_prices",
		Params:    params,
	})
}

// GetStatus gets the status of a submitted transaction.
func (c *Client) GetStatus(ctx context.Context, txID int64) (*SubmitResponse, error) {
	url := fmt.Sprintf("%s/tx/%d", c.baseURL, txID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("X-Service-ID", c.serviceID)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed: %s", resp.Status)
	}

	var result SubmitResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// Health checks if TxSubmitter is healthy.
func (c *Client) Health(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: %s", resp.Status)
	}

	return nil
}
