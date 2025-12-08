package chain

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
)

// =============================================================================
// Stack Item Parsers
// =============================================================================

func ParseHash160(item StackItem) (string, error) {
	if item.Type == "ByteString" || item.Type == "Buffer" {
		var value string
		if err := json.Unmarshal(item.Value, &value); err != nil {
			return "", err
		}
		bytes, err := hex.DecodeString(value)
		if err != nil {
			return "", err
		}
		// Reverse for big-endian display
		reversed := make([]byte, len(bytes))
		for i, b := range bytes {
			reversed[len(bytes)-1-i] = b
		}
		return "0x" + hex.EncodeToString(reversed), nil
	}
	return "", fmt.Errorf("unexpected type: %s", item.Type)
}

func ParseByteArray(item StackItem) ([]byte, error) {
	if item.Type == "ByteString" || item.Type == "Buffer" {
		var value string
		if err := json.Unmarshal(item.Value, &value); err != nil {
			return nil, err
		}
		return hex.DecodeString(value)
	}
	if item.Type == "Null" {
		return nil, nil
	}
	return nil, fmt.Errorf("unexpected type: %s", item.Type)
}

func ParseInteger(item StackItem) (*big.Int, error) {
	if item.Type == "Integer" {
		var value string
		if err := json.Unmarshal(item.Value, &value); err != nil {
			return nil, err
		}
		n := new(big.Int)
		n.SetString(value, 10)
		return n, nil
	}
	return nil, fmt.Errorf("unexpected type: %s", item.Type)
}

func ParseBoolean(item StackItem) (bool, error) {
	if item.Type == "Boolean" {
		var value bool
		if err := json.Unmarshal(item.Value, &value); err != nil {
			return false, err
		}
		return value, nil
	}
	return false, fmt.Errorf("unexpected type: %s", item.Type)
}

func ParseStringFromItem(item StackItem) (string, error) {
	if item.Type == "ByteString" || item.Type == "Buffer" {
		var value string
		if err := json.Unmarshal(item.Value, &value); err != nil {
			return "", err
		}
		bytes, err := hex.DecodeString(value)
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	}
	if item.Type == "Null" {
		return "", nil
	}
	return "", fmt.Errorf("unexpected type for string: %s", item.Type)
}

func ParseServiceRequest(item StackItem) (*ContractServiceRequest, error) {
	if item.Type != "Array" && item.Type != "Struct" {
		return nil, fmt.Errorf("expected Array or Struct, got %s", item.Type)
	}

	var items []StackItem
	if err := json.Unmarshal(item.Value, &items); err != nil {
		return nil, fmt.Errorf("unmarshal array: %w", err)
	}

	if len(items) < 12 {
		return nil, fmt.Errorf("expected at least 12 items, got %d", len(items))
	}

	id, _ := ParseInteger(items[0])
	userContract, _ := ParseHash160(items[1])
	payer, _ := ParseHash160(items[2])
	serviceType, _ := ParseStringFromItem(items[3])
	serviceContract, _ := ParseHash160(items[4])
	payload, _ := ParseByteArray(items[5])
	callbackMethod, _ := ParseStringFromItem(items[6])
	status, _ := ParseInteger(items[7])
	fee, _ := ParseInteger(items[8])
	createdAt, _ := ParseInteger(items[9])
	result, _ := ParseByteArray(items[10])
	errorStr, _ := ParseStringFromItem(items[11])

	var completedAt uint64
	if len(items) > 12 {
		ca, _ := ParseInteger(items[12])
		if ca != nil {
			completedAt = ca.Uint64()
		}
	}

	return &ContractServiceRequest{
		ID:              id,
		UserContract:    userContract,
		Payer:           payer,
		ServiceType:     serviceType,
		ServiceContract: serviceContract,
		Payload:         payload,
		CallbackMethod:  callbackMethod,
		Status:          uint8(status.Int64()),
		Fee:             fee,
		CreatedAt:       createdAt.Uint64(),
		Result:          result,
		Error:           errorStr,
		CompletedAt:     completedAt,
	}, nil
}

func ParseMixerPool(item StackItem) (*MixerPool, error) {
	if item.Type != "Array" && item.Type != "Struct" {
		return nil, fmt.Errorf("expected Array or Struct, got %s", item.Type)
	}

	var items []StackItem
	if err := json.Unmarshal(item.Value, &items); err != nil {
		return nil, fmt.Errorf("unmarshal array: %w", err)
	}

	if len(items) < 3 {
		return nil, fmt.Errorf("expected at least 3 items, got %d", len(items))
	}

	denomination, _ := ParseInteger(items[0])
	leafCount, _ := ParseInteger(items[1])
	active, _ := ParseBoolean(items[2])

	return &MixerPool{
		Denomination: denomination,
		LeafCount:    leafCount,
		Active:       active,
	}, nil
}

