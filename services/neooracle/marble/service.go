// Package oracle implements a simple oracle that can fetch external data and use secrets for auth.
package neooracle

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/R3E-Network/service_layer/internal/marble"
	"github.com/R3E-Network/service_layer/internal/runtime"
	commonservice "github.com/R3E-Network/service_layer/services/common/service"
	neostoreclient "github.com/R3E-Network/service_layer/services/neostore/client"
)

const (
	ServiceID   = "neooracle"
	ServiceName = "NeoOracle Service"
	Version     = "1.0.0"
)

// Service implements the oracle.
type Service struct {
	*commonservice.BaseService
	secretClient *neostoreclient.Client
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
	base := commonservice.NewBase(&commonservice.BaseConfig{
		ID:      ServiceID,
		Name:    ServiceName,
		Version: Version,
		Marble:  cfg.Marble,
	})

	strict := runtime.StrictIdentityMode() || (cfg.Marble != nil && cfg.Marble.IsEnclave())
	if strict {
		validAllowlistEntries := 0
		for _, raw := range cfg.URLAllowlist.Prefixes {
			if _, ok := parseURLAllowlistEntry(raw); ok {
				validAllowlistEntries++
			}
		}
		if validAllowlistEntries == 0 {
			return nil, fmt.Errorf("neooracle: URL allowlist is required in strict identity mode (set ORACLE_HTTP_ALLOWLIST)")
		}
	}

	httpClient := cfg.SecretsHTTPClient
	if httpClient == nil && cfg.Marble != nil {
		httpClient = cfg.Marble.HTTPClient()
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	var secretClient *neostoreclient.Client
	if secretsBaseURL := strings.TrimSpace(cfg.SecretsBaseURL); secretsBaseURL != "" {
		client, err := neostoreclient.New(neostoreclient.Config{
			BaseURL:    secretsBaseURL,
			HTTPClient: httpClient,
			ServiceID:  ServiceID,
		})
		if err != nil {
			return nil, fmt.Errorf("neooracle: configure secret store client: %w", err)
		}
		secretClient = client
	}

	maxBytes := cfg.MaxBodyBytes
	if maxBytes <= 0 {
		maxBytes = 2 * 1024 * 1024 // 2MB default
	}

	s := &Service{
		BaseService:  base,
		secretClient: secretClient,
		httpClient: func() *http.Client {
			client := &http.Client{Timeout: 20 * time.Second}
			if cfg.Marble != nil {
				client = cfg.Marble.ExternalHTTPClient()
				client.Timeout = 20 * time.Second
			}
			return client
		}(),
		maxBodyBytes: maxBytes,
		allowlist:    cfg.URLAllowlist,
	}

	base.RegisterStandardRoutes()
	s.registerRoutes()
	return s, nil
}
