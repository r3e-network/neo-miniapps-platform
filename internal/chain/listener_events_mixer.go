package chain

import "fmt"

// =============================================================================
// Mixer Service Events (Double-Blind HD Multi-sig Mixer v4.0)
// =============================================================================
// Note: Pool accounts are managed entirely off-chain via HD derivation.
// No on-chain pool account registration events - preserves privacy.

// MixerServiceRegisteredEvent represents a ServiceRegistered event.
// Event: ServiceRegistered(serviceId, teePubKey)
type MixerServiceRegisteredEvent struct {
	ServiceID []byte
	TeePubKey []byte
}

// ParseMixerServiceRegisteredEvent parses a ServiceRegistered event.
func ParseMixerServiceRegisteredEvent(event *ContractEvent) (*MixerServiceRegisteredEvent, error) {
	if event.EventName != "ServiceRegistered" {
		return nil, fmt.Errorf("not a ServiceRegistered event")
	}
	if len(event.State) < 2 {
		return nil, fmt.Errorf("invalid event state: expected 2 items, got %d", len(event.State))
	}

	serviceID, err := parseByteArray(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse serviceId: %w", err)
	}

	teePubKey, err := parseByteArray(event.State[1])
	if err != nil {
		return nil, fmt.Errorf("parse teePubKey: %w", err)
	}

	return &MixerServiceRegisteredEvent{
		ServiceID: serviceID,
		TeePubKey: teePubKey,
	}, nil
}

// MixerBondDepositedEvent represents a BondDeposited event.
// Event: BondDeposited(serviceId, amount, totalBond)
type MixerBondDepositedEvent struct {
	ServiceID []byte
	Amount    uint64
	TotalBond uint64
}

// ParseMixerBondDepositedEvent parses a BondDeposited event.
func ParseMixerBondDepositedEvent(event *ContractEvent) (*MixerBondDepositedEvent, error) {
	if event.EventName != "BondDeposited" {
		return nil, fmt.Errorf("not a BondDeposited event")
	}
	if len(event.State) < 3 {
		return nil, fmt.Errorf("invalid event state: expected 3 items, got %d", len(event.State))
	}

	serviceID, err := parseByteArray(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse serviceId: %w", err)
	}

	amount, err := parseInteger(event.State[1])
	if err != nil {
		return nil, fmt.Errorf("parse amount: %w", err)
	}

	totalBond, err := parseInteger(event.State[2])
	if err != nil {
		return nil, fmt.Errorf("parse totalBond: %w", err)
	}

	return &MixerBondDepositedEvent{
		ServiceID: serviceID,
		Amount:    amount.Uint64(),
		TotalBond: totalBond.Uint64(),
	}, nil
}

// MixerRequestCreatedEvent represents a RequestCreated event from MixerService.
// Event: RequestCreated(requestId, depositor, amount, mixOption, deadline)
type MixerRequestCreatedEvent struct {
	RequestID uint64
	Depositor string
	Amount    uint64
	MixOption uint64
	Deadline  uint64
}

// ParseMixerRequestCreatedEvent parses a RequestCreated event.
func ParseMixerRequestCreatedEvent(event *ContractEvent) (*MixerRequestCreatedEvent, error) {
	if event.EventName != "RequestCreated" {
		return nil, fmt.Errorf("not a RequestCreated event")
	}
	if len(event.State) < 5 {
		return nil, fmt.Errorf("invalid event state: expected 5 items, got %d", len(event.State))
	}

	requestID, err := parseInteger(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse requestId: %w", err)
	}

	depositor, err := parseHash160(event.State[1])
	if err != nil {
		return nil, fmt.Errorf("parse depositor: %w", err)
	}

	amount, err := parseInteger(event.State[2])
	if err != nil {
		return nil, fmt.Errorf("parse amount: %w", err)
	}

	mixOption, err := parseInteger(event.State[3])
	if err != nil {
		return nil, fmt.Errorf("parse mixOption: %w", err)
	}

	deadline, err := parseInteger(event.State[4])
	if err != nil {
		return nil, fmt.Errorf("parse deadline: %w", err)
	}

	return &MixerRequestCreatedEvent{
		RequestID: requestID.Uint64(),
		Depositor: depositor,
		Amount:    amount.Uint64(),
		MixOption: mixOption.Uint64(),
		Deadline:  deadline.Uint64(),
	}, nil
}

// MixerRequestClaimedEvent represents a RequestClaimed event.
// Event: RequestClaimed(requestId, serviceId, recipientCount, claimTime)
// Note: Recipients are HD-derived pool accounts, not registered on-chain.
type MixerRequestClaimedEvent struct {
	RequestID      uint64
	ServiceID      []byte
	RecipientCount int
	ClaimTime      uint64
}

