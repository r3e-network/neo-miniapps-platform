// Package vrfchain provides VRF-specific chain interaction.
package vrfchain

import (
	"fmt"

	"github.com/R3E-Network/service_layer/internal/chain"
)

// =============================================================================
// VRF Event Parser Registration
// =============================================================================

func init() {
	chain.RegisterEventParser("vrf", &RequestEventParser{})
	chain.RegisterEventParser("vrf", &FulfilledEventParser{})
}

// =============================================================================
// VRF Service Events
// =============================================================================

// VRFRequestEvent represents a VRFRequest event from VRFService.
// Event: VRFRequest(requestId, userContract, seed, numWords)
type VRFRequestEvent struct {
	RequestID    uint64
	UserContract string
	Seed         []byte
	NumWords     uint64
}

// RequestEventParser parses VRFRequest events.
type RequestEventParser struct{}

// CanParse returns true if this parser can handle the event.
func (p *RequestEventParser) CanParse(event *chain.ContractEvent) bool {
	return event.EventName == "VRFRequest"
}

// Parse parses the event.
func (p *RequestEventParser) Parse(event *chain.ContractEvent) (interface{}, error) {
	return ParseVRFRequestEvent(event)
}

// ParseVRFRequestEvent parses a VRFRequest event.
func ParseVRFRequestEvent(event *chain.ContractEvent) (*VRFRequestEvent, error) {
	if event.EventName != "VRFRequest" {
		return nil, fmt.Errorf("not a VRFRequest event")
	}
	if len(event.State) < 4 {
		return nil, fmt.Errorf("invalid event state: expected 4 items, got %d", len(event.State))
	}

	requestID, err := chain.ParseInteger(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse requestId: %w", err)
	}

	userContract, err := chain.ParseHash160(event.State[1])
	if err != nil {
		return nil, fmt.Errorf("parse userContract: %w", err)
	}

	seed, err := chain.ParseByteArray(event.State[2])
	if err != nil {
		return nil, fmt.Errorf("parse seed: %w", err)
	}

	numWords, err := chain.ParseInteger(event.State[3])
	if err != nil {
		return nil, fmt.Errorf("parse numWords: %w", err)
	}

	return &VRFRequestEvent{
		RequestID:    requestID.Uint64(),
		UserContract: userContract,
		Seed:         seed,
		NumWords:     numWords.Uint64(),
	}, nil
}

// VRFFulfilledEvent represents a VRFFulfilled event.
// Event: VRFFulfilled(requestId, randomWords, proof)
type VRFFulfilledEvent struct {
	RequestID   uint64
	RandomWords []byte
	Proof       []byte
}

// FulfilledEventParser parses VRFFulfilled events.
type FulfilledEventParser struct{}

// CanParse returns true if this parser can handle the event.
func (p *FulfilledEventParser) CanParse(event *chain.ContractEvent) bool {
	return event.EventName == "VRFFulfilled"
}

// Parse parses the event.
func (p *FulfilledEventParser) Parse(event *chain.ContractEvent) (interface{}, error) {
	return ParseVRFFulfilledEvent(event)
}

// ParseVRFFulfilledEvent parses a VRFFulfilled event.
func ParseVRFFulfilledEvent(event *chain.ContractEvent) (*VRFFulfilledEvent, error) {
	if event.EventName != "VRFFulfilled" {
		return nil, fmt.Errorf("not a VRFFulfilled event")
	}
	if len(event.State) < 3 {
		return nil, fmt.Errorf("invalid event state")
	}

	requestID, err := chain.ParseInteger(event.State[0])
	if err != nil {
		return nil, err
	}

	randomWords, err := chain.ParseByteArray(event.State[1])
	if err != nil {
		return nil, err
	}

	proof, err := chain.ParseByteArray(event.State[2])
	if err != nil {
		return nil, err
	}

	return &VRFFulfilledEvent{
		RequestID:   requestID.Uint64(),
		RandomWords: randomWords,
		Proof:       proof,
	}, nil
}
