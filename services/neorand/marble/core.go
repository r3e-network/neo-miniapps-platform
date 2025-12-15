package neorand

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/R3E-Network/service_layer/internal/crypto"
)

// =============================================================================
// Core Logic
// =============================================================================

// GenerateRandomness generates verifiable random numbers.
func (s *Service) GenerateRandomness(ctx context.Context, seed string, numWords int) (*DirectRandomResponse, error) {
	if numWords <= 0 {
		numWords = 1
	}
	if numWords > MaxNumWords {
		numWords = MaxNumWords
	}

	seedBytes, err := hex.DecodeString(seed)
	if err != nil {
		seedBytes = []byte(seed)
	}

	// Generate VRF proof
	vrfProof, err := crypto.GenerateVRF(s.privateKey, seedBytes)
	if err != nil {
		return nil, fmt.Errorf("generate VRF: %w", err)
	}

	// Generate multiple random words from the VRF output
	randomWords := make([]string, numWords)
	for i := 0; i < numWords; i++ {
		wordInput := make([]byte, 0, len(vrfProof.Output)+1)
		wordInput = append(wordInput, vrfProof.Output...)
		wordInput = append(wordInput, byte(i))
		wordHash := crypto.Hash256(wordInput)
		randomWords[i] = hex.EncodeToString(wordHash)
	}

	return &DirectRandomResponse{
		RequestID:   uuid.New().String(),
		Seed:        seed,
		RandomWords: randomWords,
		Proof:       hex.EncodeToString(vrfProof.Proof),
		PublicKey:   hex.EncodeToString(vrfProof.PublicKey),
		Timestamp:   time.Now().Format(time.RFC3339),
	}, nil
}

// VerifyRandomness verifies a VRF proof.
func (s *Service) VerifyRandomness(req *VerifyRequest) (bool, error) {
	seedBytes, err := hex.DecodeString(req.Seed)
	if err != nil {
		seedBytes = []byte(req.Seed)
	}

	proofBytes, err := hex.DecodeString(req.Proof)
	if err != nil {
		return false, fmt.Errorf("invalid proof hex: %w", err)
	}

	pubKeyBytes, err := hex.DecodeString(req.PublicKey)
	if err != nil {
		return false, fmt.Errorf("invalid public key hex: %w", err)
	}

	// Parse public key
	if len(pubKeyBytes) != 33 {
		return false, fmt.Errorf("invalid public key length")
	}

	// Decompress public key
	x, y := elliptic.UnmarshalCompressed(elliptic.P256(), pubKeyBytes)
	if x == nil {
		return false, fmt.Errorf("invalid compressed public key")
	}

	publicKey := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     y,
	}

	// Deserialize and verify VRF proof directly
	proofData, err := crypto.DeserializeVRFProof(proofBytes)
	if err != nil {
		return false, fmt.Errorf("invalid proof format: %w", err)
	}

	// VerifyVRFProof returns (beta, valid) - we only need validity
	_, valid := crypto.VerifyVRFProof(publicKey, seedBytes, proofData)
	return valid, nil
}
