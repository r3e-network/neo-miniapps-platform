package chain

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
)

// =============================================================================
// Stack Item Parsers
// =============================================================================

func decodeStackBytes(value string) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}

	// Explicit hex prefix (common in client-supplied values).
	if strings.HasPrefix(trimmed, "0x") || strings.HasPrefix(trimmed, "0X") {
		return hex.DecodeString(trimmed[2:])
	}

	// Neo N3 RPC encodes ByteString/Buffer stack items as base64.
	if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil {
		return decoded, nil
	}

	// Fallback: some tools/older endpoints may return raw hex without a prefix.
	if len(trimmed)%2 != 0 {
		return nil, fmt.Errorf("invalid byte string")
	}
	for _, c := range trimmed {
		if (c >= '0' && c <= '9') ||
			(c >= 'a' && c <= 'f') ||
			(c >= 'A' && c <= 'F') {
			continue
		}
		return nil, fmt.Errorf("invalid byte string")
	}
	return hex.DecodeString(trimmed)
}

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
		bytes, err := decodeStackBytes(value)
		if err != nil {
			return "", err
		}
		if len(bytes) != 20 {
			return "", fmt.Errorf("unexpected Hash160 length: %d", len(bytes))
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
		return decodeStackBytes(value)
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
		bytes, err := decodeStackBytes(value)
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
