// Package client provides a client SDK for the GasAccounting service.
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

// Client is a GasAccounting service client.
type Client struct {
	baseURL      string
	httpClient   *http.Client
	serviceID    string
	maxBodyBytes int64
}

// Config holds GasAccounting client configuration.
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
	defaultMaxBodySize = 1 << 20 // 1MiB
)

// New creates a new GasAccounting client.
func New(cfg Config) (*Client, error) {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	baseURL, _, err := slhttputil.NormalizeServiceBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("gasaccounting: %w", err)
	}

	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	} else {
		// Avoid mutating a caller-supplied client.
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

// =============================================================================
// Request/Response Types
// =============================================================================

// BalanceResponse represents a user's balance.
type BalanceResponse struct {
	UserID           int64     `json:"user_id"`
	AvailableBalance int64     `json:"available_balance"`
	ReservedBalance  int64     `json:"reserved_balance"`
	TotalBalance     int64     `json:"total_balance"`
	AsOf             time.Time `json:"as_of"`
}

// DepositRequest is a request to deposit GAS.
type DepositRequest struct {
	UserID    int64  `json:"user_id"`
	Amount    int64  `json:"amount"`
	TxHash    string `json:"tx_hash"`
	Reference string `json:"reference,omitempty"`
}

// DepositResponse is the response from a deposit.
type DepositResponse struct {
	EntryID     int64     `json:"entry_id"`
	NewBalance  int64     `json:"new_balance"`
	DepositedAt time.Time `json:"deposited_at"`
}

// ConsumeRequest is a request to consume GAS.
type ConsumeRequest struct {
	UserID      int64  `json:"user_id"`
	Amount      int64  `json:"amount"`
	ServiceID   string `json:"service_id"`
	RequestID   string `json:"request_id"`
	Description string `json:"description"`
}

// ConsumeResponse is the response from consuming GAS.
type ConsumeResponse struct {
	EntryID    int64     `json:"entry_id"`
	NewBalance int64     `json:"new_balance"`
	ConsumedAt time.Time `json:"consumed_at"`
}

// ReserveRequest is a request to reserve GAS.
type ReserveRequest struct {
	UserID    int64         `json:"user_id"`
	Amount    int64         `json:"amount"`
	ServiceID string        `json:"service_id"`
	RequestID string        `json:"request_id"`
	TTL       time.Duration `json:"ttl"`
}

// ReserveResponse is the response from reserving GAS.
type ReserveResponse struct {
	ReservationID string    `json:"reservation_id"`
	Amount        int64     `json:"amount"`
	ExpiresAt     time.Time `json:"expires_at"`
	NewAvailable  int64     `json:"new_available"`
}

// ReleaseRequest is a request to release a reservation.
type ReleaseRequest struct {
	ReservationID string `json:"reservation_id"`
	Consume       bool   `json:"consume"`
	ActualAmount  int64  `json:"actual_amount,omitempty"`
}

// ReleaseResponse is the response from releasing a reservation.
type ReleaseResponse struct {
	EntryID      int64 `json:"entry_id"`
	Released     int64 `json:"released"`
	Consumed     int64 `json:"consumed"`
	NewAvailable int64 `json:"new_available"`
}

// LedgerEntry represents a ledger entry.
type LedgerEntry struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	EntryType      string    `json:"entry_type"`
	Amount         int64     `json:"amount"`
	BalanceAfter   int64     `json:"balance_after"`
	ReferenceID    string    `json:"reference_id"`
	ReferenceType  string    `json:"reference_type"`
	ServiceID      string    `json:"service_id"`
	Description    string    `json:"description"`
	CreatedAt      time.Time `json:"created_at"`
	IdempotencyKey string    `json:"idempotency_key"`
}

// HistoryResponse is the response for ledger history.
type HistoryResponse struct {
	Entries    []*LedgerEntry `json:"entries"`
	TotalCount int            `json:"total_count"`
	HasMore    bool           `json:"has_more"`
}

