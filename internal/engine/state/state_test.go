package state

import (
	"encoding/json"
	"testing"
)

func TestStatus_String(t *testing.T) {
	tests := []struct {
		status   Status
		expected string
	}{
		{StatusUnknown, "unknown"},
		{StatusRegistered, "registered"},
		{StatusStarting, "starting"},
		{StatusRunning, "running"},
		{StatusStopping, "stopping"},
		{StatusStopped, "stopped"},
		{StatusFailed, "failed"},
		{StatusStopFailed, "stop-failed"},
		{Status(99), "status(99)"},
	}

	for _, tc := range tests {
		if got := tc.status.String(); got != tc.expected {
			t.Errorf("Status(%d).String() = %q, want %q", tc.status, got, tc.expected)
		}
	}
}

func TestParseStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected Status
	}{
		{"unknown", StatusUnknown},
		{"registered", StatusRegistered},
		{"starting", StatusStarting},
		{"running", StatusRunning},
		{"started", StatusRunning},  // legacy alias
		{"ready", StatusRunning},    // legacy alias
		{"stopping", StatusStopping},
		{"stopped", StatusStopped},
		{"failed", StatusFailed},
		{"stop-failed", StatusStopFailed},
		{"stop-error", StatusStopFailed}, // legacy alias
		{"invalid", StatusUnknown},
	}

	for _, tc := range tests {
		if got := ParseStatus(tc.input); got != tc.expected {
			t.Errorf("ParseStatus(%q) = %v, want %v", tc.input, got, tc.expected)
		}
	}
}

func TestStatus_JSON(t *testing.T) {
	original := StatusRunning
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	if string(data) != `"running"` {
		t.Errorf("Marshal = %s, want \"running\"", data)
	}

	var parsed Status
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if parsed != original {
		t.Errorf("Unmarshal = %v, want %v", parsed, original)
	}
}

func TestStatus_Predicates(t *testing.T) {
	t.Run("IsTerminal", func(t *testing.T) {
		terminal := []Status{StatusStopped, StatusFailed, StatusStopFailed}
		nonTerminal := []Status{StatusUnknown, StatusRegistered, StatusStarting, StatusRunning, StatusStopping}

		for _, s := range terminal {
			if !s.IsTerminal() {
				t.Errorf("%v.IsTerminal() = false, want true", s)
			}
		}
		for _, s := range nonTerminal {
			if s.IsTerminal() {
				t.Errorf("%v.IsTerminal() = true, want false", s)
			}
		}
	})

	t.Run("IsHealthy", func(t *testing.T) {
		if !StatusRunning.IsHealthy() {
			t.Error("StatusRunning.IsHealthy() = false, want true")
		}
		for _, s := range []Status{StatusUnknown, StatusRegistered, StatusStarting, StatusStopping, StatusStopped, StatusFailed} {
			if s.IsHealthy() {
				t.Errorf("%v.IsHealthy() = true, want false", s)
			}
		}
	})

	t.Run("CanStart", func(t *testing.T) {
		canStart := []Status{StatusUnknown, StatusRegistered, StatusStopped, StatusFailed, StatusStopFailed}
		cannotStart := []Status{StatusStarting, StatusRunning, StatusStopping}

		for _, s := range canStart {
			if !s.CanStart() {
				t.Errorf("%v.CanStart() = false, want true", s)
			}
		}
		for _, s := range cannotStart {
			if s.CanStart() {
				t.Errorf("%v.CanStart() = true, want false", s)
			}
		}
	})

	t.Run("CanStop", func(t *testing.T) {
		canStop := []Status{StatusRunning, StatusStarting, StatusFailed}
		cannotStop := []Status{StatusUnknown, StatusRegistered, StatusStopping, StatusStopped, StatusStopFailed}

		for _, s := range canStop {
			if !s.CanStop() {
				t.Errorf("%v.CanStop() = false, want true", s)
			}
		}
		for _, s := range cannotStop {
			if s.CanStop() {
				t.Errorf("%v.CanStop() = true, want false", s)
			}
		}
	})
}

