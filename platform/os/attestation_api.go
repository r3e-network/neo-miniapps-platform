// Package os provides the ServiceOS abstraction layer.
package os

import (
	"context"

	"github.com/R3E-Network/service_layer/tee/types"
)

// attestationAPIImpl implements AttestationAPI.
type attestationAPIImpl struct {
	ctx         *ServiceContext
	attestation types.Attestor
}

func newAttestationAPI(ctx *ServiceContext, attestation types.Attestor) *attestationAPIImpl {
	return &attestationAPIImpl{
		ctx:         ctx,
		attestation: attestation,
	}
}

func (a *attestationAPIImpl) GenerateQuote(ctx context.Context, userData []byte) (*Quote, error) {
	if err := a.ctx.RequireCapability(CapAttestation); err != nil {
		return nil, err
	}

	quote, err := a.attestation.GenerateQuote(ctx, userData)
	if err != nil {
		return nil, err
	}

	return &Quote{
		RawQuote:  quote.RawQuote,
		UserData:  quote.UserData,
		MREnclave: quote.MREnclave,
		MRSigner:  quote.MRSigner,
		Timestamp: quote.Timestamp,
	}, nil
}

func (a *attestationAPIImpl) VerifyQuote(ctx context.Context, quote *Quote) (*QuoteVerification, error) {
	if err := a.ctx.RequireCapability(CapAttestation); err != nil {
		return nil, err
	}

	teeQuote := &types.Quote{
		RawQuote:  quote.RawQuote,
		UserData:  quote.UserData,
		MREnclave: quote.MREnclave,
		MRSigner:  quote.MRSigner,
		Timestamp: quote.Timestamp,
	}

	result, err := a.attestation.VerifyQuote(ctx, teeQuote)
	if err != nil {
		return nil, err
	}

	return &QuoteVerification{
		Valid:      result.Valid,
		MREnclave:  result.MREnclave,
		MRSigner:   result.MRSigner,
		VerifiedAt: result.VerifiedAt,
	}, nil
}

func (a *attestationAPIImpl) GetReport(ctx context.Context) (*AttestationReport, error) {
	if err := a.ctx.RequireCapability(CapAttestation); err != nil {
		return nil, err
	}

	report, err := a.attestation.GetReport(ctx)
	if err != nil {
		return nil, err
	}

	return &AttestationReport{
		EnclaveID: report.EnclaveID,
		Mode:      report.Mode,
		MREnclave: report.MREnclave,
		MRSigner:  report.MRSigner,
		Timestamp: report.Timestamp,
	}, nil
}
