// Package chain provides event parser helpers for contract events.
package chain

import (
	"fmt"
)

// =============================================================================
// Simple Event Parser Helper
// =============================================================================

// SimpleEventParser is a helper that creates EventParser implementations
// from a simple parse function. This reduces boilerplate when registering
// event parsers.
//
// Usage:
//
//	func init() {
//	    chain.RegisterEventParser("service",
//	        chain.NewEventParser("EventName", func(e *chain.ContractEvent) (any, error) {
//	            return ParseMyEvent(e)
//	        }))
//	}
type SimpleEventParser struct {
	eventName string
	parseFn   func(*ContractEvent) (any, error)
}

// NewEventParser creates a simple event parser for a specific event name.
func NewEventParser(eventName string, parseFn func(*ContractEvent) (any, error)) EventParser {
	return &SimpleEventParser{
		eventName: eventName,
		parseFn:   parseFn,
	}
}

// CanParse returns true if the event name matches.
func (p *SimpleEventParser) CanParse(event *ContractEvent) bool {
	return event.EventName == p.eventName
}

// Parse delegates to the parse function.
func (p *SimpleEventParser) Parse(event *ContractEvent) (interface{}, error) {
	return p.parseFn(event)
}

// =============================================================================
// Event Parsing Helpers
// =============================================================================

// EventState provides helper methods for parsing event state items.
type EventState struct {
	State []StackItem
}

// NewEventState creates an EventState helper from a ContractEvent.
func NewEventState(event *ContractEvent) *EventState {
	return &EventState{State: event.State}
}

// RequireMinItems validates the minimum number of state items.
func (e *EventState) RequireMinItems(minItems int, eventName string) error {
	if len(e.State) < minItems {
		return fmt.Errorf("%s: expected at least %d items, got %d", eventName, minItems, len(e.State))
	}
	return nil
}

// Integer parses the item at index as *big.Int.
func (e *EventState) Integer(index int, fieldName string) (uint64, error) {
	if index >= len(e.State) {
		return 0, fmt.Errorf("index %d out of range for %s", index, fieldName)
	}
	val, err := ParseInteger(e.State[index])
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", fieldName, err)
	}
	return val.Uint64(), nil
}

// String parses the item at index as string.
func (e *EventState) String(index int, fieldName string) (string, error) {
	if index >= len(e.State) {
		return "", fmt.Errorf("index %d out of range for %s", index, fieldName)
	}
	val, err := ParseStringFromItem(e.State[index])
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", fieldName, err)
	}
	return val, nil
}

// Hash160 parses the item at index as a hash160 (address).
func (e *EventState) Hash160(index int, fieldName string) (string, error) {
	if index >= len(e.State) {
		return "", fmt.Errorf("index %d out of range for %s", index, fieldName)
	}
	val, err := ParseHash160(e.State[index])
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", fieldName, err)
	}
	return val, nil
}

// ByteArray parses the item at index as []byte.
func (e *EventState) ByteArray(index int, fieldName string) ([]byte, error) {
	if index >= len(e.State) {
		return nil, fmt.Errorf("index %d out of range for %s", index, fieldName)
	}
	val, err := ParseByteArray(e.State[index])
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", fieldName, err)
	}
	return val, nil
}

// Boolean parses the item at index as bool.
func (e *EventState) Boolean(index int, fieldName string) (bool, error) {
	if index >= len(e.State) {
		return false, fmt.Errorf("index %d out of range for %s", index, fieldName)
	}
	val, err := ParseBoolean(e.State[index])
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", fieldName, err)
	}
	return val, nil
}

// =============================================================================
// Multi-Event Parser
// =============================================================================

// MultiEventParser combines multiple event parsers into one.
// Events are routed to the appropriate parser based on event name.
type MultiEventParser struct {
	parsers map[string]func(*ContractEvent) (any, error)
}

// NewMultiEventParser creates a multi-event parser.
func NewMultiEventParser() *MultiEventParser {
	return &MultiEventParser{
		parsers: make(map[string]func(*ContractEvent) (any, error)),
	}
}

// Register adds a parser for a specific event name.
func (m *MultiEventParser) Register(eventName string, parseFn func(*ContractEvent) (any, error)) *MultiEventParser {
	m.parsers[eventName] = parseFn
	return m
}

// CanParse returns true if any registered parser can handle the event.
func (m *MultiEventParser) CanParse(event *ContractEvent) bool {
	_, ok := m.parsers[event.EventName]
	return ok
}

// Parse delegates to the appropriate parser.
func (m *MultiEventParser) Parse(event *ContractEvent) (interface{}, error) {
	if parseFn, ok := m.parsers[event.EventName]; ok {
		return parseFn(event)
	}
	return nil, fmt.Errorf("no parser for event: %s", event.EventName)
}
