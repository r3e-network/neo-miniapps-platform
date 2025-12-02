package httpapi

import (
	"testing"

	core "github.com/R3E-Network/service_layer/system/os/framework/core"
)

func TestParseLimitParam_DefaultAndClamp(t *testing.T) {
	limit, err := parseLimitParam("", 50)
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if limit != 50 {
		t.Fatalf("expected default 50, got %d", limit)
	}

	limit, err = parseLimitParam("9999", 10)
	if err != nil {
		t.Fatalf("expected clamp to succeed: %v", err)
	}
	if limit != core.MaxListLimit {
		t.Fatalf("expected clamped limit %d, got %d", core.MaxListLimit, limit)
	}
}

func TestParseLimitParam_Invalid(t *testing.T) {
	if _, err := parseLimitParam("abc", 0); err == nil {
		t.Fatalf("expected error for non-numeric limit")
	}
	if _, err := parseLimitParam("0", 0); err == nil {
		t.Fatalf("expected error for non-positive limit")
	}
}
