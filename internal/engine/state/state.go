// Package state provides unified service state definitions shared across
// the engine and framework. This ensures consistent state semantics throughout
// the service layer runtime.
package state

import (
	"encoding/json"
	"fmt"
)

// Status represents the lifecycle status of a module.
type Status int32

const (
	// StatusUnknown indicates an uninitialized or unknown state.
	StatusUnknown Status = iota

	// StatusRegistered indicates the module has been registered but not started.
	StatusRegistered

	// StatusStarting indicates the module is in the process of starting.
	StatusStarting

	// StatusRunning indicates the module has started successfully and is running.
	StatusRunning

	// StatusStopping indicates the module is in the process of stopping.
	StatusStopping

	// StatusStopped indicates the module has stopped cleanly.
	StatusStopped

	// StatusFailed indicates the module failed during start or while running.
	StatusFailed

	// StatusStopFailed indicates the module failed during stop.
	StatusStopFailed
)

// String returns the string representation of the status.
func (s Status) String() string {
	switch s {
	case StatusUnknown:
		return "unknown"
	case StatusRegistered:
		return "registered"
	case StatusStarting:
		return "starting"
	case StatusRunning:
		return "running"
	case StatusStopping:
		return "stopping"
	case StatusStopped:
		return "stopped"
	case StatusFailed:
		return "failed"
	case StatusStopFailed:
		return "stop-failed"
	default:
		return fmt.Sprintf("status(%d)", s)
	}
}

// MarshalJSON implements json.Marshaler.
func (s Status) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// UnmarshalJSON implements json.Unmarshaler.
func (s *Status) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	*s = ParseStatus(str)
	return nil
}

// ParseStatus converts a string to Status.
func ParseStatus(s string) Status {
	switch s {
	case "unknown":
		return StatusUnknown
	case "registered":
		return StatusRegistered
	case "starting":
		return StatusStarting
	case "running", "started", "ready": // Accept legacy aliases
		return StatusRunning
	case "stopping":
		return StatusStopping
	case "stopped":
		return StatusStopped
	case "failed":
		return StatusFailed
	case "stop-failed", "stop-error":
		return StatusStopFailed
	default:
		return StatusUnknown
	}
}

// IsTerminal returns true if this status represents a terminal state.
func (s Status) IsTerminal() bool {
	return s == StatusStopped || s == StatusFailed || s == StatusStopFailed
}

// IsHealthy returns true if this status represents a healthy state.
func (s Status) IsHealthy() bool {
	return s == StatusRunning
}

// CanStart returns true if the module can be started from this status.
func (s Status) CanStart() bool {
	return s == StatusUnknown || s == StatusRegistered || s == StatusStopped || s == StatusFailed || s == StatusStopFailed
}

// CanStop returns true if the module can be stopped from this status.
func (s Status) CanStop() bool {
	return s == StatusRunning || s == StatusStarting || s == StatusFailed
}

// Readiness represents the readiness state of a module.
type Readiness int32

const (
	// ReadinessUnknown indicates readiness has not been determined.
	ReadinessUnknown Readiness = iota

	// ReadinessReady indicates the module is ready to serve traffic.
	ReadinessReady

	// ReadinessNotReady indicates the module is not ready to serve traffic.
	ReadinessNotReady
)

// String returns the string representation of readiness.
func (r Readiness) String() string {
	switch r {
	case ReadinessUnknown:
		return "unknown"
	case ReadinessReady:
		return "ready"
	case ReadinessNotReady:
		return "not-ready"
	default:
		return fmt.Sprintf("readiness(%d)", r)
	}
}

// MarshalJSON implements json.Marshaler.
func (r Readiness) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.String())
}

// UnmarshalJSON implements json.Unmarshaler.
func (r *Readiness) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	*r = ParseReadiness(str)
	return nil
}

// ParseReadiness converts a string to Readiness.
func ParseReadiness(s string) Readiness {
	switch s {
	case "unknown":
		return ReadinessUnknown
	case "ready":
		return ReadinessReady
	case "not-ready", "notready":
		return ReadinessNotReady
	default:
		return ReadinessUnknown
	}
}

// IsReady returns true if the readiness state is ready.
func (r Readiness) IsReady() bool {
	return r == ReadinessReady
}

// Health represents the combined health state of a module.
type Health struct {
	Status    Status    `json:"status"`
	Readiness Readiness `json:"readiness"`
	Error     string    `json:"error,omitempty"`
}

// IsHealthy returns true if both status and readiness indicate health.
func (h Health) IsHealthy() bool {
	return h.Status.IsHealthy() && h.Readiness.IsReady()
}

// NewHealth creates a new Health with the given status and readiness.
func NewHealth(status Status, readiness Readiness) Health {
	return Health{
		Status:    status,
		Readiness: readiness,
	}
}

// NewHealthWithError creates a new Health with error message.
func NewHealthWithError(status Status, readiness Readiness, err string) Health {
	return Health{
		Status:    status,
		Readiness: readiness,
		Error:     err,
	}
}

// ValidTransitions defines allowed state transitions.
var ValidTransitions = map[Status][]Status{
	StatusUnknown:    {StatusRegistered},
	StatusRegistered: {StatusStarting},
	StatusStarting:   {StatusRunning, StatusFailed},
	StatusRunning:    {StatusStopping, StatusFailed},
	StatusStopping:   {StatusStopped, StatusStopFailed},
	StatusStopped:    {StatusStarting},
	StatusFailed:     {StatusStarting, StatusStopping},
	StatusStopFailed: {StatusStarting},
}

// CanTransition returns true if the transition from -> to is valid.
func CanTransition(from, to Status) bool {
	allowed, ok := ValidTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// TransitionError represents an invalid state transition.
type TransitionError struct {
	From Status
	To   Status
}

// Error implements error.
func (e TransitionError) Error() string {
	return fmt.Sprintf("invalid state transition: %s -> %s", e.From, e.To)
}

// NewTransitionError creates a new TransitionError.
func NewTransitionError(from, to Status) TransitionError {
	return TransitionError{From: from, To: to}
}
