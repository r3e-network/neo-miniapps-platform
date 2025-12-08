// Package oracle provides HTTP handlers for the oracle service.
package oraclemarble

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/R3E-Network/service_layer/internal/httputil"
	"github.com/google/uuid"
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
