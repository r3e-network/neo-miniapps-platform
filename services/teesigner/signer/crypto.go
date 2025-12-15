package signer

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/hkdf"
)

const (
	KeyVersionV1 = "v1"
)

var (
	hkdfSalt = []byte("tee-signer")
)

func DeriveP256PrivateKey(masterKeySeed []byte, keyVersion string) (*ecdsa.PrivateKey, error) {
	if len(masterKeySeed) == 0 {
		return nil, fmt.Errorf("MASTER_KEY_SEED is required")
	}
	keyVersion = strings.TrimSpace(keyVersion)
	if keyVersion == "" {
		return nil, fmt.Errorf("keyVersion is required")
	}

	info := []byte("neo-signer-" + keyVersion)
	reader := hkdf.New(sha256.New, masterKeySeed, hkdfSalt, info)

	okm := make([]byte, 32)
	if _, err := io.ReadFull(reader, okm); err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	curve := elliptic.P256()
	n := curve.Params().N

	// Map OKM into [1, n-1] to avoid invalid private keys.
	d := new(big.Int).SetBytes(okm)
	nMinusOne := new(big.Int).Sub(n, big.NewInt(1))
	d.Mod(d, nMinusOne)
	d.Add(d, big.NewInt(1))

	priv := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: curve,
		},
		D: d,
	}
	priv.PublicKey.X, priv.PublicKey.Y = curve.ScalarBaseMult(d.Bytes())
	if priv.PublicKey.X == nil || priv.PublicKey.Y == nil || !curve.IsOnCurve(priv.PublicKey.X, priv.PublicKey.Y) {
		return nil, fmt.Errorf("derived key is not on curve")
	}

	return priv, nil
}

func KeyVersionFromTime(t time.Time) string {
	return "v" + strconv.FormatInt(t.Unix(), 10)
}

func SignHashP256(randReader io.Reader, privateKey *ecdsa.PrivateKey, hash []byte) ([]byte, error) {
	if privateKey == nil {
		return nil, fmt.Errorf("privateKey is required")
	}
	if randReader == nil {
		randReader = rand.Reader
	}
	if len(hash) == 0 {
		return nil, fmt.Errorf("hash is required")
	}

	r, s, err := ecdsa.Sign(randReader, privateKey, hash)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	// Neo N3 uses 64-byte signature (r || s), each 32 bytes.
	signature := make([]byte, 64)
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	copy(signature[32-len(rBytes):32], rBytes)
	copy(signature[64-len(sBytes):64], sBytes)
	return signature, nil
}

func DecodeTxHashHex(raw string) (canonical string, hash []byte, err error) {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "0x")
	trimmed = strings.TrimPrefix(trimmed, "0X")
	if trimmed == "" {
		return "", nil, fmt.Errorf("tx_hash is required")
	}

	decoded, err := hex.DecodeString(trimmed)
	if err != nil {
		return "", nil, fmt.Errorf("tx_hash must be hex: %w", err)
	}
	if len(decoded) != 32 {
		return "", nil, fmt.Errorf("tx_hash must be 32 bytes, got %d", len(decoded))
	}

	return "0x" + hex.EncodeToString(decoded), decoded, nil
}
