package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// RequestParser handles HTTP request parsing into APIRequest.
// This consolidates duplicated parsing logic from api_router.go and service_router.go.
type RequestParser struct {
	maxBodySize int64
}

// NewRequestParser creates a new request parser.
func NewRequestParser() *RequestParser {
	return &RequestParser{
		maxBodySize: 10 << 20, // 10MB default
	}
}

// WithMaxBodySize sets the maximum body size for parsing.
func (p *RequestParser) WithMaxBodySize(size int64) *RequestParser {
	p.maxBodySize = size
	return p
}

// Parse extracts data from HTTP request into APIRequest.
// accountID should be pre-extracted from the URL path.
// pathParams should be pre-extracted based on endpoint pattern matching.
func (p *RequestParser) Parse(req *http.Request, accountID string, pathParams map[string]string) (APIRequest, error) {
	apiReq := APIRequest{
		AccountID:  accountID,
		PathParams: pathParams,
		Query:      make(map[string]string),
		Body:       make(map[string]any),
	}

	if apiReq.PathParams == nil {
		apiReq.PathParams = make(map[string]string)
	}

	// Extract query parameters
	for k, v := range req.URL.Query() {
		if len(v) > 0 {
			apiReq.Query[k] = v[0]
		}
	}

	// Parse JSON body for POST/PUT/PATCH
	if req.Method == POST || req.Method == PUT || req.Method == PATCH {
		if req.Body != nil {
			defer req.Body.Close()

			// Limit body size
			reader := io.LimitReader(req.Body, p.maxBodySize)
			body, err := io.ReadAll(reader)
			if err != nil {
				return apiReq, fmt.Errorf("read body: %w", err)
			}
			if len(body) > 0 {
				if err := json.Unmarshal(body, &apiReq.Body); err != nil {
					return apiReq, fmt.Errorf("invalid JSON: %w", err)
				}
			}
		}
	}

	return apiReq, nil
}

// ParseWithAutoExtract parses request and auto-extracts accountID from path.
func (p *RequestParser) ParseWithAutoExtract(req *http.Request) (APIRequest, error) {
	accountID := ExtractAccountID(req.URL.Path)
	pathParams := ExtractPathParams(req.URL.Path)
	return p.Parse(req, accountID, pathParams)
}

// ExtractAccountID extracts accountID from /accounts/{accountID}/...
func ExtractAccountID(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 && parts[0] == "accounts" {
		return parts[1]
	}
	return ""
}

// ExtractPathParams extracts {id} style params from URL using simple heuristics.
// For precise extraction, use MatchPathParams with a pattern.
func ExtractPathParams(path string) map[string]string {
	params := make(map[string]string)
	parts := strings.Split(strings.Trim(path, "/"), "/")

	// Simple heuristic: last segment after service name is often an ID
	if len(parts) >= 4 {
		// /accounts/{accountID}/{service}/{resource}/{id}
		params["id"] = parts[len(parts)-1]
	}

	return params
}

// MatchPathParams extracts path parameters by comparing URL path with endpoint pattern.
// pattern: "jobs/{id}" or "jobs/{id}/comments"
// urlPath: "/accounts/123/automation/jobs/456"
func MatchPathParams(urlPath, pattern string) map[string]string {
	params := make(map[string]string)

	urlParts := strings.Split(strings.Trim(urlPath, "/"), "/")
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")

	if len(patternParts) == 0 {
		return params
	}

	// Find where pattern starts in URL
	for i := 0; i <= len(urlParts)-len(patternParts); i++ {
		match := true
		tempParams := make(map[string]string)

		for j, pp := range patternParts {
			up := urlParts[i+j]
			if strings.HasPrefix(pp, "{") && strings.HasSuffix(pp, "}") {
				paramName := pp[1 : len(pp)-1]
				tempParams[paramName] = up
			} else if pp != up {
				match = false
				break
			}
		}

		if match {
			return tempParams
		}
	}

	return params
}

// MatchEndpointPath checks if a URL path matches an endpoint pattern.
// Returns true and extracted parameters if matched.
func MatchEndpointPath(urlPath, pattern string) (bool, map[string]string) {
	urlPath = strings.Trim(urlPath, "/")
	pattern = strings.Trim(pattern, "/")

	urlParts := strings.Split(urlPath, "/")
	patternParts := strings.Split(pattern, "/")

	if urlPath == "" {
		urlParts = []string{}
	}
	if pattern == "" {
		patternParts = []string{}
	}

	if len(urlParts) != len(patternParts) {
		return false, nil
	}

	params := make(map[string]string)
	for i, pp := range patternParts {
		up := urlParts[i]
		if strings.HasPrefix(pp, "{") && strings.HasSuffix(pp, "}") {
			paramName := pp[1 : len(pp)-1]
			params[paramName] = up
		} else if pp != up {
			return false, nil
		}
	}

	return true, params
}

// WriteJSON writes JSON response.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// WriteError writes JSON error response.
func WriteError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

// Default parser instance for convenience.
var defaultParser = NewRequestParser()

// ParseRequest parses an HTTP request using the default parser.
func ParseRequest(req *http.Request, accountID string, pathParams map[string]string) (APIRequest, error) {
	return defaultParser.Parse(req, accountID, pathParams)
}
