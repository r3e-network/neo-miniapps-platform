// Package base provides base components for all services.
package base

// StoreType represents the type of store to use.
type StoreType string

const (
	// StoreTypeSupabase uses Supabase PostgreSQL storage.
	StoreTypeSupabase StoreType = "supabase"
)

// GetStoreType returns the configured store type.
// Always returns Supabase - memory storage is not supported.
func GetStoreType() StoreType {
	return StoreTypeSupabase
}

// NewGenericStore creates a Supabase store for the given table.
func NewGenericStore[T Entity](tableName string) *SupabaseStore[T] {
	config := DefaultSupabaseConfig()
	return NewSupabaseStore[T](config, tableName)
}

// StoreConfig holds configuration for store creation.
type StoreConfig struct {
	TableName string
	Supabase  SupabaseConfig
}

// NewStoreWithConfig creates a Supabase store with explicit configuration.
func NewStoreWithConfig[T Entity](config StoreConfig) *SupabaseStore[T] {
	return NewSupabaseStore[T](config.Supabase, config.TableName)
}
