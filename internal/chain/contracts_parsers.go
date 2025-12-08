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

func parseHash160(item StackItem) (string, error) {
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

func parseByteArray(item StackItem) ([]byte, error) {
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

func parseInteger(item StackItem) (*big.Int, error) {
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

func parseBoolean(item StackItem) (bool, error) {
	if item.Type == "Boolean" {
		var value bool
		if err := json.Unmarshal(item.Value, &value); err != nil {
			return false, err
		}
		return value, nil
	}
	return false, fmt.Errorf("unexpected type: %s", item.Type)
}

func parseStringFromItem(item StackItem) (string, error) {
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

func parseServiceRequest(item StackItem) (*ServiceRequest, error) {
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

	id, _ := parseInteger(items[0])
	userContract, _ := parseHash160(items[1])
	payer, _ := parseHash160(items[2])
	serviceType, _ := parseStringFromItem(items[3])
	serviceContract, _ := parseHash160(items[4])
	payload, _ := parseByteArray(items[5])
	callbackMethod, _ := parseStringFromItem(items[6])
	status, _ := parseInteger(items[7])
	fee, _ := parseInteger(items[8])
	createdAt, _ := parseInteger(items[9])
	result, _ := parseByteArray(items[10])
	errorStr, _ := parseStringFromItem(items[11])

	var completedAt uint64
	if len(items) > 12 {
		ca, _ := parseInteger(items[12])
		if ca != nil {
			completedAt = ca.Uint64()
		}
	}

	return &ServiceRequest{
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

func parseMixerPool(item StackItem) (*MixerPool, error) {
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

	denomination, _ := parseInteger(items[0])
	leafCount, _ := parseInteger(items[1])
	active, _ := parseBoolean(items[2])

	return &MixerPool{
		Denomination: denomination,
		LeafCount:    leafCount,
		Active:       active,
	}, nil
}

func parsePriceData(item StackItem) (*PriceData, error) {
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

	feedID, _ := parseStringFromItem(items[0])
	price, _ := parseInteger(items[1])
	decimals, _ := parseInteger(items[2])
	timestamp, _ := parseInteger(items[3])
	updatedBy, _ := parseHash160(items[4])

	return &PriceData{
		FeedID:    feedID,
		Price:     price,
		Decimals:  decimals,
		Timestamp: timestamp.Uint64(),
		UpdatedBy: updatedBy,
	}, nil
}

func parseFeedConfig(item StackItem) (*FeedConfig, error) {
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

	feedID, _ := parseStringFromItem(items[0])
	description, _ := parseStringFromItem(items[1])
	decimals, _ := parseInteger(items[2])
	active, _ := parseBoolean(items[3])
	createdAt, _ := parseInteger(items[4])

	return &FeedConfig{
		FeedID:      feedID,
		Description: description,
		Decimals:    decimals,
		Active:      active,
		CreatedAt:   createdAt.Uint64(),
	}, nil
}

func parseTrigger(item StackItem) (*Trigger, error) {
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

	triggerID, _ := parseInteger(items[0])
	requestID, _ := parseInteger(items[1])
	owner, _ := parseHash160(items[2])
	targetContract, _ := parseHash160(items[3])
	callbackMethod, _ := parseStringFromItem(items[4])
	triggerType, _ := parseInteger(items[5])
	condition, _ := parseStringFromItem(items[6])
	callbackData, _ := parseByteArray(items[7])
	maxExecutions, _ := parseInteger(items[8])
	executionCount, _ := parseInteger(items[9])
	status, _ := parseInteger(items[10])
	createdAt, _ := parseInteger(items[11])
	lastExecutedAt, _ := parseInteger(items[12])
	expiresAt, _ := parseInteger(items[13])

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

func parseExecutionRecord(item StackItem) (*ExecutionRecord, error) {
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

	triggerID, _ := parseInteger(items[0])
	executionNumber, _ := parseInteger(items[1])
	timestamp, _ := parseInteger(items[2])
	success, _ := parseBoolean(items[3])
	executedBy, _ := parseHash160(items[4])

	return &ExecutionRecord{
		TriggerID:       triggerID,
		ExecutionNumber: executionNumber,
		Timestamp:       timestamp.Uint64(),
		Success:         success,
		ExecutedBy:      executedBy,
	}, nil
}
