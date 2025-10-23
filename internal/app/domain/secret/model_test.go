package secret

import (
	"testing"
	"time"
)

func TestSecretToMetadata(t *testing.T) {
	now := time.Now()
	sec := Secret{
		ID:        "secret-1",
		AccountID: "acct-1",
		Name:      "api-key",
		Version:   2,
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now,
	}

	meta := sec.ToMetadata()
	if meta.ID != sec.ID || meta.AccountID != sec.AccountID {
		t.Fatalf("metadata fields mismatch")
	}
	if meta.Version != 2 {
		t.Fatalf("expected version to propagate")
	}
}
