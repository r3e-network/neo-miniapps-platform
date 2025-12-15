package signer

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"math/big"
	"testing"
)

func TestDeriveP256PrivateKey_Deterministic(t *testing.T) {
	seed := []byte("master-seed-for-tests")

	k1, err := DeriveP256PrivateKey(seed, KeyVersionV1)
	if err != nil {
		t.Fatalf("DeriveP256PrivateKey: %v", err)
	}
	k2, err := DeriveP256PrivateKey(seed, KeyVersionV1)
	if err != nil {
		t.Fatalf("DeriveP256PrivateKey: %v", err)
	}

	if k1.D.Cmp(k2.D) != 0 {
		t.Fatalf("expected deterministic D, got %v vs %v", k1.D, k2.D)
	}
	if k1.PublicKey.X.Cmp(k2.PublicKey.X) != 0 || k1.PublicKey.Y.Cmp(k2.PublicKey.Y) != 0 {
		t.Fatal("expected deterministic public key")
	}
}

func TestDeriveP256PrivateKey_VersionChangesKey(t *testing.T) {
	seed := []byte("master-seed-for-tests")

	k1, err := DeriveP256PrivateKey(seed, "v1")
	if err != nil {
		t.Fatalf("DeriveP256PrivateKey(v1): %v", err)
	}
	k2, err := DeriveP256PrivateKey(seed, "v2")
	if err != nil {
		t.Fatalf("DeriveP256PrivateKey(v2): %v", err)
	}

	if k1.D.Cmp(k2.D) == 0 {
		t.Fatal("expected different key material for different versions")
	}
}

func TestDeriveP256PrivateKey_Validation(t *testing.T) {
	if _, err := DeriveP256PrivateKey(nil, KeyVersionV1); err == nil {
		t.Fatal("expected error for empty seed")
	}
	if _, err := DeriveP256PrivateKey([]byte("seed"), ""); err == nil {
		t.Fatal("expected error for empty key version")
	}
}

func TestSignHashP256_Verifies(t *testing.T) {
	priv, err := DeriveP256PrivateKey([]byte("seed"), KeyVersionV1)
	if err != nil {
		t.Fatalf("DeriveP256PrivateKey: %v", err)
	}

	msgHash := sha256.Sum256([]byte("hello"))
	sig, err := SignHashP256(nil, priv, msgHash[:])
	if err != nil {
		t.Fatalf("SignHashP256: %v", err)
	}
	if len(sig) != 64 {
		t.Fatalf("expected 64-byte signature, got %d", len(sig))
	}

	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	if !ecdsa.Verify(&priv.PublicKey, msgHash[:], r, s) {
		t.Fatal("signature did not verify")
	}
}

func TestSignHashP256_Validation(t *testing.T) {
	if _, err := SignHashP256(nil, nil, []byte("x")); err == nil {
		t.Fatal("expected error for nil private key")
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), deterministicReader{})
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	if _, err := SignHashP256(nil, priv, nil); err == nil {
		t.Fatal("expected error for empty hash")
	}
}

func TestDecodeTxHashHex(t *testing.T) {
	canonical, hash, err := DecodeTxHashHex("0X" + repeatHex("A1", 32))
	if err != nil {
		t.Fatalf("DecodeTxHashHex: %v", err)
	}
	if canonical != "0x"+repeatHex("a1", 32) {
		t.Fatalf("unexpected canonical: %s", canonical)
	}
	if len(hash) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(hash))
	}
}

func TestDecodeTxHashHex_Errors(t *testing.T) {
	for _, tc := range []string{
		"",
		"0x",
		"xyz",
		"0x1",                      // odd length
		"0x" + repeatHex("00", 31), // wrong size
		"0x" + repeatHex("00", 33), // wrong size
	} {
		if _, _, err := DecodeTxHashHex(tc); err == nil {
			t.Fatalf("expected error for %q", tc)
		}
	}
}

func repeatHex(bytePair string, count int) string {
	out := make([]byte, 0, len(bytePair)*count)
	for i := 0; i < count; i++ {
		out = append(out, bytePair...)
	}
	return string(out)
}

// deterministicReader is a minimal io.Reader returning a stream of 0x42 bytes.
// It's only used to avoid depending on OS randomness in tests that don't care.
type deterministicReader struct{}

func (deterministicReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0x42
	}
	return len(p), nil
}
