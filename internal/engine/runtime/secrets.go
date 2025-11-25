package runtime

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/R3E-Network/service_layer/internal/services/secrets"
)

func loadSecretsCipher(value string) (secrets.Cipher, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return nil, nil
	}

	key, err := parseEncryptionKey(raw)
	if err != nil {
		return nil, err
	}

	cipher, err := secrets.NewAESCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher, nil
}

func parseEncryptionKey(value string) ([]byte, error) {
	switch {
	case value == "":
		return nil, errors.New("encryption key is empty")
	case isValidKeyLength(len(value)):
		return []byte(value), nil
	}
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil && isValidKeyLength(len(decoded)) {
		return decoded, nil
	}
	if decoded, err := hex.DecodeString(value); err == nil && isValidKeyLength(len(decoded)) {
		return decoded, nil
	}
	return nil, fmt.Errorf("expected 16, 24, or 32 byte key")
}

func isValidKeyLength(n int) bool {
	switch n {
	case 16, 24, 32:
		return true
	default:
		return false
	}
}
