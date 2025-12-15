package crypto

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"math/big"
	"strings"
	"testing"
)

func TestHMACSignAndVerify(t *testing.T) {
	key := []byte("test-key")
	data := []byte("test-data")

	sig := HMACSign(key, data)
	if len(sig) != 32 {
		t.Fatalf("HMACSign() len = %d, want 32", len(sig))
	}
	if !HMACVerify(key, data, sig) {
		t.Fatalf("HMACVerify() returned false for valid signature")
	}
	if HMACVerify(key, []byte("other-data"), sig) {
		t.Fatalf("HMACVerify() returned true for wrong data")
	}

	badSig := append([]byte(nil), sig...)
	badSig[0] ^= 0xFF
	if HMACVerify(key, data, badSig) {
		t.Fatalf("HMACVerify() returned true for tampered signature")
	}
}

func TestDeriveKey_ReturnsErrorWhenRequestedTooLong(t *testing.T) {
	masterKey := []byte("test-master-key-32-bytes-long!!")
	salt := []byte("test-salt")

	// HKDF is limited to 255*HashLen bytes (HashLen=32 for SHA256 => 8160 bytes).
	_, err := DeriveKey(masterKey, salt, "purpose", 9000)
	if err == nil || !strings.Contains(err.Error(), "derive key") {
		t.Fatalf("DeriveKey() error = %v, want wrapped derive key error", err)
	}
}

func TestEncryptDecrypt_InvalidKeyLength(t *testing.T) {
	key := []byte("short-key")
	if _, err := Encrypt(key, []byte("hello")); err == nil {
		t.Fatalf("Encrypt() expected error for invalid key length")
	}
	if _, err := Decrypt(key, []byte("ciphertext")); err == nil {
		t.Fatalf("Decrypt() expected error for invalid key length")
	}
}

func TestPublicKeyFromBytes_RoundTripCompressedAndUncompressed(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	compressed := PublicKeyToBytes(kp.PublicKey)
	parsed, err := PublicKeyFromBytes(compressed)
	if err != nil {
		t.Fatalf("PublicKeyFromBytes(compressed): %v", err)
	}
	if parsed.X.Cmp(kp.PublicKey.X) != 0 || parsed.Y.Cmp(kp.PublicKey.Y) != 0 {
		t.Fatalf("compressed parse mismatch")
	}

	curve := elliptic.P256()
	byteLen := (curve.Params().BitSize + 7) / 8
	uncompressed := make([]byte, 1+2*byteLen)
	uncompressed[0] = 0x04
	xBytes := kp.PublicKey.X.Bytes()
	yBytes := kp.PublicKey.Y.Bytes()
	copy(uncompressed[1+byteLen-len(xBytes):1+byteLen], xBytes)
	copy(uncompressed[1+2*byteLen-len(yBytes):], yBytes)
	parsed2, err := PublicKeyFromBytes(uncompressed)
	if err != nil {
		t.Fatalf("PublicKeyFromBytes(uncompressed): %v", err)
	}
	if parsed2.X.Cmp(kp.PublicKey.X) != 0 || parsed2.Y.Cmp(kp.PublicKey.Y) != 0 {
		t.Fatalf("uncompressed parse mismatch")
	}
}

func TestPublicKeyFromBytes_CompressedParityFlip(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	compressed := PublicKeyToBytes(kp.PublicKey)
	flipped := append([]byte(nil), compressed...)
	if flipped[0] == 0x02 {
		flipped[0] = 0x03
	} else {
		flipped[0] = 0x02
	}

	parsed, err := PublicKeyFromBytes(flipped)
	if err != nil {
		t.Fatalf("PublicKeyFromBytes(flipped): %v", err)
	}
	if parsed.X.Cmp(kp.PublicKey.X) != 0 {
		t.Fatalf("X mismatch after parity flip")
	}
	if parsed.Y.Cmp(kp.PublicKey.Y) == 0 {
		t.Fatalf("expected Y to differ after parity flip")
	}

	expectedY := new(big.Int).Sub(elliptic.P256().Params().P, kp.PublicKey.Y)
	if parsed.Y.Cmp(expectedY) != 0 {
		t.Fatalf("expected Y == P - original Y after parity flip")
	}
}

