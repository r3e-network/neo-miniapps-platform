package crypto

import (
	"bytes"
	"testing"
)

func TestDeriveEnvelopeKey(t *testing.T) {
	t.Run("valid 32-byte key", func(t *testing.T) {
		masterKey := make([]byte, 32)
		for i := range masterKey {
			masterKey[i] = byte(i)
		}
		subject := []byte("test-subject")
		info := "test-info"

		key, err := deriveEnvelopeKey(masterKey, subject, info)
		if err != nil {
			t.Fatalf("deriveEnvelopeKey() error = %v", err)
		}
		if len(key) != 32 {
			t.Errorf("derived key length = %d, want 32", len(key))
		}
	})

	t.Run("deterministic derivation", func(t *testing.T) {
		masterKey := make([]byte, 32)
		subject := []byte("subject")
		info := "info"

		key1, _ := deriveEnvelopeKey(masterKey, subject, info)
		key2, _ := deriveEnvelopeKey(masterKey, subject, info)

		if !bytes.Equal(key1, key2) {
			t.Error("same inputs should produce same key")
		}
	})

	t.Run("different subjects produce different keys", func(t *testing.T) {
		masterKey := make([]byte, 32)
		info := "info"

		key1, _ := deriveEnvelopeKey(masterKey, []byte("subject1"), info)
		key2, _ := deriveEnvelopeKey(masterKey, []byte("subject2"), info)

		if bytes.Equal(key1, key2) {
			t.Error("different subjects should produce different keys")
		}
	})

	t.Run("invalid key length", func(t *testing.T) {
		masterKey := make([]byte, 16) // Wrong length
		_, err := deriveEnvelopeKey(masterKey, []byte("subject"), "info")
		if err == nil {
			t.Error("expected error for invalid key length")
		}
	})
}

func TestEnvelopeAAD(t *testing.T) {
	subject := []byte("test-subject")
	info := "test-info"

	aad := envelopeAAD(subject, info)

	// AAD should be: info + 0 + subject
	expected := append([]byte(info), 0)
	expected = append(expected, subject...)

	if !bytes.Equal(aad, expected) {
		t.Errorf("envelopeAAD() = %v, want %v", aad, expected)
	}
}

