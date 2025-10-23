package account

import (
	"testing"
	"time"
)

func TestAccountModelAllowsMetadata(t *testing.T) {
	now := time.Now()
	acct := Account{
		ID:        "acct-1",
		Owner:     "owner",
		Metadata:  map[string]string{"env": "prod"},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if acct.Metadata["env"] != "prod" {
		t.Fatalf("expected metadata to persist, got %#v", acct.Metadata)
	}
	if !acct.CreatedAt.Equal(now) || !acct.UpdatedAt.Equal(now) {
		t.Fatalf("expected timestamps to be preserved")
	}
}