func TestPublicKeyFromBytes_InvalidInputs(t *testing.T) {
	// Invalid length.
	if _, err := PublicKeyFromBytes([]byte{0x02, 0x01}); err == nil {
		t.Fatalf("expected error for invalid public key length")
	}

	// Invalid uncompressed prefix.
	badUncompressed := make([]byte, 65)
	badUncompressed[0] = 0x05
	if _, err := PublicKeyFromBytes(badUncompressed); err == nil {
		t.Fatalf("expected error for invalid uncompressed public key prefix")
	}
}

func TestPublicKeyFromBytes_InvalidCompressedPoint(t *testing.T) {
	curve := elliptic.P256()

	var invalidX *big.Int
	for i := 0; i < 10_000; i++ {
		x := big.NewInt(int64(i))
		if y := decompressPoint(curve, x, false); y == nil {
			invalidX = x
			break
		}
	}
	if invalidX == nil {
		t.Fatalf("failed to find an invalid x-coordinate candidate")
	}

	xBytes := invalidX.Bytes()
	xPadded := make([]byte, 32)
	copy(xPadded[32-len(xBytes):], xBytes)

	compressed := make([]byte, 33)
	compressed[0] = 0x02
	copy(compressed[1:], xPadded)

	if _, err := PublicKeyFromBytes(compressed); err == nil {
		t.Fatalf("expected error for invalid compressed public key")
	}
}

func TestPublicKeyToAddress_DeterministicAndValidPrefix(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	addr1 := PublicKeyToAddress(kp.PublicKey)
	addr2 := PublicKeyToAddress(kp.PublicKey)

	if addr1 == "" || addr2 == "" {
		t.Fatalf("PublicKeyToAddress() returned empty address")
	}
	if addr1 != addr2 {
		t.Fatalf("PublicKeyToAddress() not deterministic")
	}
	if addr1[0] != 'N' {
		t.Fatalf("PublicKeyToAddress() prefix = %q, want 'N'", addr1[0])
	}
}

func TestVRFProofHelpersAndErrorPaths(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	// Serialize nil proof.
	if got := SerializeVRFProof(nil); got != nil {
		t.Fatalf("SerializeVRFProof(nil) = %v, want nil", got)
	}

	// Deserialize invalid proof lengths.
	if _, proofErr := DeserializeVRFProof([]byte{0x01}); proofErr == nil {
		t.Fatalf("expected error for invalid proof length")
	}

	// Deserialize invalid Gamma point.
	invalid := make([]byte, 97) // all zeros => invalid compressed point
	if _, proofErr := DeserializeVRFProof(invalid); proofErr == nil {
		t.Fatalf("expected error for invalid Gamma point")
	}

	// VerifyVRF wrapper should reject invalid proof bytes.
	if VerifyVRF(kp.PublicKey, []byte("alpha"), &VRFProof{Proof: []byte("short")}) {
		t.Fatalf("VerifyVRF() returned true for invalid proof format")
	}

	// VerifyVRFProof should reject invalid Gamma points.
	proof, err := GenerateVRF(kp.PrivateKey, []byte("alpha"))
	if err != nil {
		t.Fatalf("GenerateVRF: %v", err)
	}
	proofData, err := DeserializeVRFProof(proof.Proof)
	if err != nil {
		t.Fatalf("DeserializeVRFProof(valid): %v", err)
	}

	badProof := *proofData
	badProof.GammaX = big.NewInt(0)
	badProof.GammaY = big.NewInt(0)
	if _, ok := VerifyVRFProof(kp.PublicKey, []byte("alpha"), &badProof); ok {
		t.Fatalf("VerifyVRFProof() returned true for invalid Gamma point")
	}

	// VerifyVRF wrapper should reject mismatched output.
	proof.Output = bytes.Repeat([]byte{0xFF}, len(proof.Output))
	if VerifyVRF(kp.PublicKey, []byte("alpha"), proof) {
		t.Fatalf("VerifyVRF() returned true for mismatched output")
	}
}

func TestGenerateVRFProof_ErrorPaths(t *testing.T) {
	if _, err := GenerateVRFProof(nil, []byte("alpha")); err == nil {
		t.Fatalf("expected error when private key is nil")
	}

	// Wrong curve.
	privWrong, err := ecdsa.GenerateKey(elliptic.P224(), crand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey(P224): %v", err)
	}
	if _, err := GenerateVRFProof(privWrong, []byte("alpha")); err == nil {
		t.Fatalf("expected error for unsupported curve")
	}
}
