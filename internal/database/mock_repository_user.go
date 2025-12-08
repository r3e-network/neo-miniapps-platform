package database

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// =============================================================================
// User Operations (implements UserRepository)
// =============================================================================

func (m *MockRepository) GetUser(ctx context.Context, id string) (*User, error) {
	if err := m.checkError(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if user, ok := m.users[id]; ok {
		return user, nil
	}
	return nil, NewNotFoundError("user", id)
}

func (m *MockRepository) GetUserByAddress(ctx context.Context, address string) (*User, error) {
	if err := m.checkError(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, user := range m.users {
		if user.Address == address {
			return user, nil
		}
	}
	return nil, NewNotFoundError("user", address)
}

func (m *MockRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	if err := m.checkError(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, user := range m.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, NewNotFoundError("user", email)
}

func (m *MockRepository) CreateUser(ctx context.Context, user *User) error {
	if err := m.checkError(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if user.ID == "" {
		user.ID = uuid.New().String()
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}
	user.UpdatedAt = time.Now()
	m.users[user.ID] = user
	return nil
}

func (m *MockRepository) UpdateUserEmail(ctx context.Context, userID, email string) error {
	if err := m.checkError(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if user, ok := m.users[userID]; ok {
		user.Email = email
		user.UpdatedAt = time.Now()
		return nil
	}
	return NewNotFoundError("user", userID)
}

func (m *MockRepository) UpdateUserNonce(ctx context.Context, userID, nonce string) error {
	if err := m.checkError(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.users[userID]; ok {
		return nil
	}
	return NewNotFoundError("user", userID)
}

// =============================================================================
// Service Request Operations (implements ServiceRequestRepository)
// =============================================================================

func (m *MockRepository) GetServiceRequests(ctx context.Context, userID string, limit int) ([]ServiceRequest, error) {
	if err := m.checkError(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []ServiceRequest
	for _, req := range m.serviceRequests {
		if req.UserID == userID {
			result = append(result, *req)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (m *MockRepository) CreateServiceRequest(ctx context.Context, req *ServiceRequest) error {
	if err := m.checkError(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = time.Now()
	}
	m.serviceRequests[req.ID] = req
	return nil
}

func (m *MockRepository) UpdateServiceRequest(ctx context.Context, req *ServiceRequest) error {
	if err := m.checkError(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.serviceRequests[req.ID] = req
	return nil
}

// =============================================================================
// Price Feed Operations (implements PriceFeedRepository)
// =============================================================================

func (m *MockRepository) GetLatestPrice(ctx context.Context, feedID string) (*PriceFeed, error) {
	if err := m.checkError(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var latest *PriceFeed
	for _, feed := range m.priceFeeds {
		if feed.FeedID == feedID {
			if latest == nil || feed.Timestamp.After(latest.Timestamp) {
				latest = feed
			}
		}
	}
	if latest == nil {
		return nil, NewNotFoundError("price_feed", feedID)
	}
	return latest, nil
}

func (m *MockRepository) CreatePriceFeed(ctx context.Context, feed *PriceFeed) error {
	if err := m.checkError(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if feed.ID == "" {
		feed.ID = uuid.New().String()
	}
	if feed.Timestamp.IsZero() {
		feed.Timestamp = time.Now()
	}
	m.priceFeeds[feed.ID] = feed
	return nil
}
