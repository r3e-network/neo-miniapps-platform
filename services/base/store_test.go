// Package base provides base components for all services.
package base

import (
	"context"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// BaseEntity Tests
// =============================================================================

func TestBaseEntity_GetID(t *testing.T) {
	entity := &BaseEntity{ID: "test-id"}
	if entity.GetID() != "test-id" {
		t.Errorf("GetID() = %s, want test-id", entity.GetID())
	}
}

func TestBaseEntity_GetCreatedAt(t *testing.T) {
	now := time.Now()
	entity := &BaseEntity{CreatedAt: now}
	if !entity.GetCreatedAt().Equal(now) {
		t.Errorf("GetCreatedAt() = %v, want %v", entity.GetCreatedAt(), now)
	}
}

func TestBaseEntity_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	entity := &BaseEntity{UpdatedAt: now}
	if !entity.GetUpdatedAt().Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", entity.GetUpdatedAt(), now)
	}
}

func TestBaseEntity_SetTimestamps(t *testing.T) {
	entity := &BaseEntity{}

	// First call sets both
	entity.SetTimestamps()
	if entity.CreatedAt.IsZero() {
		t.Error("SetTimestamps() should set CreatedAt")
	}
	if entity.UpdatedAt.IsZero() {
		t.Error("SetTimestamps() should set UpdatedAt")
	}

	createdAt := entity.CreatedAt
	time.Sleep(1 * time.Millisecond)

	// Second call only updates UpdatedAt
	entity.SetTimestamps()
	if !entity.CreatedAt.Equal(createdAt) {
		t.Error("SetTimestamps() should not change CreatedAt on second call")
	}
	if entity.UpdatedAt.Equal(createdAt) {
		t.Error("SetTimestamps() should update UpdatedAt on second call")
	}
}

func TestBaseEntity_GenerateID(t *testing.T) {
	entity := &BaseEntity{}

	// Generate ID when empty
	entity.GenerateID()
	if entity.ID == "" {
		t.Error("GenerateID() should set ID")
	}

	firstID := entity.ID

	// Should not change existing ID
	entity.GenerateID()
	if entity.ID != firstID {
		t.Error("GenerateID() should not change existing ID")
	}
}

func TestGenerateUUID_Uniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := generateUUID()
		if ids[id] {
			t.Errorf("generateUUID() produced duplicate: %s", id)
		}
		ids[id] = true
	}
}

// =============================================================================
// MemoryStore Tests
// =============================================================================

// testEntity implements Entity for testing.
type testEntity struct {
	BaseEntity
	Name string `json:"name"`
}

func TestNewMemoryStore(t *testing.T) {
	store := NewMemoryStore[*testEntity]()
	if store == nil {
		t.Fatal("NewMemoryStore returned nil")
	}
}

func TestMemoryStore_Initialize(t *testing.T) {
	store := NewMemoryStore[*testEntity]()
	ctx := context.Background()

	if err := store.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error: %v", err)
	}

	// Health should pass after initialize
	if err := store.Health(ctx); err != nil {
		t.Errorf("Health() error after Initialize: %v", err)
	}
}

func TestMemoryStore_Close(t *testing.T) {
	store := NewMemoryStore[*testEntity]()
	ctx := context.Background()

	store.Initialize(ctx)

	if err := store.Close(ctx); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	// Health should fail after close
	if err := store.Health(ctx); err == nil {
		t.Error("Health() should error after Close")
	}
}

func TestMemoryStore_CRUD(t *testing.T) {
	store := NewMemoryStore[*testEntity]()
	ctx := context.Background()
	store.Initialize(ctx)
	defer store.Close(ctx)

	entity := &testEntity{
		BaseEntity: BaseEntity{ID: "test-1"},
		Name:       "Test Entity",
	}

	// Create
	if err := store.Create(ctx, entity); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Get
	got, err := store.Get(ctx, "test-1")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got.Name != "Test Entity" {
		t.Errorf("Get() Name = %s, want Test Entity", got.Name)
	}

	// Update
	entity.Name = "Updated Entity"
	if err := store.Update(ctx, entity); err != nil {
		t.Fatalf("Update() error: %v", err)
	}

	got, _ = store.Get(ctx, "test-1")
	if got.Name != "Updated Entity" {
		t.Errorf("after Update() Name = %s, want Updated Entity", got.Name)
	}

	// Delete
	if err := store.Delete(ctx, "test-1"); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	_, err = store.Get(ctx, "test-1")
	if err == nil {
		t.Error("Get() should error after Delete")
	}
}

