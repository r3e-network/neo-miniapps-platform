package jam

import (
	"context"
	"errors"
)

// Engine wires refiner, attestors, and accumulator into a simple pipeline.
// It does not persist state; callers handle storage and scheduling.
type Engine struct {
	Preimages   PreimageStore
	Refiner     Refiner
	Attestors   []Attestor
	Accumulator Accumulator
	Threshold   int // minimum attestations required before apply
}

// Process runs refine -> attest (quorum) -> accumulate.
func (e Engine) Process(ctx context.Context, pkg WorkPackage) (WorkReport, []Attestation, error) {
	if err := e.validate(); err != nil {
		return WorkReport{}, nil, err
	}
	report, err := e.Refiner.Refine(ctx, pkg, e.Preimages)
	if err != nil {
		return WorkReport{}, nil, err
	}

	attns := make([]Attestation, 0, len(e.Attestors))
	for _, a := range e.Attestors {
		attn, err := a.Attest(ctx, report)
		if err != nil {
			return WorkReport{}, nil, err
		}
		attns = append(attns, attn)
		if len(attns) >= e.Threshold {
			break
		}
	}

	if len(attns) < e.Threshold {
		return WorkReport{}, attns, errors.New("insufficient attestations")
	}

	if err := e.Accumulator.Accumulate(ctx, report, nil); err != nil {
		return WorkReport{}, attns, err
	}
	return report, attns, nil
}

func (e Engine) validate() error {
	if e.Refiner == nil {
		return errors.New("refiner is required")
	}
	if e.Accumulator == nil {
		return errors.New("accumulator is required")
	}
	if e.Preimages == nil {
		return errors.New("preimage store is required")
	}
	if len(e.Attestors) == 0 {
		return errors.New("at least one attestor is required")
	}
	if e.Threshold <= 0 {
		return errors.New("threshold must be positive")
	}
	if e.Threshold > len(e.Attestors) {
		return errors.New("threshold exceeds available attestors")
	}
	return nil
}
