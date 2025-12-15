package httputil

import (
	"errors"
	"strings"
	"testing"
)

func TestReadAllWithLimit(t *testing.T) {
	got, truncated, err := ReadAllWithLimit(strings.NewReader("hello world"), 5)
	if err != nil {
		t.Fatalf("ReadAllWithLimit() error = %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("ReadAllWithLimit() = %q, want %q", string(got), "hello")
	}
	if !truncated {
		t.Fatal("ReadAllWithLimit() truncated = false, want true")
	}
}

func TestReadAllWithLimit_NoTruncation(t *testing.T) {
	got, truncated, err := ReadAllWithLimit(strings.NewReader("hello"), 5)
	if err != nil {
		t.Fatalf("ReadAllWithLimit() error = %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("ReadAllWithLimit() = %q, want %q", string(got), "hello")
	}
	if truncated {
		t.Fatal("ReadAllWithLimit() truncated = true, want false")
	}
}

func TestReadAllStrict_Truncates(t *testing.T) {
	_, err := ReadAllStrict(strings.NewReader("hello world"), 5)
	if err == nil {
		t.Fatal("ReadAllStrict() error = nil, want error")
	}
	var tooLarge *BodyTooLargeError
	if !errors.As(err, &tooLarge) {
		t.Fatalf("ReadAllStrict() error = %T, want *BodyTooLargeError", err)
	}
	if tooLarge.Limit != 5 {
		t.Fatalf("BodyTooLargeError.Limit = %d, want 5", tooLarge.Limit)
	}
}

func TestReadAllWithLimit_NilReader(t *testing.T) {
	_, _, err := ReadAllWithLimit(nil, 5)
	if err == nil {
		t.Fatal("ReadAllWithLimit() error = nil, want error")
	}
}
