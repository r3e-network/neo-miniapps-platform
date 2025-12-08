package mixerchain

import (
	"fmt"

	"github.com/R3E-Network/service_layer/internal/chain"
)

// =============================================================================
// Mixer Service Events (v5.1 - Off-Chain First with On-Chain Dispute Only)
// =============================================================================
//
// Architecture: Off-Chain Mixing with On-Chain Dispute Resolution Only
// - Normal flow has ZERO on-chain events (all off-chain)
// - On-chain events only occur during:
//   1. Service registration and bond management
//   2. Dispute submission by user
//   3. Dispute resolution by TEE
//   4. Refund claims
//
// Pool accounts are managed entirely off-chain via HD derivation.
// No on-chain pool account registration events - preserves privacy.

// MixerServiceRegisteredEvent represents a ServiceRegistered event.
// Event: ServiceRegistered(serviceId, teePubKey)
type MixerServiceRegisteredEvent struct {
	ServiceID []byte
	TeePubKey []byte
}

// ParseMixerServiceRegisteredEvent parses a ServiceRegistered event.
func ParseMixerServiceRegisteredEvent(event *chain.ContractEvent) (*MixerServiceRegisteredEvent, error) {
	if event.EventName != "ServiceRegistered" {
		return nil, fmt.Errorf("not a ServiceRegistered event")
	}
	if len(event.State) < 2 {
		return nil, fmt.Errorf("invalid event state: expected 2 items, got %d", len(event.State))
	}

	serviceID, err := chain.ParseByteArray(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse serviceId: %w", err)
	}

	teePubKey, err := chain.ParseByteArray(event.State[1])
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
func ParseMixerBondDepositedEvent(event *chain.ContractEvent) (*MixerBondDepositedEvent, error) {
	if event.EventName != "BondDeposited" {
		return nil, fmt.Errorf("not a BondDeposited event")
	}
	if len(event.State) < 3 {
		return nil, fmt.Errorf("invalid event state: expected 3 items, got %d", len(event.State))
	}

	serviceID, err := chain.ParseByteArray(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse serviceId: %w", err)
	}

	amount, err := chain.ParseInteger(event.State[1])
	if err != nil {
		return nil, fmt.Errorf("parse amount: %w", err)
	}

	totalBond, err := chain.ParseInteger(event.State[2])
	if err != nil {
		return nil, fmt.Errorf("parse totalBond: %w", err)
	}

	return &MixerBondDepositedEvent{
		ServiceID: serviceID,
		Amount:    amount.Uint64(),
		TotalBond: totalBond.Uint64(),
	}, nil
}

// MixerDisputeSubmittedEvent represents a DisputeSubmitted event.
// Event: DisputeSubmitted(requestHash, user, amount, serviceId, deadline)
// This event is emitted when a user submits a dispute for an incomplete mix request.
type MixerDisputeSubmittedEvent struct {
	RequestHash []byte
	User        string
	Amount      uint64
	ServiceID   []byte
	Deadline    uint64
}

// ParseMixerDisputeSubmittedEvent parses a DisputeSubmitted event.
func ParseMixerDisputeSubmittedEvent(event *chain.ContractEvent) (*MixerDisputeSubmittedEvent, error) {
	if event.EventName != "DisputeSubmitted" {
		return nil, fmt.Errorf("not a DisputeSubmitted event")
	}
	if len(event.State) < 5 {
		return nil, fmt.Errorf("invalid event state: expected 5 items, got %d", len(event.State))
	}

	requestHash, err := chain.ParseByteArray(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse requestHash: %w", err)
	}

	user, err := chain.ParseHash160(event.State[1])
	if err != nil {
		return nil, fmt.Errorf("parse user: %w", err)
	}

	amount, err := chain.ParseInteger(event.State[2])
	if err != nil {
		return nil, fmt.Errorf("parse amount: %w", err)
	}

	serviceID, err := chain.ParseByteArray(event.State[3])
	if err != nil {
		return nil, fmt.Errorf("parse serviceId: %w", err)
	}

	deadline, err := chain.ParseInteger(event.State[4])
	if err != nil {
		return nil, fmt.Errorf("parse deadline: %w", err)
	}

	return &MixerDisputeSubmittedEvent{
		RequestHash: requestHash,
		User:        user,
		Amount:      amount.Uint64(),
		ServiceID:   serviceID,
		Deadline:    deadline.Uint64(),
	}, nil
}

// MixerDisputeResolvedEvent represents a DisputeResolved event.
// Event: DisputeResolved(requestHash, serviceId, outputsHash)
// This event is emitted when the TEE submits completion proof to resolve a dispute.
type MixerDisputeResolvedEvent struct {
	RequestHash []byte
	ServiceID   []byte
	OutputsHash []byte
}

// ParseMixerDisputeResolvedEvent parses a DisputeResolved event.
func ParseMixerDisputeResolvedEvent(event *chain.ContractEvent) (*MixerDisputeResolvedEvent, error) {
	if event.EventName != "DisputeResolved" {
		return nil, fmt.Errorf("not a DisputeResolved event")
	}
	if len(event.State) < 3 {
		return nil, fmt.Errorf("invalid event state: expected 3 items, got %d", len(event.State))
	}

	requestHash, err := chain.ParseByteArray(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse requestHash: %w", err)
	}

	serviceID, err := chain.ParseByteArray(event.State[1])
	if err != nil {
		return nil, fmt.Errorf("parse serviceId: %w", err)
	}

	outputsHash, err := chain.ParseByteArray(event.State[2])
	if err != nil {
		return nil, fmt.Errorf("parse outputsHash: %w", err)
	}

	return &MixerDisputeResolvedEvent{
		RequestHash: requestHash,
		ServiceID:   serviceID,
		OutputsHash: outputsHash,
	}, nil
}

// MixerRefundClaimedEvent represents a RefundClaimed event.
// Event: RefundClaimed(requestHash, user, amount)
// This event is emitted when a user claims a refund after dispute deadline passes.
type MixerRefundClaimedEvent struct {
	RequestHash []byte
	User        string
	Amount      uint64
}

// ParseMixerRefundClaimedEvent parses a RefundClaimed event.
func ParseMixerRefundClaimedEvent(event *chain.ContractEvent) (*MixerRefundClaimedEvent, error) {
	if event.EventName != "RefundClaimed" {
		return nil, fmt.Errorf("not a RefundClaimed event")
	}
	if len(event.State) < 3 {
		return nil, fmt.Errorf("invalid event state: expected 3 items, got %d", len(event.State))
	}

	requestHash, err := chain.ParseByteArray(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse requestHash: %w", err)
	}

	user, err := chain.ParseHash160(event.State[1])
	if err != nil {
		return nil, fmt.Errorf("parse user: %w", err)
	}

	amount, err := chain.ParseInteger(event.State[2])
	if err != nil {
		return nil, fmt.Errorf("parse amount: %w", err)
	}

	return &MixerRefundClaimedEvent{
		RequestHash: requestHash,
		User:        user,
		Amount:      amount.Uint64(),
	}, nil
}

// MixerBondSlashedEvent represents a BondSlashed event.
// Event: BondSlashed(serviceId, slashedAmount, remainingBond)
// This event is emitted when a service's bond is slashed due to failed dispute resolution.
type MixerBondSlashedEvent struct {
	ServiceID     []byte
	SlashedAmount uint64
	RemainingBond uint64
}

// ParseMixerBondSlashedEvent parses a BondSlashed event.
func ParseMixerBondSlashedEvent(event *chain.ContractEvent) (*MixerBondSlashedEvent, error) {
	if event.EventName != "BondSlashed" {
		return nil, fmt.Errorf("not a BondSlashed event")
	}
	if len(event.State) < 3 {
		return nil, fmt.Errorf("invalid event state: expected 3 items, got %d", len(event.State))
	}

	serviceID, err := chain.ParseByteArray(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse serviceId: %w", err)
	}

	slashedAmount, err := chain.ParseInteger(event.State[1])
	if err != nil {
		return nil, fmt.Errorf("parse slashedAmount: %w", err)
	}

	remainingBond, err := chain.ParseInteger(event.State[2])
	if err != nil {
		return nil, fmt.Errorf("parse remainingBond: %w", err)
	}

	return &MixerBondSlashedEvent{
		ServiceID:     serviceID,
		SlashedAmount: slashedAmount.Uint64(),
		RemainingBond: remainingBond.Uint64(),
	}, nil
}