func TestMemoryStore_Create_Duplicate(t *testing.T) {
	store := NewMemoryStore[*testEntity]()
	ctx := context.Background()
	store.Initialize(ctx)
	defer store.Close(ctx)

	entity := &testEntity{
		BaseEntity: BaseEntity{ID: "test-1"},
		Name:       "Test Entity",
	}

	store.Create(ctx, entity)

	// Duplicate create should fail
	if err := store.Create(ctx, entity); err == nil {
		t.Error("Create() should error for duplicate ID")
	}
}

func TestMemoryStore_Update_NotFound(t *testing.T) {
	store := NewMemoryStore[*testEntity]()
	ctx := context.Background()
	store.Initialize(ctx)
	defer store.Close(ctx)

	entity := &testEntity{
		BaseEntity: BaseEntity{ID: "non-existent"},
		Name:       "Test Entity",
	}

	if err := store.Update(ctx, entity); err == nil {
		t.Error("Update() should error for non-existent entity")
	}
}

func TestMemoryStore_Get_NotFound(t *testing.T) {
	store := NewMemoryStore[*testEntity]()
	ctx := context.Background()
	store.Initialize(ctx)
	defer store.Close(ctx)

	_, err := store.Get(ctx, "non-existent")
	if err == nil {
		t.Error("Get() should error for non-existent entity")
	}
}

func TestMemoryStore_List(t *testing.T) {
	store := NewMemoryStore[*testEntity]()
	ctx := context.Background()
	store.Initialize(ctx)
	defer store.Close(ctx)

	// Empty list
	entities, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(entities) != 0 {
		t.Errorf("List() returned %d entities, want 0", len(entities))
	}

	// Add entities
	for i := 0; i < 5; i++ {
		entity := &testEntity{
			BaseEntity: BaseEntity{ID: "test-" + string(rune('a'+i))},
			Name:       "Test Entity",
		}
		store.Create(ctx, entity)
	}

	entities, err = store.List(ctx)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(entities) != 5 {
		t.Errorf("List() returned %d entities, want 5", len(entities))
	}
}

func TestMemoryStore_Count(t *testing.T) {
	store := NewMemoryStore[*testEntity]()
	ctx := context.Background()
	store.Initialize(ctx)
	defer store.Close(ctx)

	// Empty count
	count, err := store.Count(ctx)
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}
	if count != 0 {
		t.Errorf("Count() = %d, want 0", count)
	}

	// Add entities
	for i := 0; i < 5; i++ {
		entity := &testEntity{
			BaseEntity: BaseEntity{ID: "test-" + string(rune('a'+i))},
			Name:       "Test Entity",
		}
		store.Create(ctx, entity)
	}

	count, err = store.Count(ctx)
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}
	if count != 5 {
		t.Errorf("Count() = %d, want 5", count)
	}
}

func TestMemoryStore_JSONSnapshot(t *testing.T) {
	store := NewMemoryStore[*testEntity]()
	ctx := context.Background()
	store.Initialize(ctx)
	defer store.Close(ctx)

	entity := &testEntity{
		BaseEntity: BaseEntity{ID: "test-1"},
		Name:       "Test Entity",
	}
	store.Create(ctx, entity)

	snapshot, err := store.JSONSnapshot(ctx)
	if err != nil {
		t.Fatalf("JSONSnapshot() error: %v", err)
	}
	if len(snapshot) == 0 {
		t.Error("JSONSnapshot() returned empty data")
	}
}

