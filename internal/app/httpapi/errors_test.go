package httpapi

import (
	"errors"
	"testing"
)

func TestErrorVariables(t *testing.T) {
	// Test that errors are properly defined
	if ErrNeoUnavailable == nil {
		t.Fatal("ErrNeoUnavailable should not be nil")
	}
	if ErrInvalidHeight == nil {
		t.Fatal("ErrInvalidHeight should not be nil")
	}
	if ErrMissingHeight == nil {
		t.Fatal("ErrMissingHeight should not be nil")
	}

	// Test error messages
	if ErrNeoUnavailable.Error() != "neo indexer not configured" {
		t.Fatalf("unexpected error message: %s", ErrNeoUnavailable.Error())
	}
	if ErrInvalidHeight.Error() != "height must be a positive integer" {
		t.Fatalf("unexpected error message: %s", ErrInvalidHeight.Error())
	}
	if ErrMissingHeight.Error() != "height path parameter required" {
		t.Fatalf("unexpected error message: %s", ErrMissingHeight.Error())
	}

	// Test that errors are distinct
	if errors.Is(ErrNeoUnavailable, ErrInvalidHeight) {
		t.Fatal("errors should be distinct")
	}
	if errors.Is(ErrInvalidHeight, ErrMissingHeight) {
		t.Fatal("errors should be distinct")
	}
}