func TestEncryptDecryptEnvelope(t *testing.T) {
	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i)
	}
	subject := []byte("user-123")
	info := "secret-data"

	t.Run("round trip", func(t *testing.T) {
		plaintext := []byte("Hello, World!")

		ciphertext, err := EncryptEnvelope(masterKey, subject, info, plaintext)
		if err != nil {
			t.Fatalf("EncryptEnvelope() error = %v", err)
		}

		// Verify prefix
		if !bytes.HasPrefix(ciphertext, []byte("v1:")) {
			t.Error("ciphertext should have v1: prefix")
		}

		decrypted, err := DecryptEnvelope(masterKey, subject, info, ciphertext)
		if err != nil {
			t.Fatalf("DecryptEnvelope() error = %v", err)
		}

		if !bytes.Equal(decrypted, plaintext) {
			t.Errorf("decrypted = %s, want %s", string(decrypted), string(plaintext))
		}
	})

	t.Run("empty plaintext", func(t *testing.T) {
		ciphertext, err := EncryptEnvelope(masterKey, subject, info, nil)
		if err != nil {
			t.Fatalf("EncryptEnvelope() error = %v", err)
		}
		if ciphertext != nil {
			t.Error("empty plaintext should return nil ciphertext")
		}
	})

	t.Run("empty ciphertext", func(t *testing.T) {
		plaintext, err := DecryptEnvelope(masterKey, subject, info, nil)
		if err != nil {
			t.Fatalf("DecryptEnvelope() error = %v", err)
		}
		if plaintext != nil {
			t.Error("empty ciphertext should return nil plaintext")
		}
	})

	t.Run("invalid master key length", func(t *testing.T) {
		badKey := make([]byte, 16)
		_, err := EncryptEnvelope(badKey, subject, info, []byte("test"))
		if err == nil {
			t.Error("expected error for invalid key length")
		}
	})

	t.Run("wrong subject fails decryption", func(t *testing.T) {
		plaintext := []byte("secret")
		ciphertext, _ := EncryptEnvelope(masterKey, subject, info, plaintext)

		_, err := DecryptEnvelope(masterKey, []byte("wrong-subject"), info, ciphertext)
		if err == nil {
			t.Error("expected error for wrong subject")
		}
	})

	t.Run("wrong info fails decryption", func(t *testing.T) {
		plaintext := []byte("secret")
		ciphertext, _ := EncryptEnvelope(masterKey, subject, info, plaintext)

		_, err := DecryptEnvelope(masterKey, subject, "wrong-info", ciphertext)
		if err == nil {
			t.Error("expected error for wrong info")
		}
	})

	t.Run("wrong master key fails decryption", func(t *testing.T) {
		plaintext := []byte("secret")
		ciphertext, _ := EncryptEnvelope(masterKey, subject, info, plaintext)

		wrongKey := make([]byte, 32)
		wrongKey[0] = 0xFF
		_, err := DecryptEnvelope(wrongKey, subject, info, ciphertext)
		if err == nil {
			t.Error("expected error for wrong master key")
		}
	})

	t.Run("invalid base64 ciphertext", func(t *testing.T) {
		_, err := DecryptEnvelope(masterKey, subject, info, []byte("v1:!!!invalid-base64!!!"))
		if err == nil {
			t.Error("expected error for invalid base64")
		}
	})

	t.Run("ciphertext too short", func(t *testing.T) {
		// v1: prefix + very short base64 (less than nonce size)
		_, err := DecryptEnvelope(masterKey, subject, info, []byte("v1:YWJj"))
		if err == nil {
			t.Error("expected error for ciphertext too short")
		}
	})

	t.Run("tampered ciphertext", func(t *testing.T) {
		plaintext := []byte("secret")
		ciphertext, _ := EncryptEnvelope(masterKey, subject, info, plaintext)

		// Tamper with the ciphertext
		tampered := make([]byte, len(ciphertext))
		copy(tampered, ciphertext)
		tampered[len(tampered)-1] ^= 0xFF

		_, err := DecryptEnvelope(masterKey, subject, info, tampered)
		if err == nil {
			t.Error("expected error for tampered ciphertext")
		}
	})

	t.Run("ciphertext without prefix", func(t *testing.T) {
		plaintext := []byte("secret")
		ciphertext, _ := EncryptEnvelope(masterKey, subject, info, plaintext)

		// Remove the v1: prefix
		withoutPrefix := bytes.TrimPrefix(ciphertext, []byte("v1:"))

		// Should still work (prefix is optional for decryption)
		decrypted, err := DecryptEnvelope(masterKey, subject, info, withoutPrefix)
		if err != nil {
			t.Fatalf("DecryptEnvelope() error = %v", err)
		}
		if !bytes.Equal(decrypted, plaintext) {
			t.Errorf("decrypted = %s, want %s", string(decrypted), string(plaintext))
		}
	})
}

func TestEncryptEnvelopeUniqueness(t *testing.T) {
	masterKey := make([]byte, 32)
	subject := []byte("subject")
	info := "info"
	plaintext := []byte("same plaintext")

	// Encrypt same plaintext twice
	ct1, _ := EncryptEnvelope(masterKey, subject, info, plaintext)
	ct2, _ := EncryptEnvelope(masterKey, subject, info, plaintext)

	// Ciphertexts should be different due to random nonce
	if bytes.Equal(ct1, ct2) {
		t.Error("encrypting same plaintext twice should produce different ciphertexts")
	}

	// But both should decrypt to same plaintext
	pt1, _ := DecryptEnvelope(masterKey, subject, info, ct1)
	pt2, _ := DecryptEnvelope(masterKey, subject, info, ct2)

	if !bytes.Equal(pt1, pt2) || !bytes.Equal(pt1, plaintext) {
		t.Error("both ciphertexts should decrypt to same plaintext")
	}
}
