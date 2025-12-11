// Package neovault provides lifecycle management for the privacy neovault service.
package neovaultmarble

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
)

// =============================================================================
// Lifecycle
// =============================================================================

// Start starts the neovault service and background workers.
func (s *Service) Start(ctx context.Context) error {
	if err := s.BaseService.Start(ctx); err != nil {
		return err
	}

	// Resume deposited/mixing requests from persistence
	go s.resumeRequests(ctx)

	// Start background workers (pool management now in neoaccounts service)
	go s.runMixingLoop(ctx)
	go s.runDeliveryChecker(ctx)

	return nil
}

// Stop stops the neovault service.
func (s *Service) Stop() error {
	return s.BaseService.Stop()
}

// resumeRequests loads requests in deposited/mixing state and resumes processing.
func (s *Service) resumeRequests(ctx context.Context) {
	if s.repo == nil {
		return
	}

	// Kick off mixing for deposited requests
	if deposited, err := s.repo.ListByStatus(ctx, string(StatusDeposited)); err == nil {
		for i := range deposited {
			req := RequestFromRecord(&deposited[i])
			go s.startMixing(ctx, req)
		}
	}
}

// =============================================================================
// On-Chain Dispute Submission
// =============================================================================

// submitCompletionProofOnChain submits the completion proof to the on-chain contract.
// This is called ONLY during dispute resolution.
func (s *Service) submitCompletionProofOnChain(ctx context.Context, request *MixRequest) (string, error) {
	if s.teeFulfiller == nil {
		return "", fmt.Errorf("TEE fulfiller not configured")
	}

	proof := request.CompletionProof
	if proof == nil {
		return "", fmt.Errorf("no completion proof")
	}

	// Serialize proof for on-chain submission
	proofBytes, err := json.Marshal(proof)
	if err != nil {
		return "", fmt.Errorf("marshal proof: %w", err)
	}

	// Parse request ID as big.Int for contract call
	requestIDBigInt := new(big.Int)
	// Use hash of request ID as numeric identifier
	idHash := sha256.Sum256([]byte(request.ID))
	requestIDBigInt.SetBytes(idHash[:8])

	// Submit via TEE fulfiller (this is the ONLY on-chain submission in normal neovault flow)
	txHash, err := s.teeFulfiller.FulfillRequest(ctx, requestIDBigInt, proofBytes)
	if err != nil {
		return "", fmt.Errorf("fulfill request: %w", err)
	}

	return txHash, nil
}
