// Package neovaultmarble provides privacy-preserving transaction mixing service.
package neovaultmarble

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	gsclient "github.com/R3E-Network/service_layer/services/globalsigner/client"
	txclient "github.com/R3E-Network/service_layer/services/txsubmitter/client"
)

// =============================================================================
// TxSubmitter + GlobalSigner Adapter
// =============================================================================

// ServiceAdapter wraps TxSubmitter and GlobalSigner clients for NeoVault operations.
// This replaces direct TEEFulfiller and masterKey usage.
type ServiceAdapter struct {
	txClient *txclient.Client
	gsClient *gsclient.Client
}

// NewServiceAdapter creates a new adapter for NeoVault.
func NewServiceAdapter(txClient *txclient.Client, gsClient *gsclient.Client) *ServiceAdapter {
	return &ServiceAdapter{
		txClient: txClient,
		gsClient: gsClient,
	}
}

// =============================================================================
// GlobalSigner Integration (Proof Signing)
// =============================================================================

// SignRequestProof signs a request proof using GlobalSigner.
// This replaces local masterKey signing.
func (a *ServiceAdapter) SignRequestProof(ctx context.Context, requestHash []byte) ([]byte, string, error) {
	if a.gsClient == nil {
		return nil, "", fmt.Errorf("globalsigner client not configured")
	}

	resp, err := a.gsClient.Sign(ctx, &gsclient.SignRequest{
		Domain: "neovault:request",
		Data:   hex.EncodeToString(requestHash),
	})
	if err != nil {
		return nil, "", fmt.Errorf("sign request proof: %w", err)
	}

	sig, err := hex.DecodeString(resp.Signature)
	if err != nil {
		return nil, "", fmt.Errorf("decode signature: %w", err)
	}

	return sig, resp.KeyVersion, nil
}

// SignCompletionProof signs a completion proof using GlobalSigner.
func (a *ServiceAdapter) SignCompletionProof(ctx context.Context, completionHash []byte) ([]byte, string, error) {
	if a.gsClient == nil {
		return nil, "", fmt.Errorf("globalsigner client not configured")
	}

	resp, err := a.gsClient.Sign(ctx, &gsclient.SignRequest{
		Domain: "neovault:completion",
		Data:   hex.EncodeToString(completionHash),
	})
	if err != nil {
		return nil, "", fmt.Errorf("sign completion proof: %w", err)
	}

	sig, err := hex.DecodeString(resp.Signature)
	if err != nil {
		return nil, "", fmt.Errorf("decode signature: %w", err)
	}

	return sig, resp.KeyVersion, nil
}

// GetSignerPublicKey gets the current signer public key for verification.
func (a *ServiceAdapter) GetSignerPublicKey(ctx context.Context) (string, string, error) {
	if a.gsClient == nil {
		return "", "", fmt.Errorf("globalsigner client not configured")
	}

	att, err := a.gsClient.GetAttestation(ctx)
	if err != nil {
		return "", "", fmt.Errorf("get attestation: %w", err)
	}

	return att.PubKeyHex, att.KeyVersion, nil
}

// =============================================================================
// TxSubmitter Integration (Chain Operations)
// =============================================================================

// FulfillRequest submits a fulfill request via TxSubmitter.
func (a *ServiceAdapter) FulfillRequest(ctx context.Context, requestID *big.Int, resultBytes []byte) (string, error) {
	if a.txClient == nil {
		return "", fmt.Errorf("txsubmitter client not configured")
	}

	resp, err := a.txClient.FulfillRequest(ctx, requestID.String(), hex.EncodeToString(resultBytes))
	if err != nil {
		return "", fmt.Errorf("fulfill request: %w", err)
	}

	if resp.Error != "" {
		return "", fmt.Errorf("txsubmitter error: %s", resp.Error)
	}

	return resp.TxHash, nil
}

// ResolveDispute submits a dispute resolution via TxSubmitter.
func (a *ServiceAdapter) ResolveDispute(ctx context.Context, requestID string, completionProof []byte) (string, error) {
	if a.txClient == nil {
		return "", fmt.Errorf("txsubmitter client not configured")
	}

	resp, err := a.txClient.FulfillRequest(ctx, requestID, hex.EncodeToString(completionProof))
	if err != nil {
		return "", fmt.Errorf("resolve dispute: %w", err)
	}

	if resp.Error != "" {
		return "", fmt.Errorf("txsubmitter error: %s", resp.Error)
	}

	return resp.TxHash, nil
}

// =============================================================================
// Service Integration
// =============================================================================

// SetServiceAdapter sets the service adapter for chain and signing operations.
func (s *Service) SetServiceAdapter(adapter *ServiceAdapter) {
	s.serviceAdapter = adapter
}

// SetTxSubmitterClient sets the TxSubmitter client.
func (s *Service) SetTxSubmitterClient(client *txclient.Client) {
	if s.serviceAdapter == nil {
		s.serviceAdapter = &ServiceAdapter{}
	}
	s.serviceAdapter.txClient = client
}

// SetGlobalSignerClient sets the GlobalSigner client.
func (s *Service) SetGlobalSignerClient(client *gsclient.Client) {
	if s.serviceAdapter == nil {
		s.serviceAdapter = &ServiceAdapter{}
	}
	s.serviceAdapter.gsClient = client
}

// signRequestProofViaGlobalSigner signs using GlobalSigner instead of local key.
func (s *Service) signRequestProofViaGlobalSigner(ctx context.Context, requestHash []byte) ([]byte, string, error) {
	if s.serviceAdapter == nil || s.serviceAdapter.gsClient == nil {
		// Fallback to local signing
		return s.signWithMasterKey(requestHash)
	}
	return s.serviceAdapter.SignRequestProof(ctx, requestHash)
}

// signCompletionProofViaGlobalSigner signs using GlobalSigner instead of local key.
func (s *Service) signCompletionProofViaGlobalSigner(ctx context.Context, completionHash []byte) ([]byte, string, error) {
	if s.serviceAdapter == nil || s.serviceAdapter.gsClient == nil {
		// Fallback to local signing
		return s.signWithMasterKey(completionHash)
	}
	return s.serviceAdapter.SignCompletionProof(ctx, completionHash)
}

// signWithMasterKey is the legacy local signing method.
func (s *Service) signWithMasterKey(data []byte) ([]byte, string, error) {
	if len(s.masterKey) == 0 {
		return nil, "", fmt.Errorf("master key not configured")
	}
	// Legacy signing implementation would go here
	return nil, "local", fmt.Errorf("legacy signing not implemented - use GlobalSigner")
}