func ParsePriceData(item StackItem) (*PriceData, error) {
	if item.Type != "Array" && item.Type != "Struct" {
		return nil, fmt.Errorf("expected Array or Struct, got %s", item.Type)
	}

	var items []StackItem
	if err := json.Unmarshal(item.Value, &items); err != nil {
		return nil, fmt.Errorf("unmarshal array: %w", err)
	}

	if len(items) < 5 {
		return nil, fmt.Errorf("expected at least 5 items, got %d", len(items))
	}

	feedID, _ := ParseStringFromItem(items[0])
	price, _ := ParseInteger(items[1])
	decimals, _ := ParseInteger(items[2])
	timestamp, _ := ParseInteger(items[3])
	updatedBy, _ := ParseHash160(items[4])

	return &PriceData{
		FeedID:    feedID,
		Price:     price,
		Decimals:  decimals,
		Timestamp: timestamp.Uint64(),
		UpdatedBy: updatedBy,
	}, nil
}

func ParseFeedConfig(item StackItem) (*ContractFeedConfig, error) {
	if item.Type != "Array" && item.Type != "Struct" {
		return nil, fmt.Errorf("expected Array or Struct, got %s", item.Type)
	}

	var items []StackItem
	if err := json.Unmarshal(item.Value, &items); err != nil {
		return nil, fmt.Errorf("unmarshal array: %w", err)
	}

	if len(items) < 5 {
		return nil, fmt.Errorf("expected at least 5 items, got %d", len(items))
	}

	feedID, _ := ParseStringFromItem(items[0])
	description, _ := ParseStringFromItem(items[1])
	decimals, _ := ParseInteger(items[2])
	active, _ := ParseBoolean(items[3])
	createdAt, _ := ParseInteger(items[4])

	return &ContractFeedConfig{
		FeedID:      feedID,
		Description: description,
		Decimals:    decimals,
		Active:      active,
		CreatedAt:   createdAt.Uint64(),
	}, nil
}

func ParseTrigger(item StackItem) (*Trigger, error) {
	if item.Type != "Array" && item.Type != "Struct" {
		return nil, fmt.Errorf("expected Array or Struct, got %s", item.Type)
	}

	var items []StackItem
	if err := json.Unmarshal(item.Value, &items); err != nil {
		return nil, fmt.Errorf("unmarshal array: %w", err)
	}

	if len(items) < 14 {
		return nil, fmt.Errorf("expected at least 14 items, got %d", len(items))
	}

	triggerID, _ := ParseInteger(items[0])
	requestID, _ := ParseInteger(items[1])
	owner, _ := ParseHash160(items[2])
	targetContract, _ := ParseHash160(items[3])
	callbackMethod, _ := ParseStringFromItem(items[4])
	triggerType, _ := ParseInteger(items[5])
	condition, _ := ParseStringFromItem(items[6])
	callbackData, _ := ParseByteArray(items[7])
	maxExecutions, _ := ParseInteger(items[8])
	executionCount, _ := ParseInteger(items[9])
	status, _ := ParseInteger(items[10])
	createdAt, _ := ParseInteger(items[11])
	lastExecutedAt, _ := ParseInteger(items[12])
	expiresAt, _ := ParseInteger(items[13])

	return &Trigger{
		TriggerID:      triggerID,
		RequestID:      requestID,
		Owner:          owner,
		TargetContract: targetContract,
		CallbackMethod: callbackMethod,
		TriggerType:    uint8(triggerType.Int64()),
		Condition:      condition,
		CallbackData:   callbackData,
		MaxExecutions:  maxExecutions,
		ExecutionCount: executionCount,
		Status:         uint8(status.Int64()),
		CreatedAt:      createdAt.Uint64(),
		LastExecutedAt: lastExecutedAt.Uint64(),
		ExpiresAt:      expiresAt.Uint64(),
	}, nil
}

func ParseExecutionRecord(item StackItem) (*ExecutionRecord, error) {
	if item.Type != "Array" && item.Type != "Struct" {
		return nil, fmt.Errorf("expected Array or Struct, got %s", item.Type)
	}

	var items []StackItem
	if err := json.Unmarshal(item.Value, &items); err != nil {
		return nil, fmt.Errorf("unmarshal array: %w", err)
	}

	if len(items) < 5 {
		return nil, fmt.Errorf("expected at least 5 items, got %d", len(items))
	}

	triggerID, _ := ParseInteger(items[0])
	executionNumber, _ := ParseInteger(items[1])
	timestamp, _ := ParseInteger(items[2])
	success, _ := ParseBoolean(items[3])
	executedBy, _ := ParseHash160(items[4])

	return &ExecutionRecord{
		TriggerID:       triggerID,
		ExecutionNumber: executionNumber,
		Timestamp:       timestamp.Uint64(),
		Success:         success,
		ExecutedBy:      executedBy,
	}, nil
}
