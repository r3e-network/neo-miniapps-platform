// Package client provides a Supabase client for the Neo Service Layer.
// This client is designed to work within SGX enclaves and integrates
// with the MarbleRun-based confidential computing architecture.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a Supabase REST API client.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// Config holds client configuration.
type Config struct {
	URL        string
	APIKey     string
	HTTPClient *http.Client
}

// New creates a new Supabase client.
func New(cfg Config) (*Client, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("URL is required")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("APIKey is required")
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	return &Client{
		baseURL:    strings.TrimSuffix(cfg.URL, "/"),
		apiKey:     cfg.APIKey,
		httpClient: httpClient,
	}, nil
}

// =============================================================================
// Database Operations (PostgREST)
// =============================================================================

// From starts a query builder for a table.
func (c *Client) From(table string) *QueryBuilder {
	return &QueryBuilder{
		client: c,
		table:  table,
	}
}

// QueryBuilder builds PostgREST queries.
type QueryBuilder struct {
	client   *Client
	table    string
	columns  string
	filters  []string
	orders   []string
	limit    int
	offset   int
	single   bool
	count    string // exact, planned, estimated
	upsert   bool
	onConflict string
}

// Select specifies columns to select.
func (q *QueryBuilder) Select(columns string) *QueryBuilder {
	q.columns = columns
	return q
}

// Eq adds an equality filter.
func (q *QueryBuilder) Eq(column string, value any) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=eq.%v", column, value))
	return q
}

// Neq adds a not-equal filter.
func (q *QueryBuilder) Neq(column string, value any) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=neq.%v", column, value))
	return q
}

// Gt adds a greater-than filter.
func (q *QueryBuilder) Gt(column string, value any) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=gt.%v", column, value))
	return q
}

// Gte adds a greater-than-or-equal filter.
func (q *QueryBuilder) Gte(column string, value any) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=gte.%v", column, value))
	return q
}

// Lt adds a less-than filter.
func (q *QueryBuilder) Lt(column string, value any) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=lt.%v", column, value))
	return q
}

// Lte adds a less-than-or-equal filter.
func (q *QueryBuilder) Lte(column string, value any) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=lte.%v", column, value))
	return q
}

// Like adds a LIKE filter.
func (q *QueryBuilder) Like(column string, pattern string) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=like.%s", column, pattern))
	return q
}

// ILike adds a case-insensitive LIKE filter.
func (q *QueryBuilder) ILike(column string, pattern string) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=ilike.%s", column, pattern))
	return q
}

// In adds an IN filter.
func (q *QueryBuilder) In(column string, values []any) *QueryBuilder {
	strValues := make([]string, len(values))
	for i, v := range values {
		strValues[i] = fmt.Sprintf("%v", v)
	}
	q.filters = append(q.filters, fmt.Sprintf("%s=in.(%s)", column, strings.Join(strValues, ",")))
	return q
}

// Is adds an IS filter (for NULL, TRUE, FALSE).
func (q *QueryBuilder) Is(column string, value any) *QueryBuilder {
	q.filters = append(q.filters, fmt.Sprintf("%s=is.%v", column, value))
	return q
}

// Order adds an ORDER BY clause.
func (q *QueryBuilder) Order(column string, ascending bool) *QueryBuilder {
	dir := "asc"
	if !ascending {
		dir = "desc"
	}
	q.orders = append(q.orders, fmt.Sprintf("%s.%s", column, dir))
	return q
}

// Limit sets the LIMIT.
func (q *QueryBuilder) Limit(n int) *QueryBuilder {
	q.limit = n
	return q
}

// Offset sets the OFFSET.
func (q *QueryBuilder) Offset(n int) *QueryBuilder {
	q.offset = n
	return q
}

// Single expects a single result.
func (q *QueryBuilder) Single() *QueryBuilder {
	q.single = true
	return q
}

// Count includes count in response.
func (q *QueryBuilder) Count(countType string) *QueryBuilder {
	q.count = countType
	return q
}

