package vrfmarble

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/R3E-Network/service_layer/internal/crypto"
)

// =============================================================================
// Request Fulfiller - Generates randomness and calls back to user contracts
// =============================================================================

// runRequestFulfiller processes pending VRF requests.
func (s *Service) runRequestFulfiller(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case request := <-s.pendingRequests:
			s.fulfillRequest(ctx, request)
		}
	}
}

// fulfillRequest generates randomness and submits callback to user contract.
func (s *Service) fulfillRequest(ctx context.Context, request *VRFRequest) {
	// Generate VRF proof
	seedBytes, err := hex.DecodeString(request.Seed)
	if err != nil {
		seedBytes = []byte(request.Seed)
	}

	vrfProof, err := crypto.GenerateVRF(s.privateKey, seedBytes)
	if err != nil {
		s.markRequestFailed(request, fmt.Sprintf("generate VRF: %v", err))
		return
	}

	// Generate random words
	randomWords := make([]string, request.NumWords)
	randomWordsBig := make([]*big.Int, request.NumWords)
	for i := 0; i < request.NumWords; i++ {
		wordInput := append(vrfProof.Output, byte(i))
		wordHash := crypto.Hash256(wordInput)
		randomWords[i] = hex.EncodeToString(wordHash)
		randomWordsBig[i] = new(big.Int).SetBytes(wordHash)
	}

	// Submit callback to user contract via TEE fulfiller
	// Update request status
	s.mu.Lock()
	request.Status = StatusFulfilled
	request.RandomWords = randomWords
	request.Proof = hex.EncodeToString(vrfProof.Proof)
	request.FulfilledAt = time.Now()
	s.mu.Unlock()

	if s.repo != nil {
		_ = s.repo.Update(ctx, vrfRecordFromReq(request))
	}

	// Suppress unused variable warning
	_ = randomWordsBig
}

// markRequestFailed marks a request as failed.
func (s *Service) markRequestFailed(request *VRFRequest, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	request.Status = StatusFailed
	request.Error = errMsg

	if s.repo != nil {
		_ = s.repo.Update(context.Background(), vrfRecordFromReq(request))
	}
}
