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

// ParseArray extracts an array of StackItems from a parent StackItem.
func ParseArray(item StackItem) ([]StackItem, error) {
	if item.Type != "Array" && item.Type != "Struct" {
		return nil, fmt.Errorf("expected Array or Struct, got %s", item.Type)
	}

	var items []StackItem
	if err := json.Unmarshal(item.Value, &items); err != nil {
		return nil, fmt.Errorf("unmarshal array: %w", err)
	}
	return items, nil
}

// ParseString parses a string from a StackItem.
// Alias for ParseStringFromItem for consistency.
func ParseString(item StackItem) (string, error) {
	return ParseStringFromItem(item)
}

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

	id, err := ParseInteger(items[0])
	if err != nil {
		return nil, fmt.Errorf("parse id: %w", err)
	}
	userContract, err := ParseHash160(items[1])
	if err != nil {
		return nil, fmt.Errorf("parse userContract: %w", err)
	}
	payer, err := ParseHash160(items[2])
	if err != nil {
		return nil, fmt.Errorf("parse payer: %w", err)
	}
	serviceType, err := ParseStringFromItem(items[3])
	if err != nil {
		return nil, fmt.Errorf("parse serviceType: %w", err)
	}
	serviceContract, err := ParseHash160(items[4])
	if err != nil {
		return nil, fmt.Errorf("parse serviceContract: %w", err)
	}
	payload, err := ParseByteArray(items[5])
	if err != nil {
		return nil, fmt.Errorf("parse payload: %w", err)
	}
	callbackMethod, err := ParseStringFromItem(items[6])
	if err != nil {
		return nil, fmt.Errorf("parse callbackMethod: %w", err)
	}
	status, err := ParseInteger(items[7])
	if err != nil {
		return nil, fmt.Errorf("parse status: %w", err)
	}
	fee, err := ParseInteger(items[8])
	if err != nil {
		return nil, fmt.Errorf("parse fee: %w", err)
	}
	createdAt, err := ParseInteger(items[9])
	if err != nil {
		return nil, fmt.Errorf("parse createdAt: %w", err)
	}
	// result and error can be null, so we don't fail on parse errors
	var result []byte
	if v, parseErr := ParseByteArray(items[10]); parseErr == nil {
		result = v
	}
	var errorStr string
	if v, parseErr := ParseStringFromItem(items[11]); parseErr == nil {
		errorStr = v
	}

	var completedAt uint64
	if len(items) > 12 {
		ca, parseErr := ParseInteger(items[12])
		if parseErr == nil && ca != nil {
			completedAt = ca.Uint64()
		}
	}

	statusU8, err := uint8FromBigInt(status)
	if err != nil {
		return nil, fmt.Errorf("parse status: %w", err)
	}

	return &ContractServiceRequest{
		ID:              id,
		UserContract:    userContract,
		Payer:           payer,
		ServiceType:     serviceType,
		ServiceContract: serviceContract,
		Payload:         payload,
		CallbackMethod:  callbackMethod,
		Status:          statusU8,
		Fee:             fee,
		CreatedAt:       createdAt.Uint64(),
		Result:          result,
		Error:           errorStr,
		CompletedAt:     completedAt,
	}, nil
}

