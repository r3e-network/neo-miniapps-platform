// Package oracle provides a simple data-fetching oracle service.
package oraclemarble

// QueryInput is the request payload to fetch external data.
type QueryInput struct {
	URL         string            `json:"url"`
	Method      string            `json:"method,omitempty"`        // default: GET
	Headers     map[string]string `json:"headers,omitempty"`       // optional additional headers
	SecretName  string            `json:"secret_name,omitempty"`   // optional: fetch secret and send as Authorization bearer
	SecretAsKey string            `json:"secret_as_key,omitempty"` // optional: header key to place secret in (default Authorization: Bearer <secret>)
	Body        string            `json:"body,omitempty"`          // optional body for POST/PUT
}

// QueryResponse returns the fetched data.
type QueryResponse struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}