// Execute executes a SELECT query.
func (q *QueryBuilder) Execute(ctx context.Context) (*Response, error) {
	reqURL := fmt.Sprintf("%s/rest/v1/%s", q.client.baseURL, q.table)

	params := url.Values{}
	if q.columns != "" {
		params.Set("select", q.columns)
	}
	for _, f := range q.filters {
		parts := strings.SplitN(f, "=", 2)
		if len(parts) == 2 {
			params.Add(parts[0], parts[1])
		}
	}
	if len(q.orders) > 0 {
		params.Set("order", strings.Join(q.orders, ","))
	}
	if q.limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", q.limit))
	}
	if q.offset > 0 {
		params.Set("offset", fmt.Sprintf("%d", q.offset))
	}

	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	q.client.setHeaders(req)
	if q.single {
		req.Header.Set("Accept", "application/vnd.pgrst.object+json")
	}
	if q.count != "" {
		req.Header.Set("Prefer", fmt.Sprintf("count=%s", q.count))
	}

	return q.client.do(req)
}

// Insert inserts data into the table.
func (q *QueryBuilder) Insert(data any) *QueryBuilder {
	q.upsert = false
	return q
}

// Upsert upserts data into the table.
func (q *QueryBuilder) Upsert(data any, onConflict string) *QueryBuilder {
	q.upsert = true
	q.onConflict = onConflict
	return q
}

// ExecuteInsert executes an INSERT operation.
func (q *QueryBuilder) ExecuteInsert(ctx context.Context, data any) (*Response, error) {
	reqURL := fmt.Sprintf("%s/rest/v1/%s", q.client.baseURL, q.table)

	body, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	q.client.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	prefer := "return=representation"
	if q.upsert {
		prefer = "resolution=merge-duplicates," + prefer
		if q.onConflict != "" {
			req.Header.Set("On-Conflict", q.onConflict)
		}
	}
	req.Header.Set("Prefer", prefer)

	return q.client.do(req)
}

// ExecuteUpdate executes an UPDATE operation.
func (q *QueryBuilder) ExecuteUpdate(ctx context.Context, data any) (*Response, error) {
	reqURL := fmt.Sprintf("%s/rest/v1/%s", q.client.baseURL, q.table)

	params := url.Values{}
	for _, f := range q.filters {
		parts := strings.SplitN(f, "=", 2)
		if len(parts) == 2 {
			params.Add(parts[0], parts[1])
		}
	}
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	body, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PATCH", reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	q.client.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")

	return q.client.do(req)
}

// ExecuteDelete executes a DELETE operation.
func (q *QueryBuilder) ExecuteDelete(ctx context.Context) (*Response, error) {
	reqURL := fmt.Sprintf("%s/rest/v1/%s", q.client.baseURL, q.table)

	params := url.Values{}
	for _, f := range q.filters {
		parts := strings.SplitN(f, "=", 2)
		if len(parts) == 2 {
			params.Add(parts[0], parts[1])
		}
	}
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "DELETE", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	q.client.setHeaders(req)
	req.Header.Set("Prefer", "return=representation")

	return q.client.do(req)
}

// =============================================================================
// RPC (Stored Procedures)
// =============================================================================

// RPC calls a stored procedure.
func (c *Client) RPC(ctx context.Context, fn string, params any) (*Response, error) {
	reqURL := fmt.Sprintf("%s/rest/v1/rpc/%s", c.baseURL, fn)

	var body io.Reader
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal params: %w", err)
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	c.setHeaders(req)
	if params != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.do(req)
}

// =============================================================================
// Auth Operations
// =============================================================================

// Auth returns an auth client.
func (c *Client) Auth() *AuthClient {
	return &AuthClient{client: c}
}

// AuthClient handles authentication operations.
type AuthClient struct {
	client *Client
}