// ParseMixerRequestClaimedEvent parses a RequestClaimed event.
func ParseMixerRequestClaimedEvent(event *ContractEvent) (*MixerRequestClaimedEvent, error) {
	if event.EventName != "RequestClaimed" {
		return nil, fmt.Errorf("not a RequestClaimed event")
	}
	if len(event.State) < 4 {
		return nil, fmt.Errorf("invalid event state: expected 4 items, got %d", len(event.State))
	}

	requestID, err := parseInteger(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse requestId: %w", err)
	}

	serviceID, err := parseByteArray(event.State[1])
	if err != nil {
		return nil, fmt.Errorf("parse serviceId: %w", err)
	}

	recipientCount, err := parseInteger(event.State[2])
	if err != nil {
		return nil, fmt.Errorf("parse recipientCount: %w", err)
	}

	claimTime, err := parseInteger(event.State[3])
	if err != nil {
		return nil, fmt.Errorf("parse claimTime: %w", err)
	}

	return &MixerRequestClaimedEvent{
		RequestID:      requestID.Uint64(),
		ServiceID:      serviceID,
		RecipientCount: int(recipientCount.Int64()),
		ClaimTime:      claimTime.Uint64(),
	}, nil
}

// MixerRequestCompletedEvent represents a RequestCompleted event.
// Event: RequestCompleted(requestId, serviceId, outputsHash)
type MixerRequestCompletedEvent struct {
	RequestID   uint64
	ServiceID   []byte
	OutputsHash []byte // Hash of all final transfers to target addresses
}

// ParseMixerRequestCompletedEvent parses a RequestCompleted event.
func ParseMixerRequestCompletedEvent(event *ContractEvent) (*MixerRequestCompletedEvent, error) {
	if event.EventName != "RequestCompleted" {
		return nil, fmt.Errorf("not a RequestCompleted event")
	}
	if len(event.State) < 3 {
		return nil, fmt.Errorf("invalid event state: expected 3 items, got %d", len(event.State))
	}

	requestID, err := parseInteger(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse requestId: %w", err)
	}

	serviceID, err := parseByteArray(event.State[1])
	if err != nil {
		return nil, fmt.Errorf("parse serviceId: %w", err)
	}

	outputsHash, err := parseByteArray(event.State[2])
	if err != nil {
		return nil, fmt.Errorf("parse outputsHash: %w", err)
	}

	return &MixerRequestCompletedEvent{
		RequestID:   requestID.Uint64(),
		ServiceID:   serviceID,
		OutputsHash: outputsHash,
	}, nil
}

// MixerRefundClaimedEvent represents a RefundClaimed event.
// Event: RefundClaimed(requestId, user, amount)
type MixerRefundClaimedEvent struct {
	RequestID uint64
	User      string
	Amount    uint64
}

// ParseMixerRefundClaimedEvent parses a RefundClaimed event.
func ParseMixerRefundClaimedEvent(event *ContractEvent) (*MixerRefundClaimedEvent, error) {
	if event.EventName != "RefundClaimed" {
		return nil, fmt.Errorf("not a RefundClaimed event")
	}
	if len(event.State) < 3 {
		return nil, fmt.Errorf("invalid event state: expected 3 items, got %d", len(event.State))
	}

	requestID, err := parseInteger(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse requestId: %w", err)
	}

	user, err := parseHash160(event.State[1])
	if err != nil {
		return nil, fmt.Errorf("parse user: %w", err)
	}

	amount, err := parseInteger(event.State[2])
	if err != nil {
		return nil, fmt.Errorf("parse amount: %w", err)
	}

	return &MixerRefundClaimedEvent{
		RequestID: requestID.Uint64(),
		User:      user,
		Amount:    amount.Uint64(),
	}, nil
}

// MixerBondSlashedEvent represents a BondSlashed event.
// Event: BondSlashed(serviceId, slashedAmount, remainingBond)
type MixerBondSlashedEvent struct {
	ServiceID     []byte
	SlashedAmount uint64
	RemainingBond uint64
}

// ParseMixerBondSlashedEvent parses a BondSlashed event.
func ParseMixerBondSlashedEvent(event *ContractEvent) (*MixerBondSlashedEvent, error) {
	if event.EventName != "BondSlashed" {
		return nil, fmt.Errorf("not a BondSlashed event")
	}
	if len(event.State) < 3 {
		return nil, fmt.Errorf("invalid event state: expected 3 items, got %d", len(event.State))
	}

	serviceID, err := parseByteArray(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse serviceId: %w", err)
	}

	slashedAmount, err := parseInteger(event.State[1])
	if err != nil {
		return nil, fmt.Errorf("parse slashedAmount: %w", err)
	}

	remainingBond, err := parseInteger(event.State[2])
	if err != nil {
		return nil, fmt.Errorf("parse remainingBond: %w", err)
	}

	return &MixerBondSlashedEvent{
		ServiceID:     serviceID,
		SlashedAmount: slashedAmount.Uint64(),
		RemainingBond: remainingBond.Uint64(),
	}, nil
}
