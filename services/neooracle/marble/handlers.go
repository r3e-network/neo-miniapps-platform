// Package neooracle provides HTTP handlers for the neooracle service.
package neooracle

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/R3E-Network/service_layer/internal/httputil"
)

// =============================================================================
// HTTP Handlers
// =============================================================================

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
	if httputil.StrictIdentityMode() {
		parsed, err := url.Parse(input.URL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || !strings.EqualFold(parsed.Scheme, "https") {
			httputil.BadRequest(w, "only https urls are allowed in strict identity mode")
			return
		}
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
		if s.secretClient == nil {
			httputil.ServiceUnavailable(w, "secret store not configured (set SECRETS_BASE_URL)")
			return
		}
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

	respBody, truncated, err := httputil.ReadAllWithLimit(resp.Body, s.maxBodyBytes)
	if err != nil {
		httputil.InternalError(w, fmt.Sprintf("failed to read response body: %v", err))
		return
	}
	if truncated {
		httputil.WriteErrorResponse(w, r, http.StatusBadGateway, "", "upstream response too large", map[string]any{
			"limit_bytes": s.maxBodyBytes,
		})
		return
	}

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
