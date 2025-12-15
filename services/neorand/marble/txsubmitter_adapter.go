// Package neorand provides VRF randomness service.
package neorand

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/R3E-Network/service_layer/internal/crypto"
	txclient "github.com/R3E-Network/service_layer/services/txsubmitter/client"
)

// =============================================================================
// TxSubmitter Adapter
// =============================================================================

// TxSubmitterAdapter wraps TxSubmitter client for NeoRand chain operations.
// This replaces direct TEEFulfiller usage for centralized chain writes.
type TxSubmitterAdapter struct {
	client *txclient.Client
}

// NewTxSubmitterAdapter creates a new TxSubmitter adapter for NeoRand.
func NewTxSubmitterAdapter(client *txclient.Client) *TxSubmitterAdapter {
	return &TxSubmitterAdapter{
		client: client,
	}
}

// FulfillRequest submits a VRF fulfillment via TxSubmitter.
func (a *TxSubmitterAdapter) FulfillRequest(ctx context.Context, requestID *big.Int, resultBytes []byte) (string, error) {
	if a.client == nil {
		return "", fmt.Errorf("txsubmitter client not configured")
	}

	resp, err := a.client.FulfillRequest(ctx, requestID.String(), hex.EncodeToString(resultBytes))
	if err != nil {
		return "", fmt.Errorf("fulfill request via txsubmitter: %w", err)
	}

	if resp.Error != "" {
		return "", fmt.Errorf("txsubmitter error: %s", resp.Error)
	}

	return resp.TxHash, nil
}

// FailRequest submits a VRF failure via TxSubmitter.
func (a *TxSubmitterAdapter) FailRequest(ctx context.Context, requestID *big.Int, errorMsg string) (string, error) {
	if a.client == nil {
		return "", fmt.Errorf("txsubmitter client not configured")
	}

	resp, err := a.client.FailRequest(ctx, requestID.String(), errorMsg)
	if err != nil {
		return "", fmt.Errorf("fail request via txsubmitter: %w", err)
	}

	if resp.Error != "" {
		return "", fmt.Errorf("txsubmitter error: %s", resp.Error)
	}

	return resp.TxHash, nil
}

// =============================================================================
// Service Integration
// =============================================================================

// SetTxSubmitterClient sets the TxSubmitter client for chain operations.
// This enables migration from direct TEEFulfiller to centralized TxSubmitter.
func (s *Service) SetTxSubmitterClient(client *txclient.Client) {
	s.txSubmitterAdapter = NewTxSubmitterAdapter(client)
}

// fulfillRequestViaTxSubmitter generates randomness and submits via TxSubmitter.
func (s *Service) fulfillRequestViaTxSubmitter(ctx context.Context, request *Request) {
	if s.txSubmitterAdapter == nil {
		// Fallback to legacy TEEFulfiller
		s.fulfillRequest(ctx, request)
		return
	}

	// Generate VRF proof
	seedBytes, err := hex.DecodeString(request.Seed)
	if err != nil {
		seedBytes = []byte(request.Seed)
	}

	vrfProof, err := crypto.GenerateVRF(s.privateKey, seedBytes)
	if err != nil {
		s.markRequestFailedViaTxSubmitter(ctx, request, fmt.Sprintf("generate VRF: %v", err))
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

	// Parse request ID
	requestIDBig := new(big.Int)
	if _, ok := requestIDBig.SetString(request.RequestID, 10); !ok {
		requestIDBig.SetString(request.RequestID, 16)
	}

	// Encode random words as bytes for callback
	resultBytes := make([]byte, 0, 4+len(randomWordsBig)*32)
	resultBytes = append(resultBytes, byte(len(randomWordsBig)))
	for _, word := range randomWordsBig {
		wordBytes := word.Bytes()
		padded := make([]byte, 32)
		copy(padded[32-len(wordBytes):], wordBytes)
		resultBytes = append(resultBytes, padded...)
	}

	// Submit via TxSubmitter
	txHash, err := s.txSubmitterAdapter.FulfillRequest(ctx, requestIDBig, resultBytes)
	if err != nil {
		s.Logger().WithContext(ctx).WithError(err).WithFields(map[string]interface{}{
			"request_id": request.RequestID,
		}).Error("failed to fulfill VRF request via TxSubmitter")
		s.markRequestFailedViaTxSubmitter(ctx, request, err.Error())
		return
	}

	s.Logger().WithContext(ctx).WithFields(map[string]interface{}{
		"request_id": request.RequestID,
		"tx_hash":    txHash,
		"num_words":  request.NumWords,
	}).Info("VRF request fulfilled via TxSubmitter")

	// Update request status in memory and persist
	s.mu.Lock()
	request.Status = StatusFulfilled
	request.RandomWords = randomWords
	request.Proof = hex.EncodeToString(vrfProof.Proof)
	request.FulfilledAt = time.Now()
	s.mu.Unlock()

	if s.repo != nil {
		if err := s.repo.Update(ctx, neorandRecordFromReq(request)); err != nil {
			s.Logger().WithContext(ctx).WithError(err).WithField("request_id", request.RequestID).Warn("failed to persist fulfilled request")
		}
	}
}

// markRequestFailedViaTxSubmitter marks a request as failed via TxSubmitter.
func (s *Service) markRequestFailedViaTxSubmitter(ctx context.Context, request *Request, errorMsg string) {
	if s.txSubmitterAdapter == nil {
		s.markRequestFailed(ctx, request, errorMsg)
		return
	}

	requestIDBig := new(big.Int)
	if _, ok := requestIDBig.SetString(request.RequestID, 10); !ok {
		requestIDBig.SetString(request.RequestID, 16)
	}

	_, err := s.txSubmitterAdapter.FailRequest(ctx, requestIDBig, errorMsg)
	if err != nil {
		s.Logger().WithContext(ctx).WithError(err).WithFields(map[string]interface{}{
			"request_id": request.RequestID,
		}).Error("failed to mark request as failed via TxSubmitter")
	}
}
