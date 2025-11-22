package jam

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
)

type stubPreimages struct{}

func (stubPreimages) Stat(_ context.Context, hash string) (Preimage, error) {
	return Preimage{Hash: hash}, nil
}
func (stubPreimages) Get(_ context.Context, _ string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (stubPreimages) Put(_ context.Context, hash string, _ string, _ io.Reader, _ int64) (Preimage, error) {
	return Preimage{Hash: hash}, nil
}

type stubRefiner struct {
	report WorkReport
	err    error
}

func (s stubRefiner) Refine(_ context.Context, _ WorkPackage, _ PreimageStore) (WorkReport, error) {
	return s.report, s.err
}

type stubAttestor struct {
	id      string
	err     error
	counter *atomic.Int64
}

func (s stubAttestor) Attest(_ context.Context, _ WorkReport) (Attestation, error) {
	if s.counter != nil {
		s.counter.Add(1)
	}
	if s.err != nil {
		return Attestation{}, s.err
	}
	return Attestation{WorkerID: s.id}, nil
}

type stubAccumulator struct {
	calls *atomic.Int64
	err   error
}

func (s stubAccumulator) Accumulate(_ context.Context, _ WorkReport, _ []Message) error {
	if s.calls != nil {
		s.calls.Add(1)
	}
	return s.err
}

func TestEngineHappyPath(t *testing.T) {
	refiner := stubRefiner{report: WorkReport{ID: "rep", PackageID: "pkg", ServiceID: "svc", RefineOutputHash: "h"}}
	attnCount := &atomic.Int64{}
	accumCount := &atomic.Int64{}

	engine := Engine{
		Preimages:   stubPreimages{},
		Refiner:     refiner,
		Attestors:   []Attestor{stubAttestor{id: "a1", counter: attnCount}, stubAttestor{id: "a2", counter: attnCount}},
		Accumulator: stubAccumulator{calls: accumCount},
		Threshold:   1,
	}
	report, attns, err := engine.Process(context.Background(), WorkPackage{
		ID:        "pkg",
		ServiceID: "svc",
		Items: []WorkItem{
			{ID: "i1", PackageID: "pkg", Kind: "k", ParamsHash: "p"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if report.ID != "rep" {
		t.Fatalf("wrong report")
	}
	if len(attns) != 1 {
		t.Fatalf("expected 1 attestation, got %d", len(attns))
	}
	if attnCount.Load() != 1 {
		t.Fatalf("attestor count %d", attnCount.Load())
	}
	if accumCount.Load() != 1 {
		t.Fatalf("accumulator count %d", accumCount.Load())
	}
}

func TestEngineThresholdEnforced(t *testing.T) {
	refiner := stubRefiner{report: WorkReport{ID: "rep", PackageID: "pkg", ServiceID: "svc", RefineOutputHash: "h"}}
	engine := Engine{
		Preimages:   stubPreimages{},
		Refiner:     refiner,
		Attestors:   []Attestor{stubAttestor{id: "a1", err: errors.New("fail")}},
		Accumulator: stubAccumulator{},
		Threshold:   1,
	}
	_, _, err := engine.Process(context.Background(), WorkPackage{
		ID:        "pkg",
		ServiceID: "svc",
		Items:     []WorkItem{{ID: "i1", PackageID: "pkg", Kind: "k", ParamsHash: "p"}},
	})
	if err == nil {
		t.Fatalf("expected error when attestor fails")
	}
}

func TestEngineValidation(t *testing.T) {
	engine := Engine{}
	_, _, err := engine.Process(context.Background(), WorkPackage{})
	if err == nil {
		t.Fatalf("expected validation error")
	}
}
