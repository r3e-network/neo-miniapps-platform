// Package database provides Supabase database integration.
package database

import (
	"context"
	"sync"
)

// MockRepository is an in-memory implementation of RepositoryInterface for testing.
type MockRepository struct {
	mu sync.RWMutex

	// Data stores
	users               map[string]*User
	serviceRequests     map[string]*ServiceRequest
	priceFeeds          map[string]*PriceFeed
	secrets             map[string]*Secret
	gasBankAccounts     map[string]*GasBankAccount
	gasBankTransactions map[string]*GasBankTransaction
	depositRequests     map[string]*DepositRequest

	// Error injection for testing error paths
	ErrorOnNextCall error
}

// NewMockRepository creates a new mock repository for testing.
func NewMockRepository() *MockRepository {
	return &MockRepository{
		users:               make(map[string]*User),
		serviceRequests:     make(map[string]*ServiceRequest),
		priceFeeds:          make(map[string]*PriceFeed),
		secrets:             make(map[string]*Secret),
		gasBankAccounts:     make(map[string]*GasBankAccount),
		gasBankTransactions: make(map[string]*GasBankTransaction),
		depositRequests:     make(map[string]*DepositRequest),
	}
}

// checkError returns and clears any injected error.
func (m *MockRepository) checkError() error {
	if m.ErrorOnNextCall != nil {
		err := m.ErrorOnNextCall
		m.ErrorOnNextCall = nil
		return err
	}
	return nil
}

// Reset clears all data in the mock repository.
func (m *MockRepository) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users = make(map[string]*User)
	m.serviceRequests = make(map[string]*ServiceRequest)
	m.priceFeeds = make(map[string]*PriceFeed)
	m.secrets = make(map[string]*Secret)
	m.gasBankAccounts = make(map[string]*GasBankAccount)
	m.gasBankTransactions = make(map[string]*GasBankTransaction)
	m.depositRequests = make(map[string]*DepositRequest)
	m.ErrorOnNextCall = nil
}

// HealthCheck simulates a healthy database connection for tests.
func (m *MockRepository) HealthCheck(ctx context.Context) error {
	if err := m.checkError(); err != nil {
		return err
	}
	return nil
}

// Ensure MockRepository implements RepositoryInterface
var _ RepositoryInterface = (*MockRepository)(nil)
