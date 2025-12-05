package base

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// Test that setHeaders prefers access token from context for RLS.
func TestSupabaseStoreSetHeadersUsesAccessTokenFromContext(t *testing.T) {
	store := NewSupabaseStore[*mockEntity](SupabaseConfig{
		URL:        "http://example.com",
		ServiceKey: "service-key",
	}, "mock")

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	ctx := WithAccessToken(context.Background(), "user-token")

	store.setHeaders(ctx, req)

	if got := req.Header.Get("Authorization"); got != "Bearer user-token" {
		t.Fatalf("expected Authorization with user token, got %s", got)
	}
	if got := req.Header.Get("apikey"); got != "user-token" {
		t.Fatalf("expected apikey with user token, got %s", got)
	}
}

type mockEntity struct {
	BaseEntity
}

func (m *mockEntity) GetID() string           { return m.ID }
func (m *mockEntity) GetCreatedAt() time.Time { return time.Time{} }
func (m *mockEntity) GetUpdatedAt() time.Time { return time.Time{} }
