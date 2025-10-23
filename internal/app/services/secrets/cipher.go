package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
)

// Cipher encrypts and decrypts secret values.
type Cipher interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
}

type noopCipher struct{}

func (noopCipher) Encrypt(plaintext []byte) ([]byte, error) {
	return append([]byte(nil), plaintext...), nil
}
func (noopCipher) Decrypt(ciphertext []byte) ([]byte, error) {
	return append([]byte(nil), ciphertext...), nil
}

// NewAESCipher constructs an AES-GCM cipher from the provided key.
func NewAESCipher(key []byte) (Cipher, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	return &aesCipher{gcm: gcm}, nil
}

type aesCipher struct {
	gcm cipher.AEAD
}

func (c *aesCipher) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	sealed := c.gcm.Seal(nonce, nonce, plaintext, nil)
	return sealed, nil
}

func (c *aesCipher) Decrypt(ciphertext []byte) ([]byte, error) {
	ns := c.gcm.NonceSize()
	if len(ciphertext) < ns {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce := ciphertext[:ns]
	data := ciphertext[ns:]
	plaintext, err := c.gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plaintext, nil
}
