package pricefeed

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/app/domain/pricefeed"
	"github.com/R3E-Network/service_layer/pkg/logger"
)

// Fetcher retrieves prices for a feed.
type Fetcher interface {
	Fetch(ctx context.Context, feed pricefeed.Feed) (float64, string, error)
}

// FetcherFunc adapts a function to the Fetcher interface.
type FetcherFunc func(ctx context.Context, feed pricefeed.Feed) (float64, string, error)

func (f FetcherFunc) Fetch(ctx context.Context, feed pricefeed.Feed) (float64, string, error) {
	if f == nil {
		return 0, "", nil
	}
	return f(ctx, feed)
}

// HTTPFetcher retrieves prices from an HTTP endpoint.
type HTTPFetcher struct {
	client   *http.Client
	endpoint *url.URL
	apiKey   string
	log      *logger.Logger
}

// NewHTTPFetcher constructs a fetcher that calls the provided endpoint.
func NewHTTPFetcher(client *http.Client, endpoint string, apiKey string, log *logger.Logger) (*HTTPFetcher, error) {
	if strings.TrimSpace(endpoint) == "" {
		return nil, fmt.Errorf("price feed fetcher endpoint is required")
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse endpoint: %w", err)
	}
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	if log == nil {
		log = logger.NewDefault("pricefeed-http-fetcher")
	}
	return &HTTPFetcher{
		client:   client,
		endpoint: u,
		apiKey:   strings.TrimSpace(apiKey),
		log:      log,
	}, nil
}

func (f *HTTPFetcher) Fetch(ctx context.Context, feed pricefeed.Feed) (float64, string, error) {
	reqURL := *f.endpoint
	q := reqURL.Query()
	q.Set("base", feed.BaseAsset)
	q.Set("quote", feed.QuoteAsset)
	reqURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return 0, "", fmt.Errorf("build request: %w", err)
	}
	if f.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+f.apiKey)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("fetch price: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var payload struct {
		Price  float64 `json:"price"`
		Source string  `json:"source"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, "", fmt.Errorf("decode response: %w", err)
	}
	if payload.Price <= 0 {
		return 0, "", fmt.Errorf("invalid price in response")
	}
	if payload.Source == "" {
		payload.Source = reqURL.Host
	}
	return payload.Price, payload.Source, nil
}
