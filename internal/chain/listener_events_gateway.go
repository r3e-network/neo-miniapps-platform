package chain

import "fmt"

// =============================================================================
// Gateway Events (ServiceLayerGateway contract)
// =============================================================================

// ServiceRequestEvent represents a ServiceRequest event from Gateway.
// Event: ServiceRequest(requestId, userContract, caller, serviceType, payload)
type ServiceRequestEvent struct {
	RequestID    uint64
	UserContract string
	Caller       string
	ServiceType  string
	Payload      []byte
}

// ParseServiceRequestEvent parses a ServiceRequest event from Gateway.
func ParseServiceRequestEvent(event *ContractEvent) (*ServiceRequestEvent, error) {
	if event.EventName != "ServiceRequest" {
		return nil, fmt.Errorf("not a ServiceRequest event")
	}
	if len(event.State) < 5 {
		return nil, fmt.Errorf("invalid event state: expected 5 items, got %d", len(event.State))
	}

	requestID, err := parseInteger(event.State[0])
	if err != nil {
		return nil, fmt.Errorf("parse requestId: %w", err)
	}

	userContract, err := parseHash160(event.State[1])
	if err != nil {
		return nil, fmt.Errorf("parse userContract: %w", err)
	}

	caller, err := parseHash160(event.State[2])
	if err != nil {
		return nil, fmt.Errorf("parse caller: %w", err)
	}

	serviceType, err := parseStringFromItem(event.State[3])
	if err != nil {
		return nil, fmt.Errorf("parse serviceType: %w", err)
	}

	payload, err := parseByteArray(event.State[4])
	if err != nil {
		return nil, fmt.Errorf("parse payload: %w", err)
	}

	return &ServiceRequestEvent{
		RequestID:    requestID.Uint64(),
		UserContract: userContract,
		Caller:       caller,
		ServiceType:  serviceType,
		Payload:      payload,
	}, nil
}

// RequestFulfilledEvent represents a RequestFulfilled event from Gateway.
// Event: RequestFulfilled(requestId, result)
type RequestFulfilledEvent struct {
	RequestID uint64
	Result    []byte
}

// ParseRequestFulfilledEvent parses a RequestFulfilled event.
func ParseRequestFulfilledEvent(event *ContractEvent) (*RequestFulfilledEvent, error) {
	if event.EventName != "RequestFulfilled" {
		return nil, fmt.Errorf("not a RequestFulfilled event")
	}
	if len(event.State) < 2 {
		return nil, fmt.Errorf("invalid event state")
	}

	requestID, err := parseInteger(event.State[0])
	if err != nil {
		return nil, err
	}

	result, err := parseByteArray(event.State[1])
	if err != nil {
		return nil, err
	}

	return &RequestFulfilledEvent{
		RequestID: requestID.Uint64(),
		Result:    result,
	}, nil
}

// RequestFailedEvent represents a RequestFailed event from Gateway.
// Event: RequestFailed(requestId, reason)
type RequestFailedEvent struct {
	RequestID uint64
	Reason    string
}

// ParseRequestFailedEvent parses a RequestFailed event.
func ParseRequestFailedEvent(event *ContractEvent) (*RequestFailedEvent, error) {
	if event.EventName != "RequestFailed" {
		return nil, fmt.Errorf("not a RequestFailed event")
	}
	if len(event.State) < 2 {
		return nil, fmt.Errorf("invalid event state")
	}

	requestID, err := parseInteger(event.State[0])
	if err != nil {
		return nil, err
	}

	reason, err := parseStringFromItem(event.State[1])
	if err != nil {
		return nil, err
	}

	return &RequestFailedEvent{
		RequestID: requestID.Uint64(),
		Reason:    reason,
	}, nil
}