// SignUp creates a new user.
func (a *AuthClient) SignUp(ctx context.Context, email, password string) (*AuthResponse, error) {
	reqURL := fmt.Sprintf("%s/auth/v1/signup", a.client.baseURL)

	body, _ := json.Marshal(map[string]string{
		"email":    email,
		"password": password,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	a.client.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.do(req)
	if err != nil {
		return nil, err
	}

	var authResp AuthResponse
	if err := json.Unmarshal(resp.Body, &authResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &authResp, nil
}

// SignIn signs in a user.
func (a *AuthClient) SignIn(ctx context.Context, email, password string) (*AuthResponse, error) {
	reqURL := fmt.Sprintf("%s/auth/v1/token?grant_type=password", a.client.baseURL)

	body, _ := json.Marshal(map[string]string{
		"email":    email,
		"password": password,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	a.client.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.do(req)
	if err != nil {
		return nil, err
	}

	var authResp AuthResponse
	if err := json.Unmarshal(resp.Body, &authResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &authResp, nil
}

// GetUser gets the current user.
func (a *AuthClient) GetUser(ctx context.Context, accessToken string) (*User, error) {
	reqURL := fmt.Sprintf("%s/auth/v1/user", a.client.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	a.client.setHeaders(req)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.do(req)
	if err != nil {
		return nil, err
	}

	var user User
	if err := json.Unmarshal(resp.Body, &user); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &user, nil
}

// AuthResponse is the response from auth operations.
type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	User         *User  `json:"user"`
}

// User represents a Supabase user.
type User struct {
	ID               string                 `json:"id"`
	Email            string                 `json:"email"`
	Phone            string                 `json:"phone"`
	Role             string                 `json:"role"`
	EmailConfirmedAt string                 `json:"email_confirmed_at"`
	CreatedAt        string                 `json:"created_at"`
	UpdatedAt        string                 `json:"updated_at"`
	AppMetadata      map[string]any         `json:"app_metadata"`
	UserMetadata     map[string]any         `json:"user_metadata"`
}

// =============================================================================
// Storage Operations
// =============================================================================

// Storage returns a storage client.
func (c *Client) Storage() *StorageClient {
	return &StorageClient{client: c}
}

// StorageClient handles storage operations.
type StorageClient struct {
	client *Client
}

// From returns a bucket client.
func (s *StorageClient) From(bucket string) *BucketClient {
	return &BucketClient{
		client: s.client,
		bucket: bucket,
	}
}

// BucketClient handles bucket operations.
type BucketClient struct {
	client *Client
	bucket string
}

// Upload uploads a file.
func (b *BucketClient) Upload(ctx context.Context, path string, data []byte, contentType string) (*Response, error) {
	reqURL := fmt.Sprintf("%s/storage/v1/object/%s/%s", b.client.baseURL, b.bucket, path)

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	b.client.setHeaders(req)
	req.Header.Set("Content-Type", contentType)

	return b.client.do(req)
}

// Download downloads a file.
func (b *BucketClient) Download(ctx context.Context, path string) ([]byte, error) {
	reqURL := fmt.Sprintf("%s/storage/v1/object/%s/%s", b.client.baseURL, b.bucket, path)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	b.client.setHeaders(req)

	resp, err := b.client.do(req)
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

// Delete deletes a file.
func (b *BucketClient) Delete(ctx context.Context, paths []string) (*Response, error) {
	reqURL := fmt.Sprintf("%s/storage/v1/object/%s", b.client.baseURL, b.bucket)

	body, _ := json.Marshal(map[string][]string{
		"prefixes": paths,
	})

	req, err := http.NewRequestWithContext(ctx, "DELETE", reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	b.client.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	return b.client.do(req)
}

// GetPublicURL returns the public URL for a file.
func (b *BucketClient) GetPublicURL(path string) string {
	return fmt.Sprintf("%s/storage/v1/object/public/%s/%s", b.client.baseURL, b.bucket, path)
}

// =============================================================================
// Response Types
// =============================================================================

// Response is a generic API response.
type Response struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// JSON unmarshals the response body into v.
func (r *Response) JSON(v any) error {
	return json.Unmarshal(r.Body, v)
}

// Error returns an error if the response indicates failure.
func (r *Response) Error() error {
	if r.StatusCode >= 400 {
		var errResp struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal(r.Body, &errResp); err == nil {
			if errResp.Message != "" {
				return fmt.Errorf("supabase error: %s", errResp.Message)
			}
			if errResp.Error != "" {
				return fmt.Errorf("supabase error: %s", errResp.Error)
			}
		}
		return fmt.Errorf("supabase error: status %d", r.StatusCode)
	}
	return nil
}

// =============================================================================
// Internal Methods
// =============================================================================

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}
}

func (c *Client) do(req *http.Request) (*Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       body,
		Headers:    resp.Header,
	}, nil
}
