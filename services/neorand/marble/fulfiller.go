package neorand

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
		case <-s.StopChan():
			return
		case request := <-s.pendingRequests:
			s.fulfillRequest(ctx, request)
		}
	}
}

// fulfillRequest generates randomness and submits callback to user contract.
func (s *Service) fulfillRequest(ctx context.Context, request *Request) {
	// Generate VRF proof
	seedBytes, err := hex.DecodeString(request.Seed)
	if err != nil {
		seedBytes = []byte(request.Seed)
	}

	vrfProof, err := crypto.GenerateVRF(s.privateKey, seedBytes)
	if err != nil {
		s.markRequestFailed(ctx, request, fmt.Sprintf("generate VRF: %v", err))
		return
	}

	// Generate random words
	randomWords := make([]string, request.NumWords)
	randomWordsBig := make([]*big.Int, request.NumWords)
	for i := 0; i < request.NumWords; i++ {
		wordInput := make([]byte, 0, len(vrfProof.Output)+1)
		wordInput = append(wordInput, vrfProof.Output...)
		wordInput = append(wordInput, byte(i))
		wordHash := crypto.Hash256(wordInput)
		randomWords[i] = hex.EncodeToString(wordHash)
		randomWordsBig[i] = new(big.Int).SetBytes(wordHash)
	}

	// Submit callback to user contract via TEE fulfiller (if available)
	if s.teeFulfiller != nil && request.RequestID != "" {
		// Parse request ID to big.Int
		requestIDBig := new(big.Int)
		if _, ok := requestIDBig.SetString(request.RequestID, 10); !ok {
			// Try hex format
			requestIDBig.SetString(request.RequestID, 16)
		}

		// Encode random words as bytes for callback
		// Format: [numWords][word1][word2]...
		resultBytes := make([]byte, 0, 4+len(randomWordsBig)*32)
		resultBytes = append(resultBytes, byte(len(randomWordsBig)))
		for _, word := range randomWordsBig {
			wordBytes := word.Bytes()
			// Pad to 32 bytes
			padded := make([]byte, 32)
			copy(padded[32-len(wordBytes):], wordBytes)
			resultBytes = append(resultBytes, padded...)
		}

		// Submit to chain
		txHash, err := s.teeFulfiller.FulfillRequest(ctx, requestIDBig, resultBytes)
		if err != nil {
			s.markRequestFailed(ctx, request, fmt.Sprintf("chain callback failed: %v", err))
			return
		}

		// Log successful submission
		s.Logger().WithContext(ctx).WithFields(map[string]any{
			"request_id": request.RequestID,
			"tx_hash":    txHash,
		}).Info("request fulfilled on-chain")
	}

	// Update request status after successful chain submission
	s.mu.Lock()
	request.Status = StatusFulfilled
	request.RandomWords = randomWords
	request.Proof = hex.EncodeToString(vrfProof.Proof)
	request.FulfilledAt = time.Now()
	s.mu.Unlock()

	if s.repo != nil {
		updateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := s.repo.Update(updateCtx, neorandRecordFromReq(request)); err != nil {
			s.Logger().WithContext(ctx).WithError(err).WithField("request_id", request.RequestID).Warn("failed to persist fulfilled request")
		}
	}
}

// markRequestFailed marks a request as failed.
func (s *Service) markRequestFailed(ctx context.Context, request *Request, errMsg string) {
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	request.Status = StatusFailed
	request.Error = errMsg
	s.mu.Unlock()

	if s.repo == nil {
		return
	}

	updateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.repo.Update(updateCtx, neorandRecordFromReq(request)); err != nil {
		s.Logger().WithContext(ctx).WithError(err).WithFields(map[string]any{
			"request_id": request.RequestID,
			"status":     StatusFailed,
		}).Warn("failed to persist failed request")
	}
}
