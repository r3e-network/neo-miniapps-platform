// Package integration provides integration tests for the service layer.
package integration

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/R3E-Network/service_layer/internal/crypto"
	"github.com/R3E-Network/service_layer/internal/marble"
)

func TestMarbleInitialization(t *testing.T) {
	m, err := marble.New(marble.Config{
		MarbleType: "test",
	})
	if err != nil {
		t.Fatalf("Failed to create marble: %v", err)
	}

	if m.MarbleType() != "test" {
		t.Errorf("Expected marble type 'test', got '%s'", m.MarbleType())
	}
}

func TestCryptoKeyDerivation(t *testing.T) {
	masterKey := []byte("test-master-key-32-bytes-long!!")
	salt := []byte("test-salt")

	key1, err := crypto.DeriveKey(masterKey, salt, "purpose1", 32)
	if err != nil {
		t.Fatalf("Failed to derive key: %v", err)
	}

	key2, err := crypto.DeriveKey(masterKey, salt, "purpose2", 32)
	if err != nil {
		t.Fatalf("Failed to derive key: %v", err)
	}

	if hex.EncodeToString(key1) == hex.EncodeToString(key2) {
		t.Error("Keys with different purposes should be different")
	}
}

func TestCryptoEncryption(t *testing.T) {
	key := make([]byte, 32)
	copy(key, []byte("test-encryption-key-32-bytes!!!"))

	plaintext := []byte("Hello, World!")

	ciphertext, err := crypto.Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	decrypted, err := crypto.Decrypt(key, ciphertext)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted text doesn't match: got '%s', want '%s'", decrypted, plaintext)
	}
}

func TestCryptoSigning(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	data := []byte("test data to sign")

	signature, err := crypto.Sign(keyPair.PrivateKey, data)
	if err != nil {
		t.Fatalf("Failed to sign: %v", err)
	}

	if len(signature) != 64 {
		t.Errorf("Expected 64-byte signature, got %d bytes", len(signature))
	}

	if !crypto.Verify(keyPair.PublicKey, data, signature) {
		t.Error("Signature verification failed")
	}
}

func TestVRFGeneration(t *testing.T) {
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	alpha := []byte("test seed")

	proof, err := crypto.GenerateVRF(keyPair.PrivateKey, alpha)
	if err != nil {
		t.Fatalf("Failed to generate VRF: %v", err)
	}

	if len(proof.Output) != 32 {
		t.Errorf("Expected 32-byte output, got %d bytes", len(proof.Output))
	}

	if !crypto.VerifyVRF(keyPair.PublicKey, alpha, proof) {
		t.Error("VRF verification failed")
	}
}

func TestServiceFramework(t *testing.T) {
	m, _ := marble.New(marble.Config{MarbleType: "test"})

	svc := marble.NewService(marble.ServiceConfig{
		ID:      "test-service",
		Name:    "Test Service",
		Version: "1.0.0",
		Marble:  m,
		DB:      nil,
	})

	if svc.ID() != "test-service" {
		t.Errorf("Expected ID 'test-service', got '%s'", svc.ID())
	}

	if svc.Name() != "Test Service" {
		t.Errorf("Expected name 'Test Service', got '%s'", svc.Name())
	}

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	if !svc.IsRunning() {
		t.Error("Service should be running")
	}

	if err := svc.Stop(); err != nil {
		t.Fatalf("Failed to stop service: %v", err)
	}

	if svc.IsRunning() {
		t.Error("Service should not be running")
	}
}

func TestHash256(t *testing.T) {
	data := []byte("test data")
	hash := crypto.Hash256(data)

	if len(hash) != 32 {
		t.Errorf("Expected 32-byte hash, got %d bytes", len(hash))
	}

	// Same input should produce same hash
	hash2 := crypto.Hash256(data)
	if hex.EncodeToString(hash) != hex.EncodeToString(hash2) {
		t.Error("Hash should be deterministic")
	}
}

func TestHash160(t *testing.T) {
	data := []byte("test data")
	hash := crypto.Hash160(data)

	if len(hash) != 20 {
		t.Errorf("Expected 20-byte hash, got %d bytes", len(hash))
	}
}

func TestPublicKeyToScriptHash(t *testing.T) {
	keyPair, _ := crypto.GenerateKeyPair()
	pubKeyBytes := crypto.PublicKeyToBytes(keyPair.PublicKey)

	scriptHash := crypto.PublicKeyToScriptHash(pubKeyBytes)

	if len(scriptHash) != 20 {
		t.Errorf("Expected 20-byte script hash, got %d bytes", len(scriptHash))
	}
}

func TestScriptHashToAddress(t *testing.T) {
	// Test with a known script hash
	scriptHash := make([]byte, 20)
	for i := range scriptHash {
		scriptHash[i] = byte(i)
	}

	address := crypto.ScriptHashToAddress(scriptHash)

	if len(address) == 0 {
		t.Error("Address should not be empty")
	}

	// Neo N3 addresses start with 'N'
	if address[0] != 'N' {
		t.Errorf("Neo N3 address should start with 'N', got '%c'", address[0])
	}
}

func TestZeroBytes(t *testing.T) {
	data := []byte("sensitive data")
	crypto.ZeroBytes(data)

	for i, b := range data {
		if b != 0 {
			t.Errorf("Byte at index %d should be 0, got %d", i, b)
		}
	}
}

func TestGenerateRandomBytes(t *testing.T) {
	bytes1, err := crypto.GenerateRandomBytes(32)
	if err != nil {
		t.Fatalf("Failed to generate random bytes: %v", err)
	}

	bytes2, err := crypto.GenerateRandomBytes(32)
	if err != nil {
		t.Fatalf("Failed to generate random bytes: %v", err)
	}

	if hex.EncodeToString(bytes1) == hex.EncodeToString(bytes2) {
		t.Error("Random bytes should be different")
	}
}

func BenchmarkEncryption(b *testing.B) {
	key := make([]byte, 32)
	copy(key, []byte("benchmark-key-32-bytes-long!!!!"))
	plaintext := make([]byte, 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = crypto.Encrypt(key, plaintext)
	}
}

func BenchmarkSigning(b *testing.B) {
	keyPair, _ := crypto.GenerateKeyPair()
	data := make([]byte, 256)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = crypto.Sign(keyPair.PrivateKey, data)
	}
}

func BenchmarkVRF(b *testing.B) {
	keyPair, _ := crypto.GenerateKeyPair()
	alpha := []byte("benchmark seed")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = crypto.GenerateVRF(keyPair.PrivateKey, alpha)
	}
}

// TestContext verifies context cancellation works correctly
func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	select {
	case <-ctx.Done():
		// Expected
	case <-time.After(200 * time.Millisecond):
		t.Error("Context should have been canceled")
	}
}
