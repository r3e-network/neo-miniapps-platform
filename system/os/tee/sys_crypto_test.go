package tee

import (
	"bytes"
	"testing"
)

func TestSysCrypto_Hash(t *testing.T) {
	crypto := NewSysCrypto()

	tests := []struct {
		name      string
		algorithm string
		data      []byte
		wantLen   int
		wantErr   bool
	}{
		{"sha256", "sha256", []byte("hello world"), 32, false},
		{"SHA256", "SHA256", []byte("hello world"), 32, false},
		{"sha512", "sha512", []byte("hello world"), 64, false},
		{"sha3-256", "sha3-256", []byte("hello world"), 32, false},
		{"sha3-512", "sha3-512", []byte("hello world"), 64, false},
		{"keccak256", "keccak256", []byte("hello world"), 32, false},
		{"invalid", "invalid", []byte("hello world"), 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := crypto.Hash(tt.algorithm, tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Hash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(hash) != tt.wantLen {
				t.Errorf("Hash() len = %d, want %d", len(hash), tt.wantLen)
			}
		})
	}
}

func TestSysCrypto_HashConsistency(t *testing.T) {
	crypto := NewSysCrypto()
	data := []byte("test data for hashing")

	hash1, err := crypto.Hash("sha256", data)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	hash2, err := crypto.Hash("sha256", data)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	if !bytes.Equal(hash1, hash2) {
		t.Error("Hash() should return consistent results for same input")
	}
}

func TestSysCrypto_RandomBytes(t *testing.T) {
	crypto := NewSysCrypto()

	tests := []struct {
		name    string
		length  int
		wantErr bool
	}{
		{"16 bytes", 16, false},
		{"32 bytes", 32, false},
		{"64 bytes", 64, false},
		{"1024 bytes", 1024, false},
		{"zero length", 0, true},
		{"negative length", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := crypto.RandomBytes(tt.length)
			if (err != nil) != tt.wantErr {
				t.Errorf("RandomBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(bytes) != tt.length {
				t.Errorf("RandomBytes() len = %d, want %d", len(bytes), tt.length)
			}
		})
	}
}

func TestSysCrypto_RandomBytesUniqueness(t *testing.T) {
	crypto := NewSysCrypto()

	bytes1, err := crypto.RandomBytes(32)
	if err != nil {
		t.Fatalf("RandomBytes() error = %v", err)
	}

	bytes2, err := crypto.RandomBytes(32)
	if err != nil {
		t.Fatalf("RandomBytes() error = %v", err)
	}

	if bytes.Equal(bytes1, bytes2) {
		t.Error("RandomBytes() should return unique values")
	}
}

func TestSysCrypto_GenerateKey_ECDSA(t *testing.T) {
	crypto := NewSysCrypto()

	kp, err := crypto.GenerateKey("ecdsa-p256")
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	if kp.KeyID == "" {
		t.Error("GenerateKey() KeyID should not be empty")
	}

	if kp.KeyType != "ecdsa-p256" {
		t.Errorf("GenerateKey() KeyType = %s, want ecdsa-p256", kp.KeyType)
	}

	// P-256 uncompressed public key is 65 bytes (0x04 + 32 bytes X + 32 bytes Y)
	if len(kp.PublicKey) != 65 {
		t.Errorf("GenerateKey() PublicKey len = %d, want 65", len(kp.PublicKey))
	}

	if kp.PublicKey[0] != 0x04 {
		t.Error("GenerateKey() PublicKey should be uncompressed format (0x04 prefix)")
	}
}

func TestSysCrypto_GenerateKey_AES(t *testing.T) {
	crypto := NewSysCrypto()

	tests := []struct {
		name    string
		keyType string
	}{
		{"AES-128", "aes-128"},
		{"AES-256", "aes-256"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kp, err := crypto.GenerateKey(tt.keyType)
			if err != nil {
				t.Fatalf("GenerateKey() error = %v", err)
			}

			if kp.KeyID == "" {
				t.Error("GenerateKey() KeyID should not be empty")
			}

			// AES keys don't have public keys
			if kp.PublicKey != nil {
				t.Error("GenerateKey() AES key should not have PublicKey")
			}
		})
	}
}

func TestSysCrypto_GenerateKey_Invalid(t *testing.T) {
	crypto := NewSysCrypto()

	_, err := crypto.GenerateKey("invalid-key-type")
	if err == nil {
		t.Error("GenerateKey() should return error for invalid key type")
	}
}

