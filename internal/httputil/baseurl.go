package httputil

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/R3E-Network/service_layer/internal/runtime"
)

// BaseURLOptions configures NormalizeBaseURL.
type BaseURLOptions struct {
	// RequireHTTPSInStrictMode enforces https URLs whenever runtime.StrictIdentityMode()
	// is enabled (production/SGX/MarbleRun TLS).
	RequireHTTPSInStrictMode bool
}

// NormalizeBaseURL normalizes and validates a base URL used for service-to-service calls.
//
// It trims whitespace, removes trailing slashes, validates scheme/host, disallows
// user info, and optionally enforces https in strict identity mode.
func NormalizeBaseURL(raw string, opts BaseURLOptions) (string, *url.URL, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(raw), "/")
	if baseURL == "" {
		return "", nil, fmt.Errorf("base URL is required")
	}

	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", nil, fmt.Errorf("base URL must be a valid URL")
	}
	if parsed.User != nil {
		return "", nil, fmt.Errorf("base URL must not include user info")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", nil, fmt.Errorf("base URL scheme must be http or https")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", nil, fmt.Errorf("base URL must not include query or fragment")
	}
	if opts.RequireHTTPSInStrictMode && runtime.StrictIdentityMode() && parsed.Scheme != "https" {
		return "", nil, fmt.Errorf("base URL must use https in strict identity mode")
	}

	return baseURL, parsed, nil
}

// NormalizeServiceBaseURL is the standard normalization used by service clients.
// It enforces https whenever strict identity mode is enabled.
func NormalizeServiceBaseURL(raw string) (string, *url.URL, error) {
	return NormalizeBaseURL(raw, BaseURLOptions{RequireHTTPSInStrictMode: true})
}
