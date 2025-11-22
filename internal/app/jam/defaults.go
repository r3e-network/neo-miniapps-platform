package jam

import (
	"context"

	"github.com/google/uuid"
)

// HashRefiner produces a report whose output hash is the package hash.
type HashRefiner struct{}

func (HashRefiner) Refine(ctx context.Context, pkg WorkPackage, _ PreimageStore) (WorkReport, error) {
	if err := pkg.ValidateBasic(); err != nil {
		return WorkReport{}, err
	}
	sum, err := pkg.Hash()
	if err != nil {
		return WorkReport{}, err
	}
	return WorkReport{
		ID:               uuid.NewString(),
		PackageID:        pkg.ID,
		ServiceID:        pkg.ServiceID,
		RefineOutputHash: sum,
	}, nil
}

// StaticAttestor signs a report with a fixed worker id/weight.
type StaticAttestor struct {
	WorkerID string
	Weight   int64
}

func (s StaticAttestor) Attest(ctx context.Context, report WorkReport) (Attestation, error) {
	return Attestation{
		ReportID:      report.ID,
		WorkerID:      s.WorkerID,
		Weight:        s.Weight,
		Engine:        "hash-refiner",
		EngineVersion: "0.1",
	}, nil
}

// NoopAccumulator applies nothing; useful for prototypes.
type NoopAccumulator struct{}

func (NoopAccumulator) Accumulate(ctx context.Context, _ WorkReport, _ []Message) error {
	return nil
}