func TestSysCrypto_EncryptDecrypt(t *testing.T) {
	crypto := NewSysCrypto()

	// Generate AES key
	kp, err := crypto.GenerateKey("aes-256")
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	plaintext := []byte("secret message to encrypt")

	// Encrypt
	ciphertext, err := crypto.Encrypt(kp.KeyID, plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	if bytes.Equal(ciphertext, plaintext) {
		t.Error("Encrypt() ciphertext should not equal plaintext")
	}

	// Decrypt
	decrypted, err := crypto.Decrypt(kp.KeyID, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("Decrypt() = %s, want %s", decrypted, plaintext)
	}
}

func TestSysCrypto_EncryptDecrypt_DifferentCiphertexts(t *testing.T) {
	crypto := NewSysCrypto()

	kp, err := crypto.GenerateKey("aes-256")
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	plaintext := []byte("same message")

	// Encrypt twice - should produce different ciphertexts due to random nonce
	ct1, err := crypto.Encrypt(kp.KeyID, plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	ct2, err := crypto.Encrypt(kp.KeyID, plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	if bytes.Equal(ct1, ct2) {
		t.Error("Encrypt() should produce different ciphertexts for same plaintext (random nonce)")
	}

	// Both should decrypt to same plaintext
	pt1, _ := crypto.Decrypt(kp.KeyID, ct1)
	pt2, _ := crypto.Decrypt(kp.KeyID, ct2)

	if !bytes.Equal(pt1, pt2) || !bytes.Equal(pt1, plaintext) {
		t.Error("Decrypt() should produce same plaintext from different ciphertexts")
	}
}

func TestSysCrypto_Encrypt_InvalidKey(t *testing.T) {
	crypto := NewSysCrypto()

	_, err := crypto.Encrypt("nonexistent-key", []byte("data"))
	if err == nil {
		t.Error("Encrypt() should return error for nonexistent key")
	}
}

func TestSysCrypto_Decrypt_InvalidKey(t *testing.T) {
	crypto := NewSysCrypto()

	_, err := crypto.Decrypt("nonexistent-key", []byte("data"))
	if err == nil {
		t.Error("Decrypt() should return error for nonexistent key")
	}
}

func TestSysCrypto_SignVerify(t *testing.T) {
	crypto := NewSysCrypto().(*sysCryptoImpl)

	// Generate ECDSA key
	kp, err := crypto.GenerateKey("ecdsa-p256")
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	data := []byte("message to sign")

	// Sign
	signature, err := crypto.Sign(data)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	if len(signature) != 64 {
		t.Errorf("Sign() signature len = %d, want 64", len(signature))
	}

	// Verify
	valid, err := crypto.Verify(data, signature, kp.PublicKey)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	if !valid {
		t.Error("Verify() should return true for valid signature")
	}
}

func TestSysCrypto_Verify_InvalidSignature(t *testing.T) {
	crypto := NewSysCrypto().(*sysCryptoImpl)

	// Generate ECDSA key
	kp, err := crypto.GenerateKey("ecdsa-p256")
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	data := []byte("message to sign")

	// Create invalid signature
	invalidSig := make([]byte, 64)

	valid, err := crypto.Verify(data, invalidSig, kp.PublicKey)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	if valid {
		t.Error("Verify() should return false for invalid signature")
	}
}

func TestSysCrypto_Verify_WrongData(t *testing.T) {
	crypto := NewSysCrypto().(*sysCryptoImpl)

	// Generate ECDSA key
	kp, err := crypto.GenerateKey("ecdsa-p256")
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	data := []byte("original message")
	wrongData := []byte("different message")

	// Sign original data
	signature, err := crypto.Sign(data)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	// Verify with wrong data
	valid, err := crypto.Verify(wrongData, signature, kp.PublicKey)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	if valid {
		t.Error("Verify() should return false when data doesn't match signature")
	}
}

func TestSysCrypto_HMAC(t *testing.T) {
	crypto := NewSysCrypto().(*sysCryptoImpl)

	key := []byte("secret-key")
	data := []byte("message to authenticate")

	tests := []struct {
		name      string
		algorithm string
		wantLen   int
		wantErr   bool
	}{
		{"sha256", "sha256", 32, false},
		{"sha512", "sha512", 64, false},
		{"invalid", "invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mac, err := crypto.HMAC(tt.algorithm, key, data)
			if (err != nil) != tt.wantErr {
				t.Errorf("HMAC() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(mac) != tt.wantLen {
				t.Errorf("HMAC() len = %d, want %d", len(mac), tt.wantLen)
			}
		})
	}
}

func TestSysCrypto_HMAC_Consistency(t *testing.T) {
	crypto := NewSysCrypto().(*sysCryptoImpl)

	key := []byte("secret-key")
	data := []byte("message")

	mac1, _ := crypto.HMAC("sha256", key, data)
	mac2, _ := crypto.HMAC("sha256", key, data)

	if !bytes.Equal(mac1, mac2) {
		t.Error("HMAC() should return consistent results for same input")
	}
}