func ParseNeoVaultPool(item StackItem) (*NeoVaultPool, error) {
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

	denomination, err := ParseInteger(items[0])
	if err != nil {
		return nil, fmt.Errorf("parse denomination: %w", err)
	}
	leafCount, err := ParseInteger(items[1])
	if err != nil {
		return nil, fmt.Errorf("parse leafCount: %w", err)
	}
	active, err := ParseBoolean(items[2])
	if err != nil {
		return nil, fmt.Errorf("parse active: %w", err)
	}

	return &NeoVaultPool{
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

	feedID, err := ParseStringFromItem(items[0])
	if err != nil {
		return nil, fmt.Errorf("parse feedID: %w", err)
	}
	price, err := ParseInteger(items[1])
	if err != nil {
		return nil, fmt.Errorf("parse price: %w", err)
	}
	decimals, err := ParseInteger(items[2])
	if err != nil {
		return nil, fmt.Errorf("parse decimals: %w", err)
	}
	timestamp, err := ParseInteger(items[3])
	if err != nil {
		return nil, fmt.Errorf("parse timestamp: %w", err)
	}
	updatedBy, err := ParseHash160(items[4])
	if err != nil {
		return nil, fmt.Errorf("parse updatedBy: %w", err)
	}

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

	feedID, err := ParseStringFromItem(items[0])
	if err != nil {
		return nil, fmt.Errorf("parse feedID: %w", err)
	}
	description, err := ParseStringFromItem(items[1])
	if err != nil {
		return nil, fmt.Errorf("parse description: %w", err)
	}
	decimals, err := ParseInteger(items[2])
	if err != nil {
		return nil, fmt.Errorf("parse decimals: %w", err)
	}
	active, err := ParseBoolean(items[3])
	if err != nil {
		return nil, fmt.Errorf("parse active: %w", err)
	}
	createdAt, err := ParseInteger(items[4])
	if err != nil {
		return nil, fmt.Errorf("parse createdAt: %w", err)
	}

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

	triggerID, err := ParseInteger(items[0])
	if err != nil {
		return nil, fmt.Errorf("parse triggerID: %w", err)
	}
	requestID, err := ParseInteger(items[1])
	if err != nil {
		return nil, fmt.Errorf("parse requestID: %w", err)
	}
	owner, err := ParseHash160(items[2])
	if err != nil {
		return nil, fmt.Errorf("parse owner: %w", err)
	}
	targetContract, err := ParseHash160(items[3])
	if err != nil {
		return nil, fmt.Errorf("parse targetContract: %w", err)
	}
	callbackMethod, err := ParseStringFromItem(items[4])
	if err != nil {
		return nil, fmt.Errorf("parse callbackMethod: %w", err)
	}
	triggerType, err := ParseInteger(items[5])
	if err != nil {
		return nil, fmt.Errorf("parse triggerType: %w", err)
	}
	condition, err := ParseStringFromItem(items[6])
	if err != nil {
		return nil, fmt.Errorf("parse condition: %w", err)
	}
	// callbackData can be null
	var callbackData []byte
	if v, parseErr := ParseByteArray(items[7]); parseErr == nil {
		callbackData = v
	}
	maxExecutions, err := ParseInteger(items[8])
	if err != nil {
		return nil, fmt.Errorf("parse maxExecutions: %w", err)
	}
	executionCount, err := ParseInteger(items[9])
	if err != nil {
		return nil, fmt.Errorf("parse executionCount: %w", err)
	}
	status, err := ParseInteger(items[10])
	if err != nil {
		return nil, fmt.Errorf("parse status: %w", err)
	}
	createdAt, err := ParseInteger(items[11])
	if err != nil {
		return nil, fmt.Errorf("parse createdAt: %w", err)
	}
	lastExecutedAt, err := ParseInteger(items[12])
	if err != nil {
		return nil, fmt.Errorf("parse lastExecutedAt: %w", err)
	}
	expiresAt, err := ParseInteger(items[13])
	if err != nil {
		return nil, fmt.Errorf("parse expiresAt: %w", err)
	}

	triggerTypeU8, err := uint8FromBigInt(triggerType)
	if err != nil {
		return nil, fmt.Errorf("parse triggerType: %w", err)
	}
	statusU8, err := uint8FromBigInt(status)
	if err != nil {
		return nil, fmt.Errorf("parse status: %w", err)
	}

	return &Trigger{
		TriggerID:      triggerID,
		RequestID:      requestID,
		Owner:          owner,
		TargetContract: targetContract,
		CallbackMethod: callbackMethod,
		TriggerType:    triggerTypeU8,
		Condition:      condition,
		CallbackData:   callbackData,
		MaxExecutions:  maxExecutions,
		ExecutionCount: executionCount,
		Status:         statusU8,
		CreatedAt:      createdAt.Uint64(),
		LastExecutedAt: lastExecutedAt.Uint64(),
		ExpiresAt:      expiresAt.Uint64(),
	}, nil
}

func uint8FromBigInt(v *big.Int) (uint8, error) {
	if v == nil {
		return 0, fmt.Errorf("nil value")
	}
	if v.Sign() < 0 || v.BitLen() > 8 {
		return 0, fmt.Errorf("value %s out of uint8 range", v.String())
	}
	// Range checked via BitLen/Sign above.
	return uint8(v.Uint64()), nil // #nosec G115
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

	triggerID, err := ParseInteger(items[0])
	if err != nil {
		return nil, fmt.Errorf("parse triggerID: %w", err)
	}
	executionNumber, err := ParseInteger(items[1])
	if err != nil {
		return nil, fmt.Errorf("parse executionNumber: %w", err)
	}
	timestamp, err := ParseInteger(items[2])
	if err != nil {
		return nil, fmt.Errorf("parse timestamp: %w", err)
	}
	success, err := ParseBoolean(items[3])
	if err != nil {
		return nil, fmt.Errorf("parse success: %w", err)
	}
	executedBy, err := ParseHash160(items[4])
	if err != nil {
		return nil, fmt.Errorf("parse executedBy: %w", err)
	}

	return &ExecutionRecord{
		TriggerID:       triggerID,
		ExecutionNumber: executionNumber,
		Timestamp:       timestamp.Uint64(),
		Success:         success,
		ExecutedBy:      executedBy,
	}, nil
}