func TestMemoryStore_NotReady(t *testing.T) {
	store := NewMemoryStore[*testEntity]()
	ctx := context.Background()

	entity := &testEntity{
		BaseEntity: BaseEntity{ID: "test-1"},
		Name:       "Test Entity",
	}

	// All operations should fail when not initialized
	if _, err := store.Get(ctx, "test-1"); err == nil {
		t.Error("Get() should error when not ready")
	}
	if err := store.Create(ctx, entity); err == nil {
		t.Error("Create() should error when not ready")
	}
	if err := store.Update(ctx, entity); err == nil {
		t.Error("Update() should error when not ready")
	}
	if err := store.Delete(ctx, "test-1"); err == nil {
		t.Error("Delete() should error when not ready")
	}
	if _, err := store.List(ctx); err == nil {
		t.Error("List() should error when not ready")
	}
	if _, err := store.Count(ctx); err == nil {
		t.Error("Count() should error when not ready")
	}
	if _, err := store.JSONSnapshot(ctx); err == nil {
		t.Error("JSONSnapshot() should error when not ready")
	}
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	store := NewMemoryStore[*testEntity]()
	ctx := context.Background()
	store.Initialize(ctx)
	defer store.Close(ctx)

	var wg sync.WaitGroup

	// Concurrent creates
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			entity := &testEntity{
				BaseEntity: BaseEntity{ID: "concurrent-" + string(rune(idx))},
				Name:       "Test Entity",
			}
			store.Create(ctx, entity)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.List(ctx)
		}()
	}

	wg.Wait()
}

// =============================================================================
// JSONStore Tests
// =============================================================================

func TestNewJSONStore(t *testing.T) {
	store := NewJSONStore[*testEntity]("/tmp/test.json")
	if store == nil {
		t.Fatal("NewJSONStore returned nil")
	}
}

func TestJSONStore_Initialize(t *testing.T) {
	store := NewJSONStore[*testEntity]("/tmp/test.json")
	ctx := context.Background()

	if err := store.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error: %v", err)
	}

	if err := store.Health(ctx); err != nil {
		t.Errorf("Health() error after Initialize: %v", err)
	}
}

func TestJSONStore_Close(t *testing.T) {
	store := NewJSONStore[*testEntity]("/tmp/test.json")
	ctx := context.Background()

	store.Initialize(ctx)

	if err := store.Close(ctx); err != nil {
		t.Fatalf("Close() error: %v", err)
	}

	if err := store.Health(ctx); err == nil {
		t.Error("Health() should error after Close")
	}
}

// =============================================================================
// QueryOptions Tests
// =============================================================================

func TestDefaultQueryOptions(t *testing.T) {
	opts := DefaultQueryOptions()

	if opts.Offset != 0 {
		t.Errorf("Offset = %d, want 0", opts.Offset)
	}
	if opts.Limit != 100 {
		t.Errorf("Limit = %d, want 100", opts.Limit)
	}
}

// =============================================================================
// PaginatedResult Tests
// =============================================================================

func TestNewPaginatedResult(t *testing.T) {
	items := []string{"a", "b", "c"}
	result := NewPaginatedResult(items, 10, 0, 3)

	if len(result.Items) != 3 {
		t.Errorf("Items length = %d, want 3", len(result.Items))
	}
	if result.Total != 10 {
		t.Errorf("Total = %d, want 10", result.Total)
	}
	if result.Offset != 0 {
		t.Errorf("Offset = %d, want 0", result.Offset)
	}
	if result.Limit != 3 {
		t.Errorf("Limit = %d, want 3", result.Limit)
	}
	if !result.HasMore {
		t.Error("HasMore = false, want true")
	}
}

func TestNewPaginatedResult_NoMore(t *testing.T) {
	items := []string{"a", "b", "c"}
	result := NewPaginatedResult(items, 3, 0, 10)

	if result.HasMore {
		t.Error("HasMore = true, want false")
	}
}

// =============================================================================
// Serialization Tests
// =============================================================================

func TestToJSON(t *testing.T) {
	entity := &testEntity{
		BaseEntity: BaseEntity{ID: "test-1"},
		Name:       "Test Entity",
	}

	data, err := ToJSON(entity)
	if err != nil {
		t.Fatalf("ToJSON() error: %v", err)
	}
	if len(data) == 0 {
		t.Error("ToJSON() returned empty data")
	}
}

func TestFromJSON(t *testing.T) {
	data := []byte(`{"id":"test-1","name":"Test Entity"}`)

	entity, err := FromJSON[testEntity](data)
	if err != nil {
		t.Fatalf("FromJSON() error: %v", err)
	}
	if entity.ID != "test-1" {
		t.Errorf("FromJSON() ID = %s, want test-1", entity.ID)
	}
	if entity.Name != "Test Entity" {
		t.Errorf("FromJSON() Name = %s, want Test Entity", entity.Name)
	}
}

func TestFromJSON_Invalid(t *testing.T) {
	data := []byte(`invalid json`)

	_, err := FromJSON[testEntity](data)
	if err == nil {
		t.Error("FromJSON() should error for invalid JSON")
	}
}
