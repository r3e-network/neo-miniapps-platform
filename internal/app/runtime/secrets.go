package runtime

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/R3E-Network/service_layer/internal/app/services/secrets"
)

func loadSecretsCipher() (secrets.Cipher, error) {
	raw := strings.TrimSpace(os.Getenv("SECRET_ENCRYPTION_KEY"))
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
	case len(value) == 16 || len(value) == 24 || len(value) == 32:
		return []byte(value), nil
	case len(value) == 44:
		decoded, err := decodeBase64(value)
		if err != nil {
			return nil, err
		}
		if len(decoded) != 32 {
			return nil, fmt.Errorf("expected 32 bytes, got %d", len(decoded))
		}
		return decoded, nil
	case len(value) == 64:
		decoded, err := decodeHex(value)
		if err != nil {
			return nil, err
		}
		if len(decoded) != 32 {
			return nil, fmt.Errorf("expected 32 bytes, got %d", len(decoded))
		}
		return decoded, nil
	default:
		return nil, fmt.Errorf("unsupported encryption key format")
	}
}

func decodeBase64(raw string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}
	return data, nil
}

func decodeHex(raw string) ([]byte, error) {
	data, err := hex.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("hex decode: %w", err)
	}
	return data, nil
}
