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
	"time"
)

// Client is a GasAccounting service client.
type Client struct {
	baseURL    string
	httpClient *http.Client
	serviceID  string
}

// New creates a new GasAccounting client.
func New(baseURL, serviceID string) *Client {
	return &Client{
		baseURL:   baseURL,
		serviceID: serviceID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
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
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errResp struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil && errResp.Error != "" {
			return fmt.Errorf("gasaccounting error: %s", errResp.Error)
		}
		return fmt.Errorf("gasaccounting error: status %d", resp.StatusCode)
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}
