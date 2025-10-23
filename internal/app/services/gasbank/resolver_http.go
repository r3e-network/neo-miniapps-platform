package gasbank

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/gasbank"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// HTTPWithdrawalResolver polls an HTTP endpoint to settle withdrawals.
type HTTPWithdrawalResolver struct {
	client   *http.Client
	endpoint *url.URL
	apiKey   string
	log      *logger.Logger
}

// NewHTTPWithdrawalResolver builds a resolver that calls the provided endpoint.
func NewHTTPWithdrawalResolver(client *http.Client, endpoint, apiKey string, log *logger.Logger) (*HTTPWithdrawalResolver, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("resolver endpoint required")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse resolver endpoint: %w", err)
	}
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	if log == nil {
		log = logger.NewDefault("gasbank-http-resolver")
	}
	return &HTTPWithdrawalResolver{
		client:   client,
		endpoint: parsed,
		apiKey:   strings.TrimSpace(apiKey),
		log:      log,
	}, nil
}

func (r *HTTPWithdrawalResolver) Resolve(ctx context.Context, tx gasbank.Transaction) (bool, bool, string, time.Duration, error) {
	requestURL := *r.endpoint
	q := requestURL.Query()
	q.Set("transaction_id", tx.ID)
	requestURL.RawQuery = q.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return false, false, "", 0, fmt.Errorf("build resolver request: %w", err)
	}
	if r.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.apiKey)
	}

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return false, false, "", 0, fmt.Errorf("resolver request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, false, "", 0, fmt.Errorf("resolver status %d", resp.StatusCode)
	}

	var payload struct {
		Done       bool    `json:"done"`
		Success    bool    `json:"success"`
		Message    string  `json:"message"`
		RetryAfter float64 `json:"retry_after_seconds"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return false, false, "", 0, fmt.Errorf("decode resolver response: %w", err)
	}

	retry := time.Duration(payload.RetryAfter * float64(time.Second))
	if retry <= 0 {
		retry = 5 * time.Second
	}

	if !payload.Done {
		return false, false, "", retry, nil
	}

	return true, payload.Success, payload.Message, 0, nil
}
