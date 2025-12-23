package database

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// =============================================================================
// Repository Pattern
// =============================================================================

// Repository provides data access methods.
type Repository struct {
	client *Client
}

// NewRepository creates a new repository.
func NewRepository(client *Client) *Repository {
	return &Repository{client: client}
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
	if unmarshalErr := json.Unmarshal(data, &users); unmarshalErr != nil {
		return nil, fmt.Errorf("%w: unmarshal users: %v", ErrDatabaseError, unmarshalErr)
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

	escapedAddress := url.QueryEscape(address)

	// Prefer the wallet bindings table so any bound wallet (not just the primary)
	// can be used for wallet-based login/nonce flows.
	walletData, err := r.client.request(ctx, "GET", "user_wallets", nil, "address=eq."+escapedAddress+"&limit=2")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDatabaseError, err)
	}

	var wallets []UserWallet
	if unmarshalErr := json.Unmarshal(walletData, &wallets); unmarshalErr != nil {
		return nil, fmt.Errorf("%w: unmarshal wallets: %v", ErrDatabaseError, unmarshalErr)
	}
	if len(wallets) > 1 {
		return nil, fmt.Errorf("%w: wallet address %q is bound to multiple users", ErrDatabaseError, address)
	}
	if len(wallets) == 1 {
		return r.GetUser(ctx, wallets[0].UserID)
	}

	// Backward compatible fallback for legacy records that used users.address
	// without a corresponding wallet binding.
	data, err := r.client.request(ctx, "GET", "users", nil, "address=eq."+escapedAddress+"&limit=1")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDatabaseError, err)
	}

	var users []User
	if unmarshalErr := json.Unmarshal(data, &users); unmarshalErr != nil {
		return nil, fmt.Errorf("%w: unmarshal users: %v", ErrDatabaseError, unmarshalErr)
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
	if unmarshalErr := json.Unmarshal(data, &users); unmarshalErr != nil {
		return nil, fmt.Errorf("%w: unmarshal users: %v", ErrDatabaseError, unmarshalErr)
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

	payload := map[string]any{
		"id": user.ID,
	}

	if email := SanitizeString(user.Email); email != "" {
		if err := ValidateEmail(email); err != nil {
			return err
		}
		payload["email"] = email
	}

	if addr := strings.TrimSpace(user.Address); addr != "" {
		if err := ValidateAddress(addr); err != nil {
			return err
		}
		payload["address"] = addr
	}

	if nonce := SanitizeString(user.Nonce); nonce != "" {
		payload["nonce"] = nonce
	}

	_, err := r.client.request(ctx, "POST", "users", payload, "")
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

	// Query pool_accounts table which has proper permissions (users table is restricted)
	if _, err := r.client.request(checkCtx, "GET", "pool_accounts", nil, "select=id&limit=1"); err != nil {
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
	if unmarshalErr := json.Unmarshal(data, &requests); unmarshalErr != nil {
		return nil, fmt.Errorf("%w: unmarshal service requests: %v", ErrDatabaseError, unmarshalErr)
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
	if unmarshalErr := json.Unmarshal(data, &feeds); unmarshalErr != nil {
		return nil, fmt.Errorf("%w: unmarshal price feeds: %v", ErrDatabaseError, unmarshalErr)
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
