package gasbank

import (
	"context"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/services/base"
)

// Ensures gasbank store initialize path handles missing Supabase gracefully.
func TestStoreInitializeWithSupabaseConfig(t *testing.T) {
	cfg := base.SupabaseConfig{
		URL:        "http://localhost:54321",
		ServiceKey: "service-key",
	}
	store := NewStoreWithConfig(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := store.Initialize(ctx); err == nil {
		t.Log("store initialized (unexpected with dummy Supabase); ensure Supabase is reachable for full integration")
	}
}
