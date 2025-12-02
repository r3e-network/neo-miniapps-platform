package service

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// Encoding helper functions for consistent key/data handling.
// These consolidate duplicate implementations across cmd tools.

// DecodeKey decodes a key from hex or base64 format.
// It tries hex first, then base64.
func DecodeKey(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("empty key")
	}
	if b, err := hex.DecodeString(value); err == nil {
		return b, nil
	}
	if b, err := base64.StdEncoding.DecodeString(value); err == nil {
		return b, nil
	}
	return nil, fmt.Errorf("key must be hex or base64")
}

// EncodeKeyHex encodes a key to hex format.
func EncodeKeyHex(data []byte) string {
	return hex.EncodeToString(data)
}

// EncodeKeyBase64 encodes a key to base64 format.
func EncodeKeyBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
