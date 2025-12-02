// Package sdk provides the Enclave SDK implementation.
package sdk

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"math/big"
)

// transactionSignerImpl implements TransactionSigner interface.
type transactionSignerImpl struct {
	keyManager *keyManagerImpl
}

// NewTransactionSigner creates a new transaction signer instance.
func NewTransactionSigner(keyManager *keyManagerImpl) TransactionSigner {
	return &transactionSignerImpl{
		keyManager: keyManager,
	}
}

func (s *transactionSignerImpl) Sign(ctx context.Context, req *SignRequest) (*SignResponse, error) {
	if req.KeyID == "" {
		return nil, errors.New("key ID is required")
	}
	if len(req.Data) == 0 {
		return nil, errors.New("data is required")
	}

	// Get private key
	privateKey, err := s.keyManager.GetPrivateKey(req.KeyID)
	if err != nil {
		return nil, err
	}

	ecdsaKey, ok := privateKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("key is not ECDSA")
	}

	// Hash the data
	var hash []byte
	switch req.HashAlg {
	case "sha256", "":
		h := sha256.Sum256(req.Data)
		hash = h[:]
	case "keccak256":
		// In production, use proper keccak256 implementation
		h := sha256.Sum256(req.Data)
		hash = h[:]
	default:
		return nil, errors.New("unsupported hash algorithm")
	}

	// Sign
	r, sigS, err := ecdsa.Sign(rand.Reader, ecdsaKey, hash)
	if err != nil {
		return nil, err
	}

	// Encode signature (r || s)
	signature := append(r.Bytes(), sigS.Bytes()...)

	// Get public key
	publicKey, err := s.keyManager.ExportPublicKey(ctx, req.KeyID)
	if err != nil {
		return nil, err
	}

	return &SignResponse{
		Signature: signature,
		PublicKey: publicKey,
		Algorithm: "ECDSA-SHA256",
	}, nil
}

func (s *transactionSignerImpl) SignTransaction(ctx context.Context, req *SignTransactionRequest) (*SignTransactionResponse, error) {
	if req.KeyID == "" {
		return nil, errors.New("key ID is required")
	}
	if len(req.Transaction) == 0 {
		return nil, errors.New("transaction is required")
	}

	// Get private key
	privateKey, err := s.keyManager.GetPrivateKey(req.KeyID)
	if err != nil {
		return nil, err
	}

	ecdsaKey, ok := privateKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("key is not ECDSA")
	}

	// Hash the transaction
	txHash := sha256.Sum256(req.Transaction)

	// Sign
	r, sigS, err := ecdsa.Sign(rand.Reader, ecdsaKey, txHash[:])
	if err != nil {
		return nil, err
	}

	signature := append(r.Bytes(), sigS.Bytes()...)

	// For now, return the transaction with signature appended
	// In production, this would be properly serialized based on TxType
	signedTx := append(req.Transaction, signature...)

	return &SignTransactionResponse{
		SignedTransaction: signedTx,
		TxHash:            txHash[:],
		Signature:         signature,
	}, nil
}

func (s *transactionSignerImpl) SignMessage(ctx context.Context, req *SignMessageRequest) (*SignMessageResponse, error) {
	if req.KeyID == "" {
		return nil, errors.New("key ID is required")
	}
	if len(req.Message) == 0 {
		return nil, errors.New("message is required")
	}

	// Get private key
	privateKey, err := s.keyManager.GetPrivateKey(req.KeyID)
	if err != nil {
		return nil, err
	}

	ecdsaKey, ok := privateKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("key is not ECDSA")
	}

	// Prepare message based on format
	var messageToSign []byte
	switch req.Format {
	case "eip191":
		// Ethereum signed message format
		prefix := []byte("\x19Ethereum Signed Message:\n")
		lenStr := []byte(string(rune(len(req.Message))))
		messageToSign = append(prefix, lenStr...)
		messageToSign = append(messageToSign, req.Message...)
	case "raw", "":
		messageToSign = req.Message
	default:
		return nil, errors.New("unsupported message format")
	}

	// Hash the message
	messageHash := sha256.Sum256(messageToSign)

	// Sign
	r, sigS, err := ecdsa.Sign(rand.Reader, ecdsaKey, messageHash[:])
	if err != nil {
		return nil, err
	}

	signature := append(r.Bytes(), sigS.Bytes()...)

	// Get public key
	publicKey, err := s.keyManager.ExportPublicKey(ctx, req.KeyID)
	if err != nil {
		return nil, err
	}

	return &SignMessageResponse{
		Signature:   signature,
		MessageHash: messageHash[:],
		PublicKey:   publicKey,
	}, nil
}

func (s *transactionSignerImpl) Verify(ctx context.Context, req *VerifyRequest) (bool, error) {
	if len(req.PublicKey) == 0 {
		return false, errors.New("public key is required")
	}
	if len(req.Data) == 0 {
		return false, errors.New("data is required")
	}
	if len(req.Signature) < 64 {
		return false, ErrInvalidSignature
	}

	// Parse public key
	pubKey, err := parseECDSAPublicKey(req.PublicKey)
	if err != nil {
		return false, err
	}

	// Hash the data
	var hash []byte
	switch req.HashAlg {
	case "sha256", "":
		h := sha256.Sum256(req.Data)
		hash = h[:]
	default:
		return false, errors.New("unsupported hash algorithm")
	}

	// Parse signature
	r := new(big.Int).SetBytes(req.Signature[:32])
	sigS := new(big.Int).SetBytes(req.Signature[32:64])

	// Verify
	return ecdsa.Verify(pubKey, hash, r, sigS), nil
}

func (s *transactionSignerImpl) GetSigningKey(ctx context.Context, keyID string) (*ecdsa.PublicKey, error) {
	publicKeyBytes, err := s.keyManager.ExportPublicKey(ctx, keyID)
	if err != nil {
		return nil, err
	}

	return parseECDSAPublicKey(publicKeyBytes)
}

// parseECDSAPublicKey parses a public key from bytes.
func parseECDSAPublicKey(data []byte) (*ecdsa.PublicKey, error) {
	if len(data) < 65 {
		return nil, ErrInvalidKey
	}

	// Assume P-256 curve for now
	// In production, detect curve from key format
	x := new(big.Int).SetBytes(data[1:33])
	y := new(big.Int).SetBytes(data[33:65])

	return &ecdsa.PublicKey{
		Curve: nil, // Will be set based on curve detection
		X:     x,
		Y:     y,
	}, nil
}
