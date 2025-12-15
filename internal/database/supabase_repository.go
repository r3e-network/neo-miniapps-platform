package database

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// =============================================================================
// Repository Pattern
// =============================================================================

// Repository provides data access methods.
type Repository struct {
	client *Client

	// oauthTokensMasterKey optionally enables at-rest encryption for OAuth
	// access/refresh tokens stored in Supabase. When unset, the repository falls
	// back to reading OAUTH_TOKENS_MASTER_KEY from the environment.
	oauthTokensMasterKey []byte
}

// NewRepository creates a new repository.
func NewRepository(client *Client) *Repository {
	return &Repository{client: client}
}

// SetOAuthTokensMasterKey configures an explicit OAuth token master key.
// The key must be exactly 32 bytes (AES-256).
func (r *Repository) SetOAuthTokensMasterKey(key []byte) error {
	if r == nil {
		return fmt.Errorf("%w: repository cannot be nil", ErrInvalidInput)
	}
	if len(key) == 0 {
		r.oauthTokensMasterKey = nil
		return nil
	}
	if len(key) != oauthTokensMasterKeyLength {
		return fmt.Errorf("%w: OAUTH_TOKENS_MASTER_KEY must be %d bytes, got %d", ErrInvalidInput, oauthTokensMasterKeyLength, len(key))
	}

	copied := make([]byte, len(key))
	copy(copied, key)
	r.oauthTokensMasterKey = copied
	return nil
}

func (r *Repository) oauthTokensMasterKeyOptional() (key []byte, ok bool, err error) {
	if r != nil && len(r.oauthTokensMasterKey) > 0 {
		return r.oauthTokensMasterKey, true, nil
	}
	return oauthTokensMasterKeyOptional()
}

// Request makes an HTTP request to the Supabase REST API.
// This method is exported to allow service-specific repositories to make database calls.
func (r *Repository) Request(ctx context.Context, method, table string, body interface{}, query string) ([]byte, error) {
	return r.client.request(ctx, method, table, body, query)
}

// =============================================================================
// User Operations
// =============================================================================

// GetUser retrieves a user by ID.
func (r *Repository) GetUser(ctx context.Context, id string) (*User, error) {
	if err := ValidateID(id); err != nil {
		return nil, err
	}

	data, err := r.client.request(ctx, "GET", "users", nil, "id=eq."+url.QueryEscape(id)+"&limit=1")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDatabaseError, err)
	}

	var users []User
	if err := json.Unmarshal(data, &users); err != nil {
		return nil, fmt.Errorf("%w: unmarshal users: %v", ErrDatabaseError, err)
	}
	if len(users) == 0 {
		return nil, NewNotFoundError("user", id)
	}
	return &users[0], nil
}

// GetUserByAddress retrieves a user by wallet address.
func (r *Repository) GetUserByAddress(ctx context.Context, address string) (*User, error) {
	if err := ValidateAddress(address); err != nil {
		return nil, err
	}

	data, err := r.client.request(ctx, "GET", "users", nil, "address=eq."+url.QueryEscape(address)+"&limit=1")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDatabaseError, err)
	}

	var users []User
	if err := json.Unmarshal(data, &users); err != nil {
		return nil, fmt.Errorf("%w: unmarshal users: %v", ErrDatabaseError, err)
	}
	if len(users) == 0 {
		return nil, NewNotFoundError("user", address)
	}
	return &users[0], nil
}

// GetUserByEmail retrieves a user by email.
func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	if err := ValidateEmail(email); err != nil {
		return nil, err
	}
	if email == "" {
		return nil, fmt.Errorf("%w: email cannot be empty", ErrInvalidInput)
	}

	data, err := r.client.request(ctx, "GET", "users", nil, "email=eq."+url.QueryEscape(email)+"&limit=1")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDatabaseError, err)
	}

	var users []User
	if err := json.Unmarshal(data, &users); err != nil {
		return nil, fmt.Errorf("%w: unmarshal users: %v", ErrDatabaseError, err)
	}
	if len(users) == 0 {
		return nil, NewNotFoundError("user", email)
	}
	return &users[0], nil
}

// CreateUser creates a new user.
func (r *Repository) CreateUser(ctx context.Context, user *User) error {
	if user == nil {
		return fmt.Errorf("%w: user cannot be nil", ErrInvalidInput)
	}
	if user.ID == "" {
		return fmt.Errorf("%w: user id cannot be empty", ErrInvalidInput)
	}

	_, err := r.client.request(ctx, "POST", "users", user, "")
	if err != nil {
		return fmt.Errorf("%w: create user: %v", ErrDatabaseError, err)
	}
	return nil
}

// UpdateUserEmail updates user's email.
func (r *Repository) UpdateUserEmail(ctx context.Context, userID, email string) error {
	if err := ValidateUserID(userID); err != nil {
		return err
	}
	if err := ValidateEmail(email); err != nil {
		return err
	}

	update := map[string]interface{}{
		"email":      email,
		"updated_at": time.Now(),
	}
	_, err := r.client.request(ctx, "PATCH", "users", update, "id=eq."+url.QueryEscape(userID))
	if err != nil {
		return fmt.Errorf("%w: update user email: %v", ErrDatabaseError, err)
	}
	return nil
}

