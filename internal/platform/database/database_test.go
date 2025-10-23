package database

import (
	"context"
	"testing"
)

func TestOpenRequiresDSN(t *testing.T) {
	if _, err := Open(context.Background(), " "); err == nil {
		t.Fatalf("expected error when DSN empty")
	}
}
