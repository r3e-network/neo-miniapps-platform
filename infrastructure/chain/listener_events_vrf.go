package chain

import "fmt"

// =============================================================================
// VRF Events (VRFService contract)
// =============================================================================

// VRFRequestEvent represents a VRFRequest event from VRFService.
// Event: VRFRequest(requestId, userContract, seed, numWords)
type VRFRequestEvent struct {
	RequestID    uint64
	UserContract string
	Seed         []byte
	NumWords     uint64
}

// ParseVRFRequestEvent parses a VRFRequest event.
func ParseVRFRequestEvent(event *ContractEvent) (*VRFRequestEvent, error) {
	if event.EventName != "VRFRequest" {
		return nil, fmt.Errorf("not a VRFRequest event")
	}
	if len(event.State) < 4 {
		return nil, fmt.Errorf("invalid event state: expected 4 items, got %d", len(event.State))
	}

	requestID, err := ParseInteger(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse requestId: %w", err)
	}

	userContract, err := ParseHash160(event.State[1])
	if err != nil {
		return nil, fmt.Errorf("parse userContract: %w", err)
	}

	seed, err := ParseByteArray(event.State[2])
	if err != nil {
		return nil, fmt.Errorf("parse seed: %w", err)
	}

	numWords, err := ParseInteger(event.State[3])
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

// ParseVRFFulfilledEvent parses a VRFFulfilled event.
func ParseVRFFulfilledEvent(event *ContractEvent) (*VRFFulfilledEvent, error) {
	if event.EventName != "VRFFulfilled" {
		return nil, fmt.Errorf("not a VRFFulfilled event")
	}
	if len(event.State) < 3 {
		return nil, fmt.Errorf("invalid event state: expected 3 items, got %d", len(event.State))
	}

	requestID, err := ParseInteger(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse requestId: %w", err)
	}

	randomWords, err := ParseByteArray(event.State[1])
	if err != nil {
		return nil, fmt.Errorf("parse randomWords: %w", err)
	}

	proof, err := ParseByteArray(event.State[2])
	if err != nil {
		return nil, fmt.Errorf("parse proof: %w", err)
	}

	return &VRFFulfilledEvent{
		RequestID:   requestID.Uint64(),
		RandomWords: randomWords,
		Proof:       proof,
	}, nil
}

