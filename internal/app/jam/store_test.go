package jam

import (
	"context"
	"testing"
)

func TestInMemoryStoreEnqueueAndReport(t *testing.T) {
	store := NewInMemoryStore()
	pkg := WorkPackage{
		ID:        "pkg",
		ServiceID: "svc",
		Items:     []WorkItem{{ID: "i1", PackageID: "pkg", Kind: "k", ParamsHash: "p"}},
	}
	if err := store.EnqueuePackage(context.Background(), pkg); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	got, ok, err := store.NextPending(context.Background())
	if err != nil || !ok {
		t.Fatalf("expected pending package, err=%v", err)
	}
	if got.ID != pkg.ID {
		t.Fatalf("wrong package id: %s", got.ID)
	}

	report := WorkReport{ID: "rep", PackageID: pkg.ID, ServiceID: pkg.ServiceID, RefineOutputHash: "h"}
	attns := []Attestation{{ReportID: "rep", WorkerID: "w"}}
	if err := store.SaveReport(context.Background(), report, attns); err != nil {
		t.Fatalf("save report failed: %v", err)
	}
	if err := store.UpdatePackageStatus(context.Background(), pkg.ID, PackageStatusApplied); err != nil {
		t.Fatalf("update status failed: %v", err)
	}
	saved, ok := store.ReportFor(pkg.ID)
	if !ok || saved.ID != report.ID {
		t.Fatalf("report not found for package")
	}
	savedAttns := store.AttestationsFor(report.ID)
	if len(savedAttns) != 1 || savedAttns[0].WorkerID != "w" {
		t.Fatalf("unexpected attestations: %+v", savedAttns)
	}
}

func TestCoordinatorProcessesPackage(t *testing.T) {
	store := NewInMemoryStore()
	pkg := WorkPackage{
		ID:        "pkg",
		ServiceID: "svc",
		Items:     []WorkItem{{ID: "i1", PackageID: "pkg", Kind: "k", ParamsHash: "p"}},
	}
	if err := store.EnqueuePackage(context.Background(), pkg); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	engine := Engine{
		Preimages:   stubPreimages{},
		Refiner:     stubRefiner{report: WorkReport{ID: "rep", PackageID: "pkg", ServiceID: "svc", RefineOutputHash: "h"}},
		Attestors:   []Attestor{stubAttestor{id: "a1"}},
		Accumulator: stubAccumulator{},
		Threshold:   1,
	}
	coord := Coordinator{Store: store, Engine: engine}
	ok, err := coord.ProcessNext(context.Background())
	if err != nil {
		t.Fatalf("process failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected a package to be processed")
	}
	savedStatus := store.pkgs[pkg.ID].Status
	if savedStatus != PackageStatusApplied {
		t.Fatalf("status not applied: %s", savedStatus)
	}
	if _, ok := store.ReportFor(pkg.ID); !ok {
		t.Fatalf("report missing")
	}
}