// UpdateUserNonce updates the user's nonce for signature verification.
func (r *Repository) UpdateUserNonce(ctx context.Context, userID, nonce string) error {
	if err := ValidateUserID(userID); err != nil {
		return err
	}

	update := map[string]interface{}{
		"nonce": SanitizeString(nonce),
	}
	_, err := r.client.request(ctx, "PATCH", "users", update, "id=eq."+url.QueryEscape(userID))
	if err != nil {
		return fmt.Errorf("%w: update user nonce: %v", ErrDatabaseError, err)
	}
	return nil
}

// HealthCheck verifies database connectivity by issuing a lightweight query.
func (r *Repository) HealthCheck(ctx context.Context) error {
	if r == nil || r.client == nil {
		return fmt.Errorf("repository not initialized")
	}
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if _, err := r.client.request(checkCtx, "GET", "users", nil, "select=id&limit=1"); err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	return nil
}

// =============================================================================
// Service Request Operations
// =============================================================================

// GetServiceRequests retrieves service requests for a user.
func (r *Repository) GetServiceRequests(ctx context.Context, userID string, limit int) ([]ServiceRequest, error) {
	if err := ValidateUserID(userID); err != nil {
		return nil, err
	}
	limit = ValidateLimit(limit, 50, 1000)

	query := fmt.Sprintf("user_id=eq.%s&order=created_at.desc&limit=%d", url.QueryEscape(userID), limit)
	data, err := r.client.request(ctx, "GET", "service_requests", nil, query)
	if err != nil {
		return nil, fmt.Errorf("%w: get service requests: %v", ErrDatabaseError, err)
	}

	var requests []ServiceRequest
	if err := json.Unmarshal(data, &requests); err != nil {
		return nil, fmt.Errorf("%w: unmarshal service requests: %v", ErrDatabaseError, err)
	}
	return requests, nil
}

// CreateServiceRequest creates a new service request.
func (r *Repository) CreateServiceRequest(ctx context.Context, req *ServiceRequest) error {
	if req == nil {
		return fmt.Errorf("%w: service request cannot be nil", ErrInvalidInput)
	}
	if req.ID == "" {
		return fmt.Errorf("%w: service request id cannot be empty", ErrInvalidInput)
	}

	_, err := r.client.request(ctx, "POST", "service_requests", req, "")
	if err != nil {
		return fmt.Errorf("%w: create service request: %v", ErrDatabaseError, err)
	}
	return nil
}

// UpdateServiceRequest updates a service request.
func (r *Repository) UpdateServiceRequest(ctx context.Context, req *ServiceRequest) error {
	if req == nil {
		return fmt.Errorf("%w: service request cannot be nil", ErrInvalidInput)
	}
	if err := ValidateID(req.ID); err != nil {
		return err
	}

	_, err := r.client.request(ctx, "PATCH", "service_requests", req, "id=eq."+url.QueryEscape(req.ID))
	if err != nil {
		return fmt.Errorf("%w: update service request: %v", ErrDatabaseError, err)
	}
	return nil
}

// =============================================================================
// Price Feed Operations
// =============================================================================

// GetLatestPrice retrieves the latest price for a feed.
func (r *Repository) GetLatestPrice(ctx context.Context, feedID string) (*PriceFeed, error) {
	if feedID == "" {
		return nil, fmt.Errorf("%w: feed_id cannot be empty", ErrInvalidInput)
	}
	feedID = SanitizeString(feedID)

	query := fmt.Sprintf("feed_id=eq.%s&order=timestamp.desc&limit=1", url.QueryEscape(feedID))
	data, err := r.client.request(ctx, "GET", "price_feeds", nil, query)
	if err != nil {
		return nil, fmt.Errorf("%w: get latest price: %v", ErrDatabaseError, err)
	}

	var feeds []PriceFeed
	if err := json.Unmarshal(data, &feeds); err != nil {
		return nil, fmt.Errorf("%w: unmarshal price feeds: %v", ErrDatabaseError, err)
	}
	if len(feeds) == 0 {
		return nil, NewNotFoundError("price_feed", feedID)
	}
	return &feeds[0], nil
}

// CreatePriceFeed creates a new price feed entry.
func (r *Repository) CreatePriceFeed(ctx context.Context, feed *PriceFeed) error {
	if feed == nil {
		return fmt.Errorf("%w: price feed cannot be nil", ErrInvalidInput)
	}
	if feed.FeedID == "" {
		return fmt.Errorf("%w: feed_id cannot be empty", ErrInvalidInput)
	}

	_, err := r.client.request(ctx, "POST", "price_feeds", feed, "")
	if err != nil {
		return fmt.Errorf("%w: create price feed: %v", ErrDatabaseError, err)
	}
	return nil
}