// =============================================================================
// Client Methods
// =============================================================================

// GetBalance returns a user's current balance.
func (c *Client) GetBalance(ctx context.Context, userID int64) (*BalanceResponse, error) {
	u := fmt.Sprintf("%s/balance?user_id=%d", c.baseURL, userID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	var resp BalanceResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// Deposit records a GAS deposit.
func (c *Client) Deposit(ctx context.Context, req *DepositRequest) (*DepositResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/deposit", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	var resp DepositResponse
	if err := c.do(httpReq, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// Consume deducts GAS for a service operation.
func (c *Client) Consume(ctx context.Context, userID int64, amount int64, requestID, description string) (*ConsumeResponse, error) {
	req := &ConsumeRequest{
		UserID:      userID,
		Amount:      amount,
		ServiceID:   c.serviceID,
		RequestID:   requestID,
		Description: description,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/consume", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	var resp ConsumeResponse
	if err := c.do(httpReq, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// Reserve reserves GAS for a pending operation.
func (c *Client) Reserve(ctx context.Context, userID int64, amount int64, requestID string, ttl time.Duration) (*ReserveResponse, error) {
	req := &ReserveRequest{
		UserID:    userID,
		Amount:    amount,
		ServiceID: c.serviceID,
		RequestID: requestID,
		TTL:       ttl,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/reserve", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	var resp ReserveResponse
	if err := c.do(httpReq, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// Release releases or consumes a reservation.
func (c *Client) Release(ctx context.Context, reservationID string, consume bool, actualAmount int64) (*ReleaseResponse, error) {
	req := &ReleaseRequest{
		ReservationID: reservationID,
		Consume:       consume,
		ActualAmount:  actualAmount,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/release", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	var resp ReleaseResponse
	if err := c.do(httpReq, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// GetHistory returns ledger history for a user.
func (c *Client) GetHistory(ctx context.Context, userID int64, entryType string, limit, offset int) (*HistoryResponse, error) {
	params := url.Values{}
	params.Set("user_id", strconv.FormatInt(userID, 10))
	if entryType != "" {
		params.Set("type", entryType)
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		params.Set("offset", strconv.Itoa(offset))
	}

	u := fmt.Sprintf("%s/history?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	var resp HistoryResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// =============================================================================
// Helper Methods
// =============================================================================

func (c *Client) do(req *http.Request, result any) error {
	if c == nil {
		return fmt.Errorf("gasaccounting: client is nil")
	}
	if c.httpClient == nil {
		return fmt.Errorf("gasaccounting: http client not configured")
	}

	if req != nil && c.serviceID != "" && req.Header.Get(serviceauth.ServiceIDHeader) == "" {
		req.Header.Set(serviceauth.ServiceIDHeader, c.serviceID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, truncated, readErr := slhttputil.ReadAllWithLimit(resp.Body, 32<<10)
		if readErr != nil {
			return fmt.Errorf("gasaccounting error: %s (failed to read body: %v)", resp.Status, readErr)
		}

		msg := strings.TrimSpace(string(body))
		if msg != "" {
			var errResp struct {
				Error   string `json:"error"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(body, &errResp); err == nil {
				if errResp.Error != "" {
					return fmt.Errorf("gasaccounting error: %s", errResp.Error)
				}
				if errResp.Message != "" {
					return fmt.Errorf("gasaccounting error: %s", errResp.Message)
				}
			}
			if truncated {
				msg += "...(truncated)"
			}
			return fmt.Errorf("gasaccounting error: %s - %s", resp.Status, msg)
		}

		return fmt.Errorf("gasaccounting error: %s", resp.Status)
	}

	if result != nil {
		respBody, err := slhttputil.ReadAllStrict(resp.Body, c.maxBodyBytes)
		if err != nil {
			return fmt.Errorf("read response: %w", err)
		}
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}
