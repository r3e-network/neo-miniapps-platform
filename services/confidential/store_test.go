package confidential

import (
	"context"
	"testing"

	"github.com/R3E-Network/service_layer/services/base"
)

// Basic store init test to ensure Supabase config is required.
func TestStoreInitializeRequiresConfig(t *testing.T) {
	store := &Store{
		requests: base.NewSupabaseStore[*ComputeRequest](base.SupabaseConfig{}, "secrets"),
	}
	if err := store.Initialize(context.Background()); err == nil {
		t.Fatalf("expected error on missing Supabase URL/service key")
	}
}
