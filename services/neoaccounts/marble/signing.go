// Package neoaccounts provides transaction signing for the neoaccounts service.
package neoaccountsmarble

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"fmt"
	"math/big"

	intcrypto "github.com/R3E-Network/service_layer/internal/crypto"
)

// SignTransaction signs a transaction hash with an account's private key.
// The account must be locked by the requesting service.
func (s *Service) SignTransaction(ctx context.Context, serviceID string, accountID string, txHash []byte) (*SignTransactionResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	acc, err := s.repo.GetByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("account not found: %w", err)
	}

	if acc.LockedBy != serviceID {
		return nil, fmt.Errorf("account not locked by service %s", serviceID)
	}

	priv, err := s.getPrivateKey(accountID)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	signature, err := signHash(priv, txHash)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	pubBytes := intcrypto.PublicKeyToBytes(&priv.PublicKey)

	return &SignTransactionResponse{
		AccountID: accountID,
		Signature: signature,
		PublicKey: pubBytes,
	}, nil
}

// BatchSign signs multiple transaction hashes.
func (s *Service) BatchSign(ctx context.Context, serviceID string, requests []SignRequest) *BatchSignResponse {
	resp := &BatchSignResponse{
		Signatures: make([]SignTransactionResponse, 0, len(requests)),
		Errors:     make([]string, 0),
	}

	for _, req := range requests {
		sig, err := s.SignTransaction(ctx, serviceID, req.AccountID, req.TxHash)
		if err != nil {
			resp.Errors = append(resp.Errors, fmt.Sprintf("%s: %v", req.AccountID, err))
			continue
		}
		resp.Signatures = append(resp.Signatures, *sig)
	}

	return resp
}

// signHash signs a hash using ECDSA.
func signHash(priv *ecdsa.PrivateKey, hash []byte) ([]byte, error) {
	r, s, err := ecdsa.Sign(rand.Reader, priv, hash)
	if err != nil {
		return nil, err
	}

	rBytes := r.Bytes()
	sBytes := s.Bytes()

	signature := make([]byte, 64)
	copy(signature[32-len(rBytes):32], rBytes)
	copy(signature[64-len(sBytes):64], sBytes)

	return signature, nil
}

// verifySignature verifies an ECDSA signature.
func verifySignature(pub *ecdsa.PublicKey, hash, signature []byte) bool {
	if len(signature) != 64 {
		return false
	}

	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])

	return ecdsa.Verify(pub, hash, r, s)
}
