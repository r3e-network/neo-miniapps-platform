// Package oracle implements a simple oracle that can fetch external data and use secrets for auth.
package neooracle

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/R3E-Network/service_layer/internal/marble"
	commonservice "github.com/R3E-Network/service_layer/services/common/service"
)

const (
	ServiceID   = "neooracle"
	ServiceName = "NeoNeoOracle Service"
	Version     = "1.0.0"
)

// SecretsClient fetches secrets from the Secrets service over mTLS.
type SecretsClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewSecretsClient(baseURL string, httpClient *http.Client) *SecretsClient {
	return &SecretsClient{baseURL: baseURL, httpClient: httpClient}
}

func (c *SecretsClient) GetSecret(ctx context.Context, userID, name string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/secrets/%s", c.baseURL, name), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-User-ID", userID)
	req.Header.Set("X-Service-ID", ServiceID) // identify oracle as caller to Secrets service

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("secret service error: %s", resp.Status)
	}

	var out struct {
		Name    string `json:"name"`
		Value   string `json:"value"`
		Version int    `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Value, nil
}

// Service implements the oracle.
type Service struct {
	*commonservice.BaseService
	secretClient *SecretsClient
	httpClient   *http.Client
	maxBodyBytes int64
	allowlist    URLAllowlist
}

// Config configures the oracle.
type Config struct {
	Marble            *marble.Marble
	SecretsBaseURL    string
	SecretsHTTPClient *http.Client // optional (defaults to Marble mTLS client)
	MaxBodyBytes      int64        // optional response cap; default 2MB
	URLAllowlist      URLAllowlist // optional allowlist for outbound fetch
}

// New creates a new NeoOracle service.
func New(cfg Config) (*Service, error) {
	base := commonservice.NewBase(commonservice.BaseConfig{
		ID:      ServiceID,
		Name:    ServiceName,
		Version: Version,
		Marble:  cfg.Marble,
	})

	httpClient := cfg.SecretsHTTPClient
	if httpClient == nil && cfg.Marble != nil {
		httpClient = cfg.Marble.HTTPClient()
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	secretClient := NewSecretsClient(cfg.SecretsBaseURL, httpClient)

	maxBytes := cfg.MaxBodyBytes
	if maxBytes <= 0 {
		maxBytes = 2 * 1024 * 1024 // 2MB default
	}

	s := &Service{
		BaseService:  base,
		secretClient: secretClient,
		httpClient:   &http.Client{Timeout: 20 * time.Second},
		maxBodyBytes: maxBytes,
		allowlist:    cfg.URLAllowlist,
	}
	s.registerRoutes()
	return s, nil
}
