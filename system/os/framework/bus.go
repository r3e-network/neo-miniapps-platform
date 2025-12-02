package framework

import (
	"context"
	"encoding/json"
	"fmt"
)

// BusClient wraps publish/push/invoke helpers provided by the service engine.
// Services can depend on this interface instead of the concrete engine.
type BusClient interface {
	PublishEvent(ctx context.Context, event string, payload any) error
	PushData(ctx context.Context, topic string, payload any) error
	InvokeCompute(ctx context.Context, payload any) ([]ComputeResult, error)
}

// ComputeResult mirrors engine.InvokeResult without importing engine here.
type ComputeResult struct {
	Module string
	Result any
	Err    error
}

// Success returns true if the compute result has no error.
func (r ComputeResult) Success() bool {
	return r.Err == nil
}

// Failed returns true if the compute result has an error.
func (r ComputeResult) Failed() bool {
	return r.Err != nil
}

// Error returns the error message or empty string if successful.
func (r ComputeResult) Error() string {
	if r.Err == nil {
		return ""
	}
	return r.Err.Error()
}

// ResultAs attempts to unmarshal/convert the result to the given type.
// Returns an error if the conversion fails.
func (r ComputeResult) ResultAs(target any) error {
	if r.Err != nil {
		return r.Err
	}
	if r.Result == nil {
		return nil
	}

	// If target is same type, try direct assignment
	switch t := target.(type) {
	case *string:
		if s, ok := r.Result.(string); ok {
			*t = s
			return nil
		}
	case *int:
		if i, ok := r.Result.(int); ok {
			*t = i
			return nil
		}
	case *int64:
		if i, ok := r.Result.(int64); ok {
			*t = i
			return nil
		}
		// Handle float64 from JSON unmarshaling
		if f, ok := r.Result.(float64); ok {
			*t = int64(f)
			return nil
		}
	case *float64:
		if f, ok := r.Result.(float64); ok {
			*t = f
			return nil
		}
	case *bool:
		if b, ok := r.Result.(bool); ok {
			*t = b
			return nil
		}
	case *map[string]any:
		if m, ok := r.Result.(map[string]any); ok {
			*t = m
			return nil
		}
	case *[]any:
		if a, ok := r.Result.([]any); ok {
			*t = a
			return nil
		}
	}

	// Try JSON round-trip for complex types
	data, err := json.Marshal(r.Result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to unmarshal result: %w", err)
	}
	return nil
}

// MustResultAs is like ResultAs but panics on error.
func (r ComputeResult) MustResultAs(target any) {
	if err := r.ResultAs(target); err != nil {
		panic(fmt.Sprintf("MustResultAs failed: %v", err))
	}
}

// String returns a human-readable representation of the result.
func (r ComputeResult) String() string {
	if r.Err != nil {
		return fmt.Sprintf("ComputeResult{Module: %q, Error: %q}", r.Module, r.Err.Error())
	}
	return fmt.Sprintf("ComputeResult{Module: %q, Result: %v}", r.Module, r.Result)
}

// ComputeResults is a slice of ComputeResult with helper methods.
type ComputeResults []ComputeResult

// AllSuccessful returns true if all results are successful.
func (rs ComputeResults) AllSuccessful() bool {
	for _, r := range rs {
		if r.Failed() {
			return false
		}
	}
	return true
}

// AnyFailed returns true if any result has an error.
func (rs ComputeResults) AnyFailed() bool {
	for _, r := range rs {
		if r.Failed() {
			return true
		}
	}
	return false
}

// Successful returns only the successful results.
func (rs ComputeResults) Successful() ComputeResults {
	var result ComputeResults
	for _, r := range rs {
		if r.Success() {
			result = append(result, r)
		}
	}
	return result
}

// Failed returns only the failed results.
func (rs ComputeResults) Failed() ComputeResults {
	var result ComputeResults
	for _, r := range rs {
		if r.Failed() {
			result = append(result, r)
		}
	}
	return result
}

// ByModule returns the result for a specific module, or nil if not found.
func (rs ComputeResults) ByModule(module string) *ComputeResult {
	for i := range rs {
		if rs[i].Module == module {
			return &rs[i]
		}
	}
	return nil
}

// Modules returns a list of all module names in the results.
func (rs ComputeResults) Modules() []string {
	modules := make([]string, len(rs))
	for i, r := range rs {
		modules[i] = r.Module
	}
	return modules
}

// FirstError returns the first error found, or nil if all successful.
func (rs ComputeResults) FirstError() error {
	for _, r := range rs {
		if r.Err != nil {
			return r.Err
		}
	}
	return nil
}

// Errors returns all errors from failed results.
func (rs ComputeResults) Errors() []error {
	var errs []error
	for _, r := range rs {
		if r.Err != nil {
			errs = append(errs, r.Err)
		}
	}
	return errs
}

// Count returns the total number of results.
func (rs ComputeResults) Count() int {
	return len(rs)
}

// SuccessCount returns the number of successful results.
func (rs ComputeResults) SuccessCount() int {
	count := 0
	for _, r := range rs {
		if r.Success() {
			count++
		}
	}
	return count
}

// FailedCount returns the number of failed results.
func (rs ComputeResults) FailedCount() int {
	count := 0
	for _, r := range rs {
		if r.Failed() {
			count++
		}
	}
	return count
}

// NewComputeResult creates a successful ComputeResult.
func NewComputeResult(module string, result any) ComputeResult {
	return ComputeResult{
		Module: module,
		Result: result,
	}
}

// NewComputeResultError creates a failed ComputeResult.
func NewComputeResultError(module string, err error) ComputeResult {
	return ComputeResult{
		Module: module,
		Err:    err,
	}
}