func TestReadiness_String(t *testing.T) {
	tests := []struct {
		readiness Readiness
		expected  string
	}{
		{ReadinessUnknown, "unknown"},
		{ReadinessReady, "ready"},
		{ReadinessNotReady, "not-ready"},
		{Readiness(99), "readiness(99)"},
	}

	for _, tc := range tests {
		if got := tc.readiness.String(); got != tc.expected {
			t.Errorf("Readiness(%d).String() = %q, want %q", tc.readiness, got, tc.expected)
		}
	}
}

func TestParseReadiness(t *testing.T) {
	tests := []struct {
		input    string
		expected Readiness
	}{
		{"unknown", ReadinessUnknown},
		{"ready", ReadinessReady},
		{"not-ready", ReadinessNotReady},
		{"notready", ReadinessNotReady}, // alternate
		{"invalid", ReadinessUnknown},
	}

	for _, tc := range tests {
		if got := ParseReadiness(tc.input); got != tc.expected {
			t.Errorf("ParseReadiness(%q) = %v, want %v", tc.input, got, tc.expected)
		}
	}
}

func TestReadiness_JSON(t *testing.T) {
	original := ReadinessReady
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	if string(data) != `"ready"` {
		t.Errorf("Marshal = %s, want \"ready\"", data)
	}

	var parsed Readiness
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if parsed != original {
		t.Errorf("Unmarshal = %v, want %v", parsed, original)
	}
}

func TestReadiness_IsReady(t *testing.T) {
	if !ReadinessReady.IsReady() {
		t.Error("ReadinessReady.IsReady() = false, want true")
	}
	if ReadinessNotReady.IsReady() {
		t.Error("ReadinessNotReady.IsReady() = true, want false")
	}
	if ReadinessUnknown.IsReady() {
		t.Error("ReadinessUnknown.IsReady() = true, want false")
	}
}

func TestHealth_IsHealthy(t *testing.T) {
	tests := []struct {
		health   Health
		expected bool
	}{
		{NewHealth(StatusRunning, ReadinessReady), true},
		{NewHealth(StatusRunning, ReadinessNotReady), false},
		{NewHealth(StatusStopped, ReadinessReady), false},
		{NewHealth(StatusFailed, ReadinessNotReady), false},
	}

	for _, tc := range tests {
		if got := tc.health.IsHealthy(); got != tc.expected {
			t.Errorf("Health{%v, %v}.IsHealthy() = %v, want %v",
				tc.health.Status, tc.health.Readiness, got, tc.expected)
		}
	}
}

func TestCanTransition(t *testing.T) {
	valid := []struct {
		from, to Status
	}{
		{StatusUnknown, StatusRegistered},
		{StatusRegistered, StatusStarting},
		{StatusStarting, StatusRunning},
		{StatusStarting, StatusFailed},
		{StatusRunning, StatusStopping},
		{StatusRunning, StatusFailed},
		{StatusStopping, StatusStopped},
		{StatusStopping, StatusStopFailed},
		{StatusStopped, StatusStarting},
		{StatusFailed, StatusStarting},
		{StatusFailed, StatusStopping},
		{StatusStopFailed, StatusStarting},
	}

	for _, tc := range valid {
		if !CanTransition(tc.from, tc.to) {
			t.Errorf("CanTransition(%v, %v) = false, want true", tc.from, tc.to)
		}
	}

	invalid := []struct {
		from, to Status
	}{
		{StatusUnknown, StatusRunning},
		{StatusRegistered, StatusRunning},
		{StatusRunning, StatusRegistered},
		{StatusStopped, StatusRunning},
		{StatusStopFailed, StatusStopped},
	}

	for _, tc := range invalid {
		if CanTransition(tc.from, tc.to) {
			t.Errorf("CanTransition(%v, %v) = true, want false", tc.from, tc.to)
		}
	}
}

func TestTransitionError(t *testing.T) {
	err := NewTransitionError(StatusStopped, StatusRunning)
	expected := "invalid state transition: stopped -> running"
	if err.Error() != expected {
		t.Errorf("TransitionError.Error() = %q, want %q", err.Error(), expected)
	}
}
