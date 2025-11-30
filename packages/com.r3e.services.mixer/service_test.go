package mixer

import (
	"context"
	"testing"
)

func TestService_CreateMixRequest(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_GetMixRequest(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_ListMixRequests(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_ConfirmDeposit(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_StartMixing(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_CompleteMixRequest(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_CreateWithdrawalClaim(t *testing.T) {
	t.Skipf("test requires database; run with integration test suite")
}

func TestService_Lifecycle(t *testing.T) {
	svc := New(nil, nil, nil, nil, nil)
	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := svc.Ready(context.Background()); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := svc.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

func TestService_Manifest(t *testing.T) {
	svc := New(nil, nil, nil, nil, nil)
	m := svc.Manifest()
	if m.Name != "mixer" {
		t.Fatalf("expected name mixer, got %s", m.Name)
	}
}

func TestService_Descriptor(t *testing.T) {
	svc := New(nil, nil, nil, nil, nil)
	d := svc.Descriptor()
	if d.Name != "mixer" {
		t.Fatalf("expected name mixer, got %s", d.Name)
	}
}

func TestService_Domain(t *testing.T) {
	svc := New(nil, nil, nil, nil, nil)
	if svc.Domain() != "mixer" {
		t.Fatalf("expected domain mixer")
	}
}

func TestMixDuration_ToDuration(t *testing.T) {
	tests := []struct {
		input    MixDuration
		expected string
	}{
		{MixDuration30Min, "30m0s"},
		{MixDuration1Hour, "1h0m0s"},
		{MixDuration24Hour, "24h0m0s"},
		{MixDuration7Day, "168h0m0s"},
	}

	for _, tt := range tests {
		got := tt.input.ToDuration().String()
		if got != tt.expected {
			t.Errorf("MixDuration(%s).ToDuration() = %s, want %s", tt.input, got, tt.expected)
		}
	}
}

func TestParseMixDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected MixDuration
	}{
		{"30m", MixDuration30Min},
		{"30min", MixDuration30Min},
		{"1h", MixDuration1Hour},
		{"1hour", MixDuration1Hour},
		{"24h", MixDuration24Hour},
		{"1d", MixDuration24Hour},
		{"7d", MixDuration7Day},
		{"7day", MixDuration7Day},
		{"unknown", MixDuration1Hour}, // default
	}

	for _, tt := range tests {
		got := ParseMixDuration(tt.input)
		if got != tt.expected {
			t.Errorf("ParseMixDuration(%s) = %s, want %s", tt.input, got, tt.expected)
		}
	}
}
