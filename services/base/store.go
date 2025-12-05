// Package base provides base components for all services.
package base

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// StoreInterface is the base interface for service data stores.
// It extends Component interface with Close method for backward compatibility.
type StoreInterface interface {
	Component
	// Close closes the store (alias for Shutdown for backward compatibility)
	Close(ctx context.Context) error
}

// Entity is the base interface for stored entities.
type Entity interface {
	GetID() string
	GetCreatedAt() time.Time
	GetUpdatedAt() time.Time
}

// BaseEntity provides common entity fields.
type BaseEntity struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GetID returns the entity ID.
func (e *BaseEntity) GetID() string {
	return e.ID
}

// GetCreatedAt returns the creation time.
func (e *BaseEntity) GetCreatedAt() time.Time {
	return e.CreatedAt
}

// GetUpdatedAt returns the update time.
func (e *BaseEntity) GetUpdatedAt() time.Time {
	return e.UpdatedAt
}

// SetTimestamps sets creation and update timestamps.
func (e *BaseEntity) SetTimestamps() {
	now := time.Now()
	if e.CreatedAt.IsZero() {
		e.CreatedAt = now
	}
	e.UpdatedAt = now
}

// GenerateID generates a unique ID for the entity if not already set.
func (e *BaseEntity) GenerateID() {
	if e.ID == "" {
		e.ID = generateUUID()
	}
}

// generateUUID generates a simple UUID-like string.
func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// =============================================================================
// In-Memory Store
// =============================================================================

// MemoryStore provides an in-memory store implementation.
type MemoryStore[T Entity] struct {
	mu    sync.RWMutex
	data  map[string]T
	ready bool
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore[T Entity]() *MemoryStore[T] {
	return &MemoryStore[T]{
		data: make(map[string]T),
	}
}

// Initialize initializes the store.
func (s *MemoryStore[T]) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ready = true
	return nil
}

// Close closes the store.
func (s *MemoryStore[T]) Close(ctx context.Context) error {
	return s.Shutdown(ctx)
}

// Shutdown shuts down the store (implements Component interface).
func (s *MemoryStore[T]) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ready = false
	s.data = make(map[string]T)
	return nil
}

// Health checks store health.
func (s *MemoryStore[T]) Health(ctx context.Context) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.ready {
		return fmt.Errorf("store not ready")
	}
	return nil
}

// Get retrieves an entity by ID.
func (s *MemoryStore[T]) Get(ctx context.Context, id string) (T, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var zero T
	if !s.ready {
		return zero, fmt.Errorf("store not ready")
	}

	entity, ok := s.data[id]
	if !ok {
		return zero, fmt.Errorf("entity not found: %s", id)
	}
	return entity, nil
}

// Create creates a new entity.
func (s *MemoryStore[T]) Create(ctx context.Context, entity T) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	id := entity.GetID()
	if _, exists := s.data[id]; exists {
		return fmt.Errorf("entity already exists: %s", id)
	}

	s.data[id] = entity
	return nil
}

// Update updates an existing entity.
func (s *MemoryStore[T]) Update(ctx context.Context, entity T) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	id := entity.GetID()
	if _, exists := s.data[id]; !exists {
		return fmt.Errorf("entity not found: %s", id)
	}

	s.data[id] = entity
	return nil
}

// Delete deletes an entity.
func (s *MemoryStore[T]) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("store not ready")
	}

	delete(s.data, id)
	return nil
}

// List returns all entities.
func (s *MemoryStore[T]) List(ctx context.Context) ([]T, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}

	entities := make([]T, 0, len(s.data))
	for _, entity := range s.data {
		entities = append(entities, entity)
	}
	return entities, nil
}

// Count returns the number of entities.
func (s *MemoryStore[T]) Count(ctx context.Context) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return 0, fmt.Errorf("store not ready")
	}

	return len(s.data), nil
}

// JSONSnapshot returns a JSON-encoded snapshot of the store's data.
func (s *MemoryStore[T]) JSONSnapshot(ctx context.Context) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready {
		return nil, fmt.Errorf("store not ready")
	}
	entities := make([]T, 0, len(s.data))
	for _, entity := range s.data {
		entities = append(entities, entity)
	}
	return json.Marshal(entities)
}

// =============================================================================
// JSON Store (File-based)
// =============================================================================

// JSONStore provides a JSON file-based store.
type JSONStore[T Entity] struct {
	mu       sync.RWMutex
	filePath string
	data     map[string]T
	ready    bool
}

// NewJSONStore creates a new JSON store.
func NewJSONStore[T Entity](filePath string) *JSONStore[T] {
	return &JSONStore[T]{
		filePath: filePath,
		data:     make(map[string]T),
	}
}

// Initialize initializes the store.
func (s *JSONStore[T]) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ready = true
	return nil
}

// Close closes the store.
func (s *JSONStore[T]) Close(ctx context.Context) error {
	return s.Shutdown(ctx)
}

// Shutdown shuts down the store (implements Component interface).
func (s *JSONStore[T]) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ready = false
	return nil
}

// Health checks store health.
func (s *JSONStore[T]) Health(ctx context.Context) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.ready {
		return fmt.Errorf("store not ready")
	}
	return nil
}

// =============================================================================
// Query Helpers
// =============================================================================

// QueryOptions specifies query parameters.
type QueryOptions struct {
	Offset  int
	Limit   int
	OrderBy string
	Desc    bool
	Filters map[string]any
}

// DefaultQueryOptions returns default query options.
func DefaultQueryOptions() QueryOptions {
	return QueryOptions{
		Offset: 0,
		Limit:  100,
	}
}

// PaginatedResult contains paginated query results.
type PaginatedResult[T any] struct {
	Items   []T  `json:"items"`
	Total   int  `json:"total"`
	Offset  int  `json:"offset"`
	Limit   int  `json:"limit"`
	HasMore bool `json:"has_more"`
}

// NewPaginatedResult creates a new paginated result.
func NewPaginatedResult[T any](items []T, total, offset, limit int) *PaginatedResult[T] {
	return &PaginatedResult[T]{
		Items:   items,
		Total:   total,
		Offset:  offset,
		Limit:   limit,
		HasMore: offset+len(items) < total,
	}
}

// =============================================================================
// Serialization Helpers
// =============================================================================

// ToJSON converts an entity to JSON.
func ToJSON(entity any) ([]byte, error) {
	return json.Marshal(entity)
}

// FromJSON converts JSON to an entity.
func FromJSON[T any](data []byte) (T, error) {
	var entity T
	err := json.Unmarshal(data, &entity)
	return entity, err
}
