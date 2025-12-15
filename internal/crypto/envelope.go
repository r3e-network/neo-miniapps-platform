package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
)

const envelopeVersionPrefix = "v1:"

func deriveEnvelopeKey(masterKey, subject []byte, info string) ([]byte, error) {
	if len(masterKey) != 32 {
		return nil, fmt.Errorf("master key must be 32 bytes, got %d", len(masterKey))
	}

	mac := hmac.New(sha256.New, masterKey)
	_, _ = mac.Write([]byte(info))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write(subject)
	return mac.Sum(nil), nil
}

func envelopeAAD(subject []byte, info string) []byte {
	aad := make([]byte, 0, len(info)+1+len(subject))
	aad = append(aad, info...)
	aad = append(aad, 0)
	aad = append(aad, subject...)
	return aad
}

// EncryptEnvelope encrypts plaintext using a key derived from masterKey + subject + info.
// The output is ASCII-safe (`v1:` + base64url(nonce|ciphertext)).
func EncryptEnvelope(masterKey, subject []byte, info string, plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, nil
	}

	key, err := deriveEnvelopeKey(masterKey, subject, info)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("read nonce: %w", err)
	}

	aad := envelopeAAD(subject, info)
	ciphertext := aead.Seal(nil, nonce, plaintext, aad)

	buf := make([]byte, 0, len(nonce)+len(ciphertext))
	buf = append(buf, nonce...)
	buf = append(buf, ciphertext...)

	encoded := base64.RawURLEncoding.EncodeToString(buf)
	return []byte(envelopeVersionPrefix + encoded), nil
}

// DecryptEnvelope decrypts ciphertext previously produced by EncryptEnvelope.
func DecryptEnvelope(masterKey, subject []byte, info string, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, nil
	}

	encoded := strings.TrimSpace(string(ciphertext))
	encoded = strings.TrimPrefix(encoded, envelopeVersionPrefix)

	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}

	key, err := deriveEnvelopeKey(masterKey, subject, info)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	if len(raw) < aead.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce := raw[:aead.NonceSize()]
	body := raw[aead.NonceSize():]
	aad := envelopeAAD(subject, info)

	plaintext, err := aead.Open(nil, nonce, body, aad)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plaintext, nil
}
