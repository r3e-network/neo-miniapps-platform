// Package template provides storage operations for the template service.
package template

import (
	"context"
	"fmt"
	"sync"

	"github.com/R3E-Network/service_layer/platform/os"
)

// Store handles data persistence for the template service.
type Store struct {
	mu    sync.RWMutex
	os    os.ServiceOS
	ready bool

	// In-memory fallback when database is not available
	items map[string]*Item
}

// NewStore creates a new store for the template service.
func NewStore(serviceOS os.ServiceOS) *Store {
	return &Store{
		os:    serviceOS,
		items: make(map[string]*Item),
	}
}

// Initialize initializes the store.
func (s *Store) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.os.Logger().Info("initializing template store")
	s.ready = true
	s.os.Logger().Info("template store initialized")
	return nil
}

// Close closes the store.
func (s *Store) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.os.Logger().Info("closing template store")
	s.ready = false
	return nil
}

// Health checks store health.
func (s *Store) Health(ctx context.Context) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}
	return nil
}

// =============================================================================
// CRUD Operations
// =============================================================================

// CreateItem creates a new item.
func (s *Store) CreateItem(ctx context.Context, item *Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	// Use database if available
	if s.os.HasCapability(os.CapDatabase) {
		_, err := s.os.Database().From("items").Insert(item).Execute(ctx)
		if err != nil {
			return fmt.Errorf("insert item: %w", err)
		}
		return nil
	}

	// Fallback to in-memory storage
	if item.ID == "" {
		item.ID = generateID()
	}
	s.items[item.ID] = item
	return nil
}

// GetItem retrieves an item by ID.
func (s *Store) GetItem(ctx context.Context, id string) (*Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}

	// Use database if available
	if s.os.HasCapability(os.CapDatabase) {
		var item Item
		err := s.os.Database().From("items").
			Select("*").
			Eq("id", id).
			Single().
			ExecuteInto(ctx, &item)
		if err != nil {
			return nil, fmt.Errorf("get item: %w", err)
		}
		return &item, nil
	}

	// Fallback to in-memory storage
	item, exists := s.items[id]
	if !exists {
		return nil, fmt.Errorf("item not found: %s", id)
	}
	return item, nil
}

// UpdateItem updates an existing item.
func (s *Store) UpdateItem(ctx context.Context, item *Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	// Use database if available
	if s.os.HasCapability(os.CapDatabase) {
		_, err := s.os.Database().From("items").
			Update(item).
			Eq("id", item.ID).
			Execute(ctx)
		if err != nil {
			return fmt.Errorf("update item: %w", err)
		}
		return nil
	}

	// Fallback to in-memory storage
	if _, exists := s.items[item.ID]; !exists {
		return fmt.Errorf("item not found: %s", item.ID)
	}
	s.items[item.ID] = item
	return nil
}

// DeleteItem deletes an item by ID.
func (s *Store) DeleteItem(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	// Use database if available
	if s.os.HasCapability(os.CapDatabase) {
		_, err := s.os.Database().From("items").
			Delete().
			Eq("id", id).
			Execute(ctx)
		if err != nil {
			return fmt.Errorf("delete item: %w", err)
		}
		return nil
	}

	// Fallback to in-memory storage
	delete(s.items, id)
	return nil
}

// ListItems lists items with optional filtering.
func (s *Store) ListItems(ctx context.Context, req *ListItemsRequest) (*ListItemsResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}

	// Use database if available
	if s.os.HasCapability(os.CapDatabase) {
		query := s.os.Database().From("items").Select("*")

		if req.Status != nil {
			query = query.Eq("status", string(*req.Status))
		}

		if req.Limit > 0 {
			query = query.Limit(req.Limit)
		}

		if req.Offset > 0 {
			query = query.Offset(req.Offset)
		}

		var items []*Item
		err := query.ExecuteInto(ctx, &items)
		if err != nil {
			return nil, fmt.Errorf("list items: %w", err)
		}

		return &ListItemsResponse{
			Items:      items,
			TotalCount: len(items),
			HasMore:    len(items) == req.Limit,
		}, nil
	}

	// Fallback to in-memory storage
	items := make([]*Item, 0, len(s.items))
	for _, item := range s.items {
		if req.Status != nil && item.Status != *req.Status {
			continue
		}
		items = append(items, item)
	}

	// Apply pagination
	start := req.Offset
	if start > len(items) {
		start = len(items)
	}
	end := start + req.Limit
	if end > len(items) || req.Limit == 0 {
		end = len(items)
	}

	return &ListItemsResponse{
		Items:      items[start:end],
		TotalCount: len(items),
		HasMore:    end < len(items),
	}, nil
}

// GetStats returns store statistics.
func (s *Store) GetStats(ctx context.Context) (*Stats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}

	// Count items
	totalItems := len(s.items)
	activeItems := 0
	for _, item := range s.items {
		if item.Status == ItemStatusActive {
			activeItems++
		}
	}

	return &Stats{
		TotalItems:  totalItems,
		ActiveItems: activeItems,
	}, nil
}

// =============================================================================
// Helper Functions
// =============================================================================

func generateID() string {
	// Simple ID generation - in production use UUID
	return fmt.Sprintf("item_%d", len("items"))
}
