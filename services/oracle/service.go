// Package oracle implements a simple oracle that can fetch external data and use secrets for auth.
package oracle

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/httputil"
	"github.com/R3E-Network/service_layer/internal/marble"
	"github.com/google/uuid"
)

const (
	ServiceID   = "oracle"
	ServiceName = "Oracle Service"
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
	*marble.Service
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

// New creates a new Oracle service.
func New(cfg Config) (*Service, error) {
	base := marble.NewService(marble.ServiceConfig{
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
		Service:      base,
		secretClient: secretClient,
		httpClient:   &http.Client{Timeout: 20 * time.Second},
		maxBodyBytes: maxBytes,
		allowlist:    cfg.URLAllowlist,
	}
	s.registerRoutes()
	return s, nil
}

func (s *Service) registerRoutes() {
	r := s.Router()
	r.HandleFunc("/health", marble.HealthHandler(s.Service)).Methods("GET")
	r.HandleFunc("/query", s.handleQuery).Methods("POST")
}

// handleQuery fetches external data, optionally injecting a secret for auth.
func (s *Service) handleQuery(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.RequireUserID(w, r)
	if !ok {
		return
	}

	var input QueryInput
	if !httputil.DecodeJSON(w, r, &input) {
		return
	}

	if input.URL == "" {
		httputil.BadRequest(w, "url required")
		return
	}
	if !s.allowlist.Allows(input.URL) {
		httputil.BadRequest(w, "url not allowed")
		return
	}
	method := strings.ToUpper(strings.TrimSpace(input.Method))
	if method == "" {
		method = http.MethodGet
	}

	headers := make(http.Header)
	for k, v := range input.Headers {
		headers.Set(k, v)
	}

	// If a secret is requested, fetch it over mTLS and inject.
	if input.SecretName != "" {
		secret, err := s.secretClient.GetSecret(r.Context(), userID, input.SecretName)
		if err != nil {
			httputil.InternalError(w, fmt.Sprintf("failed to fetch secret: %v", err))
			return
		}
		key := input.SecretAsKey
		if key == "" {
			key = "Authorization"
			secret = "Bearer " + secret
		}
		headers.Set(key, secret)
	}

	var body io.Reader
	if input.Body != "" {
		body = bytes.NewBufferString(input.Body)
	}

	req, err := http.NewRequestWithContext(r.Context(), method, input.URL, body)
	if err != nil {
		httputil.BadRequest(w, err.Error())
		return
	}
	req.Header = headers
	req.Header.Set("X-Request-ID", uuid.New().String())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		httputil.InternalError(w, fmt.Sprintf("request failed: %v", err))
		return
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, s.maxBodyBytes)
	respBody, _ := io.ReadAll(limited)

	outHeaders := map[string]string{}
	for k, vals := range resp.Header {
		if len(vals) > 0 {
			outHeaders[k] = vals[0]
		}
	}

	httputil.WriteJSON(w, http.StatusOK, QueryResponse{
		StatusCode: resp.StatusCode,
		Headers:    outHeaders,
		Body:       string(respBody),
	})
}
