package database

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

const (
	oauthTokensMasterKeyEnv    = "OAUTH_TOKENS_MASTER_KEY"  //nolint:gosec // G101: env var name, not a credential value.
	oauthTokensEnvelopeInfo    = "supabase:oauth_tokens:v1" //nolint:gosec // G101: encryption context label, not a credential.
	oauthTokensMasterKeyLength = 32
)

func oauthTokensMasterKeyOptional() (key []byte, ok bool, err error) {
	raw := strings.TrimSpace(os.Getenv(oauthTokensMasterKeyEnv))
	if raw == "" {
		return nil, false, nil
	}

	raw = strings.TrimPrefix(raw, "0x")
	raw = strings.TrimPrefix(raw, "0X")

	key, err = hex.DecodeString(raw)
	if err != nil {
		return nil, false, fmt.Errorf("decode %s: %w", oauthTokensMasterKeyEnv, err)
	}
	if len(key) != oauthTokensMasterKeyLength {
		return nil, false, fmt.Errorf("%s must decode to %d bytes, got %d", oauthTokensMasterKeyEnv, oauthTokensMasterKeyLength, len(key))
	}
	return key, true, nil
}

func oauthTokensMasterKey() ([]byte, error) {
	key, ok, err := oauthTokensMasterKeyOptional()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%s is required", oauthTokensMasterKeyEnv)
	}
	return key, nil
}
